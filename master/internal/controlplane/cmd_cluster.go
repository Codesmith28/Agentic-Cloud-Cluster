package controlplane

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ---------- Individual cluster command handlers ----------

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
