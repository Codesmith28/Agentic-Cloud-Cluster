package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"master/internal/db"
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
	resultDB       *db.ResultDB
	fileMetadataDB *db.FileMetadataDB
	fileStorage    *storage.FileStorageService
	masterID       string
	masterAddress  string

	taskChan chan *TaskAssignment

	// Task queue for tasks waiting for resources
	taskQueue   []*QueuedTask
	queueMu     sync.RWMutex
	queueTicker *time.Ticker

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
	LatestGPU     float64 // Latest GPU usage from heartbeat
	TaskCount     int     // Number of running tasks from latest heartbeat
	// Resource tracking
	AllocatedCPU     float64
	AllocatedMemory  float64
	AllocatedStorage float64
	AllocatedGPU     float64
	AvailableCPU     float64
	AvailableMemory  float64
	AvailableStorage float64
	AvailableGPU     float64
}

// TaskAssignment represents a task to be sent to a worker
type TaskAssignment struct {
	Task     *pb.Task
	WorkerID string
}

// NewMasterServer creates a new master server instance
func NewMasterServer(workerDB *db.WorkerDB, taskDB *db.TaskDB, assignmentDB *db.AssignmentDB, resultDB *db.ResultDB, fileMetadataDB *db.FileMetadataDB, fileStorage *storage.FileStorageService, telemetryMgr *telemetry.TelemetryManager) *MasterServer {
	return &MasterServer{
		workers:          make(map[string]*WorkerState),
		workerDB:         workerDB,
		taskDB:           taskDB,
		assignmentDB:     assignmentDB,
		resultDB:         resultDB,
		fileMetadataDB:   fileMetadataDB,
		fileStorage:      fileStorage,
		masterID:         "",
		masterAddress:    "",
		taskChan:         make(chan *TaskAssignment, 100),
		taskQueue:        make([]*QueuedTask, 0),
		scheduler:        scheduler.NewRoundRobinScheduler(), // Use Round-Robin as default
		telemetryManager: telemetryMgr,
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
				TotalGpu:     w.TotalGPU,
			},
			LastHeartbeat:    w.LastHeartbeat,
			IsActive:         w.IsActive,
			RunningTasks:     make(map[string]bool),
			AllocatedCPU:     w.AllocatedCPU,
			AllocatedMemory:  w.AllocatedMemory,
			AllocatedStorage: w.AllocatedStorage,
			AllocatedGPU:     w.AllocatedGPU,
			AvailableCPU:     w.AvailableCPU,
			AvailableMemory:  w.AvailableMemory,
			AvailableStorage: w.AvailableStorage,
			AvailableGPU:     w.AvailableGPU,
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

	// Check if already exists
	if _, exists := s.workers[workerID]; exists {
		return fmt.Errorf("worker %s already registered", workerID)
	}

	// Add to database
	if s.workerDB != nil {
		exists, err := s.workerDB.WorkerExists(ctx, workerID)
		if err != nil {
			return fmt.Errorf("check worker existence: %w", err)
		}
		if exists {
			return fmt.Errorf("worker %s already exists in database", workerID)
		}

		if err := s.workerDB.RegisterWorker(ctx, workerID, workerIP); err != nil {
			return fmt.Errorf("register worker in db: %w", err)
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
		AllocatedGPU:     0.0,
		AvailableCPU:     0.0,
		AvailableMemory:  0.0,
		AvailableStorage: 0.0,
		AvailableGPU:     0.0,
	}

	log.Printf("Manually registered worker: %s (Address: %s)", workerID, workerIP)
	return nil
}

// UpdateWorkerResourcesInMemory updates worker resources in memory (called from HTTP API after manual registration)
func (s *MasterServer) UpdateWorkerResourcesInMemory(workerID string, totalCPU, totalMemory, totalStorage, totalGPU float64) {
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
	worker.Info.TotalGpu = totalGPU

	// Calculate available resources (total - allocated)
	worker.AvailableCPU = totalCPU - worker.AllocatedCPU
	worker.AvailableMemory = totalMemory - worker.AllocatedMemory
	worker.AvailableStorage = totalStorage - worker.AllocatedStorage
	worker.AvailableGPU = totalGPU - worker.AllocatedGPU

	// Mark worker as active since it has been configured
	worker.IsActive = true

	log.Printf("Updated worker %s resources: CPU=%.2f, Memory=%.2f, Storage=%.2f, GPU=%.2f",
		workerID, totalCPU, totalMemory, totalStorage, totalGPU)
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
		CPU, Memory, Storage, GPU float64
		TaskIDs                   map[string]bool
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
				CPU, Memory, Storage, GPU float64
				TaskIDs                   map[string]bool
			}{TaskIDs: make(map[string]bool)}
		}

		alloc := actualAllocations[workerID]
		alloc.CPU += task.ReqCPU
		alloc.Memory += task.ReqMemory
		alloc.Storage += task.ReqStorage
		alloc.GPU += task.ReqGPU
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
			worker.AllocatedStorage != actual.Storage ||
			worker.AllocatedGPU != actual.GPU {

			oldCPU := worker.AllocatedCPU
			oldMem := worker.AllocatedMemory

			// Fix the allocations
			worker.AllocatedCPU = actual.CPU
			worker.AllocatedMemory = actual.Memory
			worker.AllocatedStorage = actual.Storage
			worker.AllocatedGPU = actual.GPU

			// Recalculate available resources
			worker.AvailableCPU = worker.Info.TotalCpu - actual.CPU
			worker.AvailableMemory = worker.Info.TotalMemory - actual.Memory
			worker.AvailableStorage = worker.Info.TotalStorage - actual.Storage
			worker.AvailableGPU = worker.Info.TotalGpu - actual.GPU

			// Update running tasks map
			worker.RunningTasks = actual.TaskIDs

			// Update in database
			// First release all old allocations, then allocate the correct amount
			if s.workerDB != nil && oldCPU > 0 {
				if err := s.workerDB.ReleaseResources(ctx, workerID,
					oldCPU, oldMem, worker.AllocatedStorage, worker.AllocatedGPU); err != nil {
					log.Printf("⚠ Failed to release old resources for %s in DB: %v", workerID, err)
				}
			}

			// Now allocate the correct amount
			if s.workerDB != nil && actual.CPU > 0 {
				if err := s.workerDB.AllocateResources(ctx, workerID,
					actual.CPU, actual.Memory, actual.Storage, actual.GPU); err != nil {
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
	var actualCPU, actualMemory, actualStorage, actualGPU float64
	actualTaskIDs := make(map[string]bool)

	for _, task := range tasks {
		// Get assignment to find which worker
		assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, task.TaskID)
		if err != nil {
			log.Printf("  ⚠ Task %s has no assignment, skipping", task.TaskID)
			continue
		}

		if assignment.WorkerID == workerID {
			log.Printf("  📋 Found task %s assigned to %s (CPU=%.1f, Mem=%.1f, Storage=%.1f, GPU=%.1f)",
				task.TaskID, workerID, task.ReqCPU, task.ReqMemory, task.ReqStorage, task.ReqGPU)
			actualCPU += task.ReqCPU
			actualMemory += task.ReqMemory
			actualStorage += task.ReqStorage
			actualGPU += task.ReqGPU
			actualTaskIDs[task.TaskID] = true
		}
	}

	// Update worker's allocated resources
	worker.AllocatedCPU = actualCPU
	worker.AllocatedMemory = actualMemory
	worker.AllocatedStorage = actualStorage
	worker.AllocatedGPU = actualGPU

	// Recalculate available resources
	worker.AvailableCPU = worker.Info.TotalCpu - actualCPU
	worker.AvailableMemory = worker.Info.TotalMemory - actualMemory
	worker.AvailableStorage = worker.Info.TotalStorage - actualStorage
	worker.AvailableGPU = worker.Info.TotalGpu - actualGPU

	// Update running tasks map
	worker.RunningTasks = actualTaskIDs

	// Update database with correct allocations
	if s.workerDB != nil {
		if err := s.workerDB.SetWorkerResources(ctx, workerID,
			actualCPU, actualMemory, actualStorage, actualGPU,
			worker.AvailableCPU, worker.AvailableMemory, worker.AvailableStorage, worker.AvailableGPU); err != nil {
			log.Printf("⚠ Failed to update resources for %s in DB: %v", workerID, err)
		}
	}

	log.Printf("  ✓ Reconciled %s: CPU=%.1f, Memory=%.1f, Storage=%.1f, GPU=%.1f, Tasks=%d",
		workerID, actualCPU, actualMemory, actualStorage, actualGPU, len(actualTaskIDs))
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
	s.reconnectTicker = time.NewTicker(30 * time.Second) // Check every 30 seconds
	s.reconnectStop = make(chan bool)

	go func() {
		log.Println("🔄 Worker reconnection monitor started")
		for {
			select {
			case <-s.reconnectTicker.C:
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

	// Worker IS pre-registered - update with full specs but preserve the IP from manual registration
	preservedIP := existingWorker.Info.WorkerIp
	existingWorker.Info = info

	// If worker didn't provide IP or provided empty IP, use the one from manual registration
	if existingWorker.Info.WorkerIp == "" {
		existingWorker.Info.WorkerIp = preservedIP
		log.Printf("✓ Worker %s registered - using pre-configured address: %s", info.WorkerId, preservedIP)
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
		existingWorker.AllocatedGPU = 0.0

		// Initialize available resources to total
		existingWorker.AvailableCPU = info.TotalCpu
		existingWorker.AvailableMemory = info.TotalMemory
		existingWorker.AvailableStorage = info.TotalStorage
		existingWorker.AvailableGPU = info.TotalGpu

		// Trigger reconciliation for this specific worker to fix resources based on actual running tasks
		s.reconcileSingleWorker(ctx, info.WorkerId, existingWorker)
	} else {
		// Worker is already connected with same specs, just update available resources
		existingWorker.AvailableCPU = info.TotalCpu - existingWorker.AllocatedCPU
		existingWorker.AvailableMemory = info.TotalMemory - existingWorker.AllocatedMemory
		existingWorker.AvailableStorage = info.TotalStorage - existingWorker.AllocatedStorage
		existingWorker.AvailableGPU = info.TotalGpu - existingWorker.AllocatedGPU
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
	worker.LatestCPU = hb.CpuUsage
	worker.LatestMemory = hb.MemoryUsage
	worker.LatestGPU = hb.GpuUsage
	worker.TaskCount = len(hb.RunningTasks)

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

	// Get task info to retrieve resource requirements
	var taskResources *db.Task
	if s.taskDB != nil {
		task, err := s.taskDB.GetTask(context.Background(), result.TaskId)
		if err != nil {
			log.Printf("  ⚠ Warning: Failed to get task info for resource release: %v", err)
		} else {
			taskResources = task
		}
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
			worker.AllocatedGPU -= taskResources.ReqGPU
			worker.AvailableCPU += taskResources.ReqCPU
			worker.AvailableMemory += taskResources.ReqMemory
			worker.AvailableStorage += taskResources.ReqStorage
			worker.AvailableGPU += taskResources.ReqGPU

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
			if worker.AllocatedGPU < 0 {
				worker.AllocatedGPU = 0
			}

			// Update database
			if s.workerDB != nil {
				if err := s.workerDB.ReleaseResources(ctx, result.WorkerId,
					taskResources.ReqCPU, taskResources.ReqMemory,
					taskResources.ReqStorage, taskResources.ReqGPU); err != nil {
					log.Printf("  ⚠ Warning: Failed to release resources in database: %v", err)
				} else {
					log.Printf("  ✓ Released resources: CPU=%.2f, Memory=%.2f, Storage=%.2f, GPU=%.2f",
						taskResources.ReqCPU, taskResources.ReqMemory, taskResources.ReqStorage, taskResources.ReqGPU)
				}
			}
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
			return &pb.Ack{
				Success: true,
				Message: "Task result received (status preserved as cancelled)",
			}, nil
		}

		status := "completed"
		if result.Status == "cancelled" {
			status = "cancelled"
			log.Printf("  ℹ Confirming task %s 'cancelled' status (already set by master)", result.TaskId)
		} else if result.Status != "success" {
			status = "failed"
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

	return &pb.Ack{
		Success: true,
		Message: "Task result received and processed",
	}, nil
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
	for k, v := range s.workers {
		workers[k] = v
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
	GPUUsage         float64
	TotalCPU         float64
	TotalMemory      float64
	TotalStorage     float64
	TotalGPU         float64
	AllocatedCPU     float64
	AllocatedMemory  float64
	AllocatedStorage float64
	AllocatedGPU     float64
	AvailableCPU     float64
	AvailableMemory  float64
	AvailableStorage float64
	AvailableGPU     float64
	RunningTasks     []string
	TaskCount        int
}

// ClusterSnapshot represents a point-in-time snapshot of the entire cluster
type ClusterSnapshot struct {
	Timestamp         time.Time
	Workers           []WorkerStateSnapshot
	TotalWorkers      int
	ActiveWorkers     int
	InactiveWorkers   int
	TotalTasks        int
	TotalCPU          float64
	AllocatedCPU      float64
	AvailableCPU      float64
	CPUUtilization    float64 // Percentage
	TotalMemory       float64
	AllocatedMemory   float64
	AvailableMemory   float64
	MemoryUtilization float64 // Percentage
	TotalGPU          float64
	AllocatedGPU      float64
	AvailableGPU      float64
	GPUUtilization    float64 // Percentage
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
		var totalCPU, totalMemory, totalStorage, totalGPU float64
		var workerIP string
		if worker.Info != nil {
			totalCPU = worker.Info.TotalCpu
			totalMemory = worker.Info.TotalMemory
			totalStorage = worker.Info.TotalStorage
			totalGPU = worker.Info.TotalGpu
			workerIP = worker.Info.WorkerIp
		}

		workerSnapshot := WorkerStateSnapshot{
			WorkerID:         workerID,
			WorkerIP:         workerIP,
			Status:           status,
			LastHeartbeat:    worker.LastHeartbeat,
			HeartbeatAgo:     heartbeatAgo,
			CPUUsage:         worker.LatestCPU,
			MemoryUsage:      worker.LatestMemory,
			GPUUsage:         worker.LatestGPU,
			TotalCPU:         totalCPU,
			TotalMemory:      totalMemory,
			TotalStorage:     totalStorage,
			TotalGPU:         totalGPU,
			AllocatedCPU:     worker.AllocatedCPU,
			AllocatedMemory:  worker.AllocatedMemory,
			AllocatedStorage: worker.AllocatedStorage,
			AllocatedGPU:     worker.AllocatedGPU,
			AvailableCPU:     worker.AvailableCPU,
			AvailableMemory:  worker.AvailableMemory,
			AvailableStorage: worker.AvailableStorage,
			AvailableGPU:     worker.AvailableGPU,
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
		snapshot.TotalGPU += totalGPU
		snapshot.AllocatedCPU += worker.AllocatedCPU
		snapshot.AllocatedMemory += worker.AllocatedMemory
		snapshot.AllocatedGPU += worker.AllocatedGPU
		snapshot.AvailableCPU += worker.AvailableCPU
		snapshot.AvailableMemory += worker.AvailableMemory
		snapshot.AvailableGPU += worker.AvailableGPU
	}

	snapshot.InactiveWorkers = snapshot.TotalWorkers - snapshot.ActiveWorkers

	// Calculate utilization percentages
	if snapshot.TotalCPU > 0 {
		snapshot.CPUUtilization = (snapshot.AllocatedCPU / snapshot.TotalCPU) * 100
	}
	if snapshot.TotalMemory > 0 {
		snapshot.MemoryUtilization = (snapshot.AllocatedMemory / snapshot.TotalMemory) * 100
	}
	if snapshot.TotalGPU > 0 {
		snapshot.GPUUtilization = (snapshot.AllocatedGPU / snapshot.TotalGPU) * 100
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
	output += "WORKER         STATUS  HEARTBEAT    CPU%   MEM%   GPU%   ALLOC(C/M/G)         AVAIL(C/M/G)         TASKS\n"
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
		gpuUsage := fmt.Sprintf("%.1f", worker.GPUUsage)

		// Allocated resources
		allocStr := fmt.Sprintf("%.1f/%.1f/%.1f",
			worker.AllocatedCPU, worker.AllocatedMemory, worker.AllocatedGPU)

		// Available resources
		availStr := fmt.Sprintf("%.1f/%.1f/%.1f",
			worker.AvailableCPU, worker.AvailableMemory, worker.AvailableGPU)

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
			displayID, status, worker.HeartbeatAgo, cpuUsage, memUsage, gpuUsage,
			allocStr, availStr, taskStr)
	}

	output += "\n"

	// Cluster summary
	output += fmt.Sprintf("Cluster: %d workers (%d active) | %d tasks | CPU: %.1f/%.1f (%.0f%%) | Mem: %.1f/%.1f GB (%.0f%%)\n\n",
		snapshot.TotalWorkers, snapshot.ActiveWorkers, snapshot.TotalTasks,
		snapshot.AllocatedCPU, snapshot.TotalCPU, snapshot.CPUUtilization,
		snapshot.AllocatedMemory, snapshot.TotalMemory, snapshot.MemoryUtilization)

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

// SubmitTask submits a task to the system for scheduling
// ALL tasks go through the queue first, then the scheduler assigns them to workers
func (s *MasterServer) SubmitTask(ctx context.Context, task *pb.Task) (*pb.TaskAck, error) {
	// Store task in database as queued
	if s.taskDB != nil {
		dbTask := &db.Task{
			TaskID:      task.TaskId,
			UserID:      task.UserId,
			TaskName:    task.TaskName,
			SubmittedAt: task.SubmittedAt,
			DockerImage: task.DockerImage,
			Command:     task.Command,
			ReqCPU:      task.ReqCpu,
			ReqMemory:   task.ReqMemory,
			ReqStorage:  task.ReqStorage,
			ReqGPU:      task.ReqGpu,
			Status:      "queued",
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
	log.Printf("🎯 Direct dispatch request: Task %s -> Worker %s", task.TaskId, workerID)

	// Store task in database as queued first
	if s.taskDB != nil {
		dbTask := &db.Task{
			TaskID:      task.TaskId,
			UserID:      task.UserId,
			TaskName:    task.TaskName,
			SubmittedAt: task.SubmittedAt,
			DockerImage: task.DockerImage,
			Command:     task.Command,
			ReqCPU:      task.ReqCpu,
			ReqMemory:   task.ReqMemory,
			ReqStorage:  task.ReqStorage,
			ReqGPU:      task.ReqGpu,
			Status:      "queued",
		}
		if err := s.taskDB.CreateTask(ctx, dbTask); err != nil {
			log.Printf("Warning: Failed to store task in database: %v", err)
		}
	}

	// Directly assign to the specified worker (bypassing queue and scheduler)
	ack, err := s.assignTaskToWorker(ctx, task, workerID)
	if err != nil {
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Failed to dispatch task to worker %s: %v", workerID, err),
		}, nil
	}

	if !ack.Success {
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

	return &pb.TaskAck{
		Success: true,
		Message: "Task cancelled successfully",
	}, nil
}

// StartQueueProcessor starts the background task queue processor
func (s *MasterServer) StartQueueProcessor() {
	s.queueTicker = time.NewTicker(5 * time.Second) // Check queue every 5 seconds
	go s.processQueue()
	log.Printf("✓ Task queue processor started (checking every 5s)")
}

// StopQueueProcessor stops the background task queue processor
func (s *MasterServer) StopQueueProcessor() {
	if s.queueTicker != nil {
		s.queueTicker.Stop()
		log.Printf("✓ Task queue processor stopped")
	}
}

// processQueue continuously attempts to schedule and assign queued tasks
// This is the main scheduler that selects workers for tasks
func (s *MasterServer) processQueue() {
	for range s.queueTicker.C {
		s.queueMu.Lock()
		if len(s.taskQueue) == 0 {
			s.queueMu.Unlock()
			continue
		}

		// Try to schedule and assign tasks from the queue
		remainingTasks := make([]*QueuedTask, 0)
		for _, qt := range s.taskQueue {
			// Find the best worker for this task using the scheduler
			selectedWorker := s.selectWorkerForTask(qt.Task)

			if selectedWorker == "" {
				// No suitable worker available, keep in queue
				qt.Retries++
				qt.LastError = "No suitable worker available with sufficient resources"
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

			// Try to assign the task to the selected worker
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			ack, err := s.assignTaskToWorker(ctx, qt.Task, selectedWorker)
			cancel()

			if err != nil || !ack.Success {
				// Assignment failed, keep in queue and try again later
				qt.Retries++
				if err != nil {
					qt.LastError = err.Error()
				} else {
					qt.LastError = ack.Message
				}
				remainingTasks = append(remainingTasks, qt)

				if qt.Retries == 1 || qt.Retries%10 == 0 {
					log.Printf("📋 Queue: Task %s assignment to %s failed (attempt %d): %s",
						qt.Task.TaskId, selectedWorker, qt.Retries, qt.LastError)
				}
			} else {
				log.Printf("✓ Queue: Task %s successfully assigned to %s after %d attempts",
					qt.Task.TaskId, selectedWorker, qt.Retries)
			}
		}

		s.taskQueue = remainingTasks
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
			AvailableGPU:     worker.AvailableGPU,
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

	log.Printf("📋 Task %s queued: %s", task.TaskId, reason)
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
	if worker.AvailableGPU < task.ReqGpu {
		s.mu.Unlock()
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Insufficient GPU: worker has %.2f available, task requires %.2f",
				worker.AvailableGPU, task.ReqGpu),
		}, nil
	}

	workerIP := worker.Info.WorkerIp
	s.mu.Unlock()

	// Connect to worker and assign task
	conn, err := grpc.Dial(workerIP, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Failed to connect to worker: %v", err)}, nil
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	ack, err := client.AssignTask(ctx, task)
	if err != nil {
		// Update task status to failed if assignment fails
		if s.taskDB != nil {
			s.taskDB.UpdateTaskStatus(ctx, task.TaskId, "failed")
		}
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Failed to assign task: %v", err)}, nil
	}

	if ack.Success {
		s.mu.Lock()
		// Ensure RunningTasks map is initialized (defensive programming)
		if worker.RunningTasks == nil {
			worker.RunningTasks = make(map[string]bool)
		}
		// Mark task as running on worker
		worker.RunningTasks[task.TaskId] = true

		// 🚨 ALLOCATE RESOURCES - Update both in-memory and database
		worker.AllocatedCPU += task.ReqCpu
		worker.AllocatedMemory += task.ReqMemory
		worker.AllocatedStorage += task.ReqStorage
		worker.AllocatedGPU += task.ReqGpu
		worker.AvailableCPU -= task.ReqCpu
		worker.AvailableMemory -= task.ReqMemory
		worker.AvailableStorage -= task.ReqStorage
		worker.AvailableGPU -= task.ReqGpu
		s.mu.Unlock()

		// Update database
		if s.workerDB != nil {
			if err := s.workerDB.AllocateResources(ctx, workerID,
				task.ReqCpu, task.ReqMemory, task.ReqStorage, task.ReqGpu); err != nil {
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
			}
			if err := s.assignmentDB.CreateAssignment(ctx, assignment); err != nil {
				log.Printf("Warning: Failed to store assignment in database: %v", err)
			}
		}

		log.Println("\n═══════════════════════════════════════════════════════")
		log.Println("  📤 TASK ASSIGNED TO WORKER")
		log.Println("═══════════════════════════════════════════════════════")
		log.Printf("  Task ID:           %s", task.TaskId)
		log.Printf("  User ID:           %s", task.UserId)
		log.Printf("  Assigned Worker:   %s", workerID)
		log.Printf("  Docker Image:      %s", task.DockerImage)
		log.Println("───────────────────────────────────────────────────────")
		log.Println("  Resource Requirements:")
		log.Printf("    • CPU Cores:     %.2f cores", task.ReqCpu)
		log.Printf("    • Memory:        %.2f GB", task.ReqMemory)
		log.Printf("    • Storage:       %.2f GB", task.ReqStorage)
		log.Printf("    • GPU Cores:     %.2f cores", task.ReqGpu)
		log.Println("═══════════════════════════════════════════════════════")
		log.Println("")
	}

	return ack, err
}
