package server

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"master/internal/scheduler"
	pb "master/proto"
)

// ---------------------------------------------------------------------------
// End-to-end scheduler integration tests
//
// These tests exercise the full in-memory scheduling pipeline:
//   register workers → submit tasks → schedule → verify assignments → analytics
//
// They do NOT start gRPC or MongoDB — everything runs in-process using the
// MasterServer's in-memory state.
// ---------------------------------------------------------------------------

// schedulerTestHarness wraps a MasterServer with helpers for testing.
type schedulerTestHarness struct {
	t       *testing.T
	server  *MasterServer
	sched   scheduler.Scheduler
	results map[string]*scheduledResult // taskID → result
}

type scheduledResult struct {
	TaskID   string
	WorkerID string
	ReqCPU   float64
	ReqMem   float64
	ReqStor  float64
	TaskType string
}

func newSchedulerTestHarness(t *testing.T, sched scheduler.Scheduler) *schedulerTestHarness {
	t.Helper()
	s := NewMasterServer(nil, nil, nil, nil, nil, nil, nil, nil)
	s.SetScheduler(sched)
	return &schedulerTestHarness{
		t:       t,
		server:  s,
		sched:   sched,
		results: make(map[string]*scheduledResult),
	}
}

func (h *schedulerTestHarness) addWorker(id, ip string, cpu, mem, stor float64) {
	h.t.Helper()
	h.server.mu.Lock()
	defer h.server.mu.Unlock()
	h.server.workers[id] = &WorkerState{
		Info: &pb.WorkerInfo{
			WorkerId:     id,
			WorkerIp:     ip,
			TotalCpu:     cpu,
			TotalMemory:  mem,
			TotalStorage: stor,
		},
		IsActive:         true,
		LastHeartbeat:     time.Now().Unix(),
		RunningTasks:      make(map[string]bool),
		AvailableCPU:      cpu,
		AvailableMemory:   mem,
		AvailableStorage:  stor,
		AllocatedCPU:      0,
		AllocatedMemory:   0,
		AllocatedStorage:  0,
	}
}

// submitAndSchedule enqueues a task, runs selection, and simulates resource
// reservation (the part assignTaskToWorker does before gRPC). Returns the
// selected workerID or "" if unschedulable.
func (h *schedulerTestHarness) submitAndSchedule(task *pb.Task) string {
	h.t.Helper()

	// Normalise the task just like the real path does.
	normalizeTaskForScheduling(task)

	selected := h.server.selectWorkerForTask(task)
	if selected == "" {
		return ""
	}

	// Simulate resource reservation (same logic as assignTaskToWorker).
	h.server.mu.Lock()
	worker, ok := h.server.workers[selected]
	if ok {
		worker.AllocatedCPU += task.ReqCpu
		worker.AllocatedMemory += task.ReqMemory
		worker.AllocatedStorage += task.ReqStorage
		worker.AvailableCPU -= task.ReqCpu
		worker.AvailableMemory -= task.ReqMemory
		worker.AvailableStorage -= task.ReqStorage
		if worker.RunningTasks == nil {
			worker.RunningTasks = make(map[string]bool)
		}
		worker.RunningTasks[task.TaskId] = true
	}
	h.server.mu.Unlock()

	h.results[task.TaskId] = &scheduledResult{
		TaskID:   task.TaskId,
		WorkerID: selected,
		ReqCPU:   task.ReqCpu,
		ReqMem:   task.ReqMemory,
		ReqStor:  task.ReqStorage,
		TaskType: task.TaskType,
	}
	return selected
}

