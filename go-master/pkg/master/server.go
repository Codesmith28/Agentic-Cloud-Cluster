package master

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
	"github.com/Codesmith28/CloudAI/pkg/taskqueue"
	"github.com/Codesmith28/CloudAI/pkg/workerregistry"
	"github.com/google/uuid"
)

// MasterServer implements SchedulerServiceServer
type MasterServer struct {
	pb.UnimplementedSchedulerServiceServer
	taskQueue *taskqueue.TaskQueue
	registry  *workerregistry.Registry
}

// NewMasterServer creates a new master server
func NewMasterServer(tq *taskqueue.TaskQueue, reg *workerregistry.Registry) *MasterServer {
	return &MasterServer{
		taskQueue: tq,
		registry:  reg,
	}
}

// SubmitTask handles task submission from clients
func (s *MasterServer) SubmitTask(ctx context.Context, req *pb.SubmitTaskRequest) (*pb.SubmitTaskResponse, error) {
	if req.Task == nil {
		return &pb.SubmitTaskResponse{
			TaskId:  "",
			Message: "task cannot be nil",
		}, fmt.Errorf("task cannot be nil")
	}

	// Generate task ID if not provided
	if req.Task.Id == "" {
		req.Task.Id = uuid.New().String()
	}

	log.Printf("Received task submission: %s (type=%s, cpu=%.2f, mem=%d MB, gpu=%d, priority=%d)",
		req.Task.Id, req.Task.TaskType, req.Task.CpuReq, req.Task.MemMb, req.Task.GpuReq, req.Task.Priority)

	// Enqueue the task
	if err := s.taskQueue.Enqueue(req.Task); err != nil {
		return &pb.SubmitTaskResponse{
			TaskId:  req.Task.Id,
			Message: fmt.Sprintf("failed to enqueue task: %v", err),
		}, err
	}

	return &pb.SubmitTaskResponse{
		TaskId:  req.Task.Id,
		Message: "task submitted successfully",
	}, nil
}

// GetTaskStatus retrieves the current status of a task
func (s *MasterServer) GetTaskStatus(ctx context.Context, req *pb.GetTaskStatusRequest) (*pb.GetTaskStatusResponse, error) {
	status, err := s.taskQueue.GetStatus(req.TaskId)
	if err != nil {
		return &pb.GetTaskStatusResponse{
			TaskId:  req.TaskId,
			Status:  "UNKNOWN",
			Message: err.Error(),
		}, nil
	}

	return &pb.GetTaskStatusResponse{
		TaskId:  req.TaskId,
		Status:  status,
		Message: "ok",
	}, nil
}

// CancelTask cancels a pending or running task
func (s *MasterServer) CancelTask(ctx context.Context, req *pb.CancelTaskRequest) (*pb.CancelTaskResponse, error) {
	log.Printf("Cancelling task: %s", req.TaskId)

	if err := s.taskQueue.Remove(req.TaskId); err != nil {
		return &pb.CancelTaskResponse{
			Success: false,
			Message: fmt.Sprintf("failed to cancel task: %v", err),
		}, nil
	}

	return &pb.CancelTaskResponse{
		Success: true,
		Message: "task cancelled successfully",
	}, nil
}

// RegisterWorker handles worker registration
func (s *MasterServer) RegisterWorker(ctx context.Context, req *pb.RegisterWorkerRequest) (*pb.RegisterWorkerResponse, error) {
	if req.Worker == nil {
		return &pb.RegisterWorkerResponse{
			Success: false,
			Message: "worker cannot be nil",
		}, fmt.Errorf("worker cannot be nil")
	}

	log.Printf("Registering worker: %s (CPU=%.2f, Mem=%d MB, GPU=%d)",
		req.Worker.Id, req.Worker.TotalCpu, req.Worker.TotalMem, req.Worker.Gpus)

	// Set initial free resources to total resources
	if req.Worker.FreeCpu == 0 {
		req.Worker.FreeCpu = req.Worker.TotalCpu
	}
	if req.Worker.FreeMem == 0 {
		req.Worker.FreeMem = req.Worker.TotalMem
	}
	if req.Worker.FreeGpus == 0 {
		req.Worker.FreeGpus = req.Worker.Gpus
	}
	req.Worker.LastSeenUnix = time.Now().Unix()

	if err := s.registry.UpdateHeartbeat(req.Worker); err != nil {
		return &pb.RegisterWorkerResponse{
			Success: false,
			Message: fmt.Sprintf("failed to register worker: %v", err),
		}, err
	}

	return &pb.RegisterWorkerResponse{
		Success: true,
		Message: "worker registered successfully",
	}, nil
}

// Heartbeat handles periodic worker heartbeats
func (s *MasterServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if req.Worker == nil {
		return &pb.HeartbeatResponse{
			Success: false,
		}, fmt.Errorf("worker cannot be nil")
	}

	req.Worker.LastSeenUnix = time.Now().Unix()

	if err := s.registry.UpdateHeartbeat(req.Worker); err != nil {
		return &pb.HeartbeatResponse{
			Success: false,
		}, err
	}

	return &pb.HeartbeatResponse{
		Success: true,
	}, nil
}

// ListWorkers returns all active workers
func (s *MasterServer) ListWorkers(ctx context.Context, req *pb.ListWorkersRequest) (*pb.ListWorkersResponse, error) {
	workers := s.registry.GetSnapshot()
	return &pb.ListWorkersResponse{
		Workers: workers,
	}, nil
}

// ReportTaskCompletion handles task completion reports from workers
func (s *MasterServer) ReportTaskCompletion(ctx context.Context, req *pb.TaskCompletionRequest) (*pb.TaskCompletionResponse, error) {
	log.Printf("Task %s completed on worker %s: success=%v, duration=%ds",
		req.TaskId, req.WorkerId, req.Success, req.ActualDurationSec)

	// Update task status
	var status string
	if req.Success {
		status = "COMPLETED"
	} else {
		status = "FAILED"
	}

	if err := s.taskQueue.UpdateStatus(req.TaskId, status); err != nil {
		log.Printf("Warning: failed to update task status: %v", err)
	}

	// Release resources reserved for this task
	if err := s.registry.Release(req.TaskId); err != nil {
		log.Printf("Warning: failed to release resources: %v", err)
	}

	return &pb.TaskCompletionResponse{
		Acknowledged: true,
	}, nil
}
