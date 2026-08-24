package ports

import (
	"context"
	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/domain"
)

// TaskRepository defines persistence operations for tasks.
type TaskRepository interface {
	Create(ctx context.Context, task *domain.Task) error
	GetByID(ctx context.Context, taskID string) (*domain.Task, error)
	GetByStatus(ctx context.Context, status domain.TaskStatus) ([]*domain.Task, error)
	GetAll(ctx context.Context) ([]*domain.Task, error)
	UpdateStatus(ctx context.Context, taskID string, status domain.TaskStatus) error
	UpdateAttempt(ctx context.Context, taskID, attemptID string, attemptNo int32, workerID string) error
	Delete(ctx context.Context, taskID string) error
}

// WorkerRepository defines persistence operations for worker nodes.
type WorkerRepository interface {
	Register(ctx context.Context, workerID, workerIP string) error
	Unregister(ctx context.Context, workerID string) error
	GetByID(ctx context.Context, workerID string) (*domain.Worker, error)
	GetAll(ctx context.Context) ([]*domain.Worker, error)
	UpdateResources(ctx context.Context, workerID string, totalCPU, totalMemory, totalStorage float64) error
	AllocateResources(ctx context.Context, workerID string, cpu, memory, storage float64) error
	ReleaseResources(ctx context.Context, workerID string, cpu, memory, storage float64) error
}

// AssignmentRepository defines persistence operations for task assignments.
type AssignmentRepository interface {
	Create(ctx context.Context, assignment *domain.Assignment) error
	GetByTaskID(ctx context.Context, taskID string) (*domain.Assignment, error)
	GetByWorker(ctx context.Context, workerID string) ([]*domain.Assignment, error)
	Delete(ctx context.Context, taskID string) error
}

// AttemptRepository defines persistence operations for execution attempts.
type AttemptRepository interface {
	Create(ctx context.Context, attempt *domain.TaskAttempt) error
	GetByID(ctx context.Context, attemptID string) (*domain.TaskAttempt, error)
	GetByTaskID(ctx context.Context, taskID string) ([]*domain.TaskAttempt, error)
	GetActiveByWorker(ctx context.Context, workerID string) ([]*domain.TaskAttempt, error)
	TouchHeartbeat(ctx context.Context, attemptID string, heartbeatTs int64) error
	Complete(ctx context.Context, attemptID, status, reason, resultStatus, logs, location string, files []string) error
}

// ResultRepository defines persistence operations for task terminal results.
type ResultRepository interface {
	Create(ctx context.Context, result *domain.TaskResult) error
	GetByTaskID(ctx context.Context, taskID string) (*domain.TaskResult, error)
	GetByWorker(ctx context.Context, workerID string) ([]*domain.TaskResult, error)
}
