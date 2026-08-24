package domain

import "time"

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task represents a unit of work submitted to the cluster.
type Task struct {
	TaskID            string     `json:"task_id" bson:"task_id"`
	UserID            string     `json:"user_id" bson:"user_id"`
	TaskName          string     `json:"task_name" bson:"task_name"`
	SubmittedAt       int64      `json:"submitted_at" bson:"submitted_at"`
	DockerImage       string     `json:"docker_image" bson:"docker_image"`
	Command           string     `json:"command" bson:"command"`
	ReqCPU            float64    `json:"req_cpu" bson:"req_cpu"`
	ReqMemory         float64    `json:"req_memory" bson:"req_memory"`
	ReqStorage        float64    `json:"req_storage" bson:"req_storage"`
	Tag               string     `json:"tag,omitempty" bson:"tag,omitempty"`
	KValue            float64    `json:"k_value,omitempty" bson:"k_value,omitempty"`
	TaskType          string     `json:"task_type,omitempty" bson:"task_type,omitempty"`
	SLAMultiplier     float64    `json:"sla_multiplier" bson:"sla_multiplier"`
	Deadline          time.Time  `json:"deadline,omitempty" bson:"deadline,omitempty"`
	Tau               float64    `json:"tau,omitempty" bson:"tau,omitempty"`
	CurrentAttemptID  string     `json:"current_attempt_id,omitempty" bson:"current_attempt_id,omitempty"`
	CurrentAttemptNo  int32      `json:"current_attempt_no,omitempty" bson:"current_attempt_no,omitempty"`
	RecoveryCount     int32      `json:"recovery_count,omitempty" bson:"recovery_count,omitempty"`
	LastFailureReason string     `json:"last_failure_reason,omitempty" bson:"last_failure_reason,omitempty"`
	LastWorkerID      string     `json:"last_worker_id,omitempty" bson:"last_worker_id,omitempty"`
	Status            TaskStatus `json:"status" bson:"status"`
	CreatedAt         time.Time  `json:"created_at" bson:"created_at"`
	StartedAt         time.Time  `json:"started_at,omitempty" bson:"started_at,omitempty"`
	CompletedAt       time.Time  `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}

// IsTerminal returns true if the task is in a final state.
func (t *Task) IsTerminal() bool {
	return t.Status == TaskStatusCompleted || t.Status == TaskStatusFailed || t.Status == TaskStatusCancelled
}
