package workerregistry

import (
	"fmt"
	"sync"
	"time"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
)

// Registry manages worker state and availability
// It provides thread-safe operations for tracking workers, their resources,
// and resource reservations for task assignments
type Registry struct {
	mu sync.RWMutex

	// workers maps worker ID to worker info
	workers map[string]*pb.Worker

	// reservations maps task ID to reservation details
	reservations map[string]*Reservation

	// subscribers receive events when registry changes
	subscribers []chan<- RegistryEvent
}

// Reservation tracks resources reserved for a specific task
type Reservation struct {
	TaskID    string
	WorkerID  string
	CpuCores  float64
	MemoryMB  int32
	GpuUnits  int32
	CreatedAt time.Time
	ExpiresAt time.Time
}

// RegistryEvent represents a change in the registry
type RegistryEvent struct {
	Type      string // "worker_added", "worker_updated", "worker_removed"
	WorkerID  string
	Worker    *pb.Worker
	Timestamp time.Time
}

// NewRegistry creates a new worker registry
func NewRegistry() *Registry {
	return &Registry{
		workers:      make(map[string]*pb.Worker),
		reservations: make(map[string]*Reservation),
		subscribers:  make([]chan<- RegistryEvent, 0),
	}
}

// UpdateHeartbeat updates worker status from heartbeat
// This is called when a worker sends a heartbeat to the master
func (r *Registry) UpdateHeartbeat(worker *pb.Worker) error {
	if worker == nil || worker.Id == "" {
		return fmt.Errorf("invalid worker: missing ID")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	worker.LastSeenUnix = now

	_, exists := r.workers[worker.Id]

	if exists {
		// Update existing worker
		r.workers[worker.Id] = worker
		r.notifySubscribers(RegistryEvent{
			Type:      "worker_updated",
			WorkerID:  worker.Id,
			Worker:    worker,
			Timestamp: time.Now(),
		})
	} else {
		// New worker registration
		r.workers[worker.Id] = worker
		r.notifySubscribers(RegistryEvent{
			Type:      "worker_added",
			WorkerID:  worker.Id,
			Worker:    worker,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// GetSnapshot returns a copy of all active workers
// This is used by the scheduler to get current worker state
func (r *Registry) GetSnapshot() []*pb.Worker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make([]*pb.Worker, 0, len(r.workers))
	for _, worker := range r.workers {
		// Create a copy to avoid external modifications
		workerCopy := &pb.Worker{
			Id:           worker.Id,
			TotalCpu:     worker.TotalCpu,
			TotalMem:     worker.TotalMem,
			Gpus:         worker.Gpus,
			TotalDiskMb:  worker.TotalDiskMb,
			Labels:       worker.Labels,
			FreeCpu:      worker.FreeCpu,
			FreeMem:      worker.FreeMem,
			FreeGpus:     worker.FreeGpus,
			LastSeenUnix: worker.LastSeenUnix,
			ExecModes:    worker.ExecModes,
			Architecture: worker.Architecture,
			Accelerators: worker.Accelerators,
			Zone:         worker.Zone,
			Meta:         worker.Meta,
			Extra:        worker.Extra,
		}
		snapshot = append(snapshot, workerCopy)
	}
	return snapshot
}

// Reserve resources for a task on a specific worker
// This prevents over-allocation by tracking reserved resources
func (r *Registry) Reserve(taskID string, workerID string, cpuReq float64, memMB, gpu int32, ttl time.Duration) error {
	if taskID == "" || workerID == "" {
		return fmt.Errorf("invalid reservation: missing task ID or worker ID")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	worker, exists := r.workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	// Check if resources are available
	if worker.FreeCpu < cpuReq {
		return fmt.Errorf("insufficient CPU on worker %s: need %.2f, have %.2f", workerID, cpuReq, worker.FreeCpu)
	}
	if worker.FreeMem < memMB {
		return fmt.Errorf("insufficient memory on worker %s: need %d MB, have %d MB", workerID, memMB, worker.FreeMem)
	}
	if worker.FreeGpus < gpu {
		return fmt.Errorf("insufficient GPU on worker %s: need %d, have %d", workerID, gpu, worker.FreeGpus)
	}

	// Create reservation
	now := time.Now()
	reservation := &Reservation{
		TaskID:    taskID,
		WorkerID:  workerID,
		CpuCores:  cpuReq,
		MemoryMB:  memMB,
		GpuUnits:  gpu,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	r.reservations[taskID] = reservation

	// Update worker's free resources
	worker.FreeCpu -= cpuReq
	worker.FreeMem -= memMB
	worker.FreeGpus -= gpu

	return nil
}

// Release resources when task completes
// This returns resources back to the worker's available pool
func (r *Registry) Release(taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	reservation, exists := r.reservations[taskID]
	if !exists {
		return fmt.Errorf("reservation for task %s not found", taskID)
	}

	worker, exists := r.workers[reservation.WorkerID]
	if exists {
		// Return resources to worker
		worker.FreeCpu += reservation.CpuCores
		worker.FreeMem += reservation.MemoryMB
		worker.FreeGpus += reservation.GpuUnits
	}

	// Remove reservation
	delete(r.reservations, taskID)

	return nil
}

// Subscribe to registry events
// Returns a channel that will receive events when workers are added/updated/removed
func (r *Registry) Subscribe() <-chan RegistryEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan RegistryEvent, 10)
	r.subscribers = append(r.subscribers, ch)
	return ch
}

// notifySubscribers sends an event to all subscribers
func (r *Registry) notifySubscribers(event RegistryEvent) {
	for _, ch := range r.subscribers {
		select {
		case ch <- event:
		default:
			// Skip if channel is full (non-blocking)
		}
	}
}

// CleanupStaleWorkers removes workers that haven't sent heartbeat within timeout
// This should be called periodically to detect failed workers
func (r *Registry) CleanupStaleWorkers(timeout time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	cutoff := now - int64(timeout.Seconds())
	removed := make([]string, 0)

	for id, worker := range r.workers {
		if worker.LastSeenUnix < cutoff {
			delete(r.workers, id)
			removed = append(removed, id)
			r.notifySubscribers(RegistryEvent{
				Type:      "worker_removed",
				WorkerID:  id,
				Worker:    worker,
				Timestamp: time.Now(),
			})
		}
	}

	return removed
}

// CleanupExpiredReservations removes reservations that have expired
// This prevents resource leaks if task assignments fail
func (r *Registry) CleanupExpiredReservations() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	expired := make([]string, 0)

	for taskID, reservation := range r.reservations {
		if now.After(reservation.ExpiresAt) {
			// Return resources to worker
			if worker, exists := r.workers[reservation.WorkerID]; exists {
				worker.FreeCpu += reservation.CpuCores
				worker.FreeMem += reservation.MemoryMB
				worker.FreeGpus += reservation.GpuUnits
			}
			delete(r.reservations, taskID)
			expired = append(expired, taskID)
		}
	}

	return expired
}

// GetWorker retrieves a specific worker by ID
func (r *Registry) GetWorker(workerID string) (*pb.Worker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	worker, exists := r.workers[workerID]
	if !exists {
		return nil, fmt.Errorf("worker %s not found", workerID)
	}

	// Return a copy
	return &pb.Worker{
		Id:           worker.Id,
		TotalCpu:     worker.TotalCpu,
		TotalMem:     worker.TotalMem,
		Gpus:         worker.Gpus,
		TotalDiskMb:  worker.TotalDiskMb,
		Labels:       worker.Labels,
		FreeCpu:      worker.FreeCpu,
		FreeMem:      worker.FreeMem,
		FreeGpus:     worker.FreeGpus,
		LastSeenUnix: worker.LastSeenUnix,
		ExecModes:    worker.ExecModes,
		Architecture: worker.Architecture,
		Accelerators: worker.Accelerators,
		Zone:         worker.Zone,
		Meta:         worker.Meta,
		Extra:        worker.Extra,
	}, nil
}

// GetReservation retrieves reservation details for a task
func (r *Registry) GetReservation(taskID string) (*Reservation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reservation, exists := r.reservations[taskID]
	if !exists {
		return nil, fmt.Errorf("reservation for task %s not found", taskID)
	}

	// Return a copy
	return &Reservation{
		TaskID:    reservation.TaskID,
		WorkerID:  reservation.WorkerID,
		CpuCores:  reservation.CpuCores,
		MemoryMB:  reservation.MemoryMB,
		GpuUnits:  reservation.GpuUnits,
		CreatedAt: reservation.CreatedAt,
		ExpiresAt: reservation.ExpiresAt,
	}, nil
}

// Count returns the number of registered workers
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.workers)
}

// ReservationCount returns the number of active reservations
func (r *Registry) ReservationCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.reservations)
}
