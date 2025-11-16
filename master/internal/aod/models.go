package aod

import (
	"master/internal/scheduler"
)

// Chromosome represents an individual in the genetic algorithm population.
// It contains all GA-evolvable parameters that affect scheduling decisions.
type Chromosome struct {
	// Theta parameters for execution time prediction (EDD §3.5)
	Theta scheduler.Theta

	// Risk parameters for base risk calculation (EDD §3.7)
	Risk scheduler.Risk

	// Affinity weights for computing affinity scores (EDD §5.3)
	AffinityW scheduler.AffinityWeights

	// Penalty weights for computing penalty scores (EDD §5.4)
	PenaltyW scheduler.PenaltyWeights

	// Fitness score for this chromosome (higher is better)
	Fitness float64
}

// Population represents a collection of chromosomes in the GA
type Population []Chromosome

// Metrics represents the metrics used for fitness computation (EDD §5.5)
type Metrics struct {
	// SLASuccess is the ratio of tasks that met their SLA deadline
	// Range: [0.0, 1.0], higher is better
	SLASuccess float64

	// Utilization is the average resource utilization across all workers
	// Range: [0.0, 1.0], higher is better (but not at cost of SLA)
	Utilization float64

	// EnergyNorm is the normalized energy consumption
	// Range: [0.0, 1.0], lower is better
	EnergyNorm float64

	// OverloadNorm is the normalized time workers spent overloaded
	// Range: [0.0, 1.0], lower is better
	OverloadNorm float64
}

// GAConfig represents the configuration parameters for the genetic algorithm
type GAConfig struct {
	// PopulationSize is the number of chromosomes in each generation
	// Typical range: 20-100
	// Larger = better exploration, slower convergence
	PopulationSize int

	// Generations is the number of GA iterations to run per epoch
	// Typical range: 10-50
	// More generations = better convergence, longer runtime
	Generations int

	// MutationRate is the probability of mutating each gene
	// Range: [0.0, 1.0], typical: 0.05-0.2
	// Higher = more exploration, risk of instability
	MutationRate float64

	// CrossoverRate is the probability of crossover between parents
	// Range: [0.0, 1.0], typical: 0.6-0.9
	// Higher = more offspring diversity
	CrossoverRate float64

	// FitnessWeights are the weights for computing fitness (EDD §5.5)
	// [w1, w2, w3, w4] for [SLA, Utilization, Energy, Overload]
	// Fitness = w1*SLA + w2*Util - w3*Energy - w4*Overload
	FitnessWeights [4]float64

	// ElitismCount is the number of top chromosomes to preserve unchanged
	// Typical: 1-5
	// Ensures best solutions are never lost
	ElitismCount int

	// TournamentSize is the number of candidates in tournament selection
	// Typical: 2-5
	// Higher = stronger selection pressure
	TournamentSize int
}

// GetDefaultGAConfig returns sensible default GA configuration
func GetDefaultGAConfig() GAConfig {
	return GAConfig{
		PopulationSize: 20,
		Generations:    10,
		MutationRate:   0.1,
		CrossoverRate:  0.7,
		FitnessWeights: [4]float64{
			0.4, // w1: SLA success (most important)
			0.3, // w2: Utilization (important)
			0.2, // w3: Energy (moderate)
			0.1, // w4: Overload (less important, often correlated with SLA)
		},
		ElitismCount:   2,
		TournamentSize: 3,
	}
}

// Clone creates a deep copy of a chromosome
func (c *Chromosome) Clone() Chromosome {
	clone := Chromosome{
		Theta: c.Theta,
		Risk:  c.Risk,
		AffinityW: scheduler.AffinityWeights{
			A1: c.AffinityW.A1,
			A2: c.AffinityW.A2,
			A3: c.AffinityW.A3,
		},
		PenaltyW: scheduler.PenaltyWeights{
			G1: c.PenaltyW.G1,
			G2: c.PenaltyW.G2,
			G3: c.PenaltyW.G3,
		},
		Fitness: c.Fitness,
	}
	return clone
}

