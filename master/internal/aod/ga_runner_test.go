package aod

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"master/internal/db"
	"master/internal/scheduler"
)

// TestInitializePopulation verifies population initialization
func TestInitializePopulation(t *testing.T) {
	config := GetDefaultGAConfig()
	config.PopulationSize = 20

	trainedTheta := scheduler.Theta{
		Theta1: 0.15,
		Theta2: 0.12,
		Theta3: 0.35,
		Theta4: 0.25,
	}

	population := initializePopulation(config, trainedTheta)

	if len(population) != config.PopulationSize {
		t.Errorf("Expected population size %d, got %d", config.PopulationSize, len(population))
	}

	// First chromosome should have trained Theta
	if population[0].Theta != trainedTheta {
		t.Errorf("First chromosome should have trained Theta")
	}

	// All chromosomes should be valid
	for i, chrom := range population {
		if !chrom.IsValid() {
			t.Errorf("Chromosome %d is invalid: %+v", i, chrom)
		}
	}
}

// TestEvaluatePopulation verifies fitness evaluation
func TestEvaluatePopulation(t *testing.T) {
	config := GetDefaultGAConfig()

	population := Population{
		{
			Theta:     scheduler.Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
			Risk:      scheduler.Risk{Alpha: 10.0, Beta: 1.0},
			AffinityW: scheduler.AffinityWeights{A1: 1.0, A2: 2.0, A3: 0.5},
			PenaltyW:  scheduler.PenaltyWeights{G1: 2.0, G2: 1.0, G3: 0.5},
			Fitness:   0.0,
		},
		{
			Theta:     scheduler.Theta{Theta1: 0.2, Theta2: 0.2, Theta3: 0.4, Theta4: 0.3},
			Risk:      scheduler.Risk{Alpha: 15.0, Beta: 1.5},
			AffinityW: scheduler.AffinityWeights{A1: 1.5, A2: 2.5, A3: 0.8},
			PenaltyW:  scheduler.PenaltyWeights{G1: 2.5, G2: 1.5, G3: 0.8},
			Fitness:   0.0,
		},
	}

	history := generateMockTaskHistory(50)
	workerStats := generateMockWorkerStats(3)

	evaluated := evaluatePopulation(population, history, workerStats, config)

	// All chromosomes should have non-zero fitness
	for i, chrom := range evaluated {
		if chrom.Fitness == 0.0 {
			t.Errorf("Chromosome %d has zero fitness", i)
		}
	}
}

// TestTournamentSelection verifies tournament selection logic
func TestTournamentSelection(t *testing.T) {
	population := Population{
		{Fitness: 0.5},
		{Fitness: 0.8},
		{Fitness: 0.3},
		{Fitness: 0.9},
		{Fitness: 0.6},
	}

	// Run selection multiple times to ensure it picks high-fitness individuals
	selectedCount := make(map[float64]int)
	iterations := 100

	for i := 0; i < iterations; i++ {
		selected := tournamentSelection(population, 3)
		selectedCount[selected.Fitness]++
	}

	// Higher fitness chromosomes should be selected more often
	// 0.9 and 0.8 should be selected more than 0.3
	if selectedCount[0.3] > selectedCount[0.9] {
		t.Errorf("Low fitness selected more than high fitness: %v", selectedCount)
	}
}

// TestTournamentSelectionEdgeCases tests edge cases
func TestTournamentSelectionEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		population     Population
		tournamentSize int
	}{
		{
			name:           "empty population",
			population:     Population{},
			tournamentSize: 3,
		},
		{
			name:           "single chromosome",
			population:     Population{{Fitness: 0.5}},
			tournamentSize: 3,
		},
		{
			name:           "tournament size larger than population",
			population:     Population{{Fitness: 0.5}, {Fitness: 0.8}},
			tournamentSize: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			_ = tournamentSelection(tt.population, tt.tournamentSize)
		})
	}
}

