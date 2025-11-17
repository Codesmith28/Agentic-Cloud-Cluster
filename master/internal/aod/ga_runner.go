package aod

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"master/internal/db"
	"master/internal/scheduler"
)

// RunGAEpoch executes one complete genetic algorithm training cycle.
//
// This function:
// 1. Fetches historical task and worker data from the database
// 2. Trains Theta parameters using linear regression
// 3. Evolves a population of chromosomes using genetic operators
// 4. Builds affinity and penalty matrices from the best chromosome
// 5. Saves the optimized parameters to a JSON file for RTS to load
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - historyDB: Database connection for fetching historical data
//   - config: GA configuration (population size, mutation rate, etc.)
//   - paramsOutputPath: File path to save the optimized GAParams JSON
//
// Returns: error if any step fails
func RunGAEpoch(ctx context.Context, historyDB *db.HistoryDB, config GAConfig, paramsOutputPath string) error {
	log.Println("🧬 Starting GA epoch...")
	startTime := time.Now()

	// Step 1: Fetch historical data (last 24 hours)
	until := time.Now()
	since := until.Add(-24 * time.Hour)

	log.Printf("📊 Fetching task history from %s to %s", since.Format(time.RFC3339), until.Format(time.RFC3339))
	history, err := historyDB.GetTaskHistory(ctx, since, until)
	if err != nil {
		return fmt.Errorf("fetch task history: %w", err)
	}

	log.Printf("📊 Fetching worker stats from %s to %s", since.Format(time.RFC3339), until.Format(time.RFC3339))
	workerStats, err := historyDB.GetWorkerStats(ctx, since, until)
	if err != nil {
		return fmt.Errorf("fetch worker stats: %w", err)
	}

	log.Printf("✓ Retrieved %d task history records and %d worker stats", len(history), len(workerStats))

	// Step 2: Check if we have sufficient data
	minDataPoints := 2 // Minimum tasks required for meaningful training
	if len(history) < minDataPoints {
		log.Printf("⚠️  Insufficient data (%d tasks < %d required), using default parameters", len(history), minDataPoints)
		return saveDefaultParams(paramsOutputPath)
	}

	// Step 3: Train Theta using linear regression
	log.Println("🔧 Training Theta parameters using linear regression...")
	theta := TrainTheta(history)
	log.Printf("✓ Theta trained: θ₁=%.4f, θ₂=%.4f, θ₃=%.4f, θ₄=%.4f",
		theta.Theta1, theta.Theta2, theta.Theta3, theta.Theta4)

	// Step 4: Initialize population
	log.Printf("🧬 Initializing population (size=%d)", config.PopulationSize)
	population := initializePopulation(config, theta)
	log.Printf("✓ Population initialized with %d chromosomes", len(population))

	// Step 5: Evolve population over multiple generations
	for gen := 0; gen < config.Generations; gen++ {
		// Evaluate fitness for all chromosomes
		population = evaluatePopulation(population, history, workerStats, config)

		// Get statistics for logging
		best := population.GetBest()
		avg := population.GetAverageFitness()
		worst := population.GetWorst()

		log.Printf("🧬 Generation %d/%d: Best=%.4f, Avg=%.4f, Worst=%.4f",
			gen+1, config.Generations, best.Fitness, avg, worst.Fitness)

		// If this is the last generation, don't evolve further
		if gen == config.Generations-1 {
			break
		}

		// Sort population by fitness (best first)
		sort.Sort(population)

		// Create next generation
		nextGen := make(Population, 0, config.PopulationSize)

		// Elitism: preserve top chromosomes unchanged
		for i := 0; i < config.ElitismCount && i < len(population); i++ {
			nextGen = append(nextGen, population[i].Clone())
		}

		// Fill rest of population with offspring
		for len(nextGen) < config.PopulationSize {
			// Selection: pick two parents
			parent1 := tournamentSelection(population, config.TournamentSize)
			parent2 := tournamentSelection(population, config.TournamentSize)

			// Crossover: combine parents to create offspring
			child1, child2 := crossover(parent1, parent2, config.CrossoverRate)

			// Mutation: introduce random variations
			child1 = mutate(child1, config.MutationRate)
			child2 = mutate(child2, config.MutationRate)

			// Add valid offspring to next generation
			if child1.IsValid() && len(nextGen) < config.PopulationSize {
				nextGen = append(nextGen, child1)
			}
			if child2.IsValid() && len(nextGen) < config.PopulationSize {
				nextGen = append(nextGen, child2)
			}
		}

		population = nextGen
	}

	// Step 6: Get best chromosome from final population
	sort.Sort(population)
	best := population.GetBest()
	log.Printf("🏆 Best chromosome fitness: %.4f", best.Fitness)

	// Step 7: Build affinity matrix using best AffinityWeights
	log.Println("🔧 Building affinity matrix from best chromosome...")
	affinityMatrix := BuildAffinityMatrix(history, best.AffinityW)
	log.Printf("✓ Affinity matrix built with %d task types", len(affinityMatrix))

	// Step 8: Build penalty vector using best PenaltyWeights
	log.Println("🔧 Building penalty vector from best chromosome...")
	penaltyVector := BuildPenaltyVector(workerStats, best.PenaltyW)
	log.Printf("✓ Penalty vector built for %d workers", len(penaltyVector))

	// Step 9: Create GAParams structure
	params := scheduler.GAParams{
		Theta:          best.Theta,
		Risk:           best.Risk,
		AffinityW:      best.AffinityW,
		PenaltyW:       best.PenaltyW,
		AffinityMatrix: affinityMatrix,
		PenaltyVector:  penaltyVector,
	}

	// Step 10: Save to JSON file
	if err := saveGAParams(params, paramsOutputPath); err != nil {
		return fmt.Errorf("save GA params: %w", err)
	}

	elapsed := time.Since(startTime)
	log.Printf("✅ GA epoch completed in %s, parameters saved to %s", elapsed, paramsOutputPath)

	return nil
}