// IsValid checks if a chromosome has valid parameter values
func (c *Chromosome) IsValid() bool {
	// Theta parameters should be positive and reasonable
	if c.Theta.Theta1 < 0 || c.Theta.Theta1 > 2.0 {
		return false
	}
	if c.Theta.Theta2 < 0 || c.Theta.Theta2 > 2.0 {
		return false
	}
	if c.Theta.Theta3 < 0 || c.Theta.Theta3 > 2.0 {
		return false
	}
	if c.Theta.Theta4 < 0 || c.Theta.Theta4 > 2.0 {
		return false
	}

	// Risk parameters should be positive
	if c.Risk.Alpha <= 0 || c.Risk.Alpha > 100.0 {
		return false
	}
	if c.Risk.Beta <= 0 || c.Risk.Beta > 100.0 {
		return false
	}

	// Affinity weights should be positive
	if c.AffinityW.A1 <= 0 || c.AffinityW.A1 > 10.0 {
		return false
	}
	if c.AffinityW.A2 <= 0 || c.AffinityW.A2 > 10.0 {
		return false
	}
	if c.AffinityW.A3 < 0 || c.AffinityW.A3 > 10.0 {
		return false
	}

	// Penalty weights should be positive
	if c.PenaltyW.G1 < 0 || c.PenaltyW.G1 > 10.0 {
		return false
	}
	if c.PenaltyW.G2 < 0 || c.PenaltyW.G2 > 10.0 {
		return false
	}
	if c.PenaltyW.G3 < 0 || c.PenaltyW.G3 > 10.0 {
		return false
	}

	return true
}

// Len returns the number of chromosomes in the population
func (p Population) Len() int {
	return len(p)
}

// Swap swaps two chromosomes in the population (for sorting)
func (p Population) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

// Less compares two chromosomes by fitness (for sorting in descending order)
func (p Population) Less(i, j int) bool {
	return p[i].Fitness > p[j].Fitness // Higher fitness is better
}

// GetBest returns the chromosome with the highest fitness
func (p Population) GetBest() Chromosome {
	if len(p) == 0 {
		return Chromosome{}
	}

	best := p[0]
	for i := 1; i < len(p); i++ {
		if p[i].Fitness > best.Fitness {
			best = p[i]
		}
	}
	return best
}

// GetWorst returns the chromosome with the lowest fitness
func (p Population) GetWorst() Chromosome {
	if len(p) == 0 {
		return Chromosome{}
	}

	worst := p[0]
	for i := 1; i < len(p); i++ {
		if p[i].Fitness < worst.Fitness {
			worst = p[i]
		}
	}
	return worst
}

// GetAverageFitness computes the average fitness across the population
func (p Population) GetAverageFitness() float64 {
	if len(p) == 0 {
		return 0.0
	}

	sum := 0.0
	for i := 0; i < len(p); i++ {
		sum += p[i].Fitness
	}
	return sum / float64(len(p))
}

// ComputeMetrics computes fitness metrics from individual components
func (m *Metrics) ComputeFitness(weights [4]float64) float64 {
	// Fitness = w1*SLA + w2*Util - w3*Energy - w4*Overload
	// Higher values are better (maximize SLA and Util, minimize Energy and Overload)
	fitness := weights[0]*m.SLASuccess +
		weights[1]*m.Utilization -
		weights[2]*m.EnergyNorm -
		weights[3]*m.OverloadNorm

	return fitness
}

// IsValid checks if metrics are within valid ranges
func (m *Metrics) IsValid() bool {
	// All metrics should be in [0, 1] range
	if m.SLASuccess < 0 || m.SLASuccess > 1.0 {
		return false
	}
	if m.Utilization < 0 || m.Utilization > 1.0 {
		return false
	}
	if m.EnergyNorm < 0 || m.EnergyNorm > 1.0 {
		return false
	}
	if m.OverloadNorm < 0 || m.OverloadNorm > 1.0 {
		return false
	}
	return true
}
