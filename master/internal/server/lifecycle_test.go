package server

import (
	"testing"
	"time"

	"master/internal/db"
	"master/internal/scheduler"
	pb "master/proto"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTestServer creates a minimal MasterServer with nil DB dependencies,
// suitable for testing in-memory logic that does not touch MongoDB.
func newTestServer() *MasterServer {
	s := NewMasterServer(nil, nil, nil, nil, nil, nil, nil, nil)
	return s
}

func newTestServerWithScheduler(sched scheduler.Scheduler) *MasterServer {
	s := newTestServer()
	s.SetScheduler(sched)
	return s
}

func addWorkerToServer(s *MasterServer, id, ip string, totalCPU, totalMem, totalStor float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers[id] = &WorkerState{
		Info: &pb.WorkerInfo{
			WorkerId:     id,
			WorkerIp:     ip,
			TotalCpu:     totalCPU,
			TotalMemory:  totalMem,
			TotalStorage: totalStor,
		},
		IsActive:         true,
		LastHeartbeat:     time.Now().Unix(),
		RunningTasks:      make(map[string]bool),
		AvailableCPU:      totalCPU,
		AvailableMemory:   totalMem,
		AvailableStorage:  totalStor,
		AllocatedCPU:      0,
		AllocatedMemory:   0,
		AllocatedStorage:  0,
	}
}

// ---------------------------------------------------------------------------
// Task normalization tests
// ---------------------------------------------------------------------------

func TestNormalizeTaskTypeInference(t *testing.T) {
	tests := []struct {
		name     string
		cpu      float64
		mem      float64
		wantType string
	}{
		{"cpu-light for low resources", 0.5, 0.5, "cpu-light"},
		{"cpu-heavy for high CPU", 5.0, 1.0, "cpu-heavy"},
		{"memory-heavy for high memory", 1.0, 16.0, "memory-heavy"},
		{"mixed for zero CPU", 0.0, 0.0, "mixed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &pb.Task{
				TaskId:    "t-test",
				ReqCpu:    tt.cpu,
				ReqMemory: tt.mem,
				TaskType:  "", // force inference
			}
			normalizeTaskForScheduling(task)
			if task.TaskType != tt.wantType {
				t.Fatalf("expected type %s, got %s (cpu=%.1f mem=%.1f)",
					tt.wantType, task.TaskType, tt.cpu, tt.mem)
			}
		})
	}
}

func TestNormalizeTaskSetsDefaults(t *testing.T) {
	task := &pb.Task{
		TaskId:    "t-1",
		ReqCpu:    1.0,
		ReqMemory: 2.0,
	}
	meta := normalizeTaskForScheduling(task)

	if task.SubmittedAt <= 0 {
		t.Fatal("expected SubmittedAt to be set")
	}
	if task.SlaMultiplier <= 0 {
		t.Fatal("expected SlaMultiplier to be set")
	}
	if meta.deadline.IsZero() {
		t.Fatal("expected deadline to be computed")
	}
	if meta.tau <= 0 {
		t.Fatal("expected tau to be positive")
	}
}

func TestNormalizeTaskPreservesExplicitType(t *testing.T) {
	task := &pb.Task{
		TaskId:    "t-1",
		ReqCpu:    0.5,
		ReqMemory: 0.5,
		TaskType:  "mixed",
	}
	normalizeTaskForScheduling(task)
	if task.TaskType != "mixed" {
		t.Fatalf("expected explicit type preserved, got %s", task.TaskType)
	}
}

// ---------------------------------------------------------------------------
// Terminal state tests
// ---------------------------------------------------------------------------

func TestTaskIsTerminal(t *testing.T) {
	terminal := []string{"completed", "failed", "cancelled"}
	for _, s := range terminal {
		if !taskIsTerminal(s) {
			t.Fatalf("expected %q to be terminal", s)
		}
	}

	nonTerminal := []string{"pending", "running", "queued", "assigned", ""}
	for _, s := range nonTerminal {
		if taskIsTerminal(s) {
			t.Fatalf("expected %q to be non-terminal", s)
		}
	}
}

// ---------------------------------------------------------------------------
// Attempt ID generation tests
// ---------------------------------------------------------------------------

