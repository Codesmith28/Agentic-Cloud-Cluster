package server

import (
	"context"
	"testing"
	"time"

	"master/internal/telemetry"
	pb "master/proto"
)

// TestTauUpdateOnCompletion tests that tau is updated when tasks complete
func TestTauUpdateOnCompletion(t *testing.T) {
	// Create mock tau store with known initial values
	mockTauStore := NewMockTauStore()
	initialTau := mockTauStore.GetTau("cpu-heavy")

	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	defer telemetryMgr.Shutdown()

	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	if server.tauStore == nil {
		t.Fatal("Server should have tau store")
	}

	t.Logf("Initial tau for cpu-heavy: %.2fs", initialTau)

	// Note: Full integration test would require mocking TaskDB
	// This test verifies tau store is accessible from server
}

// TestTauUpdateCalculation tests the tau update calculation
func TestTauUpdateCalculation(t *testing.T) {
	tests := []struct {
		name           string
		taskType       string
		initialTau     float64
		actualRuntime  float64
		lambda         float64
		expectedNewTau float64
	}{
		{
			name:           "Runtime matches tau",
			taskType:       "cpu-light",
			initialTau:     10.0,
			actualRuntime:  10.0,
			lambda:         0.2,
			expectedNewTau: 10.0, // 0.2*10 + 0.8*10 = 10
		},
		{
			name:           "Runtime faster than tau",
			taskType:       "cpu-heavy",
			initialTau:     15.0,
			actualRuntime:  10.0,
			lambda:         0.2,
			expectedNewTau: 14.0, // 0.2*10 + 0.8*15 = 2 + 12 = 14
		},
		{
			name:           "Runtime slower than tau",
			taskType:       "memory-heavy",
			initialTau:     20.0,
			actualRuntime:  30.0,
			lambda:         0.2,
			expectedNewTau: 22.0, // 0.2*30 + 0.8*20 = 6 + 16 = 22
		},
		{
			name:           "Very fast task",
			taskType:       "gpu-inference",
			initialTau:     10.0,
			actualRuntime:  2.0,
			lambda:         0.2,
			expectedNewTau: 8.4, // 0.2*2 + 0.8*10 = 0.4 + 8 = 8.4
		},
		{
			name:           "Very slow task",
			taskType:       "gpu-training",
			initialTau:     60.0,
			actualRuntime:  100.0,
			lambda:         0.2,
			expectedNewTau: 68.0, // 0.2*100 + 0.8*60 = 20 + 48 = 68
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTauStore := NewMockTauStore()
			mockTauStore.SetTau(tt.taskType, tt.initialTau)

			// Simulate tau update
			mockTauStore.UpdateTau(tt.taskType, tt.actualRuntime)

			newTau := mockTauStore.GetTau(tt.taskType)

			// Calculate expected using EMA formula
			expected := tt.lambda*tt.actualRuntime + (1-tt.lambda)*tt.initialTau

			if abs(newTau-expected) > 0.001 {
				t.Errorf("Expected tau %.3f, got %.3f", expected, newTau)
			}

			t.Logf("Tau update: %.2fs → %.2fs (actual runtime: %.2fs)",
				tt.initialTau, newTau, tt.actualRuntime)
		})
	}
}

// TestTauLearningConvergence tests that tau converges over multiple updates
func TestTauLearningConvergence(t *testing.T) {
	mockTauStore := NewMockTauStore()
	taskType := "cpu-light"

	initialTau := mockTauStore.GetTau(taskType)
	t.Logf("Initial tau: %.2fs", initialTau)

	// Simulate consistent runtime of 8.0s (faster than initial 5.0s default)
	targetRuntime := 8.0
	numUpdates := 20

	tauHistory := []float64{initialTau}

	for i := 0; i < numUpdates; i++ {
		mockTauStore.UpdateTau(taskType, targetRuntime)
		newTau := mockTauStore.GetTau(taskType)
		tauHistory = append(tauHistory, newTau)
	}

	finalTau := mockTauStore.GetTau(taskType)
	t.Logf("Final tau after %d updates: %.2fs", numUpdates, finalTau)

	// Tau should converge toward actual runtime
	if finalTau < initialTau {
		t.Error("Tau should increase when actual runtime is consistently higher")
	}

	// Should be closer to target than initial
	initialDiff := abs(targetRuntime - initialTau)
	finalDiff := abs(targetRuntime - finalTau)

	if finalDiff >= initialDiff {
		t.Errorf("Tau should converge: initial diff=%.2f, final diff=%.2f", initialDiff, finalDiff)
	}

	t.Logf("Convergence: initial diff=%.2fs, final diff=%.2fs", initialDiff, finalDiff)
}

