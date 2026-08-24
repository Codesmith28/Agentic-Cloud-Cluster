package domain

import "time"

// TaskAttempt records the execution lifecycle of a single task attempt on a worker.
type TaskAttempt struct {
	AttemptID      string    `json:"attempt_id" bson:"attempt_id"`
	TaskID         string    `json:"task_id" bson:"task_id"`
	WorkerID       string    `json:"worker_id" bson:"worker_id"`
	AttemptNo      int32     `json:"attempt_no" bson:"attempt_no"`
	Status         string    `json:"status" bson:"status"`
	FailureReason  string    `json:"failure_reason,omitempty" bson:"failure_reason,omitempty"`
	LoadAtStart    float64   `json:"load_at_start" bson:"load_at_start"`
	AssignedAt     time.Time `json:"assigned_at" bson:"assigned_at"`
	LastHeartbeat  int64     `json:"last_heartbeat" bson:"last_heartbeat"`
	CompletedAt    time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	ResultStatus   string    `json:"result_status,omitempty" bson:"result_status,omitempty"`
	Logs           string    `json:"logs,omitempty" bson:"logs,omitempty"`
	ResultLocation string    `json:"result_location,omitempty" bson:"result_location,omitempty"`
	OutputFiles    []string  `json:"output_files,omitempty" bson:"output_files,omitempty"`
}
