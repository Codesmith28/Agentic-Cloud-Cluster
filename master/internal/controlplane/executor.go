package controlplane

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"master/internal/benchmark"
	"master/internal/server"
	"master/internal/storage"
	"master/internal/system"
	pb "master/proto"
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

// GetTaskRows fetches tasks from the database for a given status and returns TaskRow slices.
// If status is empty, all major statuses are queried.
func (e *Executor) GetTaskRows(status string) ([]TaskRow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statuses := []string{status}
	if status == "" {
		statuses = []string{"pending", "running", "completed", "failed"}
	}

	var rows []TaskRow
	for _, st := range statuses {
		tasks, err := e.srv.GetTasksByStatus(ctx, st)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s tasks: %w", st, err)
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
	}
	return rows, nil
}

// GetQueueRows returns the current task queue as QueueRow slices.
func (e *Executor) GetQueueRows() []QueueRow {
	queuedTasks := e.srv.GetQueuedTasks()
	rows := make([]QueueRow, 0, len(queuedTasks))
	for _, qt := range queuedTasks {
		target := ""
		if qt.Task.TargetWorkerId != "" {
			target = qt.Task.TargetWorkerId
		}
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
			TargetWorker: target,
		})
	}
	return rows
}

// ---------- Command execution ----------

// ExecuteCommand parses a raw input string and dispatches to the appropriate handler.
// It returns a CommandOutcome with transcript text and optional UI effects.
func (e *Executor) ExecuteCommand(input string) CommandOutcome {
	input = strings.TrimSpace(input)
	if input == "" {
		return CommandOutcome{}
	}

	parts := strings.Fields(input)
	command := parts[0]

	switch command {
	case "help":
		return e.cmdHelp()
	case "status":
		return e.cmdStatus()
	case "workers":
		return e.cmdWorkers()
	case "stats":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: stats <worker_id>\nExample: stats worker-1"}
		}
		return e.cmdStats(parts[1])
	case "internal-state":
		return e.cmdInternalState()
	case "fix-resources":
		return e.cmdFixResources()
	case "list-tasks":
		status := ""
		if len(parts) >= 2 {
			status = parts[1]
		}
		return e.cmdListTasks(status)
	case "register":
		if len(parts) < 3 {
			return CommandOutcome{Transcript: "Usage: register <worker_id> <worker_ip:port>\nExample: register worker-1 192.168.1.100:50052"}
		}
		return e.cmdRegister(parts[1], parts[2])
	case "unregister":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: unregister <worker_id>"}
		}
		return e.cmdUnregister(parts[1])
	case "task":
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
			return CommandOutcome{Transcript: "Usage: cancel <task_id>\nExample: cancel task-123"}
		}
		return e.cmdCancel(parts[1])
	case "queue":
		return e.cmdQueue()
	case "monitor":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: monitor <task_id>\nExample: monitor task-123"}
		}
		return e.cmdMonitor(parts[1])
	case "benchmark", "bench":
		return e.cmdBenchmark(parts)
	case "workload-submit":
		return e.cmdWorkloadSubmit(parts)
	case "files":
		if len(parts) < 2 {
			return CommandOutcome{Transcript: "Usage: files <user_id> [requesting_user]\nExample: files alice"}
		}
		requestingUser := parts[1]
		if len(parts) >= 3 {
			requestingUser = parts[2]
		}
		return e.cmdListFiles(requestingUser, parts[1])
	case "task-files":
		if len(parts) < 3 {
			return CommandOutcome{Transcript: "Usage: task-files <task_id> <user_id> [requesting_user]\nExample: task-files task-123 alice"}
		}
		requestingUser := parts[2]
		if len(parts) >= 4 {
			requestingUser = parts[3]
		}
		return e.cmdTaskFiles(parts[1], requestingUser, parts[2])
	case "download":
		if len(parts) < 3 {
			return CommandOutcome{Transcript: "Usage: download <task_id> <user_id> [requesting_user] [output_dir]\nExample: download task-123 alice"}
		}
		requestingUser := parts[2]
		outputDir := "."
		if len(parts) >= 4 {
			requestingUser = parts[3]
		}
		if len(parts) >= 5 {
			outputDir = parts[4]
		}
		return e.cmdDownload(parts[1], requestingUser, parts[2], outputDir)
	case "exit", "quit":
		return CommandOutcome{
			Transcript: "Shutting down master...",
			Effects:    []UIEffect{{Type: EffectNone, Payload: "exit"}},
		}
	default:
		return CommandOutcome{
			Transcript: fmt.Sprintf("Unknown command: %s. Type 'help' for available commands.", command),
		}
	}
}

// ---------- Individual command handlers ----------

