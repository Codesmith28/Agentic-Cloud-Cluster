package aod

import (
	"master/internal/scheduler"
	"sort"
	"testing"
)

// TestChromosomeClone tests that Clone creates a proper deep copy
func TestChromosomeClone(t *testing.T) {
	original := Chromosome{
		Theta: scheduler.Theta{
			Theta1: 0.1,
			Theta2: 0.2,
			Theta3: 0.3,
			Theta4: 0.4,
		},
		Risk: scheduler.Risk{
			Alpha: 10.0,
			Beta:  1.0,
		},
		AffinityW: scheduler.AffinityWeights{
			A1: 1.0,
			A2: 2.0,
			A3: 0.5,
		},
		PenaltyW: scheduler.PenaltyWeights{
			G1: 2.0,
			G2: 1.0,
			G3: 0.5,
		},
		Fitness: 0.85,
	}

	clone := original.Clone()

	// Verify values are equal
	if clone.Theta.Theta1 != original.Theta.Theta1 {
		t.Errorf("Clone Theta1 = %f, want %f", clone.Theta.Theta1, original.Theta.Theta1)
	}
	if clone.Risk.Alpha != original.Risk.Alpha {
		t.Errorf("Clone Alpha = %f, want %f", clone.Risk.Alpha, original.Risk.Alpha)
	}
	if clone.Fitness != original.Fitness {
		t.Errorf("Clone Fitness = %f, want %f", clone.Fitness, original.Fitness)
	}

	// Modify clone and verify original is unchanged
	clone.Theta.Theta1 = 0.9
	clone.Fitness = 0.95

	if original.Theta.Theta1 != 0.1 {
		t.Errorf("Original Theta1 changed after clone modification")
	}
	if original.Fitness != 0.85 {
		t.Errorf("Original Fitness changed after clone modification")
	}
}

