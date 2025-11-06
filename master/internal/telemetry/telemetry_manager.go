package telemetry

import (
	"context"
	"log"
	"sync"
	"time"

	pb "master/proto"
)

// WorkerTelemetryData holds the latest telemetry information for a worker
type WorkerTelemetryData struct {
	WorkerID     string
	CpuUsage     float64
	MemoryUsage  float64
	GpuUsage     float64
	RunningTasks []*pb.RunningTask
	LastUpdate   int64
	IsActive     bool
}

// TelemetryManager manages telemetry reception from multiple workers
// Each worker gets its own goroutine to process heartbeats
type TelemetryManager struct {
	// Map of worker ID to their telemetry data
	workerData map[string]*WorkerTelemetryData
	mu         sync.RWMutex

	// Map of worker ID to their heartbeat channel
	workerChannels map[string]chan *pb.Heartbeat
	channelMu      sync.RWMutex

	// Callback to notify when telemetry is updated (optional)
	onUpdate func(workerID string, data *WorkerTelemetryData)

	// Context for managing goroutines
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Timeout for marking workers as inactive
	inactivityTimeout time.Duration
	
	// Quiet mode suppresses verbose logging
	quietMode bool
}

// NewTelemetryManager creates a new telemetry manager
func NewTelemetryManager(inactivityTimeout time.Duration) *TelemetryManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TelemetryManager{
		workerData:        make(map[string]*WorkerTelemetryData),
		workerChannels:    make(map[string]chan *pb.Heartbeat),
		ctx:               ctx,
		cancel:            cancel,
		inactivityTimeout: inactivityTimeout,
		quietMode:         true, // Enable quiet mode by default to not interfere with CLI
	}
}

// SetQuietMode enables or disables verbose logging
func (tm *TelemetryManager) SetQuietMode(quiet bool) {
	tm.quietMode = quiet
}

// SetUpdateCallback sets a callback function to be called when telemetry is updated
func (tm *TelemetryManager) SetUpdateCallback(callback func(workerID string, data *WorkerTelemetryData)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.onUpdate = callback
}

// RegisterWorker registers a new worker and starts a dedicated goroutine for its telemetry
func (tm *TelemetryManager) RegisterWorker(workerID string) {
	tm.channelMu.Lock()
	defer tm.channelMu.Unlock()

	// Check if worker already has a channel
	if _, exists := tm.workerChannels[workerID]; exists {
		log.Printf("Worker %s already registered for telemetry", workerID)
		return
	}

	// Create channel for this worker's heartbeats
	heartbeatChan := make(chan *pb.Heartbeat, 10) // Buffered channel
	tm.workerChannels[workerID] = heartbeatChan

	// Initialize worker data
	tm.mu.Lock()
	tm.workerData[workerID] = &WorkerTelemetryData{
		WorkerID:   workerID,
		IsActive:   false,
		LastUpdate: time.Now().Unix(),
	}
	tm.mu.Unlock()

	// Start dedicated goroutine for this worker
	tm.wg.Add(1)
	go tm.processWorkerTelemetry(workerID, heartbeatChan)

	if !tm.quietMode {
		log.Printf("Telemetry manager: Registered worker %s with dedicated thread", workerID)
	}
}

// UnregisterWorker removes a worker and stops its telemetry goroutine
func (tm *TelemetryManager) UnregisterWorker(workerID string) {
	tm.channelMu.Lock()
	defer tm.channelMu.Unlock()

	// Close the channel (this will cause the goroutine to exit)
	if ch, exists := tm.workerChannels[workerID]; exists {
		close(ch)
		delete(tm.workerChannels, workerID)
		log.Printf("Telemetry manager: Unregistered worker %s", workerID)
	}

	// Remove worker data
	tm.mu.Lock()
	delete(tm.workerData, workerID)
	tm.mu.Unlock()
}

// ProcessHeartbeat receives a heartbeat and forwards it to the worker's dedicated thread
func (tm *TelemetryManager) ProcessHeartbeat(hb *pb.Heartbeat) error {
	tm.channelMu.RLock()
	ch, exists := tm.workerChannels[hb.WorkerId]
	tm.channelMu.RUnlock()

	if !exists {
		// Worker not registered, auto-register it
		tm.RegisterWorker(hb.WorkerId)

		// Get the channel again after registration
		tm.channelMu.RLock()
		ch = tm.workerChannels[hb.WorkerId]
		tm.channelMu.RUnlock()
	}

	// Send heartbeat to worker's dedicated thread (non-blocking)
	select {
	case ch <- hb:
		return nil
	default:
		log.Printf("Warning: Heartbeat channel full for worker %s, dropping heartbeat", hb.WorkerId)
		return nil
	}
}

