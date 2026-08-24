package server

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "master/proto"
	mastermetrics "master/internal/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// LoadWorkersFromDB loads registered workers from database into memory
func (s *MasterServer) LoadWorkersFromDB(ctx context.Context) error {
	if s.workerDB == nil {
		return nil
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

	s.ReconcileWorkerResources(ctx)
	log.Printf("Loaded %d workers from database", len(workers))
	return nil
}

// ManualRegisterWorker manually registers a worker (called from CLI)
func (s *MasterServer) ManualRegisterWorker(ctx context.Context, workerID, workerIP string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, exists := s.workers[workerID]; exists {
		oldAddress := ""
		if existing.Info == nil {
			existing.Info = &pb.WorkerInfo{WorkerId: workerID}
		} else {
			oldAddress = existing.Info.WorkerIp
		}

		existing.Info.WorkerId = workerID
		existing.Info.WorkerIp = workerIP
		existing.IsActive = false
		existing.LastHeartbeat = 0

		if s.workerDB != nil {
			if err := s.workerDB.UpdateWorkerAddress(ctx, workerID, workerIP); err != nil {
				return fmt.Errorf("update worker address in db: %w", err)
			}
		}

		log.Printf("Updated worker registration: %s (Address: %s -> %s)", workerID, oldAddress, workerIP)
		return nil
	}

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

	s.workers[workerID] = &WorkerState{
		Info: &pb.WorkerInfo{
			WorkerId: workerID,
			WorkerIp: workerIP,
		},
		LastHeartbeat: 0,
		IsActive:      false,
		RunningTasks:  make(map[string]bool),
	}

	log.Printf("Pre-registered worker: %s (Address: %s) - waiting for worker to connect", workerID, workerIP)
	return nil
}

// UpdateWorkerResourcesInMemory updates in-memory worker resource tracking
func (s *MasterServer) UpdateWorkerResourcesInMemory(workerID string, totalCPU, totalMemory, totalStorage float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, exists := s.workers[workerID]
	if !exists {
		return
	}

	worker.Info.TotalCpu = totalCPU
	worker.Info.TotalMemory = totalMemory
	worker.Info.TotalStorage = totalStorage

	worker.AvailableCPU = totalCPU - worker.AllocatedCPU
	worker.AvailableMemory = totalMemory - worker.AllocatedMemory
	worker.AvailableStorage = totalStorage - worker.AllocatedStorage

	if worker.AvailableCPU < 0 {
		worker.AvailableCPU = 0
	}
	if worker.AvailableMemory < 0 {
		worker.AvailableMemory = 0
	}
	if worker.AvailableStorage < 0 {
		worker.AvailableStorage = 0
	}
}

// ReconcileWorkerResources reconciles in-memory worker allocations with actual running tasks in DB
func (s *MasterServer) ReconcileWorkerResources(ctx context.Context) error {
	if s.taskDB == nil || s.assignmentDB == nil {
		return nil
	}

	tasks, err := s.taskDB.GetTasksByStatus(ctx, "running")
	if err != nil {
		log.Printf("⚠ Failed to get running tasks for reconciliation: %v", err)
		return err
	}

	actualAllocations := make(map[string]struct {
		CPU, Memory, Storage float64
		TaskIDs              map[string]bool
	})

	for _, task := range tasks {
		assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, task.TaskID)
		if err != nil {
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

	fixedCount := 0
	for workerID, worker := range s.workers {
		actual := actualAllocations[workerID]

		if worker.AllocatedCPU != actual.CPU ||
			worker.AllocatedMemory != actual.Memory ||
			worker.AllocatedStorage != actual.Storage {

			oldCPU := worker.AllocatedCPU
			oldMem := worker.AllocatedMemory
			oldStorage := worker.AllocatedStorage

			worker.AllocatedCPU = actual.CPU
			worker.AllocatedMemory = actual.Memory
			worker.AllocatedStorage = actual.Storage

			worker.AvailableCPU = worker.Info.TotalCpu - actual.CPU
			worker.AvailableMemory = worker.Info.TotalMemory - actual.Memory
			worker.AvailableStorage = worker.Info.TotalStorage - actual.Storage

			worker.RunningTasks = actual.TaskIDs

			if s.workerDB != nil && (oldCPU > 0 || oldMem > 0 || oldStorage > 0) {
				if err := s.workerDB.ReleaseResources(ctx, workerID,
					oldCPU, oldMem, oldStorage); err != nil {
					log.Printf("⚠ Failed to release old resources for %s in DB: %v", workerID, err)
				}
			}

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

func (s *MasterServer) reconcileSingleWorker(ctx context.Context, workerID string, worker *WorkerState) {
	if s.taskDB == nil || s.assignmentDB == nil {
		log.Printf("⚠ Resource reconciliation skipped for %s: databases not available", workerID)
		return
	}

	tasks, err := s.taskDB.GetTasksByStatus(ctx, "running")
	if err != nil {
		log.Printf("⚠ Failed to get running tasks for reconciliation: %v", err)
		return
	}

	var actualCPU, actualMemory, actualStorage float64
	actualTaskIDs := make(map[string]bool)

	for _, task := range tasks {
		assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, task.TaskID)
		if err != nil {
			continue
		}

		if assignment.WorkerID == workerID {
			actualCPU += task.ReqCPU
			actualMemory += task.ReqMemory
			actualStorage += task.ReqStorage
			actualTaskIDs[task.TaskID] = true
		}
	}

	worker.AllocatedCPU = actualCPU
	worker.AllocatedMemory = actualMemory
	worker.AllocatedStorage = actualStorage

	worker.AvailableCPU = worker.Info.TotalCpu - actualCPU
	worker.AvailableMemory = worker.Info.TotalMemory - actualMemory
	worker.AvailableStorage = worker.Info.TotalStorage - actualStorage

	worker.RunningTasks = actualTaskIDs

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

// ManualRegisterAndNotify registers a worker and immediately notifies it of master info
func (s *MasterServer) ManualRegisterAndNotify(ctx context.Context, workerID, workerIP, masterID, masterAddress string) error {
	if err := s.ManualRegisterWorker(ctx, workerID, workerIP); err != nil {
		return err
	}

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
	}()

	return nil
}

// StartWorkerReconnectionMonitor starts a background heartbeat check loop
func (s *MasterServer) StartWorkerReconnectionMonitor() {
	s.reconnectTicker = time.NewTicker(5 * time.Second)
	s.reconnectStop = make(chan bool)

	go func() {
		log.Println("🔄 Worker reconnection monitor started")
		for {
			select {
			case <-s.reconnectTicker.C:
				s.checkAndMarkInactiveWorkers()
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

func (s *MasterServer) checkAndMarkInactiveWorkers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	const heartbeatTimeout = 30

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

func (s *MasterServer) attemptWorkerReconnections() {
	s.mu.RLock()
	masterID := s.masterID
	masterAddress := s.masterAddress

	inactiveWorkers := make(map[string]string)
	for workerID, worker := range s.workers {
		if !worker.IsActive && worker.Info != nil && worker.Info.WorkerIp != "" {
			inactiveWorkers[workerID] = worker.Info.WorkerIp
		}
	}
	s.mu.RUnlock()

	if len(inactiveWorkers) > 0 {
		log.Printf("🔄 Attempting to reconnect to %d inactive worker(s)...", len(inactiveWorkers))
		for workerID, workerIP := range inactiveWorkers {
			go s.attemptSingleWorkerReconnection(workerID, workerIP, masterID, masterAddress)
		}
	}
}

func (s *MasterServer) attemptSingleWorkerReconnection(workerID, workerIP, masterID, masterAddress string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, workerIP,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		return
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	mi := &pb.MasterInfo{MasterId: masterID, MasterAddress: masterAddress}
	ack, err := client.MasterRegister(ctx, mi)
	if err != nil {
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

	if _, exists := s.workers[workerID]; !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	if s.workerDB != nil {
		if err := s.workerDB.UnregisterWorker(ctx, workerID); err != nil {
			return fmt.Errorf("unregister worker from db: %w", err)
		}
	}

	if s.telemetryManager != nil {
		s.telemetryManager.UnregisterWorker(workerID)
	}

	delete(s.workers, workerID)
	log.Printf("Unregistered worker: %s", workerID)
	return nil
}

// BroadcastMasterRegistration calls MasterRegister on all pre-registered workers
func (s *MasterServer) BroadcastMasterRegistration(masterID, masterAddress string) {
	s.mu.RLock()
	var workersToNotify []string
	for _, worker := range s.workers {
		if worker.Info != nil && worker.Info.WorkerIp != "" {
			workersToNotify = append(workersToNotify, worker.Info.WorkerIp)
		}
	}
	s.mu.RUnlock()

	log.Printf("📢 Broadcasting MasterRegister to %d workers...", len(workersToNotify))

	for _, workerIP := range workersToNotify {
		go func(ip string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			conn, err := grpc.DialContext(ctx, ip,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock())
			if err != nil {
				log.Printf("Failed to connect to worker at %s for MasterRegister: %v", ip, err)
				return
			}
			defer conn.Close()

			client := pb.NewMasterWorkerClient(conn)
			mi := &pb.MasterInfo{
				MasterId:      masterID,
				MasterAddress: masterAddress,
			}

			ack, err := client.MasterRegister(ctx, mi)
			if err != nil {
				log.Printf("MasterRegister to %s failed: %v", ip, err)
				return
			}

			if ack != nil && ack.Success {
				log.Printf("✓ Worker at %s acknowledged MasterRegister", ip)
			} else {
				log.Printf("Worker at %s rejected MasterRegister", ip)
			}
		}(workerIP)
	}
}
