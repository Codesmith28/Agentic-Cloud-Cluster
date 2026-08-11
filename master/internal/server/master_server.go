// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"master/internal/db"
	mastermetrics "master/internal/metrics"
	"master/internal/scheduler"
	"master/internal/storage"
	"master/internal/telemetry"
	pb "master/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// QueuedTask represents a task waiting to be scheduled and assigned
type QueuedTask struct {
	Task      *pb.Task
	QueuedAt  time.Time
	Retries   int
	LastError string
}

// TaskSubmission represents the result of submitting a task to the system
type TaskSubmission struct {
	TaskID   string
	Queued   bool
	Position int
	Message  string
}

// MasterServer handles gRPC requests from workers
type MasterServer struct {
	pb.UnimplementedMasterWorkerServer

	workers        map[string]*WorkerState
	mu             sync.RWMutex
	workerDB       *db.WorkerDB
	taskDB         *db.TaskDB
	assignmentDB   *db.AssignmentDB
	attemptDB      *db.AttemptDB
	resultDB       *db.ResultDB
	fileMetadataDB *db.FileMetadataDB
	fileStorage    *storage.FileStorageService
	masterID       string
	masterAddress  string

	taskChan chan *TaskAssignment

	// Task queue for tasks waiting for resources
	taskQueue            []*QueuedTask
	processingTasks      map[string]bool
	cancellationRequests map[string]bool
	queueMu              sync.RWMutex
	queueTicker          *time.Ticker
	queueStop            chan struct{}
	queueWG              sync.WaitGroup

	// In-memory resource cache: taskID -> resource requirements.
	// Used to free resources on task completion when taskDB is unavailable.
	taskResourceCache   map[string]*db.Task
	taskResourceCacheMu sync.Mutex

	// Task scheduler
	scheduler scheduler.Scheduler

	// Telemetry manager for handling worker telemetry in separate threads
	telemetryManager *telemetry.TelemetryManager

	// Worker reconnection
	reconnectTicker *time.Ticker
	reconnectStop   chan bool
}

// WorkerState tracks the current state of a worker
type WorkerState struct {
	Info          *pb.WorkerInfo
	LastHeartbeat int64
	IsActive      bool
	RunningTasks  map[string]bool
	LatestCPU     float64 // Latest CPU usage from heartbeat
	LatestMemory  float64 // Latest memory usage from heartbeat
	LatestStorage float64 // Latest storage usage from heartbeat
	TaskCount     int     // Number of running tasks from latest heartbeat
	// Resource tracking
	AllocatedCPU     float64
	AllocatedMemory  float64
	AllocatedStorage float64
	AvailableCPU     float64
	AvailableMemory  float64
	AvailableStorage float64
}

// TaskAssignment represents a task to be sent to a worker
type TaskAssignment struct {
	Task     *pb.Task
	WorkerID string
}

// NewMasterServer creates a new master server instance
func NewMasterServer(workerDB *db.WorkerDB, taskDB *db.TaskDB, assignmentDB *db.AssignmentDB, attemptDB *db.AttemptDB, resultDB *db.ResultDB, fileMetadataDB *db.FileMetadataDB, fileStorage *storage.FileStorageService, telemetryMgr *telemetry.TelemetryManager) *MasterServer {
	return &MasterServer{
		workers:              make(map[string]*WorkerState),
		workerDB:             workerDB,
		taskDB:               taskDB,
		assignmentDB:         assignmentDB,
		attemptDB:            attemptDB,
		resultDB:             resultDB,
		fileMetadataDB:       fileMetadataDB,
		fileStorage:          fileStorage,
		masterID:             "",
		masterAddress:        "",
		taskChan:             make(chan *TaskAssignment, 100),
		taskQueue:            make([]*QueuedTask, 0),
		processingTasks:      make(map[string]bool),
		cancellationRequests: make(map[string]bool),
		taskResourceCache:    make(map[string]*db.Task),
		scheduler:            scheduler.NewRoundRobinScheduler(), // Use Round-Robin as default
		telemetryManager:     telemetryMgr,
	}
}

// SetMasterInfo sets the master ID and address
func (s *MasterServer) SetMasterInfo(masterID, masterAddress string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.masterID = masterID
	s.masterAddress = masterAddress
	log.Printf("Master info set: ID=%s, Address=%s", masterID, masterAddress)
}

// GetMasterInfo returns the master ID and address
func (s *MasterServer) GetMasterInfo() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.masterID, s.masterAddress
}

// SetScheduler sets the task scheduler
func (s *MasterServer) SetScheduler(sched scheduler.Scheduler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduler = sched
	log.Printf("Scheduler set: %s", sched.GetName())
}

// GetSchedulerName returns the currently configured scheduler name.
func (s *MasterServer) GetSchedulerName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.scheduler == nil {
		return ""
	}
	return s.scheduler.GetName()
}

