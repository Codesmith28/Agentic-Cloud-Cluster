package scheduler

import (
	"math"
	"testing"
	"time"

	pb "master/proto"
)

// ========================================
// OPTIMIZATION-FOCUSED TESTS
// These tests verify RTS optimizes scheduling decisions,
// not just assigns tasks to any available worker
// ========================================

// TestRTS_OptimizesDeadlineCompliance verifies RTS prioritizes meeting deadlines
func TestRTS_OptimizesDeadlineCompliance(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPUHeavy, 20.0) // 20 seconds estimated runtime
	telSource := NewMockTelemetrySource()

	// Worker1: Fast but heavily loaded (might miss deadline)
	telSource.AddWorker(WorkerView{
		ID:           "worker-fast-loaded",
		CPUAvail:     16.0,
		MemAvail:     32.0,
		GPUAvail:     4.0,
		StorageAvail: 200.0,
		Load:         0.95, // 95% loaded - high risk of delay
	})

	// Worker2: Moderate speed but lightly loaded (better for deadline)
	telSource.AddWorker(WorkerView{
		ID:           "worker-moderate-free",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.1, // 10% loaded - low risk
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Task with tight deadline (40 seconds = 2.0 * 20.0 tau)
	task := &pb.Task{
		TaskId:     "urgent-task",
		TaskType:   TaskTypeCPUHeavy,
		ReqCpu:     4.0,
		ReqMemory:  8.0,
		ReqGpu:     1.0,
		ReqStorage: 50.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-fast-loaded":   {WorkerID: "worker-fast-loaded", IsActive: true},
		"worker-moderate-free": {WorkerID: "worker-moderate-free", IsActive: true},
	}

	selected := rts.SelectWorker(task, workers)

	// Should prefer the lightly loaded worker to meet deadline
	// Even though the other worker is "faster" in terms of resources
	if selected != "worker-moderate-free" {
		t.Errorf("OPTIMIZATION FAILURE: Expected worker-moderate-free (lower load = better deadline compliance), got %s", selected)
	}
}

// TestRTS_OptimizesResourceUtilization verifies RTS considers resource efficiency
// This test verifies that RTS prefers workers with appropriate resources over oversized ones
// when load balancing is also considered
func TestRTS_OptimizesResourceUtilization(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPULight, 10.0)
	telSource := NewMockTelemetrySource()

	// Worker1: Massive resources but higher load
	telSource.AddWorker(WorkerView{
		ID:           "worker-oversized",
		CPUAvail:     64.0,
		MemAvail:     128.0,
		GPUAvail:     8.0,
		StorageAvail: 1000.0,
		Load:         0.3, // Higher load
	})

	// Worker2: Right-sized resources with lower load
	telSource.AddWorker(WorkerView{
		ID:           "worker-rightsized",
		CPUAvail:     4.0,
		MemAvail:     8.0,
		GPUAvail:     0.0,
		StorageAvail: 50.0,
		Load:         0.1, // Lower load - this makes it the better choice
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Set parameters to consider both contention and load
	params := GetDefaultGAParams()
	params.Theta.Theta1 = 0.2 // CPU contention coefficient
	params.Theta.Theta2 = 0.2 // Memory contention coefficient
	params.Risk.Beta = 2.0    // Load penalty matters more
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	// Light task - doesn't need massive resources
	task := &pb.Task{
		TaskId:     "light-task",
		TaskType:   TaskTypeCPULight,
		ReqCpu:     2.0,
		ReqMemory:  4.0,
		ReqGpu:     0.0,
		ReqStorage: 20.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-oversized":  {WorkerID: "worker-oversized", IsActive: true},
		"worker-rightsized": {WorkerID: "worker-rightsized", IsActive: true},
	}

	selected := rts.SelectWorker(task, workers)

	// Should prefer right-sized worker (better overall: lower load + adequate resources)
	if selected != "worker-rightsized" {
		t.Errorf("OPTIMIZATION FAILURE: Expected worker-rightsized (lower load + adequate resources), got %s", selected)
	}
}

// TestRTS_OptimizesLoadBalancing verifies RTS balances load across workers
func TestRTS_OptimizesLoadBalancing(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPULight, 5.0)
	telSource := NewMockTelemetrySource()

	// Worker1: Heavily loaded
	telSource.AddWorker(WorkerView{
		ID:           "worker-busy",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.85, // 85% busy
	})

	// Worker2: Lightly loaded
	telSource.AddWorker(WorkerView{
		ID:           "worker-idle",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.15, // 15% busy
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Increase Beta to heavily penalize high load
	params := GetDefaultGAParams()
	params.Risk.Beta = 5.0 // Strong load penalty
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	task := &pb.Task{
		TaskId:     "task1",
		TaskType:   TaskTypeCPULight,
		ReqCpu:     2.0,
		ReqMemory:  4.0,
		ReqGpu:     0.0,
		ReqStorage: 20.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-busy": {WorkerID: "worker-busy", IsActive: true},
		"worker-idle": {WorkerID: "worker-idle", IsActive: true},
	}

	selected := rts.SelectWorker(task, workers)

	// Should select less loaded worker for better load balancing
	if selected != "worker-idle" {
		t.Errorf("OPTIMIZATION FAILURE: Expected worker-idle (lower load), got %s", selected)
	}
}

// TestRTS_OptimizesWithAffinityMatrix verifies workload affinity optimization
func TestRTS_OptimizesWithAffinityMatrix(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeGPUInference, 15.0)
	telSource := NewMockTelemetrySource()

	// Worker1: GPU specialist
	telSource.AddWorker(WorkerView{
		ID:           "worker-gpu-specialist",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     4.0,
		StorageAvail: 100.0,
		Load:         0.4,
	})

	// Worker2: CPU specialist (has GPU but not optimized)
	telSource.AddWorker(WorkerView{
		ID:           "worker-cpu-specialist",
		CPUAvail:     16.0,
		MemAvail:     32.0,
		GPUAvail:     2.0,
		StorageAvail: 200.0,
		Load:         0.3, // Lower load but not specialized
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Set affinity matrix to prefer GPU specialist for GPU tasks
	params := GetDefaultGAParams()
	params.AffinityMatrix = map[string]map[string]float64{
		TaskTypeGPUInference: {
			"worker-gpu-specialist": 10.0, // Strong affinity
			"worker-cpu-specialist": -2.0, // Negative affinity
		},
	}
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	task := &pb.Task{
		TaskId:     "inference-task",
		TaskType:   TaskTypeGPUInference,
		ReqCpu:     4.0,
		ReqMemory:  8.0,
		ReqGpu:     1.0,
		ReqStorage: 50.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-gpu-specialist": {WorkerID: "worker-gpu-specialist", IsActive: true},
		"worker-cpu-specialist": {WorkerID: "worker-cpu-specialist", IsActive: true},
	}

	selected := rts.SelectWorker(task, workers)

	// Should select GPU specialist despite slightly higher load
	if selected != "worker-gpu-specialist" {
		t.Errorf("OPTIMIZATION FAILURE: Expected worker-gpu-specialist (high affinity), got %s", selected)
	}
}

// TestRTS_OptimizesWithPenalties verifies penalty-based optimization
func TestRTS_OptimizesWithPenalties(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPULight, 10.0)
	telSource := NewMockTelemetrySource()

	// Worker1: Good specs but unreliable (should have penalty)
	telSource.AddWorker(WorkerView{
		ID:           "worker-unreliable",
		CPUAvail:     16.0,
		MemAvail:     32.0,
		GPUAvail:     4.0,
		StorageAvail: 200.0,
		Load:         0.2,
	})

	// Worker2: Moderate specs but reliable
	telSource.AddWorker(WorkerView{
		ID:           "worker-reliable",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.3,
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Set penalty for unreliable worker
	params := GetDefaultGAParams()
	params.PenaltyVector = map[string]float64{
		"worker-unreliable": 15.0, // High penalty (e.g., from past failures)
		"worker-reliable":   0.0,  // No penalty
	}
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	task := &pb.Task{
		TaskId:     "important-task",
		TaskType:   TaskTypeCPULight,
		ReqCpu:     4.0,
		ReqMemory:  8.0,
		ReqGpu:     0.0,
		ReqStorage: 50.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-unreliable": {WorkerID: "worker-unreliable", IsActive: true},
		"worker-reliable":   {WorkerID: "worker-reliable", IsActive: true},
	}

	selected := rts.SelectWorker(task, workers)

	// Should avoid unreliable worker despite better specs
	if selected != "worker-reliable" {
		t.Errorf("OPTIMIZATION FAILURE: Expected worker-reliable (no penalty), got %s", selected)
	}
}

// TestRTS_OptimizesMultiObjective verifies multi-objective optimization
// Tests that RTS balances multiple factors: deadline, load, affinity, and penalties
func TestRTS_OptimizesMultiObjective(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeGPUTraining, 60.0)
	telSource := NewMockTelemetrySource()

	// Worker1: High load, no affinity, no penalty - POOR CHOICE
	telSource.AddWorker(WorkerView{
		ID:           "worker-poor",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.9, // Very high load
	})

	// Worker2: Low load, high affinity, no penalty - BEST CHOICE
	telSource.AddWorker(WorkerView{
		ID:           "worker-optimal",
		CPUAvail:     16.0,
		MemAvail:     32.0,
		GPUAvail:     4.0,
		StorageAvail: 200.0,
		Load:         0.2, // Low load
	})

	// Worker3: Low load, no affinity, high penalty - MEDIOCRE CHOICE
	telSource.AddWorker(WorkerView{
		ID:           "worker-penalized",
		CPUAvail:     12.0,
		MemAvail:     24.0,
		GPUAvail:     3.0,
		StorageAvail: 150.0,
		Load:         0.25, // Low load but has penalty
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Set up multi-objective parameters
	params := GetDefaultGAParams()
	params.Risk.Alpha = 10.0 // Deadline violations are costly
	params.Risk.Beta = 3.0   // Load matters
	params.AffinityMatrix = map[string]map[string]float64{
		TaskTypeGPUTraining: {
			"worker-optimal":   8.0, // Strong positive affinity
			"worker-poor":      0.0,
			"worker-penalized": 0.0,
		},
	}
	params.PenaltyVector = map[string]float64{
		"worker-optimal":   0.0,
		"worker-poor":      0.0,
		"worker-penalized": 5.0, // Penalty from past issues
	}
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	task := &pb.Task{
		TaskId:     "training-task",
		TaskType:   TaskTypeGPUTraining,
		ReqCpu:     8.0,
		ReqMemory:  16.0,
		ReqGpu:     2.0,
		ReqStorage: 100.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-poor":      {WorkerID: "worker-poor", IsActive: true},
		"worker-optimal":   {WorkerID: "worker-optimal", IsActive: true},
		"worker-penalized": {WorkerID: "worker-penalized", IsActive: true},
	}

	selected := rts.SelectWorker(task, workers)

	// Should select worker-optimal (best overall score across all factors)
	if selected != "worker-optimal" {
		t.Errorf("OPTIMIZATION FAILURE: Expected worker-optimal (best multi-objective score), got %s", selected)
	}
}

// TestRTS_OptimizesDeadlineViolationPrevention verifies tight deadline handling
func TestRTS_OptimizesDeadlineViolationPrevention(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPUHeavy, 30.0) // 30 second estimate
	telSource := NewMockTelemetrySource()

	// Worker1: Resources tight, would likely violate deadline
	telSource.AddWorker(WorkerView{
		ID:           "worker-tight",
		CPUAvail:     4.0, // Just enough CPU
		MemAvail:     8.0, // Just enough memory
		GPUAvail:     1.0,
		StorageAvail: 60.0,
		Load:         0.7, // High load
	})

	// Worker2: Plenty of resources, will comfortably meet deadline
	telSource.AddWorker(WorkerView{
		ID:           "worker-comfortable",
		CPUAvail:     32.0, // Plenty of CPU
		MemAvail:     64.0, // Plenty of memory
		GPUAvail:     8.0,
		StorageAvail: 500.0,
		Load:         0.1, // Low load
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 1.5) // Tight SLA (1.5x)
	defer rts.Shutdown()

	// High Alpha = severe penalty for deadline violations
	params := GetDefaultGAParams()
	params.Risk.Alpha = 20.0 // Very high deadline violation penalty
	params.Risk.Beta = 1.0
	params.Theta.Theta1 = 0.4 // High CPU contention impact
	params.Theta.Theta2 = 0.4 // High Memory contention impact
	params.Theta.Theta4 = 0.3 // High load impact
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	task := &pb.Task{
		TaskId:     "deadline-critical-task",
		TaskType:   TaskTypeCPUHeavy,
		ReqCpu:     4.0,
		ReqMemory:  8.0,
		ReqGpu:     1.0,
		ReqStorage: 50.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-tight":       {WorkerID: "worker-tight", IsActive: true},
		"worker-comfortable": {WorkerID: "worker-comfortable", IsActive: true},
	}

	selected := rts.SelectWorker(task, workers)

	// Should select worker with comfortable resources to avoid deadline violation
	if selected != "worker-comfortable" {
		t.Errorf("OPTIMIZATION FAILURE: Expected worker-comfortable (lower deadline violation risk), got %s", selected)
	}
}

// TestRTS_OptimizesConsistentlyOverMultipleTasks verifies optimization consistency
func TestRTS_OptimizesConsistentlyOverMultipleTasks(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPULight, 5.0)
	telSource := NewMockTelemetrySource()

	// Worker1: Good overall
	telSource.AddWorker(WorkerView{
		ID:           "worker-good",
		CPUAvail:     16.0,
		MemAvail:     32.0,
		GPUAvail:     2.0,
		StorageAvail: 200.0,
		Load:         0.2,
	})

	// Worker2: Poor overall
	telSource.AddWorker(WorkerView{
		ID:           "worker-poor",
		CPUAvail:     4.0,
		MemAvail:     8.0,
		GPUAvail:     0.0,
		StorageAvail: 50.0,
		Load:         0.8,
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	workers := map[string]*WorkerInfo{
		"worker-good": {WorkerID: "worker-good", IsActive: true},
		"worker-poor": {WorkerID: "worker-poor", IsActive: true},
	}

	// Submit 10 similar tasks - should consistently select better worker
	goodCount := 0
	poorCount := 0

	for i := 0; i < 10; i++ {
		task := &pb.Task{
			TaskId:     "task-" + string(rune(i)),
			TaskType:   TaskTypeCPULight,
			ReqCpu:     2.0,
			ReqMemory:  4.0,
			ReqGpu:     0.0,
			ReqStorage: 20.0,
		}

		selected := rts.SelectWorker(task, workers)

		if selected == "worker-good" {
			goodCount++
		} else if selected == "worker-poor" {
			poorCount++
		}
	}

	// Should consistently prefer the better worker (at least 90% of the time)
	if goodCount < 9 {
		t.Errorf("OPTIMIZATION FAILURE: Expected consistent selection of worker-good (>90%%), got %d/10", goodCount)
	}
}

// TestRTS_OptimizesResourceContention verifies contention-aware scheduling
func TestRTS_OptimizesResourceContention(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeMemoryHeavy, 25.0)
	telSource := NewMockTelemetrySource()

	// Worker1: CPU-heavy task, low memory contention
	telSource.AddWorker(WorkerView{
		ID:           "worker-low-mem-contention",
		CPUAvail:     8.0,
		MemAvail:     64.0, // Lots of memory available
		GPUAvail:     2.0,
		StorageAvail: 200.0,
		Load:         0.3,
	})

	// Worker2: High memory contention
	telSource.AddWorker(WorkerView{
		ID:           "worker-high-mem-contention",
		CPUAvail:     16.0, // More CPU
		MemAvail:     16.0, // Limited memory
		GPUAvail:     4.0,
		StorageAvail: 300.0,
		Load:         0.3,
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// High Theta2 = memory contention heavily impacts execution time
	params := GetDefaultGAParams()
	params.Theta.Theta1 = 0.1 // Low CPU impact
	params.Theta.Theta2 = 0.8 // High memory contention impact
	params.Theta.Theta3 = 0.1 // Low GPU impact
	params.Theta.Theta4 = 0.2 // Moderate load impact
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	// Memory-heavy task
	task := &pb.Task{
		TaskId:     "memory-intensive-task",
		TaskType:   TaskTypeMemoryHeavy,
		ReqCpu:     4.0,
		ReqMemory:  12.0, // Significant memory requirement
		ReqGpu:     0.0,
		ReqStorage: 50.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-low-mem-contention":  {WorkerID: "worker-low-mem-contention", IsActive: true},
		"worker-high-mem-contention": {WorkerID: "worker-high-mem-contention", IsActive: true},
	}

	selected := rts.SelectWorker(task, workers)

	// Should prefer worker with lower memory contention
	if selected != "worker-low-mem-contention" {
		t.Errorf("OPTIMIZATION FAILURE: Expected worker-low-mem-contention (lower memory contention), got %s", selected)
	}
}

// TestRTS_OptimizesRiskScoreCalculation verifies risk formula correctness
func TestRTS_OptimizesRiskScoreCalculation(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPULight, 10.0)
	telSource := NewMockTelemetrySource()

	// Add workers with known characteristics
	telSource.AddWorker(WorkerView{
		ID:           "worker-test",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.5,
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Set known parameters
	params := &GAParams{
		Theta: Theta{
			Theta1: 0.1,
			Theta2: 0.1,
			Theta3: 0.1,
			Theta4: 0.2,
		},
		Risk: Risk{
			Alpha: 10.0,
			Beta:  1.0,
		},
		AffinityMatrix: map[string]map[string]float64{
			TaskTypeCPULight: {
				"worker-test": 2.0,
			},
		},
		PenaltyVector: map[string]float64{
			"worker-test": 0.5,
		},
	}
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	// Create task
	task := &pb.Task{
		TaskId:     "test-task",
		TaskType:   TaskTypeCPULight,
		ReqCpu:     2.0,
		ReqMemory:  4.0,
		ReqGpu:     0.0,
		ReqStorage: 20.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-test": {WorkerID: "worker-test", IsActive: true},
	}

	// Build task and worker views
	now := time.Now()
	taskView := rts.buildTaskView(task, now)
	workerViews := rts.buildWorkerViews(workers)

	if len(workerViews) != 1 {
		t.Fatalf("Expected 1 worker view, got %d", len(workerViews))
	}

	workerView := workerViews[0]

	// Calculate risk components manually
	// E_hat = 10 * (1 + 0.1*(2/8) + 0.1*(4/16) + 0.1*(0/2) + 0.2*0.5)
	// E_hat = 10 * (1 + 0.025 + 0.025 + 0 + 0.1) = 10 * 1.15 = 11.5
	expectedEHat := 11.5
	eHat := rts.predictExecTime(taskView, workerView, params.Theta)
	if math.Abs(eHat-expectedEHat) > 0.1 {
		t.Errorf("E_hat calculation error: expected %.2f, got %.2f", expectedEHat, eHat)
	}

	// f_hat = now + 11.5 seconds
	// deadline = now + 20 seconds (2.0 * 10.0)
	// delta = max(0, f_hat - deadline) = 0 (no violation)
	// R_base = 10.0 * 0 + 1.0 * 0.5 = 0.5
	expectedBaseRisk := 0.5
	baseRisk := rts.computeBaseRisk(taskView, workerView, eHat, params.Risk.Alpha, params.Risk.Beta)
	if math.Abs(baseRisk-expectedBaseRisk) > 0.01 {
		t.Errorf("Base risk calculation error: expected %.2f, got %.2f", expectedBaseRisk, baseRisk)
	}

	// R_final = 0.5 - 2.0 + 0.5 = -1.0 (negative is good!)
	expectedFinalRisk := -1.0
	finalRisk := rts.computeFinalRisk(baseRisk, taskView.Type, workerView.ID, params)
	if math.Abs(finalRisk-expectedFinalRisk) > 0.01 {
		t.Errorf("Final risk calculation error: expected %.2f, got %.2f", expectedFinalRisk, finalRisk)
	}

	t.Logf("✓ Risk calculation verified: E_hat=%.2f, BaseRisk=%.2f, FinalRisk=%.2f",
		eHat, baseRisk, finalRisk)
}

// TestRTS_OptimizesBetterThanRoundRobin compares RTS vs Round-Robin
func TestRTS_OptimizesBetterThanRoundRobin(t *testing.T) {
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPUHeavy, 20.0)
	telSource := NewMockTelemetrySource()

	// Worker1: Poor choice (high load, limited resources)
	telSource.AddWorker(WorkerView{
		ID:           "worker1",
		CPUAvail:     4.0,
		MemAvail:     8.0,
		GPUAvail:     0.0,
		StorageAvail: 50.0,
		Load:         0.95, // Very high load
	})

	// Worker2: Good choice (low load, ample resources)
	telSource.AddWorker(WorkerView{
		ID:           "worker2",
		CPUAvail:     16.0,
		MemAvail:     32.0,
		GPUAvail:     4.0,
		StorageAvail: 200.0,
		Load:         0.1, // Low load
	})

	// Round-Robin scheduler
	rrScheduler := NewRoundRobinScheduler()

	// RTS scheduler
	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	task := &pb.Task{
		TaskId:     "compare-task",
		TaskType:   TaskTypeCPUHeavy,
		ReqCpu:     4.0,
		ReqMemory:  8.0,
		ReqGpu:     0.0,
		ReqStorage: 50.0,
	}

	workers := map[string]*WorkerInfo{
		"worker1": {WorkerID: "worker1", IsActive: true},
		"worker2": {WorkerID: "worker2", IsActive: true},
	}

	// Round-Robin selection (will alternate, not optimize)
	rrSelection := rrScheduler.SelectWorker(task, workers)
	t.Logf("Round-Robin selected: %s", rrSelection)

	// RTS selection (should optimize)
	rtsSelection := rts.SelectWorker(task, workers)
	t.Logf("RTS selected: %s", rtsSelection)

	// RTS should select worker2 (better choice)
	if rtsSelection != "worker2" {
		t.Errorf("OPTIMIZATION FAILURE: RTS did not outperform Round-Robin. Expected worker2, got %s", rtsSelection)
	}

	// Verify RTS makes smarter choice than RR
	if rtsSelection == rrSelection && rrSelection == "worker1" {
		t.Error("OPTIMIZATION FAILURE: RTS made same poor choice as Round-Robin")
	}
}

// TestRTS_OptimizesWithDynamicParameterUpdates verifies parameter hot-reloading improves decisions
func TestRTS_OptimizesWithDynamicParameterUpdates(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPULight, 10.0)
	telSource := NewMockTelemetrySource()

	// Two similar workers
	telSource.AddWorker(WorkerView{
		ID:           "worker1",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.5,
	})

	telSource.AddWorker(WorkerView{
		ID:           "worker2",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.5,
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	task := &pb.Task{
		TaskId:     "dynamic-task",
		TaskType:   TaskTypeCPULight,
		ReqCpu:     2.0,
		ReqMemory:  4.0,
		ReqGpu:     0.0,
		ReqStorage: 20.0,
	}

	workers := map[string]*WorkerInfo{
		"worker1": {WorkerID: "worker1", IsActive: true},
		"worker2": {WorkerID: "worker2", IsActive: true},
	}

	// First selection with no affinity
	selection1 := rts.SelectWorker(task, workers)
	t.Logf("Selection without affinity: %s", selection1)

	// Update parameters to favor worker2
	params := GetDefaultGAParams()
	params.AffinityMatrix = map[string]map[string]float64{
		TaskTypeCPULight: {
			"worker2": 10.0, // Strong preference for worker2
			"worker1": 0.0,
		},
	}
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	// Second selection with affinity
	selection2 := rts.SelectWorker(task, workers)
	t.Logf("Selection with affinity for worker2: %s", selection2)

	// After parameter update, should prefer worker2
	if selection2 != "worker2" {
		t.Errorf("OPTIMIZATION FAILURE: Dynamic parameter update did not change decision. Expected worker2, got %s", selection2)
	}
}
