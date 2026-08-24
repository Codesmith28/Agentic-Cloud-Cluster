package ports

import (
	"context"
	"time"
)

// TaskView provides read-only attributes of a task required for scheduling.
type TaskView struct {
	TaskID        string
	DockerImage   string
	Command       string
	ReqCPU        float64
	ReqMemory     float64
	ReqStorage    float64
	TaskType      string
	SLAMultiplier float64
	UserID        string
	ArrivalTime   time.Time
	Tau           float64
	Deadline      time.Time
}

// WorkerView provides read-only attributes of a worker required for scheduling.
type WorkerView struct {
	WorkerID        string
	IsActive        bool
	WorkerIP        string
	AvailableCPU    float64
	AvailableMemory float64
	AvailableStorage float64
	TotalCPU        float64
	TotalMemory     float64
	TotalStorage    float64
	CurrentCPUUsage float64
	CurrentMemUsage float64
	Load            float64
}

// Scheduler defines the contract for task-to-worker dispatch algorithms.
type Scheduler interface {
	SelectWorker(task TaskView, workers map[string]*WorkerView) string
	GetName() string
	Reset()
}

// TaskOutcome represents execution feedback for reinforcement learning / online tuning.
type TaskOutcome struct {
	TaskID           string
	WorkerID         string
	Status           string
	Reward           float64
	RuntimeSeconds   float64
	SLASuccess       bool
	ClusterHash      string
	ModelVersionHint string
	CompletedAt      time.Time
}

// OutcomeReporter accepts post-execution metrics for adaptive schedulers.
type OutcomeReporter interface {
	ReportOutcome(ctx context.Context, outcome TaskOutcome) error
}
