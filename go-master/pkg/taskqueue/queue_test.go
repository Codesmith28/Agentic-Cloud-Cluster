package taskqueue

import (
	"testing"
	"time"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
)

func TestEnqueueDequeue(t *testing.T) {
	queue := NewTaskQueue()

	task1 := &pb.Task{Id: "task-1", Priority: 5}
	task2 := &pb.Task{Id: "task-2", Priority: 10}
	task3 := &pb.Task{Id: "task-3", Priority: 1}

	err := queue.Enqueue(task1)
	if err != nil {
		t.Fatalf("Enqueue task1 failed: %v", err)
	}

	err = queue.Enqueue(task2)
	if err != nil {
		t.Fatalf("Enqueue task2 failed: %v", err)
	}

	err = queue.Enqueue(task3)
	if err != nil {
		t.Fatalf("Enqueue task3 failed: %v", err)
	}

	// Dequeue should return highest priority first
	batch := queue.DequeueBatch(2)

	if len(batch) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(batch))
	}

	if batch[0].Id != "task-2" {
		t.Errorf("Expected task-2 first (priority 10), got %s", batch[0].Id)
	}

	if batch[1].Id != "task-1" {
		t.Errorf("Expected task-1 second (priority 5), got %s", batch[1].Id)
	}
}

func TestDeadlinePriority(t *testing.T) {
	queue := NewTaskQueue()

	now := time.Now().Unix()

	task1 := &pb.Task{Id: "task-1", Priority: 5, DeadlineUnix: now + 3600}
	task2 := &pb.Task{Id: "task-2", Priority: 5, DeadlineUnix: now + 1800}

	err := queue.Enqueue(task1)
	if err != nil {
		t.Fatalf("Enqueue task1 failed: %v", err)
	}

	err = queue.Enqueue(task2)
	if err != nil {
		t.Fatalf("Enqueue task2 failed: %v", err)
	}

	batch := queue.DequeueBatch(1)

	// Earlier deadline should come first
	if batch[0].Id != "task-2" {
		t.Errorf("Expected task-2 with earlier deadline, got %s", batch[0].Id)
	}
}

func TestUpdateStatus(t *testing.T) {
	queue := NewTaskQueue()

	task := &pb.Task{Id: "task-1", Priority: 5}
	err := queue.Enqueue(task)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	err = queue.UpdateStatus("task-1", "RUNNING")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	status, err := queue.GetStatus("task-1")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status != "RUNNING" {
		t.Errorf("Expected RUNNING status, got %v", status)
	}
}

func TestDuplicateEnqueue(t *testing.T) {
	queue := NewTaskQueue()

	task := &pb.Task{Id: "task-1", Priority: 5}
	err := queue.Enqueue(task)
	if err != nil {
		t.Fatalf("First enqueue failed: %v", err)
	}

	// Try to enqueue same task again
	err = queue.Enqueue(task)
	if err == nil {
		t.Error("Expected error when enqueueing duplicate task, got nil")
	}
}

func TestRemoveTask(t *testing.T) {
	queue := NewTaskQueue()

	task1 := &pb.Task{Id: "task-1", Priority: 5}
	task2 := &pb.Task{Id: "task-2", Priority: 10}

	queue.Enqueue(task1)
	queue.Enqueue(task2)

	err := queue.Remove("task-1")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if queue.Size() != 1 {
		t.Errorf("Expected queue size 1, got %d", queue.Size())
	}

	batch := queue.DequeueBatch(1)
	if batch[0].Id != "task-2" {
		t.Errorf("Expected task-2 to remain, got %s", batch[0].Id)
	}
}

func TestPeekPending(t *testing.T) {
	queue := NewTaskQueue()

	task1 := &pb.Task{Id: "task-1", Priority: 5}
	task2 := &pb.Task{Id: "task-2", Priority: 10}

	queue.Enqueue(task1)
	queue.Enqueue(task2)

	// Peek should not remove tasks
	pending := queue.PeekPending()
	if len(pending) != 2 {
		t.Fatalf("Expected 2 pending tasks, got %d", len(pending))
	}

	// Size should still be 2
	if queue.Size() != 2 {
		t.Errorf("Expected queue size 2 after peek, got %d", queue.Size())
	}
}

func TestQueueSize(t *testing.T) {
	queue := NewTaskQueue()

	if queue.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", queue.Size())
	}

	task1 := &pb.Task{Id: "task-1", Priority: 5}
	queue.Enqueue(task1)

	if queue.Size() != 1 {
		t.Errorf("Expected size 1, got %d", queue.Size())
	}

	queue.DequeueBatch(1)

	if queue.Size() != 0 {
		t.Errorf("Expected size 0 after dequeue, got %d", queue.Size())
	}
}

func TestFailedTaskRequeue(t *testing.T) {
	queue := NewTaskQueue()

	task := &pb.Task{Id: "task-1", Priority: 5}
	queue.Enqueue(task)

	// Dequeue the task
	batch := queue.DequeueBatch(1)
	if len(batch) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(batch))
	}

	// Queue should be empty now
	if queue.Size() != 0 {
		t.Errorf("Expected empty queue, got size %d", queue.Size())
	}

	// Mark task as failed - should re-enqueue
	err := queue.UpdateStatus("task-1", "FAILED")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	// Task should be back in queue
	if queue.Size() != 1 {
		t.Errorf("Expected task to be re-queued, got size %d", queue.Size())
	}
}

func TestGetAllTasks(t *testing.T) {
	queue := NewTaskQueue()

	task1 := &pb.Task{Id: "task-1", Priority: 5}
	task2 := &pb.Task{Id: "task-2", Priority: 10}

	queue.Enqueue(task1)
	queue.Enqueue(task2)

	allTasks := queue.GetAllTasks()
	if len(allTasks) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(allTasks))
	}

	if _, exists := allTasks["task-1"]; !exists {
		t.Error("Expected task-1 in all tasks")
	}

	if _, exists := allTasks["task-2"]; !exists {
		t.Error("Expected task-2 in all tasks")
	}
}

func TestFIFOOrderingSamePriority(t *testing.T) {
	queue := NewTaskQueue()

	// All tasks with same priority - should follow FIFO
	task1 := &pb.Task{Id: "task-1", Priority: 5}
	task2 := &pb.Task{Id: "task-2", Priority: 5}
	task3 := &pb.Task{Id: "task-3", Priority: 5}

	queue.Enqueue(task1)
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	queue.Enqueue(task2)
	time.Sleep(1 * time.Millisecond)
	queue.Enqueue(task3)

	batch := queue.DequeueBatch(3)

	if len(batch) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(batch))
	}

	if batch[0].Id != "task-1" {
		t.Errorf("Expected task-1 first (FIFO), got %s", batch[0].Id)
	}

	if batch[1].Id != "task-2" {
		t.Errorf("Expected task-2 second (FIFO), got %s", batch[1].Id)
	}

	if batch[2].Id != "task-3" {
		t.Errorf("Expected task-3 third (FIFO), got %s", batch[2].Id)
	}
}