func (e *Executor) cmdHelp() CommandOutcome {
	var b strings.Builder
	b.WriteString("\nAvailable commands:\n")
	b.WriteString("  help                           - Show this help message\n")
	b.WriteString("  status                         - Show cluster status\n")
	b.WriteString("  workers                        - List all registered workers\n")
	b.WriteString("  stats <worker_id>              - Show detailed stats for a worker\n")
	b.WriteString("  internal-state                 - Dump complete in-memory state of all workers\n")
	b.WriteString("  fix-resources                  - Fix stale resource allocations\n")
	b.WriteString("  list-tasks [status]            - List all tasks (or filter by: queued/pending/running/completed/failed/cancelled)\n")
	b.WriteString("  register <id> <ip:port>        - Manually register a worker\n")
	b.WriteString("  unregister <id>                - Unregister a worker\n")
	b.WriteString("  task <docker_img> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-k <1.5-2.5>] [-type <task_type>]\n")
	b.WriteString("                                 - Submit task (scheduler selects worker)\n")
	b.WriteString("  dispatch <worker_id> <docker_img> [options]  - Dispatch task directly to specific worker (testing)\n")
	b.WriteString("  monitor <task_id>              - Monitor live logs for a task\n")
	b.WriteString("  cancel <task_id>               - Cancel a running task\n")
	b.WriteString("  queue                          - Show pending tasks in the queue\n")
	b.WriteString("  benchmark [profile|all]        - Run scheduler benchmark suite and generate report artifacts\n")
	b.WriteString("  workload-submit <profile> [options] - Submit predefined workload to master at scheduled intervals\n")
	b.WriteString("  files <user_id> [requesting_user]  - List all files for a user\n")
	b.WriteString("  task-files <task_id> <user_id> [requesting_user]  - View files for a specific task\n")
	b.WriteString("  download <task_id> <user_id> [requesting_user] [output_dir]  - Download all task files\n")
	b.WriteString("  exit/quit                      - Shutdown master node\n")
	b.WriteString("\nTask Types (-type flag):\n")
	b.WriteString("  cpu-light                      - Light CPU workloads\n")
	b.WriteString("  cpu-heavy                      - Heavy CPU workloads\n")
	b.WriteString("  memory-heavy                   - Memory-intensive workloads\n")
	b.WriteString("  mixed                          - Mixed CPU/memory/storage workloads\n")
	b.WriteString("\nExamples:\n")
	b.WriteString("  register worker-2 192.168.1.100:50052\n")
	b.WriteString("  stats worker-1\n")
	b.WriteString("  task docker.io/user/sample-task:latest\n")
	b.WriteString("  task docker.io/user/sample-task:latest -cpu_cores 2.0 -mem 1.0 -storage 5.0\n")
	b.WriteString("  task myapp:latest -cpu_cores 4 -mem 8 -k 1.8 -type cpu-heavy\n")
	b.WriteString("  dispatch worker-1 docker.io/user/sample-task:latest -cpu_cores 2.0 -mem 1.0\n")
	b.WriteString("  monitor task-123\n")
	b.WriteString("  cancel task-123\n")
	b.WriteString("  queue\n")
	b.WriteString("  benchmark all\n")
	b.WriteString("  benchmark showcase -seed 42\n")
	b.WriteString("  workload-submit showcase -speed 10 -limit 30\n")
	b.WriteString("  files alice\n")
	b.WriteString("  task-files task-123 alice\n")
	b.WriteString("  download task-123 alice\n")
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdStatus() CommandOutcome {
	workers := e.srv.GetWorkers()

	activeCount := 0
	totalTasks := 0
	for _, w := range workers {
		if w.IsActive {
			activeCount++
		}
		totalTasks += len(w.RunningTasks)
	}

	var b strings.Builder
	b.WriteString("╔═══ Cluster Status ═══\n")
	b.WriteString(fmt.Sprintf("║ Total Workers: %d\n", len(workers)))
	b.WriteString(fmt.Sprintf("║ Active Workers: %d\n", activeCount))
	b.WriteString(fmt.Sprintf("║ Running Tasks: %d\n", totalTasks))
	b.WriteString("╚══════════════════════\n")
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdWorkers() CommandOutcome {
	workers := e.srv.GetWorkers()

	if len(workers) == 0 {
		return CommandOutcome{Transcript: "No workers registered yet."}
	}

	var b strings.Builder
	b.WriteString("\n╔═══ Registered Workers ═══\n")
	for id, w := range workers {
		status := "🟢 Active"
		if !w.IsActive {
			status = "🔴 Inactive"
		}
		b.WriteString(fmt.Sprintf("║ %s\n", id))
		b.WriteString(fmt.Sprintf("║   Status: %s\n", status))
		b.WriteString(fmt.Sprintf("║   IP: %s\n", w.Info.WorkerIp))
		b.WriteString("║   Resources:\n")
		b.WriteString(fmt.Sprintf("║     CPU:     %.1f total, %.1f allocated, %.1f available\n",
			w.Info.TotalCpu, w.AllocatedCPU, w.AvailableCPU))
		b.WriteString(fmt.Sprintf("║     Memory:  %.1f GB total, %.1f GB allocated, %.1f GB available\n",
			w.Info.TotalMemory, w.AllocatedMemory, w.AvailableMemory))
		b.WriteString(fmt.Sprintf("║     Storage: %.1f GB total, %.1f GB allocated, %.1f GB available\n",
			w.Info.TotalStorage, w.AllocatedStorage, w.AvailableStorage))
		b.WriteString(fmt.Sprintf("║   Running Tasks: %d\n", len(w.RunningTasks)))
		b.WriteString("║\n")
	}
	b.WriteString("╚═══════════════════════\n")
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdStats(workerID string) CommandOutcome {
	worker, exists := e.srv.GetWorkerStats(workerID)
	if !exists {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Worker '%s' not found", workerID),
			Err:        fmt.Errorf("worker %s not found", workerID),
		}
	}

	status := "🟢 Active"
	if !worker.IsActive {
		status = "🔴 Inactive"
	}

	lastSeen := "Never"
	if worker.LastHeartbeat > 0 {
		duration := time.Now().Unix() - worker.LastHeartbeat
		if duration < 60 {
			lastSeen = fmt.Sprintf("%d seconds ago", duration)
		} else if duration < 3600 {
			lastSeen = fmt.Sprintf("%d minutes ago", duration/60)
		} else {
			lastSeen = fmt.Sprintf("%d hours ago", duration/3600)
		}
	}

	cpuUtilPct := 0.0
	memUtilPct := 0.0
	storageUtilPct := 0.0
	if worker.Info.TotalCpu > 0 {
		cpuUtilPct = (worker.AllocatedCPU / worker.Info.TotalCpu) * 100
	}
	if worker.Info.TotalMemory > 0 {
		memUtilPct = (worker.AllocatedMemory / worker.Info.TotalMemory) * 100
	}
	if worker.Info.TotalStorage > 0 {
		storageUtilPct = (worker.AllocatedStorage / worker.Info.TotalStorage) * 100
	}

	var b strings.Builder
	b.WriteString("╔═══════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("║ Worker: %s\n", workerID))
	b.WriteString("╠═══════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("║ Status:          %s\n", status))
	b.WriteString(fmt.Sprintf("║ Address:         %s\n", worker.Info.WorkerIp))
	b.WriteString(fmt.Sprintf("║ Last Seen:       %s\n", lastSeen))
	b.WriteString("║\n")
	b.WriteString("║ Resources (Total / Allocated / Available):\n")
	b.WriteString(fmt.Sprintf("║   CPU:           %.2f / %.2f / %.2f cores (%.1f%% used)\n",
		worker.Info.TotalCpu, worker.AllocatedCPU, worker.AvailableCPU, worker.LatestCPU))
	b.WriteString(fmt.Sprintf("║   Memory:        %.2f / %.2f / %.2f GB (%.2f%% used)\n",
		worker.Info.TotalMemory, worker.AllocatedMemory, worker.AvailableMemory, worker.LatestMemory))
	b.WriteString(fmt.Sprintf("║   Storage:       %.2f / %.2f / %.2f GB\n",
		worker.Info.TotalStorage, worker.AllocatedStorage, worker.AvailableStorage))
	b.WriteString("║\n")
	b.WriteString("║ Resource Utilization:\n")
	b.WriteString(fmt.Sprintf("║   CPU Allocated:   %.1f%%\n", cpuUtilPct))
	b.WriteString(fmt.Sprintf("║   Mem Allocated:   %.1f%%\n", memUtilPct))
	b.WriteString(fmt.Sprintf("║   Storage Alloc.:  %.1f%%\n", storageUtilPct))
	b.WriteString("║\n")
	b.WriteString(fmt.Sprintf("║ Running Tasks:   %d\n", worker.TaskCount))
	b.WriteString("╚═══════════════════════════════════════════════════\n")

	return CommandOutcome{
		Transcript: b.String(),
		Effects:    []UIEffect{{Type: EffectFocusWorker, Payload: workerID}},
	}
}

func (e *Executor) cmdInternalState() CommandOutcome {
	output := e.srv.DumpInMemoryState()
	return CommandOutcome{Transcript: output}
}

func (e *Executor) cmdFixResources() CommandOutcome {
	var b strings.Builder
	b.WriteString("\n🔄 Reconciling worker resources...\n")
	b.WriteString("This will fix any stale resource allocations from completed tasks.\n")

	ctx := context.Background()
	if err := e.srv.ReconcileWorkerResourcesPublic(ctx); err != nil {
		b.WriteString(fmt.Sprintf("❌ Failed to reconcile resources: %v\n", err))
		return CommandOutcome{Transcript: b.String(), Err: err}
	}

	b.WriteString("\n✓ Resource reconciliation complete!\n")
	b.WriteString("   Run 'workers' to see updated resource allocations.\n")
	return CommandOutcome{
		Transcript: b.String(),
		Effects:    []UIEffect{{Type: EffectRefresh}},
	}
}

func (e *Executor) cmdListTasks(status string) CommandOutcome {
	if status != "" {
		return e.listTasksByStatus(status)
	}
	return e.listAllTasksCategorically()
}

// taskWithWorker bundles a task's display fields with its resolved worker assignment.
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
			b.WriteString(fmt.Sprintf("      CPU:      %.1f cores | Memory: %.1f GB | Storage: %.1f GB\n",
				info.reqCPU, info.reqMemory, info.reqStorage))
			b.WriteString(fmt.Sprintf("      Created:  %s\n", info.createdAt.Format("2006-01-02 15:04:05")))
			if i < len(infos)-1 {
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("💡 Tip: Use 'list-tasks <status>' to see details for a specific status\n")
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) listTasksByStatus(status string) CommandOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tasks, err := e.srv.GetTasksByStatus(ctx, status)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to get tasks: %v", err),
			Err:        err,
		}
	}

	if len(tasks) == 0 {
		return CommandOutcome{Transcript: fmt.Sprintf("\n✓ No tasks with status '%s'", status)}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n╔═══ Tasks with status: %s ═══\n", status))
	b.WriteString(fmt.Sprintf("║ Found %d task(s)\n", len(tasks)))
	b.WriteString("╠═══════════════════════════════════════════════════════\n")

	for i, task := range tasks {
		b.WriteString(fmt.Sprintf("║\n║ [%d] Task ID: %s\n", i+1, task.TaskID))
		b.WriteString(fmt.Sprintf("║     User ID:       %s\n", task.UserID))
		b.WriteString(fmt.Sprintf("║     Docker Image:  %s\n", task.DockerImage))
		b.WriteString(fmt.Sprintf("║     Status:        %s\n", task.Status))

		assignment, aErr := e.srv.GetAssignmentByTaskID(ctx, task.TaskID)
		if aErr == nil && assignment != nil {
			b.WriteString(fmt.Sprintf("║     Assigned To:   %s\n", assignment.WorkerID))
		} else {
			b.WriteString("║     Assigned To:   (no assignment)\n")
		}

		b.WriteString("║     Resources:\n")
		b.WriteString(fmt.Sprintf("║       CPU:     %.2f cores\n", task.ReqCPU))
		b.WriteString(fmt.Sprintf("║       Memory:  %.2f GB\n", task.ReqMemory))
		b.WriteString(fmt.Sprintf("║       Storage: %.2f GB\n", task.ReqStorage))
		b.WriteString(fmt.Sprintf("║     Created:   %s\n", task.CreatedAt.Format("2006-01-02 15:04:05")))

		if i < len(tasks)-1 {
			b.WriteString("║     ───────────────────────────────────────────────────\n")
		}
	}

	b.WriteString("╚═══════════════════════════════════════════════════════\n")

	if status == "running" && len(tasks) > 0 {
		b.WriteString("\n💡 Tip: If these tasks are not actually running, use 'fix-resources'\n")
		b.WriteString("   to reconcile and free up allocated resources.\n")
	}
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdRegister(workerID, workerIP string) CommandOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	masterID, masterAddress := e.srv.GetMasterInfo()
	if masterID == "" || masterAddress == "" {
		return CommandOutcome{
			Transcript: "❌ Master info not set. Cannot register worker.",
			Err:        fmt.Errorf("master info not set"),
		}
	}

	err := e.srv.ManualRegisterAndNotify(ctx, workerID, workerIP, masterID, masterAddress)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to register worker: %v", err),
			Err:        err,
		}
	}

	return CommandOutcome{
		Transcript: fmt.Sprintf("✅ Worker %s registered with address %s\n   Master is notifying worker... Check logs for confirmation.", workerID, workerIP),
		Effects:    []UIEffect{{Type: EffectRefresh}},
	}
}

