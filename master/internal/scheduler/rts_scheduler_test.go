package scheduler

import (
	"context"
	"testing"
	"time"

	pb "master/proto"
)

// Mock TauStore for testing
type MockTauStore struct {
	tauValues map[string]float64
}

func NewMockTauStore() *MockTauStore {
	return &MockTauStore{
		tauValues: map[string]float64{
			TaskTypeCPULight:     5.0,
			TaskTypeCPUHeavy:     15.0,
			TaskTypeMemoryHeavy:  20.0,
			TaskTypeGPUInference: 10.0,
			TaskTypeGPUTraining:  60.0,
			TaskTypeMixed:        10.0,
		},
	}
}

func (m *MockTauStore) GetTau(taskType string) float64 {
	if tau, ok := m.tauValues[taskType]; ok {
		return tau
	}
	return 10.0 // Default
}

func (m *MockTauStore) UpdateTau(taskType string, actualRuntime float64) {
	m.tauValues[taskType] = actualRuntime
}

func (m *MockTauStore) SetTau(taskType string, tau float64) {
	m.tauValues[taskType] = tau
}

// Mock TelemetrySource for testing
type MockTelemetrySource struct {
	workerViews []WorkerView
	workerLoads map[string]float64
}

func NewMockTelemetrySource() *MockTelemetrySource {
	return &MockTelemetrySource{
		workerViews: []WorkerView{},
		workerLoads: make(map[string]float64),
	}
}

func (m *MockTelemetrySource) AddWorker(view WorkerView) {
	m.workerViews = append(m.workerViews, view)
	m.workerLoads[view.ID] = view.Load
}

func (m *MockTelemetrySource) GetWorkerViews(ctx context.Context) ([]WorkerView, error) {
	return m.workerViews, nil
}

func (m *MockTelemetrySource) GetWorkerLoad(workerID string) float64 {
	if load, ok := m.workerLoads[workerID]; ok {
		return load
	}
	return 0.0
}

// Test RTSScheduler implements Scheduler interface
func TestRTSScheduler_Interface(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	telSource := NewMockTelemetrySource()

	var _ Scheduler = NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
}

// Test RTSScheduler initialization
func TestNewRTSScheduler(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	telSource := NewMockTelemetrySource()

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)

	if rts == nil {
		t.Fatal("RTSScheduler should not be nil")
	}
	if rts.GetName() != "RTS" {
		t.Errorf("Expected name 'RTS', got '%s'", rts.GetName())
	}
	if rts.slaMultiplier != 2.0 {
		t.Errorf("Expected slaMultiplier 2.0, got %.1f", rts.slaMultiplier)
	}

	// Cleanup
	rts.Shutdown()
}

