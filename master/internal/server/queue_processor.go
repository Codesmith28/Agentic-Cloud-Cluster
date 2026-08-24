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
	"master/internal/telemetry"
	pb "master/proto"
)

type normalizedTaskMetadata struct {
	taskType string
	tau      float64
	deadline time.Time
}

func normalizeTaskForScheduling(task *pb.Task) normalizedTaskMetadata {
	if task == nil {
		return normalizedTaskMetadata{
			taskType: scheduler.TaskTypeMixed,
			tau:      telemetry.DefaultTauForTaskType(scheduler.TaskTypeMixed),
			deadline: time.Now(),
		}
	}

	if task.SubmittedAt <= 0 {
		task.SubmittedAt = time.Now().Unix()
	}

	if task.SlaMultiplier < 1.5 || task.SlaMultiplier > 2.5 {
		task.SlaMultiplier = 2.0
	}

	taskType := task.TaskType
	if !scheduler.ValidateTaskType(taskType) {
		taskType = scheduler.InferTaskType(task)
	}
	task.TaskType = taskType

	tau := telemetry.DefaultTauForTaskType(taskType)
	deadline := time.Unix(task.SubmittedAt, 0).Add(time.Duration(task.SlaMultiplier * tau * float64(time.Second)))

	return normalizedTaskMetadata{
		taskType: taskType,
		tau:      tau,
		deadline: deadline,
	}
}

func queueReasonLabel(reason string) string {
	switch {
	case strings.Contains(reason, "Recovered"):
		return "recovery"
	case strings.Contains(reason, "Restored"):
		return "restore"
	case strings.Contains(reason, "submitted"):
		return "submit"
	default:
		return "queue"
	}
}

func (s *MasterServer) queueContainsTask(taskID string) bool {
	s.queueMu.RLock()
	defer s.queueMu.RUnlock()

	if s.processingTasks[taskID] {
		return true
	}
	for _, qt := range s.taskQueue {
		if qt != nil && qt.Task != nil && qt.Task.TaskId == taskID {
			return true
		}
	}
	return false
}

func (s *MasterServer) enqueueRecoveredTask(task *db.Task, reason string) {
	if task == nil || task.TaskID == "" {
		return
	}
	if s.queueContainsTask(task.TaskID) {
		return
	}

	requeuedTask := buildProtoTaskFromDB(task)
	if requeuedTask == nil {
		return
	}
	requeuedTask.TargetWorkerId = ""
	s.EnqueueTask(requeuedTask, reason)
}

// StartQueueProcessor starts the background task queue processor
func (s *MasterServer) StartQueueProcessor() {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()

	if s.queueTicker != nil {
		return
	}

	s.queueTicker = time.NewTicker(5 * time.Second)
	s.queueStop = make(chan struct{})
	s.queueWG.Add(1)
	go s.processQueue(s.queueTicker, s.queueStop)
	log.Printf("✓ Task queue processor started (checking every 5s)")
}

// StopQueueProcessor stops the background task queue processor
func (s *MasterServer) StopQueueProcessor() {
	s.queueMu.Lock()
	ticker := s.queueTicker
	stopCh := s.queueStop
	s.queueTicker = nil
	s.queueStop = nil
	s.queueMu.Unlock()

	if ticker != nil {
		ticker.Stop()
	}
	if stopCh != nil {
		close(stopCh)
	}
	s.queueWG.Wait()
	log.Printf("✓ Task queue processor stopped")
}