func TestNextAttemptIDFormat(t *testing.T) {
	tests := []struct {
		taskID    string
		attemptNo int32
		want      string
	}{
		{"task-1", 1, "att-task-1-1"},
		{"task-1", 2, "att-task-1-2"},
		{"abc-def", 10, "att-abc-def-10"},
	}
	for _, tt := range tests {
		got := nextAttemptID(tt.taskID, tt.attemptNo)
		if got != tt.want {
			t.Fatalf("nextAttemptID(%q, %d) = %q, want %q", tt.taskID, tt.attemptNo, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Stale-result suppression tests (expanded)
// ---------------------------------------------------------------------------

func TestShouldIgnoreAttemptResultComprehensive(t *testing.T) {
	tests := []struct {
		name             string
		currentAttemptID string
		resultAttemptID  string
		persistedStatus  string
		want             bool
	}{
		// Accept current active attempt
		{"current attempt accepted", "att-t-1-2", "att-t-1-2", "", false},
		{"current attempt accepted with running status", "att-t-1-2", "att-t-1-2", db.AttemptStatusRunning, false},
		{"current attempt accepted with assigned status", "att-t-1-1", "att-t-1-1", db.AttemptStatusAssigned, false},

		// Reject stale/old attempts
		{"old attempt rejected", "att-t-1-2", "att-t-1-1", "", true},
		{"lost attempt rejected", "att-t-1-2", "att-t-1-2", db.AttemptStatusLost, true},
		{"stale attempt rejected", "att-t-1-2", "att-t-1-1", db.AttemptStatusStale, true},

		// Edge cases
		{"empty result attempt is not stale", "att-t-1-2", "", "", false},
		{"both empty is not stale", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldIgnoreAttemptResult(tt.currentAttemptID, tt.resultAttemptID, tt.persistedStatus)
			if got != tt.want {
				t.Fatalf("shouldIgnoreAttemptResult(%q, %q, %q) = %v, want %v",
					tt.currentAttemptID, tt.resultAttemptID, tt.persistedStatus, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Resource release tests (in-memory, no DB)
// ---------------------------------------------------------------------------

func TestReleaseTaskResourcesClampsToZero(t *testing.T) {
	s := newTestServer()
	addWorkerToServer(s, "w-1", "127.0.0.1:50052", 4, 8, 40)

	s.mu.Lock()
	worker := s.workers["w-1"]
	// Simulate allocation
	worker.AllocatedCPU = 2.0
	worker.AllocatedMemory = 4.0
	worker.AllocatedStorage = 20.0
	worker.AvailableCPU = 2.0
	worker.AvailableMemory = 4.0
	worker.AvailableStorage = 20.0

	task := &db.Task{TaskID: "t-1", ReqCPU: 3.0, ReqMemory: 5.0, ReqStorage: 25.0}
	s.releaseTaskResourcesLocked(nil, "w-1", worker, task)
	s.mu.Unlock()

	// Allocated should clamp to 0 (not go negative)
	if worker.AllocatedCPU < 0 {
		t.Fatalf("AllocatedCPU went negative: %f", worker.AllocatedCPU)
	}
	if worker.AllocatedMemory < 0 {
		t.Fatalf("AllocatedMemory went negative: %f", worker.AllocatedMemory)
	}
	// Available should clamp to total
	if worker.AvailableCPU > worker.Info.TotalCpu {
		t.Fatalf("AvailableCPU exceeded total: %f > %f", worker.AvailableCPU, worker.Info.TotalCpu)
	}
	if worker.AvailableMemory > worker.Info.TotalMemory {
		t.Fatalf("AvailableMemory exceeded total: %f > %f", worker.AvailableMemory, worker.Info.TotalMemory)
	}
}

func TestReleaseTaskResourcesUpdatesCorrectly(t *testing.T) {
	s := newTestServer()
	addWorkerToServer(s, "w-1", "127.0.0.1:50052", 8, 16, 100)

	s.mu.Lock()
	worker := s.workers["w-1"]
	worker.AllocatedCPU = 4.0
	worker.AllocatedMemory = 8.0
	worker.AllocatedStorage = 50.0
	worker.AvailableCPU = 4.0
	worker.AvailableMemory = 8.0
	worker.AvailableStorage = 50.0
	worker.RunningTasks["t-1"] = true

	task := &db.Task{TaskID: "t-1", ReqCPU: 2.0, ReqMemory: 4.0, ReqStorage: 20.0}
	s.releaseTaskResourcesLocked(nil, "w-1", worker, task)
	s.mu.Unlock()

	if worker.AllocatedCPU != 2.0 {
		t.Fatalf("expected AllocatedCPU=2.0, got %f", worker.AllocatedCPU)
	}
	if worker.AvailableCPU != 6.0 {
		t.Fatalf("expected AvailableCPU=6.0, got %f", worker.AvailableCPU)
	}
	if _, exists := worker.RunningTasks["t-1"]; exists {
		t.Fatal("expected task t-1 removed from RunningTasks")
	}
}

func TestReleaseNilWorkerOrTaskDoesNotPanic(t *testing.T) {
	s := newTestServer()
	// Should not panic
	s.releaseTaskResourcesLocked(nil, "w-1", nil, &db.Task{TaskID: "t-1"})
	s.releaseTaskResourcesLocked(nil, "w-1", &WorkerState{}, nil)
}

// ---------------------------------------------------------------------------
// Queue management tests
// ---------------------------------------------------------------------------

func TestEnqueueAndGetQueuedTasks(t *testing.T) {
	s := newTestServer()
	task := &pb.Task{TaskId: "t-1", ReqCpu: 1, ReqMemory: 1}
	s.EnqueueTask(task, "test submission")

	queued := s.GetQueuedTasks()
	if len(queued) != 1 {
		t.Fatalf("expected 1 queued task, got %d", len(queued))
	}
	if queued[0].Task.TaskId != "t-1" {
		t.Fatalf("expected task t-1, got %s", queued[0].Task.TaskId)
	}
}

func TestRemoveQueuedTaskByID(t *testing.T) {
	s := newTestServer()
	s.EnqueueTask(&pb.Task{TaskId: "t-1"}, "test")
	s.EnqueueTask(&pb.Task{TaskId: "t-2"}, "test")

	removed := s.removeQueuedTaskByID("t-1")
	if !removed {
		t.Fatal("expected removeQueuedTaskByID to return true")
	}

	queued := s.GetQueuedTasks()
	if len(queued) != 1 {
		t.Fatalf("expected 1 remaining task, got %d", len(queued))
	}
	if queued[0].Task.TaskId != "t-2" {
		t.Fatalf("expected remaining task t-2, got %s", queued[0].Task.TaskId)
	}
}

func TestRemoveNonExistentTaskReturnsFalse(t *testing.T) {
	s := newTestServer()
	s.EnqueueTask(&pb.Task{TaskId: "t-1"}, "test")

	if s.removeQueuedTaskByID("t-999") {
		t.Fatal("expected false for non-existent task")
	}
}

func TestQueueContainsTask(t *testing.T) {
	s := newTestServer()
	s.EnqueueTask(&pb.Task{TaskId: "t-1"}, "test")

	if !s.queueContainsTask("t-1") {
		t.Fatal("expected queue to contain t-1")
	}
	if s.queueContainsTask("t-999") {
		t.Fatal("expected queue to not contain t-999")
	}
}

// ---------------------------------------------------------------------------
// Processing / cancellation request tests
// ---------------------------------------------------------------------------

func TestProcessingTaskTracking(t *testing.T) {
	s := newTestServer()

	s.queueMu.Lock()
	s.processingTasks["t-1"] = true
	s.queueMu.Unlock()

	if !s.isTaskBeingProcessed("t-1") {
		t.Fatal("expected t-1 to be tracked as processing")
	}
	if s.isTaskBeingProcessed("t-999") {
		t.Fatal("expected t-999 to not be processing")
	}
}

func TestCancellationRequestLifecycle(t *testing.T) {
	s := newTestServer()

	if s.isTaskCancellationRequested("t-1") {
		t.Fatal("expected no cancellation request initially")
	}

	// requestTaskCancellation requires the task to be in processingTasks first
	s.queueMu.Lock()
	s.processingTasks["t-1"] = true
	s.cancellationRequests["t-1"] = true
	s.queueMu.Unlock()

	if !s.isTaskCancellationRequested("t-1") {
		t.Fatal("expected cancellation request to be set")
	}

	s.clearTaskCancellationRequest("t-1")
	if s.isTaskCancellationRequested("t-1") {
		t.Fatal("expected cancellation request to be cleared")
	}
}

// ---------------------------------------------------------------------------
// Queue processor start/stop tests
// ---------------------------------------------------------------------------

func TestQueueProcessorStartStopIdempotent(t *testing.T) {
	s := newTestServer()
	s.StartQueueProcessor()

	done := make(chan struct{})
	go func() {
		s.StopQueueProcessor()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StopQueueProcessor blocked longer than 2s")
	}
}

// ---------------------------------------------------------------------------
// Worker selection tests (with scheduler)
// ---------------------------------------------------------------------------

func TestSelectWorkerForTaskUsesScheduler(t *testing.T) {
	rr := scheduler.NewRoundRobinScheduler()
	s := newTestServerWithScheduler(rr)
	addWorkerToServer(s, "w-1", "127.0.0.1:50052", 4, 8, 40)
	addWorkerToServer(s, "w-2", "127.0.0.1:50053", 4, 8, 40)

	task := &pb.Task{TaskId: "t-1", ReqCpu: 1, ReqMemory: 1, ReqStorage: 1}
	selected := s.selectWorkerForTask(task)
	if selected == "" {
		t.Fatal("expected scheduler to select a worker")
	}
}

func TestSelectWorkerReturnsEmptyWithNoWorkers(t *testing.T) {
	rr := scheduler.NewRoundRobinScheduler()
	s := newTestServerWithScheduler(rr)
	task := &pb.Task{TaskId: "t-1", ReqCpu: 1, ReqMemory: 1, ReqStorage: 1}
	selected := s.selectWorkerForTask(task)
	if selected != "" {
		t.Fatalf("expected empty with no workers, got %s", selected)
	}
}

// ---------------------------------------------------------------------------
// Worker state management tests
// ---------------------------------------------------------------------------

func TestWorkerInactiveAfterHeartbeatTimeout(t *testing.T) {
	s := newTestServer()
	addWorkerToServer(s, "w-1", "127.0.0.1:50052", 4, 8, 40)

	s.mu.Lock()
	s.workers["w-1"].LastHeartbeat = time.Now().Add(-5 * time.Minute).Unix()
	s.mu.Unlock()

	s.checkAndMarkInactiveWorkers()

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.workers["w-1"].IsActive {
		t.Fatal("expected worker to be marked inactive after heartbeat timeout")
	}
}

func TestWorkerRemainsActiveWithRecentHeartbeat(t *testing.T) {
	s := newTestServer()
	addWorkerToServer(s, "w-1", "127.0.0.1:50052", 4, 8, 40)

	s.checkAndMarkInactiveWorkers()

	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.workers["w-1"].IsActive {
		t.Fatal("expected worker to remain active with recent heartbeat")
	}
}

// ---------------------------------------------------------------------------
// Usage normalization tests
// ---------------------------------------------------------------------------

func TestNormalizeUsageFraction(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{0.5, 0.5},     // already fraction
		{50.0, 0.50},   // percentage → fraction
		{100.0, 1.0},   // full utilization
		{0.0, 0.0},     // zero
		{-1.0, 0.0},    // negative clamped to zero
	}

	for _, tt := range tests {
		got := normalizeUsageFraction(tt.input)
		if got != tt.want {
			t.Fatalf("normalizeUsageFraction(%f) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// buildProtoTaskFromDB tests
// ---------------------------------------------------------------------------

func TestBuildProtoTaskFromDB(t *testing.T) {
	dbTask := &db.Task{
		TaskID:      "t-1",
		DockerImage: "alpine:3.18",
		Command:     "echo hello",
		ReqCPU:      2.0,
		ReqMemory:   4.0,
		ReqStorage:  10.0,
		TaskType:    "cpu-heavy",
		TaskName:    "test-task",
		UserID:      "user-1",
	}

	protoTask := buildProtoTaskFromDB(dbTask)
	if protoTask.TaskId != "t-1" {
		t.Fatalf("expected TaskId=t-1, got %s", protoTask.TaskId)
	}
	if protoTask.DockerImage != "alpine:3.18" {
		t.Fatalf("expected DockerImage=alpine:3.18, got %s", protoTask.DockerImage)
	}
	if protoTask.ReqCpu != 2.0 {
		t.Fatalf("expected ReqCpu=2.0, got %f", protoTask.ReqCpu)
	}
	if protoTask.TaskType != "cpu-heavy" {
		t.Fatalf("expected TaskType=cpu-heavy, got %s", protoTask.TaskType)
	}
}

// ---------------------------------------------------------------------------
// Outcome reward computation tests
// ---------------------------------------------------------------------------

func TestComputeOutcomeReward(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		runtime float64
		sla     bool
		wantPos bool // expect positive reward
	}{
		{"success with SLA", "success", 10.0, true, true},
		{"success without SLA short runtime", "success", 100.0, false, true},
		{"failed", "failed", 0.0, false, false},
		{"cancelled", "cancelled", 0.0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reward := computeOutcomeReward(tt.status, tt.runtime, tt.sla)
			if tt.wantPos && reward <= 0 {
				t.Fatalf("expected positive reward for %s, got %f", tt.name, reward)
			}
			if !tt.wantPos && reward > 0 {
				t.Fatalf("expected non-positive reward for %s, got %f", tt.name, reward)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Cluster snapshot tests
// ---------------------------------------------------------------------------

func TestGetClusterSnapshot(t *testing.T) {
	s := newTestServer()
	addWorkerToServer(s, "w-1", "127.0.0.1:50052", 4, 8, 40)
	addWorkerToServer(s, "w-2", "127.0.0.1:50053", 2, 4, 20)
	s.EnqueueTask(&pb.Task{TaskId: "t-1"}, "test")

	snap := s.GetClusterSnapshot()
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snap.Workers) != 2 {
		t.Fatalf("expected 2 workers in snapshot, got %d", len(snap.Workers))
	}
}