// simulateCompletion releases resources as if the task finished.
func (h *schedulerTestHarness) simulateCompletion(taskID string) {
	h.t.Helper()
	res, ok := h.results[taskID]
	if !ok {
		return
	}
	h.server.mu.Lock()
	defer h.server.mu.Unlock()
	worker, exists := h.server.workers[res.WorkerID]
	if !exists {
		return
	}
	worker.AllocatedCPU -= res.ReqCPU
	worker.AllocatedMemory -= res.ReqMem
	worker.AllocatedStorage -= res.ReqStor
	worker.AvailableCPU += res.ReqCPU
	worker.AvailableMemory += res.ReqMem
	worker.AvailableStorage += res.ReqStor
	if worker.AllocatedCPU < 0 {
		worker.AllocatedCPU = 0
	}
	if worker.AllocatedMemory < 0 {
		worker.AllocatedMemory = 0
	}
	if worker.AllocatedStorage < 0 {
		worker.AllocatedStorage = 0
	}
	delete(worker.RunningTasks, taskID)
}

// --- Analytics helpers ---

type schedulerAnalytics struct {
	TotalTasks        int
	ScheduledTasks    int
	Unschedulable     int
	AssignmentsByWorker map[string]int
	BalanceScore      float64 // coefficient of variation of assignment counts
}

func (h *schedulerTestHarness) computeAnalytics() schedulerAnalytics {
	assigned := make(map[string]int)
	for _, r := range h.results {
		assigned[r.WorkerID]++
	}

	counts := make([]float64, 0, len(assigned))
	for _, c := range assigned {
		counts = append(counts, float64(c))
	}

	var balanceScore float64
	if len(counts) > 1 {
		var sum float64
		for _, c := range counts {
			sum += c
		}
		mean := sum / float64(len(counts))
		var variance float64
		for _, c := range counts {
			d := c - mean
			variance += d * d
		}
		variance /= float64(len(counts))
		if mean > 0 {
			balanceScore = 1.0 - (variance / (mean * mean)) // 1 = perfect balance
		}
	}

	return schedulerAnalytics{
		AssignmentsByWorker: assigned,
		ScheduledTasks:      len(h.results),
		BalanceScore:        balanceScore,
	}
}

func (h *schedulerTestHarness) snapshotResources() map[string][3]float64 {
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()
	snap := make(map[string][3]float64)
	for id, w := range h.server.workers {
		snap[id] = [3]float64{w.AvailableCPU, w.AvailableMemory, w.AvailableStorage}
	}
	return snap
}

// =========================================================================
// Tests
// =========================================================================

func TestE2E_RoundRobinDistributesAcrossWorkers(t *testing.T) {
	h := newSchedulerTestHarness(t, scheduler.NewRoundRobinScheduler())
	h.addWorker("w-small", "10.0.0.1:50052", 2, 4, 20)
	h.addWorker("w-medium", "10.0.0.2:50052", 4, 8, 40)
	h.addWorker("w-large", "10.0.0.3:50052", 8, 16, 80)

	// Submit 6 lightweight tasks — RR should spread them.
	for i := 0; i < 6; i++ {
		task := &pb.Task{
			TaskId:    fmt.Sprintf("t-%d", i),
			ReqCpu:    0.5,
			ReqMemory: 0.5,
			ReqStorage: 1.0,
			TaskType:  "cpu-light",
		}
		w := h.submitAndSchedule(task)
		if w == "" {
			t.Fatalf("task t-%d: expected scheduling, got unschedulable", i)
		}
	}

	analytics := h.computeAnalytics()
	if analytics.ScheduledTasks != 6 {
		t.Fatalf("expected 6 scheduled, got %d", analytics.ScheduledTasks)
	}

	// Each worker should get exactly 2 tasks with 3 workers and 6 tasks.
	for wID, count := range analytics.AssignmentsByWorker {
		if count != 2 {
			t.Fatalf("worker %s got %d tasks, expected 2 for balanced RR", wID, count)
		}
	}
}