// processQueue continuously attempts to schedule and assign queued tasks
func (s *MasterServer) processQueue(ticker *time.Ticker, stopCh <-chan struct{}) {
	defer s.queueWG.Done()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
		}

		s.queueMu.Lock()
		if len(s.taskQueue) == 0 {
			s.queueMu.Unlock()
			mastermetrics.Get().SetQueueDepth(0)
			continue
		}
		tasksToProcess := make([]*QueuedTask, len(s.taskQueue))
		copy(tasksToProcess, s.taskQueue)
		s.taskQueue = s.taskQueue[:0]
		for _, qt := range tasksToProcess {
			if qt != nil && qt.Task != nil {
				s.processingTasks[qt.Task.TaskId] = true
			}
		}
		s.queueMu.Unlock()
		mastermetrics.Get().SetQueueDepth(0)

		remainingTasks := make([]*QueuedTask, 0, len(tasksToProcess))
		for _, qt := range tasksToProcess {
			if qt == nil || qt.Task == nil {
				continue
			}
			taskID := qt.Task.TaskId
			if !s.isTaskBeingProcessed(taskID) {
				continue
			}
			if s.isTaskCancellationRequested(taskID) {
				s.clearTaskCancellationRequest(taskID)
				s.updateTaskStatusSafe(taskID, "cancelled")
				mastermetrics.Get().IncTaskDequeued("cancelled")
				continue
			}

			schedulingStarted := time.Now()
			selectedWorker := s.selectWorkerForTask(qt.Task)

			if selectedWorker == "" {
				qt.Retries++
				qt.LastError = "No suitable worker available with sufficient resources"
				s.updateTaskStatusSafe(taskID, "queued")
				remainingTasks = append(remainingTasks, qt)

				if qt.Retries == 1 || qt.Retries%10 == 0 {
					log.Printf("📋 Queue: Task %s still waiting (attempt %d): %s",
						qt.Task.TaskId, qt.Retries, qt.LastError)
				}
				continue
			}

			qt.Task.TargetWorkerId = selectedWorker

			if s.isTaskCancellationRequested(taskID) {
				s.clearTaskCancellationRequest(taskID)
				s.updateTaskStatusSafe(taskID, "cancelled")
				mastermetrics.Get().IncTaskDequeued("cancelled")
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			ack, err := s.assignTaskToWorker(ctx, qt.Task, selectedWorker)
			cancel()
			mastermetrics.Get().ObserveSchedulingLatency(s.scheduler.GetName(), schedulingStarted)

			if err != nil || ack == nil || !ack.Success {
				qt.Retries++
				if err != nil {
					qt.LastError = err.Error()
				} else if ack == nil {
					qt.LastError = "empty acknowledgment from worker"
				} else {
					qt.LastError = ack.Message
				}
				s.updateTaskStatusSafe(taskID, "queued")
				remainingTasks = append(remainingTasks, qt)

				if qt.Retries == 1 || qt.Retries%10 == 0 {
					log.Printf("📋 Queue: Task %s assignment to %s failed (attempt %d): %s",
						qt.Task.TaskId, selectedWorker, qt.Retries, qt.LastError)
				}
			} else {
				mastermetrics.Get().IncTaskDequeued("assigned")
				mastermetrics.Get().ObserveQueueWait(s.scheduler.GetName(), qt.Task.TaskType, qt.QueuedAt)
				mastermetrics.Get().IncSchedulerSelection(s.scheduler.GetName(), qt.Task.TaskType, selectedWorker)
				log.Printf("✓ Queue: Task %s successfully assigned to %s after %d attempts",
					qt.Task.TaskId, selectedWorker, qt.Retries)
				if s.isTaskCancellationRequested(taskID) {
					s.clearTaskCancellationRequest(taskID)
					cancelCtx, cancelTask := context.WithTimeout(context.Background(), 30*time.Second)
					cancelAck, cancelErr := s.CancelTask(cancelCtx, &pb.TaskID{TaskId: taskID})
					if cancelErr != nil {
						log.Printf("⚠️  Failed to cancel task %s after assignment: %v", taskID, cancelErr)
					} else if cancelAck != nil && !cancelAck.Success {
						log.Printf("⚠️  Task %s post-assignment cancellation was rejected: %s", taskID, cancelAck.Message)
					}
					cancelTask()
				}
			}
		}

		s.queueMu.Lock()
		for _, qt := range tasksToProcess {
			if qt != nil && qt.Task != nil {
				delete(s.processingTasks, qt.Task.TaskId)
				delete(s.cancellationRequests, qt.Task.TaskId)
			}
		}
		if len(remainingTasks) > 0 {
			s.taskQueue = append(remainingTasks, s.taskQueue...)
		}
		mastermetrics.Get().SetQueueDepth(len(s.taskQueue))
		s.queueMu.Unlock()
	}
}

func (s *MasterServer) selectWorkerForTask(task *pb.Task) string {
	s.mu.RLock()

	workerInfos := make(map[string]*scheduler.WorkerInfo)
	for id, worker := range s.workers {
		workerInfos[id] = &scheduler.WorkerInfo{
			WorkerID:         id,
			IsActive:         worker.IsActive,
			WorkerIP:         worker.Info.WorkerIp,
			AvailableCPU:     worker.AvailableCPU,
			AvailableMemory:  worker.AvailableMemory,
			AvailableStorage: worker.AvailableStorage,
			TotalCPU:         worker.Info.TotalCpu,
			TotalMemory:      worker.Info.TotalMemory,
			TotalStorage:     worker.Info.TotalStorage,
			CurrentCPUUsage:  worker.LatestCPU,
			CurrentMemUsage:  worker.LatestMemory,
		}
	}

	s.mu.RUnlock()

	return s.scheduler.SelectWorker(task, workerInfos)
}

// EnqueueTask adds a task to the queue
func (s *MasterServer) EnqueueTask(task *pb.Task, reason string) {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()

	qt := &QueuedTask{
		Task:      task,
		QueuedAt:  time.Now(),
		Retries:   0,
		LastError: reason,
	}
	s.taskQueue = append(s.taskQueue, qt)
	mastermetrics.Get().IncTaskEnqueued(queueReasonLabel(reason))
	mastermetrics.Get().SetQueueDepth(len(s.taskQueue))

	log.Printf("📋 Task %s queued: %s", task.TaskId, reason)
}