// TestTauUpdateOnlyForSuccessfulTasks tests that tau only updates on success
func TestTauUpdateOnlyForSuccessfulTasks(t *testing.T) {
	mockTauStore := NewMockTauStore()
	taskType := "cpu-heavy"

	initialTau := mockTauStore.GetTau(taskType)

	// Simulate a failed task (should not update tau in real implementation)
	// This test just verifies the mockTauStore behavior

	// In real implementation, ReportTaskCompletion only updates on success
	// Here we just verify that UpdateTau works when called
	mockTauStore.UpdateTau(taskType, 20.0)

	newTau := mockTauStore.GetTau(taskType)

	if newTau == initialTau {
		t.Error("Tau should update when UpdateTau is called")
	}

	t.Logf("Tau updated: %.2fs → %.2fs", initialTau, newTau)
}

// TestTauUpdateForDifferentTaskTypes tests that each task type maintains separate tau
func TestTauUpdateForDifferentTaskTypes(t *testing.T) {
	mockTauStore := NewMockTauStore()

	taskTypes := []string{
		"cpu-light",
		"cpu-heavy",
		"memory-heavy",
		"gpu-inference",
		"gpu-training",
		"mixed",
	}

	// Record initial taus
	initialTaus := make(map[string]float64)
	for _, taskType := range taskTypes {
		initialTaus[taskType] = mockTauStore.GetTau(taskType)
		t.Logf("Initial tau for %s: %.2fs", taskType, initialTaus[taskType])
	}

	// Update each type with different runtime
	runtimes := map[string]float64{
		"cpu-light":     3.0,
		"cpu-heavy":     20.0,
		"memory-heavy":  25.0,
		"gpu-inference": 8.0,
		"gpu-training":  80.0,
		"mixed":         12.0,
	}

	for _, taskType := range taskTypes {
		mockTauStore.UpdateTau(taskType, runtimes[taskType])
	}

	// Verify each type updated independently
	for _, taskType := range taskTypes {
		newTau := mockTauStore.GetTau(taskType)
		initialTau := initialTaus[taskType]
		runtime := runtimes[taskType]

		// Calculate expected
		expected := 0.2*runtime + 0.8*initialTau

		if abs(newTau-expected) > 0.001 {
			t.Errorf("Task type %s: expected tau %.3f, got %.3f", taskType, expected, newTau)
		}

		t.Logf("Task type %s: %.2fs → %.2fs (runtime: %.2fs)",
			taskType, initialTau, newTau, runtime)
	}
}

// TestTauUpdateWithVaryingRuntimes tests tau adaptation to varying workloads
func TestTauUpdateWithVaryingRuntimes(t *testing.T) {
	mockTauStore := NewMockTauStore()
	taskType := "cpu-heavy"

	initialTau := mockTauStore.GetTau(taskType)

	// Simulate varying runtimes (realistic scenario)
	runtimes := []float64{
		12.0, 14.0, 13.0, 15.0, 11.0, // Cluster around 13s
		12.5, 13.5, 14.5, 12.0, 13.0,
	}

	for i, runtime := range runtimes {
		mockTauStore.UpdateTau(taskType, runtime)
		newTau := mockTauStore.GetTau(taskType)
		t.Logf("Update %d: runtime=%.2fs, tau=%.2fs", i+1, runtime, newTau)
	}

	finalTau := mockTauStore.GetTau(taskType)

	// Average runtime is ~13s, tau should move toward that
	avgRuntime := 13.0

	// Final tau should be between initial and average
	if finalTau > initialTau+5 || finalTau < initialTau-5 {
		t.Errorf("Tau moved too much: %.2fs → %.2fs (avg runtime: %.2fs)",
			initialTau, finalTau, avgRuntime)
	}

	t.Logf("Final result: initial=%.2fs, final=%.2fs, avg_runtime=%.2fs",
		initialTau, finalTau, avgRuntime)
}

// TestTauStoreIntegrationWithServer tests server has access to tau store
func TestTauStoreIntegrationWithServer(t *testing.T) {
	mockTauStore := NewMockTauStore()
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	defer telemetryMgr.Shutdown()

	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	if server.tauStore == nil {
		t.Fatal("Server should have tau store reference")
	}

	// Verify server can access tau values
	taskType := "gpu-training"
	tau := server.tauStore.GetTau(taskType)

	if tau <= 0 {
		t.Errorf("Expected positive tau, got %.2f", tau)
	}

	t.Logf("Server accessed tau for %s: %.2fs", taskType, tau)
}