func (e *Executor) cmdUnregister(workerID string) CommandOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := e.srv.UnregisterWorker(ctx, workerID)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to unregister worker: %v", err),
			Err:        err,
		}
	}

	return CommandOutcome{
		Transcript: fmt.Sprintf("✅ Worker %s has been unregistered", workerID),
		Effects:    []UIEffect{{Type: EffectRefresh}},
	}
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

	// Validate task type
	validTypes := []string{"cpu-light", "cpu-heavy", "memory-heavy", "mixed"}
	if taskType != "" {
		valid := false
		for _, vt := range validTypes {
			if taskType == vt {
				valid = true
				break
			}
		}
		if !valid {
			warnings = append(warnings, fmt.Sprintf("⚠️  Warning: Invalid task type '%s'. Must be one of: %v\n    Task type will be automatically inferred from resources.", taskType, validTypes))
			taskType = ""
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
	b.WriteString("───────────────────────────────────────────────────────\n")
	b.WriteString("  Resource Requirements:\n")
	b.WriteString(fmt.Sprintf("    • CPU Cores:     %.2f cores\n", reqCPU))
	b.WriteString(fmt.Sprintf("    • Memory:        %.2f GB\n", reqMemory))
	b.WriteString(fmt.Sprintf("    • Storage:       %.2f GB\n", reqStorage))
	b.WriteString("───────────────────────────────────────────────────────\n")
	if taskType != "" {
		b.WriteString("  Task Classification:\n")
		b.WriteString(fmt.Sprintf("    • Type:          %s (user-specified)\n", taskType))
	} else {
		b.WriteString("  Task Classification:\n")
		b.WriteString("    • Type:          (will be inferred from resources)\n")
	}
	b.WriteString("───────────────────────────────────────────────────────\n")
	b.WriteString("  SLA Configuration:\n")
	b.WriteString(fmt.Sprintf("    • SLA Multiplier (k): %.1f (Deadline = k × τ)\n", slaMultiplier))
	b.WriteString("───────────────────────────────────────────────────────\n")
	b.WriteString("  Note: Scheduler will automatically select best worker\n")
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
	b.WriteString("    Use 'queue' command to view queued tasks\n")
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
	b.WriteString(fmt.Sprintf("  Task Name:         %s\n", taskName))
	b.WriteString(fmt.Sprintf("  Target Worker:     %s\n", workerID))
	b.WriteString(fmt.Sprintf("  Docker Image:      %s\n", dockerImage))
	b.WriteString(fmt.Sprintf("  Submitted At:      %s\n", time.Unix(submittedAt, 0).Format("2006-01-02 15:04:05")))
	b.WriteString("───────────────────────────────────────────────────────\n")
	b.WriteString("  Resource Requirements:\n")
	b.WriteString(fmt.Sprintf("    • CPU Cores:     %.2f cores\n", reqCPU))
	b.WriteString(fmt.Sprintf("    • Memory:        %.2f GB\n", reqMemory))
	b.WriteString(fmt.Sprintf("    • Storage:       %.2f GB\n", reqStorage))
	b.WriteString("───────────────────────────────────────────────────────\n")
	b.WriteString("  ⚠️  NOTE: Bypassing scheduler - dispatching directly!\n")
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
	b.WriteString("    Use 'monitor <task_id>' command to view task logs\n")
	return CommandOutcome{
		Transcript: b.String(),
		Effects: []UIEffect{
			{Type: EffectRefresh},
			{Type: EffectFocusTask, Payload: taskID},
		},
	}
}

func (e *Executor) cmdCancel(taskID string) CommandOutcome {
	var b strings.Builder
	b.WriteString("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	b.WriteString("  🛑 CANCELLING TASK\n")
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	b.WriteString(fmt.Sprintf("  Task ID: %s\n", taskID))
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ack, err := e.srv.CancelTask(ctx, &pb.TaskID{TaskId: taskID})
	if err != nil {
		b.WriteString(fmt.Sprintf("\n❌ Error cancelling task: %v\n", err))
		return CommandOutcome{Transcript: b.String(), Err: err}
	}
	if !ack.Success {
		b.WriteString(fmt.Sprintf("\n❌ Failed to cancel task: %s\n", ack.Message))
		return CommandOutcome{Transcript: b.String(), Err: fmt.Errorf("cancel failed: %s", ack.Message)}
	}

	b.WriteString("\n✅ Task cancelled successfully!\n")
	b.WriteString(fmt.Sprintf("   %s\n", ack.Message))
	return CommandOutcome{
		Transcript: b.String(),
		Effects:    []UIEffect{{Type: EffectRefresh}},
	}
}

func (e *Executor) cmdQueue() CommandOutcome {
	queuedTasks := e.srv.GetQueuedTasks()

	if len(queuedTasks) == 0 {
		return CommandOutcome{Transcript: "\n✓ Task queue is empty"}
	}

	var b strings.Builder
	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  📋 QUEUED TASKS (%d pending)\n", len(queuedTasks)))
	b.WriteString("═══════════════════════════════════════════════════════\n")

	for i, qt := range queuedTasks {
		timeInQueue := time.Since(qt.QueuedAt)
		workerStatus := "Waiting for scheduler"
		if qt.Task.TargetWorkerId != "" {
			workerStatus = qt.Task.TargetWorkerId
		}

		b.WriteString(fmt.Sprintf("\n[%d] Task ID: %s\n", i+1, qt.Task.TaskId))
		b.WriteString(fmt.Sprintf("    Assigned Worker: %s\n", workerStatus))
		b.WriteString(fmt.Sprintf("    Docker Image:    %s\n", qt.Task.DockerImage))
		b.WriteString(fmt.Sprintf("    User ID:         %s\n", qt.Task.UserId))
		b.WriteString("    ───────────────────────────────────────────────\n")
		b.WriteString("    Resource Requirements:\n")
		b.WriteString(fmt.Sprintf("      • CPU Cores:     %.2f cores\n", qt.Task.ReqCpu))
		b.WriteString(fmt.Sprintf("      • Memory:        %.2f GB\n", qt.Task.ReqMemory))
		b.WriteString(fmt.Sprintf("      • Storage:       %.2f GB\n", qt.Task.ReqStorage))
		b.WriteString("    ───────────────────────────────────────────────\n")
		b.WriteString(fmt.Sprintf("    Queued At:       %s\n", qt.QueuedAt.Format("2006-01-02 15:04:05")))
		b.WriteString(fmt.Sprintf("    Time in Queue:   %s\n", fmtDuration(timeInQueue)))
		b.WriteString(fmt.Sprintf("    Retry Attempts:  %d\n", qt.Retries))
		if qt.LastError != "" {
			b.WriteString(fmt.Sprintf("    Status:          %s\n", qt.LastError))
		}
	}

	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("  Note: Scheduler checks queue every 5s and assigns\n")
	b.WriteString("  tasks to workers with available resources\n")
	b.WriteString("═══════════════════════════════════════════════════════\n")
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdMonitor(taskID string) CommandOutcome {
	return CommandOutcome{
		Transcript: fmt.Sprintf("Opening monitor for task %s...", taskID),
		Effects:    []UIEffect{{Type: EffectOpenMonitor, Payload: taskID}},
	}
}

func (e *Executor) cmdBenchmark(parts []string) CommandOutcome {
	profile := benchmark.ProfileAll
	seed := time.Now().Unix()
	outputBase := filepath.Join("..", "results", "benchmarks")

	if len(parts) >= 2 {
		if parts[1] == "list" {
			var b strings.Builder
			b.WriteString("Available benchmark profiles:\n")
			for _, name := range benchmark.AvailableProfiles() {
				b.WriteString(fmt.Sprintf("  - %s\n", name))
			}
			b.WriteString("  - all\n")
			return CommandOutcome{Transcript: b.String()}
		}
		profile = parts[1]
	}

	for i := 2; i < len(parts); i++ {
		switch parts[i] {
		case "-seed":
			if i+1 < len(parts) {
				if parsed, err := strconv.ParseInt(parts[i+1], 10, 64); err == nil {
					seed = parsed
					i++
				}
			}
		case "-out":
			if i+1 < len(parts) {
				outputBase = parts[i+1]
				i++
			}
		}
	}

	var b strings.Builder
	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("  📊 SCHEDULER BENCHMARK SUITE\n")
	b.WriteString("═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  Profile: %s\n", profile))
	b.WriteString(fmt.Sprintf("  Seed:    %d\n", seed))
	b.WriteString(fmt.Sprintf("  Output:  %s\n", outputBase))
	b.WriteString("───────────────────────────────────────────────────────\n")

	suite, err := benchmark.RunSuite(profile, seed)
	if err != nil {
		b.WriteString(fmt.Sprintf("❌ Benchmark failed: %v\n", err))
		return CommandOutcome{Transcript: b.String(), Err: err}
	}

	outputDir, err := benchmark.WriteArtifacts(suite, outputBase)
	if err != nil {
		b.WriteString(fmt.Sprintf("❌ Failed to write benchmark artifacts: %v\n", err))
		return CommandOutcome{Transcript: b.String(), Err: err}
	}

	sortedResults := make([]benchmark.SchedulerResult, 0)
	for _, profileResult := range suite.Profiles {
		b.WriteString("\n-------------------------------------------------------\n")
		b.WriteString(fmt.Sprintf("Profile: %s\n", profileResult.Profile))
		b.WriteString(profileResult.Description + "\n")
		b.WriteString(fmt.Sprintf("Winner:  %s\n", profileResult.Winner))
		b.WriteString("-------------------------------------------------------\n")
		b.WriteString("Scheduler      SLA%    P95 Wait(s)  Throughput/min  Makespan(s)  CPU Util%  Balance\n")

		sortedResults = sortedResults[:0]
		sortedResults = append(sortedResults, profileResult.SchedulerResults...)
		sort.Slice(sortedResults, func(i, j int) bool {
			return sortedResults[i].SchedulerName < sortedResults[j].SchedulerName
		})
		for _, result := range sortedResults {
			m := result.Metrics
			b.WriteString(fmt.Sprintf("%-13s %-7.2f %-12.2f %-15.2f %-12.2f %-9.2f %.3f\n",
				result.SchedulerName,
				m.SLASuccessRatePct,
				m.P95QueueWaitSec,
				m.ThroughputTasksPerMin,
				m.MakespanSec,
				m.CPUUtilizationPct,
				m.WorkerBalanceScore,
			))
		}

		b.WriteString(fmt.Sprintf("\nRTS vs RR improvements: SLA %+0.2f%% | P95 wait %+0.2f%% | Makespan %+0.2f%% | Throughput %+0.2f%%\n",
			profileResult.SLAImprovementPct,
			profileResult.WaitP95ReductionPct,
			profileResult.MakespanReductionPct,
			profileResult.ThroughputGainPct,
		))
	}

	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("✅ Benchmark complete\n")
	b.WriteString(fmt.Sprintf("   Report folder: %s\n", outputDir))
	b.WriteString(fmt.Sprintf("   HTML report:   %s\n", filepath.Join(outputDir, "report.html")))
	b.WriteString(fmt.Sprintf("   Summary JSON:  %s\n", filepath.Join(outputDir, "summary.json")))
	b.WriteString(fmt.Sprintf("   Metrics CSV:   %s\n", filepath.Join(outputDir, "metrics.csv")))
	b.WriteString("═══════════════════════════════════════════════════════\n")
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdWorkloadSubmit(parts []string) CommandOutcome {
	if len(parts) < 2 {
		return CommandOutcome{
			Transcript: "Usage: workload-submit <profile> [-speed <factor>] [-limit <n>] [-dry-run]\nExample: workload-submit showcase -speed 10 -limit 30\nUse 'benchmark list' to view profiles",
		}
	}

	profileName := parts[1]
	speedFactor := 10.0
	limit := -1
	dryRun := false

	for i := 2; i < len(parts); i++ {
		switch parts[i] {
		case "-speed":
			if i+1 < len(parts) {
				if parsed, err := strconv.ParseFloat(parts[i+1], 64); err == nil && parsed > 0 {
					speedFactor = parsed
					i++
				}
			}
		case "-limit":
			if i+1 < len(parts) {
				if parsed, err := strconv.Atoi(parts[i+1]); err == nil && parsed > 0 {
					limit = parsed
					i++
				}
			}
		case "-dry-run":
			dryRun = true
		}
	}

	profile, err := benchmark.GetWorkloadProfile(profileName)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ %v", err),
			Err:        err,
		}
	}

	tasks := append([]benchmark.WorkloadTask(nil), profile.Tasks...)
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ArrivalOffset < tasks[j].ArrivalOffset })
	if limit > 0 && limit < len(tasks) {
		tasks = tasks[:limit]
	}

	var b strings.Builder
	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("  🧪 PREDEFINED WORKLOAD SUBMISSION\n")
	b.WriteString("═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  Profile:       %s\n", profile.Name))
	b.WriteString(fmt.Sprintf("  Description:   %s\n", profile.Description))
	b.WriteString(fmt.Sprintf("  Tasks:         %d\n", len(tasks)))
	b.WriteString(fmt.Sprintf("  Speed factor:  %.2fx\n", speedFactor))
	mode := "submit"
	if dryRun {
		mode = "dry-run"
	}
	b.WriteString(fmt.Sprintf("  Mode:          %s\n", mode))
	b.WriteString("───────────────────────────────────────────────────────\n")

	previousOffset := time.Duration(0)
	successCount := 0
	failureCount := 0

	for idx, wt := range tasks {
		delta := wt.ArrivalOffset - previousOffset
		if delta < 0 {
			delta = 0
		}
		previousOffset = wt.ArrivalOffset

		sleepFor := time.Duration(float64(delta) / speedFactor)
		if sleepFor > 0 {
			time.Sleep(sleepFor)
		}

		taskID := fmt.Sprintf("wl-%s-%03d-%d", profile.Name, idx, time.Now().UnixNano())
		taskName := wt.TaskName
		if taskName == "" {
			taskName = strings.ReplaceAll(wt.TaskType, "-", "_")
		}

		task := &pb.Task{
			TaskId:        taskID,
			TaskName:      taskName,
			DockerImage:   wt.DockerImage,
			Command:       wt.Command,
			ReqCpu:        wt.ReqCPU,
			ReqMemory:     wt.ReqMemory,
			ReqStorage:    wt.ReqStorage,
			TaskType:      wt.TaskType,
			SlaMultiplier: wt.SLAMultiplier,
			UserId:        "benchmark",
			SubmittedAt:   time.Now().Unix(),
		}

		if dryRun {
			b.WriteString(fmt.Sprintf("[%03d/%03d] %s type=%s cpu=%.1f mem=%.1f storage=%.1f offset=%s\n",
				idx+1, len(tasks), taskID, wt.TaskType, wt.ReqCPU, wt.ReqMemory, wt.ReqStorage, wt.ArrivalOffset))
			successCount++
			continue
		}

		ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		ack, submitErr := e.srv.SubmitTask(ctx, task)
		ctxCancel()
		if submitErr != nil || (ack != nil && !ack.Success) {
			failureCount++
			if submitErr != nil {
				b.WriteString(fmt.Sprintf("[%03d/%03d] ❌ %s submit error: %v\n", idx+1, len(tasks), taskID, submitErr))
			} else {
				b.WriteString(fmt.Sprintf("[%03d/%03d] ❌ %s rejected: %s\n", idx+1, len(tasks), taskID, ack.Message))
			}
			continue
		}

		successCount++
		b.WriteString(fmt.Sprintf("[%03d/%03d] ✅ %s queued\n", idx+1, len(tasks), taskID))
	}

	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	if dryRun {
		b.WriteString(fmt.Sprintf("✅ Dry-run complete (%d task events generated)\n", successCount))
	} else {
		b.WriteString(fmt.Sprintf("✅ Workload submission complete: %d queued, %d failed\n", successCount, failureCount))
		b.WriteString("   Use 'queue' and 'list-tasks running' to monitor execution\n")
	}
	b.WriteString("═══════════════════════════════════════════════════════\n")
	return CommandOutcome{
		Transcript: b.String(),
		Effects:    []UIEffect{{Type: EffectRefresh}},
	}
}