// initializePopulation creates the initial population of chromosomes.
//
// The population is seeded with one chromosome using the trained Theta values,
// and the rest are random variations around sensible defaults.
func initializePopulation(config GAConfig, trainedTheta scheduler.Theta) Population {
	population := make(Population, 0, config.PopulationSize)

	// First chromosome uses trained Theta and default weights
	baseChromosome := Chromosome{
		Theta:     trainedTheta,
		Risk:      defaultRisk(),
		AffinityW: defaultAffinityWeights(),
		PenaltyW:  defaultPenaltyWeights(),
		Fitness:   0.0,
	}
	population = append(population, baseChromosome)

	// Generate random variations for the rest
	for i := 1; i < config.PopulationSize; i++ {
		chromosome := Chromosome{
			Theta:     randomTheta(),
			Risk:      randomRisk(),
			AffinityW: randomAffinityWeights(),
			PenaltyW:  randomPenaltyWeights(),
			Fitness:   0.0,
		}

		// Ensure chromosome is valid
		if chromosome.IsValid() {
			population = append(population, chromosome)
		} else {
			// If invalid, use a slight variation of base chromosome
			population = append(population, mutate(baseChromosome, 0.1))
		}
	}

	return population
}

// evaluatePopulation computes fitness for each chromosome in the population.
func evaluatePopulation(pop Population, history []db.TaskHistory, workerStats []db.WorkerStats, config GAConfig) Population {
	for i := 0; i < len(pop); i++ {
		pop[i].Fitness = ComputeFitness(pop[i], history, workerStats, config)
	}
	return pop
}

