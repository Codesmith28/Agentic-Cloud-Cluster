package scheduler

import (
	"context"
	"fmt"

	"master/internal/db"
	"master/internal/telemetry"
)

// WorkerDBInterface defines the interface for accessing worker capacity data
type WorkerDBInterface interface {
	GetWorker(ctx context.Context, workerID string) (*db.WorkerDocument, error)
	GetAllWorkers(ctx context.Context) ([]db.WorkerDocument, error)
}

// TelemetrySource provides an interface for the RTS scheduler to access worker state
// It bridges the gap between TelemetryManager (real-time usage) and WorkerDB (capacity)
type TelemetrySource interface {
	// GetWorkerViews returns the current state of all active workers
	// Each WorkerView contains:
	//   - Available resources (Total - Allocated from DB)
	//   - Current load (normalized usage from telemetry)
	GetWorkerViews(ctx context.Context) ([]WorkerView, error)

	// GetWorkerLoad returns the normalized load for a specific worker
	// Returns 0.0 if worker not found or inactive
	GetWorkerLoad(workerID string) float64
}

// MasterTelemetrySource implements TelemetrySource using the master's telemetry and worker DB
type MasterTelemetrySource struct {
	telemetryMgr *telemetry.TelemetryManager
	workerDB     WorkerDBInterface
}

// NewMasterTelemetrySource creates a new telemetry source for the RTS scheduler
func NewMasterTelemetrySource(telemetryMgr *telemetry.TelemetryManager, workerDB WorkerDBInterface) *MasterTelemetrySource {
	return &MasterTelemetrySource{
		telemetryMgr: telemetryMgr,
		workerDB:     workerDB,
	}
}

// GetWorkerViews returns the current state of all active workers
func (mts *MasterTelemetrySource) GetWorkerViews(ctx context.Context) ([]WorkerView, error) {
	// Get all workers from DB (for capacity information)
	workers, err := mts.workerDB.GetAllWorkers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get workers from DB: %w", err)
	}

	// Get all telemetry data (for usage information)
	telemetryData := mts.telemetryMgr.GetAllWorkerTelemetry()

	// Build WorkerView for each active worker
	var views []WorkerView
	for _, worker := range workers {
		// Only include active workers
		if !worker.IsActive {
			continue
		}

		// Compute available resources (Total - Allocated)
		cpuAvail := worker.TotalCPU - worker.AllocatedCPU
		memAvail := worker.TotalMemory - worker.AllocatedMemory
		gpuAvail := worker.TotalGPU - worker.AllocatedGPU
		storageAvail := worker.TotalStorage - worker.AllocatedStorage

		// Ensure non-negative values (in case of oversubscription)
		if cpuAvail < 0 {
			cpuAvail = 0
		}
		if memAvail < 0 {
			memAvail = 0
		}
		if gpuAvail < 0 {
			gpuAvail = 0
		}
		if storageAvail < 0 {
			storageAvail = 0
		}

		// Compute normalized load from telemetry data
		load := mts.computeNormalizedLoad(worker.WorkerID, telemetryData, &worker)

		view := WorkerView{
			ID:           worker.WorkerID,
			CPUAvail:     cpuAvail,
			MemAvail:     memAvail,
			GPUAvail:     gpuAvail,
			StorageAvail: storageAvail,
			Load:         load,
		}

		views = append(views, view)
	}

	return views, nil
}

// GetWorkerLoad returns the normalized load for a specific worker
func (mts *MasterTelemetrySource) GetWorkerLoad(workerID string) float64 {
	// Get telemetry data for this worker
	telData, exists := mts.telemetryMgr.GetWorkerTelemetry(workerID)
	if !exists || !telData.IsActive {
		return 0.0
	}

	// Get worker capacity from DB
	worker, err := mts.workerDB.GetWorker(context.Background(), workerID)
	if err != nil || worker == nil {
		return 0.0
	}

	// Get all telemetry data and compute load
	telemetryData := mts.telemetryMgr.GetAllWorkerTelemetry()
	return mts.computeNormalizedLoad(workerID, telemetryData, worker)
}

// computeNormalizedLoad computes the normalized load for a worker
// Load is based on the weighted combination of CPU, Memory, and GPU usage
// Formula: Load = (w_cpu * CPU_usage + w_mem * Mem_usage + w_gpu * GPU_usage) / (w_cpu + w_mem + w_gpu)
// where weights are proportional to the resource capacities
func (mts *MasterTelemetrySource) computeNormalizedLoad(
	workerID string,
	telemetryData map[string]*telemetry.WorkerTelemetryData,
	worker *db.WorkerDocument,
) float64 {
	// Get telemetry data
	telData, exists := telemetryData[workerID]
	if !exists || !telData.IsActive {
		return 0.0
	}

	// Extract usage percentages from telemetry
	cpuUsage := telData.CpuUsage    // 0.0 to 1.0 (percentage)
	memUsage := telData.MemoryUsage // 0.0 to 1.0 (percentage)
	gpuUsage := telData.GpuUsage    // 0.0 to 1.0 (percentage)

	// Compute weights based on resource capacities
	// This ensures resources with higher capacity have more influence on the load metric
	wCPU := worker.TotalCPU
	wMem := worker.TotalMemory / 10.0 // Scale down memory to be comparable to CPU
	wGPU := worker.TotalGPU * 2.0     // Scale up GPU to emphasize its importance

	// Handle edge case: no resources
	totalWeight := wCPU + wMem + wGPU
	if totalWeight == 0 {
		return 0.0
	}

	// Compute weighted average load
	load := (wCPU*cpuUsage + wMem*memUsage + wGPU*gpuUsage) / totalWeight

	// Clamp to [0.0, inf) - can exceed 1.0 if oversubscribed
	if load < 0 {
		load = 0.0
	}

	return load
}