func TestE2E_SchedulerRespectsResourceLimits(t *testing.T) {
	h := newSchedulerTestHarness(t, scheduler.NewRoundRobinScheduler())
	// Small worker that can only hold 2 tasks.
	h.addWorker("w-tiny", "10.0.0.1:50052", 2, 2, 20)

	task1 := &pb.Task{TaskId: "t-1", ReqCpu: 1, ReqMemory: 1, ReqStorage: 5, TaskType: "cpu-light"}
	task2 := &pb.Task{TaskId: "t-2", ReqCpu: 1, ReqMemory: 1, ReqStorage: 5, TaskType: "cpu-light"}
	task3 := &pb.Task{TaskId: "t-3", ReqCpu: 1, ReqMemory: 1, ReqStorage: 5, TaskType: "cpu-light"}

	w1 := h.submitAndSchedule(task1)
	w2 := h.submitAndSchedule(task2)
	w3 := h.submitAndSchedule(task3)

	if w1 == "" || w2 == "" {
		t.Fatal("first two tasks should be schedulable")
	}
	if w3 != "" {
		t.Fatalf("third task should be unschedulable (no CPU left), got %s", w3)
	}

	// After completing task1, task3 should become schedulable.
	h.simulateCompletion("t-1")
	w3 = h.submitAndSchedule(task3)
	if w3 == "" {
		t.Fatal("task3 should be schedulable after completion of task1")
	}
}

func TestE2E_ResourceTrackingAccuracy(t *testing.T) {
	h := newSchedulerTestHarness(t, scheduler.NewRoundRobinScheduler())
	h.addWorker("w-1", "10.0.0.1:50052", 4, 8, 40)

	before := h.snapshotResources()

	task := &pb.Task{TaskId: "t-1", ReqCpu: 1.5, ReqMemory: 3.0, ReqStorage: 10.0, TaskType: "cpu-heavy"}
	h.submitAndSchedule(task)

	after := h.snapshotResources()

	// Available should decrease by exactly the requested amounts.
	cpuDelta := before["w-1"][0] - after["w-1"][0]
	memDelta := before["w-1"][1] - after["w-1"][1]
	storDelta := before["w-1"][2] - after["w-1"][2]

	if cpuDelta != 1.5 {
		t.Fatalf("CPU delta: expected 1.5, got %f", cpuDelta)
	}
	if memDelta != 3.0 {
		t.Fatalf("Memory delta: expected 3.0, got %f", memDelta)
	}
	if storDelta != 10.0 {
		t.Fatalf("Storage delta: expected 10.0, got %f", storDelta)
	}

	// After completion, resources should be fully restored.
	h.simulateCompletion("t-1")
	restored := h.snapshotResources()
	if restored["w-1"][0] != before["w-1"][0] {
		t.Fatalf("CPU not restored: expected %f, got %f", before["w-1"][0], restored["w-1"][0])
	}
	if restored["w-1"][1] != before["w-1"][1] {
		t.Fatalf("Memory not restored: expected %f, got %f", before["w-1"][1], restored["w-1"][1])
	}
}

func TestE2E_HeterogeneousClusterFavorsSuitableWorkers(t *testing.T) {
	h := newSchedulerTestHarness(t, scheduler.NewRoundRobinScheduler())
	// Worker with lots of CPU but little memory.
	h.addWorker("w-cpu", "10.0.0.1:50052", 16, 2, 40)
	// Worker with lots of memory but little CPU.
	h.addWorker("w-mem", "10.0.0.2:50052", 2, 32, 40)

	// Memory-heavy task should only be schedulable on w-mem.
	memTask := &pb.Task{TaskId: "t-mem", ReqCpu: 1, ReqMemory: 16, ReqStorage: 5, TaskType: "memory-heavy"}
	w := h.submitAndSchedule(memTask)
	if w != "w-mem" {
		t.Fatalf("memory-heavy task should go to w-mem, got %s", w)
	}

	// CPU-heavy task should only be schedulable on w-cpu.
	cpuTask := &pb.Task{TaskId: "t-cpu", ReqCpu: 8, ReqMemory: 1, ReqStorage: 5, TaskType: "cpu-heavy"}
	w = h.submitAndSchedule(cpuTask)
	if w != "w-cpu" {
		t.Fatalf("cpu-heavy task should go to w-cpu, got %s", w)
	}
}