// LoadWorkersFromDB loads registered workers from database into memory
func (s *MasterServer) LoadWorkersFromDB(ctx context.Context) error {
	if s.workerDB == nil {
		return nil // DB not configured, skip
	}

	workers, err := s.workerDB.GetAllWorkers(ctx)
	if err != nil {
		return fmt.Errorf("load workers from db: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, w := range workers {
		s.workers[w.WorkerID] = &WorkerState{
			Info: &pb.WorkerInfo{
				WorkerId:     w.WorkerID,
				WorkerIp:     w.WorkerIP,
				TotalCpu:     w.TotalCPU,
				TotalMemory:  w.TotalMemory,
				TotalStorage: w.TotalStorage,
			},
			LastHeartbeat:    w.LastHeartbeat,
			IsActive:         w.IsActive,
			RunningTasks:     make(map[string]bool),
			AllocatedCPU:     w.AllocatedCPU,
			AllocatedMemory:  w.AllocatedMemory,
			AllocatedStorage: w.AllocatedStorage,
			AvailableCPU:     w.AvailableCPU,
			AvailableMemory:  w.AvailableMemory,
			AvailableStorage: w.AvailableStorage,
		}
	}

	// Reconcile resources based on actual running tasks
	s.ReconcileWorkerResources(ctx)

	log.Printf("Loaded %d workers from database", len(workers))
	return nil
}

// ManualRegisterWorker manually registers a worker (called from CLI)
func (s *MasterServer) ManualRegisterWorker(ctx context.Context, workerID, workerIP string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If worker already exists, treat this as an address refresh/update.
	if existing, exists := s.workers[workerID]; exists {
		oldAddress := ""
		if existing.Info == nil {
			existing.Info = &pb.WorkerInfo{WorkerId: workerID}
		} else {
			oldAddress = existing.Info.WorkerIp
		}

		existing.Info.WorkerId = workerID
		existing.Info.WorkerIp = workerIP
		existing.IsActive = false // Will become active again after registration + heartbeat.
		existing.LastHeartbeat = 0

		if s.workerDB != nil {
			if err := s.workerDB.UpdateWorkerAddress(ctx, workerID, workerIP); err != nil {
				return fmt.Errorf("update worker address in db: %w", err)
			}
		}

		log.Printf("Updated worker registration: %s (Address: %s -> %s)", workerID, oldAddress, workerIP)
		return nil
	}

	// Add to database
	if s.workerDB != nil {
		exists, err := s.workerDB.WorkerExists(ctx, workerID)
		if err != nil {
			return fmt.Errorf("check worker existence: %w", err)
		}
		if exists {
			if err := s.workerDB.UpdateWorkerAddress(ctx, workerID, workerIP); err != nil {
				return fmt.Errorf("update worker address in db: %w", err)
			}
			log.Printf("Updated existing worker in DB: %s (Address: %s)", workerID, workerIP)
		} else {
			if err := s.workerDB.RegisterWorker(ctx, workerID, workerIP); err != nil {
				return fmt.Errorf("register worker in db: %w", err)
			}
		}
	}

	// Add to memory with minimal info
	s.workers[workerID] = &WorkerState{
		Info: &pb.WorkerInfo{
			WorkerId: workerID,
			WorkerIp: workerIP, // Format: "ip:port"
			// Resource info will be filled when worker connects
		},
		IsActive:     false, // Not active until worker connects
		RunningTasks: make(map[string]bool),
		// Initialize resource tracking to 0
		AllocatedCPU:     0.0,
		AllocatedMemory:  0.0,
		AllocatedStorage: 0.0,
		AvailableCPU:     0.0,
		AvailableMemory:  0.0,
		AvailableStorage: 0.0,
	}

	log.Printf("Manually registered worker: %s (Address: %s)", workerID, workerIP)
	return nil
}

// UpdateWorkerResourcesInMemory updates worker resources in memory (called from HTTP API after manual registration)
func (s *MasterServer) UpdateWorkerResourcesInMemory(workerID string, totalCPU, totalMemory, totalStorage float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, exists := s.workers[workerID]
	if !exists {
		log.Printf("Warning: Cannot update resources for non-existent worker: %s", workerID)
		return
	}

	// Update the worker's info with the provided resources
	worker.Info.TotalCpu = totalCPU
	worker.Info.TotalMemory = totalMemory
	worker.Info.TotalStorage = totalStorage

	// Calculate available resources (total - allocated)
	worker.AvailableCPU = totalCPU - worker.AllocatedCPU
	worker.AvailableMemory = totalMemory - worker.AllocatedMemory
	worker.AvailableStorage = totalStorage - worker.AllocatedStorage

	// Mark worker as active since it has been configured
	worker.IsActive = true

	log.Printf("Updated worker %s resources: CPU=%.2f, Memory=%.2f, Storage=%.2f",
		workerID, totalCPU, totalMemory, totalStorage)
}

// ReconcileWorkerResources reconciles allocated resources based on actual running tasks
// This fixes stale resource allocations from completed tasks
// Should be called: 1) On startup after loading workers, 2) Periodically, 3) After crashes
func (s *MasterServer) ReconcileWorkerResources(ctx context.Context) error {
	// This function assumes s.mu is already locked by the caller

	if s.taskDB == nil || s.assignmentDB == nil {
		log.Printf("⚠ Resource reconciliation skipped: databases not available")
		return nil
	}

	log.Printf("🔄 Starting resource reconciliation...")

	// Get all running tasks from database
	tasks, err := s.taskDB.GetTasksByStatus(ctx, "running")
	if err != nil {
		log.Printf("⚠ Failed to get running tasks for reconciliation: %v", err)
		return err
	}

	// Build map of actual allocations per worker
	actualAllocations := make(map[string]struct {
		CPU, Memory, Storage float64
		TaskIDs              map[string]bool
	})

	for _, task := range tasks {
		// Get assignment to find which worker
		assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, task.TaskID)
		if err != nil {
			log.Printf("⚠ Task %s has no assignment, skipping", task.TaskID)
			continue
		}

		workerID := assignment.WorkerID
		if _, exists := actualAllocations[workerID]; !exists {
			actualAllocations[workerID] = struct {
				CPU, Memory, Storage float64
				TaskIDs              map[string]bool
			}{TaskIDs: make(map[string]bool)}
		}

		alloc := actualAllocations[workerID]
		alloc.CPU += task.ReqCPU
		alloc.Memory += task.ReqMemory
		alloc.Storage += task.ReqStorage
		alloc.TaskIDs[task.TaskID] = true
		actualAllocations[workerID] = alloc
	}

	// Now reconcile each worker
	fixedCount := 0
	for workerID, worker := range s.workers {
		actual := actualAllocations[workerID]

		// Check if resources are out of sync
		if worker.AllocatedCPU != actual.CPU ||
			worker.AllocatedMemory != actual.Memory ||
			worker.AllocatedStorage != actual.Storage {

			oldCPU := worker.AllocatedCPU
			oldMem := worker.AllocatedMemory
			oldStorage := worker.AllocatedStorage

			// Fix the allocations
			worker.AllocatedCPU = actual.CPU
			worker.AllocatedMemory = actual.Memory
			worker.AllocatedStorage = actual.Storage

			// Recalculate available resources
			worker.AvailableCPU = worker.Info.TotalCpu - actual.CPU
			worker.AvailableMemory = worker.Info.TotalMemory - actual.Memory
			worker.AvailableStorage = worker.Info.TotalStorage - actual.Storage

			// Update running tasks map
			worker.RunningTasks = actual.TaskIDs

			// Update in database
			// First release all old allocations, then allocate the correct amount
			if s.workerDB != nil && (oldCPU > 0 || oldMem > 0 || oldStorage > 0) {
				if err := s.workerDB.ReleaseResources(ctx, workerID,
					oldCPU, oldMem, oldStorage); err != nil {
					log.Printf("⚠ Failed to release old resources for %s in DB: %v", workerID, err)
				}
			}

			// Now allocate the correct amount
			if s.workerDB != nil && actual.CPU > 0 {
				if err := s.workerDB.AllocateResources(ctx, workerID,
					actual.CPU, actual.Memory, actual.Storage); err != nil {
					log.Printf("⚠ Failed to allocate resources for %s in DB: %v", workerID, err)
				}
			}

			log.Printf("  ✓ Fixed %s: CPU %.1f→%.1f, Memory %.1f→%.1f, Tasks: %d",
				workerID, oldCPU, actual.CPU, oldMem, actual.Memory, len(actual.TaskIDs))
			fixedCount++
		}
	}

	if fixedCount > 0 {
		log.Printf("✓ Resource reconciliation complete: fixed %d workers", fixedCount)
	} else {
		log.Printf("✓ Resource reconciliation complete: all workers correct")
	}

	return nil
}

// ReconcileWorkerResourcesPublic is a public wrapper that acquires the lock
func (s *MasterServer) ReconcileWorkerResourcesPublic(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ReconcileWorkerResources(ctx)
}

