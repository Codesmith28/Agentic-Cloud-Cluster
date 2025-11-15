package server

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"worker/internal/executor"
	"worker/internal/telemetry"
	pb "worker/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// WorkerServer handles incoming gRPC requests from master
type WorkerServer struct {
	pb.UnimplementedMasterWorkerServer

	workerID         string
	executor         *executor.TaskExecutor
	monitor          *telemetry.Monitor
	masterAddr       string
	masterRegistered bool
	mu               sync.RWMutex
}

// NewWorkerServer creates a new worker server instance
func NewWorkerServer(workerID string, monitor *telemetry.Monitor) (*WorkerServer, error) {
	exec, err := executor.NewTaskExecutor()
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	return &WorkerServer{
		workerID:         workerID,
		executor:         exec,
		monitor:          monitor,
		masterAddr:       "", // Will be set when master registers
		masterRegistered: false,
		mu:               sync.RWMutex{},
	}, nil
}

// MasterRegister handles master registration from master node
func (s *WorkerServer) MasterRegister(ctx context.Context, masterInfo *pb.MasterInfo) (*pb.RegisterAck, error) {
	log.Printf("Master registration request from: %s (%s)", masterInfo.MasterId, masterInfo.MasterAddress)

	s.mu.Lock()
	s.masterAddr = masterInfo.MasterAddress
	s.masterRegistered = true
	s.mu.Unlock()

	// Update monitor with master address
	s.monitor.SetMasterAddress(s.masterAddr)

	// Now that we know the master, start the registration process
	go s.registerWithMaster()

	return &pb.RegisterAck{
		Success: true,
		Message: fmt.Sprintf("Worker %s registered with master %s", s.workerID, masterInfo.MasterId),
	}, nil
}

// registerWithMaster registers this worker with the master (called after master registers with us)
func (s *WorkerServer) registerWithMaster() {
	s.mu.RLock()
	masterAddr := s.masterAddr
	s.mu.RUnlock()

	if masterAddr == "" {
		log.Printf("Cannot register with master: no master address set")
		return
	}

	log.Printf("Registering worker %s with master at %s", s.workerID, masterAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, masterAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		log.Printf("Failed to connect to master for registration: %v", err)
		return
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)

	// Get system resources
	workerInfo := &pb.WorkerInfo{
		WorkerId:     s.workerID,
		WorkerIp:     "", // Will be filled by master based on connection
		TotalCpu:     float64(runtime.NumCPU()),
		TotalMemory:  8.0,   // Simplified - in real implementation, get actual memory
		TotalStorage: 100.0, // Simplified
		TotalGpu:     0.0,   // Simplified
	}

	ack, err := client.RegisterWorker(ctx, workerInfo)
	if err != nil {
		log.Printf("Failed to register with master: %v", err)
		return
	}

	if ack.Success {
		log.Printf("✓ Successfully registered with master: %s", ack.Message)
	} else {
		log.Printf("❌ Master rejected registration: %s", ack.Message)
	}
}

// AssignTask handles task assignment from master
func (s *WorkerServer) AssignTask(ctx context.Context, task *pb.Task) (*pb.TaskAck, error) {
	s.mu.RLock()
	registered := s.masterRegistered
	s.mu.RUnlock()

	if !registered {
		return &pb.TaskAck{
			Success: false,
			Message: "Master not registered yet",
		}, nil
	}

	// Print comprehensive task details with all system requirements
	log.Println(" ")
	log.Println("═══════════════════════════════════════════════════════")
	log.Println("  📥 TASK RECEIVED FROM MASTER")
	log.Println("═══════════════════════════════════════════════════════")
	log.Printf("  Task ID:           %s", task.TaskId)
	log.Printf("  Docker Image:      %s", task.DockerImage)
	log.Printf("  Command:           %s", task.Command)
	log.Printf("  Target Worker:     %s", task.TargetWorkerId)
	log.Println("───────────────────────────────────────────────────────")
	log.Println("  System Requirements:")
	log.Printf("    • CPU Cores:     %.2f cores", task.ReqCpu)
	log.Printf("    • Memory:        %.2f GB", task.ReqMemory)
	log.Printf("    • Storage:       %.2f GB", task.ReqStorage)
	log.Printf("    • GPU Cores:     %.2f cores", task.ReqGpu)
	log.Println("═══════════════════════════════════════════════════════")
	log.Printf("  ✓ Task accepted - Starting execution...")
	log.Println("═══════════════════════════════════════════════════════")
	log.Println("")

	// Add task to monitoring
	s.monitor.AddTask(task.TaskId, task.ReqCpu, task.ReqMemory, task.ReqGpu)

	// Execute task in background with a fresh context (not tied to RPC timeout)
	go s.executeTask(task)

	return &pb.TaskAck{
		Success: true,
		Message: "Task accepted",
	}, nil
}

// executeTask runs the task and reports result
func (s *WorkerServer) executeTask(task *pb.Task) {
	// Create a new context for task execution (not tied to RPC timeout)
	ctx := context.Background()

	// Execute the task with resource constraints
	result := s.executor.ExecuteTask(ctx, task.TaskId, task.DockerImage, task.Command,
		task.ReqCpu, task.ReqMemory, task.ReqGpu)

	// Remove from monitoring
	s.monitor.RemoveTask(task.TaskId)

	// Report result to master with a timeout
	reportCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	taskResult := &pb.TaskResult{
		TaskId:         task.TaskId,
		WorkerId:       s.workerID,
		Status:         result.Status,
		Logs:           result.Logs,
		ResultLocation: "", // Not implemented yet
	}

	s.mu.RLock()
	masterAddr := s.masterAddr
	s.mu.RUnlock()

	if err := telemetry.ReportTaskResult(reportCtx, masterAddr, taskResult); err != nil {
		log.Printf("Failed to report task result: %v", err)
	}
}

