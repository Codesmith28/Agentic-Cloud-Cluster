package telemetry

import (
	"sync"
	"testing"
)

// TestNewInMemoryTauStore tests the constructor
func TestNewInMemoryTauStore(t *testing.T) {
	store := NewInMemoryTauStore()

	if store == nil {
		t.Fatal("Expected non-nil store")
	}

	if store.lambda != 0.2 {
		t.Errorf("Expected default lambda 0.2, got %f", store.lambda)
	}

	// Verify all 6 task types are initialized
	expectedTypes := []string{
		TaskTypeCPULight,
		TaskTypeCPUHeavy,
		TaskTypeMemoryHeavy,
		TaskTypeGPUInference,
		TaskTypeGPUTraining,
		TaskTypeMixed,
	}

	for _, taskType := range expectedTypes {
		tau := store.GetTau(taskType)
		if tau <= 0 {
			t.Errorf("Task type %s should have positive default tau, got %f", taskType, tau)
		}
	}
}

// TestGetTauDefaults tests that default tau values are correct for each task type
func TestGetTauDefaults(t *testing.T) {
	store := NewInMemoryTauStore()

	tests := []struct {
		taskType    string
		expectedTau float64
	}{
		{TaskTypeCPULight, 5.0},
		{TaskTypeCPUHeavy, 15.0},
		{TaskTypeMemoryHeavy, 20.0},
		{TaskTypeGPUInference, 10.0},
		{TaskTypeGPUTraining, 60.0},
		{TaskTypeMixed, 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.taskType, func(t *testing.T) {
			tau := store.GetTau(tt.taskType)
			if tau != tt.expectedTau {
				t.Errorf("GetTau(%s) = %f, want %f", tt.taskType, tau, tt.expectedTau)
			}
		})
	}
}

// TestGetTauInvalidType tests behavior with invalid task type
func TestGetTauInvalidType(t *testing.T) {
	store := NewInMemoryTauStore()

	// Should return mixed type default for unknown types
	tau := store.GetTau("invalid-type")
	expectedDefault := defaultTauValues[TaskTypeMixed]
	if tau != expectedDefault {
		t.Errorf("GetTau(invalid) = %f, want %f (mixed default)", tau, expectedDefault)
	}
}

// TestSetTau tests explicit tau setting
func TestSetTau(t *testing.T) {
	store := NewInMemoryTauStore()

	// Set a custom tau value
	store.SetTau(TaskTypeCPULight, 7.5)

	tau := store.GetTau(TaskTypeCPULight)
	if tau != 7.5 {
		t.Errorf("After SetTau(cpu-light, 7.5), GetTau = %f, want 7.5", tau)
	}

	// Other task types should remain at defaults
	gpuTau := store.GetTau(TaskTypeGPUTraining)
	if gpuTau != 60.0 {
		t.Errorf("Other task types should retain defaults, got %f for gpu-training", gpuTau)
	}
}

// TestSetTauInvalidType tests that invalid types are ignored
func TestSetTauInvalidType(t *testing.T) {
	store := NewInMemoryTauStore()

	// Try to set tau for invalid type
	store.SetTau("invalid-type", 100.0)

	// Should not have created entry
	allTau := store.GetAllTau()
	if _, exists := allTau["invalid-type"]; exists {
		t.Error("Invalid task type should not be added to store")
	}
}

// TestSetTauInvalidValue tests that negative/zero values are ignored
func TestSetTauInvalidValue(t *testing.T) {
	store := NewInMemoryTauStore()

	originalTau := store.GetTau(TaskTypeCPULight)

	// Try to set invalid values
	store.SetTau(TaskTypeCPULight, 0)
	store.SetTau(TaskTypeCPULight, -5.0)

	// Tau should remain unchanged
	tau := store.GetTau(TaskTypeCPULight)
	if tau != originalTau {
		t.Errorf("Tau should not change with invalid values, got %f, want %f", tau, originalTau)
	}
}

