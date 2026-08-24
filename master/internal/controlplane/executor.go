package controlplane

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"master/internal/server"
	"master/internal/storage"
	"master/internal/system"
)

// Executor provides a shared command execution layer for both the CLI and TUI.
// It wraps MasterServer + FileStorageService + HostResourceSampler and returns
// structured CommandOutcome values instead of printing directly to stdout.
type Executor struct {
	srv         *server.MasterServer
	fs          *storage.FileStorageService
	hostSampler *system.HostResourceSampler
}

// NewExecutor creates a new Executor. hostSampler may be nil.
func NewExecutor(srv *server.MasterServer, fs *storage.FileStorageService, hostSampler *system.HostResourceSampler) *Executor {
	return &Executor{
		srv:         srv,
		fs:          fs,
		hostSampler: hostSampler,
	}
}

// ---------- Data accessors ----------

// GetDashboard builds a DashboardSnapshot from the cluster state and host resources.
func (e *Executor) GetDashboard() DashboardSnapshot {
	snap := e.srv.GetClusterSnapshot()
	queuedTasks := e.srv.GetQueuedTasks()

	ds := DashboardSnapshot{
		Timestamp:            snap.Timestamp,
		TotalWorkers:         snap.TotalWorkers,
		ActiveWorkers:        snap.ActiveWorkers,
		InactiveWorkers:      snap.InactiveWorkers,
		RunningTasks:         snap.TotalTasks,
		QueuedTasks:          len(queuedTasks),
		SchedulerName:        e.srv.GetSchedulerName(),
		ClusterCPUTotal:      snap.TotalCPU,
		ClusterCPUAllocated:  snap.AllocatedCPU,
		ClusterCPUUtil:       snap.CPUUtilization,
		ClusterMemTotal:      snap.TotalMemory,
		ClusterMemAllocated:  snap.AllocatedMemory,
		ClusterMemUtil:       snap.MemoryUtilization,
		ClusterStorTotal:     snap.TotalStorage,
		ClusterStorAllocated: snap.AllocatedStorage,
		ClusterStorUtil:      snap.StorageUtilization,
	}

	if e.hostSampler != nil {
		hr := e.hostSampler.GetLatest()
		ds.HostResources = MasterHostResources{
			CPUPercent:  hr.CPUPercent,
			MemPercent:  hr.MemPercent,
			MemUsedGB:   hr.MemUsedGB,
			MemTotalGB:  hr.MemTotalGB,
			StorUsedGB:  hr.StorUsedGB,
			StorTotalGB: hr.StorTotalGB,
			StorPercent: hr.StorPercent,
			NumCPU:      hr.NumCPU,
			Hostname:    hr.Hostname,
			SampledAt:   hr.SampledAt,
		}
	}

	return ds
}

// GetWorkerRows converts the live server worker data into WorkerRow slices.
func (e *Executor) GetWorkerRows() []WorkerRow {
	workers := e.srv.GetWorkers()
	rows := make([]WorkerRow, 0, len(workers))

	for id, w := range workers {
		lastHB := "never"
		if w.LastHeartbeat > 0 {
			d := time.Since(time.Unix(w.LastHeartbeat, 0))
			if d < time.Minute {
				lastHB = fmt.Sprintf("%ds ago", int(d.Seconds()))
			} else if d < time.Hour {
				lastHB = fmt.Sprintf("%dm ago", int(d.Minutes()))
			} else {
				lastHB = fmt.Sprintf("%dh ago", int(d.Hours()))
			}
		}

		taskIDs := make([]string, 0, len(w.RunningTasks))
		for tid := range w.RunningTasks {
			taskIDs = append(taskIDs, tid)
		}
		sort.Strings(taskIDs)

		rows = append(rows, WorkerRow{
			WorkerID:       id,
			WorkerIP:       w.Info.WorkerIp,
			IsActive:       w.IsActive,
			LastHeartbeat:  lastHB,
			CPUUsage:       w.LatestCPU,
			MemUsage:       w.LatestMemory,
			StorUsage:      w.LatestStorage,
			TotalCPU:       w.Info.TotalCpu,
			TotalMem:       w.Info.TotalMemory,
			TotalStor:      w.Info.TotalStorage,
			AllocatedCPU:   w.AllocatedCPU,
			AllocatedMem:   w.AllocatedMemory,
			AllocatedStor:  w.AllocatedStorage,
			AvailableCPU:   w.AvailableCPU,
			AvailableMem:   w.AvailableMemory,
			AvailableStor:  w.AvailableStorage,
			TaskCount:      w.TaskCount,
			RunningTaskIDs: taskIDs,
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].WorkerID < rows[j].WorkerID })
	return rows
}

