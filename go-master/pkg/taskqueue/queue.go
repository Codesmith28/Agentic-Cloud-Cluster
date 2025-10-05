package taskqueue

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
)

// TaskQueue manages pending tasks with priority ordering
type TaskQueue struct {
	mu    sync.RWMutex
	tasks map[string]*TaskEntry // taskID -> TaskEntry
	pq    PriorityQueue
	cond  *sync.Cond
}

type TaskEntry struct {
	Task      *pb.Task
	Status    string
	CreatedAt time.Time
	index     int // heap index
}

// PriorityQueue implements heap.Interface
type PriorityQueue []*TaskEntry

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// Higher priority first, then earliest deadline
	if pq[i].Task.Priority != pq[j].Task.Priority {
		return pq[i].Task.Priority > pq[j].Task.Priority
	}

	// If both have deadlines, earlier deadline first
	if pq[i].Task.DeadlineUnix > 0 && pq[j].Task.DeadlineUnix > 0 {
		return pq[i].Task.DeadlineUnix < pq[j].Task.DeadlineUnix
	}

	// Deadline task comes first
	if pq[i].Task.DeadlineUnix > 0 {
		return true
	}
	if pq[j].Task.DeadlineUnix > 0 {
		return false
	}

	// Otherwise FIFO
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	entry := x.(*TaskEntry)
	entry.index = n
	*pq = append(*pq, entry)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	*pq = old[0 : n-1]
	return entry
}

// NewTaskQueue creates a new task queue
func NewTaskQueue() *TaskQueue {
	tq := &TaskQueue{
		tasks: make(map[string]*TaskEntry),
		pq:    make(PriorityQueue, 0),
	}
	tq.cond = sync.NewCond(&tq.mu)
	heap.Init(&tq.pq)
	return tq
}

// Enqueue adds a task to the queue
func (tq *TaskQueue) Enqueue(task *pb.Task) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	if _, exists := tq.tasks[task.Id]; exists {
		return fmt.Errorf("task %s already exists", task.Id)
	}

	entry := &TaskEntry{
		Task:      task,
		Status:    "PENDING",
		CreatedAt: time.Now(),
	}

	tq.tasks[task.Id] = entry
	heap.Push(&tq.pq, entry)

	// Signal waiting goroutines
	tq.cond.Signal()

	return nil
}

// DequeueBatch retrieves up to n highest priority tasks
func (tq *TaskQueue) DequeueBatch(n int) []*pb.Task {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	result := make([]*pb.Task, 0, n)

	for i := 0; i < n && tq.pq.Len() > 0; i++ {
		entry := heap.Pop(&tq.pq).(*TaskEntry)
		entry.Status = "SCHEDULED"
		result = append(result, entry.Task)
	}

	return result
}

// PeekPending returns pending tasks without removing them
func (tq *TaskQueue) PeekPending() []*pb.Task {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	result := make([]*pb.Task, 0, tq.pq.Len())
	for _, entry := range tq.pq {
		result = append(result, entry.Task)
	}

	return result
}

// UpdateStatus updates task status
func (tq *TaskQueue) UpdateStatus(taskID string, status string) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	entry, exists := tq.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	entry.Status = status

	// If task failed or cancelled, put it back in queue
	if status == "FAILED" {
		if entry.index == -1 { // Not in heap
			heap.Push(&tq.pq, entry)
		}
	}

	return nil
}

// GetStatus returns task status
func (tq *TaskQueue) GetStatus(taskID string) (string, error) {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	entry, exists := tq.tasks[taskID]
	if !exists {
		return "PENDING", fmt.Errorf("task %s not found", taskID)
	}

	return entry.Status, nil
}

// Remove removes a task from the queue
func (tq *TaskQueue) Remove(taskID string) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	entry, exists := tq.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if entry.index >= 0 {
		heap.Remove(&tq.pq, entry.index)
	}

	delete(tq.tasks, taskID)
	return nil
}

// WaitForTasks blocks until tasks are available
func (tq *TaskQueue) WaitForTasks() {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	
	for tq.pq.Len() == 0 {
		tq.cond.Wait()
	}
}

// Size returns the number of pending tasks
func (tq *TaskQueue) Size() int {
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	return tq.pq.Len()
}

// GetAllTasks returns all tasks (for debugging/monitoring)
func (tq *TaskQueue) GetAllTasks() map[string]*TaskEntry {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	// Return a copy to prevent external modification
	tasksCopy := make(map[string]*TaskEntry, len(tq.tasks))
	for k, v := range tq.tasks {
		entryCopy := &TaskEntry{
			Task:      v.Task,
			Status:    v.Status,
			CreatedAt: v.CreatedAt,
			index:     v.index,
		}
		tasksCopy[k] = entryCopy
	}
	return tasksCopy
}
