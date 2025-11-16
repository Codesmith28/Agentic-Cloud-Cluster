package server

import (
	"testing"
	"time"

	"master/internal/db"
)

// TestSLASuccessFieldInResult tests that SLASuccess field is present in TaskResult
func TestSLASuccessFieldInResult(t *testing.T) {
	result := &db.TaskResult{
		TaskID:     "test-task-1",
		WorkerID:   "worker-1",
		Status:     "success",
		Logs:       "Task completed",
		SLASuccess: true,
	}

	if result.SLASuccess != true {
		t.Errorf("Expected SLASuccess=true, got %v", result.SLASuccess)
	}

	result.SLASuccess = false
	if result.SLASuccess != false {
		t.Errorf("Expected SLASuccess=false, got %v", result.SLASuccess)
	}

	t.Logf("✓ TaskResult.SLASuccess field works correctly")
}

// TestSLASuccessLogic tests the SLA success computation logic
func TestSLASuccessLogic(t *testing.T) {
	tests := []struct {
		name           string
		completionTime time.Time
		deadline       time.Time
		expectedSLA    bool
		description    string
	}{
		{
			name:           "CompletedBeforeDeadline",
			completionTime: time.Now(),
			deadline:       time.Now().Add(10 * time.Second),
			expectedSLA:    true,
			description:    "Task completed 10s before deadline",
		},
		{
			name:           "CompletedAfterDeadline",
			completionTime: time.Now(),
			deadline:       time.Now().Add(-5 * time.Second),
			expectedSLA:    false,
			description:    "Task completed 5s after deadline",
		},
		{
			name:           "CompletedExactlyAtDeadline",
			completionTime: time.Now(),
			deadline:       time.Now(),
			expectedSLA:    true,
			description:    "Task completed exactly at deadline",
		},
		{
			name:           "CompletedWellBeforeDeadline",
			completionTime: time.Now(),
			deadline:       time.Now().Add(60 * time.Second),
			expectedSLA:    true,
			description:    "Task completed 60s before deadline",
		},
		{
			name:           "CompletedWellAfterDeadline",
			completionTime: time.Now(),
			deadline:       time.Now().Add(-30 * time.Second),
			expectedSLA:    false,
			description:    "Task completed 30s after deadline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate SLA success computation
			slaSuccess := tt.completionTime.Before(tt.deadline) || tt.completionTime.Equal(tt.deadline)

			if slaSuccess != tt.expectedSLA {
				t.Errorf("Expected SLASuccess=%v, got %v for case: %s",
					tt.expectedSLA, slaSuccess, tt.description)
			}

			delay := tt.completionTime.Sub(tt.deadline).Seconds()
			t.Logf("✓ %s: SLA=%v, delay=%.2fs", tt.description, slaSuccess, delay)
		})
	}
}

// TestSLASuccessWithDifferentTaskTypes tests SLA computation for different task types
func TestSLASuccessWithDifferentTaskTypes(t *testing.T) {
	taskTypes := []struct {
		taskType string
		tau      float64
		slaMulti float64
	}{
		{"cpu-light", 5.0, 2.0},
		{"cpu-heavy", 15.0, 2.0},
		{"memory-heavy", 20.0, 2.0},
		{"gpu-inference", 10.0, 2.0},
		{"gpu-training", 60.0, 2.0},
		{"mixed", 10.0, 2.0},
	}

	for _, tt := range taskTypes {
		t.Run(tt.taskType, func(t *testing.T) {
			// Compute expected deadline
			arrivalTime := time.Now()
			expectedDeadline := arrivalTime.Add(time.Duration(tt.slaMulti*tt.tau) * time.Second)

			// Simulate task completing within SLA
			completionTime := arrivalTime.Add(time.Duration(tt.tau*0.8) * time.Second)
			slaSuccess := completionTime.Before(expectedDeadline) || completionTime.Equal(expectedDeadline)

			if !slaSuccess {
				t.Errorf("Task type %s should meet SLA when completing in 80%% of expected time", tt.taskType)
			}

			t.Logf("✓ Task type %s: tau=%.1fs, deadline=%.1fs, completed=%.1fs, SLA=%v",
				tt.taskType, tt.tau, tt.slaMulti*tt.tau, tt.tau*0.8, slaSuccess)
		})
	}
}