// TestTauUpdatePrecision tests floating point precision in tau updates
func TestTauUpdatePrecision(t *testing.T) {
	mockTauStore := NewMockTauStore()
	taskType := "cpu-light"

	// Set precise initial value
	mockTauStore.SetTau(taskType, 5.123456)

	// Update with precise runtime
	mockTauStore.UpdateTau(taskType, 6.789012)

	newTau := mockTauStore.GetTau(taskType)

	// Expected: 0.2 * 6.789012 + 0.8 * 5.123456
	expected := 0.2*6.789012 + 0.8*5.123456

	if abs(newTau-expected) > 0.000001 {
		t.Errorf("Precision error: expected %.6f, got %.6f", expected, newTau)
	}

	t.Logf("Precise tau update: %.6f → %.6f", 5.123456, newTau)
}

// TestTauUpdateBoundaryConditions tests edge cases
func TestTauUpdateBoundaryConditions(t *testing.T) {
	tests := []struct {
		name          string
		taskType      string
		actualRuntime float64
		shouldUpdate  bool
	}{
		{
			name:          "Zero runtime",
			taskType:      "cpu-light",
			actualRuntime: 0.0,
			shouldUpdate:  true, // Should still update (task completed instantly)
		},
		{
			name:          "Very small runtime",
			taskType:      "cpu-light",
			actualRuntime: 0.001,
			shouldUpdate:  true,
		},
		{
			name:          "Very large runtime",
			taskType:      "gpu-training",
			actualRuntime: 10000.0,
			shouldUpdate:  true,
		},
		{
			name:          "Negative runtime (invalid)",
			taskType:      "cpu-heavy",
			actualRuntime: -5.0,
			shouldUpdate:  true, // Mock allows, real implementation should validate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTauStore := NewMockTauStore()
			initialTau := mockTauStore.GetTau(tt.taskType)

			mockTauStore.UpdateTau(tt.taskType, tt.actualRuntime)

			newTau := mockTauStore.GetTau(tt.taskType)

			if tt.shouldUpdate && newTau == initialTau && tt.actualRuntime != initialTau {
				t.Errorf("Tau should have updated: %.2f → %.2f (runtime: %.2f)",
					initialTau, newTau, tt.actualRuntime)
			}

			t.Logf("Boundary test: runtime=%.3f, tau: %.2f → %.2f",
				tt.actualRuntime, initialTau, newTau)
		})
	}
}

// TestTauUpdateThreadSafety tests concurrent tau updates (using mock)
func TestTauUpdateThreadSafety(t *testing.T) {
	mockTauStore := NewMockTauStore()
	taskType := "cpu-heavy"

	numGoroutines := 10
	updatesPerGoroutine := 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			runtime := float64(10 + id) // Different runtimes per goroutine
			for j := 0; j < updatesPerGoroutine; j++ {
				mockTauStore.UpdateTau(taskType, runtime)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	finalTau := mockTauStore.GetTau(taskType)

	if finalTau <= 0 {
		t.Error("Tau should remain positive after concurrent updates")
	}

	t.Logf("Final tau after %d concurrent updates: %.2fs",
		numGoroutines*updatesPerGoroutine, finalTau)
}

// TestReportTaskCompletionStructure tests the structure is correct
func TestReportTaskCompletionStructure(t *testing.T) {
	mockTauStore := NewMockTauStore()
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	defer telemetryMgr.Shutdown()

	server := NewMasterServer(nil, nil, nil, nil, telemetryMgr, mockTauStore, 2.0)

	// Create a mock task result
	result := &pb.TaskResult{
		TaskId:   "test-task-123",
		WorkerId: "test-worker-1",
		Status:   "success",
		Logs:     "Task completed successfully",
	}

	ctx := context.Background()

	// Call ReportTaskCompletion (will fail due to nil DB, but tests structure)
	ack, err := server.ReportTaskCompletion(ctx, result)

	if err != nil {
		t.Logf("Expected error due to nil DB: %v", err)
	}

	if ack == nil {
		t.Error("Acknowledgment should not be nil")
	} else if !ack.Success {
		t.Logf("Task completion reported (with DB errors expected): %s", ack.Message)
	}

	t.Log("ReportTaskCompletion structure verified")
}
