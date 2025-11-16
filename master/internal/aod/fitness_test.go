package aod

import (
	"fmt"
	"math"
	"testing"
	"time"

	"master/internal/db"
	"master/internal/scheduler"
)

// TestComputeSLASuccess tests SLA success rate computation
func TestComputeSLASuccess(t *testing.T) {
	testCases := []struct {
		name     string
		history  []db.TaskHistory
		expected float64
	}{
		{
			name:     "empty_history",
			history:  []db.TaskHistory{},
			expected: 0.0,
		},
		{
			name: "all_success",
			history: []db.TaskHistory{
				{TaskID: "t1", SLASuccess: true},
				{TaskID: "t2", SLASuccess: true},
				{TaskID: "t3", SLASuccess: true},
			},
			expected: 1.0,
		},
		{
			name: "all_failures",
			history: []db.TaskHistory{
				{TaskID: "t1", SLASuccess: false},
				{TaskID: "t2", SLASuccess: false},
				{TaskID: "t3", SLASuccess: false},
			},
			expected: 0.0,
		},
		{
			name: "50_percent",
			history: []db.TaskHistory{
				{TaskID: "t1", SLASuccess: true},
				{TaskID: "t2", SLASuccess: false},
				{TaskID: "t3", SLASuccess: true},
				{TaskID: "t4", SLASuccess: false},
			},
			expected: 0.5,
		},
		{
			name: "75_percent",
			history: []db.TaskHistory{
				{TaskID: "t1", SLASuccess: true},
				{TaskID: "t2", SLASuccess: true},
				{TaskID: "t3", SLASuccess: true},
				{TaskID: "t4", SLASuccess: false},
			},
			expected: 0.75,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := computeSLASuccess(tc.history)
			if math.Abs(result-tc.expected) > 0.001 {
				t.Errorf("Expected SLA success = %.3f, got %.3f", tc.expected, result)
			}
		})
	}
}

// TestComputeUtilization tests resource utilization computation
func TestComputeUtilization(t *testing.T) {
	testCases := []struct {
		name     string
		stats    []db.WorkerStats
		expected float64
	}{
		{
			name:     "empty_stats",
			stats:    []db.WorkerStats{},
			expected: 0.0,
		},
		{
			name: "zero_utilization",
			stats: []db.WorkerStats{
				{
					WorkerID:     "w1",
					CPUUsedTotal: 0,
					MemUsedTotal: 0,
					GPUUsedTotal: 0,
					TotalTime:    100,
				},
			},
			expected: 0.0,
		},
		{
			name: "full_utilization",
			stats: []db.WorkerStats{
				{
					WorkerID:     "w1",
					CPUUsedTotal: 300, // 3 resources × 100 time units
					MemUsedTotal: 300,
					GPUUsedTotal: 300,
					TotalTime:    100,
				},
			},
			expected: 3.0, // (300+300+300) / (3 × 100) = 3.0 (oversubscribed)
		},
		{
			name: "50_percent_utilization",
			stats: []db.WorkerStats{
				{
					WorkerID:     "w1",
					CPUUsedTotal: 150, // 50% of full utilization
					MemUsedTotal: 150,
					GPUUsedTotal: 150,
					TotalTime:    100,
				},
			},
			expected: 1.5, // (150+150+150) / (3 × 100) = 1.5
		},
		{
			name: "multiple_workers_average",
			stats: []db.WorkerStats{
				{
					WorkerID:     "w1",
					CPUUsedTotal: 300,
					MemUsedTotal: 300,
					GPUUsedTotal: 300,
					TotalTime:    100,
				},
				{
					WorkerID:     "w2",
					CPUUsedTotal: 0,
					MemUsedTotal: 0,
					GPUUsedTotal: 0,
					TotalTime:    100,
				},
			},
			expected: 1.5, // (3.0 + 0.0) / 2
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := computeUtilization(tc.stats)
			if math.Abs(result-tc.expected) > 0.001 {
				t.Errorf("Expected utilization = %.3f, got %.3f", tc.expected, result)
			}
		})
	}
}

