package scheduler

import pb "master/proto"

// Scheduler is the interface that all scheduling algorithms must implement
type Scheduler interface {
	// SelectWorker selects the best worker for the given task
	// Returns worker ID or empty string if no suitable worker found
	SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string

	// GetName returns the name of the scheduling algorithm
	GetName() string

	// Reset resets any internal state (useful for testing)
	Reset()
}