// TestUpdateTauEMA tests the EMA update formula
func TestUpdateTauEMA(t *testing.T) {
	store := NewInMemoryTauStore()

	// Get initial tau for cpu-light (default 5.0)
	initialTau := store.GetTau(TaskTypeCPULight)
	if initialTau != 5.0 {
		t.Fatalf("Expected initial tau 5.0, got %f", initialTau)
	}

	// Update with actual runtime of 10.0 seconds
	actualRuntime := 10.0
	store.UpdateTau(TaskTypeCPULight, actualRuntime)

	// Calculate expected: tau_new = 0.2 * 10.0 + 0.8 * 5.0 = 2.0 + 4.0 = 6.0
	expectedTau := 0.2*actualRuntime + 0.8*initialTau

	newTau := store.GetTau(TaskTypeCPULight)
	if newTau != expectedTau {
		t.Errorf("After UpdateTau, tau = %f, want %f", newTau, expectedTau)
	}
}

// TestUpdateTauMultiple tests multiple sequential updates
func TestUpdateTauMultiple(t *testing.T) {
	store := NewInMemoryTauStore()

	taskType := TaskTypeCPUHeavy
	initialTau := store.GetTau(taskType) // 15.0

	// First update: runtime = 20.0
	store.UpdateTau(taskType, 20.0)
	tau1 := store.GetTau(taskType)
	expected1 := 0.2*20.0 + 0.8*initialTau // 4.0 + 12.0 = 16.0
	if tau1 != expected1 {
		t.Errorf("After first update, tau = %f, want %f", tau1, expected1)
	}

	// Second update: runtime = 25.0
	store.UpdateTau(taskType, 25.0)
	tau2 := store.GetTau(taskType)
	expected2 := 0.2*25.0 + 0.8*tau1 // 5.0 + 0.8*16.0 = 5.0 + 12.8 = 17.8
	if tau2 != expected2 {
		t.Errorf("After second update, tau = %f, want %f", tau2, expected2)
	}
}

// TestUpdateTauInvalidType tests that invalid types are ignored
func TestUpdateTauInvalidType(t *testing.T) {
	store := NewInMemoryTauStore()

	// Try to update invalid type
	store.UpdateTau("invalid-type", 100.0)

	// Should not have created entry
	allTau := store.GetAllTau()
	if _, exists := allTau["invalid-type"]; exists {
		t.Error("Invalid task type should not be added via UpdateTau")
	}
}

// TestUpdateTauInvalidRuntime tests that negative/zero runtimes are ignored
func TestUpdateTauInvalidRuntime(t *testing.T) {
	store := NewInMemoryTauStore()

	originalTau := store.GetTau(TaskTypeCPULight)

	// Try to update with invalid runtimes
	store.UpdateTau(TaskTypeCPULight, 0)
	store.UpdateTau(TaskTypeCPULight, -10.0)

	// Tau should remain unchanged
	tau := store.GetTau(TaskTypeCPULight)
	if tau != originalTau {
		t.Errorf("Tau should not change with invalid runtime, got %f, want %f", tau, originalTau)
	}
}

// TestConcurrentAccess tests thread safety of the store
func TestConcurrentAccess(t *testing.T) {
	store := NewInMemoryTauStore()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = store.GetTau(TaskTypeCPULight)
			}
		}()
	}

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				runtime := float64(5 + id + j%10)
				store.UpdateTau(TaskTypeCPULight, runtime)
			}
		}(i)
	}

	wg.Wait()

	// Verify store is still consistent
	tau := store.GetTau(TaskTypeCPULight)
	if tau <= 0 {
		t.Error("Tau should be positive after concurrent updates")
	}
}

// TestGetAllTau tests retrieval of all tau values
func TestGetAllTau(t *testing.T) {
	store := NewInMemoryTauStore()

	// Modify one value
	store.SetTau(TaskTypeCPULight, 7.0)

	allTau := store.GetAllTau()

	// Should have all 6 task types
	if len(allTau) != 6 {
		t.Errorf("Expected 6 task types, got %d", len(allTau))
	}

	// Check modified value
	if allTau[TaskTypeCPULight] != 7.0 {
		t.Errorf("Expected cpu-light tau = 7.0, got %f", allTau[TaskTypeCPULight])
	}

	// Verify it's a copy (modifications shouldn't affect store)
	allTau[TaskTypeCPUHeavy] = 999.0
	actualTau := store.GetTau(TaskTypeCPUHeavy)
	if actualTau == 999.0 {
		t.Error("GetAllTau should return a copy, not direct access to internal map")
	}
}

