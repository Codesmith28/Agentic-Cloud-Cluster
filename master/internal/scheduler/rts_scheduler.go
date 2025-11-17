package scheduler

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"master/internal/telemetry"
	pb "master/proto"
)

// RTSScheduler implements Risk-aware Task Scheduling with GA-optimized parameters
type RTSScheduler struct {
	// Fallback scheduler (Round-Robin)
	rrScheduler Scheduler

	// Tau store for runtime estimates
	tauStore telemetry.TauStore

	// Telemetry source for worker state
	telemetrySource TelemetrySource

	// GA parameters (thread-safe)
	params     *GAParams
	paramsMu   sync.RWMutex
	paramsPath string

	// SLA multiplier (k factor)
	slaMultiplier float64

	// Context for background tasks
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRTSScheduler creates a new RTS scheduler with fallback to Round-Robin
func NewRTSScheduler(
	rrScheduler Scheduler,
	tauStore telemetry.TauStore,
	telemetrySource TelemetrySource,
	paramsPath string,
	slaMultiplier float64,
) *RTSScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &RTSScheduler{
		rrScheduler:     rrScheduler,
		tauStore:        tauStore,
		telemetrySource: telemetrySource,
		paramsPath:      paramsPath,
		slaMultiplier:   slaMultiplier,
		ctx:             ctx,
		cancel:          cancel,
	}

	// Load initial parameters (or defaults if file doesn't exist)
	s.params = LoadGAParamsOrDefault(paramsPath)
	log.Printf("✓ RTS Scheduler initialized with params from %s", paramsPath)

	// Start background params reloader
	s.startParamsReloader()

	return s
}

// GetName returns the scheduler name
func (s *RTSScheduler) GetName() string {
	return "RTS"
}

// Reset resets internal state (useful for testing)
func (s *RTSScheduler) Reset() {
	// RTS is stateless, but reset fallback scheduler
	s.rrScheduler.Reset()
}

// Shutdown gracefully stops the scheduler
func (s *RTSScheduler) Shutdown() {
	s.cancel()
}

// SelectWorker implements the RTS scheduling algorithm (EDD §3.9)
func (s *RTSScheduler) SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string {
	// Step 1: Build TaskView from pb.Task
	now := time.Now()
	taskView := s.buildTaskView(task, now)

	// Step 2: Build WorkerViews from workers + telemetry
	workerViews := s.buildWorkerViews(workers)

	// Step 3: Filter feasible workers
	feasibleWorkers := s.filterFeasible(taskView, workerViews)

	if len(feasibleWorkers) == 0 {
		log.Printf("⚠️ RTS: No feasible workers for task %s (type=%s), falling back to Round-Robin",
			task.TaskId, taskView.Type)
		return s.rrScheduler.SelectWorker(task, workers)
	}

	// Step 4: Load GA parameters (thread-safe)
	params := s.getGAParamsSafe()

	// Step 5: Compute risk for each feasible worker and select best
	bestWorkerID := ""
	bestRisk := math.Inf(1) // Start with positive infinity

	for _, workerView := range feasibleWorkers {
		// Predict execution time (EDD §3.5)
		eHat := s.predictExecTime(taskView, workerView, params.Theta)

		// Compute base risk (EDD §3.7)
		baseRisk := s.computeBaseRisk(taskView, workerView, eHat, params.Risk.Alpha, params.Risk.Beta)

		// Compute final risk with affinity and penalty (EDD §3.8)
		finalRisk := s.computeFinalRisk(baseRisk, taskView.Type, workerView.ID, params)

		// Track best worker (lowest risk)
		if finalRisk < bestRisk && !math.IsInf(finalRisk, 0) && !math.IsNaN(finalRisk) {
			bestRisk = finalRisk
			bestWorkerID = workerView.ID
		}
	}

	// Step 6: Validate result and fallback if needed
	if bestWorkerID == "" || math.IsInf(bestRisk, 0) || math.IsNaN(bestRisk) {
		log.Printf("⚠️ RTS: Invalid risk scores for task %s, falling back to Round-Robin", task.TaskId)
		return s.rrScheduler.SelectWorker(task, workers)
	}

	log.Printf("✓ RTS: Selected worker %s for task %s (type=%s, risk=%.2f)",
		bestWorkerID, task.TaskId, taskView.Type, bestRisk)

	return bestWorkerID
}

// buildTaskView constructs a TaskView from a protobuf Task
func (s *RTSScheduler) buildTaskView(task *pb.Task, now time.Time) TaskView {
	// Use NewTaskViewFromProto which handles task type inference
	tau := s.tauStore.GetTau(task.TaskType)
	if tau == 0 {
		// If task type not set, infer it first
		taskType := InferTaskType(task)
		tau = s.tauStore.GetTau(taskType)
	}

	return NewTaskViewFromProto(task, now, tau, s.slaMultiplier)
}