// reconcileSingleWorker reconciles resources for a specific worker based on actual running tasks
// This function assumes s.mu is already locked by the caller
func (s *MasterServer) reconcileSingleWorker(ctx context.Context, workerID string, worker *WorkerState) {
	if s.taskDB == nil || s.assignmentDB == nil {
		log.Printf("⚠ Resource reconciliation skipped for %s: databases not available", workerID)
		return
	}

	// Get all running tasks assigned to this worker
	tasks, err := s.taskDB.GetTasksByStatus(ctx, "running")
	if err != nil {
		log.Printf("⚠ Failed to get running tasks for reconciliation: %v", err)
		return
	}

	log.Printf("  🔍 Reconciliation: Found %d tasks with 'running' status in database", len(tasks))

	// Calculate actual resource usage from running tasks
	var actualCPU, actualMemory, actualStorage float64
	actualTaskIDs := make(map[string]bool)

	for _, task := range tasks {
		// Get assignment to find which worker
		assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, task.TaskID)
		if err != nil {
			log.Printf("  ⚠ Task %s has no assignment, skipping", task.TaskID)
			continue
		}

		if assignment.WorkerID == workerID {
			log.Printf("  📋 Found task %s assigned to %s (CPU=%.1f, Mem=%.1f, Storage=%.1f)",
				task.TaskID, workerID, task.ReqCPU, task.ReqMemory, task.ReqStorage)
			actualCPU += task.ReqCPU
			actualMemory += task.ReqMemory
			actualStorage += task.ReqStorage
			actualTaskIDs[task.TaskID] = true
		}
	}

	// Update worker's allocated resources
	worker.AllocatedCPU = actualCPU
	worker.AllocatedMemory = actualMemory
	worker.AllocatedStorage = actualStorage

	// Recalculate available resources
	worker.AvailableCPU = worker.Info.TotalCpu - actualCPU
	worker.AvailableMemory = worker.Info.TotalMemory - actualMemory
	worker.AvailableStorage = worker.Info.TotalStorage - actualStorage

	// Update running tasks map
	worker.RunningTasks = actualTaskIDs

	// Update database with correct allocations
	if s.workerDB != nil {
		if err := s.workerDB.SetWorkerResources(ctx, workerID,
			actualCPU, actualMemory, actualStorage,
			worker.AvailableCPU, worker.AvailableMemory, worker.AvailableStorage); err != nil {
			log.Printf("⚠ Failed to update resources for %s in DB: %v", workerID, err)
		}
	}

	log.Printf("  ✓ Reconciled %s: CPU=%.1f, Memory=%.1f, Storage=%.1f, Tasks=%d",
		workerID, actualCPU, actualMemory, actualStorage, len(actualTaskIDs))
}

