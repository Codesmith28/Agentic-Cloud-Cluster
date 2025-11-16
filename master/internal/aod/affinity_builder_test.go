package aod

import (
	"math"
	"testing"
	"time"

	"master/internal/db"
	"master/internal/scheduler"
)

// TestBuildAffinityMatrixWithInsufficientData tests affinity computation with sparse data
func TestBuildAffinityMatrixWithInsufficientData(t *testing.T) {
	history := []db.TaskHistory{
		{
			TaskID:        "task-1",
			WorkerID:      "worker-1",
			Type:          "cpu-light",
			ActualRuntime: 10.0,
			SLASuccess:    true,
			LoadAtStart:   0.5,
		},
		// Only 1 record - insufficient for meaningful statistics
	}

	weights := scheduler.AffinityWeights{
		A1: 1.0,
		A2: 2.0,
		A3: 0.5,
	}

	affinity := BuildAffinityMatrix(history, weights)

	// Should have 6 task types (even with sparse data)
	if len(affinity) != 6 {
		t.Errorf("Expected 6 task types, got %d", len(affinity))
	}

	// cpu-light should have worker-1 with neutral affinity (insufficient data)
	if val, ok := affinity["cpu-light"]["worker-1"]; !ok || val != 0.0 {
		t.Errorf("Expected neutral affinity 0.0 for insufficient data, got %.3f", val)
	}
}

// TestBuildAffinityMatrixWithGoodWorker tests affinity for a fast, reliable worker
func TestBuildAffinityMatrixWithGoodWorker(t *testing.T) {
	now := time.Now()

	// Create history where worker-1 is consistently fast and reliable
	history := []db.TaskHistory{
		// worker-1: fast (8s avg) and reliable (100% SLA)
		{
			TaskID:        "task-1",
			WorkerID:      "worker-1",
			Type:          "cpu-light",
			ArrivalTime:   now,
			ActualRuntime: 8.0,
			SLASuccess:    true,
			LoadAtStart:   0.4,
		},
		{
			TaskID:        "task-2",
			WorkerID:      "worker-1",
			Type:          "cpu-light",
			ArrivalTime:   now.Add(time.Minute),
			ActualRuntime: 7.0,
			SLASuccess:    true,
			LoadAtStart:   0.3,
		},
		{
			TaskID:        "task-3",
			WorkerID:      "worker-1",
			Type:          "cpu-light",
			ArrivalTime:   now.Add(2 * time.Minute),
			ActualRuntime: 9.0,
			SLASuccess:    true,
			LoadAtStart:   0.5,
		},
		// worker-2: slower (12s avg) and less reliable (67% SLA)
		{
			TaskID:        "task-4",
			WorkerID:      "worker-2",
			Type:          "cpu-light",
			ArrivalTime:   now,
			ActualRuntime: 12.0,
			SLASuccess:    true,
			LoadAtStart:   0.6,
		},
		{
			TaskID:        "task-5",
			WorkerID:      "worker-2",
			Type:          "cpu-light",
			ArrivalTime:   now.Add(time.Minute),
			ActualRuntime: 13.0,
			SLASuccess:    false, // SLA violation
			LoadAtStart:   0.7,
		},
		{
			TaskID:        "task-6",
			WorkerID:      "worker-2",
			Type:          "cpu-light",
			ArrivalTime:   now.Add(2 * time.Minute),
			ActualRuntime: 11.0,
			SLASuccess:    true,
			LoadAtStart:   0.8,
		},
	}

	weights := scheduler.AffinityWeights{
		A1: 1.0, // Speed weight
		A2: 2.0, // SLA reliability weight
		A3: 0.5, // Overload penalty
	}

	affinity := BuildAffinityMatrix(history, weights)

	// Check worker-1 affinity (should be positive - fast and reliable)
	w1Affinity := affinity["cpu-light"]["worker-1"]
	if w1Affinity <= 0 {
		t.Errorf("Expected positive affinity for fast/reliable worker-1, got %.3f", w1Affinity)
	}

	// Check worker-2 affinity (should be lower - slower and less reliable)
	w2Affinity := affinity["cpu-light"]["worker-2"]
	if w2Affinity >= w1Affinity {
		t.Errorf("Expected worker-1 affinity (%.3f) > worker-2 affinity (%.3f)", w1Affinity, w2Affinity)
	}

	t.Logf("✓ Worker-1 affinity: %.3f (fast, reliable)", w1Affinity)
	t.Logf("✓ Worker-2 affinity: %.3f (slower, less reliable)", w2Affinity)
}

