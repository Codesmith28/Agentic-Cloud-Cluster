package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"master/internal/db"
	mastermetrics "master/internal/metrics"
	"master/internal/scheduler"
	pb "master/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func taskIsTerminal(status string) bool {
	switch strings.ToLower(status) {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func nextAttemptID(taskID string, attemptNo int32) string {
	return fmt.Sprintf("att-%s-%d", taskID, attemptNo)
}

func shouldIgnoreAttemptResult(currentAttemptID, resultAttemptID, persistedAttemptStatus string) bool {
	if resultAttemptID == "" {
		return false
	}
	if persistedAttemptStatus == db.AttemptStatusLost || persistedAttemptStatus == db.AttemptStatusStale {
		return true
	}
	return currentAttemptID != "" && resultAttemptID != currentAttemptID
}

func buildProtoTaskFromDB(task *db.Task) *pb.Task {
	if task == nil {
		return nil
	}
	return &pb.Task{
		TaskId:        task.TaskID,
		UserId:        task.UserID,
		TaskName:      task.TaskName,
		SubmittedAt:   task.SubmittedAt,
		DockerImage:   task.DockerImage,
		Command:       task.Command,
		ReqCpu:        task.ReqCPU,
		ReqMemory:     task.ReqMemory,
		ReqStorage:    task.ReqStorage,
		TaskType:      task.TaskType,
		SlaMultiplier: task.SLAMultiplier,
		AttemptId:     task.CurrentAttemptID,
		AttemptNo:     task.CurrentAttemptNo,
	}
}

func (s *MasterServer) releaseTaskResourcesLocked(ctx context.Context, workerID string, worker *WorkerState, task *db.Task) {
	if worker == nil || task == nil {
		return
	}

	if worker.RunningTasks != nil {
		delete(worker.RunningTasks, task.TaskID)
	}

	worker.AllocatedCPU -= task.ReqCPU
	worker.AllocatedMemory -= task.ReqMemory
	worker.AllocatedStorage -= task.ReqStorage
	worker.AvailableCPU += task.ReqCPU
	worker.AvailableMemory += task.ReqMemory
	worker.AvailableStorage += task.ReqStorage

	if worker.AllocatedCPU < 0 {
		worker.AllocatedCPU = 0
	}
	if worker.AllocatedMemory < 0 {
		worker.AllocatedMemory = 0
	}
	if worker.AllocatedStorage < 0 {
		worker.AllocatedStorage = 0
	}
	if worker.AvailableCPU > worker.Info.TotalCpu {
		worker.AvailableCPU = worker.Info.TotalCpu
	}
	if worker.AvailableMemory > worker.Info.TotalMemory {
		worker.AvailableMemory = worker.Info.TotalMemory
	}
	if worker.AvailableStorage > worker.Info.TotalStorage {
		worker.AvailableStorage = worker.Info.TotalStorage
	}

	if s.workerDB != nil {
		if err := s.workerDB.ReleaseResources(ctx, workerID, task.ReqCPU, task.ReqMemory, task.ReqStorage); err != nil {
			log.Printf("⚠ Warning: failed to release stranded resources for %s: %v", task.TaskID, err)
		}
	}
}

func (s *MasterServer) recoverWorkerTasksLocked(ctx context.Context, workerID string, worker *WorkerState, failureReason string) {
	if s.taskDB == nil {
		return
	}
	recoveryStarted := time.Now()
	defer mastermetrics.Get().ObserveRecoveryDuration(failureReason, recoveryStarted)

	recovered := make(map[string]bool)
	recoverTask := func(taskID string, assignment *db.Assignment) {
		if taskID == "" || recovered[taskID] {
			return
		}
		recovered[taskID] = true

		task, err := s.taskDB.GetTask(ctx, taskID)
		if err != nil {
			log.Printf("⚠ Warning: failed to load task %s during recovery: %v", taskID, err)
			return
		}
		if task == nil || taskIsTerminal(task.Status) {
			return
		}

		attemptID := task.CurrentAttemptID
		if assignment != nil && assignment.AttemptID != "" {
			attemptID = assignment.AttemptID
		}

		if s.attemptDB != nil && attemptID != "" {
			if err := s.attemptDB.MarkAttemptLost(ctx, attemptID, failureReason); err != nil {
				log.Printf("⚠ Warning: failed to mark attempt %s lost: %v", attemptID, err)
			}
		}

		s.releaseTaskResourcesLocked(ctx, workerID, worker, task)

		if s.assignmentDB != nil {
			if err := s.assignmentDB.DeleteAssignment(ctx, taskID); err != nil && !strings.Contains(err.Error(), "not found") {
				log.Printf("⚠ Warning: failed to delete assignment for %s: %v", taskID, err)
			}
		}

		if err := s.taskDB.MarkTaskForRequeue(ctx, taskID, failureReason); err != nil {
			log.Printf("⚠ Warning: failed to mark task %s for requeue: %v", taskID, err)
			return
		}

		task.Status = "queued"
		task.LastFailureReason = failureReason
		task.RecoveryCount++
		mastermetrics.Get().IncTaskRequeue(failureReason, task.TaskType)
		s.enqueueRecoveredTask(task, fmt.Sprintf("Recovered after %s", failureReason))
		log.Printf("↻ Requeued task %s after %s", taskID, failureReason)
	}

	if s.assignmentDB != nil {
		assignments, err := s.assignmentDB.GetAssignmentsByWorker(ctx, workerID)
		if err == nil {
			for _, assignment := range assignments {
				recoverTask(assignment.TaskID, assignment)
			}
		} else {
			log.Printf("⚠ Warning: failed to list assignments for %s during recovery: %v", workerID, err)
		}
	}

	if worker != nil && worker.RunningTasks != nil {
		for taskID := range worker.RunningTasks {
			recoverTask(taskID, nil)
		}
	}
}

func (s *MasterServer) reportSchedulingOutcomeAsync(taskResources *db.Task, result *pb.TaskResult) {
	if result == nil {
		return
	}
	reporter, ok := s.scheduler.(scheduler.OutcomeReporter)
	if !ok {
		return
	}

	normalizedStatus := normalizeOutcomeStatus(result.Status)
	now := time.Now()
	runtimeSeconds := 0.0
	slaSuccess := false
	clusterHash := ""

	var taskPB *pb.Task
	if taskResources != nil {
		taskPB = &pb.Task{
			TaskId:        taskResources.TaskID,
			UserId:        taskResources.UserID,
			TaskName:      taskResources.TaskName,
			SubmittedAt:   taskResources.SubmittedAt,
			DockerImage:   taskResources.DockerImage,
			Command:       taskResources.Command,
			ReqCpu:        taskResources.ReqCPU,
			ReqMemory:     taskResources.ReqMemory,
			ReqStorage:    taskResources.ReqStorage,
			TaskType:      taskResources.TaskType,
			SlaMultiplier: taskResources.SLAMultiplier,
		}

		if !taskResources.StartedAt.IsZero() {
			runtimeSeconds = now.Sub(taskResources.StartedAt).Seconds()
		}
		if !taskResources.StartedAt.IsZero() && !taskResources.CompletedAt.IsZero() {
			runtimeSeconds = taskResources.CompletedAt.Sub(taskResources.StartedAt).Seconds()
		}
		if runtimeSeconds < 0 {
			runtimeSeconds = 0
		}
		if !taskResources.Deadline.IsZero() {
			slaSuccess = !now.After(taskResources.Deadline)
		}
	}

	reward := computeOutcomeReward(normalizedStatus, runtimeSeconds, slaSuccess)
	outcome := scheduler.TaskOutcome{
		TaskID:         result.TaskId,
		WorkerID:       result.WorkerId,
		Status:         normalizedStatus,
		Reward:         reward,
		RuntimeSeconds: runtimeSeconds,
		SLASuccess:     slaSuccess,
		Task:           taskPB,
		ClusterHash:    clusterHash,
		CompletedAt:    now,
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := reporter.ReportOutcome(ctx, outcome); err != nil {
			log.Printf("⚠️  Scheduler outcome report failed for task %s: %v", outcome.TaskID, err)
		}
	}()
}

func normalizeOutcomeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "completed":
		return "success"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return "failed"
	}
}

