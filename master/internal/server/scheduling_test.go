package server

import (
	"testing"
	"time"

	"master/internal/telemetry"
	pb "master/proto"
)

func TestNormalizeTaskForSchedulingInfersTypeAndDefaults(t *testing.T) {
	task := &pb.Task{
		TaskId:      "task-1",
		ReqCpu:      2.0,
		ReqMemory:   16.0,
		ReqStorage:  1.0,
		TaskType:    "",
		SubmittedAt: 0,
	}

	meta := normalizeTaskForScheduling(task)

	if task.SubmittedAt <= 0 {
		t.Fatalf("expected SubmittedAt to be set, got %d", task.SubmittedAt)
	}
	if task.SlaMultiplier != 2.0 {
		t.Fatalf("expected default SLA multiplier 2.0, got %.2f", task.SlaMultiplier)
	}
	if task.TaskType != "memory-heavy" {
		t.Fatalf("expected inferred task type memory-heavy, got %s", task.TaskType)
	}
	if meta.taskType != task.TaskType {
		t.Fatalf("metadata task type mismatch: %s vs %s", meta.taskType, task.TaskType)
	}

	expectedTau := telemetry.DefaultTauForTaskType(task.TaskType)
	if meta.tau != expectedTau {
		t.Fatalf("expected tau %.2f, got %.2f", expectedTau, meta.tau)
	}

	arrival := time.Unix(task.SubmittedAt, 0)
	if !meta.deadline.After(arrival) {
		t.Fatalf("expected deadline after arrival: arrival=%s deadline=%s", arrival, meta.deadline)
	}
}

func TestRemoveQueuedTaskByIDDoesNotAffectInFlightProcessing(t *testing.T) {
	s := NewMasterServer(nil, nil, nil, nil, nil, nil, nil, nil)
	s.queueMu.Lock()
	s.processingTasks["task-1"] = true
	s.queueMu.Unlock()

	if removed := s.removeQueuedTaskByID("task-1"); removed {
		t.Fatal("expected removeQueuedTaskByID to return false for in-flight task")
	}
	if !s.isTaskBeingProcessed("task-1") {
		t.Fatal("expected in-flight processing marker to remain set")
	}
}

func TestQueueProcessorStopsCleanly(t *testing.T) {
	s := NewMasterServer(nil, nil, nil, nil, nil, nil, nil, nil)
	s.StartQueueProcessor()

	done := make(chan struct{})
	go func() {
		s.StopQueueProcessor()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StopQueueProcessor blocked")
	}
}
