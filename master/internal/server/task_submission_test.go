package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"master/internal/telemetry"
	pb "master/proto"
)

// MockTauStore implements TauStore for testing
type MockTauStore struct {
	tauValues map[string]float64
	getCalls  map[string]int
	mu        sync.RWMutex // Add mutex for thread safety
}

func NewMockTauStore() *MockTauStore {
	return &MockTauStore{
		tauValues: map[string]float64{
			"cpu-light":     5.0,
			"cpu-heavy":     15.0,
			"memory-heavy":  20.0,
			"gpu-inference": 10.0,
			"gpu-training":  60.0,
			"mixed":         10.0,
		},
		getCalls: make(map[string]int),
	}
}

func (m *MockTauStore) GetTau(taskType string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.getCalls[taskType]++
	if tau, exists := m.tauValues[taskType]; exists {
		return tau
	}
	return 10.0 // default
}

func (m *MockTauStore) UpdateTau(taskType string, actualRuntime float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Apply EMA formula: tau_new = 0.2 * actualRuntime + 0.8 * tau_old
	lambda := 0.2

	// Get old tau without calling GetTau to avoid deadlock
	oldTau := m.tauValues[taskType]
	if oldTau == 0 {
		// Use default if not found
		defaultTaus := map[string]float64{
			"cpu-light":     5.0,
			"cpu-heavy":     15.0,
			"memory-heavy":  20.0,
			"gpu-inference": 10.0,
			"gpu-training":  60.0,
			"mixed":         10.0,
		}
		if val, ok := defaultTaus[taskType]; ok {
			oldTau = val
		} else {
			oldTau = 10.0
		}
	}

	newTau := lambda*actualRuntime + (1-lambda)*oldTau
	m.tauValues[taskType] = newTau
}

func (m *MockTauStore) SetTau(taskType string, tau float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tauValues[taskType] = tau
}

// TestTaskSubmissionWithTauStore tests that SubmitTask uses tau store correctly
func TestTaskSubmissionWithTauStore(t *testing.T) {
	// Create mock tau store
	mockTauStore := NewMockTauStore()
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)

	// Create server with tau store
	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	ctx := context.Background()

	// Test task with explicit task type
	task := &pb.Task{
		TaskId:      "test-task-1",
		DockerImage: "alpine:latest",
		Command:     "echo test",
		ReqCpu:      2.0,
		ReqMemory:   4.0,
		TaskType:    "cpu-light",
	}

	ack, err := server.SubmitTask(ctx, task)

	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	if !ack.Success {
		t.Errorf("Expected success=true, got false: %s", ack.Message)
	}

	// Verify tau store was called for cpu-light
	if mockTauStore.getCalls["cpu-light"] != 1 {
		t.Errorf("Expected GetTau to be called once for cpu-light, got %d calls",
			mockTauStore.getCalls["cpu-light"])
	}

	// Verify message contains tau information
	if ack.Message == "" {
		t.Error("Expected non-empty message")
	}
}

// TestTaskSubmissionWithInference tests task type inference
func TestTaskSubmissionWithInference(t *testing.T) {
	mockTauStore := NewMockTauStore()
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	ctx := context.Background()

	tests := []struct {
		name             string
		task             *pb.Task
		expectedTauType  string
		expectedTauValue float64
	}{
		{
			name: "CPU-heavy task",
			task: &pb.Task{
				TaskId:      "cpu-heavy-task",
				DockerImage: "alpine:latest",
				ReqCpu:      8.0,
				ReqMemory:   4.0,
				ReqGpu:      0.0,
				// TaskType empty - should infer
			},
			expectedTauType:  "cpu-heavy",
			expectedTauValue: 15.0,
		},
		{
			name: "GPU-training task",
			task: &pb.Task{
				TaskId:      "gpu-train-task",
				DockerImage: "tensorflow:latest",
				ReqCpu:      8.0,
				ReqMemory:   16.0,
				ReqGpu:      4.0,
				// TaskType empty - should infer
			},
			expectedTauType:  "gpu-training",
			expectedTauValue: 60.0,
		},
		{
			name: "Memory-heavy task",
			task: &pb.Task{
				TaskId:      "mem-heavy-task",
				DockerImage: "redis:latest",
				ReqCpu:      2.0,
				ReqMemory:   16.0,
				ReqGpu:      0.0,
				// TaskType empty - should infer
			},
			expectedTauType:  "memory-heavy",
			expectedTauValue: 20.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset call counts
			mockTauStore.getCalls = make(map[string]int)

			ack, err := server.SubmitTask(ctx, tt.task)

			if err != nil {
				t.Fatalf("SubmitTask failed: %v", err)
			}

			if !ack.Success {
				t.Errorf("Expected success, got: %s", ack.Message)
			}

			// Verify tau store was called for the inferred type
			if mockTauStore.getCalls[tt.expectedTauType] != 1 {
				t.Errorf("Expected GetTau(%s) to be called once, got %d calls",
					tt.expectedTauType, mockTauStore.getCalls[tt.expectedTauType])
			}
		})
	}
}