// TestCrossover verifies crossover operation
func TestCrossover(t *testing.T) {
	parent1 := Chromosome{
		Theta:     scheduler.Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
		Risk:      scheduler.Risk{Alpha: 10.0, Beta: 1.0},
		AffinityW: scheduler.AffinityWeights{A1: 1.0, A2: 2.0, A3: 0.5},
		PenaltyW:  scheduler.PenaltyWeights{G1: 2.0, G2: 1.0, G3: 0.5},
		Fitness:   0.5,
	}

	parent2 := Chromosome{
		Theta:     scheduler.Theta{Theta1: 0.2, Theta2: 0.2, Theta3: 0.4, Theta4: 0.3},
		Risk:      scheduler.Risk{Alpha: 15.0, Beta: 1.5},
		AffinityW: scheduler.AffinityWeights{A1: 1.5, A2: 2.5, A3: 0.8},
		PenaltyW:  scheduler.PenaltyWeights{G1: 2.5, G2: 1.5, G3: 0.8},
		Fitness:   0.8,
	}

	// Test crossover with high rate
	child1, child2 := crossover(parent1, parent2, 1.0)

	// Children should be different from parents (statistically)
	// At least one gene should differ
	if child1.Theta == parent1.Theta && child1.Risk == parent1.Risk &&
		child1.AffinityW == parent1.AffinityW && child1.PenaltyW == parent1.PenaltyW {
		// This could happen by chance, but unlikely with uniform crossover
		t.Log("Child1 identical to parent1 (rare but possible)")
	}

	// Verify both children are produced
	if child2.Fitness != 0.0 && child2.Fitness != parent2.Fitness {
		// Fitness should be reset to 0
		t.Error("Child fitness should be reset to 0")
	}

	// Test crossover with zero rate
	child1NoX, child2NoX := crossover(parent1, parent2, 0.0)

	// With 0% crossover rate, children should be clones of parents
	if child1NoX.Theta != parent1.Theta {
		t.Error("With 0% crossover, child1 should be clone of parent1")
	}
	if child2NoX.Theta != parent2.Theta {
		t.Error("With 0% crossover, child2 should be clone of parent2")
	}
}

// TestMutate verifies mutation operation
func TestMutate(t *testing.T) {
	original := Chromosome{
		Theta:     scheduler.Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
		Risk:      scheduler.Risk{Alpha: 10.0, Beta: 1.0},
		AffinityW: scheduler.AffinityWeights{A1: 1.0, A2: 2.0, A3: 0.5},
		PenaltyW:  scheduler.PenaltyWeights{G1: 2.0, G2: 1.0, G3: 0.5},
		Fitness:   0.5,
	}

	// Test mutation with high rate
	mutated := mutate(original, 1.0)

	// At least one gene should be different
	allSame := mutated.Theta == original.Theta &&
		mutated.Risk == original.Risk &&
		mutated.AffinityW == original.AffinityW &&
		mutated.PenaltyW == original.PenaltyW

	if allSame {
		t.Error("With 100% mutation rate, at least one gene should change")
	}

	// Mutated chromosome should still be valid
	if !mutated.IsValid() {
		t.Errorf("Mutated chromosome is invalid: %+v", mutated)
	}

	// Test mutation with zero rate
	notMutated := mutate(original, 0.0)

	if notMutated.Theta != original.Theta {
		t.Error("With 0% mutation rate, Theta should not change")
	}
}

// TestMutationBounds verifies that mutation respects parameter bounds
func TestMutationBounds(t *testing.T) {
	chromosome := Chromosome{
		Theta:     scheduler.Theta{Theta1: 1.9, Theta2: 1.9, Theta3: 1.9, Theta4: 1.9},
		Risk:      scheduler.Risk{Alpha: 95.0, Beta: 95.0},
		AffinityW: scheduler.AffinityWeights{A1: 9.5, A2: 9.5, A3: 9.5},
		PenaltyW:  scheduler.PenaltyWeights{G1: 9.5, G2: 9.5, G3: 9.5},
		Fitness:   0.5,
	}

	// Mutate many times
	for i := 0; i < 100; i++ {
		mutated := mutate(chromosome, 1.0)
		if !mutated.IsValid() {
			t.Errorf("Mutation produced invalid chromosome: %+v", mutated)
		}
	}
}