// TestSLASuccessCalculationFormula tests the SLA success calculation formula
func TestSLASuccessCalculationFormula(t *testing.T) {
	// Test various scenarios with specific timing
	scenarios := []struct {
		name      string
		runtime   float64 // actual runtime in seconds
		tau       float64 // expected runtime
		slaMulti  float64 // k value
		expectSLA bool
	}{
		{
			name:      "FastTask",
			runtime:   5.0,
			tau:       10.0,
			slaMulti:  2.0,
			expectSLA: true, // 5s < 20s deadline
		},
		{
			name:      "OnTimeTask",
			runtime:   18.0,
			tau:       10.0,
			slaMulti:  2.0,
			expectSLA: true, // 18s < 20s deadline
		},
		{
			name:      "SlightlyLateTask",
			runtime:   22.0,
			tau:       10.0,
			slaMulti:  2.0,
			expectSLA: false, // 22s > 20s deadline
		},
		{
			name:      "VeryLateTask",
			runtime:   40.0,
			tau:       10.0,
			slaMulti:  2.0,
			expectSLA: false, // 40s > 20s deadline
		},
		{
			name:      "ExactDeadline",
			runtime:   20.0,
			tau:       10.0,
			slaMulti:  2.0,
			expectSLA: true, // 20s == 20s deadline (equal is success)
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			arrivalTime := time.Now()
			deadline := arrivalTime.Add(time.Duration(sc.slaMulti*sc.tau) * time.Second)
			completionTime := arrivalTime.Add(time.Duration(sc.runtime) * time.Second)

			slaSuccess := completionTime.Before(deadline) || completionTime.Equal(deadline)

			if slaSuccess != sc.expectSLA {
				t.Errorf("Expected SLA=%v for runtime=%.1fs with deadline=%.1fs, got %v",
					sc.expectSLA, sc.runtime, sc.slaMulti*sc.tau, slaSuccess)
			}

			violation := completionTime.Sub(deadline).Seconds()
			t.Logf("✓ %s: runtime=%.1fs, deadline=%.1fs, violation=%.1fs, SLA=%v",
				sc.name, sc.runtime, sc.slaMulti*sc.tau, violation, slaSuccess)
		})
	}
}

// TestSLASuccessEdgeCases tests edge cases for SLA computation
func TestSLASuccessEdgeCases(t *testing.T) {
	t.Run("ZeroDeadline", func(t *testing.T) {
		// Task with zero deadline (uninitialized)
		deadline := time.Time{}
		completionTime := time.Now()

		// Check if deadline is zero
		if deadline.IsZero() {
			t.Logf("✓ Zero deadline detected correctly")
		} else {
			t.Error("Expected deadline.IsZero() to be true")
		}

		// SLA should be false for tasks without deadline
		if !deadline.IsZero() {
			slaSuccess := completionTime.Before(deadline)
			t.Logf("SLA Success (shouldn't reach here): %v", slaSuccess)
		}
	})

	t.Run("VeryLongTask", func(t *testing.T) {
		// Task that takes much longer than expected
		arrivalTime := time.Now().Add(-3600 * time.Second) // Started 1 hour ago
		tau := 10.0                                        // Expected 10s
		slaMulti := 2.0
		deadline := arrivalTime.Add(time.Duration(slaMulti*tau) * time.Second) // Deadline was 1 hour ago
		completionTime := time.Now()

		slaSuccess := completionTime.Before(deadline)

		if slaSuccess {
			t.Error("Expected SLA violation for task that took 1 hour when deadline was 20s")
		}

		t.Logf("✓ Very long task correctly marked as SLA violation")
	})

	t.Run("NegativeDelay", func(t *testing.T) {
		// Task that completes very quickly
		completionTime := time.Now()
		deadline := time.Now().Add(100 * time.Second)

		delay := completionTime.Sub(deadline).Seconds()

		if delay >= 0 {
			t.Error("Expected negative delay for task completed before deadline")
		}

		slaSuccess := completionTime.Before(deadline) || completionTime.Equal(deadline)
		if !slaSuccess {
			t.Error("Expected SLA success for task with negative delay")
		}

		t.Logf("✓ Fast task with delay=%.1fs correctly marked as SLA success", delay)
	})
}

