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
}

// TaskAssignment represents a task to be sent to a worker
type TaskAssignment struct {
	Task     *pb.Task
	WorkerID string
}

// NewMasterServer creates a new master server instance
func NewMasterServer(workerDB *db.WorkerDB, telemetryMgr *telemetry.TelemetryManager) *MasterServer {
	return &MasterServer{
		workers:          make(map[string]*WorkerState),
		workerDB:         workerDB,
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
			LastHeartbeat: w.LastHeartbeat,
			IsActive:      w.IsActive,
			RunningTasks:  make(map[string]bool),
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

	log.Printf("Task completion: %s from worker %s [Status: %s]",
		result.TaskId, result.WorkerId, result.Status)

	if len(result.Logs) > 0 {
		log.Printf("Task logs:\n%s", result.Logs)
	}

	// Remove task from worker's running tasks
	if worker, exists := s.workers[result.WorkerId]; exists {
		delete(worker.RunningTasks, result.TaskId)
	}

	// TODO: Store result in MongoDB

	return &pb.Ack{
		Success: true,
		Message: "Task result received",
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
		return &pb.TaskAck{Success: false, Message: fmt.Sprintf("Failed to assign task: %v", err)}, nil
	}

	if ack.Success {
		// Mark task as running on worker
		worker.RunningTasks[task.TaskId] = true
		log.Println("\n═══════════════════════════════════════════════════════")
		log.Println("  📤 TASK ASSIGNED TO WORKER")
		log.Println("═══════════════════════════════════════════════════════")
		log.Printf("  Task ID:           %s", task.TaskId)
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
	return &pb.TaskAck{Success: false, Message: "Not implemented"}, nil
}
