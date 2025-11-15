package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"master/internal/db"
	"master/internal/telemetry"
	pb "master/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MasterServer handles gRPC requests from workers
type MasterServer struct {
	pb.UnimplementedMasterWorkerServer

	workers       map[string]*WorkerState
	mu            sync.RWMutex
	workerDB      *db.WorkerDB
	taskDB        *db.TaskDB
	assignmentDB  *db.AssignmentDB
	resultDB      *db.ResultDB
	masterID      string
	masterAddress string

	taskChan chan *TaskAssignment

	// Telemetry manager for handling worker telemetry in separate threads
	telemetryManager *telemetry.TelemetryManager
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
func NewMasterServer(workerDB *db.WorkerDB, taskDB *db.TaskDB, assignmentDB *db.AssignmentDB, resultDB *db.ResultDB, telemetryMgr *telemetry.TelemetryManager) *MasterServer {
	return &MasterServer{
		workers:          make(map[string]*WorkerState),
		workerDB:         workerDB,
		taskDB:           taskDB,
		assignmentDB:     assignmentDB,
		resultDB:         resultDB,
		masterID:         "",
		masterAddress:    "",
		taskChan:         make(chan *TaskAssignment, 100),
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

	// Initialize available resources (total - already allocated)
	existingWorker.AvailableCPU = info.TotalCpu - existingWorker.AllocatedCPU
	existingWorker.AvailableMemory = info.TotalMemory - existingWorker.AllocatedMemory
	existingWorker.AvailableStorage = info.TotalStorage - existingWorker.AllocatedStorage
	existingWorker.AvailableGPU = info.TotalGpu - existingWorker.AllocatedGPU

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
		delete(worker.RunningTasks, result.TaskId)

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
		for taskID := range worker.RunningTasks {
			runningTasks = append(runningTasks, taskID)
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
		snapshot.TotalTasks += len(worker.RunningTasks)
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

// AssignTask assigns a task to a specific worker (target_worker_id is required)
func (s *MasterServer) AssignTask(ctx context.Context, task *pb.Task) (*pb.TaskAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate that target_worker_id is specified
	if task.TargetWorkerId == "" {
		return &pb.TaskAck{Success: false, Message: "target_worker_id is required"}, nil
	}

	// Find the specified worker
	worker, exists := s.workers[task.TargetWorkerId]
	if !exists {
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Worker %s not found", task.TargetWorkerId)}, nil
	}
	if !worker.IsActive {
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Worker %s is not active", task.TargetWorkerId)}, nil
	}

	// Validate worker IP is set
	if worker.Info.WorkerIp == "" {
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Worker %s has no IP address configured", task.TargetWorkerId)}, nil
	}

	// CHECK RESOURCE AVAILABILITY - Prevent Oversubscription
	if worker.AvailableCPU < task.ReqCpu {
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Insufficient CPU: worker has %.2f available, task requires %.2f",
				worker.AvailableCPU, task.ReqCpu),
		}, nil
	}
	if worker.AvailableMemory < task.ReqMemory {
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Insufficient Memory: worker has %.2f GB available, task requires %.2f GB",
				worker.AvailableMemory, task.ReqMemory),
		}, nil
	}
	if worker.AvailableStorage < task.ReqStorage {
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Insufficient Storage: worker has %.2f GB available, task requires %.2f GB",
				worker.AvailableStorage, task.ReqStorage),
		}, nil
	}
	if worker.AvailableGPU < task.ReqGpu {
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Insufficient GPU: worker has %.2f available, task requires %.2f",
				worker.AvailableGPU, task.ReqGpu),
		}, nil
	}

	// Store task in database (if DB is available)
	if s.taskDB != nil {
		dbTask := &db.Task{
			TaskID:      task.TaskId,
			UserID:      task.UserId,
			DockerImage: task.DockerImage,
			Command:     task.Command,
			ReqCPU:      task.ReqCpu,
			ReqMemory:   task.ReqMemory,
			ReqStorage:  task.ReqStorage,
			ReqGPU:      task.ReqGpu,
			Status:      "pending",
		}
		if err := s.taskDB.CreateTask(ctx, dbTask); err != nil {
			log.Printf("Warning: Failed to store task in database: %v", err)
		}
	}

	log.Printf("Connecting to worker %s at %s", task.TargetWorkerId, worker.Info.WorkerIp)

	// Connect to worker and assign task
	conn, err := grpc.Dial(worker.Info.WorkerIp, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

		// Update database
		if s.workerDB != nil {
			if err := s.workerDB.AllocateResources(ctx, task.TargetWorkerId,
				task.ReqCpu, task.ReqMemory, task.ReqStorage, task.ReqGpu); err != nil {
				log.Printf("Warning: Failed to allocate resources in database: %v", err)
			}
		}

		// Store assignment in database (if DB is available)
		if s.assignmentDB != nil {
			assignment := &db.Assignment{
				AssignmentID: fmt.Sprintf("ass-%s", task.TaskId),
				TaskID:       task.TaskId,
				WorkerID:     task.TargetWorkerId,
			}
			if err := s.assignmentDB.CreateAssignment(ctx, assignment); err != nil {
				log.Printf("Warning: Failed to store assignment in database: %v", err)
			}
		}

		// Update task status to running (if DB is available)
		if s.taskDB != nil {
			if err := s.taskDB.UpdateTaskStatus(ctx, task.TaskId, "running"); err != nil {
				log.Printf("Warning: Failed to update task status: %v", err)
			}
		}

		log.Println("\n═══════════════════════════════════════════════════════")
		log.Println("  📤 TASK ASSIGNED TO WORKER")
		log.Println("═══════════════════════════════════════════════════════")
		log.Printf("  Task ID:           %s", task.TaskId)
		log.Printf("  User ID:           %s", task.UserId)
		log.Printf("  Target Worker:     %s", task.TargetWorkerId)
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

	return ack, nil
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
			// Task is completed, return stored logs
			s.mu.RUnlock()
			logHandler(result.Logs, true)
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
		if worker.RunningTasks[taskID.TaskId] {
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

	// Connect to worker and send cancel request
	conn, err := grpc.Dial(targetWorker.Info.WorkerIp, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("  ✗ Failed to connect to worker: %v", err)
		log.Printf("  ⚠ Database updated but worker not reachable")
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Database updated but failed to connect to worker: %v", err),
		}, nil
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	ack, err := client.CancelTask(ctx, taskID)
	if err != nil {
		log.Printf("  ✗ Failed to cancel task on worker: %v", err)
		log.Printf("  ⚠ Database updated but worker communication failed")
		return &pb.TaskAck{
			Success: false,
			Message: fmt.Sprintf("Database updated but failed to cancel on worker: %v", err),
		}, nil
	}

	if !ack.Success {
		log.Printf("  ✗ Worker rejected cancellation: %s", ack.Message)
		log.Printf("  ⚠ Database marked as cancelled but worker could not stop task")
		return ack, nil
	}

	// Remove task from worker's running tasks
	delete(targetWorker.RunningTasks, taskID.TaskId)

	log.Printf("  ✓ Task cancelled successfully on worker")
	log.Printf("  ✓ Container stopped and database updated")
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return &pb.TaskAck{
		Success: true,
		Message: "Task cancelled successfully",
	}, nil
}
