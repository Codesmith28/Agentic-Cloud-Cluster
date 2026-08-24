package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"master/internal/db"
	mastermetrics "master/internal/metrics"
	pb "master/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RegisterWorker handles worker registration requests
func (s *MasterServer) RegisterWorker(ctx context.Context, info *pb.WorkerInfo) (*pb.RegisterAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existingWorker, exists := s.workers[info.WorkerId]
	if !exists {
		log.Printf("❌ Rejected unauthorized worker registration attempt: %s (Address: %s)",
			info.WorkerId, info.WorkerIp)
		return &pb.RegisterAck{
			Success: false,
			Message: fmt.Sprintf("Worker %s is not authorized. Admin must register it first using: register %s <ip:port>",
				info.WorkerId, info.WorkerId),
		}, fmt.Errorf("worker %s not authorized - must be pre-registered by admin", info.WorkerId)
	}

	isNewConnection := existingWorker.Info.TotalCpu == 0 || !existingWorker.IsActive

	if existingWorker.RunningTasks == nil {
		existingWorker.RunningTasks = make(map[string]bool)
	}

	preservedIP := existingWorker.Info.WorkerIp
	reportedIP := info.WorkerIp
	existingWorker.Info = info

	if preservedIP != "" {
		existingWorker.Info.WorkerIp = preservedIP
		if reportedIP != "" && reportedIP != preservedIP {
			log.Printf("ℹ️ Worker %s reported address %s; keeping configured address %s",
				info.WorkerId, reportedIP, preservedIP)
		}
	} else if existingWorker.Info.WorkerIp == "" {
		existingWorker.Info.WorkerIp = reportedIP
	}

	existingWorker.IsActive = true
	existingWorker.LastHeartbeat = time.Now().Unix()

	if isNewConnection {
		log.Printf("🔄 Worker %s connected with new specs, reconciling resources...", info.WorkerId)
		existingWorker.AllocatedCPU = 0.0
		existingWorker.AllocatedMemory = 0.0
		existingWorker.AllocatedStorage = 0.0
		existingWorker.AvailableCPU = info.TotalCpu
		existingWorker.AvailableMemory = info.TotalMemory
		existingWorker.AvailableStorage = info.TotalStorage
		s.reconcileSingleWorker(ctx, info.WorkerId, existingWorker)
	} else {
		existingWorker.AvailableCPU = info.TotalCpu - existingWorker.AllocatedCPU
		existingWorker.AvailableMemory = info.TotalMemory - existingWorker.AllocatedMemory
		existingWorker.AvailableStorage = info.TotalStorage - existingWorker.AllocatedStorage
	}

	if s.workerDB != nil {
		if err := s.workerDB.UpdateWorkerInfo(ctx, existingWorker.Info); err != nil {
			log.Printf("Warning: failed to update worker in db: %v", err)
		}
	}

	if s.telemetryManager != nil {
		s.telemetryManager.RegisterWorker(info.WorkerId)
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

	worker.LatestCPU = normalizeUsageFraction(hb.CpuUsage)
	worker.LatestMemory = normalizeUsageFraction(hb.MemoryUsage)
	worker.LatestStorage = normalizeUsageFraction(hb.StorageUsage)
	worker.TaskCount = len(hb.RunningTasks)
	if worker.RunningTasks == nil {
		worker.RunningTasks = make(map[string]bool)
	}
	for taskID := range worker.RunningTasks {
		delete(worker.RunningTasks, taskID)
	}
	for _, runningTask := range hb.RunningTasks {
		if runningTask == nil || runningTask.TaskId == "" {
			continue
		}
		worker.RunningTasks[runningTask.TaskId] = true
		if s.attemptDB != nil && runningTask.AttemptId != "" {
			if err := s.attemptDB.TouchHeartbeat(ctx, runningTask.AttemptId, timestamp); err != nil {
				log.Printf("Warning: failed to update attempt heartbeat for %s: %v", runningTask.AttemptId, err)
			}
		}
	}

	if s.workerDB != nil {
		if err := s.workerDB.UpdateHeartbeat(ctx, hb.WorkerId, timestamp); err != nil {
			log.Printf("Warning: failed to update heartbeat in db: %v", err)
		}
	}

	if s.telemetryManager != nil {
		if err := s.telemetryManager.ProcessHeartbeat(hb); err != nil {
			log.Printf("Warning: failed to process telemetry: %v", err)
		}
	}

	return &pb.HeartbeatAck{Success: true}, nil
}

// ReportTaskCompletion handles task completion reports from workers
func (s *MasterServer) ReportTaskCompletion(ctx context.Context, result *pb.TaskResult) (*pb.Ack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("📥 Task completion report received: %s from %s [Status: %s]", result.TaskId, result.WorkerId, result.Status)

	var taskResources *db.Task
	if s.taskDB != nil {
		task, err := s.taskDB.GetTask(context.Background(), result.TaskId)
		if err != nil {
			log.Printf("  ⚠ Warning: Failed to get task info for resource release: %v", err)
		} else {
			taskResources = task
		}
	}
	if taskResources == nil {
		s.taskResourceCacheMu.Lock()
		taskResources = s.taskResourceCache[result.TaskId]
		delete(s.taskResourceCache, result.TaskId)
		s.taskResourceCacheMu.Unlock()
	}

	currentAttemptID := ""
	if taskResources != nil {
		currentAttemptID = taskResources.CurrentAttemptID
	}
	resultAttemptID := result.AttemptId
	if resultAttemptID == "" {
		resultAttemptID = currentAttemptID
	}
	if s.attemptDB != nil && resultAttemptID != "" {
		if attemptRecord, err := s.attemptDB.GetAttempt(context.Background(), resultAttemptID); err == nil && attemptRecord != nil {
			if shouldIgnoreAttemptResult(currentAttemptID, resultAttemptID, attemptRecord.Status) {
				mastermetrics.Get().IncStaleResult("late_result")
				if err := s.attemptDB.CompleteAttempt(context.Background(), resultAttemptID, db.AttemptStatusStale, "late_result", result.Status, result.Logs, result.ResultLocation, result.OutputFiles); err != nil {
					log.Printf("  ⚠ Warning: Failed to update late attempt %s: %v", resultAttemptID, err)
				}
				log.Printf("  ℹ Ignoring late result for recovered task %s from attempt %s", result.TaskId, resultAttemptID)
				return &pb.Ack{
					Success: true,
					Message: "Late result ignored because attempt was already recovered",
				}, nil
			}
		}
	}
	if taskResources != nil && shouldIgnoreAttemptResult(currentAttemptID, resultAttemptID, "") {
		if s.attemptDB != nil {
			mastermetrics.Get().IncStaleResult("stale_attempt")
			if err := s.attemptDB.CompleteAttempt(context.Background(), resultAttemptID, db.AttemptStatusStale, "late_result", result.Status, result.Logs, result.ResultLocation, result.OutputFiles); err != nil {
				log.Printf("  ⚠ Warning: Failed to mark stale attempt %s: %v", resultAttemptID, err)
			}
		}
		log.Printf("  ℹ Ignoring stale result for task %s from attempt %s (current attempt: %s)", result.TaskId, resultAttemptID, currentAttemptID)
		return &pb.Ack{
			Success: true,
			Message: "Stale attempt result recorded for audit and ignored",
		}, nil
	}

	if worker, exists := s.workers[result.WorkerId]; exists {
		if worker.RunningTasks != nil {
			delete(worker.RunningTasks, result.TaskId)
		}

		if taskResources != nil {
			worker.AllocatedCPU -= taskResources.ReqCPU
			worker.AllocatedMemory -= taskResources.ReqMemory
			worker.AllocatedStorage -= taskResources.ReqStorage
			worker.AvailableCPU += taskResources.ReqCPU
			worker.AvailableMemory += taskResources.ReqMemory
			worker.AvailableStorage += taskResources.ReqStorage

			if worker.AllocatedCPU < 0 {
				worker.AllocatedCPU = 0
			}
			if worker.AllocatedMemory < 0 {
				worker.AllocatedMemory = 0
			}
			if worker.AllocatedStorage < 0 {
				worker.AllocatedStorage = 0
			}

			if s.workerDB != nil {
				if err := s.workerDB.ReleaseResources(ctx, result.WorkerId,
					taskResources.ReqCPU, taskResources.ReqMemory,
					taskResources.ReqStorage); err != nil {
					log.Printf("  ⚠ Warning: Failed to release resources in database: %v", err)
				}
			}
		}
	}
	if s.assignmentDB != nil {
		if err := s.assignmentDB.DeleteAssignment(ctx, result.TaskId); err != nil && !strings.Contains(err.Error(), "not found") {
			log.Printf("  ⚠ Warning: Failed to delete assignment for completed task: %v", err)
		}
	}

	if s.taskDB != nil {
		existingTask, err := s.taskDB.GetTask(context.Background(), result.TaskId)
		if err != nil {
			log.Printf("  ⚠ Warning: Failed to get task status from database: %v", err)
		} else if existingTask != nil && existingTask.Status == "cancelled" {
			if s.resultDB != nil {
				existingResult, err := s.resultDB.GetResult(context.Background(), result.TaskId)
				if err == nil && existingResult != nil {
					s.reportSchedulingOutcomeAsync(taskResources, result)
					return &pb.Ack{
						Success: true,
						Message: "Task result received (status preserved as cancelled, result already stored)",
					}, nil
				}
				taskResult := &db.TaskResult{
					TaskID:   result.TaskId,
					WorkerID: result.WorkerId,
					Status:   "cancelled",
					Logs:     result.Logs,
				}
				if err := s.resultDB.CreateResult(context.Background(), taskResult); err != nil {
					log.Printf("  ⚠ Warning: Failed to store task result: %v", err)
				}
			}
			if s.attemptDB != nil && resultAttemptID != "" {
				if err := s.attemptDB.CompleteAttempt(context.Background(), resultAttemptID, db.AttemptStatusCancelled, "", result.Status, result.Logs, result.ResultLocation, result.OutputFiles); err != nil {
					log.Printf("  ⚠ Warning: Failed to finalize cancelled attempt %s: %v", resultAttemptID, err)
				}
			}
			s.reportSchedulingOutcomeAsync(taskResources, result)
			return &pb.Ack{
				Success: true,
				Message: "Task result received (status preserved as cancelled)",
			}, nil
		}

		status := "completed"
		attemptStatus := db.AttemptStatusCompleted
		failureReason := ""
		if result.Status == "cancelled" {
			status = "cancelled"
			attemptStatus = db.AttemptStatusCancelled
		} else if result.Status != "success" {
			status = "failed"
			attemptStatus = db.AttemptStatusFailed
			failureReason = "container_failed"
		}

		if err := s.taskDB.UpdateTaskStatus(context.Background(), result.TaskId, status); err != nil {
			log.Printf("  ⚠ Warning: Failed to update task status in database: %v", err)
			if result.Status != "cancelled" {
				return &pb.Ack{
					Success: false,
					Message: fmt.Sprintf("Failed to update task status: %v", err),
				}, nil
			}
		}

		if s.attemptDB != nil && resultAttemptID != "" {
			if err := s.attemptDB.CompleteAttempt(context.Background(), resultAttemptID, attemptStatus, failureReason, result.Status, result.Logs, result.ResultLocation, result.OutputFiles); err != nil {
				log.Printf("  ⚠ Warning: Failed to finalize attempt %s: %v", resultAttemptID, err)
			}
		}
		taskType := "unknown"
		if taskResources != nil {
			taskType = taskResources.TaskType
		}
		mastermetrics.Get().IncTaskTerminal(status, taskType)
	}

	if s.resultDB != nil {
		taskResult := &db.TaskResult{
			TaskID:   result.TaskId,
			WorkerID: result.WorkerId,
			Status:   result.Status,
			Logs:     result.Logs,
		}
		if err := s.resultDB.CreateResult(context.Background(), taskResult); err != nil {
			log.Printf("  ⚠ Warning: Failed to store task result: %v", err)
		}
	}

	s.reportSchedulingOutcomeAsync(taskResources, result)

	return &pb.Ack{
		Success: true,
		Message: "Task result received and processed",
	}, nil
}

// UploadTaskFiles handles file uploads from workers via streaming RPC
func (s *MasterServer) UploadTaskFiles(stream pb.MasterWorker_UploadTaskFilesServer) error {
	if s.fileStorage == nil {
		return stream.SendAndClose(&pb.FileUploadAck{
			Success:       false,
			Message:       "File storage service not available",
			FilesReceived: 0,
		})
	}

	metadata, err := s.fileStorage.ReceiveFileStream(stream)
	if err != nil {
		return stream.SendAndClose(&pb.FileUploadAck{
			Success:       false,
			Message:       fmt.Sprintf("Failed to receive files: %v", err),
			FilesReceived: 0,
		})
	}

	if s.fileMetadataDB != nil {
		dbMetadata := &db.FileMetadata{
			UserID:      metadata.UserID,
			TaskID:      metadata.TaskID,
			TaskName:    metadata.TaskName,
			Timestamp:   metadata.Timestamp,
			FilePaths:   metadata.FilePaths,
			StoragePath: metadata.StoragePath,
		}
		if err := s.fileMetadataDB.CreateFileMetadata(context.Background(), dbMetadata); err != nil {
			log.Printf("  ⚠ Warning: Failed to store file metadata in database: %v", err)
		}
	}

	return stream.SendAndClose(&pb.FileUploadAck{
		Success:       true,
		Message:       "Files uploaded successfully",
		FilesReceived: int32(len(metadata.FilePaths)),
	})
}

// SubmitTask handles task submission and enqueues it for scheduling
func (s *MasterServer) SubmitTask(ctx context.Context, task *pb.Task) (*pb.TaskAck, error) {
	if task == nil {
		return &pb.TaskAck{
			Success: false,
			Message: "task payload is required",
		}, nil
	}

	taskMeta := normalizeTaskForScheduling(task)

	if s.taskDB != nil {
		dbTask := &db.Task{
			TaskID:        task.TaskId,
			UserID:        task.UserId,
			TaskName:      task.TaskName,
			SubmittedAt:   task.SubmittedAt,
			DockerImage:   task.DockerImage,
			Command:       task.Command,
			ReqCPU:        task.ReqCpu,
			ReqMemory:     task.ReqMemory,
			ReqStorage:    task.ReqStorage,
			TaskType:      taskMeta.taskType,
			SLAMultiplier: task.SlaMultiplier,
			Tau:           taskMeta.tau,
			Deadline:      taskMeta.deadline,
			Status:        "queued",
		}
		if err := s.taskDB.CreateTask(ctx, dbTask); err != nil {
			log.Printf("Warning: Failed to store task in database: %v", err)
		}
	}

	s.EnqueueTask(task, "Task submitted to queue for scheduling")

	s.queueMu.RLock()
	position := len(s.taskQueue)
	s.queueMu.RUnlock()

	return &pb.TaskAck{
		Success: true,
		Message: fmt.Sprintf("Task submitted successfully. Queue position: %d. Scheduler will assign it to an available worker.", position),
	}, nil
}

// AssignTask redirects to SubmitTask maintaining backwards compatibility
func (s *MasterServer) AssignTask(ctx context.Context, task *pb.Task) (*pb.TaskAck, error) {
	return s.SubmitTask(ctx, task)
}

// StreamTaskLogs handles gRPC streaming of task logs
func (s *MasterServer) StreamTaskLogs(req *pb.TaskLogRequest, stream pb.MasterWorker_StreamTaskLogsServer) error {
	return fmt.Errorf("StreamTaskLogs should be called on worker, not master")
}

// CancelTask handles cancellation of a running or queued task
func (s *MasterServer) CancelTask(ctx context.Context, taskID *pb.TaskID) (*pb.TaskAck, error) {
	if s.removeQueuedTaskByID(taskID.TaskId) {
		mastermetrics.Get().IncTaskDequeued("cancelled")
		s.clearTaskCancellationRequest(taskID.TaskId)
		if s.taskDB != nil {
			if err := s.taskDB.UpdateTaskStatus(ctx, taskID.TaskId, "cancelled"); err != nil {
				return &pb.TaskAck{
					Success: false,
					Message: fmt.Sprintf("Failed to cancel queued task in database: %v", err),
				}, nil
			}
		}
		return &pb.TaskAck{
			Success: true,
			Message: "Queued task cancelled successfully",
		}, nil
	}

	if s.requestTaskCancellation(taskID.TaskId) {
		if s.taskDB != nil {
			if err := s.taskDB.UpdateTaskStatus(ctx, taskID.TaskId, "cancelled"); err != nil {
				return &pb.TaskAck{
					Success: false,
					Message: fmt.Sprintf("Failed to mark in-flight task as cancelled in database: %v", err),
				}, nil
			}
		}
		return &pb.TaskAck{
			Success: true,
			Message: "Task cancellation requested while scheduling is in-flight",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var targetWorkerID string
	var targetWorker *WorkerState

	for workerID, worker := range s.workers {
		if worker.RunningTasks != nil && worker.RunningTasks[taskID.TaskId] {
			targetWorkerID = workerID
			targetWorker = worker
			break
		}
	}

	if targetWorkerID == "" && s.assignmentDB != nil {
		workerID, err := s.assignmentDB.GetWorkerForTask(ctx, taskID.TaskId)
		if err != nil {
			return &pb.TaskAck{
				Success: false,
				Message: fmt.Sprintf("Task not found or not assigned to any worker: %v", err),
			}, nil
		}
		targetWorkerID = workerID
		targetWorker = s.workers[workerID]
		if targetWorker == nil {
			return &pb.TaskAck{
				Success: false,
				Message: fmt.Sprintf("Worker %s not found", workerID),
			}, nil
		}
	}

	if targetWorkerID == "" {
		return &pb.TaskAck{
			Success: false,
			Message: "Task not found or not running",
		}, nil
	}

	if s.taskDB != nil {
		if err := s.taskDB.UpdateTaskStatus(ctx, taskID.TaskId, "cancelled"); err != nil {
			return &pb.TaskAck{
				Success: false,
				Message: fmt.Sprintf("Failed to update database: %v", err),
			}, nil
		}
	}

	cancelCtx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()

	conn, err := grpc.Dial(targetWorker.Info.WorkerIp, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &pb.TaskAck{
			Success: true,
			Message: fmt.Sprintf("Task marked as cancelled in database (worker unreachable: %v)", err),
		}, nil
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	ack, err := client.CancelTask(cancelCtx, taskID)
	if err != nil {
		return &pb.TaskAck{
			Success: true,
			Message: fmt.Sprintf("Task marked as cancelled in database (worker communication failed: %v)", err),
		}, nil
	}

	if !ack.Success {
		return ack, nil
	}

	if targetWorker.RunningTasks != nil {
		delete(targetWorker.RunningTasks, taskID.TaskId)
	}
	s.clearTaskCancellationRequest(taskID.TaskId)

	return &pb.TaskAck{
		Success: true,
		Message: "Task cancelled successfully",
	}, nil
}