func TestE2E_MixedWorkloadBatch(t *testing.T) {
	h := newSchedulerTestHarness(t, scheduler.NewRoundRobinScheduler())
	h.addWorker("w-small", "10.0.0.1:50052", 2, 4, 20)
	h.addWorker("w-medium", "10.0.0.2:50052", 4, 8, 40)
	h.addWorker("w-large", "10.0.0.3:50052", 8, 16, 80)

	tasks := []*pb.Task{
		{TaskId: "cpu-light-1", ReqCpu: 0.3, ReqMemory: 0.25, ReqStorage: 1, TaskType: "cpu-light"},
		{TaskId: "cpu-light-2", ReqCpu: 0.5, ReqMemory: 0.5, ReqStorage: 1, TaskType: "cpu-light"},
		{TaskId: "cpu-heavy-1", ReqCpu: 2.0, ReqMemory: 1.0, ReqStorage: 5, TaskType: "cpu-heavy"},
		{TaskId: "mem-heavy-1", ReqCpu: 1.0, ReqMemory: 6.0, ReqStorage: 5, TaskType: "memory-heavy"},
		{TaskId: "mixed-1", ReqCpu: 1.5, ReqMemory: 2.0, ReqStorage: 10, TaskType: "mixed"},
		{TaskId: "cpu-heavy-2", ReqCpu: 3.0, ReqMemory: 2.0, ReqStorage: 5, TaskType: "cpu-heavy"},
		{TaskId: "mixed-2", ReqCpu: 1.0, ReqMemory: 1.5, ReqStorage: 5, TaskType: "mixed"},
		{TaskId: "cpu-light-3", ReqCpu: 0.4, ReqMemory: 0.3, ReqStorage: 1, TaskType: "cpu-light"},
	}

	scheduled := 0
	unschedulable := 0
	for _, task := range tasks {
		w := h.submitAndSchedule(task)
		if w != "" {
			scheduled++
		} else {
			unschedulable++
		}
	}

	analytics := h.computeAnalytics()
	t.Logf("Mixed workload results: scheduled=%d unschedulable=%d", scheduled, unschedulable)
	t.Logf("Assignments: %v", analytics.AssignmentsByWorker)
	t.Logf("Balance score: %.3f", analytics.BalanceScore)

	if scheduled == 0 {
		t.Fatal("expected at least some tasks scheduled")
	}
	// With 14 CPU / 28 GB mem across cluster and ~9.7 CPU / ~13.55 mem requested,
	// all 8 tasks should be schedulable.
	if scheduled != 8 {
		t.Fatalf("expected all 8 tasks scheduled, got %d (unschedulable=%d)", scheduled, unschedulable)
	}
}

func TestE2E_WorkerInactiveExcludedFromScheduling(t *testing.T) {
	h := newSchedulerTestHarness(t, scheduler.NewRoundRobinScheduler())
	h.addWorker("w-1", "10.0.0.1:50052", 4, 8, 40)
	h.addWorker("w-2", "10.0.0.2:50052", 4, 8, 40)

	// Mark w-2 as inactive.
	h.server.mu.Lock()
	h.server.workers["w-2"].IsActive = false
	h.server.mu.Unlock()

	// All tasks should go to w-1.
	for i := 0; i < 4; i++ {
		task := &pb.Task{TaskId: fmt.Sprintf("t-%d", i), ReqCpu: 0.5, ReqMemory: 0.5, ReqStorage: 1}
		w := h.submitAndSchedule(task)
		if w != "w-1" {
			t.Fatalf("task t-%d should go to w-1 (w-2 is inactive), got %s", i, w)
		}
	}
}