// TestChromosomeIsValid tests validation of chromosome parameters
func TestChromosomeIsValid(t *testing.T) {
	tests := []struct {
		name       string
		chromosome Chromosome
		wantValid  bool
	}{
		{
			name: "Valid chromosome",
			chromosome: Chromosome{
				Theta: scheduler.Theta{
					Theta1: 0.1,
					Theta2: 0.1,
					Theta3: 0.3,
					Theta4: 0.2,
				},
				Risk: scheduler.Risk{
					Alpha: 10.0,
					Beta:  1.0,
				},
				AffinityW: scheduler.AffinityWeights{
					A1: 1.0,
					A2: 2.0,
					A3: 0.5,
				},
				PenaltyW: scheduler.PenaltyWeights{
					G1: 2.0,
					G2: 1.0,
					G3: 0.5,
				},
			},
			wantValid: true,
		},
		{
			name: "Invalid Theta1 (negative)",
			chromosome: Chromosome{
				Theta: scheduler.Theta{
					Theta1: -0.1,
					Theta2: 0.1,
					Theta3: 0.3,
					Theta4: 0.2,
				},
				Risk: scheduler.Risk{Alpha: 10.0, Beta: 1.0},
				AffinityW: scheduler.AffinityWeights{
					A1: 1.0,
					A2: 2.0,
					A3: 0.5,
				},
				PenaltyW: scheduler.PenaltyWeights{
					G1: 2.0,
					G2: 1.0,
					G3: 0.5,
				},
			},
			wantValid: false,
		},
		{
			name: "Invalid Theta1 (too large)",
			chromosome: Chromosome{
				Theta: scheduler.Theta{
					Theta1: 2.5,
					Theta2: 0.1,
					Theta3: 0.3,
					Theta4: 0.2,
				},
				Risk: scheduler.Risk{Alpha: 10.0, Beta: 1.0},
				AffinityW: scheduler.AffinityWeights{
					A1: 1.0,
					A2: 2.0,
					A3: 0.5,
				},
				PenaltyW: scheduler.PenaltyWeights{
					G1: 2.0,
					G2: 1.0,
					G3: 0.5,
				},
			},
			wantValid: false,
		},
		{
			name: "Invalid Alpha (zero)",
			chromosome: Chromosome{
				Theta: scheduler.Theta{
					Theta1: 0.1,
					Theta2: 0.1,
					Theta3: 0.3,
					Theta4: 0.2,
				},
				Risk: scheduler.Risk{Alpha: 0.0, Beta: 1.0},
				AffinityW: scheduler.AffinityWeights{
					A1: 1.0,
					A2: 2.0,
					A3: 0.5,
				},
				PenaltyW: scheduler.PenaltyWeights{
					G1: 2.0,
					G2: 1.0,
					G3: 0.5,
				},
			},
			wantValid: false,
		},
		{
			name: "Invalid AffinityW A1 (negative)",
			chromosome: Chromosome{
				Theta: scheduler.Theta{
					Theta1: 0.1,
					Theta2: 0.1,
					Theta3: 0.3,
					Theta4: 0.2,
				},
				Risk: scheduler.Risk{Alpha: 10.0, Beta: 1.0},
				AffinityW: scheduler.AffinityWeights{
					A1: -1.0,
					A2: 2.0,
					A3: 0.5,
				},
				PenaltyW: scheduler.PenaltyWeights{
					G1: 2.0,
					G2: 1.0,
					G3: 0.5,
				},
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.chromosome.IsValid()
			if got != tt.wantValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

// TestPopulationSorting tests that population sorts by fitness correctly
func TestPopulationSorting(t *testing.T) {
	pop := Population{
		{Fitness: 0.5},
		{Fitness: 0.9},
		{Fitness: 0.3},
		{Fitness: 0.7},
	}

	sort.Sort(pop)

	// After sorting, should be in descending order
	expected := []float64{0.9, 0.7, 0.5, 0.3}
	for i, c := range pop {
		if c.Fitness != expected[i] {
			t.Errorf("After sort, pop[%d].Fitness = %f, want %f", i, c.Fitness, expected[i])
		}
	}
}

// TestPopulationGetBest tests getting the best chromosome
func TestPopulationGetBest(t *testing.T) {
	pop := Population{
		{Fitness: 0.5},
		{Fitness: 0.9},
		{Fitness: 0.3},
		{Fitness: 0.7},
	}

	best := pop.GetBest()
	if best.Fitness != 0.9 {
		t.Errorf("GetBest().Fitness = %f, want 0.9", best.Fitness)
	}

	// Test empty population
	emptyPop := Population{}
	emptyBest := emptyPop.GetBest()
	if emptyBest.Fitness != 0.0 {
		t.Errorf("GetBest() on empty population should return zero chromosome")
	}
}

// TestPopulationGetWorst tests getting the worst chromosome
func TestPopulationGetWorst(t *testing.T) {
	pop := Population{
		{Fitness: 0.5},
		{Fitness: 0.9},
		{Fitness: 0.3},
		{Fitness: 0.7},
	}

	worst := pop.GetWorst()
	if worst.Fitness != 0.3 {
		t.Errorf("GetWorst().Fitness = %f, want 0.3", worst.Fitness)
	}
}

// TestPopulationGetAverageFitness tests average fitness calculation
func TestPopulationGetAverageFitness(t *testing.T) {
	pop := Population{
		{Fitness: 0.4},
		{Fitness: 0.6},
		{Fitness: 0.8},
		{Fitness: 0.2},
	}

	avg := pop.GetAverageFitness()
	expected := (0.4 + 0.6 + 0.8 + 0.2) / 4.0

	if avg != expected {
		t.Errorf("GetAverageFitness() = %f, want %f", avg, expected)
	}

	// Test empty population
	emptyPop := Population{}
	emptyAvg := emptyPop.GetAverageFitness()
	if emptyAvg != 0.0 {
		t.Errorf("GetAverageFitness() on empty population should return 0.0")
	}
}

// TestMetricsComputeFitness tests fitness computation from metrics
func TestMetricsComputeFitness(t *testing.T) {
	tests := []struct {
		name    string
		metrics Metrics
		weights [4]float64
		want    float64
	}{
		{
			name: "High SLA, High Util",
			metrics: Metrics{
				SLASuccess:   0.9,
				Utilization:  0.8,
				EnergyNorm:   0.3,
				OverloadNorm: 0.1,
			},
			weights: [4]float64{0.4, 0.3, 0.2, 0.1},
			want:    0.4*0.9 + 0.3*0.8 - 0.2*0.3 - 0.1*0.1, // = 0.36 + 0.24 - 0.06 - 0.01 = 0.53
		},
		{
			name: "Low SLA, High Energy",
			metrics: Metrics{
				SLASuccess:   0.5,
				Utilization:  0.6,
				EnergyNorm:   0.8,
				OverloadNorm: 0.4,
			},
			weights: [4]float64{0.4, 0.3, 0.2, 0.1},
			want:    0.4*0.5 + 0.3*0.6 - 0.2*0.8 - 0.1*0.4, // = 0.20 + 0.18 - 0.16 - 0.04 = 0.18
		},
		{
			name: "Perfect metrics",
			metrics: Metrics{
				SLASuccess:   1.0,
				Utilization:  1.0,
				EnergyNorm:   0.0,
				OverloadNorm: 0.0,
			},
			weights: [4]float64{0.4, 0.3, 0.2, 0.1},
			want:    0.4*1.0 + 0.3*1.0 - 0.2*0.0 - 0.1*0.0, // = 0.7
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metrics.ComputeFitness(tt.weights)
			if abs(got-tt.want) > 0.001 {
				t.Errorf("ComputeFitness() = %f, want %f", got, tt.want)
			}
		})
	}
}

// TestMetricsIsValid tests validation of metrics
func TestMetricsIsValid(t *testing.T) {
	tests := []struct {
		name      string
		metrics   Metrics
		wantValid bool
	}{
		{
			name: "Valid metrics",
			metrics: Metrics{
				SLASuccess:   0.9,
				Utilization:  0.8,
				EnergyNorm:   0.3,
				OverloadNorm: 0.1,
			},
			wantValid: true,
		},
		{
			name: "Invalid SLA (negative)",
			metrics: Metrics{
				SLASuccess:   -0.1,
				Utilization:  0.8,
				EnergyNorm:   0.3,
				OverloadNorm: 0.1,
			},
			wantValid: false,
		},
		{
			name: "Invalid SLA (> 1)",
			metrics: Metrics{
				SLASuccess:   1.5,
				Utilization:  0.8,
				EnergyNorm:   0.3,
				OverloadNorm: 0.1,
			},
			wantValid: false,
		},
		{
			name: "Invalid Utilization (> 1)",
			metrics: Metrics{
				SLASuccess:   0.9,
				Utilization:  1.2,
				EnergyNorm:   0.3,
				OverloadNorm: 0.1,
			},
			wantValid: false,
		},
		{
			name: "Edge case: all zeros",
			metrics: Metrics{
				SLASuccess:   0.0,
				Utilization:  0.0,
				EnergyNorm:   0.0,
				OverloadNorm: 0.0,
			},
			wantValid: true,
		},
		{
			name: "Edge case: all ones",
			metrics: Metrics{
				SLASuccess:   1.0,
				Utilization:  1.0,
				EnergyNorm:   1.0,
				OverloadNorm: 1.0,
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metrics.IsValid()
			if got != tt.wantValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

// TestGetDefaultGAConfig tests default configuration values
func TestGetDefaultGAConfig(t *testing.T) {
	config := GetDefaultGAConfig()

	if config.PopulationSize != 20 {
		t.Errorf("Default PopulationSize = %d, want 20", config.PopulationSize)
	}

	if config.Generations != 10 {
		t.Errorf("Default Generations = %d, want 10", config.Generations)
	}

	if config.MutationRate != 0.1 {
		t.Errorf("Default MutationRate = %f, want 0.1", config.MutationRate)
	}

	if config.CrossoverRate != 0.7 {
		t.Errorf("Default CrossoverRate = %f, want 0.7", config.CrossoverRate)
	}

	if config.ElitismCount != 2 {
		t.Errorf("Default ElitismCount = %d, want 2", config.ElitismCount)
	}

	if config.TournamentSize != 3 {
		t.Errorf("Default TournamentSize = %d, want 3", config.TournamentSize)
	}

	// Verify fitness weights sum makes sense
	weightSum := config.FitnessWeights[0] + config.FitnessWeights[1] + config.FitnessWeights[2] + config.FitnessWeights[3]
	expectedSum := 1.0
	if abs(weightSum-expectedSum) > 0.001 {
		t.Errorf("Default FitnessWeights sum = %f, want %f", weightSum, expectedSum)
	}

	// Verify SLA weight is highest (most important)
	if config.FitnessWeights[0] < config.FitnessWeights[1] ||
		config.FitnessWeights[0] < config.FitnessWeights[2] ||
		config.FitnessWeights[0] < config.FitnessWeights[3] {
		t.Errorf("SLA weight should be highest, got weights: %v", config.FitnessWeights)
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
