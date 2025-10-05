package workerregistry

import (
	"testing"
	"time"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
)

// TestNewRegistry tests registry initialization
func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if registry.Count() != 0 {
		t.Errorf("Expected empty registry, got %d workers", registry.Count())
	}

	if registry.ReservationCount() != 0 {
		t.Errorf("Expected no reservations, got %d", registry.ReservationCount())
	}
}

// TestUpdateHeartbeat tests worker registration and heartbeat updates
func TestUpdateHeartbeat(t *testing.T) {
	registry := NewRegistry()

	worker := &pb.Worker{
		Id:       "worker-1",
		TotalCpu: 8.0,
		FreeCpu:  8.0,
		TotalMem: 16384,
		FreeMem:  16384,
		Gpus:     2,
		FreeGpus: 2,
	}

	// First heartbeat - registration
	err := registry.UpdateHeartbeat(worker)
	if err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 worker, got %d", registry.Count())
	}

	// Verify worker was registered
	retrieved, err := registry.GetWorker("worker-1")
	if err != nil {
		t.Fatalf("GetWorker failed: %v", err)
	}

	if retrieved.Id != "worker-1" {
		t.Errorf("Expected worker ID 'worker-1', got '%s'", retrieved.Id)
	}

	if retrieved.LastSeenUnix == 0 {
		t.Error("Expected LastSeenUnix to be set")
	}

	// Second heartbeat - update
	firstSeen := retrieved.LastSeenUnix
	time.Sleep(1100 * time.Millisecond) // Sleep over 1 second to ensure Unix timestamp changes

	worker.FreeCpu = 4.0 // Changed resource
	err = registry.UpdateHeartbeat(worker)
	if err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}

	retrieved, _ = registry.GetWorker("worker-1")
	if retrieved.FreeCpu != 4.0 {
		t.Errorf("Expected FreeCpu to be updated to 4.0, got %.2f", retrieved.FreeCpu)
	}

	if retrieved.LastSeenUnix <= firstSeen {
		t.Errorf("Expected LastSeenUnix to be updated (first=%d, second=%d)", firstSeen, retrieved.LastSeenUnix)
	}

	// Still only one worker
	if registry.Count() != 1 {
		t.Errorf("Expected 1 worker after update, got %d", registry.Count())
	}
}

// TestUpdateHeartbeatInvalidWorker tests error handling
func TestUpdateHeartbeatInvalidWorker(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name   string
		worker *pb.Worker
	}{
		{"nil worker", nil},
		{"empty ID", &pb.Worker{Id: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.UpdateHeartbeat(tt.worker)
			if err == nil {
				t.Error("Expected error for invalid worker")
			}
		})
	}
}

// TestGetSnapshot tests retrieving all workers
func TestGetSnapshot(t *testing.T) {
	registry := NewRegistry()

	// Add multiple workers
	workers := []*pb.Worker{
		{Id: "worker-1", TotalCpu: 4.0, FreeCpu: 4.0, TotalMem: 8192, FreeMem: 8192},
		{Id: "worker-2", TotalCpu: 8.0, FreeCpu: 8.0, TotalMem: 16384, FreeMem: 16384},
		{Id: "worker-3", TotalCpu: 16.0, FreeCpu: 16.0, TotalMem: 32768, FreeMem: 32768},
	}

	for _, w := range workers {
		if err := registry.UpdateHeartbeat(w); err != nil {
			t.Fatalf("UpdateHeartbeat failed: %v", err)
		}
	}

	snapshot := registry.GetSnapshot()

	if len(snapshot) != 3 {
		t.Errorf("Expected 3 workers in snapshot, got %d", len(snapshot))
	}

	// Verify snapshot is a copy (modifying it shouldn't affect registry)
	snapshot[0].FreeCpu = 0.0
	originalWorker, _ := registry.GetWorker("worker-1")
	if originalWorker.FreeCpu == 0.0 {
		t.Error("Snapshot modification affected original worker")
	}
}

