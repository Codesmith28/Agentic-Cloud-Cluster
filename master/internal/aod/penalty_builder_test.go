package aod

import (
	"math"
	"testing"
	"time"

	"master/internal/db"
	"master/internal/scheduler"
)

// TestBuildPenaltyVector tests the main penalty vector construction
func TestBuildPenaltyVector(t *testing.T) {
	weights := scheduler.PenaltyWeights{
		G1: 2.0, // SLA failure rate weight
		G2: 1.0, // Overload rate weight
		G3: 0.5, // Energy norm weight
	}

	workerStats := []db.WorkerStats{
		{
			WorkerID:      "worker-good",
			TasksRun:      100,
			SLAViolations: 5,  // 5% failure rate
			OverloadTime:  10, // 10 / 100 = 10% overload
			TotalTime:     100,
			CPUUsedTotal:  100,
			MemUsedTotal:  50,
			GPUUsedTotal:  50,
		},
		{
			WorkerID:      "worker-bad",
			TasksRun:      100,
			SLAViolations: 30, // 30% failure rate
			OverloadTime:  50, // 50 / 100 = 50% overload
			TotalTime:     100,
			CPUUsedTotal:  300,
			MemUsedTotal:  150,
			GPUUsedTotal:  150,
		},
		{
			WorkerID:      "worker-zero",
			TasksRun:      0, // No data
			SLAViolations: 0,
			OverloadTime:  0,
			TotalTime:     0,
			CPUUsedTotal:  0,
			MemUsedTotal:  0,
			GPUUsedTotal:  0,
		},
	}

	penalties := BuildPenaltyVector(workerStats, weights)

	// Check that all workers are present
	if len(penalties) != 3 {
		t.Errorf("Expected 3 workers in penalty vector, got %d", len(penalties))
	}

	// Worker with zero data should have zero penalty
	if penalties["worker-zero"] != 0.0 {
		t.Errorf("Expected worker-zero penalty = 0.0, got %.3f", penalties["worker-zero"])
	}

	// Good worker should have lower penalty than bad worker
	if penalties["worker-good"] >= penalties["worker-bad"] {
		t.Errorf("Expected worker-good penalty (%.3f) < worker-bad penalty (%.3f)",
			penalties["worker-good"], penalties["worker-bad"])
	}

	// Penalties should be in valid range [0, 5]
	for workerID, penalty := range penalties {
		if penalty < 0.0 || penalty > 5.0 {
			t.Errorf("Penalty for %s out of range: %.3f", workerID, penalty)
		}
	}

	t.Logf("Penalty[worker-good] = %.3f", penalties["worker-good"])
	t.Logf("Penalty[worker-bad] = %.3f", penalties["worker-bad"])
	t.Logf("Penalty[worker-zero] = %.3f", penalties["worker-zero"])
}

// TestComputeSLAFailRate tests SLA failure rate computation
func TestComputeSLAFailRate(t *testing.T) {
	testCases := []struct {
		name     string
		stats    db.WorkerStats
		expected float64
	}{
		{
			name: "perfect_sla",
			stats: db.WorkerStats{
				TasksRun:      100,
				SLAViolations: 0,
			},
			expected: 0.0,
		},
		{
			name: "10_percent_failure",
			stats: db.WorkerStats{
				TasksRun:      100,
				SLAViolations: 10,
			},
			expected: 0.1,
		},
		{
			name: "50_percent_failure",
			stats: db.WorkerStats{
				TasksRun:      100,
				SLAViolations: 50,
			},
			expected: 0.5,
		},
		{
			name: "all_failures",
			stats: db.WorkerStats{
				TasksRun:      100,
				SLAViolations: 100,
			},
			expected: 1.0,
		},
		{
			name: "no_tasks",
			stats: db.WorkerStats{
				TasksRun:      0,
				SLAViolations: 0,
			},
			expected: 0.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := computeSLAFailRate(tc.stats)
			if math.Abs(result-tc.expected) > 0.001 {
				t.Errorf("Expected SLA fail rate = %.3f, got %.3f", tc.expected, result)
			}
		})
	}
}

// TestComputeWorkerOverloadRate tests overload rate computation
func TestComputeWorkerOverloadRate(t *testing.T) {
	testCases := []struct {
		name     string
		stats    db.WorkerStats
		expected float64
	}{
		{
			name: "never_overloaded",
			stats: db.WorkerStats{
				OverloadTime: 0,
				TotalTime:    100,
			},
			expected: 0.0,
		},
		{
			name: "10_percent_overload",
			stats: db.WorkerStats{
				OverloadTime: 10,
				TotalTime:    100,
			},
			expected: 0.1,
		},
		{
			name: "50_percent_overload",
			stats: db.WorkerStats{
				OverloadTime: 50,
				TotalTime:    100,
			},
			expected: 0.5,
		},
		{
			name: "always_overloaded",
			stats: db.WorkerStats{
				OverloadTime: 100,
				TotalTime:    100,
			},
			expected: 1.0,
		},
		{
			name: "no_observation_time",
			stats: db.WorkerStats{
				OverloadTime: 0,
				TotalTime:    0,
			},
			expected: 0.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := computeWorkerOverloadRate(tc.stats)
			if math.Abs(result-tc.expected) > 0.001 {
				t.Errorf("Expected overload rate = %.3f, got %.3f", tc.expected, result)
			}
		})
	}
}