// tournamentSelection selects a chromosome using tournament selection.
//
// Tournament selection:
// 1. Randomly pick k chromosomes from the population
// 2. Return the one with the highest fitness
//
// This provides selection pressure while maintaining diversity.
func tournamentSelection(pop Population, tournamentSize int) Chromosome {
	if len(pop) == 0 {
		return Chromosome{}
	}

	// Handle edge case where tournament size > population size
	k := tournamentSize
	if k > len(pop) {
		k = len(pop)
	}

	// Randomly select k chromosomes
	best := pop[rand.Intn(len(pop))]
	for i := 1; i < k; i++ {
		candidate := pop[rand.Intn(len(pop))]
		if candidate.Fitness > best.Fitness {
			best = candidate
		}
	}

	return best.Clone()
}

// crossover combines two parent chromosomes to create two offspring.
//
// Uniform crossover: each gene has a 50% chance of coming from either parent.
// If random value < crossoverRate, perform crossover; otherwise return clones.
func crossover(parent1, parent2 Chromosome, crossoverRate float64) (Chromosome, Chromosome) {
	// Decide whether to perform crossover
	if rand.Float64() > crossoverRate {
		return parent1.Clone(), parent2.Clone()
	}

	child1 := Chromosome{}
	child2 := Chromosome{}

	// Crossover Theta parameters
	if rand.Float64() < 0.5 {
		child1.Theta.Theta1 = parent1.Theta.Theta1
		child2.Theta.Theta1 = parent2.Theta.Theta1
	} else {
		child1.Theta.Theta1 = parent2.Theta.Theta1
		child2.Theta.Theta1 = parent1.Theta.Theta1
	}

	if rand.Float64() < 0.5 {
		child1.Theta.Theta2 = parent1.Theta.Theta2
		child2.Theta.Theta2 = parent2.Theta.Theta2
	} else {
		child1.Theta.Theta2 = parent2.Theta.Theta2
		child2.Theta.Theta2 = parent1.Theta.Theta2
	}

	if rand.Float64() < 0.5 {
		child1.Theta.Theta3 = parent1.Theta.Theta3
		child2.Theta.Theta3 = parent2.Theta.Theta3
	} else {
		child1.Theta.Theta3 = parent2.Theta.Theta3
		child2.Theta.Theta3 = parent1.Theta.Theta3
	}

	if rand.Float64() < 0.5 {
		child1.Theta.Theta4 = parent1.Theta.Theta4
		child2.Theta.Theta4 = parent2.Theta.Theta4
	} else {
		child1.Theta.Theta4 = parent2.Theta.Theta4
		child2.Theta.Theta4 = parent1.Theta.Theta4
	}

	// Crossover Risk parameters
	if rand.Float64() < 0.5 {
		child1.Risk.Alpha = parent1.Risk.Alpha
		child2.Risk.Alpha = parent2.Risk.Alpha
	} else {
		child1.Risk.Alpha = parent2.Risk.Alpha
		child2.Risk.Alpha = parent1.Risk.Alpha
	}

	if rand.Float64() < 0.5 {
		child1.Risk.Beta = parent1.Risk.Beta
		child2.Risk.Beta = parent2.Risk.Beta
	} else {
		child1.Risk.Beta = parent2.Risk.Beta
		child2.Risk.Beta = parent1.Risk.Beta
	}

	// Crossover AffinityWeights
	if rand.Float64() < 0.5 {
		child1.AffinityW.A1 = parent1.AffinityW.A1
		child2.AffinityW.A1 = parent2.AffinityW.A1
	} else {
		child1.AffinityW.A1 = parent2.AffinityW.A1
		child2.AffinityW.A1 = parent1.AffinityW.A1
	}

	if rand.Float64() < 0.5 {
		child1.AffinityW.A2 = parent1.AffinityW.A2
		child2.AffinityW.A2 = parent2.AffinityW.A2
	} else {
		child1.AffinityW.A2 = parent2.AffinityW.A2
		child2.AffinityW.A2 = parent1.AffinityW.A2
	}

	if rand.Float64() < 0.5 {
		child1.AffinityW.A3 = parent1.AffinityW.A3
		child2.AffinityW.A3 = parent2.AffinityW.A3
	} else {
		child1.AffinityW.A3 = parent2.AffinityW.A3
		child2.AffinityW.A3 = parent1.AffinityW.A3
	}

	// Crossover PenaltyWeights
	if rand.Float64() < 0.5 {
		child1.PenaltyW.G1 = parent1.PenaltyW.G1
		child2.PenaltyW.G1 = parent2.PenaltyW.G1
	} else {
		child1.PenaltyW.G1 = parent2.PenaltyW.G1
		child2.PenaltyW.G1 = parent1.PenaltyW.G1
	}

	if rand.Float64() < 0.5 {
		child1.PenaltyW.G2 = parent1.PenaltyW.G2
		child2.PenaltyW.G2 = parent2.PenaltyW.G2
	} else {
		child1.PenaltyW.G2 = parent2.PenaltyW.G2
		child2.PenaltyW.G2 = parent1.PenaltyW.G2
	}

	if rand.Float64() < 0.5 {
		child1.PenaltyW.G3 = parent1.PenaltyW.G3
		child2.PenaltyW.G3 = parent2.PenaltyW.G3
	} else {
		child1.PenaltyW.G3 = parent2.PenaltyW.G3
		child2.PenaltyW.G3 = parent1.PenaltyW.G3
	}

	child1.Fitness = 0.0
	child2.Fitness = 0.0

	return child1, child2
}