// Test SelectWorker with feasible workers
func TestRTSScheduler_SelectWorker_Feasible(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	telSource := NewMockTelemetrySource()

	// Add workers to telemetry source
	telSource.AddWorker(WorkerView{
		ID:           "worker1",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.3,
	})
	telSource.AddWorker(WorkerView{
		ID:           "worker2",
		CPUAvail:     4.0,
		MemAvail:     8.0,
		GPUAvail:     0.0,
		StorageAvail: 50.0,
		Load:         0.6,
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Create task
	task := &pb.Task{
		TaskId:     "task1",
		TaskType:   TaskTypeCPULight,
		ReqCpu:     2.0,
		ReqMemory:  4.0,
		ReqGpu:     0.0,
		ReqStorage: 10.0,
	}

	// Create workers map
	workers := map[string]*WorkerInfo{
		"worker1": {WorkerID: "worker1", IsActive: true},
		"worker2": {WorkerID: "worker2", IsActive: true},
	}

	// Select worker
	selected := rts.SelectWorker(task, workers)

	if selected == "" {
		t.Fatal("Should select a worker")
	}
	if selected != "worker1" && selected != "worker2" {
		t.Errorf("Selected invalid worker: %s", selected)
	}
}

// Test SelectWorker fallback when no feasible workers
func TestRTSScheduler_SelectWorker_NoFeasible(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	telSource := NewMockTelemetrySource()

	// Add worker with insufficient resources
	telSource.AddWorker(WorkerView{
		ID:           "worker1",
		CPUAvail:     1.0,
		MemAvail:     2.0,
		GPUAvail:     0.0,
		StorageAvail: 10.0,
		Load:         0.9,
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Create task requiring more resources
	task := &pb.Task{
		TaskId:     "task1",
		TaskType:   TaskTypeCPUHeavy,
		ReqCpu:     4.0,
		ReqMemory:  8.0,
		ReqGpu:     0.0,
		ReqStorage: 20.0,
	}

	// Create workers map
	workers := map[string]*WorkerInfo{
		"worker1": {
			WorkerID:         "worker1",
			IsActive:         true,
			AvailableCPU:     1.0,
			AvailableMemory:  2.0,
			AvailableGPU:     0.0,
			AvailableStorage: 10.0,
		},
	}

	// Select worker - should fallback to Round-Robin
	// Round-Robin will also find no suitable worker
	selected := rts.SelectWorker(task, workers)

	if selected != "" {
		t.Errorf("Expected no worker selected due to insufficient resources, got %s", selected)
	}
}

// Test SelectWorker successful fallback to Round-Robin
func TestRTSScheduler_SelectWorker_SuccessfulFallback(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	telSource := NewMockTelemetrySource()

	// Add worker WITH telemetry BUT not in workers map initially
	// Then add worker to workers map, so RTS will find it via fallback
	telSource.AddWorker(WorkerView{
		ID:           "worker1",
		CPUAvail:     8.0,
		MemAvail:     16.0,
		GPUAvail:     2.0,
		StorageAvail: 100.0,
		Load:         0.5,
	})

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	task := &pb.Task{
		TaskId:     "task1",
		TaskType:   TaskTypeCPULight,
		ReqCpu:     2.0,
		ReqMemory:  4.0,
		ReqGpu:     0.0,
		ReqStorage: 10.0,
	}

	// Workers with sufficient resources
	workers := map[string]*WorkerInfo{
		"worker1": {
			WorkerID:         "worker1",
			IsActive:         true,
			AvailableCPU:     8.0,
			AvailableMemory:  16.0,
			AvailableGPU:     2.0,
			AvailableStorage: 100.0,
		},
	}

	// RTS should select worker1 (has both telemetry and in workers map)
	selected := rts.SelectWorker(task, workers)

	if selected != "worker1" {
		t.Errorf("Expected to select worker1, got %s", selected)
	}
}

// Test filterFeasible
func TestRTSScheduler_FilterFeasible(t *testing.T) {
	rts := &RTSScheduler{}

	task := TaskView{
		CPU:     2.0,
		Mem:     4.0,
		GPU:     1.0,
		Storage: 10.0,
	}

	workers := []WorkerView{
		{ID: "worker1", CPUAvail: 8.0, MemAvail: 16.0, GPUAvail: 2.0, StorageAvail: 100.0},
		{ID: "worker2", CPUAvail: 1.0, MemAvail: 2.0, GPUAvail: 0.0, StorageAvail: 5.0},
		{ID: "worker3", CPUAvail: 4.0, MemAvail: 8.0, GPUAvail: 1.0, StorageAvail: 50.0},
	}

	feasible := rts.filterFeasible(task, workers)

	if len(feasible) != 2 {
		t.Fatalf("Expected 2 feasible workers, got %d", len(feasible))
	}

	// Should include worker1 and worker3, not worker2
	feasibleIDs := make(map[string]bool)
	for _, w := range feasible {
		feasibleIDs[w.ID] = true
	}

	if !feasibleIDs["worker1"] {
		t.Error("worker1 should be feasible")
	}
	if feasibleIDs["worker2"] {
		t.Error("worker2 should not be feasible")
	}
	if !feasibleIDs["worker3"] {
		t.Error("worker3 should be feasible")
	}
}

// Test predictExecTime
func TestRTSScheduler_PredictExecTime(t *testing.T) {
	rts := &RTSScheduler{}

	task := TaskView{
		Tau: 10.0,
		CPU: 2.0,
		Mem: 4.0,
		GPU: 1.0,
	}

	worker := WorkerView{
		CPUAvail: 8.0,
		MemAvail: 16.0,
		GPUAvail: 2.0,
		Load:     0.5,
	}

	theta := Theta{
		Theta1: 0.1,
		Theta2: 0.1,
		Theta3: 0.3,
		Theta4: 0.2,
	}

	eHat := rts.predictExecTime(task, worker, theta)

	// Expected: 10 * (1 + 0.1*(2/8) + 0.1*(4/16) + 0.3*(1/2) + 0.2*0.5)
	// = 10 * (1 + 0.025 + 0.025 + 0.15 + 0.1)
	// = 10 * 1.3 = 13.0
	expected := 13.0
	if eHat < expected-0.1 || eHat > expected+0.1 {
		t.Errorf("Expected eHat ~%.1f, got %.1f", expected, eHat)
	}
}

// Test computeBaseRisk with no deadline violation
func TestRTSScheduler_ComputeBaseRisk_NoViolation(t *testing.T) {
	rts := &RTSScheduler{}

	task := TaskView{
		ArrivalTime: time.Now(),
		Deadline:    time.Now().Add(30 * time.Second),
	}

	worker := WorkerView{
		Load: 0.5,
	}

	eHat := 10.0 // Task will finish in 10 seconds
	alpha := 10.0
	beta := 1.0

	baseRisk := rts.computeBaseRisk(task, worker, eHat, alpha, beta)

	// No deadline violation, so delta = 0
	// Risk = 0 + 1.0 * 0.5 = 0.5
	expected := 0.5
	if baseRisk < expected-0.01 || baseRisk > expected+0.01 {
		t.Errorf("Expected baseRisk %.2f, got %.2f", expected, baseRisk)
	}
}

// Test computeBaseRisk with deadline violation
func TestRTSScheduler_ComputeBaseRisk_Violation(t *testing.T) {
	rts := &RTSScheduler{}

	task := TaskView{
		ArrivalTime: time.Now(),
		Deadline:    time.Now().Add(5 * time.Second), // Tight deadline
	}

	worker := WorkerView{
		Load: 0.5,
	}

	eHat := 10.0 // Task will take 10 seconds, deadline is 5 seconds
	alpha := 10.0
	beta := 1.0

	baseRisk := rts.computeBaseRisk(task, worker, eHat, alpha, beta)

	// Deadline violation delta = 5 seconds
	// Risk = 10.0 * 5 + 1.0 * 0.5 = 50.5
	expected := 50.5
	if baseRisk < expected-0.5 || baseRisk > expected+0.5 {
		t.Errorf("Expected baseRisk ~%.1f, got %.2f", expected, baseRisk)
	}
}

// Test computeFinalRisk with affinity and penalty
func TestRTSScheduler_ComputeFinalRisk(t *testing.T) {
	rts := &RTSScheduler{}

	params := &GAParams{
		AffinityMatrix: map[string]map[string]float64{
			TaskTypeCPULight: {
				"worker1": 2.0,  // Positive affinity (good match)
				"worker2": -1.0, // Negative affinity (poor match)
			},
		},
		PenaltyVector: map[string]float64{
			"worker1": 0.5, // Small penalty
			"worker2": 2.0, // Large penalty
		},
	}

	baseRisk := 10.0

	// Worker1: Good affinity, small penalty
	risk1 := rts.computeFinalRisk(baseRisk, TaskTypeCPULight, "worker1", params)
	// = 10.0 - 2.0 + 0.5 = 8.5
	if risk1 != 8.5 {
		t.Errorf("Worker1: Expected risk 8.5, got %.1f", risk1)
	}

	// Worker2: Poor affinity, large penalty
	risk2 := rts.computeFinalRisk(baseRisk, TaskTypeCPULight, "worker2", params)
	// = 10.0 - (-1.0) + 2.0 = 13.0
	if risk2 != 13.0 {
		t.Errorf("Worker2: Expected risk 13.0, got %.1f", risk2)
	}
}

// Test SelectWorker chooses lower risk worker
func TestRTSScheduler_SelectWorker_ChoosesLowerRisk(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	telSource := NewMockTelemetrySource()

	// Add two workers with different characteristics
	telSource.AddWorker(WorkerView{
		ID:           "worker-slow",
		CPUAvail:     4.0,
		MemAvail:     8.0,
		GPUAvail:     0.0,
		StorageAvail: 50.0,
		Load:         0.9, // High load
	})
	telSource.AddWorker(WorkerView{
		ID:           "worker-fast",
		CPUAvail:     16.0,
		MemAvail:     32.0,
		GPUAvail:     4.0,
		StorageAvail: 200.0,
		Load:         0.1, // Low load
	})

	// Create RTS with affinity favoring worker-fast
	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Set affinity to favor worker-fast
	params := GetDefaultGAParams()
	params.AffinityMatrix = map[string]map[string]float64{
		TaskTypeCPULight: {
			"worker-fast": 5.0,  // High affinity
			"worker-slow": -2.0, // Low affinity
		},
	}
	rts.paramsMu.Lock()
	rts.params = params
	rts.paramsMu.Unlock()

	task := &pb.Task{
		TaskId:     "task1",
		TaskType:   TaskTypeCPULight,
		ReqCpu:     2.0,
		ReqMemory:  4.0,
		ReqGpu:     0.0,
		ReqStorage: 10.0,
	}

	workers := map[string]*WorkerInfo{
		"worker-slow": {WorkerID: "worker-slow", IsActive: true},
		"worker-fast": {WorkerID: "worker-fast", IsActive: true},
	}

	selected := rts.SelectWorker(task, workers)

	if selected != "worker-fast" {
		t.Errorf("Expected to select worker-fast, got %s", selected)
	}
}

// Test buildTaskView
func TestRTSScheduler_BuildTaskView(t *testing.T) {
	tauStore := NewMockTauStore()
	tauStore.SetTau(TaskTypeCPULight, 15.0)

	rts := &RTSScheduler{
		tauStore:      tauStore,
		slaMultiplier: 2.0,
	}

	task := &pb.Task{
		TaskId:     "task1",
		TaskType:   TaskTypeCPULight,
		ReqCpu:     2.0,
		ReqMemory:  4.0,
		ReqGpu:     0.0,
		ReqStorage: 10.0,
	}

	now := time.Now()
	taskView := rts.buildTaskView(task, now)

	if taskView.ID != "task1" {
		t.Errorf("Expected ID 'task1', got '%s'", taskView.ID)
	}
	if taskView.Type != TaskTypeCPULight {
		t.Errorf("Expected type '%s', got '%s'", TaskTypeCPULight, taskView.Type)
	}
	if taskView.Tau != 15.0 {
		t.Errorf("Expected Tau 15.0, got %.1f", taskView.Tau)
	}
	if taskView.CPU != 2.0 {
		t.Errorf("Expected CPU 2.0, got %.1f", taskView.CPU)
	}

	// Check deadline = arrival + k * tau
	expectedDeadline := now.Add(time.Duration(2.0 * 15.0 * float64(time.Second)))
	if taskView.Deadline.Sub(expectedDeadline).Abs() > time.Second {
		t.Errorf("Deadline mismatch: expected %v, got %v", expectedDeadline, taskView.Deadline)
	}
}

// Test buildWorkerViews filters inactive workers
func TestRTSScheduler_BuildWorkerViews_FilterInactive(t *testing.T) {
	telSource := NewMockTelemetrySource()
	telSource.AddWorker(WorkerView{ID: "worker1", CPUAvail: 8.0})
	telSource.AddWorker(WorkerView{ID: "worker2", CPUAvail: 4.0})
	telSource.AddWorker(WorkerView{ID: "worker3", CPUAvail: 16.0})

	rts := &RTSScheduler{
		telemetrySource: telSource,
	}

	// Only worker1 and worker3 are in the workers map (active)
	workers := map[string]*WorkerInfo{
		"worker1": {WorkerID: "worker1", IsActive: true},
		"worker3": {WorkerID: "worker3", IsActive: true},
	}

	views := rts.buildWorkerViews(workers)

	if len(views) != 2 {
		t.Fatalf("Expected 2 workers, got %d", len(views))
	}

	// Check that only worker1 and worker3 are included
	viewIDs := make(map[string]bool)
	for _, v := range views {
		viewIDs[v.ID] = true
	}

	if !viewIDs["worker1"] {
		t.Error("worker1 should be included")
	}
	if viewIDs["worker2"] {
		t.Error("worker2 should not be included")
	}
	if !viewIDs["worker3"] {
		t.Error("worker3 should be included")
	}
}

// Test Reset
func TestRTSScheduler_Reset(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	telSource := NewMockTelemetrySource()

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	// Reset should not panic
	rts.Reset()
}

// Test with empty workers map
func TestRTSScheduler_SelectWorker_NoWorkers(t *testing.T) {
	rrScheduler := NewRoundRobinScheduler()
	tauStore := NewMockTauStore()
	telSource := NewMockTelemetrySource()

	rts := NewRTSScheduler(rrScheduler, tauStore, telSource, "test.json", 2.0)
	defer rts.Shutdown()

	task := &pb.Task{
		TaskId:   "task1",
		TaskType: TaskTypeCPULight,
		ReqCpu:   2.0,
	}

	workers := map[string]*WorkerInfo{}

	selected := rts.SelectWorker(task, workers)

	if selected != "" {
		t.Errorf("Expected empty string with no workers, got %s", selected)
	}
}