// buildWorkerViews constructs WorkerViews from available workers and telemetry
func (s *RTSScheduler) buildWorkerViews(workers map[string]*WorkerInfo) []WorkerView {
	// Get all worker views from telemetry source
	allViews, err := s.telemetrySource.GetWorkerViews(context.Background())
	if err != nil {
		log.Printf("⚠️ RTS: Failed to get worker views: %v", err)
		return []WorkerView{}
	}

	// Filter to only include workers in the provided map
	filtered := make([]WorkerView, 0, len(allViews))
	for _, view := range allViews {
		if workerInfo, exists := workers[view.ID]; exists && workerInfo.IsActive {
			filtered = append(filtered, view)
		}
	}

	return filtered
}

// filterFeasible filters workers that can accommodate the task (EDD §3.3)
func (s *RTSScheduler) filterFeasible(task TaskView, workers []WorkerView) []WorkerView {
	feasible := make([]WorkerView, 0, len(workers))

	for _, worker := range workers {
		// Check resource constraints
		if worker.CPUAvail >= task.CPU &&
			worker.MemAvail >= task.Mem &&
			worker.GPUAvail >= task.GPU &&
			worker.StorageAvail >= task.Storage {
			feasible = append(feasible, worker)
		}
	}

	return feasible
}

// predictExecTime predicts execution time for task on worker (EDD §3.5)
// Formula: E_hat = tau * (1 + theta1*(C/C_avail) + theta2*(M/M_avail) + theta3*(G/G_avail) + theta4*L)
func (s *RTSScheduler) predictExecTime(t TaskView, w WorkerView, theta Theta) float64 {
	// Base runtime
	eHat := t.Tau

	// CPU ratio term
	cpuRatio := 0.0
	if w.CPUAvail > 0 {
		cpuRatio = t.CPU / w.CPUAvail
	} else if t.CPU > 0 {
		cpuRatio = 1.0 // Full utilization if no availability but task needs it
	}

	// Memory ratio term
	memRatio := 0.0
	if w.MemAvail > 0 {
		memRatio = t.Mem / w.MemAvail
	} else if t.Mem > 0 {
		memRatio = 1.0
	}

	// GPU ratio term
	gpuRatio := 0.0
	if w.GPUAvail > 0 {
		gpuRatio = t.GPU / w.GPUAvail
	} else if t.GPU > 0 {
		gpuRatio = 1.0
	}

	// Load term
	load := w.Load

	// Apply formula
	multiplier := 1.0 + theta.Theta1*cpuRatio + theta.Theta2*memRatio + theta.Theta3*gpuRatio + theta.Theta4*load
	eHat *= multiplier

	// Ensure positive result
	if eHat < 0 {
		eHat = t.Tau
	}

	return eHat
}

// computeBaseRisk computes base risk score (EDD §3.7)
// Formula: R_base = alpha * delta + beta * L
// where delta = max(0, f_hat - deadline)
func (s *RTSScheduler) computeBaseRisk(t TaskView, w WorkerView, eHat float64, alpha, beta float64) float64 {
	// Predicted finish time
	fHat := t.ArrivalTime.Add(time.Duration(eHat * float64(time.Second)))

	// Deadline violation delta (in seconds)
	delta := 0.0
	if fHat.After(t.Deadline) {
		delta = fHat.Sub(t.Deadline).Seconds()
	}

	// Base risk = deadline penalty + load penalty
	baseRisk := alpha*delta + beta*w.Load

	return baseRisk
}

// computeFinalRisk applies affinity and penalty adjustments (EDD §3.8)
// Formula: R_final = R_base - affinity(task_type, worker) + penalty(worker)
func (s *RTSScheduler) computeFinalRisk(baseRisk float64, taskType string, workerID string, params *GAParams) float64 {
	// Get affinity for this (task type, worker) pair
	affinity := 0.0
	if params.AffinityMatrix != nil {
		if workerMap, ok := params.AffinityMatrix[taskType]; ok {
			if aff, ok := workerMap[workerID]; ok {
				affinity = aff
			}
		}
	}

	// Get penalty for this worker
	penalty := 0.0
	if params.PenaltyVector != nil {
		if pen, ok := params.PenaltyVector[workerID]; ok {
			penalty = pen
		}
	}

	// Final risk = base risk - affinity (reward) + penalty
	finalRisk := baseRisk - affinity + penalty

	return finalRisk
}

// getGAParamsSafe returns a thread-safe copy of GA parameters
func (s *RTSScheduler) getGAParamsSafe() *GAParams {
	s.paramsMu.RLock()
	defer s.paramsMu.RUnlock()
	return s.params
}

// startParamsReloader starts a background goroutine to reload GA parameters periodically
func (s *RTSScheduler) startParamsReloader() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Reload parameters from file
				newParams := LoadGAParamsOrDefault(s.paramsPath)

				// Update with write lock
				s.paramsMu.Lock()
				s.params = newParams
				s.paramsMu.Unlock()

				log.Printf("✓ RTS: Reloaded GA parameters from %s", s.paramsPath)

			case <-s.ctx.Done():
				return
			}
		}
	}()
}