// TestBuildAffinityMatrixAllTaskTypes tests that affinity matrix includes all 6 task types
func TestBuildAffinityMatrixAllTaskTypes(t *testing.T) {
	now := time.Now()

	// Create diverse history with all task types
	history := []db.TaskHistory{
		{TaskID: "t1", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.0, SLASuccess: true, LoadAtStart: 0.3},
		{TaskID: "t2", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 6.0, SLASuccess: true, LoadAtStart: 0.4},
		{TaskID: "t3", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.5, SLASuccess: true, LoadAtStart: 0.35},

		{TaskID: "t4", WorkerID: "w1", Type: "cpu-heavy", ArrivalTime: now, ActualRuntime: 15.0, SLASuccess: true, LoadAtStart: 0.6},
		{TaskID: "t5", WorkerID: "w1", Type: "cpu-heavy", ArrivalTime: now, ActualRuntime: 16.0, SLASuccess: true, LoadAtStart: 0.65},
		{TaskID: "t6", WorkerID: "w1", Type: "cpu-heavy", ArrivalTime: now, ActualRuntime: 14.0, SLASuccess: true, LoadAtStart: 0.55},

		{TaskID: "t7", WorkerID: "w2", Type: "memory-heavy", ArrivalTime: now, ActualRuntime: 20.0, SLASuccess: true, LoadAtStart: 0.5},
		{TaskID: "t8", WorkerID: "w2", Type: "memory-heavy", ArrivalTime: now, ActualRuntime: 22.0, SLASuccess: true, LoadAtStart: 0.55},
		{TaskID: "t9", WorkerID: "w2", Type: "memory-heavy", ArrivalTime: now, ActualRuntime: 21.0, SLASuccess: true, LoadAtStart: 0.52},

		{TaskID: "t10", WorkerID: "w2", Type: "gpu-inference", ArrivalTime: now, ActualRuntime: 10.0, SLASuccess: true, LoadAtStart: 0.4},
		{TaskID: "t11", WorkerID: "w2", Type: "gpu-inference", ArrivalTime: now, ActualRuntime: 11.0, SLASuccess: true, LoadAtStart: 0.45},
		{TaskID: "t12", WorkerID: "w2", Type: "gpu-inference", ArrivalTime: now, ActualRuntime: 9.0, SLASuccess: true, LoadAtStart: 0.35},

		{TaskID: "t13", WorkerID: "w3", Type: "gpu-training", ArrivalTime: now, ActualRuntime: 60.0, SLASuccess: true, LoadAtStart: 0.7},
		{TaskID: "t14", WorkerID: "w3", Type: "gpu-training", ArrivalTime: now, ActualRuntime: 65.0, SLASuccess: true, LoadAtStart: 0.75},
		{TaskID: "t15", WorkerID: "w3", Type: "gpu-training", ArrivalTime: now, ActualRuntime: 58.0, SLASuccess: true, LoadAtStart: 0.68},

		{TaskID: "t16", WorkerID: "w3", Type: "mixed", ArrivalTime: now, ActualRuntime: 12.0, SLASuccess: true, LoadAtStart: 0.5},
		{TaskID: "t17", WorkerID: "w3", Type: "mixed", ArrivalTime: now, ActualRuntime: 13.0, SLASuccess: true, LoadAtStart: 0.55},
		{TaskID: "t18", WorkerID: "w3", Type: "mixed", ArrivalTime: now, ActualRuntime: 11.0, SLASuccess: true, LoadAtStart: 0.45},
	}

	weights := scheduler.AffinityWeights{A1: 1.0, A2: 2.0, A3: 0.5}
	affinity := BuildAffinityMatrix(history, weights)

	// Verify all 6 task types are present
	expectedTypes := []string{"cpu-light", "cpu-heavy", "memory-heavy", "gpu-inference", "gpu-training", "mixed"}
	for _, taskType := range expectedTypes {
		if _, ok := affinity[taskType]; !ok {
			t.Errorf("Missing task type: %s", taskType)
		}
	}

	// Verify each task type has appropriate workers with positive affinity
	if affinity["cpu-light"]["w1"] <= 0 {
		t.Errorf("Expected positive affinity for cpu-light on w1")
	}
	if affinity["memory-heavy"]["w2"] <= 0 {
		t.Errorf("Expected positive affinity for memory-heavy on w2")
	}
	if affinity["gpu-training"]["w3"] <= 0 {
		t.Errorf("Expected positive affinity for gpu-training on w3")
	}

	t.Logf("✓ All 6 task types present in affinity matrix")
}

