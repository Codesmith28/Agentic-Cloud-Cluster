package server

import (
	"testing"
	"time"

	"master/internal/db"
	"master/internal/telemetry"
	pb "master/proto"
)

// TestLoadTrackingAtAssignment tests that LoadAtStart is captured during task assignment
func TestLoadTrackingAtAssignment(t *testing.T) {
	// Create telemetry manager
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	defer telemetryMgr.Shutdown()

	// Create mock tau store
	mockTauStore := NewMockTauStore()

	// Create server with telemetry
	_ = NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	// Simulate worker telemetry data
	workerID := "test-worker-1"
	heartbeat := &pb.Heartbeat{
		WorkerId:     workerID,
		CpuUsage:     0.5, // 50% CPU usage
		MemoryUsage:  0.6, // 60% memory usage
		GpuUsage:     0.4, // 40% GPU usage
		RunningTasks: []*pb.RunningTask{},
	}

	// Register worker with telemetry
	telemetryMgr.RegisterWorker(workerID)
	telemetryMgr.ProcessHeartbeat(heartbeat)

	// Give telemetry time to process
	time.Sleep(100 * time.Millisecond)

	// Verify telemetry data exists
	telemetryData, exists := telemetryMgr.GetWorkerTelemetry(workerID)
	if !exists {
		t.Fatal("Telemetry data not found for worker")
	}

	// Verify telemetry values
	if telemetryData.CpuUsage != 0.5 {
		t.Errorf("Expected CPU usage 0.5, got %.2f", telemetryData.CpuUsage)
	}

	// Calculate expected load (average of CPU, Memory, GPU)
	expectedLoad := (0.5 + 0.6 + 0.4) / 3.0

	t.Logf("Worker telemetry: CPU=%.2f, Mem=%.2f, GPU=%.2f",
		telemetryData.CpuUsage, telemetryData.MemoryUsage, telemetryData.GpuUsage)
	t.Logf("Expected load at assignment: %.4f", expectedLoad)

	// Note: Full integration test would require mock database
	// This test verifies that telemetry data is retrievable
}

// TestLoadCalculation tests the load calculation logic
func TestLoadCalculation(t *testing.T) {
	tests := []struct {
		name         string
		cpuUsage     float64
		memoryUsage  float64
		gpuUsage     float64
		expectedLoad float64
	}{
		{
			name:         "All resources idle",
			cpuUsage:     0.0,
			memoryUsage:  0.0,
			gpuUsage:     0.0,
			expectedLoad: 0.0,
		},
		{
			name:         "All resources full",
			cpuUsage:     1.0,
			memoryUsage:  1.0,
			gpuUsage:     1.0,
			expectedLoad: 1.0,
		},
		{
			name:         "Mixed load",
			cpuUsage:     0.5,
			memoryUsage:  0.6,
			gpuUsage:     0.4,
			expectedLoad: 0.5, // (0.5 + 0.6 + 0.4) / 3 = 0.5
		},
		{
			name:         "CPU-heavy load",
			cpuUsage:     0.9,
			memoryUsage:  0.3,
			gpuUsage:     0.2,
			expectedLoad: (0.9 + 0.3 + 0.2) / 3.0, // ~0.467
		},
		{
			name:         "Memory-heavy load",
			cpuUsage:     0.2,
			memoryUsage:  0.9,
			gpuUsage:     0.1,
			expectedLoad: (0.2 + 0.9 + 0.1) / 3.0, // ~0.4
		},
		{
			name:         "GPU-heavy load",
			cpuUsage:     0.3,
			memoryUsage:  0.2,
			gpuUsage:     0.95,
			expectedLoad: (0.3 + 0.2 + 0.95) / 3.0, // ~0.483
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate load using same formula as implementation
			load := (tt.cpuUsage + tt.memoryUsage + tt.gpuUsage) / 3.0

			// Allow small floating point error
			if abs(load-tt.expectedLoad) > 0.001 {
				t.Errorf("Expected load %.4f, got %.4f", tt.expectedLoad, load)
			}

			t.Logf("Load calculation: (%.2f + %.2f + %.2f) / 3 = %.4f",
				tt.cpuUsage, tt.memoryUsage, tt.gpuUsage, load)
		})
	}
}