// TestResourceReservation tests reserving and releasing resources
func TestResourceReservation(t *testing.T) {
	registry := NewRegistry()

	worker := &pb.Worker{
		Id:       "worker-1",
		TotalCpu: 8.0,
		FreeCpu:  8.0,
		TotalMem: 16384,
		FreeMem:  16384,
		Gpus:     2,
		FreeGpus: 2,
	}

	if err := registry.UpdateHeartbeat(worker); err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}

	// Reserve resources
	err := registry.Reserve("task-1", "worker-1", 4.0, 8192, 1, 5*time.Minute)
	if err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}

	if registry.ReservationCount() != 1 {
		t.Errorf("Expected 1 reservation, got %d", registry.ReservationCount())
	}

	// Verify resources were deducted
	w, _ := registry.GetWorker("worker-1")
	if w.FreeCpu != 4.0 {
		t.Errorf("Expected FreeCpu=4.0 after reservation, got %.2f", w.FreeCpu)
	}
	if w.FreeMem != 8192 {
		t.Errorf("Expected FreeMem=8192 after reservation, got %d", w.FreeMem)
	}
	if w.FreeGpus != 1 {
		t.Errorf("Expected FreeGpus=1 after reservation, got %d", w.FreeGpus)
	}

	// Get reservation details
	reservation, err := registry.GetReservation("task-1")
	if err != nil {
		t.Fatalf("GetReservation failed: %v", err)
	}

	if reservation.TaskID != "task-1" {
		t.Errorf("Expected TaskID='task-1', got '%s'", reservation.TaskID)
	}
	if reservation.CpuCores != 4.0 {
		t.Errorf("Expected CpuCores=4.0, got %.2f", reservation.CpuCores)
	}

	// Release resources
	err = registry.Release("task-1")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	if registry.ReservationCount() != 0 {
		t.Errorf("Expected 0 reservations after release, got %d", registry.ReservationCount())
	}

	// Verify resources were returned
	w, _ = registry.GetWorker("worker-1")
	if w.FreeCpu != 8.0 {
		t.Errorf("Expected FreeCpu=8.0 after release, got %.2f", w.FreeCpu)
	}
	if w.FreeMem != 16384 {
		t.Errorf("Expected FreeMem=16384 after release, got %d", w.FreeMem)
	}
	if w.FreeGpus != 2 {
		t.Errorf("Expected FreeGpus=2 after release, got %d", w.FreeGpus)
	}
}

// TestReserveInsufficientResources tests reservation with insufficient resources
func TestReserveInsufficientResources(t *testing.T) {
	registry := NewRegistry()

	worker := &pb.Worker{
		Id:       "worker-1",
		TotalCpu: 4.0,
		FreeCpu:  4.0,
		TotalMem: 8192,
		FreeMem:  8192,
		Gpus:     1,
		FreeGpus: 1,
	}

	if err := registry.UpdateHeartbeat(worker); err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}

	tests := []struct {
		name   string
		cpu    float64
		mem    int32
		gpu    int32
		errMsg string
	}{
		{"insufficient CPU", 8.0, 4096, 0, "insufficient CPU"},
		{"insufficient memory", 2.0, 16384, 0, "insufficient memory"},
		{"insufficient GPU", 2.0, 4096, 2, "insufficient GPU"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Reserve("task-test", "worker-1", tt.cpu, tt.mem, tt.gpu, 5*time.Minute)
			if err == nil {
				t.Error("Expected error for insufficient resources")
			}
		})
	}
}

// TestReserveNonexistentWorker tests reservation on a worker that doesn't exist
func TestReserveNonexistentWorker(t *testing.T) {
	registry := NewRegistry()

	err := registry.Reserve("task-1", "nonexistent-worker", 2.0, 4096, 0, 5*time.Minute)
	if err == nil {
		t.Error("Expected error for nonexistent worker")
	}
}

// TestReleaseNonexistentReservation tests releasing a reservation that doesn't exist
func TestReleaseNonexistentReservation(t *testing.T) {
	registry := NewRegistry()

	err := registry.Release("nonexistent-task")
	if err == nil {
		t.Error("Expected error for nonexistent reservation")
	}
}

