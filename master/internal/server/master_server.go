package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"master/internal/db"
	pb "master/proto"
)

// MasterServer handles gRPC requests from workers
type MasterServer struct {
	pb.UnimplementedMasterWorkerServer

	workers  map[string]*WorkerState
	mu       sync.RWMutex
	workerDB *db.WorkerDB

	taskChan chan *TaskAssignment
}

// WorkerState tracks the current state of a worker
type WorkerState struct {
	Info          *pb.WorkerInfo
	LastHeartbeat int64
	IsActive      bool
	RunningTasks  map[string]bool
}

// TaskAssignment represents a task to be sent to a worker
type TaskAssignment struct {
	Task     *pb.Task
	WorkerID string
}

// NewMasterServer creates a new master server instance
func NewMasterServer(workerDB *db.WorkerDB) *MasterServer {
	return &MasterServer{
		workers:  make(map[string]*WorkerState),
		workerDB: workerDB,
		taskChan: make(chan *TaskAssignment, 100),
	}
}

// LoadWorkersFromDB loads registered workers from database into memory
func (s *MasterServer) LoadWorkersFromDB(ctx context.Context) error {
	if s.workerDB == nil {
		return nil // DB not configured, skip
	}

	workers, err := s.workerDB.GetAllWorkers(ctx)
	if err != nil {
		return fmt.Errorf("load workers from db: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, w := range workers {
		s.workers[w.WorkerID] = &WorkerState{
			Info: &pb.WorkerInfo{
				WorkerId:     w.WorkerID,
				WorkerIp:     w.WorkerIP,
				TotalCpu:     w.TotalCPU,
				TotalMemory:  w.TotalMemory,
				TotalStorage: w.TotalStorage,
				TotalGpu:     w.TotalGPU,
			},
			LastHeartbeat: w.LastHeartbeat,
			IsActive:      w.IsActive,
			RunningTasks:  make(map[string]bool),
		}
	}

	log.Printf("Loaded %d workers from database", len(workers))
	return nil
}

// ManualRegisterWorker manually registers a worker (called from CLI)
func (s *MasterServer) ManualRegisterWorker(ctx context.Context, workerID, workerIP string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	if _, exists := s.workers[workerID]; exists {
		return fmt.Errorf("worker %s already registered", workerID)
	}

	// Add to database
	if s.workerDB != nil {
		exists, err := s.workerDB.WorkerExists(ctx, workerID)
		if err != nil {
			return fmt.Errorf("check worker existence: %w", err)
		}
		if exists {
			return fmt.Errorf("worker %s already exists in database", workerID)
		}

		if err := s.workerDB.RegisterWorker(ctx, workerID, workerIP); err != nil {
			return fmt.Errorf("register worker in db: %w", err)
		}
	}

	// Add to memory with minimal info
	s.workers[workerID] = &WorkerState{
		Info: &pb.WorkerInfo{
			WorkerId: workerID,
			WorkerIp: workerIP,
			// Resource info will be filled when worker connects
		},
		IsActive:     false, // Not active until worker connects
		RunningTasks: make(map[string]bool),
	}

	log.Printf("Manually registered worker: %s (IP: %s)", workerID, workerIP)
	return nil
}

// UnregisterWorker removes a worker from the system
func (s *MasterServer) UnregisterWorker(ctx context.Context, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if exists
	if _, exists := s.workers[workerID]; !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	// Remove from database
	if s.workerDB != nil {
		if err := s.workerDB.UnregisterWorker(ctx, workerID); err != nil {
			return fmt.Errorf("unregister worker from db: %w", err)
		}
	}

	// Remove from memory
	delete(s.workers, workerID)

	log.Printf("Unregistered worker: %s", workerID)
	return nil
}

// RegisterWorker handles worker registration requests
// Workers can ONLY register if they have been manually pre-registered by admin
func (s *MasterServer) RegisterWorker(ctx context.Context, info *pb.WorkerInfo) (*pb.RegisterAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if worker was manually pre-registered by admin
	existingWorker, exists := s.workers[info.WorkerId]
	if !exists {
		// Worker NOT pre-registered - reject the connection
		log.Printf("❌ Rejected unauthorized worker registration attempt: %s (IP: %s)",
			info.WorkerId, info.WorkerIp)
		return &pb.RegisterAck{
			Success: false,
			Message: fmt.Sprintf("Worker %s is not authorized. Admin must register it first using: register %s <ip>",
				info.WorkerId, info.WorkerId),
		}, fmt.Errorf("worker %s not authorized - must be pre-registered by admin", info.WorkerId)
	}

	// Worker IS pre-registered - update with full specs
	log.Printf("✓ Pre-registered worker connecting: %s (IP: %s, CPU: %.2f, Memory: %.2f GB)",
		info.WorkerId, info.WorkerIp, info.TotalCpu, info.TotalMemory)

	existingWorker.Info = info
	existingWorker.IsActive = true
	existingWorker.LastHeartbeat = time.Now().Unix()

	// Update in database
	if s.workerDB != nil {
		if err := s.workerDB.UpdateWorkerInfo(ctx, info); err != nil {
			log.Printf("Warning: failed to update worker in db: %v", err)
		}
	}

	return &pb.RegisterAck{
		Success: true,
		Message: "Worker registered successfully",
	}, nil
}

// SendHeartbeat processes heartbeat messages from workers
func (s *MasterServer) SendHeartbeat(ctx context.Context, hb *pb.Heartbeat) (*pb.HeartbeatAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, exists := s.workers[hb.WorkerId]
	if !exists {
		return &pb.HeartbeatAck{Success: false}, fmt.Errorf("worker %s not registered", hb.WorkerId)
	}

	timestamp := time.Now().Unix()
	worker.LastHeartbeat = timestamp
	worker.IsActive = true

	// Update heartbeat in database
	if s.workerDB != nil {
		if err := s.workerDB.UpdateHeartbeat(ctx, hb.WorkerId, timestamp); err != nil {
			log.Printf("Warning: failed to update heartbeat in db: %v", err)
		}
	}

	log.Printf("Heartbeat from %s: CPU=%.2f%%, Memory=%.2f%%, Running Tasks=%d",
		hb.WorkerId, hb.CpuUsage, hb.MemoryUsage, len(hb.RunningTasks))

	return &pb.HeartbeatAck{Success: true}, nil
}

// ReportTaskCompletion handles task completion reports from workers
func (s *MasterServer) ReportTaskCompletion(ctx context.Context, result *pb.TaskResult) (*pb.Ack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("Task completion: %s from worker %s [Status: %s]",
		result.TaskId, result.WorkerId, result.Status)

	if len(result.Logs) > 0 {
		log.Printf("Task logs:\n%s", result.Logs)
	}

	// Remove task from worker's running tasks
	if worker, exists := s.workers[result.WorkerId]; exists {
		delete(worker.RunningTasks, result.TaskId)
	}

	// TODO: Store result in MongoDB

	return &pb.Ack{
		Success: true,
		Message: "Task result received",
	}, nil
}

// GetWorkers returns current worker states (for CLI)
func (s *MasterServer) GetWorkers() map[string]*WorkerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make(map[string]*WorkerState)
	for k, v := range s.workers {
		workers[k] = v
	}
	return workers
}

// NotImplemented RPCs (for future implementation)
func (s *MasterServer) AssignTask(ctx context.Context, task *pb.Task) (*pb.TaskAck, error) {
	return &pb.TaskAck{Success: false, Message: "Not implemented"}, nil
}

func (s *MasterServer) CancelTask(ctx context.Context, taskID *pb.TaskID) (*pb.TaskAck, error) {
	return &pb.TaskAck{Success: false, Message: "Not implemented"}, nil
}