func TestE2E_ClusterSnapshotReflectsState(t *testing.T) {
	h := newSchedulerTestHarness(t, scheduler.NewRoundRobinScheduler())
	h.addWorker("w-1", "10.0.0.1:50052", 4, 8, 40)
	h.addWorker("w-2", "10.0.0.2:50052", 2, 4, 20)

	// Submit 2 tasks.
	h.submitAndSchedule(&pb.Task{TaskId: "t-1", ReqCpu: 1, ReqMemory: 2, ReqStorage: 5, TaskType: "cpu-light"})
	h.submitAndSchedule(&pb.Task{TaskId: "t-2", ReqCpu: 1, ReqMemory: 2, ReqStorage: 5, TaskType: "cpu-light"})

	// Enqueue another task so we can verify queue state.
	h.server.EnqueueTask(&pb.Task{TaskId: "t-queued", ReqCpu: 1, ReqMemory: 1, ReqStorage: 1}, "test")

	snap := h.server.GetClusterSnapshot()
	if snap == nil {
		t.Fatal("snapshot is nil")
	}
	if len(snap.Workers) != 2 {
		t.Fatalf("expected 2 workers in snapshot, got %d", len(snap.Workers))
	}

	// Verify running tasks are reflected.
	totalRunning := 0
	for _, ws := range snap.Workers {
		totalRunning += ws.TaskCount
		if ws.Status != "active" {
			t.Fatalf("worker %s should be active, got %s", ws.WorkerID, ws.Status)
		}
	}
	if totalRunning != 2 {
		t.Fatalf("expected 2 running tasks across cluster, got %d", totalRunning)
	}

	// Verify queue.
	queued := h.server.GetQueuedTasks()
	if len(queued) != 1 {
		t.Fatalf("expected 1 queued task, got %d", len(queued))
	}
}

func TestE2E_SchedulerComparisonAnalytics(t *testing.T) {
	// Run the same workload through two schedulers and compare analytics.
	// This mirrors what the benchmark suite does but through the server path.

	workers := []struct {
		id   string
		ip   string
		cpu  float64
		mem  float64
		stor float64
	}{
		{"w-small", "10.0.0.1:50052", 2, 4, 20},
		{"w-medium", "10.0.0.2:50052", 4, 8, 40},
		{"w-large", "10.0.0.3:50052", 8, 16, 80},
	}

	tasks := []*pb.Task{
		{TaskId: "t-1", ReqCpu: 0.5, ReqMemory: 0.5, ReqStorage: 1, TaskType: "cpu-light"},
		{TaskId: "t-2", ReqCpu: 1.0, ReqMemory: 1.0, ReqStorage: 2, TaskType: "cpu-light"},
		{TaskId: "t-3", ReqCpu: 2.0, ReqMemory: 1.5, ReqStorage: 5, TaskType: "cpu-heavy"},
		{TaskId: "t-4", ReqCpu: 1.0, ReqMemory: 4.0, ReqStorage: 5, TaskType: "memory-heavy"},
		{TaskId: "t-5", ReqCpu: 1.5, ReqMemory: 2.0, ReqStorage: 10, TaskType: "mixed"},
		{TaskId: "t-6", ReqCpu: 0.3, ReqMemory: 0.3, ReqStorage: 1, TaskType: "cpu-light"},
		{TaskId: "t-7", ReqCpu: 2.5, ReqMemory: 3.0, ReqStorage: 5, TaskType: "mixed"},
		{TaskId: "t-8", ReqCpu: 0.5, ReqMemory: 6.0, ReqStorage: 5, TaskType: "memory-heavy"},
		{TaskId: "t-9", ReqCpu: 3.0, ReqMemory: 2.0, ReqStorage: 5, TaskType: "cpu-heavy"},
		{TaskId: "t-10", ReqCpu: 1.0, ReqMemory: 1.0, ReqStorage: 2, TaskType: "cpu-light"},
	}

	runWithScheduler := func(sched scheduler.Scheduler) schedulerAnalytics {
		h := newSchedulerTestHarness(t, sched)
		for _, w := range workers {
			h.addWorker(w.id, w.ip, w.cpu, w.mem, w.stor)
		}
		for _, task := range tasks {
			// Deep copy the proto to avoid shared state between runs.
			taskCopy := &pb.Task{
				TaskId:     task.TaskId,
				ReqCpu:     task.ReqCpu,
				ReqMemory:  task.ReqMemory,
				ReqStorage: task.ReqStorage,
				TaskType:   task.TaskType,
			}
			h.submitAndSchedule(taskCopy)
		}
		return h.computeAnalytics()
	}

	rrAnalytics := runWithScheduler(scheduler.NewRoundRobinScheduler())

	t.Logf("=== Round-Robin Analytics ===")
	t.Logf("  Scheduled:   %d / %d", rrAnalytics.ScheduledTasks, len(tasks))
	t.Logf("  Balance:     %.3f", rrAnalytics.BalanceScore)

	// Print per-worker breakdown.
	workerIDs := make([]string, 0, len(rrAnalytics.AssignmentsByWorker))
	for id := range rrAnalytics.AssignmentsByWorker {
		workerIDs = append(workerIDs, id)
	}
	sort.Strings(workerIDs)
	for _, id := range workerIDs {
		t.Logf("  %s: %d tasks", id, rrAnalytics.AssignmentsByWorker[id])
	}

	// Assertions.
	if rrAnalytics.ScheduledTasks != len(tasks) {
		t.Fatalf("RR: expected all %d tasks scheduled, got %d", len(tasks), rrAnalytics.ScheduledTasks)
	}

	// With 3 workers and 10 tasks, at least 2 workers must have tasks.
	if len(rrAnalytics.AssignmentsByWorker) < 2 {
		t.Fatalf("RR: expected distribution across at least 2 workers, got %d", len(rrAnalytics.AssignmentsByWorker))
	}
}