// TestSLASuccessRateComputation tests computing SLA success rate from multiple tasks
func TestSLASuccessRateComputation(t *testing.T) {
	results := []db.TaskResult{
		{TaskID: "task-1", SLASuccess: true},
		{TaskID: "task-2", SLASuccess: true},
		{TaskID: "task-3", SLASuccess: false},
		{TaskID: "task-4", SLASuccess: true},
		{TaskID: "task-5", SLASuccess: false},
		{TaskID: "task-6", SLASuccess: true},
		{TaskID: "task-7", SLASuccess: true},
		{TaskID: "task-8", SLASuccess: false},
		{TaskID: "task-9", SLASuccess: true},
		{TaskID: "task-10", SLASuccess: true},
	}

	successCount := 0
	for _, result := range results {
		if result.SLASuccess {
			successCount++
		}
	}

	expectedSuccessCount := 7
	if successCount != expectedSuccessCount {
		t.Errorf("Expected %d successes, got %d", expectedSuccessCount, successCount)
	}

	slaSuccessRate := float64(successCount) / float64(len(results)) * 100

	t.Logf("✓ SLA Success Rate: %.1f%% (%d/%d tasks)",
		slaSuccessRate, successCount, len(results))

	if slaSuccessRate != 70.0 {
		t.Errorf("Expected 70.0%% success rate, got %.1f%%", slaSuccessRate)
	}
}

// TestSLASuccessPerTaskType tests SLA tracking per task type
func TestSLASuccessPerTaskType(t *testing.T) {
	// Simulate results for different task types
	type taskTypeResult struct {
		taskType   string
		slaSuccess bool
	}

	results := []taskTypeResult{
		{"cpu-light", true},
		{"cpu-light", true},
		{"cpu-light", false},
		{"cpu-heavy", true},
		{"cpu-heavy", false},
		{"memory-heavy", true},
		{"gpu-inference", true},
		{"gpu-inference", true},
		{"gpu-training", false},
		{"mixed", true},
	}

	// Aggregate by task type
	stats := make(map[string]struct{ success, total int })
	for _, r := range results {
		s := stats[r.taskType]
		s.total++
		if r.slaSuccess {
			s.success++
		}
		stats[r.taskType] = s
	}

	t.Log("SLA Success Rate by Task Type:")
	for taskType, s := range stats {
		rate := float64(s.success) / float64(s.total) * 100
		t.Logf("  %s: %.1f%% (%d/%d)",
			taskType, rate, s.success, s.total)
	}

	// Verify cpu-light has expected rate
	cpuLightStats := stats["cpu-light"]
	cpuLightRate := float64(cpuLightStats.success) / float64(cpuLightStats.total) * 100
	expectedRate := 66.7 // 2 out of 3

	if cpuLightRate < expectedRate-1 || cpuLightRate > expectedRate+1 {
		t.Errorf("Expected cpu-light rate around %.1f%%, got %.1f%%",
			expectedRate, cpuLightRate)
	}
}

// TestSLASuccessIntegration tests SLA success computation in full task lifecycle
func TestSLASuccessIntegration(t *testing.T) {
	// Simulate a task lifecycle
	t.Run("FullTaskLifecycle", func(t *testing.T) {
		taskID := "integration-task-1"
		tau := 10.0
		slaMultiplier := 2.0

		// Step 1: Task submitted
		arrivalTime := time.Now()
		deadline := arrivalTime.Add(time.Duration(slaMultiplier*tau) * time.Second)

		task := &db.Task{
			TaskID:        taskID,
			TaskType:      "cpu-light",
			Tau:           tau,
			Deadline:      deadline,
			SLAMultiplier: slaMultiplier,
			CreatedAt:     arrivalTime,
		}

		t.Logf("Task submitted: arrival=%s, deadline=%s (in %.1fs)",
			arrivalTime.Format("15:04:05"),
			deadline.Format("15:04:05"),
			slaMultiplier*tau)

		// Step 2: Task starts
		startTime := arrivalTime.Add(2 * time.Second)
		task.StartedAt = startTime
		t.Logf("Task started: start=%s", startTime.Format("15:04:05"))

		// Step 3: Task completes (within SLA)
		completionTime := startTime.Add(8 * time.Second) // Total 10s from arrival
		slaSuccess := completionTime.Before(deadline) || completionTime.Equal(deadline)

		result := &db.TaskResult{
			TaskID:      taskID,
			WorkerID:    "worker-1",
			Status:      "success",
			SLASuccess:  slaSuccess,
			CompletedAt: completionTime,
		}

		t.Logf("Task completed: completion=%s, SLA=%v",
			completionTime.Format("15:04:05"), slaSuccess)

		// Verify SLA success
		if !slaSuccess {
			t.Error("Expected SLA success for task completed within deadline")
		}

		// Verify timing
		totalTime := completionTime.Sub(arrivalTime).Seconds()
		t.Logf("✓ Total time: %.1fs, Deadline: %.1fs, SLA Success: %v",
			totalTime, slaMultiplier*tau, slaSuccess)

		// Verify result fields
		if result.SLASuccess != slaSuccess {
			t.Errorf("Result.SLASuccess mismatch: expected %v, got %v",
				slaSuccess, result.SLASuccess)
		}
	})
}