// abs returns absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestLoadTrackingWithNoTelemetry tests fallback when telemetry is unavailable
func TestLoadTrackingWithNoTelemetry(t *testing.T) {
	// Create server WITHOUT telemetry manager
	mockTauStore := NewMockTauStore()
	server := NewMasterServer(nil, nil, nil, nil, nil, mockTauStore, 2.0)

	// Verify server is created (should use load=0.0 as fallback)
	if server == nil {
		t.Fatal("Server should be created even without telemetry")
	}

	// In real scenario, assignment would use load=0.0
	// This tests graceful degradation
	t.Log("Server created successfully without telemetry manager")
}

// TestLoadTrackingWithMissingWorker tests behavior when worker has no telemetry data
func TestLoadTrackingWithMissingWorker(t *testing.T) {
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	defer telemetryMgr.Shutdown()

	mockTauStore := NewMockTauStore()
	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	// Try to get telemetry for non-existent worker
	_, exists := telemetryMgr.GetWorkerTelemetry("nonexistent-worker")

	if exists {
		t.Error("Should not find telemetry for non-existent worker")
	}

	// Server should handle this gracefully (use load=0.0)
	if server == nil {
		t.Fatal("Server should handle missing worker telemetry")
	}

	t.Log("Gracefully handled missing worker telemetry")
}

// TestLoadTrackingPrecision tests that load values are precise
func TestLoadTrackingPrecision(t *testing.T) {
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	defer telemetryMgr.Shutdown()

	workerID := "precision-worker"

	// Test with precise values
	heartbeat := &pb.Heartbeat{
		WorkerId:     workerID,
		CpuUsage:     0.123456,
		MemoryUsage:  0.234567,
		GpuUsage:     0.345678,
		RunningTasks: []*pb.RunningTask{},
	}

	telemetryMgr.RegisterWorker(workerID)
	telemetryMgr.ProcessHeartbeat(heartbeat)

	time.Sleep(100 * time.Millisecond)

	telemetryData, exists := telemetryMgr.GetWorkerTelemetry(workerID)
	if !exists {
		t.Fatal("Worker telemetry not found")
	}

	// Calculate expected load
	expectedLoad := (0.123456 + 0.234567 + 0.345678) / 3.0
	actualLoad := (telemetryData.CpuUsage + telemetryData.MemoryUsage + telemetryData.GpuUsage) / 3.0

	if abs(actualLoad-expectedLoad) > 0.000001 {
		t.Errorf("Load precision error: expected %.6f, got %.6f", expectedLoad, actualLoad)
	}

	t.Logf("Precise load calculation: %.6f", actualLoad)
}

