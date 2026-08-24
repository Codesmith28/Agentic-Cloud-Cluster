package server

import (
	"fmt"
	"log"
	"sync"
	"time"

	"master/internal/db"
	"master/internal/scheduler"
	"master/internal/storage"
	"master/internal/telemetry"
	pb "master/proto"
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

// MasterServer handles gRPC requests from workers and clients
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

	// In-memory resource cache: taskID -> resource requirements
	taskResourceCache   map[string]*db.Task
	taskResourceCacheMu sync.Mutex

	// Task scheduler
	scheduler scheduler.Scheduler

	// Telemetry manager
	telemetryManager *telemetry.TelemetryManager

	// Worker reconnection
	reconnectTicker *time.Ticker
	reconnectStop   chan bool
}

// WorkerState tracks the current state of a worker
type WorkerState struct {
	Info             *pb.WorkerInfo
	LastHeartbeat    int64
	IsActive         bool
	RunningTasks     map[string]bool
	LatestCPU        float64
	LatestMemory     float64
	LatestStorage    float64
	TaskCount        int
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
		scheduler:            scheduler.NewRoundRobinScheduler(),
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

// SetScheduler sets the active scheduler
func (s *MasterServer) SetScheduler(sched scheduler.Scheduler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduler = sched
	log.Printf("Active scheduler set to: %s", sched.GetName())
}

// GetSchedulerName returns the name of the active scheduler
func (s *MasterServer) GetSchedulerName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.scheduler == nil {
		return "None"
	}
	return s.scheduler.GetName()
}

// GetWorkers returns current worker states
func (s *MasterServer) GetWorkers() map[string]*WorkerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make(map[string]*WorkerState)
	now := time.Now().Unix()

	for k, v := range s.workers {
		workerCopy := *v
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

// GetWorkerTelemetry returns telemetry data for a worker
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
	Status           string
	LastHeartbeat    int64
	HeartbeatAgo     string
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
	CPUUtilization     float64
	TotalMemory        float64
	AllocatedMemory    float64
	AvailableMemory    float64
	MemoryUtilization  float64
	TotalStorage       float64
	AllocatedStorage   float64
	AvailableStorage   float64
	StorageUtilization float64
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

		status := "active"
		if !worker.IsActive {
			status = "inactive"
		}

		runningTasks := []string{}
		if worker.RunningTasks != nil {
			for taskID := range worker.RunningTasks {
				runningTasks = append(runningTasks, taskID)
			}
		}

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

	output += "WORKER         STATUS  HEARTBEAT    CPU%   MEM%   STO%   ALLOC(C/M/S)         AVAIL(C/M/S)         TASKS\n"
	output += "──────────────────────────────────────────────────────────────────────────────────────────────────────────────\n"

	for _, worker := range snapshot.Workers {
		status := "ACT"
		if worker.Status == "inactive" {
			status = "INA"
		}

		cpuUsage := fmt.Sprintf("%.1f", worker.CPUUsage)
		memUsage := fmt.Sprintf("%.1f", worker.MemoryUsage)
		storageUsage := fmt.Sprintf("%.1f", worker.StorageUsage)

		allocStr := fmt.Sprintf("%.1f/%.1f/%.1f",
			worker.AllocatedCPU, worker.AllocatedMemory, worker.AllocatedStorage)

		availStr := fmt.Sprintf("%.1f/%.1f/%.1f",
			worker.AvailableCPU, worker.AvailableMemory, worker.AvailableStorage)

		taskStr := "-"
		if len(worker.RunningTasks) > 0 {
			if len(worker.RunningTasks) <= 2 {
				taskStr = joinTasks(worker.RunningTasks)
			} else {
				taskStr = fmt.Sprintf("%s,+%d", joinTasks(worker.RunningTasks[:2]), len(worker.RunningTasks)-2)
			}
		}

		displayID := worker.WorkerID
		if len(displayID) > 14 {
			displayID = displayID[:14]
		}

		output += fmt.Sprintf("%-14s %-6s  %-11s  %-5s  %-5s  %-5s  %-19s  %-19s  %s\n",
			displayID, status, worker.HeartbeatAgo, cpuUsage, memUsage, storageUsage,
			allocStr, availStr, taskStr)
	}

	output += "\n"
	output += fmt.Sprintf("Cluster: %d workers (%d active) | %d tasks | CPU: %.1f/%.1f (%.0f%%) | Mem: %.1f/%.1f GB (%.0f%%) | Storage: %.1f/%.1f GB (%.0f%%)\n\n",
		snapshot.TotalWorkers, snapshot.ActiveWorkers, snapshot.TotalTasks,
		snapshot.AllocatedCPU, snapshot.TotalCPU, snapshot.CPUUtilization,
		snapshot.AllocatedMemory, snapshot.TotalMemory, snapshot.MemoryUtilization,
		snapshot.AllocatedStorage, snapshot.TotalStorage, snapshot.StorageUtilization)

	return output
}

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