// GetTaskRows returns a slice of TaskRow structs filtered by status (or all if status == "").
func (e *Executor) GetTaskRows(status string) ([]TaskRow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var rows []TaskRow

	fetchStatus := func(st string) error {
		tasks, err := e.srv.GetTasksByStatus(ctx, st)
		if err != nil {
			return err
		}
		for _, t := range tasks {
			workerID := ""
			assignment, aErr := e.srv.GetAssignmentByTaskID(ctx, t.TaskID)
			if aErr == nil && assignment != nil {
				workerID = assignment.WorkerID
			}
			rows = append(rows, TaskRow{
				TaskID:      t.TaskID,
				TaskName:    t.TaskName,
				UserID:      t.UserID,
				DockerImage: t.DockerImage,
				Status:      t.Status,
				WorkerID:    workerID,
				ReqCPU:      t.ReqCPU,
				ReqMem:      t.ReqMemory,
				ReqStor:     t.ReqStorage,
				CreatedAt:   t.CreatedAt,
				SLAMult:     t.SLAMultiplier,
				TaskType:    t.TaskType,
			})
		}
		return nil
	}

	if status != "" {
		if err := fetchStatus(status); err != nil {
			return nil, err
		}
	} else {
		for _, st := range []string{"pending", "running", "completed", "failed"} {
			_ = fetchStatus(st)
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})

	return rows, nil
}

// GetQueueRows returns a slice of QueueRow structs for the TUI queue panel.
func (e *Executor) GetQueueRows() []QueueRow {
	queued := e.srv.GetQueuedTasks()
	rows := make([]QueueRow, 0, len(queued))

	for _, qt := range queued {
		rows = append(rows, QueueRow{
			TaskID:       qt.Task.TaskId,
			DockerImage:  qt.Task.DockerImage,
			UserID:       qt.Task.UserId,
			ReqCPU:       qt.Task.ReqCpu,
			ReqMem:       qt.Task.ReqMemory,
			ReqStor:      qt.Task.ReqStorage,
			QueuedAt:     qt.QueuedAt,
			TimeInQueue:  time.Since(qt.QueuedAt),
			Retries:      qt.Retries,
			LastError:    qt.LastError,
			TargetWorker: qt.Task.TargetWorkerId,
		})
	}

	return rows
}

// ---------- Command Dispatch ----------

// ExecuteCommand parses a raw command string, dispatches to the appropriate handler,
// and returns a CommandOutcome.
func (e *Executor) ExecuteCommand(input string) CommandOutcome {
	input = strings.TrimSpace(input)
	if input == "" {
		return CommandOutcome{}
	}

	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "help":
		return e.cmdHelp()

	case "status":
		return e.cmdStatus()

	case "workers":
		return e.cmdWorkers()

	case "stats":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: stats <worker_id>"}
		}
		return e.cmdStats(parts[1])

	case "internal-state":
		return e.cmdInternalState()

	case "fix-resources":
		return e.cmdFixResources()

	case "list-tasks", "tasks":
		status := ""
		if len(parts) >= 2 {
			status = parts[1]
		}
		return e.cmdListTasks(status)

	case "register":
		if len(parts) < 3 {
			return CommandOutcome{Transcript: "Usage: register <id> <ip:port>"}
		}
		return e.cmdRegister(parts[1], parts[2])

	case "unregister":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: unregister <id>"}
		}
		return e.cmdUnregister(parts[1])

	case "task", "submit":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: taskUsage()}
		}
		return e.cmdSubmitTask(parts)

	case "dispatch":
		if len(parts) < 3 {
			return CommandOutcome{Transcript: dispatchUsage()}
		}
		return e.cmdDispatchTask(parts)

	case "cancel":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: cancel <task_id>"}
		}
		return e.cmdCancel(parts[1])

	case "queue":
		return e.cmdQueue()

	case "monitor":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: monitor <task_id>"}
		}
		return e.cmdMonitor(parts[1])

	case "benchmark":
		return e.cmdBenchmark(parts)

	case "workload-submit":
		return e.cmdWorkloadSubmit(parts)

	case "files":
		requestingUser := "admin"
		targetUser := "admin"
		if len(parts) >= 2 {
			targetUser = parts[1]
		}
		if len(parts) >= 3 {
			requestingUser = parts[2]
		}
		return e.cmdListFiles(requestingUser, targetUser)

	case "task-files":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: task-files <task_id> [target_user] [requesting_user]"}
		}
		taskID := parts[1]
		requestingUser := "admin"
		targetUser := "admin"
		if len(parts) >= 3 {
			targetUser = parts[2]
		}
		if len(parts) >= 4 {
			requestingUser = parts[3]
		}
		return e.cmdTaskFiles(taskID, requestingUser, targetUser)

	case "download":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: download <task_id> [target_user] [requesting_user] [output_dir]"}
		}
		taskID := parts[1]
		requestingUser := "admin"
		targetUser := "admin"
		outputDir := "./downloads"
		if len(parts) >= 3 {
			targetUser = parts[2]
		}
		if len(parts) >= 4 {
			requestingUser = parts[3]
		}
		if len(parts) >= 5 {
			outputDir = parts[4]
		}
		return e.cmdDownload(taskID, requestingUser, targetUser, outputDir)

	case "exit", "quit":
		return CommandOutcome{Transcript: "Shutting down..."}

	default:
		return CommandOutcome{Transcript: fmt.Sprintf("Unknown command: %s. Type 'help' for available commands.", cmd)}
	}
}

func taskUsage() string {
	return "Usage: task <docker_image> [-name <task_name>] [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-k <1.5-2.5>] [-type <task_type>]"
}

func dispatchUsage() string {
	return "Usage: dispatch <worker_id> <docker_image> [-name <task_name>] [-cpu_cores <num>] [-mem <gb>] [-storage <gb>]"
}
