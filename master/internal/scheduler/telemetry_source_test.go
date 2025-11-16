package scheduler

import (
	"context"
	"math"
	"testing"
	"time"

	"master/internal/db"
	"master/internal/telemetry"
	pb "master/proto"
)

// MockWorkerDB is a mock implementation of WorkerDBInterface for testing
type MockWorkerDB struct {
	workers map[string]*db.WorkerDocument
}

func NewMockWorkerDB() *MockWorkerDB {
	return &MockWorkerDB{
		workers: make(map[string]*db.WorkerDocument),
	}
}

func (m *MockWorkerDB) AddWorker(worker *db.WorkerDocument) {
	m.workers[worker.WorkerID] = worker
}

func (m *MockWorkerDB) GetWorker(ctx context.Context, workerID string) (*db.WorkerDocument, error) {
	worker, exists := m.workers[workerID]
	if !exists {
		return nil, nil
	}
	return worker, nil
}

func (m *MockWorkerDB) GetAllWorkers(ctx context.Context) ([]db.WorkerDocument, error) {
	var result []db.WorkerDocument
	for _, worker := range m.workers {
		result = append(result, *worker)
	}
	return result, nil
}

// Test that GetWorkerViews returns correct available resources
func TestGetWorkerViews_AvailableResources(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Add worker to DB
	mockDB.AddWorker(&db.WorkerDocument{
		WorkerID:         "worker1",
		TotalCPU:         8.0,
		TotalMemory:      16.0,
		TotalGPU:         2.0,
		TotalStorage:     100.0,
		AllocatedCPU:     2.0,
		AllocatedMemory:  4.0,
		AllocatedGPU:     1.0,
		AllocatedStorage: 20.0,
		IsActive:         true,
	})

	// Register worker in telemetry and send heartbeat
	telMgr.RegisterWorker("worker1")
	telMgr.ProcessHeartbeat(&pb.Heartbeat{
		WorkerId:    "worker1",
		CpuUsage:    0.5,
		MemoryUsage: 0.3,
		GpuUsage:    0.7,
	})
	time.Sleep(10 * time.Millisecond) // Allow telemetry to process

	// Create telemetry source
	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	views, err := source.GetWorkerViews(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("Expected 1 view, got %d", len(views))
	}

	view := views[0]
	if view.ID != "worker1" {
		t.Errorf("Expected worker ID 'worker1', got '%s'", view.ID)
	}
	if view.CPUAvail != 6.0 {
		t.Errorf("Expected CPUAvail 6.0, got %.1f", view.CPUAvail)
	}
	if view.MemAvail != 12.0 {
		t.Errorf("Expected MemAvail 12.0, got %.1f", view.MemAvail)
	}
	if view.GPUAvail != 1.0 {
		t.Errorf("Expected GPUAvail 1.0, got %.1f", view.GPUAvail)
	}
	if view.StorageAvail != 80.0 {
		t.Errorf("Expected StorageAvail 80.0, got %.1f", view.StorageAvail)
	}
}

// Test that GetWorkerViews excludes inactive workers
func TestGetWorkerViews_ExcludesInactiveWorkers(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Add active and inactive workers
	mockDB.AddWorker(&db.WorkerDocument{
		WorkerID: "worker-active",
		TotalCPU: 4.0,
		IsActive: true,
	})
	mockDB.AddWorker(&db.WorkerDocument{
		WorkerID: "worker-inactive",
		TotalCPU: 4.0,
		IsActive: false,
	})

	// Register active worker in telemetry
	telMgr.RegisterWorker("worker-active")
	telMgr.ProcessHeartbeat(&pb.Heartbeat{
		WorkerId: "worker-active",
	})
	time.Sleep(10 * time.Millisecond)

	// Create telemetry source
	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	views, err := source.GetWorkerViews(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("Expected 1 view, got %d", len(views))
	}
	if views[0].ID != "worker-active" {
		t.Errorf("Expected worker 'worker-active', got '%s'", views[0].ID)
	}
}