// mutate introduces random variations to a chromosome.
//
// Each gene has a probability of mutationRate to be mutated.
// Mutation adds Gaussian noise to the parameter value.
func mutate(c Chromosome, mutationRate float64) Chromosome {
	mutant := c.Clone()

	// Mutate Theta parameters
	if rand.Float64() < mutationRate {
		mutant.Theta.Theta1 += rand.NormFloat64() * 0.1
		mutant.Theta.Theta1 = math.Max(0.0, math.Min(2.0, mutant.Theta.Theta1))
	}
	if rand.Float64() < mutationRate {
		mutant.Theta.Theta2 += rand.NormFloat64() * 0.1
		mutant.Theta.Theta2 = math.Max(0.0, math.Min(2.0, mutant.Theta.Theta2))
	}
	if rand.Float64() < mutationRate {
		mutant.Theta.Theta3 += rand.NormFloat64() * 0.1
		mutant.Theta.Theta3 = math.Max(0.0, math.Min(2.0, mutant.Theta.Theta3))
	}
	if rand.Float64() < mutationRate {
		mutant.Theta.Theta4 += rand.NormFloat64() * 0.1
		mutant.Theta.Theta4 = math.Max(0.0, math.Min(2.0, mutant.Theta.Theta4))
	}

	// Mutate Risk parameters
	if rand.Float64() < mutationRate {
		mutant.Risk.Alpha += rand.NormFloat64() * 2.0
		mutant.Risk.Alpha = math.Max(0.1, math.Min(100.0, mutant.Risk.Alpha))
	}
	if rand.Float64() < mutationRate {
		mutant.Risk.Beta += rand.NormFloat64() * 0.5
		mutant.Risk.Beta = math.Max(0.1, math.Min(100.0, mutant.Risk.Beta))
	}

	// Mutate AffinityWeights
	if rand.Float64() < mutationRate {
		mutant.AffinityW.A1 += rand.NormFloat64() * 0.2
		mutant.AffinityW.A1 = math.Max(0.1, math.Min(10.0, mutant.AffinityW.A1))
	}
	if rand.Float64() < mutationRate {
		mutant.AffinityW.A2 += rand.NormFloat64() * 0.2
		mutant.AffinityW.A2 = math.Max(0.1, math.Min(10.0, mutant.AffinityW.A2))
	}
	if rand.Float64() < mutationRate {
		mutant.AffinityW.A3 += rand.NormFloat64() * 0.2
		mutant.AffinityW.A3 = math.Max(0.0, math.Min(10.0, mutant.AffinityW.A3))
	}

	// Mutate PenaltyWeights
	if rand.Float64() < mutationRate {
		mutant.PenaltyW.G1 += rand.NormFloat64() * 0.2
		mutant.PenaltyW.G1 = math.Max(0.0, math.Min(10.0, mutant.PenaltyW.G1))
	}
	if rand.Float64() < mutationRate {
		mutant.PenaltyW.G2 += rand.NormFloat64() * 0.2
		mutant.PenaltyW.G2 = math.Max(0.0, math.Min(10.0, mutant.PenaltyW.G2))
	}
	if rand.Float64() < mutationRate {
		mutant.PenaltyW.G3 += rand.NormFloat64() * 0.2
		mutant.PenaltyW.G3 = math.Max(0.0, math.Min(10.0, mutant.PenaltyW.G3))
	}

	mutant.Fitness = 0.0
	return mutant
}