func TestE2E_SaturationThenDrain(t *testing.T) {
	// Fill the cluster to saturation, then drain and verify resources recover.
	h := newSchedulerTestHarness(t, scheduler.NewRoundRobinScheduler())
	h.addWorker("w-1", "10.0.0.1:50052", 4, 8, 40)

	// Saturate: submit 4 tasks each using 1 CPU.
	var scheduledTasks []string
	for i := 0; i < 4; i++ {
		task := &pb.Task{
			TaskId: fmt.Sprintf("t-%d", i), ReqCpu: 1, ReqMemory: 1, ReqStorage: 5,
			TaskType: "cpu-light",
		}
		w := h.submitAndSchedule(task)
		if w == "" {
			t.Fatalf("task t-%d should be schedulable during saturation", i)
		}
		scheduledTasks = append(scheduledTasks, task.TaskId)
	}

	// Cluster should be saturated — next task fails.
	overflow := &pb.Task{TaskId: "t-overflow", ReqCpu: 1, ReqMemory: 1, ReqStorage: 5, TaskType: "cpu-light"}
	if w := h.submitAndSchedule(overflow); w != "" {
		t.Fatal("expected saturated cluster to reject task, but it was scheduled")
	}

	// Drain: complete all tasks.
	for _, tid := range scheduledTasks {
		h.simulateCompletion(tid)
	}

	// Resources should be fully recovered.
	snap := h.snapshotResources()
	if snap["w-1"][0] != 4.0 {
		t.Fatalf("expected 4.0 CPU available after drain, got %f", snap["w-1"][0])
	}
	if snap["w-1"][1] != 8.0 {
		t.Fatalf("expected 8.0 mem available after drain, got %f", snap["w-1"][1])
	}

	// Now the overflow task should succeed.
	if w := h.submitAndSchedule(overflow); w == "" {
		t.Fatal("expected overflow task to schedule after drain")
	}
}

func TestE2E_QueueEnqueueAndInspect(t *testing.T) {
	h := newSchedulerTestHarness(t, scheduler.NewRoundRobinScheduler())
	h.addWorker("w-1", "10.0.0.1:50052", 2, 2, 20)

	// Submit multiple tasks to the queue via EnqueueTask.
	for i := 0; i < 5; i++ {
		h.server.EnqueueTask(&pb.Task{
			TaskId:    fmt.Sprintf("q-%d", i),
			ReqCpu:    0.5,
			ReqMemory: 0.5,
			TaskType:  "cpu-light",
		}, "batch submission")
	}

	queued := h.server.GetQueuedTasks()
	if len(queued) != 5 {
		t.Fatalf("expected 5 queued tasks, got %d", len(queued))
	}

	// Verify FIFO ordering.
	for i, qt := range queued {
		expected := fmt.Sprintf("q-%d", i)
		if qt.Task.TaskId != expected {
			t.Fatalf("queue position %d: expected %s, got %s", i, expected, qt.Task.TaskId)
		}
	}
}