// TestComputeEnergyNormTotal tests energy normalization
func TestComputeEnergyNormTotal(t *testing.T) {
	testCases := []struct {
		name     string
		stats    []db.WorkerStats
		expected float64
	}{
		{
			name:     "empty_stats",
			stats:    []db.WorkerStats{},
			expected: 0.0,
		},
		{
			name: "zero_energy",
			stats: []db.WorkerStats{
				{
					CPUUsedTotal: 0,
					MemUsedTotal: 0,
					GPUUsedTotal: 0,
					TotalTime:    100,
				},
			},
			expected: 0.0,
		},
		{
			name: "full_capacity",
			stats: []db.WorkerStats{
				{
					CPUUsedTotal: 100,
					MemUsedTotal: 100,
					GPUUsedTotal: 100,
					TotalTime:    100,
				},
			},
			expected: 1.0, // (100+100+100) / (3 × 100) = 1.0
		},
		{
			name: "50_percent_energy",
			stats: []db.WorkerStats{
				{
					CPUUsedTotal: 50,
					MemUsedTotal: 50,
					GPUUsedTotal: 50,
					TotalTime:    100,
				},
			},
			expected: 0.5, // (50+50+50) / (3 × 100) = 0.5
		},
		{
			name: "oversubscribed",
			stats: []db.WorkerStats{
				{
					CPUUsedTotal: 200,
					MemUsedTotal: 200,
					GPUUsedTotal: 200,
					TotalTime:    100,
				},
			},
			expected: 2.0, // (200+200+200) / (3 × 100) = 2.0 (clamped)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := computeEnergyNormTotal(tc.stats)
			if math.Abs(result-tc.expected) > 0.001 {
				t.Errorf("Expected energy norm = %.3f, got %.3f", tc.expected, result)
			}
		})
	}
}

// TestComputeOverloadNormTotal tests overload normalization
func TestComputeOverloadNormTotal(t *testing.T) {
	testCases := []struct {
		name     string
		stats    []db.WorkerStats
		expected float64
	}{
		{
			name:     "empty_stats",
			stats:    []db.WorkerStats{},
			expected: 0.0,
		},
		{
			name: "no_overload",
			stats: []db.WorkerStats{
				{OverloadTime: 0, TotalTime: 100},
			},
			expected: 0.0,
		},
		{
			name: "full_overload",
			stats: []db.WorkerStats{
				{OverloadTime: 100, TotalTime: 100},
			},
			expected: 1.0,
		},
		{
			name: "50_percent_overload",
			stats: []db.WorkerStats{
				{OverloadTime: 50, TotalTime: 100},
			},
			expected: 0.5,
		},
		{
			name: "multiple_workers",
			stats: []db.WorkerStats{
				{OverloadTime: 30, TotalTime: 100},
				{OverloadTime: 10, TotalTime: 100},
			},
			expected: 0.2, // (30+10) / (100+100) = 0.2
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := computeOverloadNormTotal(tc.stats)
			if math.Abs(result-tc.expected) > 0.001 {
				t.Errorf("Expected overload norm = %.3f, got %.3f", tc.expected, result)
			}
		})
	}
}

// TestComputeMetrics tests the convenience function that returns all metrics
func TestComputeMetrics(t *testing.T) {
	history := []db.TaskHistory{
		{TaskID: "t1", SLASuccess: true},
		{TaskID: "t2", SLASuccess: true},
		{TaskID: "t3", SLASuccess: false},
		{TaskID: "t4", SLASuccess: true},
	}

	stats := []db.WorkerStats{
		{
			WorkerID:     "w1",
			CPUUsedTotal: 150,
			MemUsedTotal: 150,
			GPUUsedTotal: 150,
			OverloadTime: 25,
			TotalTime:    100,
		},
	}

	metrics := ComputeMetrics(history, stats)

	// Check SLA success: 3/4 = 0.75
	if math.Abs(metrics.SLASuccess-0.75) > 0.001 {
		t.Errorf("Expected SLA success = 0.75, got %.3f", metrics.SLASuccess)
	}

	// Check utilization: (150+150+150) / (3 × 100) = 1.5
	if math.Abs(metrics.Utilization-1.5) > 0.001 {
		t.Errorf("Expected utilization = 1.5, got %.3f", metrics.Utilization)
	}

	// Check energy norm: (150+150+150) / (3 × 100) = 1.5
	if math.Abs(metrics.EnergyNorm-1.5) > 0.001 {
		t.Errorf("Expected energy norm = 1.5, got %.3f", metrics.EnergyNorm)
	}

	// Check overload norm: 25 / 100 = 0.25
	if math.Abs(metrics.OverloadNorm-0.25) > 0.001 {
		t.Errorf("Expected overload norm = 0.25, got %.3f", metrics.OverloadNorm)
	}
}