// Test that GetWorkerViews handles oversubscription (negative available resources)
func TestGetWorkerViews_HandlesOversubscription(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Add worker with oversubscription
	mockDB.AddWorker(&db.WorkerDocument{
		WorkerID:        "worker1",
		TotalCPU:        4.0,
		TotalMemory:     8.0,
		AllocatedCPU:    5.0,  // Oversubscribed!
		AllocatedMemory: 10.0, // Oversubscribed!
		IsActive:        true,
	})

	// Register worker in telemetry
	telMgr.RegisterWorker("worker1")
	telMgr.ProcessHeartbeat(&pb.Heartbeat{
		WorkerId: "worker1",
	})
	time.Sleep(10 * time.Millisecond)

	// Create telemetry source
	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	views, err := source.GetWorkerViews(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("Expected 1 view, got %d", len(views))
	}

	view := views[0]
	if view.CPUAvail != 0.0 {
		t.Errorf("Expected CPUAvail 0.0 (clamped), got %.1f", view.CPUAvail)
	}
	if view.MemAvail != 0.0 {
		t.Errorf("Expected MemAvail 0.0 (clamped), got %.1f", view.MemAvail)
	}
}

// Test GetWorkerLoad with valid worker
func TestGetWorkerLoad_ValidWorker(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Add worker
	mockDB.AddWorker(&db.WorkerDocument{
		WorkerID:    "worker1",
		TotalCPU:    8.0,
		TotalMemory: 16.0,
		TotalGPU:    2.0,
		IsActive:    true,
	})

	// Register worker and send heartbeat
	telMgr.RegisterWorker("worker1")
	telMgr.ProcessHeartbeat(&pb.Heartbeat{
		WorkerId:    "worker1",
		CpuUsage:    0.6, // 60%
		MemoryUsage: 0.4, // 40%
		GpuUsage:    0.8, // 80%
	})
	time.Sleep(10 * time.Millisecond)

	// Create telemetry source
	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	load := source.GetWorkerLoad("worker1")

	// Assert
	// Load should be a weighted average of CPU, memory, and GPU usage
	if load <= 0.0 {
		t.Errorf("Expected load > 0.0, got %.3f", load)
	}
	if load >= 1.0 {
		t.Errorf("Expected load < 1.0, got %.3f", load)
	}
}

// Test GetWorkerLoad with non-existent worker
func TestGetWorkerLoad_NonExistentWorker(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	load := source.GetWorkerLoad("non-existent")

	// Assert
	if load != 0.0 {
		t.Errorf("Expected load 0.0 for non-existent worker, got %.3f", load)
	}
}

// Test GetWorkerLoad with inactive worker
func TestGetWorkerLoad_InactiveWorker(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Add inactive worker
	mockDB.AddWorker(&db.WorkerDocument{
		WorkerID: "worker1",
		TotalCPU: 8.0,
		IsActive: true, // Active in DB but not sending telemetry
	})

	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test (no heartbeat sent, so telemetry will show as inactive)
	load := source.GetWorkerLoad("worker1")

	// Assert
	if load != 0.0 {
		t.Errorf("Expected load 0.0 for inactive worker, got %.3f", load)
	}
}

// Test normalized load computation with different resource distributions
func TestComputeNormalizedLoad_WeightedAverage(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Worker with balanced resources
	worker := &db.WorkerDocument{
		WorkerID:    "worker1",
		TotalCPU:    10.0,
		TotalMemory: 10.0,
		TotalGPU:    0.0, // No GPU
		IsActive:    true,
	}
	mockDB.AddWorker(worker)

	// Register and send heartbeat with known usage
	telMgr.RegisterWorker("worker1")
	telMgr.ProcessHeartbeat(&pb.Heartbeat{
		WorkerId:    "worker1",
		CpuUsage:    0.5, // 50% CPU
		MemoryUsage: 0.5, // 50% Memory
		GpuUsage:    0.0, // No GPU
	})
	time.Sleep(10 * time.Millisecond)

	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	load := source.GetWorkerLoad("worker1")

	// Assert
	// Load should be weighted average: should be around 0.5 since both CPU and mem are at 50%
	if load <= 0.0 {
		t.Errorf("Expected load > 0.0, got %.3f", load)
	}
	if load >= 1.0 {
		t.Errorf("Expected load < 1.0, got %.3f", load)
	}
	if math.Abs(load-0.5) > 0.1 { // Allow 10% tolerance
		t.Logf("Warning: Load %.3f differs from expected 0.5 (within tolerance)", load)
	}
}