// TestComputeSpeed tests speed ratio calculation
func TestComputeSpeed(t *testing.T) {
	now := time.Now()

	history := []db.TaskHistory{
		// Baseline: avg runtime = (8+12)/2 = 10.0
		{TaskID: "t1", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 8.0, SLASuccess: true, LoadAtStart: 0.3},
		{TaskID: "t2", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 8.0, SLASuccess: true, LoadAtStart: 0.3},
		{TaskID: "t3", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 8.0, SLASuccess: true, LoadAtStart: 0.3},
		{TaskID: "t4", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 12.0, SLASuccess: true, LoadAtStart: 0.6},
		{TaskID: "t5", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 12.0, SLASuccess: true, LoadAtStart: 0.6},
		{TaskID: "t6", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 12.0, SLASuccess: true, LoadAtStart: 0.6},
	}

	// worker-1: avg runtime = 8.0, speed = 10.0/8.0 = 1.25 (faster than baseline)
	speed1 := computeSpeed("cpu-light", "w1", history)
	if math.Abs(speed1-1.25) > 0.01 {
		t.Errorf("Expected speed ≈ 1.25 for fast worker, got %.3f", speed1)
	}

	// worker-2: avg runtime = 12.0, speed = 10.0/12.0 = 0.833 (slower than baseline)
	speed2 := computeSpeed("cpu-light", "w2", history)
	if math.Abs(speed2-0.833) > 0.01 {
		t.Errorf("Expected speed ≈ 0.833 for slow worker, got %.3f", speed2)
	}

	// worker-3: no data, speed = 0.0
	speed3 := computeSpeed("cpu-light", "w3", history)
	if speed3 != 0.0 {
		t.Errorf("Expected speed = 0.0 for worker with no data, got %.3f", speed3)
	}

	t.Logf("✓ Speed ratios: w1=%.3f (fast), w2=%.3f (slow), w3=%.3f (no data)", speed1, speed2, speed3)
}

// TestComputeSLAReliability tests SLA success rate calculation
func TestComputeSLAReliability(t *testing.T) {
	now := time.Now()

	// Perfect reliability (100% SLA success)
	perfectHistory := []db.TaskHistory{
		{TaskID: "t1", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.0, SLASuccess: true, LoadAtStart: 0.3},
		{TaskID: "t2", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 6.0, SLASuccess: true, LoadAtStart: 0.4},
		{TaskID: "t3", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.5, SLASuccess: true, LoadAtStart: 0.35},
	}

	reliability := computeSLAReliability("cpu-light", "w1", perfectHistory)
	if reliability != 1.0 {
		t.Errorf("Expected reliability = 1.0 for perfect SLA, got %.3f", reliability)
	}

	// Poor reliability (33% SLA success = 67% violations)
	poorHistory := []db.TaskHistory{
		{TaskID: "t1", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.0, SLASuccess: true, LoadAtStart: 0.7},
		{TaskID: "t2", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 15.0, SLASuccess: false, LoadAtStart: 0.8},
		{TaskID: "t3", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 20.0, SLASuccess: false, LoadAtStart: 0.9},
	}

	reliability2 := computeSLAReliability("cpu-light", "w2", poorHistory)
	expected := 1.0 - (2.0 / 3.0) // 1 success, 2 failures
	if math.Abs(reliability2-expected) > 0.01 {
		t.Errorf("Expected reliability ≈ %.3f for poor SLA, got %.3f", expected, reliability2)
	}

	// Empty history
	reliability3 := computeSLAReliability("cpu-light", "w3", []db.TaskHistory{})
	if reliability3 != 0.0 {
		t.Errorf("Expected reliability = 0.0 for empty history, got %.3f", reliability3)
	}

	t.Logf("✓ SLA reliability: perfect=%.3f, poor=%.3f, empty=%.3f", reliability, reliability2, reliability3)
}

