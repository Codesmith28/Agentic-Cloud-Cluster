package domain

import "time"

// Worker represents a worker node registered with the cluster.
type Worker struct {
	WorkerID         string    `json:"worker_id" bson:"worker_id"`
	WorkerIP         string    `json:"worker_ip" bson:"worker_ip"`
	TotalCPU         float64   `json:"total_cpu" bson:"total_cpu"`
	TotalMemory      float64   `json:"total_memory" bson:"total_memory"`
	TotalStorage     float64   `json:"total_storage" bson:"total_storage"`
	AllocatedCPU     float64   `json:"allocated_cpu" bson:"allocated_cpu"`
	AllocatedMemory  float64   `json:"allocated_memory" bson:"allocated_memory"`
	AllocatedStorage float64   `json:"allocated_storage" bson:"allocated_storage"`
	AvailableCPU     float64   `json:"available_cpu" bson:"available_cpu"`
	AvailableMemory  float64   `json:"available_memory" bson:"available_memory"`
	AvailableStorage float64   `json:"available_storage" bson:"available_storage"`
	IsActive         bool      `json:"is_active" bson:"is_active"`
	LastHeartbeat    int64     `json:"last_heartbeat" bson:"last_heartbeat"`
	RegisteredAt     time.Time `json:"registered_at" bson:"registered_at"`
	UpdatedAt        time.Time `json:"updated_at" bson:"updated_at"`
}

// HasCapacity returns true if the worker can accommodate the requested resources.
func (w *Worker) HasCapacity(cpu, memory, storage float64) bool {
	return w.IsActive &&
		w.AvailableCPU >= cpu &&
		w.AvailableMemory >= memory &&
		w.AvailableStorage >= storage
}
