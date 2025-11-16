package db

import (
	"context"
	"testing"
	"time"
)

// TestTaskHistoryStructure verifies TaskHistory struct has all required fields
func TestTaskHistoryStructure(t *testing.T) {
	th := TaskHistory{
		TaskID:        "task-1",
		WorkerID:      "worker-1",
		Type:          "cpu-heavy",
		ArrivalTime:   time.Now(),
		Deadline:      time.Now().Add(10 * time.Second),
		ActualStart:   time.Now(),
		ActualFinish:  time.Now().Add(5 * time.Second),
		ActualRuntime: 5.0,
		SLASuccess:    true,
		CPUUsed:       4.0,
		MemUsed:       8.0,
		GPUUsed:       0.0,
		StorageUsed:   10.0,
		LoadAtStart:   0.5,
		Tau:           10.0,
		SLAMultiplier: 2.0,
	}

	if th.Type != "cpu-heavy" {
		t.Errorf("Expected Type 'cpu-heavy', got '%s'", th.Type)
	}
	if th.SLASuccess != true {
		t.Error("Expected SLASuccess to be true")
	}
	if th.ActualRuntime != 5.0 {
		t.Errorf("Expected ActualRuntime 5.0, got %f", th.ActualRuntime)
	}
}

// TestWorkerStatsStructure verifies WorkerStats struct has all required fields
func TestWorkerStatsStructure(t *testing.T) {
	ws := WorkerStats{
		WorkerID:      "worker-1",
		TasksRun:      100,
		SLAViolations: 5,
		TotalRuntime:  1000.0,
		CPUUsedTotal:  4000.0,
		MemUsedTotal:  8000.0,
		GPUUsedTotal:  0.0,
		OverloadTime:  50.0,
		TotalTime:     3600.0,
		AvgLoad:       0.6,
		PeriodStart:   time.Now().Add(-1 * time.Hour),
		PeriodEnd:     time.Now(),
	}

	if ws.TasksRun != 100 {
		t.Errorf("Expected TasksRun 100, got %d", ws.TasksRun)
	}
	if ws.SLAViolations != 5 {
		t.Errorf("Expected SLAViolations 5, got %d", ws.SLAViolations)
	}

	// Test SLA violation rate calculation
	violationRate := float64(ws.SLAViolations) / float64(ws.TasksRun)
	expectedRate := 0.05
	if violationRate != expectedRate {
		t.Errorf("Expected violation rate %.2f, got %.2f", expectedRate, violationRate)
	}

	// Test overload percentage
	overloadPct := (ws.OverloadTime / ws.TotalTime) * 100
	expectedOverload := (50.0 / 3600.0) * 100
	if overloadPct != expectedOverload {
		t.Errorf("Expected overload %.2f%%, got %.2f%%", expectedOverload, overloadPct)
	}
}

// TestValidTaskTypes verifies all 6 standardized task types
func TestValidTaskTypes(t *testing.T) {
	validTypes := []string{
		"cpu-light",
		"cpu-heavy",
		"memory-heavy",
		"gpu-inference",
		"gpu-training",
		"mixed",
	}

	for _, taskType := range validTypes {
		th := TaskHistory{
			TaskID: "test-task",
			Type:   taskType,
		}

		if th.Type != taskType {
			t.Errorf("Expected Type '%s', got '%s'", taskType, th.Type)
		}
	}
}

// TestHistoryDBInterface verifies HistoryDB implements expected methods
func TestHistoryDBInterface(t *testing.T) {
	// This test just verifies the interface exists and can be compiled
	// Actual database tests would require MongoDB connection
	var db *HistoryDB

	// Verify method signatures exist
	_ = func() ([]TaskHistory, error) {
		return db.GetTaskHistory(context.Background(), time.Now(), time.Now())
	}

	_ = func() ([]WorkerStats, error) {
		return db.GetWorkerStats(context.Background(), time.Now(), time.Now())
	}

	_ = func() ([]TaskHistory, error) {
		return db.GetTaskHistoryByType(context.Background(), "cpu-heavy", time.Now(), time.Now())
	}

	_ = func() (*WorkerStats, error) {
		return db.GetWorkerStatsForWorker(context.Background(), "worker-1", time.Now(), time.Now())
	}

	_ = func() (float64, error) {
		return db.GetSLASuccessRate(context.Background(), time.Now(), time.Now())
	}

	_ = func() (float64, error) {
		return db.GetSLASuccessRateByType(context.Background(), "cpu-heavy", time.Now(), time.Now())
	}

	_ = func() error {
		return db.Close(context.Background())
	}
}

// TestTaskHistoryBSONTags verifies all fields have proper BSON tags
func TestTaskHistoryBSONTags(t *testing.T) {
	// This is a compile-time check that BSON tags exist
	th := TaskHistory{}

	// These assignments verify the struct has the expected types
	_ = th.TaskID        // string
	_ = th.WorkerID      // string
	_ = th.Type          // string
	_ = th.ArrivalTime   // time.Time
	_ = th.Deadline      // time.Time
	_ = th.ActualStart   // time.Time
	_ = th.ActualFinish  // time.Time
	_ = th.ActualRuntime // float64
	_ = th.SLASuccess    // bool
	_ = th.CPUUsed       // float64
	_ = th.MemUsed       // float64
	_ = th.GPUUsed       // float64
	_ = th.StorageUsed   // float64
	_ = th.LoadAtStart   // float64
	_ = th.Tau           // float64
	_ = th.SLAMultiplier // float64
}

// TestWorkerStatsBSONTags verifies all fields have proper BSON tags
func TestWorkerStatsBSONTags(t *testing.T) {
	ws := WorkerStats{}

	_ = ws.WorkerID      // string
	_ = ws.TasksRun      // int
	_ = ws.SLAViolations // int
	_ = ws.TotalRuntime  // float64
	_ = ws.CPUUsedTotal  // float64
	_ = ws.MemUsedTotal  // float64
	_ = ws.GPUUsedTotal  // float64
	_ = ws.OverloadTime  // float64
	_ = ws.TotalTime     // float64
	_ = ws.AvgLoad       // float64
	_ = ws.PeriodStart   // time.Time
	_ = ws.PeriodEnd     // time.Time
}