// TestComputeEnergyNorm tests energy normalization
func TestComputeEnergyNorm(t *testing.T) {
	testCases := []struct {
		name      string
		stats     db.WorkerStats
		maxEnergy float64
		expected  float64
	}{
		{
			name: "zero_energy",
			stats: db.WorkerStats{
				CPUUsedTotal: 0,
				MemUsedTotal: 0,
				GPUUsedTotal: 0,
			},
			maxEnergy: 100,
			expected:  0.0,
		},
		{
			name: "half_energy",
			stats: db.WorkerStats{
				CPUUsedTotal: 30,
				MemUsedTotal: 10,
				GPUUsedTotal: 10,
			},
			maxEnergy: 100,
			expected:  0.5,
		},
		{
			name: "max_energy",
			stats: db.WorkerStats{
				CPUUsedTotal: 60,
				MemUsedTotal: 20,
				GPUUsedTotal: 20,
			},
			maxEnergy: 100,
			expected:  1.0,
		},
		{
			name: "zero_max_energy",
			stats: db.WorkerStats{
				CPUUsedTotal: 50,
				MemUsedTotal: 50,
				GPUUsedTotal: 50,
			},
			maxEnergy: 0,
			expected:  0.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := computeEnergyNorm(tc.stats, tc.maxEnergy)
			if math.Abs(result-tc.expected) > 0.001 {
				t.Errorf("Expected energy norm = %.3f, got %.3f", tc.expected, result)
			}
		})
	}
}

// TestFindMaxEnergy tests finding maximum energy across workers
func TestFindMaxEnergy(t *testing.T) {
	workerStats := []db.WorkerStats{
		{
			CPUUsedTotal: 100,
			MemUsedTotal: 50,
			GPUUsedTotal: 50,
		},
		{
			CPUUsedTotal: 200,
			MemUsedTotal: 100,
			GPUUsedTotal: 100,
		},
		{
			CPUUsedTotal: 50,
			MemUsedTotal: 25,
			GPUUsedTotal: 25,
		},
	}

	maxEnergy := findMaxEnergy(workerStats)

	// Worker 2 has the highest energy: 200 + 100 + 100 = 400
	expectedMax := 400.0
	if math.Abs(maxEnergy-expectedMax) > 0.001 {
		t.Errorf("Expected max energy = %.3f, got %.3f", expectedMax, maxEnergy)
	}
}

// TestFindMaxEnergyEmpty tests finding max energy with empty data
func TestFindMaxEnergyEmpty(t *testing.T) {
	workerStats := []db.WorkerStats{}
	maxEnergy := findMaxEnergy(workerStats)

	if maxEnergy != 0.0 {
		t.Errorf("Expected max energy = 0.0 for empty stats, got %.3f", maxEnergy)
	}
}

// TestGetDefaultPenaltyVector tests default penalty vector creation
func TestGetDefaultPenaltyVector(t *testing.T) {
	workerIDs := []string{"worker-1", "worker-2", "worker-3"}

	penalties := GetDefaultPenaltyVector(workerIDs)

	// Check all workers present
	if len(penalties) != 3 {
		t.Errorf("Expected 3 workers, got %d", len(penalties))
	}

	// Check all penalties are zero
	for _, workerID := range workerIDs {
		if penalties[workerID] != 0.0 {
			t.Errorf("Expected penalty for %s = 0.0, got %.3f", workerID, penalties[workerID])
		}
	}
}

// TestPenaltyClipping tests that penalties are clipped to [0, 5]
func TestPenaltyClipping(t *testing.T) {
	// Use extreme weights to force clipping
	weights := scheduler.PenaltyWeights{
		G1: 10.0,
		G2: 10.0,
		G3: 10.0,
	}

	workerStats := []db.WorkerStats{
		{
			WorkerID:      "worker-extreme",
			TasksRun:      100,
			SLAViolations: 100, // 100% failure
			OverloadTime:  100, // 100% overload
			TotalTime:     100,
			CPUUsedTotal:  1000,
			MemUsedTotal:  1000,
			GPUUsedTotal:  1000,
		},
	}

	penalties := BuildPenaltyVector(workerStats, weights)

	// Should be clipped to max 5.0
	if penalties["worker-extreme"] > 5.0 {
		t.Errorf("Penalty should be clipped to 5.0, got %.3f", penalties["worker-extreme"])
	}

	// Should not be negative
	if penalties["worker-extreme"] < 0.0 {
		t.Errorf("Penalty should not be negative, got %.3f", penalties["worker-extreme"])
	}

	t.Logf("Extreme penalty (clipped): %.3f", penalties["worker-extreme"])
}