// TestComputeOverloadRate tests load calculation
func TestComputeOverloadRate(t *testing.T) {
	now := time.Now()

	// Low load (avg 0.3)
	lowLoadHistory := []db.TaskHistory{
		{TaskID: "t1", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.0, SLASuccess: true, LoadAtStart: 0.2},
		{TaskID: "t2", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 6.0, SLASuccess: true, LoadAtStart: 0.3},
		{TaskID: "t3", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.5, SLASuccess: true, LoadAtStart: 0.4},
	}

	overload := computeOverloadRate("cpu-light", "w1", lowLoadHistory)
	expected := 0.3
	if math.Abs(overload-expected) > 0.01 {
		t.Errorf("Expected overload rate ≈ %.3f, got %.3f", expected, overload)
	}

	// High load (avg 0.8)
	highLoadHistory := []db.TaskHistory{
		{TaskID: "t4", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.0, SLASuccess: true, LoadAtStart: 0.7},
		{TaskID: "t5", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 6.0, SLASuccess: true, LoadAtStart: 0.8},
		{TaskID: "t6", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.5, SLASuccess: true, LoadAtStart: 0.9},
	}

	overload2 := computeOverloadRate("cpu-light", "w2", highLoadHistory)
	expected2 := 0.8
	if math.Abs(overload2-expected2) > 0.01 {
		t.Errorf("Expected overload rate ≈ %.3f, got %.3f", expected2, overload2)
	}

	t.Logf("✓ Overload rates: low=%.3f, high=%.3f", overload, overload2)
}

// TestAffinityClipping tests that affinity values are clipped to [-5, +5]
func TestAffinityClipping(t *testing.T) {
	now := time.Now()

	// Create history that would produce extreme affinity values
	// Super fast worker (2x baseline speed) with perfect SLA and low load
	history := []db.TaskHistory{
		// Baseline workers (avg runtime = 10.0)
		{TaskID: "t1", WorkerID: "w-baseline", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 10.0, SLASuccess: true, LoadAtStart: 0.5},
		{TaskID: "t2", WorkerID: "w-baseline", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 10.0, SLASuccess: true, LoadAtStart: 0.5},
		{TaskID: "t3", WorkerID: "w-baseline", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 10.0, SLASuccess: true, LoadAtStart: 0.5},

		// Super fast worker (avg runtime = 2.0)
		{TaskID: "t4", WorkerID: "w-fast", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 2.0, SLASuccess: true, LoadAtStart: 0.1},
		{TaskID: "t5", WorkerID: "w-fast", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 2.0, SLASuccess: true, LoadAtStart: 0.1},
		{TaskID: "t6", WorkerID: "w-fast", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 2.0, SLASuccess: true, LoadAtStart: 0.1},
	}

	// Use large weights to create extreme affinity
	weights := scheduler.AffinityWeights{
		A1: 10.0, // Large speed weight
		A2: 10.0, // Large SLA weight
		A3: 1.0,  // Small overload penalty
	}

	affinity := BuildAffinityMatrix(history, weights)

	// Check that affinity is clipped to max +5.0
	fastAffinity := affinity["cpu-light"]["w-fast"]
	if fastAffinity > 5.0 {
		t.Errorf("Expected affinity clipped to ≤ 5.0, got %.3f", fastAffinity)
	}
	if fastAffinity < -5.0 {
		t.Errorf("Expected affinity clipped to ≥ -5.0, got %.3f", fastAffinity)
	}

	t.Logf("✓ Affinity properly clipped: %.3f (within [-5, +5])", fastAffinity)
}

// TestFilterHistory tests history filtering functions
func TestFilterHistory(t *testing.T) {
	now := time.Now()

	history := []db.TaskHistory{
		{TaskID: "t1", WorkerID: "w1", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 5.0, SLASuccess: true, LoadAtStart: 0.3},
		{TaskID: "t2", WorkerID: "w1", Type: "cpu-heavy", ArrivalTime: now, ActualRuntime: 15.0, SLASuccess: true, LoadAtStart: 0.6},
		{TaskID: "t3", WorkerID: "w2", Type: "cpu-light", ArrivalTime: now, ActualRuntime: 6.0, SLASuccess: true, LoadAtStart: 0.4},
		{TaskID: "t4", WorkerID: "w2", Type: "memory-heavy", ArrivalTime: now, ActualRuntime: 20.0, SLASuccess: true, LoadAtStart: 0.5},
	}

	// Test filterHistory (taskType + workerID)
	filtered := filterHistory(history, "cpu-light", "w1")
	if len(filtered) != 1 || filtered[0].TaskID != "t1" {
		t.Errorf("Expected 1 record for (cpu-light, w1), got %d", len(filtered))
	}

	// Test filterHistoryByType (taskType only)
	filtered2 := filterHistoryByType(history, "cpu-light")
	if len(filtered2) != 2 {
		t.Errorf("Expected 2 records for cpu-light, got %d", len(filtered2))
	}

	// Test getUniqueWorkers
	workers := getUniqueWorkers(history)
	if len(workers) != 2 {
		t.Errorf("Expected 2 unique workers, got %d", len(workers))
	}

	t.Logf("✓ Filter functions working correctly")
}