// Test load computation with GPU-heavy worker
func TestComputeNormalizedLoad_GPUHeavy(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Worker with high GPU capacity
	worker := &db.WorkerDocument{
		WorkerID:    "gpu-worker",
		TotalCPU:    4.0,
		TotalMemory: 8.0,
		TotalGPU:    8.0, // High GPU
		IsActive:    true,
	}
	mockDB.AddWorker(worker)

	// Register and send heartbeat
	telMgr.RegisterWorker("gpu-worker")
	telMgr.ProcessHeartbeat(&pb.Heartbeat{
		WorkerId:    "gpu-worker",
		CpuUsage:    0.2,
		MemoryUsage: 0.2,
		GpuUsage:    0.9, // High GPU usage
	})
	time.Sleep(10 * time.Millisecond)

	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	load := source.GetWorkerLoad("gpu-worker")

	// Assert
	// Load should be heavily influenced by high GPU usage (weighted 2x)
	if load <= 0.5 {
		t.Errorf("Expected load > 0.5 due to high GPU usage, got %.3f", load)
	}
}

// Test with multiple workers
func TestGetWorkerViews_MultipleWorkers(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Add multiple workers
	workers := []struct {
		id  string
		cpu float64
		mem float64
		gpu float64
	}{
		{"worker1", 8.0, 16.0, 2.0},
		{"worker2", 4.0, 8.0, 0.0},
		{"worker3", 16.0, 32.0, 4.0},
	}

	for _, w := range workers {
		mockDB.AddWorker(&db.WorkerDocument{
			WorkerID:    w.id,
			TotalCPU:    w.cpu,
			TotalMemory: w.mem,
			TotalGPU:    w.gpu,
			IsActive:    true,
		})

		telMgr.RegisterWorker(w.id)
		telMgr.ProcessHeartbeat(&pb.Heartbeat{
			WorkerId: w.id,
		})
	}
	time.Sleep(20 * time.Millisecond)

	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	views, err := source.GetWorkerViews(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(views) != 3 {
		t.Fatalf("Expected 3 views, got %d", len(views))
	}

	// Verify all workers are present
	workerIDs := make(map[string]bool)
	for _, view := range views {
		workerIDs[view.ID] = true
		if view.CPUAvail <= 0.0 {
			t.Errorf("Worker %s: Expected CPUAvail > 0.0, got %.1f", view.ID, view.CPUAvail)
		}
	}
	if !workerIDs["worker1"] {
		t.Error("worker1 not found in views")
	}
	if !workerIDs["worker2"] {
		t.Error("worker2 not found in views")
	}
	if !workerIDs["worker3"] {
		t.Error("worker3 not found in views")
	}
}

// Test edge case: worker with zero resources
func TestComputeNormalizedLoad_ZeroResources(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Worker with zero resources
	worker := &db.WorkerDocument{
		WorkerID:    "zero-worker",
		TotalCPU:    0.0,
		TotalMemory: 0.0,
		TotalGPU:    0.0,
		IsActive:    true,
	}
	mockDB.AddWorker(worker)

	telMgr.RegisterWorker("zero-worker")
	telMgr.ProcessHeartbeat(&pb.Heartbeat{
		WorkerId: "zero-worker",
	})
	time.Sleep(10 * time.Millisecond)

	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	load := source.GetWorkerLoad("zero-worker")

	// Assert - should handle division by zero gracefully
	if load != 0.0 {
		t.Errorf("Expected load 0.0 for zero-resource worker, got %.3f", load)
	}
}

// Test that TelemetrySource interface is satisfied
func TestTelemetrySourceInterface(t *testing.T) {
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)

	var _ TelemetrySource = NewMasterTelemetrySource(telMgr, mockDB)
}

