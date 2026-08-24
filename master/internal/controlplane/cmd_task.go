package controlplane

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "master/proto"
)

// ---------- Task command handlers ----------

func (e *Executor) cmdListTasks(status string) CommandOutcome {
	if status != "" {
		return e.listTasksByStatus(status)
	}
	return e.listAllTasksCategorically()
}

type taskWithWorker struct {
	taskID      string
	userID      string
	dockerImage string
	status      string
	workerID    string
	reqCPU      float64
	reqMemory   float64
	reqStorage  float64
	createdAt   time.Time
}

func (e *Executor) listAllTasksCategorically() CommandOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statuses := []string{"pending", "running", "completed", "failed"}
	allTasksByStatus := make(map[string][]taskWithWorker)
	totalCount := 0

	for _, st := range statuses {
		tasks, err := e.srv.GetTasksByStatus(ctx, st)
		if err != nil {
			continue
		}
		infos := make([]taskWithWorker, len(tasks))
		for i, t := range tasks {
			workerID := ""
			assignment, aErr := e.srv.GetAssignmentByTaskID(ctx, t.TaskID)
			if aErr == nil && assignment != nil {
				workerID = assignment.WorkerID
			}
			infos[i] = taskWithWorker{
				taskID: t.TaskID, userID: t.UserID, dockerImage: t.DockerImage,
				status: t.Status, workerID: workerID,
				reqCPU: t.ReqCPU, reqMemory: t.ReqMemory, reqStorage: t.ReqStorage,
				createdAt: t.CreatedAt,
			}
		}
		allTasksByStatus[st] = infos
		totalCount += len(tasks)
	}

	if totalCount == 0 {
		return CommandOutcome{Transcript: "\n✓ No tasks found in the system"}
	}

	var b strings.Builder
	b.WriteString("\n╔═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("║  ALL TASKS - Organized by Status (%d total)\n", totalCount))
	b.WriteString("╚═══════════════════════════════════════════════════════\n")

	for _, st := range statuses {
		infos := allTasksByStatus[st]
		statusEmoji := "📋"
		switch st {
		case "pending":
			statusEmoji = "⏳"
		case "running":
			statusEmoji = "▶️ "
		case "completed":
			statusEmoji = "✅"
		case "failed":
			statusEmoji = "❌"
		}

		plural := "s"
		if len(infos) == 1 {
			plural = ""
		}
		b.WriteString(fmt.Sprintf("\n%s %s (%d task%s)\n", statusEmoji, strings.ToUpper(st), len(infos), plural))
		b.WriteString("─────────────────────────────────────────────────────────\n")

		if len(infos) == 0 {
			b.WriteString("  (none)\n")
			continue
		}

		for i, info := range infos {
			b.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, info.taskID))
			b.WriteString(fmt.Sprintf("      Image:    %s\n", info.dockerImage))
			b.WriteString(fmt.Sprintf("      User:     %s\n", info.userID))
			if info.workerID != "" {
				b.WriteString(fmt.Sprintf("      Worker:   %s\n", info.workerID))
			}
			b.WriteString(fmt.Sprintf("      Created:  %s (%s ago)\n",
				info.createdAt.Format("15:04:05"),
				time.Since(info.createdAt).Round(time.Second)))
		}
	}

	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) listTasksByStatus(status string) CommandOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tasks, err := e.srv.GetTasksByStatus(ctx, status)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Error retrieving tasks: %v", err),
			Err:        err,
		}
	}

	if len(tasks) == 0 {
		return CommandOutcome{Transcript: fmt.Sprintf("\n✓ No %s tasks found", status)}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n╔═══════════════════════════════════════════════════════\n"))
	b.WriteString(fmt.Sprintf("║  %s TASKS (%d total)\n", strings.ToUpper(status), len(tasks)))
	b.WriteString(fmt.Sprintf("╚═══════════════════════════════════════════════════════\n"))

	for i, t := range tasks {
		b.WriteString(fmt.Sprintf("  [%d] %s - %s\n", i+1, t.TaskID, t.DockerImage))
	}

	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdSubmitTask(parts []string) CommandOutcome {
	dockerImage := parts[1]

	reqCPU := 1.0
	reqMemory := 0.5
	reqStorage := 1.0
	slaMultiplier := 2.0
	taskType := ""
	taskName := ""

	var warnings []string

	for i := 2; i < len(parts); i++ {
		switch parts[i] {
		case "-cpu_cores":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqCPU = val
					i++
				}
			}
		case "-mem":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqMemory = val
					i++
				}
			}
		case "-storage":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqStorage = val
					i++
				}
			}
		case "-k", "-sla":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					if val >= 1.5 && val <= 2.5 {
						slaMultiplier = val
					} else {
						warnings = append(warnings, "⚠️  Warning: SLA multiplier (-k) must be between 1.5 and 2.5. Using default: 2.0")
					}
					i++
				}
			}
		case "-type", "-task_type", "-tag":
			if i+1 < len(parts) {
				taskType = parts[i+1]
				i++
			}
		case "-name":
			if i+1 < len(parts) {
				taskName = parts[i+1]
				i++
			}
		}
	}

	taskID := fmt.Sprintf("task-%d", time.Now().Unix())
	if taskName == "" {
		imageParts := strings.Split(dockerImage, "/")
		imageName := imageParts[len(imageParts)-1]
		imageName = strings.Split(imageName, ":")[0]
		taskName = fmt.Sprintf("%s-%d", imageName, time.Now().Unix())
	}

	submittedAt := time.Now().Unix()

	var b strings.Builder
	for _, w := range warnings {
		b.WriteString(w + "\n")
	}

	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("  📤 SUBMITTING TASK TO QUEUE\n")
	b.WriteString("═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  Task ID:           %s\n", taskID))
	b.WriteString(fmt.Sprintf("  Task Name:         %s\n", taskName))
	b.WriteString(fmt.Sprintf("  Docker Image:      %s\n", dockerImage))
	b.WriteString(fmt.Sprintf("  Submitted At:      %s\n", time.Unix(submittedAt, 0).Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("  CPU: %.2f cores | Memory: %.2f GB | Storage: %.2f GB\n", reqCPU, reqMemory, reqStorage))
	b.WriteString("═══════════════════════════════════════════════════════\n")

	task := &pb.Task{
		TaskId:        taskID,
		DockerImage:   dockerImage,
		Command:       "",
		ReqCpu:        reqCPU,
		ReqMemory:     reqMemory,
		ReqStorage:    reqStorage,
		TaskType:      taskType,
		SlaMultiplier: slaMultiplier,
		UserId:        "admin",
		TaskName:      taskName,
		SubmittedAt:   submittedAt,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ack, err := e.srv.SubmitTask(ctx, task)
	if err != nil {
		b.WriteString(fmt.Sprintf("\n❌ Failed to submit task: %v\n", err))
		return CommandOutcome{Transcript: b.String(), Err: err}
	}
	if !ack.Success {
		submitErr := fmt.Errorf("task submission failed: %s", ack.Message)
		b.WriteString(fmt.Sprintf("\n❌ Failed to submit task: %s\n", ack.Message))
		return CommandOutcome{Transcript: b.String(), Err: submitErr}
	}

	b.WriteString(fmt.Sprintf("\n✅ Task %s submitted successfully and queued for scheduling!\n", taskID))
	return CommandOutcome{
		Transcript: b.String(),
		Effects: []UIEffect{
			{Type: EffectRefresh},
			{Type: EffectFocusTask, Payload: taskID},
		},
	}
}

func (e *Executor) cmdDispatchTask(parts []string) CommandOutcome {
	workerID := parts[1]
	dockerImage := parts[2]

	reqCPU := 1.0
	reqMemory := 0.5
	reqStorage := 1.0
	taskName := ""

	for i := 3; i < len(parts); i++ {
		switch parts[i] {
		case "-cpu_cores":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqCPU = val
					i++
				}
			}
		case "-mem":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqMemory = val
					i++
				}
			}
		case "-storage":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqStorage = val
					i++
				}
			}
		case "-name":
			if i+1 < len(parts) {
				taskName = parts[i+1]
				i++
			}
		}
	}

	taskID := fmt.Sprintf("task-%d", time.Now().Unix())
	if taskName == "" {
		imageParts := strings.Split(dockerImage, "/")
		imageName := imageParts[len(imageParts)-1]
		imageName = strings.Split(imageName, ":")[0]
		taskName = fmt.Sprintf("%s-%d", imageName, time.Now().Unix())
	}

	submittedAt := time.Now().Unix()

	var b strings.Builder
	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("  🎯 DISPATCHING TASK DIRECTLY TO WORKER\n")
	b.WriteString("═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  Task ID:           %s\n", taskID))
	b.WriteString(fmt.Sprintf("  Target Worker:     %s\n", workerID))
	b.WriteString(fmt.Sprintf("  Docker Image:      %s\n", dockerImage))
	b.WriteString("═══════════════════════════════════════════════════════\n")

	task := &pb.Task{
		TaskId:      taskID,
		DockerImage: dockerImage,
		Command:     "",
		ReqCpu:      reqCPU,
		ReqMemory:   reqMemory,
		ReqStorage:  reqStorage,
		UserId:      "admin",
		TaskName:    taskName,
		SubmittedAt: submittedAt,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ack, err := e.srv.DispatchTaskToWorker(ctx, task, workerID)
	if err != nil {
		b.WriteString(fmt.Sprintf("\n❌ Failed to dispatch task: %v\n", err))
		return CommandOutcome{Transcript: b.String(), Err: err}
	}
	if !ack.Success {
		dispatchErr := fmt.Errorf("task dispatch failed: %s", ack.Message)
		b.WriteString(fmt.Sprintf("\n❌ Failed to dispatch task: %s\n", ack.Message))
		return CommandOutcome{Transcript: b.String(), Err: dispatchErr}
	}

	b.WriteString(fmt.Sprintf("\n✅ Task %s dispatched directly to worker %s!\n", taskID, workerID))
	return CommandOutcome{
		Transcript: b.String(),
		Effects: []UIEffect{
			{Type: EffectRefresh},
			{Type: EffectFocusTask, Payload: taskID},
		},
	}
}