// TestComputeFitness tests the main fitness computation
func TestComputeFitness(t *testing.T) {
	config := GAConfig{
		FitnessWeights: [4]float64{3.0, 1.0, 0.5, 1.5}, // w1, w2, w3, w4
	}

	chromosome := Chromosome{
		Theta: scheduler.Theta{
			Theta1: 0.1,
			Theta2: 0.1,
			Theta3: 0.3,
			Theta4: 0.2,
		},
	}

	history := []db.TaskHistory{
		{TaskID: "t1", SLASuccess: true},
		{TaskID: "t2", SLASuccess: true},
		{TaskID: "t3", SLASuccess: true},
		{TaskID: "t4", SLASuccess: true},
	}

	stats := []db.WorkerStats{
		{
			WorkerID:     "w1",
			CPUUsedTotal: 100,
			MemUsedTotal: 100,
			GPUUsedTotal: 100,
			OverloadTime: 10,
			TotalTime:    100,
		},
	}

	fitness := ComputeFitness(chromosome, history, stats, config)

	// Expected:
	// SLA = 1.0 (all tasks successful)
	// Util = (100+100+100) / (3×100) = 1.0
	// Energy = (100+100+100) / (3×100) = 1.0
	// Overload = 10/100 = 0.1
	// Fitness = 3.0×1.0 + 1.0×1.0 - 0.5×1.0 - 1.5×0.1
	//         = 3.0 + 1.0 - 0.5 - 0.15 = 3.35

	expected := 3.35
	if math.Abs(fitness-expected) > 0.01 {
		t.Errorf("Expected fitness = %.3f, got %.3f", expected, fitness)
	}
}

// TestComputeFitnessWithVariedPerformance tests fitness with different performance levels
func TestComputeFitnessWithVariedPerformance(t *testing.T) {
	config := GAConfig{
		FitnessWeights: [4]float64{3.0, 1.0, 0.5, 1.5},
	}

	chromosome := Chromosome{}

	// Scenario 1: Perfect performance
	history1 := []db.TaskHistory{
		{TaskID: "t1", SLASuccess: true},
		{TaskID: "t2", SLASuccess: true},
	}
	stats1 := []db.WorkerStats{
		{
			CPUUsedTotal: 100,
			MemUsedTotal: 100,
			GPUUsedTotal: 100,
			OverloadTime: 0,
			TotalTime:    100,
		},
	}

	fitness1 := ComputeFitness(chromosome, history1, stats1, config)

	// Scenario 2: Poor performance (low SLA, high overload)
	history2 := []db.TaskHistory{
		{TaskID: "t1", SLASuccess: false},
		{TaskID: "t2", SLASuccess: false},
	}
	stats2 := []db.WorkerStats{
		{
			CPUUsedTotal: 100,
			MemUsedTotal: 100,
			GPUUsedTotal: 100,
			OverloadTime: 50,
			TotalTime:    100,
		},
	}

	fitness2 := ComputeFitness(chromosome, history2, stats2, config)

	// Perfect performance should have higher fitness
	if fitness2 >= fitness1 {
		t.Errorf("Poor performance fitness (%.3f) should be < perfect performance (%.3f)",
			fitness2, fitness1)
	}

	t.Logf("Perfect performance fitness: %.3f", fitness1)
	t.Logf("Poor performance fitness: %.3f", fitness2)
}

// TestEvaluateChromosomeFitness tests the in-place fitness evaluation
func TestEvaluateChromosomeFitness(t *testing.T) {
	config := GAConfig{
		FitnessWeights: [4]float64{3.0, 1.0, 0.5, 1.5},
	}

	chromosome := Chromosome{
		Fitness: 0.0, // Initially zero
	}

	history := []db.TaskHistory{
		{TaskID: "t1", SLASuccess: true},
	}

	stats := []db.WorkerStats{
		{
			CPUUsedTotal: 50,
			MemUsedTotal: 50,
			GPUUsedTotal: 50,
			OverloadTime: 0,
			TotalTime:    100,
		},
	}

	// Fitness should be updated in-place
	EvaluateChromosomeFitness(&chromosome, history, stats, config)

	if chromosome.Fitness == 0.0 {
		t.Error("Chromosome fitness should be updated")
	}

	t.Logf("Chromosome fitness after evaluation: %.3f", chromosome.Fitness)
}

// TestGetDefaultFitnessWeights tests default weight retrieval
func TestGetDefaultFitnessWeights(t *testing.T) {
	weights := GetDefaultFitnessWeights()

	// Check we have 4 weights
	if len(weights) != 4 {
		t.Errorf("Expected 4 weights, got %d", len(weights))
	}

	// Check SLA weight is highest (w1 should be largest)
	if weights[0] <= weights[1] || weights[0] <= weights[2] || weights[0] <= weights[3] {
		t.Error("SLA weight should be highest")
	}

	// Check all weights are positive
	for i, w := range weights {
		if w <= 0 {
			t.Errorf("Weight[%d] should be positive, got %.3f", i, w)
		}
	}

	t.Logf("Default fitness weights: w1=%.1f, w2=%.1f, w3=%.1f, w4=%.1f",
		weights[0], weights[1], weights[2], weights[3])
}