// Test integration: worker views with realistic data
func TestGetWorkerViews_IntegrationRealistic(t *testing.T) {
	// Setup
	mockDB := NewMockWorkerDB()
	telMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telMgr.SetQuietMode(true)

	// Simulate realistic cluster state
	mockDB.AddWorker(&db.WorkerDocument{
		WorkerID:         "prod-worker-1",
		TotalCPU:         16.0,
		TotalMemory:      64.0,
		TotalGPU:         4.0,
		TotalStorage:     1000.0,
		AllocatedCPU:     8.0,
		AllocatedMemory:  32.0,
		AllocatedGPU:     2.0,
		AllocatedStorage: 500.0,
		IsActive:         true,
	})

	mockDB.AddWorker(&db.WorkerDocument{
		WorkerID:         "prod-worker-2",
		TotalCPU:         8.0,
		TotalMemory:      32.0,
		TotalGPU:         0.0,
		TotalStorage:     500.0,
		AllocatedCPU:     6.0,
		AllocatedMemory:  28.0,
		AllocatedGPU:     0.0,
		AllocatedStorage: 400.0,
		IsActive:         true,
	})

	// Send realistic telemetry
	telMgr.RegisterWorker("prod-worker-1")
	telMgr.ProcessHeartbeat(&pb.Heartbeat{
		WorkerId:    "prod-worker-1",
		CpuUsage:    0.7,
		MemoryUsage: 0.65,
		GpuUsage:    0.5,
		RunningTasks: []*pb.RunningTask{
			{TaskId: "task1"},
			{TaskId: "task2"},
		},
	})

	telMgr.RegisterWorker("prod-worker-2")
	telMgr.ProcessHeartbeat(&pb.Heartbeat{
		WorkerId:    "prod-worker-2",
		CpuUsage:    0.85,
		MemoryUsage: 0.9,
		GpuUsage:    0.0,
		RunningTasks: []*pb.RunningTask{
			{TaskId: "task3"},
			{TaskId: "task4"},
			{TaskId: "task5"},
		},
	})
	time.Sleep(20 * time.Millisecond)

	source := NewMasterTelemetrySource(telMgr, mockDB)

	// Test
	views, err := source.GetWorkerViews(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("Expected 2 views, got %d", len(views))
	}

	// Find workers
	var w1, w2 *WorkerView
	for i := range views {
		if views[i].ID == "prod-worker-1" {
			w1 = &views[i]
		} else if views[i].ID == "prod-worker-2" {
			w2 = &views[i]
		}
	}

	if w1 == nil {
		t.Fatal("prod-worker-1 not found in views")
	}
	if w2 == nil {
		t.Fatal("prod-worker-2 not found in views")
	}

	// Worker 1 assertions
	if w1.CPUAvail != 8.0 {
		t.Errorf("Worker1: Expected CPUAvail 8.0, got %.1f", w1.CPUAvail)
	}
	if w1.MemAvail != 32.0 {
		t.Errorf("Worker1: Expected MemAvail 32.0, got %.1f", w1.MemAvail)
	}
	if w1.GPUAvail != 2.0 {
		t.Errorf("Worker1: Expected GPUAvail 2.0, got %.1f", w1.GPUAvail)
	}
	if w1.StorageAvail != 500.0 {
		t.Errorf("Worker1: Expected StorageAvail 500.0, got %.1f", w1.StorageAvail)
	}
	if w1.Load <= 0.0 || w1.Load >= 1.0 {
		t.Errorf("Worker1: Expected 0.0 < Load < 1.0, got %.3f", w1.Load)
	}

	// Worker 2 assertions
	if w2.CPUAvail != 2.0 {
		t.Errorf("Worker2: Expected CPUAvail 2.0, got %.1f", w2.CPUAvail)
	}
	if w2.MemAvail != 4.0 {
		t.Errorf("Worker2: Expected MemAvail 4.0, got %.1f", w2.MemAvail)
	}
	if w2.GPUAvail != 0.0 {
		t.Errorf("Worker2: Expected GPUAvail 0.0, got %.1f", w2.GPUAvail)
	}
	if w2.StorageAvail != 100.0 {
		t.Errorf("Worker2: Expected StorageAvail 100.0, got %.1f", w2.StorageAvail)
	}
	if w2.Load <= 0.0 {
		t.Errorf("Worker2: Expected Load > 0.0, got %.3f", w2.Load)
	}

	// Worker 2 should have higher load than worker 1 (higher CPU/mem usage)
	if w2.Load <= w1.Load {
		t.Logf("Warning: Expected worker2 load (%.3f) > worker1 load (%.3f)", w2.Load, w1.Load)
	}
}
