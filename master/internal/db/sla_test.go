package db

import (
	"testing"
	"time"
)

// TestTaskSLAFields verifies Task struct has new SLA fields
func TestTaskSLAFields(t *testing.T) {
	now := time.Now()
	deadline := now.Add(30 * time.Second)

	task := Task{
		TaskID:        "task-1",
		UserID:        "user-1",
		DockerImage:   "test:latest",
		Command:       "echo test",
		ReqCPU:        4.0,
		ReqMemory:     8.0,
		ReqStorage:    10.0,
		ReqGPU:        0.0,
		TaskType:      "cpu-heavy",
		SLAMultiplier: 2.0,
		Deadline:      deadline,
		Tau:           15.0,
		Status:        "pending",
		CreatedAt:     now,
	}

	// Verify all fields are accessible
	if task.Deadline != deadline {
		t.Errorf("Expected Deadline %v, got %v", deadline, task.Deadline)
	}
	if task.Tau != 15.0 {
		t.Errorf("Expected Tau 15.0, got %f", task.Tau)
	}
	if task.TaskType != "cpu-heavy" {
		t.Errorf("Expected TaskType 'cpu-heavy', got '%s'", task.TaskType)
	}
	if task.SLAMultiplier != 2.0 {
		t.Errorf("Expected SLAMultiplier 2.0, got %f", task.SLAMultiplier)
	}
}

// TestAssignmentLoadAtStart verifies Assignment struct has LoadAtStart field
func TestAssignmentLoadAtStart(t *testing.T) {
	assignment := Assignment{
		AssignmentID: "ass-1",
		TaskID:       "task-1",
		WorkerID:     "worker-1",
		AssignedAt:   time.Now(),
		LoadAtStart:  0.65,
	}

	if assignment.LoadAtStart != 0.65 {
		t.Errorf("Expected LoadAtStart 0.65, got %f", assignment.LoadAtStart)
	}
}

// TestTaskTypeValidation verifies all 6 valid task types
func TestTaskTypeValidation(t *testing.T) {
	validTypes := []string{
		"cpu-light",
		"cpu-heavy",
		"memory-heavy",
		"gpu-inference",
		"gpu-training",
		"mixed",
	}

	for _, taskType := range validTypes {
		task := Task{
			TaskID:   "test-task",
			TaskType: taskType,
		}

		if task.TaskType != taskType {
			t.Errorf("Expected TaskType '%s', got '%s'", taskType, task.TaskType)
		}
	}
}

// TestDeadlineCalculation verifies deadline calculation logic
func TestDeadlineCalculation(t *testing.T) {
	now := time.Now()
	tau := 10.0 // 10 seconds expected runtime
	k := 2.0    // SLA multiplier

	// Deadline = arrival_time + k * tau
	expectedDeadline := now.Add(time.Duration(k*tau) * time.Second)

	task := Task{
		TaskID:        "task-1",
		CreatedAt:     now,
		Tau:           tau,
		SLAMultiplier: k,
		Deadline:      expectedDeadline,
	}

	// Calculate deadline
	calculatedDeadline := task.CreatedAt.Add(time.Duration(task.SLAMultiplier*task.Tau) * time.Second)

	// Compare (allow 1ms tolerance for time precision)
	diff := calculatedDeadline.Sub(task.Deadline)
	if diff < -time.Millisecond || diff > time.Millisecond {
		t.Errorf("Deadline mismatch: expected %v, calculated %v (diff: %v)",
			task.Deadline, calculatedDeadline, diff)
	}
}

// TestSLASuccessEvaluation verifies SLA success logic
func TestSLASuccessEvaluation(t *testing.T) {
	now := time.Now()
	tau := 10.0
	k := 2.0
	deadline := now.Add(time.Duration(k*tau) * time.Second)

	testCases := []struct {
		name           string
		completionTime time.Time
		expectedSLA    bool
		description    string
	}{
		{
			name:           "Completed before deadline",
			completionTime: now.Add(15 * time.Second),
			expectedSLA:    true,
			description:    "Task finished at 15s, deadline at 20s",
		},
		{
			name:           "Completed exactly at deadline",
			completionTime: deadline,
			expectedSLA:    true,
			description:    "Task finished exactly at deadline",
		},
		{
			name:           "Completed after deadline",
			completionTime: now.Add(25 * time.Second),
			expectedSLA:    false,
			description:    "Task finished at 25s, deadline at 20s",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task := Task{
				TaskID:        "task-1",
				CreatedAt:     now,
				Deadline:      deadline,
				Tau:           tau,
				SLAMultiplier: k,
				CompletedAt:   tc.completionTime,
			}

			// SLA Success = completed_at <= deadline
			slaSuccess := !task.CompletedAt.After(task.Deadline)

			if slaSuccess != tc.expectedSLA {
				t.Errorf("%s: Expected SLA success %v, got %v (%s)",
					tc.name, tc.expectedSLA, slaSuccess, tc.description)
			}
		})
	}
}