// processWorkerTelemetry is the dedicated goroutine for processing a single worker's telemetry
func (tm *TelemetryManager) processWorkerTelemetry(workerID string, heartbeatChan <-chan *pb.Heartbeat) {
	defer tm.wg.Done()
	if !tm.quietMode {
		log.Printf("Started telemetry processing thread for worker %s", workerID)
	}

	for {
		select {
		case hb, ok := <-heartbeatChan:
			if !ok {
				// Channel closed, worker unregistered
				if !tm.quietMode {
					log.Printf("Telemetry thread for worker %s shutting down", workerID)
				}
				return
			}

			// Process heartbeat
			tm.updateWorkerTelemetry(hb)

		case <-tm.ctx.Done():
			// Manager shutting down
			if !tm.quietMode {
				log.Printf("Telemetry thread for worker %s shutting down (manager closed)", workerID)
			}
			return
		}
	}
}

// updateWorkerTelemetry updates the telemetry data for a worker
func (tm *TelemetryManager) updateWorkerTelemetry(hb *pb.Heartbeat) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	data, exists := tm.workerData[hb.WorkerId]
	if !exists {
		data = &WorkerTelemetryData{
			WorkerID: hb.WorkerId,
		}
		tm.workerData[hb.WorkerId] = data
	}

	// Update telemetry data
	data.CpuUsage = hb.CpuUsage
	data.MemoryUsage = hb.MemoryUsage
	data.GpuUsage = hb.GpuUsage
	data.RunningTasks = hb.RunningTasks
	data.LastUpdate = time.Now().Unix()
	data.IsActive = true

	// Call callback if set
	if tm.onUpdate != nil {
		// Call callback without holding lock to avoid deadlocks
		go tm.onUpdate(hb.WorkerId, data)
	}
}

// GetWorkerTelemetry returns the latest telemetry data for a specific worker
func (tm *TelemetryManager) GetWorkerTelemetry(workerID string) (*WorkerTelemetryData, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	data, exists := tm.workerData[workerID]
	if !exists {
		return nil, false
	}

	// Create a copy to avoid race conditions
	dataCopy := &WorkerTelemetryData{
		WorkerID:     data.WorkerID,
		CpuUsage:     data.CpuUsage,
		MemoryUsage:  data.MemoryUsage,
		GpuUsage:     data.GpuUsage,
		RunningTasks: make([]*pb.RunningTask, len(data.RunningTasks)),
		LastUpdate:   data.LastUpdate,
		IsActive:     data.IsActive,
	}
	copy(dataCopy.RunningTasks, data.RunningTasks)

	return dataCopy, true
}

// GetAllWorkerTelemetry returns telemetry data for all workers
func (tm *TelemetryManager) GetAllWorkerTelemetry() map[string]*WorkerTelemetryData {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*WorkerTelemetryData)
	for id, data := range tm.workerData {
		// Create a copy
		dataCopy := &WorkerTelemetryData{
			WorkerID:     data.WorkerID,
			CpuUsage:     data.CpuUsage,
			MemoryUsage:  data.MemoryUsage,
			GpuUsage:     data.GpuUsage,
			RunningTasks: make([]*pb.RunningTask, len(data.RunningTasks)),
			LastUpdate:   data.LastUpdate,
			IsActive:     data.IsActive,
		}
		copy(dataCopy.RunningTasks, data.RunningTasks)
		result[id] = dataCopy
	}

	return result
}

// Start begins the telemetry manager (starts inactivity checker)
func (tm *TelemetryManager) Start() {
	tm.wg.Add(1)
	go tm.checkInactivity()
	log.Println("Telemetry manager started")
}

// checkInactivity periodically checks for inactive workers
func (tm *TelemetryManager) checkInactivity() {
	defer tm.wg.Done()

	ticker := time.NewTicker(tm.inactivityTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.markInactiveWorkers()
		case <-tm.ctx.Done():
			return
		}
	}
}

// markInactiveWorkers marks workers as inactive if they haven't sent heartbeat recently
func (tm *TelemetryManager) markInactiveWorkers() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now().Unix()
	for id, data := range tm.workerData {
		if now-data.LastUpdate > int64(tm.inactivityTimeout.Seconds()) {
			if data.IsActive {
				data.IsActive = false
				log.Printf("Worker %s marked as inactive (no heartbeat for %v)", id, tm.inactivityTimeout)
			}
		}
	}
}

// Shutdown gracefully shuts down the telemetry manager
func (tm *TelemetryManager) Shutdown() {
	log.Println("Shutting down telemetry manager...")

	// Cancel context to stop all goroutines
	tm.cancel()

	// Close all worker channels
	tm.channelMu.Lock()
	for id, ch := range tm.workerChannels {
		close(ch)
		delete(tm.workerChannels, id)
	}
	tm.channelMu.Unlock()

	// Wait for all goroutines to finish
	tm.wg.Wait()

	log.Println("Telemetry manager shutdown complete")
}

// GetWorkerCount returns the number of registered workers
func (tm *TelemetryManager) GetWorkerCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.workerData)
}

// GetActiveWorkerCount returns the number of active workers
func (tm *TelemetryManager) GetActiveWorkerCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	count := 0
	for _, data := range tm.workerData {
		if data.IsActive {
			count++
		}
	}
	return count
}