func computeOutcomeReward(status string, runtimeSeconds float64, slaSuccess bool) float64 {
	reward := 0.0
	switch status {
	case "success":
		reward += 1.0
	case "cancelled":
		reward -= 0.5
	default:
		reward -= 1.0
	}

	if slaSuccess {
		reward += 0.5
	} else {
		reward -= 0.25
	}

	if runtimeSeconds > 0 {
		reward -= minFloat(runtimeSeconds/600.0, 0.5)
	}
	return reward
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (s *MasterServer) updateTaskStatusSafe(taskID, status string) {
	if s.taskDB == nil || taskID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.taskDB.UpdateTaskStatus(ctx, taskID, status); err != nil {
		log.Printf("Warning: Failed to update task %s status to %s: %v", taskID, status, err)
	}
}

// DispatchTaskToWorker assigns a task directly to a specific worker
func (s *MasterServer) DispatchTaskToWorker(ctx context.Context, task *pb.Task, workerID string) (*pb.TaskAck, error) {
	return s.assignTaskToWorker(ctx, task, workerID)
}

// StreamTaskLogsFromWorker streams logs for a task from the worker
func (s *MasterServer) StreamTaskLogsFromWorker(ctx context.Context, taskID, userID string, logHandler func(string, bool)) error {
	s.mu.RLock()

	if s.resultDB != nil {
		result, err := s.resultDB.GetResult(ctx, taskID)
		if err == nil && result != nil {
			s.mu.RUnlock()
			lines := strings.Split(result.Logs, "\n")
			for i, line := range lines {
				time.Sleep(10 * time.Millisecond)
				isLastLine := i == len(lines)-1
				logHandler(line, isLastLine)
			}
			return nil
		}
	}

	var workerID string
	if s.assignmentDB != nil {
		assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, taskID)
		if err != nil {
			s.mu.RUnlock()
			return fmt.Errorf("failed to find assignment for task: %w", err)
		}
		workerID = assignment.WorkerID
	} else {
		s.mu.RUnlock()
		return fmt.Errorf("database not available")
	}

	worker, exists := s.workers[workerID]
	if !exists {
		s.mu.RUnlock()
		return fmt.Errorf("worker %s not found", workerID)
	}

	workerIP := worker.Info.WorkerIp
	s.mu.RUnlock()

	conn, err := grpc.Dial(workerIP, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to worker: %w", err)
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	stream, err := client.StreamTaskLogs(ctx, &pb.TaskLogRequest{
		TaskId: taskID,
		UserId: userID,
		Follow: true,
	})
	if err != nil {
		return fmt.Errorf("failed to start log stream: %w", err)
	}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			}
			return fmt.Errorf("error receiving log chunk: %w", err)
		}

		logHandler(chunk.Content, chunk.IsComplete)

		if chunk.IsComplete {
			if s.taskDB != nil && chunk.Status != "running" {
				s.taskDB.UpdateTaskStatus(ctx, taskID, chunk.Status)
			}
			return nil
		}
	}
}

