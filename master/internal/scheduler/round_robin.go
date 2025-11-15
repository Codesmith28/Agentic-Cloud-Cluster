package scheduler

import (
	"log"
	"sync"

	pb "master/proto"
)

// WorkerInfo contains information about a worker for scheduling decisions
type WorkerInfo struct {
	WorkerID         string
	IsActive         bool
	WorkerIP         string
	AvailableCPU     float64
	AvailableMemory  float64
	AvailableStorage float64
	AvailableGPU     float64
}

// RoundRobinScheduler implements a simple round-robin scheduling algorithm
type RoundRobinScheduler struct {
	lastWorkerIndex int
	mu              sync.Mutex
}

// NewRoundRobinScheduler creates a new round-robin scheduler
func NewRoundRobinScheduler() *RoundRobinScheduler {
	return &RoundRobinScheduler{
		lastWorkerIndex: -1,
	}
}

// SelectWorker selects the next available worker using round-robin algorithm
// Returns the worker ID or empty string if no suitable worker is found
func (s *RoundRobinScheduler) SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(workers) == 0 {
		log.Printf("⚠️ Scheduler: No workers available")
		return ""
	}

	// Create a sorted list of worker IDs for consistent ordering
	workerIDs := make([]string, 0, len(workers))
	for id := range workers {
		workerIDs = append(workerIDs, id)
	}

	// Sort for deterministic behavior
	sortWorkerIDs(workerIDs)

	// Start from next worker after last selected
	startIndex := (s.lastWorkerIndex + 1) % len(workerIDs)

	// Try each worker in round-robin order
	for i := 0; i < len(workerIDs); i++ {
		currentIndex := (startIndex + i) % len(workerIDs)
		workerID := workerIDs[currentIndex]
		worker := workers[workerID]

		// Check if worker is suitable
		if s.isWorkerSuitable(worker, task) {
			s.lastWorkerIndex = currentIndex
			log.Printf("🔄 Scheduler: Round-robin selected %s (index %d/%d)",
				workerID, currentIndex+1, len(workerIDs))
			return workerID
		}
	}

	log.Printf("⚠️ Scheduler: No suitable worker found for task %s (checked %d workers)",
		task.TaskId, len(workerIDs))
	return ""
}

// isWorkerSuitable checks if a worker can handle the task
func (s *RoundRobinScheduler) isWorkerSuitable(worker *WorkerInfo, task *pb.Task) bool {
	// Skip inactive workers
	if !worker.IsActive {
		return false
	}

	// Skip workers without IP configured
	if worker.WorkerIP == "" {
		return false
	}

	// Check resource availability
	if worker.AvailableCPU < task.ReqCpu {
		return false
	}
	if worker.AvailableMemory < task.ReqMemory {
		return false
	}
	if worker.AvailableStorage < task.ReqStorage {
		return false
	}
	if worker.AvailableGPU < task.ReqGpu {
		return false
	}

	return true
}

// Reset resets the scheduler state (useful for testing)
func (s *RoundRobinScheduler) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastWorkerIndex = -1
}

// GetName returns the scheduler name
func (s *RoundRobinScheduler) GetName() string {
	return "Round-Robin"
}

// sortWorkerIDs sorts worker IDs alphabetically for deterministic ordering
func sortWorkerIDs(ids []string) {
	// Simple bubble sort (sufficient for small number of workers)
	n := len(ids)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if ids[j] > ids[j+1] {
				ids[j], ids[j+1] = ids[j+1], ids[j]
			}
		}
	}
}
