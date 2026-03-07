package scheduler

import (
	"context"
	"time"

	pb "master/proto"
)

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

// TaskOutcome is fed back to schedulers that support online learning.
type TaskOutcome struct {
	TaskID           string
	WorkerID         string
	Status           string
	Reward           float64
	RuntimeSeconds   float64
	SLASuccess       bool
	Task             *pb.Task
	ClusterHash      string
	ModelVersionHint string
	CompletedAt      time.Time
}

// OutcomeReporter is implemented by schedulers that accept post-execution feedback.
type OutcomeReporter interface {
	ReportOutcome(ctx context.Context, outcome TaskOutcome) error
}