// TestTaskSubmissionWithInvalidTaskType tests validation of invalid task types
func TestTaskSubmissionWithInvalidTaskType(t *testing.T) {
	mockTauStore := NewMockTauStore()
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	ctx := context.Background()

	// Task with invalid type - should fall back to inference
	task := &pb.Task{
		TaskId:      "invalid-type-task",
		DockerImage: "alpine:latest",
		ReqCpu:      2.0,
		ReqMemory:   4.0,
		TaskType:    "invalid-type", // Invalid
	}

	ack, err := server.SubmitTask(ctx, task)

	if err != nil {
		t.Fatalf("SubmitTask should not fail with invalid type: %v", err)
	}

	if !ack.Success {
		t.Errorf("Expected success even with invalid type, got: %s", ack.Message)
	}

	// Should have inferred cpu-light (ReqCpu=2.0, no GPU)
	if mockTauStore.getCalls["cpu-light"] != 1 {
		t.Errorf("Expected GetTau(cpu-light) after invalid type, got calls: %v",
			mockTauStore.getCalls)
	}
}

// TestSLAMultiplierValidation tests SLA multiplier validation
func TestSLAMultiplierValidation(t *testing.T) {
	mockTauStore := NewMockTauStore()
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)

	tests := []struct {
		name                string
		serverSLAMultiplier float64
		taskSLAMultiplier   float64
		expectedMultiplier  float64
	}{
		{
			name:                "Valid task multiplier",
			serverSLAMultiplier: 2.0,
			taskSLAMultiplier:   1.8,
			expectedMultiplier:  1.8,
		},
		{
			name:                "Invalid task multiplier (too low)",
			serverSLAMultiplier: 2.0,
			taskSLAMultiplier:   1.0,
			expectedMultiplier:  2.0, // Falls back to server default
		},
		{
			name:                "Invalid task multiplier (too high)",
			serverSLAMultiplier: 2.0,
			taskSLAMultiplier:   3.0,
			expectedMultiplier:  2.0, // Falls back to server default
		},
		{
			name:                "Task multiplier not set",
			serverSLAMultiplier: 1.5,
			taskSLAMultiplier:   0.0,
			expectedMultiplier:  1.5, // Uses server default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, tt.serverSLAMultiplier)

			task := &pb.Task{
				TaskId:        "sla-test-task",
				DockerImage:   "alpine:latest",
				ReqCpu:        2.0,
				TaskType:      "cpu-light",
				SlaMultiplier: tt.taskSLAMultiplier,
			}

			ctx := context.Background()
			ack, err := server.SubmitTask(ctx, task)

			if err != nil {
				t.Fatalf("SubmitTask failed: %v", err)
			}

			if !ack.Success {
				t.Errorf("Expected success, got: %s", ack.Message)
			}

			// We can't directly check the multiplier used, but we can verify success
			// In a real scenario, we'd check the database or deadline
		})
	}
}

