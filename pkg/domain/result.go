package domain

import "time"

// TaskResult captures the terminal execution outcome of a task.
type TaskResult struct {
	TaskID      string    `json:"task_id" bson:"task_id"`
	WorkerID    string    `json:"worker_id" bson:"worker_id"`
	Status      string    `json:"status" bson:"status"`
	Logs        string    `json:"logs" bson:"logs"`
	CompletedAt time.Time `json:"completed_at" bson:"completed_at"`
	SLASuccess  bool      `json:"sla_success" bson:"sla_success"`
}