func (e *Executor) cmdListFiles(requestingUser, targetUser string) CommandOutcome {
	if e.fs == nil {
		return CommandOutcome{
			Transcript: "❌ File storage not available",
			Err:        fmt.Errorf("file storage not available"),
		}
	}

	fileList, err := e.fs.ListUserFilesWithAccess(requestingUser, targetUser)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to list files: %v", err),
			Err:        err,
		}
	}

	if len(fileList) == 0 {
		return CommandOutcome{Transcript: fmt.Sprintf("\n✓ No files found for user '%s'", targetUser)}
	}

	var b strings.Builder
	b.WriteString("\n╔═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("║  Files for User: %s\n", targetUser))
	b.WriteString(fmt.Sprintf("║  Total Tasks: %d\n", len(fileList)))
	b.WriteString("╠═══════════════════════════════════════════════════════\n")

	for i, metadata := range fileList {
		b.WriteString(fmt.Sprintf("║  [%d] Task: %s\n", i+1, metadata.TaskName))
		b.WriteString(fmt.Sprintf("║      Task ID:   %s\n", metadata.TaskID))
		b.WriteString(fmt.Sprintf("║      Timestamp: %s\n", metadata.Timestamp.Format("2006-01-02 15:04:05")))
		b.WriteString(fmt.Sprintf("║      Files:     %d\n", len(metadata.FilePaths)))
		for _, file := range metadata.FilePaths {
			b.WriteString(fmt.Sprintf("║        - %s\n", file))
		}
		if i < len(fileList)-1 {
			b.WriteString("║      ───────────────────────────────────────────────\n")
		}
	}

	b.WriteString("╚═══════════════════════════════════════════════════════\n")
	b.WriteString("\n💡 Tips:\n")
	b.WriteString(fmt.Sprintf("   - View task files: task-files <task_id> %s\n", targetUser))
	b.WriteString(fmt.Sprintf("   - Download file: download <task_id> <file_path> %s\n", targetUser))
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdTaskFiles(taskID, requestingUser, targetUser string) CommandOutcome {
	if e.fs == nil {
		return CommandOutcome{
			Transcript: "❌ File storage not available",
			Err:        fmt.Errorf("file storage not available"),
		}
	}

	metadata, err := e.fs.GetTaskFilesWithAccess(requestingUser, targetUser, taskID)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to get task files: %v", err),
			Err:        err,
		}
	}

	var b strings.Builder
	b.WriteString("\n╔═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("║  Task Files: %s\n", taskID))
	b.WriteString("╠═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("║  Task Name:  %s\n", metadata.TaskName))
	b.WriteString(fmt.Sprintf("║  Owner:      %s\n", targetUser))
	b.WriteString(fmt.Sprintf("║  Timestamp:  %s\n", metadata.Timestamp.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("║  Total Size: %s\n", fmtFileSize(metadata.TotalSize)))
	b.WriteString(fmt.Sprintf("║  Files:      %d\n", len(metadata.Files)))
	b.WriteString("╠═══════════════════════════════════════════════════════\n")

	if len(metadata.Files) == 0 {
		b.WriteString("║  No files generated by this task\n")
	} else {
		for i, file := range metadata.Files {
			b.WriteString(fmt.Sprintf("║  [%d] %s (%s)\n", i+1, file.Path, fmtFileSize(file.Size)))
		}
	}

	b.WriteString("╚═══════════════════════════════════════════════════════\n")

	if len(metadata.Files) > 0 {
		b.WriteString("\n💡 To download all files:\n")
		b.WriteString(fmt.Sprintf("   download %s %s\n", taskID, targetUser))
	}
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdDownload(taskID, requestingUser, targetUser, outputDir string) CommandOutcome {
	if e.fs == nil {
		return CommandOutcome{
			Transcript: "❌ File storage not available",
			Err:        fmt.Errorf("file storage not available"),
		}
	}

	metadata, err := e.fs.GetTaskFilesWithAccess(requestingUser, targetUser, taskID)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to get task files: %v", err),
			Err:        err,
		}
	}

	if len(metadata.FilePaths) == 0 {
		return CommandOutcome{Transcript: "❌ No files found for this task"}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Downloading %d file(s) from task '%s'...\n", len(metadata.FilePaths), taskID))

	taskOutputDir := filepath.Join(outputDir, taskID)
	if mkErr := os.MkdirAll(taskOutputDir, 0755); mkErr != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to create output directory: %v", mkErr),
			Err:        mkErr,
		}
	}

	successCount := 0
	totalSize := int64(0)

	for i, filePath := range metadata.FilePaths {
		b.WriteString(fmt.Sprintf("  [%d/%d] Downloading: %s\n", i+1, len(metadata.FilePaths), filePath))

		fileData, readErr := e.fs.ReadFileWithAccess(requestingUser, targetUser, taskID, filePath)
		if readErr != nil {
			b.WriteString(fmt.Sprintf("    ❌ Failed: %v\n", readErr))
			continue
		}

		outputPath := filepath.Join(taskOutputDir, filePath)
		if dirErr := os.MkdirAll(filepath.Dir(outputPath), 0755); dirErr != nil {
			b.WriteString(fmt.Sprintf("    ❌ Failed to create directory: %v\n", dirErr))
			continue
		}

		if wErr := os.WriteFile(outputPath, fileData, 0644); wErr != nil {
			b.WriteString(fmt.Sprintf("    ❌ Failed to write: %v\n", wErr))
			continue
		}

		totalSize += int64(len(fileData))
		successCount++
		b.WriteString(fmt.Sprintf("    ✓ Saved to: %s\n", outputPath))
	}

	b.WriteString("\n╔═══════════════════════════════════════════════════════\n")
	b.WriteString("║  Download Complete\n")
	b.WriteString("╠═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("║  Task:         %s\n", taskID))
	b.WriteString(fmt.Sprintf("║  Files:        %d/%d successful\n", successCount, len(metadata.FilePaths)))
	b.WriteString(fmt.Sprintf("║  Total Size:   %s\n", fmtFileSize(totalSize)))
	b.WriteString(fmt.Sprintf("║  Directory:    %s\n", taskOutputDir))
	b.WriteString("╚═══════════════════════════════════════════════════════\n")
	return CommandOutcome{Transcript: b.String()}
}

// ---------- Helpers ----------

func taskUsage() string {
	var b strings.Builder
	b.WriteString("Usage: task <docker_image> [-name <task_name>] [-cpu_cores <num>] [-mem <gb>] [-storage <gb>]\n")
	b.WriteString("  docker_image: Docker image to run\n")
	b.WriteString("  -name: Custom task name (default: auto-generated from image name)\n")
	b.WriteString("  -cpu_cores: CPU cores to allocate (default: 1.0)\n")
	b.WriteString("  -mem: Memory in GB (default: 0.5)\n")
	b.WriteString("  -storage: Storage in GB (default: 1.0)\n")
	b.WriteString("\nNote: The scheduler will automatically select the best worker.\n")
	b.WriteString("      Files generated in /output will be automatically collected and stored.\n")
	b.WriteString("\nExamples:\n")
	b.WriteString("  task docker.io/user/sample-task:latest\n")
	b.WriteString("  task docker.io/user/sample-task:latest -name my-experiment\n")
	b.WriteString("  task docker.io/user/sample-task:latest -cpu_cores 2.0 -mem 1.0 -storage 5.0\n")
	return b.String()
}

func dispatchUsage() string {
	var b strings.Builder
	b.WriteString("Usage: dispatch <worker_id> <docker_image> [-name <task_name>] [-cpu_cores <num>] [-mem <gb>] [-storage <gb>]\n")
	b.WriteString("  worker_id: Specific worker to dispatch task to\n")
	b.WriteString("  docker_image: Docker image to run\n")
	b.WriteString("  -name: Custom task name (default: auto-generated from image name)\n")
	b.WriteString("  -cpu_cores: CPU cores to allocate (default: 1.0)\n")
	b.WriteString("  -mem: Memory in GB (default: 0.5)\n")
	b.WriteString("  -storage: Storage in GB (default: 1.0)\n")
	b.WriteString("\nNote: This bypasses the scheduler and directly assigns to the specified worker.\n")
	b.WriteString("      Files generated in /output will be automatically collected and stored.\n")
	b.WriteString("\nExamples:\n")
	b.WriteString("  dispatch worker-1 docker.io/user/sample-task:latest\n")
	b.WriteString("  dispatch worker-1 docker.io/user/sample-task:latest -name my-experiment\n")
	b.WriteString("  dispatch worker-2 docker.io/user/sample-task:latest -cpu_cores 2.0 -mem 1.0\n")
	return b.String()
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func fmtFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// StreamTaskLogs starts streaming task logs. Blocking - run in a goroutine.
func (e *Executor) StreamTaskLogs(ctx context.Context, taskID string, handler func(line string, complete bool, status string) error) error {
	userID, err := e.srv.GetUserIDForTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task info: %w", err)
	}
	return e.srv.StreamTaskLogsUnified(ctx, taskID, userID, func(logLine string, isComplete bool, status string) error {
		return handler(logLine, isComplete, status)
	})
}