// saveGAParams writes GAParams to a JSON file
func saveGAParams(params scheduler.GAParams, filePath string) error {
	data, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	log.Printf("✓ GA parameters saved to %s", filePath)
	return nil
}

// saveDefaultParams writes default GAParams to JSON file
func saveDefaultParams(filePath string) error {
	params := scheduler.GAParams{
		Theta:          defaultTheta(),
		Risk:           defaultRisk(),
		AffinityW:      defaultAffinityWeights(),
		PenaltyW:       defaultPenaltyWeights(),
		AffinityMatrix: make(map[string]map[string]float64),
		PenaltyVector:  make(map[string]float64),
	}
	return saveGAParams(params, filePath)
}

// Helper functions for default values

func defaultTheta() scheduler.Theta {
	return scheduler.Theta{
		Theta1: 0.1,
		Theta2: 0.1,
		Theta3: 0.3,
		Theta4: 0.2,
	}
}

func defaultRisk() scheduler.Risk {
	return scheduler.Risk{
		Alpha: 10.0,
		Beta:  1.0,
	}
}

func defaultAffinityWeights() scheduler.AffinityWeights {
	return scheduler.AffinityWeights{
		A1: 1.0,
		A2: 2.0,
		A3: 0.5,
	}
}

func defaultPenaltyWeights() scheduler.PenaltyWeights {
	return scheduler.PenaltyWeights{
		G1: 2.0,
		G2: 1.0,
		G3: 0.5,
	}
}

// Helper functions for random initialization

func randomTheta() scheduler.Theta {
	return scheduler.Theta{
		Theta1: rand.Float64() * 0.5, // [0, 0.5]
		Theta2: rand.Float64() * 0.5, // [0, 0.5]
		Theta3: rand.Float64() * 0.8, // [0, 0.8]
		Theta4: rand.Float64() * 0.6, // [0, 0.6]
	}
}

func randomRisk() scheduler.Risk {
	return scheduler.Risk{
		Alpha: 5.0 + rand.Float64()*15.0, // [5, 20]
		Beta:  0.5 + rand.Float64()*2.0,  // [0.5, 2.5]
	}
}

func randomAffinityWeights() scheduler.AffinityWeights {
	return scheduler.AffinityWeights{
		A1: 0.5 + rand.Float64()*2.0, // [0.5, 2.5]
		A2: 1.0 + rand.Float64()*3.0, // [1.0, 4.0]
		A3: rand.Float64() * 1.5,     // [0, 1.5]
	}
}

func randomPenaltyWeights() scheduler.PenaltyWeights {
	return scheduler.PenaltyWeights{
		G1: 1.0 + rand.Float64()*3.0, // [1.0, 4.0]
		G2: 0.5 + rand.Float64()*2.0, // [0.5, 2.5]
		G3: rand.Float64() * 1.5,     // [0, 1.5]
	}
}