// TestTaskSubmissionWithDatabase tests integration with TaskDB
func TestTaskSubmissionWithDatabase(t *testing.T) {
	// Note: This test requires a mock or test database
	// For now, we test that missing DB doesn't break submission

	mockTauStore := NewMockTauStore()
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	ctx := context.Background()

	task := &pb.Task{
		TaskId:      "db-test-task",
		DockerImage: "alpine:latest",
		ReqCpu:      2.0,
		TaskType:    "cpu-light",
	}

	// Should succeed even without DB
	ack, err := server.SubmitTask(ctx, task)

	if err != nil {
		t.Fatalf("SubmitTask should work without DB: %v", err)
	}

	if !ack.Success {
		t.Errorf("Expected success without DB, got: %s", ack.Message)
	}
}

// TestDeadlineComputation tests that deadline is computed correctly
func TestDeadlineComputation(t *testing.T) {
	// This test verifies the deadline computation logic indirectly
	// by ensuring tau store is called and task succeeds

	mockTauStore := NewMockTauStore()
	mockTauStore.SetTau("cpu-heavy", 20.0) // Set specific tau

	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	ctx := context.Background()

	task := &pb.Task{
		TaskId:      "deadline-test-task",
		DockerImage: "alpine:latest",
		ReqCpu:      8.0,
		TaskType:    "cpu-heavy",
	}

	beforeSubmit := time.Now()
	ack, err := server.SubmitTask(ctx, task)
	afterSubmit := time.Now()

	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	if !ack.Success {
		t.Errorf("Expected success, got: %s", ack.Message)
	}

	// Verify tau was retrieved
	if mockTauStore.getCalls["cpu-heavy"] != 1 {
		t.Error("Expected GetTau(cpu-heavy) to be called")
	}

	// Deadline should be: now + 2.0 * 20.0 = now + 40 seconds
	// We can't check exact deadline without DB, but we verified the logic runs

	// Verify timing is reasonable
	elapsed := afterSubmit.Sub(beforeSubmit)
	if elapsed > 1*time.Second {
		t.Errorf("SubmitTask took too long: %v", elapsed)
	}
}

// TestConcurrentSubmissions tests thread safety
func TestConcurrentSubmissions(t *testing.T) {
	mockTauStore := NewMockTauStore()
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	ctx := context.Background()

	// Submit multiple tasks concurrently
	numTasks := 10
	done := make(chan bool, numTasks)

	for i := 0; i < numTasks; i++ {
		go func(id int) {
			task := &pb.Task{
				TaskId:      "concurrent-task-" + string(rune(id)),
				DockerImage: "alpine:latest",
				ReqCpu:      2.0,
				TaskType:    "cpu-light",
			}

			_, err := server.SubmitTask(ctx, task)
			if err != nil {
				t.Errorf("Concurrent submission %d failed: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all submissions
	for i := 0; i < numTasks; i++ {
		<-done
	}

	// All tasks should have succeeded without race conditions
}

// TestValidTaskTypePreserved tests that valid explicit task types are preserved
func TestValidTaskTypePreserved(t *testing.T) {
	mockTauStore := NewMockTauStore()
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	ctx := context.Background()

	validTypes := []string{
		"cpu-light",
		"cpu-heavy",
		"memory-heavy",
		"gpu-inference",
		"gpu-training",
		"mixed",
	}

	for _, taskType := range validTypes {
		t.Run(taskType, func(t *testing.T) {
			mockTauStore.getCalls = make(map[string]int)

			task := &pb.Task{
				TaskId:      "preserve-" + taskType,
				DockerImage: "alpine:latest",
				ReqCpu:      2.0,
				TaskType:    taskType, // Explicit type
			}

			ack, err := server.SubmitTask(ctx, task)

			if err != nil {
				t.Fatalf("SubmitTask failed for %s: %v", taskType, err)
			}

			if !ack.Success {
				t.Errorf("Expected success for %s, got: %s", taskType, ack.Message)
			}

			// Verify the explicit type was used (not inferred)
			if mockTauStore.getCalls[taskType] != 1 {
				t.Errorf("Expected GetTau(%s) to be called, got calls: %v",
					taskType, mockTauStore.getCalls)
			}
		})
	}
}