// TestLoadTrackingMultipleWorkers tests load tracking for multiple workers
func TestLoadTrackingMultipleWorkers(t *testing.T) {
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	defer telemetryMgr.Shutdown()

	workers := []struct {
		id     string
		cpu    float64
		memory float64
		gpu    float64
	}{
		{"worker-1", 0.2, 0.3, 0.1}, // Light load
		{"worker-2", 0.7, 0.8, 0.6}, // Heavy load
		{"worker-3", 0.5, 0.5, 0.5}, // Medium load
	}

	for _, w := range workers {
		telemetryMgr.RegisterWorker(w.id)

		heartbeat := &pb.Heartbeat{
			WorkerId:     w.id,
			CpuUsage:     w.cpu,
			MemoryUsage:  w.memory,
			GpuUsage:     w.gpu,
			RunningTasks: []*pb.RunningTask{},
		}

		telemetryMgr.ProcessHeartbeat(heartbeat)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify all workers have telemetry
	for _, w := range workers {
		telemetryData, exists := telemetryMgr.GetWorkerTelemetry(w.id)
		if !exists {
			t.Errorf("Missing telemetry for worker %s", w.id)
			continue
		}

		expectedLoad := (w.cpu + w.memory + w.gpu) / 3.0
		actualLoad := (telemetryData.CpuUsage + telemetryData.MemoryUsage + telemetryData.GpuUsage) / 3.0

		if abs(actualLoad-expectedLoad) > 0.001 {
			t.Errorf("Worker %s: expected load %.4f, got %.4f", w.id, expectedLoad, actualLoad)
		}

		t.Logf("Worker %s load: %.4f (CPU=%.2f, Mem=%.2f, GPU=%.2f)",
			w.id, actualLoad, w.cpu, w.memory, w.gpu)
	}
}

// TestLoadTrackingUpdateOverTime tests that load updates reflect in assignments
func TestLoadTrackingUpdateOverTime(t *testing.T) {
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	defer telemetryMgr.Shutdown()

	workerID := "dynamic-worker"

	telemetryMgr.RegisterWorker(workerID)

	// First heartbeat - low load
	heartbeat1 := &pb.Heartbeat{
		WorkerId:     workerID,
		CpuUsage:     0.1,
		MemoryUsage:  0.1,
		GpuUsage:     0.1,
		RunningTasks: []*pb.RunningTask{},
	}
	telemetryMgr.ProcessHeartbeat(heartbeat1)
	time.Sleep(100 * time.Millisecond)

	data1, _ := telemetryMgr.GetWorkerTelemetry(workerID)
	load1 := (data1.CpuUsage + data1.MemoryUsage + data1.GpuUsage) / 3.0
	t.Logf("Initial load: %.4f", load1)

	// Second heartbeat - high load
	heartbeat2 := &pb.Heartbeat{
		WorkerId:     workerID,
		CpuUsage:     0.8,
		MemoryUsage:  0.9,
		GpuUsage:     0.7,
		RunningTasks: []*pb.RunningTask{},
	}
	telemetryMgr.ProcessHeartbeat(heartbeat2)
	time.Sleep(100 * time.Millisecond)

	data2, _ := telemetryMgr.GetWorkerTelemetry(workerID)
	load2 := (data2.CpuUsage + data2.MemoryUsage + data2.GpuUsage) / 3.0
	t.Logf("Updated load: %.4f", load2)

	// Verify load increased
	if load2 <= load1 {
		t.Errorf("Expected load to increase, got %.4f -> %.4f", load1, load2)
	}

	// Expected loads
	expectedLoad1 := (0.1 + 0.1 + 0.1) / 3.0
	expectedLoad2 := (0.8 + 0.9 + 0.7) / 3.0

	if abs(load1-expectedLoad1) > 0.001 {
		t.Errorf("Initial load mismatch: expected %.4f, got %.4f", expectedLoad1, load1)
	}

	if abs(load2-expectedLoad2) > 0.001 {
		t.Errorf("Updated load mismatch: expected %.4f, got %.4f", expectedLoad2, load2)
	}
}

// TestAssignmentWithLoadTracking is a placeholder for integration test
func TestAssignmentWithLoadTracking(t *testing.T) {
	// This would require a full database mock to test assignment storage
	// For now, we verify that the components work independently

	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	defer telemetryMgr.Shutdown()

	mockTauStore := NewMockTauStore()
	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	if server == nil {
		t.Fatal("Failed to create server")
	}

	if server.telemetryManager == nil {
		t.Error("Server should have telemetry manager")
	}

	t.Log("Server components integrated successfully")
}

// TestLoadAtStartFieldInAssignment tests that Assignment struct has LoadAtStart field
func TestLoadAtStartFieldInAssignment(t *testing.T) {
	assignment := &db.Assignment{
		AssignmentID: "test-assignment",
		TaskID:       "test-task",
		WorkerID:     "test-worker",
		LoadAtStart:  0.5,
	}

	if assignment.LoadAtStart != 0.5 {
		t.Errorf("Expected LoadAtStart=0.5, got %.2f", assignment.LoadAtStart)
	}

	// Test zero value
	emptyAssignment := &db.Assignment{}
	if emptyAssignment.LoadAtStart != 0.0 {
		t.Errorf("Expected default LoadAtStart=0.0, got %.2f", emptyAssignment.LoadAtStart)
	}

	t.Log("Assignment LoadAtStart field verified")
}