// RestoreQueuedTasks loads persisted queued/pending tasks into the in-memory scheduler queue.
func (s *MasterServer) RestoreQueuedTasks(ctx context.Context) error {
	if s.taskDB == nil {
		return nil
	}

	statuses := []string{"queued", "pending", "running"}
	seen := make(map[string]bool)
	restored := 0

	for _, status := range statuses {
		tasks, err := s.taskDB.GetTasksByStatus(ctx, status)
		if err != nil {
			return fmt.Errorf("restore queued tasks (status=%s): %w", status, err)
		}

		for _, task := range tasks {
			if seen[task.TaskID] {
				continue
			}
			seen[task.TaskID] = true

			if status == "running" {
				if taskIsTerminal(task.Status) {
					continue
				}

				if s.assignmentDB == nil {
					if err := s.taskDB.MarkTaskForRequeue(ctx, task.TaskID, "master_restart"); err != nil {
						log.Printf("Warning: Failed to mark stranded task %s for requeue: %v", task.TaskID, err)
						continue
					}
					task.Status = "queued"
					task.LastFailureReason = "master_restart"
					s.enqueueRecoveredTask(task, "Recovered after master restart")
					restored++
					continue
				}

				assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, task.TaskID)
				if err != nil || assignment == nil {
					if err := s.taskDB.MarkTaskForRequeue(ctx, task.TaskID, "master_restart"); err != nil {
						log.Printf("Warning: Failed to mark unassigned running task %s for requeue: %v", task.TaskID, err)
						continue
					}
					task.Status = "queued"
					task.LastFailureReason = "master_restart"
					s.enqueueRecoveredTask(task, "Recovered stranded running task without assignment")
					restored++
					continue
				}

				worker, exists := s.GetWorkerStats(assignment.WorkerID)
				workerHealthy := exists && worker != nil && worker.IsActive && worker.LastHeartbeat > 0 &&
					time.Since(time.Unix(worker.LastHeartbeat, 0)) <= 30*time.Second
				if !workerHealthy {
					s.mu.Lock()
					s.recoverWorkerTasksLocked(ctx, assignment.WorkerID, worker, "master_restart")
					s.mu.Unlock()
					restored++
				}
				continue
			}

			if s.assignmentDB != nil {
				if assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, task.TaskID); err == nil && assignment != nil {
					continue
				}
			}

			restoredTask := buildProtoTaskFromDB(task)
			taskMeta := normalizeTaskForScheduling(restoredTask)
			if s.taskDB != nil {
				if err := s.taskDB.UpdateTaskWithSLA(ctx, restoredTask.TaskId, taskMeta.deadline, taskMeta.tau, taskMeta.taskType); err != nil {
					log.Printf("Warning: Failed to enrich restored task %s with SLA metadata: %v", restoredTask.TaskId, err)
				}
			}
			s.EnqueueTask(restoredTask, "Restored from persisted queue state")
			restored++
		}
	}

	if restored > 0 {
		log.Printf("✓ Restored %d queued task(s) from database", restored)
	}
	return nil
}

func (s *MasterServer) removeQueuedTaskByID(taskID string) bool {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()

	for i, qt := range s.taskQueue {
		if qt == nil || qt.Task == nil {
			continue
		}
		if qt.Task.TaskId == taskID {
			s.taskQueue = append(s.taskQueue[:i], s.taskQueue[i+1:]...)
			delete(s.cancellationRequests, taskID)
			mastermetrics.Get().SetQueueDepth(len(s.taskQueue))
			return true
		}
	}
	return false
}

func (s *MasterServer) isTaskBeingProcessed(taskID string) bool {
	s.queueMu.RLock()
	defer s.queueMu.RUnlock()
	return s.processingTasks[taskID]
}

func (s *MasterServer) requestTaskCancellation(taskID string) bool {
	s.queueMu.RLock()
	processing := s.processingTasks[taskID]
	s.queueMu.RUnlock()
	if !processing {
		return false
	}

	if s.assignmentDB != nil {
		if assignment, err := s.assignmentDB.GetAssignmentByTaskID(context.Background(), taskID); err == nil && assignment != nil {
			return false
		}
	}

	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if !s.processingTasks[taskID] {
		return false
	}
	s.cancellationRequests[taskID] = true
	return true
}

func (s *MasterServer) isTaskCancellationRequested(taskID string) bool {
	s.queueMu.RLock()
	defer s.queueMu.RUnlock()
	return s.cancellationRequests[taskID]
}

func (s *MasterServer) clearTaskCancellationRequest(taskID string) {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	delete(s.cancellationRequests, taskID)
}

// GetQueuedTasks returns a copy of the current task queue
func (s *MasterServer) GetQueuedTasks() []*QueuedTask {
	s.queueMu.RLock()
	defer s.queueMu.RUnlock()

	queueCopy := make([]*QueuedTask, len(s.taskQueue))
	copy(queueCopy, s.taskQueue)
	return queueCopy
}
