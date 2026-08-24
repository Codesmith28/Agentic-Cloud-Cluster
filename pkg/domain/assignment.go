package domain

import "time"

// Assignment records the binding between a task and a worker for an execution attempt.
type Assignment struct {
	AssignmentID string    `json:"assignment_id" bson:"assignment_id"`
	TaskID       string    `json:"task_id" bson:"task_id"`
	WorkerID     string    `json:"worker_id" bson:"worker_id"`
	AttemptID    string    `json:"attempt_id" bson:"attempt_id"`
	AttemptNo    int32     `json:"attempt_no" bson:"attempt_no"`
	AssignedAt   time.Time `json:"assigned_at" bson:"assigned_at"`
	LoadAtStart  float64   `json:"load_at_start" bson:"load_at_start"`
}
