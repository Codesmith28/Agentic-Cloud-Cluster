package server

import (
	"context"
	"fmt"
	"log"
	"sync"

	pb "master/proto/pb"
)

// MasterServer handles gRPC requests from workers
type MasterServer struct {
	pb.UnimplementedMasterWorkerServer

	workers map[string]*WorkerState
	mu      sync.RWMutex

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
func NewMasterServer() *MasterServer {
	return &MasterServer{
		workers:  make(map[string]*WorkerState),
		taskChan: make(chan *TaskAssignment, 100),
	}
}

// RegisterWorker handles worker registration requests
func (s *MasterServer) RegisterWorker(ctx context.Context, info *pb.WorkerInfo) (*pb.RegisterAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("Worker registration: %s (IP: %s, CPU: %.2f, Memory: %.2f GB)",
		info.WorkerId, info.WorkerIp, info.TotalCpu, info.TotalMemory)

	s.workers[info.WorkerId] = &WorkerState{
		Info:         info,
		IsActive:     true,
		RunningTasks: make(map[string]bool),
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

	worker.LastHeartbeat = ctx.Value("timestamp").(int64)
	worker.IsActive = true

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