// TestSubscribe tests event subscription
func TestSubscribe(t *testing.T) {
	registry := NewRegistry()

	eventChan := registry.Subscribe()

	// Register a worker
	worker := &pb.Worker{
		Id:       "worker-1",
		TotalCpu: 8.0,
		FreeCpu:  8.0,
	}

	registry.UpdateHeartbeat(worker)

	// Wait for event
	select {
	case event := <-eventChan:
		if event.Type != "worker_added" {
			t.Errorf("Expected 'worker_added' event, got '%s'", event.Type)
		}
		if event.WorkerID != "worker-1" {
			t.Errorf("Expected WorkerID='worker-1', got '%s'", event.WorkerID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event")
	}

	// Update worker
	worker.FreeCpu = 4.0
	registry.UpdateHeartbeat(worker)

	select {
	case event := <-eventChan:
		if event.Type != "worker_updated" {
			t.Errorf("Expected 'worker_updated' event, got '%s'", event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for update event")
	}
}

// TestCleanupStaleWorkers tests removing workers that haven't sent heartbeat
func TestCleanupStaleWorkers(t *testing.T) {
	registry := NewRegistry()

	// Add workers with different last seen times
	now := time.Now().Unix()

	worker1 := &pb.Worker{
		Id:           "worker-1",
		TotalCpu:     4.0,
		FreeCpu:      4.0,
		LastSeenUnix: now - 120, // 2 minutes ago
	}

	worker2 := &pb.Worker{
		Id:           "worker-2",
		TotalCpu:     8.0,
		FreeCpu:      8.0,
		LastSeenUnix: now - 30, // 30 seconds ago
	}

	registry.workers["worker-1"] = worker1
	registry.workers["worker-2"] = worker2

	// Cleanup with 60 second timeout
	removed := registry.CleanupStaleWorkers(60 * time.Second)

	if len(removed) != 1 {
		t.Errorf("Expected 1 stale worker removed, got %d", len(removed))
	}

	if removed[0] != "worker-1" {
		t.Errorf("Expected worker-1 to be removed, got %s", removed[0])
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 worker remaining, got %d", registry.Count())
	}

	// Verify only worker-2 remains
	_, err := registry.GetWorker("worker-2")
	if err != nil {
		t.Error("worker-2 should still exist")
	}

	_, err = registry.GetWorker("worker-1")
	if err == nil {
		t.Error("worker-1 should have been removed")
	}
}

// TestCleanupExpiredReservations tests removing expired reservations
func TestCleanupExpiredReservations(t *testing.T) {
	registry := NewRegistry()

	worker := &pb.Worker{
		Id:       "worker-1",
		TotalCpu: 8.0,
		FreeCpu:  8.0,
		TotalMem: 16384,
		FreeMem:  16384,
	}

	registry.UpdateHeartbeat(worker)

	// Create reservation with very short TTL
	registry.Reserve("task-1", "worker-1", 4.0, 8192, 0, 10*time.Millisecond)

	if registry.ReservationCount() != 1 {
		t.Error("Expected 1 reservation")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	expired := registry.CleanupExpiredReservations()

	if len(expired) != 1 {
		t.Errorf("Expected 1 expired reservation, got %d", len(expired))
	}

	if expired[0] != "task-1" {
		t.Errorf("Expected task-1 to be expired, got %s", expired[0])
	}

	if registry.ReservationCount() != 0 {
		t.Errorf("Expected 0 reservations after cleanup, got %d", registry.ReservationCount())
	}

	// Verify resources were returned
	w, _ := registry.GetWorker("worker-1")
	if w.FreeCpu != 8.0 {
		t.Errorf("Expected FreeCpu=8.0 after cleanup, got %.2f", w.FreeCpu)
	}
}

// TestConcurrentAccess tests thread safety
func TestConcurrentAccess(t *testing.T) {
	registry := NewRegistry()

	worker := &pb.Worker{
		Id:       "worker-1",
		TotalCpu: 100.0,
		FreeCpu:  100.0,
		TotalMem: 100000,
		FreeMem:  100000,
		Gpus:     10,
		FreeGpus: 10,
	}

	registry.UpdateHeartbeat(worker)

	// Concurrent reservations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			taskID := string(rune('a' + id))
			err := registry.Reserve(taskID, "worker-1", 5.0, 5000, 1, 5*time.Minute)
			if err != nil {
				t.Logf("Reserve %s failed: %v", taskID, err)
			}
			time.Sleep(10 * time.Millisecond)
			registry.Release(taskID)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state
	if registry.ReservationCount() != 0 {
		t.Errorf("Expected 0 reservations after all releases, got %d", registry.ReservationCount())
	}

	w, _ := registry.GetWorker("worker-1")
	if w.FreeCpu != 100.0 {
		t.Errorf("Expected all CPU returned, got %.2f", w.FreeCpu)
	}
}

// TestMultipleReservations tests multiple simultaneous reservations
func TestMultipleReservations(t *testing.T) {
	registry := NewRegistry()

	worker := &pb.Worker{
		Id:       "worker-1",
		TotalCpu: 8.0,
		FreeCpu:  8.0,
		TotalMem: 16384,
		FreeMem:  16384,
		Gpus:     2,
		FreeGpus: 2,
	}

	registry.UpdateHeartbeat(worker)

	// First reservation
	err := registry.Reserve("task-1", "worker-1", 2.0, 4096, 1, 5*time.Minute)
	if err != nil {
		t.Fatalf("First reservation failed: %v", err)
	}

	// Second reservation
	err = registry.Reserve("task-2", "worker-1", 3.0, 6144, 1, 5*time.Minute)
	if err != nil {
		t.Fatalf("Second reservation failed: %v", err)
	}

	if registry.ReservationCount() != 2 {
		t.Errorf("Expected 2 reservations, got %d", registry.ReservationCount())
	}

	// Verify cumulative resource deduction
	w, _ := registry.GetWorker("worker-1")
	if w.FreeCpu != 3.0 {
		t.Errorf("Expected FreeCpu=3.0, got %.2f", w.FreeCpu)
	}
	if w.FreeMem != 6144 {
		t.Errorf("Expected FreeMem=6144, got %d", w.FreeMem)
	}
	if w.FreeGpus != 0 {
		t.Errorf("Expected FreeGpus=0, got %d", w.FreeGpus)
	}

	// Third reservation should fail (no GPUs left)
	err = registry.Reserve("task-3", "worker-1", 1.0, 2048, 1, 5*time.Minute)
	if err == nil {
		t.Error("Expected third reservation to fail due to insufficient GPU")
	}
}
