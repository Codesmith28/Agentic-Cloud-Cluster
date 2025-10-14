package server

import (
	"context"
	"fmt"
	"log"

	"worker/internal/executor"
	"worker/internal/telemetry"
	pb "worker/proto"
)

// WorkerServer handles incoming gRPC requests from master
type WorkerServer struct {
	pb.UnimplementedMasterWorkerServer

	workerID   string
	executor   *executor.TaskExecutor
	monitor    *telemetry.Monitor
	masterAddr string
}

// NewWorkerServer creates a new worker server instance
func NewWorkerServer(workerID, masterAddr string, monitor *telemetry.Monitor) (*WorkerServer, error) {
	exec, err := executor.NewTaskExecutor()
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	return &WorkerServer{
		workerID:   workerID,
		executor:   exec,
		monitor:    monitor,
		masterAddr: masterAddr,
	}, nil
}

// AssignTask handles task assignment from master
func (s *WorkerServer) AssignTask(ctx context.Context, task *pb.Task) (*pb.TaskAck, error) {
	log.Printf("Received task assignment: %s (Image: %s)", task.TaskId, task.DockerImage)

	// Add task to monitoring
	s.monitor.AddTask(task.TaskId, task.ReqCpu, task.ReqMemory)

	// Execute task in background
	go s.executeTask(ctx, task)

	return &pb.TaskAck{
		Success: true,
		Message: "Task accepted",
	}, nil
}

// executeTask runs the task and reports result
func (s *WorkerServer) executeTask(ctx context.Context, task *pb.Task) {
	// Execute the task
	result := s.executor.ExecuteTask(ctx, task.TaskId, task.DockerImage)

	// Remove from monitoring
	s.monitor.RemoveTask(task.TaskId)

	// Report result to master
	taskResult := &pb.TaskResult{
		TaskId:         task.TaskId,
		WorkerId:       s.workerID,
		Status:         result.Status,
		Logs:           result.Logs,
		ResultLocation: "", // Not implemented yet
	}

	if err := telemetry.ReportTaskResult(ctx, s.masterAddr, taskResult); err != nil {
		log.Printf("Failed to report task result: %v", err)
	}
}

// CancelTask handles task cancellation requests (not implemented)
func (s *WorkerServer) CancelTask(ctx context.Context, taskID *pb.TaskID) (*pb.TaskAck, error) {
	return &pb.TaskAck{
		Success: false,
		Message: "Task cancellation not implemented",
	}, nil
}

// Close cleans up resources
func (s *WorkerServer) Close() error {
	return s.executor.Close()
}

// Not implemented RPCs (worker doesn't receive these)
func (s *WorkerServer) RegisterWorker(ctx context.Context, info *pb.WorkerInfo) (*pb.RegisterAck, error) {
	return &pb.RegisterAck{Success: false, Message: "Not applicable"}, nil
}

func (s *WorkerServer) SendHeartbeat(ctx context.Context, hb *pb.Heartbeat) (*pb.HeartbeatAck, error) {
	return &pb.HeartbeatAck{Success: false}, nil
}

func (s *WorkerServer) ReportTaskCompletion(ctx context.Context, result *pb.TaskResult) (*pb.Ack, error) {
	return &pb.Ack{Success: false, Message: "Not applicable"}, nil
}