// TestFitnessWithRealisticScenario tests fitness with realistic workload
func TestFitnessWithRealisticScenario(t *testing.T) {
	config := GAConfig{
		FitnessWeights: GetDefaultFitnessWeights(),
	}

	chromosome := Chromosome{}

	now := time.Now()

	// Realistic workload: 100 tasks, 90% SLA success
	history := make([]db.TaskHistory, 100)
	for i := 0; i < 100; i++ {
		history[i] = db.TaskHistory{
			TaskID:      fmt.Sprintf("task-%d", i),
			SLASuccess:  i < 90, // First 90 succeed, last 10 fail
			ArrivalTime: now.Add(-time.Duration(i) * time.Minute),
		}
	}

	// 3 workers with different characteristics
	stats := []db.WorkerStats{
		{
			WorkerID:     "worker-efficient",
			CPUUsedTotal: 1000,
			MemUsedTotal: 800,
			GPUUsedTotal: 500,
			OverloadTime: 50,
			TotalTime:    1000,
		},
		{
			WorkerID:     "worker-busy",
			CPUUsedTotal: 2000,
			MemUsedTotal: 1800,
			GPUUsedTotal: 1500,
			OverloadTime: 200,
			TotalTime:    1000,
		},
		{
			WorkerID:     "worker-idle",
			CPUUsedTotal: 300,
			MemUsedTotal: 200,
			GPUUsedTotal: 100,
			OverloadTime: 0,
			TotalTime:    1000,
		},
	}

	fitness := ComputeFitness(chromosome, history, stats, config)

	// Expected metrics:
	// SLA = 0.9 (90/100)
	// Util = avg of (2300/3000, 5300/3000, 600/3000) = avg(0.767, 1.767, 0.2) ≈ 0.911
	// Energy = (2300+5300+600) / (3×3000) = 8200/9000 ≈ 0.911
	// Overload = (50+200+0) / 3000 ≈ 0.083

	// With default weights [3.0, 1.0, 0.5, 1.5]:
	// Fitness ≈ 3.0×0.9 + 1.0×0.911 - 0.5×0.911 - 1.5×0.083
	//         ≈ 2.7 + 0.911 - 0.456 - 0.125 ≈ 3.03

	if fitness < 2.0 || fitness > 4.0 {
		t.Errorf("Fitness %.3f seems unrealistic for this scenario", fitness)
	}

	metrics := ComputeMetrics(history, stats)
	t.Logf("📊 Realistic Scenario Metrics:")
	t.Logf("  SLA Success: %.1f%%", metrics.SLASuccess*100)
	t.Logf("  Utilization: %.3f", metrics.Utilization)
	t.Logf("  Energy Norm: %.3f", metrics.EnergyNorm)
	t.Logf("  Overload Norm: %.3f", metrics.OverloadNorm)
	t.Logf("  Final Fitness: %.3f", fitness)
}

// TestFitnessWeightImpact tests how different weights affect fitness
func TestFitnessWeightImpact(t *testing.T) {
	history := []db.TaskHistory{
		{TaskID: "t1", SLASuccess: true},
		{TaskID: "t2", SLASuccess: false},
	}

	stats := []db.WorkerStats{
		{
			CPUUsedTotal: 100,
			MemUsedTotal: 100,
			GPUUsedTotal: 100,
			OverloadTime: 20,
			TotalTime:    100,
		},
	}

	chromosome := Chromosome{}

	// Config 1: SLA-focused (high w1)
	config1 := GAConfig{
		FitnessWeights: [4]float64{10.0, 1.0, 0.1, 0.1},
	}
	fitness1 := ComputeFitness(chromosome, history, stats, config1)

	// Config 2: Utilization-focused (high w2)
	config2 := GAConfig{
		FitnessWeights: [4]float64{1.0, 10.0, 0.1, 0.1},
	}
	fitness2 := ComputeFitness(chromosome, history, stats, config2)

	// Config 3: Energy-focused (high w3)
	config3 := GAConfig{
		FitnessWeights: [4]float64{1.0, 1.0, 10.0, 0.1},
	}
	fitness3 := ComputeFitness(chromosome, history, stats, config3)

	t.Logf("SLA-focused fitness: %.3f", fitness1)
	t.Logf("Utilization-focused fitness: %.3f", fitness2)
	t.Logf("Energy-focused fitness: %.3f", fitness3)

	// All should produce different results
	if fitness1 == fitness2 || fitness2 == fitness3 || fitness1 == fitness3 {
		t.Error("Different weight configurations should produce different fitness values")
	}
}