// CancelTask handles task cancellation requests
func (s *WorkerServer) CancelTask(ctx context.Context, taskID *pb.TaskID) (*pb.TaskAck, error) {
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("  🛑 TASK CANCELLATION REQUEST")
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("  Task ID: %s", taskID.TaskId)
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Cancel the task using executor
	if err := s.executor.CancelTask(ctx, taskID.TaskId); err != nil {
		log.Printf("  ✗ Failed to cancel task: %v", err)
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Failed to cancel task: %v", err),
		}, nil
	}

	// Remove from monitoring
	s.monitor.RemoveTask(taskID.TaskId)

	log.Printf("  ✓ Task cancelled successfully")
	log.Printf("  ✓ Container stopped")
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Report cancellation to master asynchronously (fire-and-forget with retries)
	// This provides redundancy - master already updated DB, this is confirmation
	go s.reportCancellationWithRetry(taskID.TaskId, 3)

	return &pb.TaskAck{
		Success: true,
		Message: "Task cancelled",
	}, nil
}

// reportCancellationWithRetry reports task cancellation to master with retry logic
// This is a confirmation/redundancy mechanism - master already updated DB optimistically
func (s *WorkerServer) reportCancellationWithRetry(taskID string, maxRetries int) error {
	s.mu.RLock()
	masterAddr := s.masterAddr
	s.mu.RUnlock()

	if masterAddr == "" {
		log.Printf("[Task %s] ⚠ Cannot report cancellation: no master address", taskID)
		return fmt.Errorf("no master address configured")
	}

	taskResult := &pb.TaskResult{
		TaskId:   taskID,
		WorkerId: s.workerID,
		Status:   "cancelled",
		Logs:     "Task was cancelled by user request",
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := telemetry.ReportTaskResult(ctx, masterAddr, taskResult)
		cancel()

		if err == nil {
			log.Printf("[Task %s] ✓ Cancellation confirmed with master (attempt %d/%d)", taskID, attempt, maxRetries)
			log.Printf("[Task %s] ✓ Result stored in RESULTS collection", taskID)
			return nil
		}

		lastErr = err
		log.Printf("[Task %s] ⚠ Failed to confirm cancellation with master (attempt %d/%d): %v", taskID, attempt, maxRetries, err)

		if attempt < maxRetries {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("[Task %s] Retrying in %v...", taskID, backoff)
			time.Sleep(backoff)
		}
	}

	log.Printf("[Task %s] ⚠ Failed to confirm cancellation after %d attempts", taskID, maxRetries)
	log.Printf("[Task %s] ℹ Database was already updated by master - this is not critical", taskID)
	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// StreamTaskLogs streams live logs for a task
func (s *WorkerServer) StreamTaskLogs(req *pb.TaskLogRequest, stream pb.MasterWorker_StreamTaskLogsServer) error {
	log.Printf("Log stream request for task: %s (user: %s, follow: %v)", req.TaskId, req.UserId, req.Follow)

	// Verify task exists on this worker
	containerID, exists := s.executor.GetContainerID(req.TaskId)
	if !exists {
		// Task not running, send error
		return stream.Send(&pb.LogChunk{
			TaskId:     req.TaskId,
			Content:    "Task not found or not running on this worker",
			IsComplete: true,
			Status:     "not_found",
		})
	}

	// Get container status
	status, err := s.executor.GetContainerStatus(stream.Context(), containerID)
	if err != nil {
		return stream.Send(&pb.LogChunk{
			TaskId:     req.TaskId,
			Content:    fmt.Sprintf("Error getting container status: %v", err),
			IsComplete: true,
			Status:     "error",
		})
	}

	// Stream logs using taskID (the broadcaster will handle multiple subscribers)
	logChan, errChan := s.executor.StreamLogs(stream.Context(), req.TaskId)

	for {
		select {
		case line, ok := <-logChan:
			if !ok {
				// Logs finished, check final status
				finalStatus, _ := s.executor.GetContainerStatus(stream.Context(), containerID)
				return stream.Send(&pb.LogChunk{
					TaskId:     req.TaskId,
					Content:    "",
					IsComplete: true,
					Status:     finalStatus,
				})
			}

			// Send log line
			if err := stream.Send(&pb.LogChunk{
				TaskId:     req.TaskId,
				Content:    line,
				Timestamp:  "", // Could add timestamp from LogLine
				IsComplete: false,
				Status:     status,
			}); err != nil {
				return fmt.Errorf("failed to send log chunk: %w", err)
			}

		case err := <-errChan:
			if err != nil {
				return stream.Send(&pb.LogChunk{
					TaskId:     req.TaskId,
					Content:    fmt.Sprintf("Error streaming logs: %v", err),
					IsComplete: true,
					Status:     "error",
				})
			}

		case <-stream.Context().Done():
			log.Printf("Client disconnected from log stream for task: %s", req.TaskId)
			return stream.Context().Err()
		}
	}
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
