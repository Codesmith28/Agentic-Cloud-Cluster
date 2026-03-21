package telemetry

import (
	"testing"
	"time"
)

func TestAddTaskCarriesAttemptMetadata(t *testing.T) {
	monitor := NewMonitor("worker-1", time.Second)
	monitor.AddTask("task-1", "att-task-1-2", 2, 1.5, 3.0)

	runningTasks := monitor.GetRunningTasks()
	if len(runningTasks) != 1 {
		t.Fatalf("expected 1 running task, got %d", len(runningTasks))
	}

	task := runningTasks[0]
	if task.TaskId != "task-1" {
		t.Fatalf("unexpected task id: %s", task.TaskId)
	}
	if task.AttemptId != "att-task-1-2" {
		t.Fatalf("unexpected attempt id: %s", task.AttemptId)
	}
	if task.AttemptNo != 2 {
		t.Fatalf("unexpected attempt no: %d", task.AttemptNo)
	}
}