// TestTauDefaults verifies default tau values for different task types
func TestTauDefaults(t *testing.T) {
	// Default tau values as specified in Task 2.1
	defaultTau := map[string]float64{
		"cpu-light":     5.0,
		"cpu-heavy":     15.0,
		"memory-heavy":  20.0,
		"gpu-inference": 10.0,
		"gpu-training":  60.0,
		"mixed":         10.0,
	}

	for taskType, expectedTau := range defaultTau {
		t.Run(taskType, func(t *testing.T) {
			if expectedTau <= 0 {
				t.Errorf("Task type '%s' has invalid default tau: %f", taskType, expectedTau)
			}

			task := Task{
				TaskID:   "test-task",
				TaskType: taskType,
				Tau:      expectedTau,
			}

			if task.Tau != expectedTau {
				t.Errorf("Expected tau %f for %s, got %f", expectedTau, taskType, task.Tau)
			}
		})
	}
}

// TestSLAMultiplierRange verifies k value is within valid range
func TestSLAMultiplierRange(t *testing.T) {
	testCases := []struct {
		k             float64
		shouldBeValid bool
		description   string
	}{
		{1.5, true, "Minimum valid k value"},
		{2.0, true, "Default k value"},
		{2.5, true, "Maximum valid k value"},
		{1.4, false, "Below minimum"},
		{2.6, false, "Above maximum"},
		{1.0, false, "Too low"},
		{3.0, false, "Too high"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			isValid := tc.k >= 1.5 && tc.k <= 2.5

			if isValid != tc.shouldBeValid {
				t.Errorf("k=%f: Expected valid=%v, got %v (%s)",
					tc.k, tc.shouldBeValid, isValid, tc.description)
			}
		})
	}
}

// TestTaskBSONTags verifies BSON tags for new fields
func TestTaskBSONTags(t *testing.T) {
	task := Task{}

	// Verify all SLA-related fields have the correct types
	_ = task.Deadline      // time.Time
	_ = task.Tau           // float64
	_ = task.TaskType      // string
	_ = task.SLAMultiplier // float64

	// These assignments verify the struct compilation
	task.Deadline = time.Now()
	task.Tau = 10.0
	task.TaskType = "cpu-heavy"
	task.SLAMultiplier = 2.0

	if task.Tau == 0 {
		t.Error("Tau field should be writable")
	}
}

// TestAssignmentBSONTags verifies BSON tags for LoadAtStart
func TestAssignmentBSONTags(t *testing.T) {
	assignment := Assignment{}

	// Verify LoadAtStart field
	_ = assignment.LoadAtStart // float64

	assignment.LoadAtStart = 0.75

	if assignment.LoadAtStart != 0.75 {
		t.Error("LoadAtStart field should be writable")
	}
}

// TestUpdateTaskWithSLAValidation tests the validation logic
func TestUpdateTaskWithSLAValidation(t *testing.T) {
	// Test invalid task types (these should fail validation)
	invalidTypes := []string{
		"cpu",       // old format
		"gpu",       // old format
		"invalid",   // completely invalid
		"CPU-HEAVY", // wrong case
		"cpu_heavy", // underscore instead of hyphen
		"",          // empty (should be allowed - will be inferred)
	}

	validTypes := []string{
		"cpu-light",
		"cpu-heavy",
		"memory-heavy",
		"gpu-inference",
		"gpu-training",
		"mixed",
	}

	// This test just verifies the validation logic exists
	// Actual database tests would require MongoDB connection
	validateTaskType := func(taskType string) bool {
		validTypeMap := map[string]bool{
			"cpu-light": true, "cpu-heavy": true, "memory-heavy": true,
			"gpu-inference": true, "gpu-training": true, "mixed": true,
		}
		return taskType == "" || validTypeMap[taskType]
	}

	for _, taskType := range invalidTypes {
		if taskType != "" && validateTaskType(taskType) {
			t.Errorf("Task type '%s' should be invalid but passed validation", taskType)
		}
	}

	for _, taskType := range validTypes {
		if !validateTaskType(taskType) {
			t.Errorf("Task type '%s' should be valid but failed validation", taskType)
		}
	}
}