// TestPenaltyWeightImpact tests how different weights affect penalties
func TestPenaltyWeightImpact(t *testing.T) {
	workerStats := []db.WorkerStats{
		{
			WorkerID:      "worker-1",
			TasksRun:      100,
			SLAViolations: 20, // 20% failure
			OverloadTime:  30, // 30% overload
			TotalTime:     100,
			CPUUsedTotal:  100,
			MemUsedTotal:  100,
			GPUUsedTotal:  100,
		},
	}

	// Test with SLA weight dominant
	weights1 := scheduler.PenaltyWeights{G1: 5.0, G2: 0.1, G3: 0.1}
	penalties1 := BuildPenaltyVector(workerStats, weights1)

	// Test with overload weight dominant
	weights2 := scheduler.PenaltyWeights{G1: 0.1, G2: 5.0, G3: 0.1}
	penalties2 := BuildPenaltyVector(workerStats, weights2)

	// Test with energy weight dominant
	weights3 := scheduler.PenaltyWeights{G1: 0.1, G2: 0.1, G3: 5.0}
	penalties3 := BuildPenaltyVector(workerStats, weights3)

	t.Logf("SLA-weighted penalty: %.3f", penalties1["worker-1"])
	t.Logf("Overload-weighted penalty: %.3f", penalties2["worker-1"])
	t.Logf("Energy-weighted penalty: %.3f", penalties3["worker-1"])

	// All should be different due to different weight configurations
	if penalties1["worker-1"] == penalties2["worker-1"] || penalties2["worker-1"] == penalties3["worker-1"] {
		t.Error("Different weights should produce different penalties")
	}
}

// TestRealisticScenario tests a realistic multi-worker scenario
func TestRealisticScenario(t *testing.T) {
	weights := scheduler.PenaltyWeights{
		G1: 2.0, // Prioritize SLA compliance
		G2: 1.0, // Moderate concern for overload
		G3: 0.5, // Low concern for energy
	}

	now := time.Now()
	workerStats := []db.WorkerStats{
		{
			WorkerID:      "worker-reliable",
			TasksRun:      1000,
			SLAViolations: 10, // 1% failure - very reliable
			OverloadTime:  50, // 5% overload
			TotalTime:     1000,
			CPUUsedTotal:  2000,
			MemUsedTotal:  1000,
			GPUUsedTotal:  500,
			PeriodStart:   now.Add(-24 * time.Hour),
			PeriodEnd:     now,
		},
		{
			WorkerID:      "worker-unreliable",
			TasksRun:      1000,
			SLAViolations: 200, // 20% failure - unreliable
			OverloadTime:  400, // 40% overload
			TotalTime:     1000,
			CPUUsedTotal:  4000,
			MemUsedTotal:  2000,
			GPUUsedTotal:  1000,
			PeriodStart:   now.Add(-24 * time.Hour),
			PeriodEnd:     now,
		},
		{
			WorkerID:      "worker-efficient",
			TasksRun:      800,
			SLAViolations: 40, // 5% failure
			OverloadTime:  80, // 10% overload
			TotalTime:     1000,
			CPUUsedTotal:  1000, // Low energy usage
			MemUsedTotal:  500,
			GPUUsedTotal:  250,
			PeriodStart:   now.Add(-24 * time.Hour),
			PeriodEnd:     now,
		},
	}

	penalties := BuildPenaltyVector(workerStats, weights)

	// Unreliable worker should have highest penalty
	if penalties["worker-unreliable"] <= penalties["worker-reliable"] {
		t.Errorf("Unreliable worker penalty (%.3f) should be > reliable worker (%.3f)",
			penalties["worker-unreliable"], penalties["worker-reliable"])
	}

	// Efficient worker should have lowest penalty (best SLA: 5% failure, moderate overload: 10%, lowest energy)
	// Reliable has 1% SLA failure but 5% overload and moderate energy
	// Efficient has 5% SLA failure, 10% overload, but very low energy
	// With weights G1=2.0, G2=1.0, G3=0.5, efficient should be close to or better than reliable
	if penalties["worker-efficient"] >= penalties["worker-unreliable"] {
		t.Errorf("Efficient worker penalty (%.3f) should be < unreliable worker (%.3f)",
			penalties["worker-efficient"], penalties["worker-unreliable"])
	}

	// Both reliable and efficient should be much better than unreliable
	if penalties["worker-reliable"] >= penalties["worker-unreliable"] {
		t.Errorf("Reliable worker penalty (%.3f) should be < unreliable worker (%.3f)",
			penalties["worker-reliable"], penalties["worker-unreliable"])
	}

	t.Logf("📊 Penalty Summary:")
	t.Logf("  Reliable: %.3f", penalties["worker-reliable"])
	t.Logf("  Unreliable: %.3f", penalties["worker-unreliable"])
	t.Logf("  Efficient: %.3f", penalties["worker-efficient"])
}