// GetUserIDForTask retrieves the user ID associated with a task
func (s *MasterServer) GetUserIDForTask(ctx context.Context, taskID string) (string, error) {
	if s.taskDB == nil {
		return "", fmt.Errorf("task database not available")
	}
	task, err := s.taskDB.GetTask(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return "", fmt.Errorf("task not found")
	}
	return task.UserID, nil
}

// GetTasksByStatus returns all tasks with a specific status
func (s *MasterServer) GetTasksByStatus(ctx context.Context, status string) ([]*db.Task, error) {
	if s.taskDB == nil {
		return nil, fmt.Errorf("task database not available")
	}
	tasks, err := s.taskDB.GetTasksByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}
	return tasks, nil
}

// GetAssignmentByTaskID returns the assignment for a specific task
func (s *MasterServer) GetAssignmentByTaskID(ctx context.Context, taskID string) (*db.Assignment, error) {
	if s.assignmentDB == nil {
		return nil, fmt.Errorf("assignment database not available")
	}
	assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assignment: %w", err)
	}
	return assignment, nil
}

func (s *MasterServer) assignTaskToWorker(ctx context.Context, task *pb.Task, workerID string) (*pb.TaskAck, error) {
	attemptNo := int32(1)
	if s.taskDB != nil && task != nil && task.TaskId != "" {
		if existingTask, err := s.taskDB.GetTask(ctx, task.TaskId); err == nil && existingTask != nil && existingTask.CurrentAttemptNo > 0 {
			attemptNo = existingTask.CurrentAttemptNo + 1
		} else if task.AttemptNo > 0 {
			attemptNo = task.AttemptNo + 1
		}
	} else if task != nil && task.AttemptNo > 0 {
		attemptNo = task.AttemptNo + 1
	}
	attemptID := nextAttemptID(task.TaskId, attemptNo)

	taskToAssign := *task
	taskToAssign.AttemptId = attemptID
	taskToAssign.AttemptNo = attemptNo

	s.mu.Lock()

	worker, exists := s.workers[workerID]
	if !exists {
		s.mu.Unlock()
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Worker %s not found", workerID)}, nil
	}
	if !worker.IsActive {
		s.mu.Unlock()
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Worker %s is not active", workerID)}, nil
	}

	if worker.Info.WorkerIp == "" {
		s.mu.Unlock()
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Worker %s has no IP address configured", workerID)}, nil
	}

	if worker.AvailableCPU < task.ReqCpu {
		s.mu.Unlock()
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Insufficient CPU: worker has %.2f available, task requires %.2f",
				worker.AvailableCPU, task.ReqCpu),
		}, nil
	}
	if worker.AvailableMemory < task.ReqMemory {
		s.mu.Unlock()
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Insufficient Memory: worker has %.2f GB available, task requires %.2f GB",
				worker.AvailableMemory, task.ReqMemory),
		}, nil
	}
	if worker.AvailableStorage < task.ReqStorage {
		s.mu.Unlock()
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Insufficient Storage: worker has %.2f GB available, task requires %.2f GB",
				worker.AvailableStorage, task.ReqStorage),
		}, nil
	}

	worker.AllocatedCPU += task.ReqCpu
	worker.AllocatedMemory += task.ReqMemory
	worker.AllocatedStorage += task.ReqStorage
	worker.AvailableCPU -= task.ReqCpu
	worker.AvailableMemory -= task.ReqMemory
	worker.AvailableStorage -= task.ReqStorage

	s.taskResourceCacheMu.Lock()
	s.taskResourceCache[task.TaskId] = &db.Task{
		TaskID:     task.TaskId,
		ReqCPU:     task.ReqCpu,
		ReqMemory:  task.ReqMemory,
		ReqStorage: task.ReqStorage,
		TaskType:   task.TaskType,
	}
	s.taskResourceCacheMu.Unlock()

	workerIP := worker.Info.WorkerIp
	loadAtStart := computeWorkerLoadAtStart(worker)
	s.mu.Unlock()

	resourcesReserved := true
	attemptCreated := false
	rollbackReservation := func() {
		if !resourcesReserved {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()

		currentWorker, ok := s.workers[workerID]
		if !ok {
			resourcesReserved = false
			return
		}

		currentWorker.AllocatedCPU -= task.ReqCpu
		currentWorker.AllocatedMemory -= task.ReqMemory
		currentWorker.AllocatedStorage -= task.ReqStorage
		currentWorker.AvailableCPU += task.ReqCpu
		currentWorker.AvailableMemory += task.ReqMemory
		currentWorker.AvailableStorage += task.ReqStorage

		if currentWorker.AllocatedCPU < 0 {
			currentWorker.AllocatedCPU = 0
		}
		if currentWorker.AllocatedMemory < 0 {
			currentWorker.AllocatedMemory = 0
		}
		if currentWorker.AllocatedStorage < 0 {
			currentWorker.AllocatedStorage = 0
		}
		resourcesReserved = false
	}

	if s.attemptDB != nil {
		attemptRecord := &db.TaskAttempt{
			AttemptID:   attemptID,
			TaskID:      task.TaskId,
			WorkerID:    workerID,
			AttemptNo:   attemptNo,
			Status:      db.AttemptStatusAssigned,
			LoadAtStart: loadAtStart,
		}
		if err := s.attemptDB.CreateAttempt(ctx, attemptRecord); err != nil {
			rollbackReservation()
			return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Failed to persist task attempt: %v", err)}, nil
		}
		attemptCreated = true
	}

	if s.taskDB != nil {
		if err := s.taskDB.UpdateTaskAttempt(ctx, task.TaskId, attemptID, attemptNo, workerID); err != nil {
			if s.attemptDB != nil && attemptCreated {
				_ = s.attemptDB.MarkAttemptLost(ctx, attemptID, "assignment_failed")
			}
			rollbackReservation()
			return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Failed to update task attempt state: %v", err)}, nil
		}
	}

	conn, err := grpc.Dial(workerIP, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		if s.attemptDB != nil && attemptCreated {
			_ = s.attemptDB.MarkAttemptLost(ctx, attemptID, "assignment_failed")
		}
		rollbackReservation()
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Failed to connect to worker: %v", err)}, nil
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	ack, err := client.AssignTask(ctx, &taskToAssign)
	if err != nil {
		if s.attemptDB != nil && attemptCreated {
			_ = s.attemptDB.MarkAttemptLost(ctx, attemptID, "assignment_failed")
		}
		rollbackReservation()
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Failed to assign task: %v", err)}, nil
	}

	if ack == nil || !ack.Success {
		if s.attemptDB != nil && attemptCreated {
			_ = s.attemptDB.MarkAttemptLost(ctx, attemptID, "assignment_failed")
		}
		rollbackReservation()
		if ack == nil {
			return &pb.TaskAck{
				Success: false,
				Message: "Worker returned empty acknowledgment",
			}, nil
		}
		return ack, nil
	}

	s.mu.Lock()
	currentWorker, ok := s.workers[workerID]
	if ok {
		if currentWorker.RunningTasks == nil {
			currentWorker.RunningTasks = make(map[string]bool)
		}
		currentWorker.RunningTasks[task.TaskId] = true
	}
	s.mu.Unlock()
	resourcesReserved = false

	if s.workerDB != nil {
		if err := s.workerDB.AllocateResources(ctx, workerID,
			task.ReqCpu, task.ReqMemory, task.ReqStorage); err != nil {
			log.Printf("Warning: Failed to allocate resources in database: %v", err)
		}
	}

	if s.taskDB != nil {
		if err := s.taskDB.UpdateTaskStatus(ctx, task.TaskId, "running"); err != nil {
			log.Printf("Warning: Failed to update task status: %v", err)
		}
	}

	if s.assignmentDB != nil {
		assignment := &db.Assignment{
			AssignmentID: fmt.Sprintf("ass-%s", task.TaskId),
			TaskID:       task.TaskId,
			WorkerID:     workerID,
			AttemptID:    attemptID,
			AttemptNo:    attemptNo,
			LoadAtStart:  loadAtStart,
		}
		if err := s.assignmentDB.CreateAssignment(ctx, assignment); err != nil {
			log.Printf("Warning: Failed to store assignment in database: %v", err)
		}
	}

	log.Println("\n═══════════════════════════════════════════════════════")
	log.Println("  📤 TASK ASSIGNED TO WORKER")
	log.Println("═══════════════════════════════════════════════════════")
	log.Printf("  Task ID:           %s", task.TaskId)
	log.Printf("  Attempt:           #%d (%s)", attemptNo, attemptID)
	log.Printf("  User ID:           %s", task.UserId)
	log.Printf("  Assigned Worker:   %s", workerID)
	log.Printf("  Docker Image:      %s", task.DockerImage)
	log.Println("═══════════════════════════════════════════════════════")

	return ack, nil
}

func computeWorkerLoadAtStart(worker *WorkerState) float64 {
	if worker == nil || worker.Info == nil {
		return 0.0
	}

	wCPU := worker.Info.TotalCpu
	wMem := worker.Info.TotalMemory / 10.0
	wStorage := worker.Info.TotalStorage / 50.0
	totalWeight := wCPU + wMem + wStorage
	if totalWeight <= 0 {
		return 0.0
	}

	load := (wCPU*worker.LatestCPU + wMem*worker.LatestMemory + wStorage*worker.LatestStorage) / totalWeight
	if load < 0 {
		return 0.0
	}
	return load
}

func normalizeUsageFraction(value float64) float64 {
	if value < 0 {
		return 0.0
	}
	if value > 1.0 {
		value = value / 100.0
	}
	return value
}