// TestNewInMemoryTauStoreWithLambda tests custom lambda constructor
func TestNewInMemoryTauStoreWithLambda(t *testing.T) {
	customLambda := 0.5
	store := NewInMemoryTauStoreWithLambda(customLambda)

	if store.GetLambda() != customLambda {
		t.Errorf("Expected lambda %f, got %f", customLambda, store.GetLambda())
	}

	// Test EMA with custom lambda
	initialTau := store.GetTau(TaskTypeCPULight) // 5.0
	store.UpdateTau(TaskTypeCPULight, 10.0)

	// With lambda=0.5: tau_new = 0.5 * 10.0 + 0.5 * 5.0 = 5.0 + 2.5 = 7.5
	expectedTau := 0.5*10.0 + 0.5*initialTau
	newTau := store.GetTau(TaskTypeCPULight)

	if newTau != expectedTau {
		t.Errorf("With lambda=0.5, tau = %f, want %f", newTau, expectedTau)
	}
}

// TestSetLambda tests lambda modification
func TestSetLambda(t *testing.T) {
	store := NewInMemoryTauStore()

	// Set new lambda
	store.SetLambda(0.3)
	if store.GetLambda() != 0.3 {
		t.Errorf("SetLambda(0.3) failed, got %f", store.GetLambda())
	}

	// Test invalid lambda values are ignored
	store.SetLambda(-0.1)
	if store.GetLambda() != 0.3 {
		t.Error("Negative lambda should be ignored")
	}

	store.SetLambda(1.5)
	if store.GetLambda() != 0.3 {
		t.Error("Lambda > 1 should be ignored")
	}
}

// TestResetToDefaults tests resetting all tau values
func TestResetToDefaults(t *testing.T) {
	store := NewInMemoryTauStore()

	// Modify all values
	store.SetTau(TaskTypeCPULight, 100.0)
	store.SetTau(TaskTypeCPUHeavy, 200.0)
	store.SetTau(TaskTypeMemoryHeavy, 300.0)

	// Verify modifications
	if store.GetTau(TaskTypeCPULight) != 100.0 {
		t.Fatal("Setup failed: tau not modified")
	}

	// Reset to defaults
	store.ResetToDefaults()

	// Verify all are back to defaults
	tests := []struct {
		taskType    string
		expectedTau float64
	}{
		{TaskTypeCPULight, 5.0},
		{TaskTypeCPUHeavy, 15.0},
		{TaskTypeMemoryHeavy, 20.0},
		{TaskTypeGPUInference, 10.0},
		{TaskTypeGPUTraining, 60.0},
		{TaskTypeMixed, 10.0},
	}

	for _, tt := range tests {
		tau := store.GetTau(tt.taskType)
		if tau != tt.expectedTau {
			t.Errorf("After ResetToDefaults, %s tau = %f, want %f", tt.taskType, tau, tt.expectedTau)
		}
	}
}

// TestIsValidTaskType tests the validation helper
func TestIsValidTaskType(t *testing.T) {
	validTypes := []string{
		TaskTypeCPULight,
		TaskTypeCPUHeavy,
		TaskTypeMemoryHeavy,
		TaskTypeGPUInference,
		TaskTypeGPUTraining,
		TaskTypeMixed,
	}

	for _, taskType := range validTypes {
		if !isValidTaskType(taskType) {
			t.Errorf("isValidTaskType(%s) = false, want true", taskType)
		}
	}

	invalidTypes := []string{
		"invalid",
		"cpu",
		"gpu",
		"",
		"CPU-LIGHT",
		"cpu_light",
	}

	for _, taskType := range invalidTypes {
		if isValidTaskType(taskType) {
			t.Errorf("isValidTaskType(%s) = true, want false", taskType)
		}
	}
}

// TestTauStoreInterface verifies InMemoryTauStore implements TauStore interface
func TestTauStoreInterface(t *testing.T) {
	var _ TauStore = (*InMemoryTauStore)(nil)
}