func (e *Executor) cmdCancel(taskID string) CommandOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ack, err := e.srv.CancelTask(ctx, &pb.TaskID{TaskId: taskID})
	if err != nil {
		return CommandOutcome{Transcript: fmt.Sprintf("❌ Error cancelling task: %v\n", err), Err: err}
	}
	if !ack.Success {
		return CommandOutcome{Transcript: fmt.Sprintf("❌ Failed to cancel task: %s\n", ack.Message), Err: fmt.Errorf("cancel failed: %s", ack.Message)}
	}

	return CommandOutcome{
		Transcript: fmt.Sprintf("✅ Task %s cancelled successfully!\n", taskID),
		Effects:    []UIEffect{{Type: EffectRefresh}},
	}
}

func (e *Executor) cmdQueue() CommandOutcome {
	queuedTasks := e.srv.GetQueuedTasks()

	if len(queuedTasks) == 0 {
		return CommandOutcome{Transcript: "\n✓ Task queue is empty"}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  📋 QUEUED TASKS (%d pending)\n", len(queuedTasks)))
	for i, qt := range queuedTasks {
		b.WriteString(fmt.Sprintf("[%d] Task ID: %s | Image: %s | CPU: %.1f | Mem: %.1f\n",
			i+1, qt.Task.TaskId, qt.Task.DockerImage, qt.Task.ReqCpu, qt.Task.ReqMemory))
	}
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdMonitor(taskID string) CommandOutcome {
	return CommandOutcome{
		Transcript: fmt.Sprintf("Opening monitor for task %s...", taskID),
		Effects:    []UIEffect{{Type: EffectOpenMonitor, Payload: taskID}},
	}
}

func (e *Executor) StreamTaskLogs(ctx context.Context, taskID string, handler func(line string, complete bool, status string) error) error {
	userID, err := e.srv.GetUserIDForTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task info: %w", err)
	}
	return e.srv.StreamTaskLogsUnified(ctx, taskID, userID, func(logLine string, isComplete bool, status string) error {
		return handler(logLine, isComplete, status)
	})
}