// TestSaveGAParams verifies parameter saving
func TestSaveGAParams(t *testing.T) {
	params := scheduler.GAParams{
		Theta:     scheduler.Theta{Theta1: 0.1, Theta2: 0.2, Theta3: 0.3, Theta4: 0.4},
		Risk:      scheduler.Risk{Alpha: 10.0, Beta: 1.0},
		AffinityW: scheduler.AffinityWeights{A1: 1.0, A2: 2.0, A3: 0.5},
		PenaltyW:  scheduler.PenaltyWeights{G1: 2.0, G2: 1.0, G3: 0.5},
		AffinityMatrix: map[string]map[string]float64{
			"cpu-light": {"worker1": 1.5, "worker2": -0.5},
		},
		PenaltyVector: map[string]float64{
			"worker1": 0.2,
			"worker2": 0.8,
		},
	}

	tmpFile := t.TempDir() + "/test_params.json"

	err := saveGAParams(params, tmpFile)
	if err != nil {
		t.Fatalf("saveGAParams failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var loaded scheduler.GAParams
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if loaded.Theta != params.Theta {
		t.Errorf("Theta mismatch: expected %+v, got %+v", params.Theta, loaded.Theta)
	}

	if loaded.AffinityMatrix["cpu-light"]["worker1"] != 1.5 {
		t.Error("AffinityMatrix not preserved correctly")
	}
}

// TestSaveDefaultParams verifies default parameter saving
func TestSaveDefaultParams(t *testing.T) {
	tmpFile := t.TempDir() + "/default_params.json"

	err := saveDefaultParams(tmpFile)
	if err != nil {
		t.Fatalf("saveDefaultParams failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var loaded scheduler.GAParams
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Check default values
	if loaded.Theta.Theta1 != 0.1 || loaded.Theta.Theta2 != 0.1 {
		t.Errorf("Unexpected default Theta: %+v", loaded.Theta)
	}
}

// TestDefaultHelpers verifies default value generation
func TestDefaultHelpers(t *testing.T) {
	theta := defaultTheta()
	if theta.Theta1 != 0.1 || theta.Theta2 != 0.1 || theta.Theta3 != 0.3 || theta.Theta4 != 0.2 {
		t.Errorf("Unexpected default Theta: %+v", theta)
	}

	risk := defaultRisk()
	if risk.Alpha != 10.0 || risk.Beta != 1.0 {
		t.Errorf("Unexpected default Risk: %+v", risk)
	}

	affinity := defaultAffinityWeights()
	if affinity.A1 != 1.0 || affinity.A2 != 2.0 || affinity.A3 != 0.5 {
		t.Errorf("Unexpected default AffinityWeights: %+v", affinity)
	}

	penalty := defaultPenaltyWeights()
	if penalty.G1 != 2.0 || penalty.G2 != 1.0 || penalty.G3 != 0.5 {
		t.Errorf("Unexpected default PenaltyWeights: %+v", penalty)
	}
}

// TestRandomHelpers verifies random initialization produces valid values
func TestRandomHelpers(t *testing.T) {
	// Test random generation multiple times
	for i := 0; i < 100; i++ {
		theta := randomTheta()
		if theta.Theta1 < 0 || theta.Theta1 > 0.5 {
			t.Errorf("randomTheta Theta1 out of range: %.4f", theta.Theta1)
		}
		if theta.Theta2 < 0 || theta.Theta2 > 0.5 {
			t.Errorf("randomTheta Theta2 out of range: %.4f", theta.Theta2)
		}

		risk := randomRisk()
		if risk.Alpha < 5.0 || risk.Alpha > 20.0 {
			t.Errorf("randomRisk Alpha out of range: %.4f", risk.Alpha)
		}
		if risk.Beta < 0.5 || risk.Beta > 2.5 {
			t.Errorf("randomRisk Beta out of range: %.4f", risk.Beta)
		}

		affinity := randomAffinityWeights()
		if affinity.A1 < 0.5 || affinity.A1 > 2.5 {
			t.Errorf("randomAffinityWeights A1 out of range: %.4f", affinity.A1)
		}

		penalty := randomPenaltyWeights()
		if penalty.G1 < 1.0 || penalty.G1 > 4.0 {
			t.Errorf("randomPenaltyWeights G1 out of range: %.4f", penalty.G1)
		}
	}
}

// Helper to generate mock task history for testing
func generateMockTaskHistory(count int) []db.TaskHistory {
	history := make([]db.TaskHistory, count)
	now := time.Now()

	taskTypes := []string{"cpu-light", "cpu-heavy", "memory-heavy", "gpu-inference", "gpu-training", "mixed"}
	workerIDs := []string{"worker1", "worker2", "worker3"}

	for i := 0; i < count; i++ {
		taskType := taskTypes[i%len(taskTypes)]
		workerID := workerIDs[i%len(workerIDs)]

		tau := 10.0
		runtime := tau * (0.8 + 0.4*float64(i%5)/5.0) // 80-120% of tau
		deadline := now.Add(time.Duration(tau*2) * time.Second)
		actualFinish := now.Add(time.Duration(runtime) * time.Second)

		history[i] = db.TaskHistory{
			TaskID:        "task-" + string(rune(i)),
			WorkerID:      workerID,
			Type:          taskType,
			ArrivalTime:   now,
			Deadline:      deadline,
			ActualStart:   now,
			ActualFinish:  actualFinish,
			ActualRuntime: runtime,
			SLASuccess:    actualFinish.Before(deadline) || actualFinish.Equal(deadline),
			CPUUsed:       2.0,
			MemUsed:       4.0,
			GPUUsed:       1.0,
			LoadAtStart:   0.5 + 0.3*float64(i%3)/3.0,
			Tau:           tau,
			SLAMultiplier: 2.0,
		}
	}

	return history
}

// Helper to generate mock worker stats for testing
func generateMockWorkerStats(count int) []db.WorkerStats {
	stats := make([]db.WorkerStats, count)
	now := time.Now()

	for i := 0; i < count; i++ {
		stats[i] = db.WorkerStats{
			WorkerID:      "worker" + string(rune('1'+i)),
			TasksRun:      50,
			SLAViolations: 5 + i*2, // Varying violation rates
			TotalRuntime:  500.0,
			CPUUsedTotal:  800.0,
			MemUsedTotal:  1600.0,
			GPUUsedTotal:  400.0,
			OverloadTime:  50.0 + float64(i*20), // Varying overload
			TotalTime:     1000.0,
			AvgLoad:       0.5 + 0.1*float64(i),
			PeriodStart:   now.Add(-24 * time.Hour),
			PeriodEnd:     now,
		}
	}

	return stats
}