// ManualRegisterAndNotify registers a worker and immediately tries to notify it of the master's address
func (s *MasterServer) ManualRegisterAndNotify(ctx context.Context, workerID, workerIP, masterID, masterAddress string) error {
	if err := s.ManualRegisterWorker(ctx, workerID, workerIP); err != nil {
		return err
	}

	// Attempt to contact worker and send MasterRegister
	go func() {
		cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(cctx, workerIP, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		if err != nil {
			log.Printf("Failed to connect to worker %s (%s) for MasterRegister: %v", workerID, workerIP, err)
			return
		}
		defer conn.Close()

		client := pb.NewMasterWorkerClient(conn)
		mi := &pb.MasterInfo{MasterId: masterID, MasterAddress: masterAddress}
		ack, err := client.MasterRegister(cctx, mi)
		if err != nil {
			log.Printf("MasterRegister RPC to worker %s (%s) failed: %v", workerID, workerIP, err)
			return
		}
		if ack != nil && !ack.Success {
			log.Printf("MasterRegister rejected by worker %s: %s", workerID, ack.Message)
		}
		// Success case: no log to keep CLI clean
	}()

	return nil
}

// StartWorkerReconnectionMonitor starts a background process that periodically attempts
// to reconnect to inactive workers
func (s *MasterServer) StartWorkerReconnectionMonitor() {
	s.reconnectTicker = time.NewTicker(5 * time.Second) // Check every 5 seconds
	s.reconnectStop = make(chan bool)

	go func() {
		log.Println("🔄 Worker reconnection monitor started")
		for {
			select {
			case <-s.reconnectTicker.C:
				s.checkAndMarkInactiveWorkers() // Check for inactive workers first
				s.attemptWorkerReconnections()
			case <-s.reconnectStop:
				log.Println("🛑 Worker reconnection monitor stopped")
				return
			}
		}
	}()
}

// StopWorkerReconnectionMonitor stops the reconnection monitor
func (s *MasterServer) StopWorkerReconnectionMonitor() {
	if s.reconnectTicker != nil {
		s.reconnectTicker.Stop()
	}
	if s.reconnectStop != nil {
		close(s.reconnectStop)
	}
}

// checkAndMarkInactiveWorkers marks workers as inactive if they haven't sent heartbeat in 30 seconds
func (s *MasterServer) checkAndMarkInactiveWorkers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	const heartbeatTimeout = 30 // 30 seconds timeout

	for workerID, worker := range s.workers {
		if worker.IsActive && worker.LastHeartbeat > 0 {
			timeSinceLastHeartbeat := now - worker.LastHeartbeat
			if timeSinceLastHeartbeat > heartbeatTimeout {
				log.Printf("⚠️ Worker %s marked as inactive (no heartbeat for %d seconds)", workerID, timeSinceLastHeartbeat)
				worker.IsActive = false
				mastermetrics.Get().IncWorkerTimeout(workerID)
				s.recoverWorkerTasksLocked(context.Background(), workerID, worker, "worker_timeout")
			}
		}
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

// attemptWorkerReconnections tries to reconnect to all inactive workers
func (s *MasterServer) attemptWorkerReconnections() {
	s.mu.RLock()
	masterID := s.masterID
	masterAddress := s.masterAddress

	// Collect inactive workers
	inactiveWorkers := make(map[string]string) // workerID -> workerIP
	for workerID, worker := range s.workers {
		if !worker.IsActive && worker.Info != nil && worker.Info.WorkerIp != "" {
			inactiveWorkers[workerID] = worker.Info.WorkerIp
		}
	}
	s.mu.RUnlock()

	// If there are inactive workers, attempt to reconnect
	if len(inactiveWorkers) > 0 {
		log.Printf("🔄 Attempting to reconnect to %d inactive worker(s)...", len(inactiveWorkers))

		for workerID, workerIP := range inactiveWorkers {
			// Launch reconnection attempt in goroutine (non-blocking)
			go s.attemptSingleWorkerReconnection(workerID, workerIP, masterID, masterAddress)
		}
	}
}

// attemptSingleWorkerReconnection attempts to reconnect to a single worker
func (s *MasterServer) attemptSingleWorkerReconnection(workerID, workerIP, masterID, masterAddress string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, workerIP,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		// Worker still offline, silently skip (don't spam logs)
		return
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	mi := &pb.MasterInfo{MasterId: masterID, MasterAddress: masterAddress}
	ack, err := client.MasterRegister(ctx, mi)
	if err != nil {
		// Failed to register, worker may not be fully ready yet
		return
	}

	if ack != nil && ack.Success {
		log.Printf("✓ Successfully reconnected to worker %s (%s)", workerID, workerIP)
	}
}

// UnregisterWorker removes a worker from the system
func (s *MasterServer) UnregisterWorker(ctx context.Context, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if exists
	if _, exists := s.workers[workerID]; !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	// Remove from database
	if s.workerDB != nil {
		if err := s.workerDB.UnregisterWorker(ctx, workerID); err != nil {
			return fmt.Errorf("unregister worker from db: %w", err)
		}
	}

	// Unregister from telemetry manager
	if s.telemetryManager != nil {
		s.telemetryManager.UnregisterWorker(workerID)
	}

	// Remove from memory
	delete(s.workers, workerID)

	log.Printf("Unregistered worker: %s", workerID)
	return nil
}

// RegisterWorker handles worker registration requests
// Workers can ONLY register if they have been manually pre-registered by admin
func (s *MasterServer) RegisterWorker(ctx context.Context, info *pb.WorkerInfo) (*pb.RegisterAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if worker was manually pre-registered by admin
	existingWorker, exists := s.workers[info.WorkerId]
	if !exists {
		// Worker NOT pre-registered - reject the connection
		log.Printf("❌ Rejected unauthorized worker registration attempt: %s (Address: %s)",
			info.WorkerId, info.WorkerIp)
		return &pb.RegisterAck{
			Success: false,
			Message: fmt.Sprintf("Worker %s is not authorized. Admin must register it first using: register %s <ip:port>",
				info.WorkerId, info.WorkerId),
		}, fmt.Errorf("worker %s not authorized - must be pre-registered by admin", info.WorkerId)
	}

	// Check if this is a new registration (worker connecting for the first time or reconnecting with new specs)
	isNewConnection := existingWorker.Info.TotalCpu == 0 || !existingWorker.IsActive

	// Ensure RunningTasks map is initialized (defensive programming)
	if existingWorker.RunningTasks == nil {
		existingWorker.RunningTasks = make(map[string]bool)
	}

	// Worker IS pre-registered - update with full specs but preserve the admin-configured endpoint.
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

	// If this is a new connection or reconnection, reconcile resources for this worker
	// to ensure allocated resources match actual running tasks
	if isNewConnection {
		log.Printf("🔄 Worker %s connected with new specs, reconciling resources...", info.WorkerId)

		// Initialize allocated resources to 0 first, reconciliation will fix them
		existingWorker.AllocatedCPU = 0.0
		existingWorker.AllocatedMemory = 0.0
		existingWorker.AllocatedStorage = 0.0

		// Initialize available resources to total
		existingWorker.AvailableCPU = info.TotalCpu
		existingWorker.AvailableMemory = info.TotalMemory
		existingWorker.AvailableStorage = info.TotalStorage

		// Trigger reconciliation for this specific worker to fix resources based on actual running tasks
		s.reconcileSingleWorker(ctx, info.WorkerId, existingWorker)
	} else {
		// Worker is already connected with same specs, just update available resources
		existingWorker.AvailableCPU = info.TotalCpu - existingWorker.AllocatedCPU
		existingWorker.AvailableMemory = info.TotalMemory - existingWorker.AllocatedMemory
		existingWorker.AvailableStorage = info.TotalStorage - existingWorker.AllocatedStorage
	}

	// Update in database
	if s.workerDB != nil {
		if err := s.workerDB.UpdateWorkerInfo(ctx, existingWorker.Info); err != nil {
			log.Printf("Warning: failed to update worker in db: %v", err)
		}
	}

	// Register worker with telemetry manager
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

	// Store latest heartbeat metrics (keep minimal data in main thread)
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

	// Update heartbeat in database
	if s.workerDB != nil {
		if err := s.workerDB.UpdateHeartbeat(ctx, hb.WorkerId, timestamp); err != nil {
			log.Printf("Warning: failed to update heartbeat in db: %v", err)
		}
	}

	// Offload telemetry processing to dedicated thread
	// This is non-blocking and won't slow down the RPC handler
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

	// Get task info to retrieve resource requirements.
	// Falls back to in-memory cache when taskDB is unavailable.
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
		delete(s.taskResourceCache, result.TaskId) // evict on first use
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

	// Remove task from worker's running tasks and release resources
	if worker, exists := s.workers[result.WorkerId]; exists {
		if worker.RunningTasks != nil {
			delete(worker.RunningTasks, result.TaskId)
		}

		// 🚨 RELEASE RESOURCES - Update both in-memory and database
		if taskResources != nil {
			worker.AllocatedCPU -= taskResources.ReqCPU
			worker.AllocatedMemory -= taskResources.ReqMemory
			worker.AllocatedStorage -= taskResources.ReqStorage
			worker.AvailableCPU += taskResources.ReqCPU
			worker.AvailableMemory += taskResources.ReqMemory
			worker.AvailableStorage += taskResources.ReqStorage

			// Ensure non-negative values (safety check)
			if worker.AllocatedCPU < 0 {
				worker.AllocatedCPU = 0
			}
			if worker.AllocatedMemory < 0 {
				worker.AllocatedMemory = 0
			}
			if worker.AllocatedStorage < 0 {
				worker.AllocatedStorage = 0
			}

			// Update database
			if s.workerDB != nil {
				if err := s.workerDB.ReleaseResources(ctx, result.WorkerId,
					taskResources.ReqCPU, taskResources.ReqMemory,
					taskResources.ReqStorage); err != nil {
					log.Printf("  ⚠ Warning: Failed to release resources in database: %v", err)
				} else {
					log.Printf("  ✓ Released resources: CPU=%.2f, Memory=%.2f, Storage=%.2f",
						taskResources.ReqCPU, taskResources.ReqMemory, taskResources.ReqStorage)
				}
			}
		}
	}
	if s.assignmentDB != nil {
		if err := s.assignmentDB.DeleteAssignment(ctx, result.TaskId); err != nil && !strings.Contains(err.Error(), "not found") {
			log.Printf("  ⚠ Warning: Failed to delete assignment for completed task: %v", err)
		}
	}

	// Update task status in database (idempotent - safe if already updated)
	// For cancelled tasks, master already updated this during CancelTask
	// This provides redundancy and updates timestamp
	if s.taskDB != nil {
		// Check if task is already cancelled - do not overwrite cancelled status
		existingTask, err := s.taskDB.GetTask(context.Background(), result.TaskId)
		if err != nil {
			log.Printf("  ⚠ Warning: Failed to get task status from database: %v", err)
		} else if existingTask != nil && existingTask.Status == "cancelled" {
			log.Printf("  ℹ Task %s is already cancelled - preserving status", result.TaskId)
			// Check if result already exists - don't store duplicate
			if s.resultDB != nil {
				existingResult, err := s.resultDB.GetResult(context.Background(), result.TaskId)
				if err == nil && existingResult != nil {
					log.Printf("  ℹ Result already stored for cancelled task - ignoring worker's confirmation report")
					s.reportSchedulingOutcomeAsync(taskResources, result)
					return &pb.Ack{
						Success: true,
						Message: "Task result received (status preserved as cancelled, result already stored)",
					}, nil
				}
				// No existing result, store this one (first report with actual logs)
				log.Printf("  ℹ Storing first result for cancelled task")
				taskResult := &db.TaskResult{
					TaskID:   result.TaskId,
					WorkerID: result.WorkerId,
					Status:   "cancelled",
					Logs:     result.Logs,
				}
				if err := s.resultDB.CreateResult(context.Background(), taskResult); err != nil {
					log.Printf("  ⚠ Warning: Failed to store task result: %v", err)
				} else {
					log.Printf("  ✓ Task result stored with 'cancelled' status")
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
			log.Printf("  ℹ Confirming task %s 'cancelled' status (already set by master)", result.TaskId)
		} else if result.Status != "success" {
			status = "failed"
			attemptStatus = db.AttemptStatusFailed
			failureReason = "container_failed"
		}

		// Idempotent update - safe to call even if already cancelled
		if err := s.taskDB.UpdateTaskStatus(context.Background(), result.TaskId, status); err != nil {
			log.Printf("  ⚠ Warning: Failed to update task status in database: %v", err)
			// For cancelled tasks this is not critical since master already updated
			if result.Status != "cancelled" {
				return &pb.Ack{
					Success: false,
					Message: fmt.Sprintf("Failed to update task status: %v", err),
				}, nil
			}
		} else {
			log.Printf("  ✓ Task status confirmed as '%s' in database", status)
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

	// Store result with logs in RESULTS collection
	if s.resultDB != nil {
		taskResult := &db.TaskResult{
			TaskID:   result.TaskId,
			WorkerID: result.WorkerId,
			Status:   result.Status,
			Logs:     result.Logs,
		}
		if err := s.resultDB.CreateResult(context.Background(), taskResult); err != nil {
			log.Printf("  ⚠ Warning: Failed to store task result: %v", err)
			// Don't fail here - status update is more critical
		} else {
			log.Printf("  ✓ Task result stored in RESULTS collection")
		}
	}

	s.reportSchedulingOutcomeAsync(taskResources, result)

	return &pb.Ack{
		Success: true,
		Message: "Task result received and processed",
	}, nil
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

// UploadTaskFiles handles file uploads from workers via streaming RPC
func (s *MasterServer) UploadTaskFiles(stream pb.MasterWorker_UploadTaskFilesServer) error {
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("  📤 FILE UPLOAD REQUEST")
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if s.fileStorage == nil {
		log.Printf("  ✗ File storage service not initialized")
		return stream.SendAndClose(&pb.FileUploadAck{
			Success:       false,
			Message:       "File storage service not available",
			FilesReceived: 0,
		})
	}

	// Receive file stream and store files
	metadata, err := s.fileStorage.ReceiveFileStream(stream)
	if err != nil {
		log.Printf("  ✗ Failed to receive files: %v", err)
		return stream.SendAndClose(&pb.FileUploadAck{
			Success:       false,
			Message:       fmt.Sprintf("Failed to receive files: %v", err),
			FilesReceived: 0,
		})
	}

	// Store metadata in database
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
		} else {
			log.Printf("  ✓ File metadata stored in database")
		}
	}

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("  ✓ FILE UPLOAD COMPLETE")
	log.Printf("  Task: %s | User: %s | Files: %d", metadata.TaskID, metadata.UserID, len(metadata.FilePaths))
	log.Printf("  Storage Path: %s", metadata.StoragePath)
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return stream.SendAndClose(&pb.FileUploadAck{
		Success:       true,
		Message:       "Files uploaded successfully",
		FilesReceived: int32(len(metadata.FilePaths)),
	})
}

// GetWorkers returns current worker states (for CLI)
func (s *MasterServer) GetWorkers() map[string]*WorkerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make(map[string]*WorkerState)
	now := time.Now().Unix()

	for k, v := range s.workers {
		// Create a copy to avoid modifying the original
		workerCopy := *v

		// Check if worker is truly active based on heartbeat timeout (30 seconds)
		if workerCopy.LastHeartbeat > 0 {
			timeSinceLastHeartbeat := now - workerCopy.LastHeartbeat
			if timeSinceLastHeartbeat > 30 {
				workerCopy.IsActive = false
			}
		}

		workers[k] = &workerCopy
	}
	return workers
}

// GetWorkerStats returns detailed stats for a specific worker
func (s *MasterServer) GetWorkerStats(workerID string) (*WorkerState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	worker, exists := s.workers[workerID]
	return worker, exists
}

// GetWorkerTelemetry returns detailed telemetry data for a specific worker
// This queries the telemetry manager's dedicated thread for the worker
func (s *MasterServer) GetWorkerTelemetry(workerID string) (*telemetry.WorkerTelemetryData, bool) {
	if s.telemetryManager == nil {
		return nil, false
	}
	return s.telemetryManager.GetWorkerTelemetry(workerID)
}

// GetAllWorkerTelemetry returns telemetry data for all workers
func (s *MasterServer) GetAllWorkerTelemetry() map[string]*telemetry.WorkerTelemetryData {
	if s.telemetryManager == nil {
		return make(map[string]*telemetry.WorkerTelemetryData)
	}
	return s.telemetryManager.GetAllWorkerTelemetry()
}

// WorkerStateSnapshot represents a point-in-time snapshot of a worker's state
type WorkerStateSnapshot struct {
	WorkerID         string
	WorkerIP         string
	Status           string // "active" or "inactive"
	LastHeartbeat    int64
	HeartbeatAgo     string // Human-readable: "5s ago", "2m ago"
	CPUUsage         float64
	MemoryUsage      float64
	StorageUsage     float64
	TotalCPU         float64
	TotalMemory      float64
	TotalStorage     float64
	AllocatedCPU     float64
	AllocatedMemory  float64
	AllocatedStorage float64
	AvailableCPU     float64
	AvailableMemory  float64
	AvailableStorage float64
	RunningTasks     []string
	TaskCount        int
}

// ClusterSnapshot represents a point-in-time snapshot of the entire cluster
type ClusterSnapshot struct {
	Timestamp          time.Time
	Workers            []WorkerStateSnapshot
	TotalWorkers       int
	ActiveWorkers      int
	InactiveWorkers    int
	TotalTasks         int
	TotalCPU           float64
	AllocatedCPU       float64
	AvailableCPU       float64
	CPUUtilization     float64 // Percentage
	TotalMemory        float64
	AllocatedMemory    float64
	AvailableMemory    float64
	MemoryUtilization  float64 // Percentage
	TotalStorage       float64
	AllocatedStorage   float64
	AvailableStorage   float64
	StorageUtilization float64 // Percentage
}

// GetClusterSnapshot returns a structured snapshot of the cluster state
func (s *MasterServer) GetClusterSnapshot() *ClusterSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := &ClusterSnapshot{
		Timestamp: time.Now(),
		Workers:   []WorkerStateSnapshot{},
	}

	for workerID, worker := range s.workers {
		// Calculate heartbeat ago
		heartbeatAgo := "never"
		if worker.LastHeartbeat > 0 {
			duration := time.Since(time.Unix(worker.LastHeartbeat, 0))
			if duration < 60*time.Second {
				heartbeatAgo = fmt.Sprintf("%ds ago", int(duration.Seconds()))
			} else if duration < 60*time.Minute {
				heartbeatAgo = fmt.Sprintf("%dm ago", int(duration.Minutes()))
			} else {
				heartbeatAgo = fmt.Sprintf("%dh ago", int(duration.Hours()))
			}
		}

		// Status
		status := "active"
		if !worker.IsActive {
			status = "inactive"
		}

		// Extract running tasks
		runningTasks := []string{}
		if worker.RunningTasks != nil {
			for taskID := range worker.RunningTasks {
				runningTasks = append(runningTasks, taskID)
			}
		}

		// Get resource totals
		var totalCPU, totalMemory, totalStorage float64
		var workerIP string
		if worker.Info != nil {
			totalCPU = worker.Info.TotalCpu
			totalMemory = worker.Info.TotalMemory
			totalStorage = worker.Info.TotalStorage
			workerIP = worker.Info.WorkerIp
		}

		workerSnapshot := WorkerStateSnapshot{
			WorkerID:         workerID,
			WorkerIP:         workerIP,
			Status:           status,
			LastHeartbeat:    worker.LastHeartbeat,
			HeartbeatAgo:     heartbeatAgo,
			CPUUsage:         worker.LatestCPU * 100.0,
			MemoryUsage:      worker.LatestMemory * 100.0,
			StorageUsage:     worker.LatestStorage * 100.0,
			TotalCPU:         totalCPU,
			TotalMemory:      totalMemory,
			TotalStorage:     totalStorage,
			AllocatedCPU:     worker.AllocatedCPU,
			AllocatedMemory:  worker.AllocatedMemory,
			AllocatedStorage: worker.AllocatedStorage,
			AvailableCPU:     worker.AvailableCPU,
			AvailableMemory:  worker.AvailableMemory,
			AvailableStorage: worker.AvailableStorage,
			RunningTasks:     runningTasks,
			TaskCount:        len(runningTasks),
		}

		snapshot.Workers = append(snapshot.Workers, workerSnapshot)

		// Aggregate cluster stats
		snapshot.TotalWorkers++
		if worker.IsActive {
			snapshot.ActiveWorkers++
		}
		if worker.RunningTasks != nil {
			snapshot.TotalTasks += len(worker.RunningTasks)
		}
		snapshot.TotalCPU += totalCPU
		snapshot.TotalMemory += totalMemory
		snapshot.TotalStorage += totalStorage
		snapshot.AllocatedCPU += worker.AllocatedCPU
		snapshot.AllocatedMemory += worker.AllocatedMemory
		snapshot.AllocatedStorage += worker.AllocatedStorage
		snapshot.AvailableCPU += worker.AvailableCPU
		snapshot.AvailableMemory += worker.AvailableMemory
		snapshot.AvailableStorage += worker.AvailableStorage
	}

	snapshot.InactiveWorkers = snapshot.TotalWorkers - snapshot.ActiveWorkers

	// Calculate utilization percentages
	if snapshot.TotalCPU > 0 {
		snapshot.CPUUtilization = (snapshot.AllocatedCPU / snapshot.TotalCPU) * 100
	}
	if snapshot.TotalMemory > 0 {
		snapshot.MemoryUtilization = (snapshot.AllocatedMemory / snapshot.TotalMemory) * 100
	}
	if snapshot.TotalStorage > 0 {
		snapshot.StorageUtilization = (snapshot.AllocatedStorage / snapshot.TotalStorage) * 100
	}

	return snapshot
}

// DumpInMemoryState returns a formatted string of the complete in-memory state
func (s *MasterServer) DumpInMemoryState() string {
	snapshot := s.GetClusterSnapshot()

	var output string
	timestamp := snapshot.Timestamp.Format("2006/01/02 15:04:05")
	output += fmt.Sprintf("\n[%s] Master In-Memory State\n\n", timestamp)

	if len(snapshot.Workers) == 0 {
		output += "No workers registered.\n\n"
		return output
	}

	// Header
	output += "WORKER         STATUS  HEARTBEAT    CPU%   MEM%   STO%   ALLOC(C/M/S)         AVAIL(C/M/S)         TASKS\n"
	output += "──────────────────────────────────────────────────────────────────────────────────────────────────────────────\n"

	for _, worker := range snapshot.Workers {
		// Status
		status := "ACT"
		if worker.Status == "inactive" {
			status = "INA"
		}

		// Resource usage
		cpuUsage := fmt.Sprintf("%.1f", worker.CPUUsage)
		memUsage := fmt.Sprintf("%.1f", worker.MemoryUsage)
		storageUsage := fmt.Sprintf("%.1f", worker.StorageUsage)

		// Allocated resources
		allocStr := fmt.Sprintf("%.1f/%.1f/%.1f",
			worker.AllocatedCPU, worker.AllocatedMemory, worker.AllocatedStorage)

		// Available resources
		availStr := fmt.Sprintf("%.1f/%.1f/%.1f",
			worker.AvailableCPU, worker.AvailableMemory, worker.AvailableStorage)

		// Running tasks
		taskStr := "-"
		if len(worker.RunningTasks) > 0 {
			if len(worker.RunningTasks) <= 2 {
				taskStr = joinTasks(worker.RunningTasks)
			} else {
				taskStr = fmt.Sprintf("%s,+%d", joinTasks(worker.RunningTasks[:2]), len(worker.RunningTasks)-2)
			}
		}

		// Truncate worker ID if too long
		displayID := worker.WorkerID
		if len(displayID) > 14 {
			displayID = displayID[:14]
		}

		output += fmt.Sprintf("%-14s %-6s  %-11s  %-5s  %-5s  %-5s  %-19s  %-19s  %s\n",
			displayID, status, worker.HeartbeatAgo, cpuUsage, memUsage, storageUsage,
			allocStr, availStr, taskStr)
	}

	output += "\n"

	// Cluster summary
	output += fmt.Sprintf("Cluster: %d workers (%d active) | %d tasks | CPU: %.1f/%.1f (%.0f%%) | Mem: %.1f/%.1f GB (%.0f%%) | Storage: %.1f/%.1f GB (%.0f%%)\n\n",
		snapshot.TotalWorkers, snapshot.ActiveWorkers, snapshot.TotalTasks,
		snapshot.AllocatedCPU, snapshot.TotalCPU, snapshot.CPUUtilization,
		snapshot.AllocatedMemory, snapshot.TotalMemory, snapshot.MemoryUtilization,
		snapshot.AllocatedStorage, snapshot.TotalStorage, snapshot.StorageUtilization)

	return output
}

// joinTasks joins task IDs with commas
func joinTasks(tasks []string) string {
	result := ""
	for i, t := range tasks {
		if i > 0 {
			result += ","
		}
		result += t
	}
	return result
}

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

func taskIsTerminal(status string) bool {
	switch status {
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

// SubmitTask submits a task to the system for scheduling
// ALL tasks go through the queue first, then the scheduler assigns them to workers
func (s *MasterServer) SubmitTask(ctx context.Context, task *pb.Task) (*pb.TaskAck, error) {
	if task == nil {
		return &pb.TaskAck{
			Success: false,
			Message: "task payload is required",
		}, nil
	}

	taskMeta := normalizeTaskForScheduling(task)

	// Store task in database as queued
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

	// Enqueue the task for scheduling
	s.EnqueueTask(task, "Task submitted to queue for scheduling")

	// Get queue position
	s.queueMu.RLock()
	position := len(s.taskQueue)
	s.queueMu.RUnlock()

	log.Printf("📋 Task %s submitted and queued (position: %d)", task.TaskId, position)

	return &pb.TaskAck{
		Success: true,
		Message: fmt.Sprintf("Task submitted successfully. Queue position: %d. Scheduler will assign it to an available worker.", position),
	}, nil
}

// AssignTask is kept for backward compatibility but now redirects to SubmitTask
// This maintains the gRPC interface contract
func (s *MasterServer) AssignTask(ctx context.Context, task *pb.Task) (*pb.TaskAck, error) {
	return s.SubmitTask(ctx, task)
}

// DispatchTaskToWorker directly dispatches a task to a specific worker, bypassing the scheduler
// This is useful for testing and debugging purposes
func (s *MasterServer) DispatchTaskToWorker(ctx context.Context, task *pb.Task, workerID string) (*pb.TaskAck, error) {
	if task == nil {
		return &pb.TaskAck{
			Success: false,
			Message: "task payload is required",
		}, nil
	}

	taskMeta := normalizeTaskForScheduling(task)
	log.Printf("🎯 Direct dispatch request: Task %s -> Worker %s", task.TaskId, workerID)

	// Store task in database as queued first
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

	// Directly assign to the specified worker (bypassing queue and scheduler)
	ack, err := s.assignTaskToWorker(ctx, task, workerID)
	if err != nil {
		s.updateTaskStatusSafe(task.TaskId, "failed")
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Failed to dispatch task to worker %s: %v", workerID, err),
		}, nil
	}

	if !ack.Success {
		s.updateTaskStatusSafe(task.TaskId, "failed")
		return ack, nil
	}

	log.Printf("✅ Task %s dispatched directly to worker %s", task.TaskId, workerID)

	return &pb.TaskAck{
		Success: true,
		Message: fmt.Sprintf("Task dispatched directly to worker %s (bypassed scheduler)", workerID),
	}, nil
}

// StreamTaskLogs handles gRPC streaming of task logs (called by master CLI)
func (s *MasterServer) StreamTaskLogs(req *pb.TaskLogRequest, stream pb.MasterWorker_StreamTaskLogsServer) error {
	// This is a stub - the master doesn't receive this call from workers
	// The actual implementation is in worker
	return fmt.Errorf("StreamTaskLogs should be called on worker, not master")
}

// StreamTaskLogsFromWorker streams logs for a task from the worker (helper method for CLI)
func (s *MasterServer) StreamTaskLogsFromWorker(ctx context.Context, taskID, userID string, logHandler func(string, bool)) error {
	s.mu.RLock()

	// First, check if task is completed and logs are in database
	if s.resultDB != nil {
		result, err := s.resultDB.GetResult(ctx, taskID)
		if err == nil && result != nil {
			// Task is completed, stream stored logs line by line
			s.mu.RUnlock()

			// Split logs by newlines and stream them
			lines := strings.Split(result.Logs, "\n")
			for i, line := range lines {
				// Send each line with a small delay to simulate streaming
				time.Sleep(10 * time.Millisecond)
				isLastLine := i == len(lines)-1
				logHandler(line, isLastLine)
			}
			return nil
		}
	}

	// Task might be running, try to stream from worker
	// Get task from database to find the worker
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

	// Get worker info
	worker, exists := s.workers[workerID]
	if !exists {
		s.mu.RUnlock()
		return fmt.Errorf("worker %s not found", workerID)
	}

	workerIP := worker.Info.WorkerIp
	s.mu.RUnlock()

	// Connect to worker
	conn, err := grpc.Dial(workerIP, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to worker: %w", err)
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)

	// Request log stream
	stream, err := client.StreamTaskLogs(ctx, &pb.TaskLogRequest{
		TaskId: taskID,
		UserId: userID,
		Follow: true,
	})
	if err != nil {
		return fmt.Errorf("failed to start log stream: %w", err)
	}

	// Stream logs
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			}
			return fmt.Errorf("error receiving log chunk: %w", err)
		}

		// Pass log content to handler
		logHandler(chunk.Content, chunk.IsComplete)

		if chunk.IsComplete {
			// Update task status in database if completed
			if s.taskDB != nil && chunk.Status != "running" {
				s.taskDB.UpdateTaskStatus(ctx, taskID, chunk.Status)
			}
			return nil
		}
	}
}

// GetUserIDForTask retrieves the user ID associated with a task from the database
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

// BroadcastMasterRegistration calls MasterRegister on all pre-registered workers
// so the master can announce its address and allow workers to connect back.
func (s *MasterServer) BroadcastMasterRegistration(masterID, masterAddress string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, ws := range s.workers {
		if ws == nil || ws.Info == nil || ws.Info.WorkerIp == "" {
			continue
		}

		workerAddr := ws.Info.WorkerIp
		go func(workerID, workerAddr string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			conn, err := grpc.DialContext(ctx, workerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
			if err != nil {
				log.Printf("Failed to connect to worker %s (%s) for MasterRegister: %v", workerID, workerAddr, err)
				return
			}
			defer conn.Close()

			client := pb.NewMasterWorkerClient(conn)
			mi := &pb.MasterInfo{MasterId: masterID, MasterAddress: masterAddress}
			ack, err := client.MasterRegister(ctx, mi)
			if err != nil {
				log.Printf("MasterRegister RPC to worker %s (%s) failed: %v", workerID, workerAddr, err)
				return
			}
			if ack != nil && ack.Success {
				log.Printf("MasterRegister acknowledged by worker %s: %s", workerID, ack.Message)
			} else if ack != nil {
				log.Printf("MasterRegister rejected by worker %s: %s", workerID, ack.Message)
			}
		}(id, workerAddr)
	}
}

func (s *MasterServer) CancelTask(ctx context.Context, taskID *pb.TaskID) (*pb.TaskAck, error) {
	// Handle queued tasks first (no worker assignment yet).
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
		log.Printf("✓ Cancelled queued task %s before assignment", taskID.TaskId)
		return &pb.TaskAck{
			Success: true,
			Message: "Queued task cancelled successfully",
		}, nil
	}

	// If scheduling is currently in-flight for this task, mark it for cancellation.
	// processQueue will honor this flag before or immediately after assignment.
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

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("  🛑 CANCELLING TASK")
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("  Task ID: %s", taskID.TaskId)

	// Find which worker has this task
	var targetWorkerID string
	var targetWorker *WorkerState

	// First check in-memory running tasks
	for workerID, worker := range s.workers {
		if worker.RunningTasks != nil && worker.RunningTasks[taskID.TaskId] {
			targetWorkerID = workerID
			targetWorker = worker
			break
		}
	}

	// If not found in memory, check database
	if targetWorkerID == "" && s.assignmentDB != nil {
		workerID, err := s.assignmentDB.GetWorkerForTask(ctx, taskID.TaskId)
		if err != nil {
			log.Printf("  ✗ Task %s not found on any worker", taskID.TaskId)
			return &pb.TaskAck{
				Success: false,
				Message: fmt.Sprintf("Task not found or not assigned to any worker: %v", err),
			}, nil
		}
		targetWorkerID = workerID
		targetWorker = s.workers[workerID]
		if targetWorker == nil {
			log.Printf("  ✗ Worker %s not found", workerID)
			return &pb.TaskAck{
				Success: false,
				Message: fmt.Sprintf("Worker %s not found", workerID),
			}, nil
		}
	}

	if targetWorkerID == "" {
		log.Printf("  ✗ Task not found")
		return &pb.TaskAck{
			Success: false,
			Message: "Task not found or not running",
		}, nil
	}

	log.Printf("  Target Worker: %s (%s)", targetWorkerID, targetWorker.Info.WorkerIp)
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Update task status in database FIRST (optimistic update)
	// This ensures database is always updated even if worker communication fails
	if s.taskDB != nil {
		if err := s.taskDB.UpdateTaskStatus(ctx, taskID.TaskId, "cancelled"); err != nil {
			log.Printf("  ✗ CRITICAL: Failed to update task status in database: %v", err)
			return &pb.TaskAck{
				Success: false,
				Message: fmt.Sprintf("Failed to update database: %v", err),
			}, nil
		} else {
			log.Printf("  ✓ Task status updated to 'cancelled' in database")
		}
	} else {
		log.Printf("  ⚠ Warning: No database configured, task status not persisted")
	}

	// Connect to worker and send cancel request with extended timeout
	// Use a longer timeout for cancellation as it may involve stopping containers
	cancelCtx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()

	conn, err := grpc.Dial(targetWorker.Info.WorkerIp, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("  ✗ Failed to connect to worker: %v", err)
		log.Printf("  ⚠ Database updated but worker not reachable")
		// This is not a critical failure - DB is updated, worker will see it
		return &pb.TaskAck{
			Success: true,
			Message: fmt.Sprintf("Task marked as cancelled in database (worker unreachable: %v)", err),
		}, nil
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	ack, err := client.CancelTask(cancelCtx, taskID)
	if err != nil {
		log.Printf("  ✗ Failed to cancel task on worker: %v", err)
		log.Printf("  ⚠ Database updated but worker communication failed")
		// This is not a critical failure - DB is updated correctly
		return &pb.TaskAck{
			Success: true,
			Message: fmt.Sprintf("Task marked as cancelled in database (worker communication failed: %v)", err),
		}, nil
	}

	if !ack.Success {
		log.Printf("  ✗ Worker rejected cancellation: %s", ack.Message)
		log.Printf("  ⚠ Database marked as cancelled but worker could not stop task")
		return ack, nil
	}

	// Remove task from worker's running tasks
	if targetWorker.RunningTasks != nil {
		delete(targetWorker.RunningTasks, taskID.TaskId)
	}

	log.Printf("  ✓ Task cancelled successfully on worker")
	log.Printf("  ✓ Container stopped and database updated")
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	s.clearTaskCancellationRequest(taskID.TaskId)

	return &pb.TaskAck{
		Success: true,
		Message: "Task cancelled successfully",
	}, nil
}

// StartQueueProcessor starts the background task queue processor
func (s *MasterServer) StartQueueProcessor() {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()

	if s.queueTicker != nil {
		return
	}

	s.queueTicker = time.NewTicker(5 * time.Second) // Check queue every 5 seconds
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
// This is the main scheduler that selects workers for tasks
func (s *MasterServer) processQueue(ticker *time.Ticker, stopCh <-chan struct{}) {
	defer s.queueWG.Done()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
		}

		// Snapshot queue under lock and release before scheduling/RPC work.
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

		// Try to schedule and assign tasks from the queue.
		remainingTasks := make([]*QueuedTask, 0, len(tasksToProcess))
		for _, qt := range tasksToProcess {
			if qt == nil || qt.Task == nil {
				continue
			}
			taskID := qt.Task.TaskId
			// Task may have been cancelled while this queue cycle was in flight.
			if !s.isTaskBeingProcessed(taskID) {
				continue
			}
			if s.isTaskCancellationRequested(taskID) {
				s.clearTaskCancellationRequest(taskID)
				s.updateTaskStatusSafe(taskID, "cancelled")
				mastermetrics.Get().IncTaskDequeued("cancelled")
				continue
			}

			// Find the best worker for this task using the scheduler
			schedulingStarted := time.Now()
			selectedWorker := s.selectWorkerForTask(qt.Task)

			if selectedWorker == "" {
				// No suitable worker available, keep in queue
				qt.Retries++
				qt.LastError = "No suitable worker available with sufficient resources"
				s.updateTaskStatusSafe(taskID, "queued")
				remainingTasks = append(remainingTasks, qt)

				// Log only on first retry and every 10th retry to avoid spam
				if qt.Retries == 1 || qt.Retries%10 == 0 {
					log.Printf("📋 Queue: Task %s still waiting (attempt %d): %s",
						qt.Task.TaskId, qt.Retries, qt.LastError)
				}
				continue
			}

			// Set the selected worker as the target
			qt.Task.TargetWorkerId = selectedWorker

			// Re-check cancellation before sending gRPC assignment.
			if s.isTaskCancellationRequested(taskID) {
				s.clearTaskCancellationRequest(taskID)
				s.updateTaskStatusSafe(taskID, "cancelled")
				mastermetrics.Get().IncTaskDequeued("cancelled")
				continue
			}

			// Try to assign the task to the selected worker
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			ack, err := s.assignTaskToWorker(ctx, qt.Task, selectedWorker)
			cancel()
			mastermetrics.Get().ObserveSchedulingLatency(s.scheduler.GetName(), schedulingStarted)

			if err != nil || ack == nil || !ack.Success {
				// Assignment failed, keep in queue and try again later
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

		// Preserve FIFO: retries from the current cycle stay ahead of new arrivals.
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

// selectWorkerForTask uses the configured scheduler to select the best worker for a task
// Returns the worker ID or empty string if no suitable worker is found
func (s *MasterServer) selectWorkerForTask(task *pb.Task) string {
	s.mu.RLock()

	// Convert WorkerState map to scheduler.WorkerInfo map
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

	// Use the configured scheduler to select worker
	selectedWorker := s.scheduler.SelectWorker(task, workerInfos)
	return selectedWorker
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
// It is intended to be called once during startup.
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

			// Skip tasks that already have assignments.
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

// removeQueuedTaskByID removes a queued task if present and returns true when removed.
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

// isTaskBeingProcessed returns true if a task is currently scheduled in this queue cycle.
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

	// If assignment already exists, task is no longer just "in-flight scheduling";
	// let the normal cancellation path handle it.
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

// GetQueuedTasks returns a copy of the current task queue
func (s *MasterServer) GetQueuedTasks() []*QueuedTask {
	s.queueMu.RLock()
	defer s.queueMu.RUnlock()

	// Return a copy to avoid race conditions
	queueCopy := make([]*QueuedTask, len(s.taskQueue))
	copy(queueCopy, s.taskQueue)
	return queueCopy
}

// assignTaskToWorker assigns a task to a specific worker
// This is called by the scheduler after selecting an appropriate worker
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

	// Find the specified worker
	worker, exists := s.workers[workerID]
	if !exists {
		s.mu.Unlock()
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Worker %s not found", workerID)}, nil
	}
	if !worker.IsActive {
		s.mu.Unlock()
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Worker %s is not active", workerID)}, nil
	}

	// Validate worker IP is set
	if worker.Info.WorkerIp == "" {
		s.mu.Unlock()
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Worker %s has no IP address configured", workerID)}, nil
	}

	// CHECK RESOURCE AVAILABILITY - Prevent Oversubscription
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
	// Reserve resources before releasing lock to prevent oversubscription races.
	worker.AllocatedCPU += task.ReqCpu
	worker.AllocatedMemory += task.ReqMemory
	worker.AllocatedStorage += task.ReqStorage
	worker.AvailableCPU -= task.ReqCpu
	worker.AvailableMemory -= task.ReqMemory
	worker.AvailableStorage -= task.ReqStorage

	// Cache resource requirements so SendTaskResult can free them without DB.
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

	// Connect to worker and assign task
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

	// Update database
	if s.workerDB != nil {
		if err := s.workerDB.AllocateResources(ctx, workerID,
			task.ReqCpu, task.ReqMemory, task.ReqStorage); err != nil {
			log.Printf("Warning: Failed to allocate resources in database: %v", err)
		}
	}

	// Update task status to running
	if s.taskDB != nil {
		if err := s.taskDB.UpdateTaskStatus(ctx, task.TaskId, "running"); err != nil {
			log.Printf("Warning: Failed to update task status: %v", err)
		}
	}

	// Store assignment in database
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
	log.Println("───────────────────────────────────────────────────────")
	log.Println("  Resource Requirements:")
	log.Printf("    • CPU Cores:     %.2f cores", task.ReqCpu)
	log.Printf("    • Memory:        %.2f GB", task.ReqMemory)
	log.Printf("    • Storage:       %.2f GB", task.ReqStorage)
	log.Println("═══════════════════════════════════════════════════════")
	log.Println("")

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
	// Backward compatibility for older workers that still report 0-100.
	if value > 1.0 {
		value = value / 100.0
	}
	return value
}
