package aod

import (
	"log"
	"math"

	"master/internal/db"
	"master/internal/scheduler"
)

// BuildAffinityMatrix computes affinity scores for each (taskType, workerID) pair
// based on historical performance metrics.
//
// Affinity represents how well a worker performs for a specific task type:
//   - Positive affinity: worker is preferred (fast, reliable)
//   - Negative affinity: worker should be avoided (slow, unreliable)
//
// Formula (EDD §5.3):
//
//	A[taskType][workerID] = a1*speed + a2*SLA_reliability - a3*overload_rate
//
// Returns a nested map: map[taskType][workerID]float64
// Affinity values are clipped to [-5.0, +5.0] for numerical stability
func BuildAffinityMatrix(history []db.TaskHistory, weights scheduler.AffinityWeights) map[string]map[string]float64 {
	affinity := make(map[string]map[string]float64)

	// Define the 6 standardized task types
	taskTypes := []string{
		"cpu-light",
		"cpu-heavy",
		"memory-heavy",
		"gpu-inference",
		"gpu-training",
		"mixed",
	}

	// Get all unique workers from history
	workers := getUniqueWorkers(history)

	// For each task type, compute affinity with each worker
	for _, taskType := range taskTypes {
		affinity[taskType] = make(map[string]float64)

		for _, workerID := range workers {
			// Filter history for this (taskType, workerID) pair
			pairHistory := filterHistory(history, taskType, workerID)

			// Need at least 3 data points for meaningful statistics
			if len(pairHistory) < 3 {
				// Default to neutral affinity (0.0) for insufficient data
				affinity[taskType][workerID] = 0.0
				continue
			}

			// Compute the three affinity components
			speed := computeSpeed(taskType, workerID, history)
			slaReliability := computeSLAReliability(taskType, workerID, pairHistory)
			overloadRate := computeOverloadRate(taskType, workerID, pairHistory)

			// Compute raw affinity
			rawAffinity := weights.A1*speed + weights.A2*slaReliability - weights.A3*overloadRate

			// Clip to [-5, +5] for numerical stability
			clippedAffinity := math.Max(-5.0, math.Min(5.0, rawAffinity))

			affinity[taskType][workerID] = clippedAffinity

			log.Printf("📊 Affinity[%s][%s] = %.3f (speed=%.3f, SLA=%.3f, overload=%.3f)",
				taskType, workerID, clippedAffinity, speed, slaReliability, overloadRate)
		}
	}

	return affinity
}

// computeSpeed calculates how fast a worker executes a specific task type
// relative to the baseline (average across all workers for that type).
//
// Formula:
//
//	speed = baseline_runtime / worker_avg_runtime
//
// Returns:
//   - > 1.0: worker is faster than average (positive affinity component)
//   - < 1.0: worker is slower than average (negative affinity component)
//   - 0.0: insufficient data or worker never ran this task type
func computeSpeed(taskType, workerID string, history []db.TaskHistory) float64 {
	// Get baseline runtime for this task type (average across all workers)
	baseline := computeBaselineRuntime(taskType, history)
	if baseline <= 0 {
		return 0.0 // No data for this task type
	}

	// Get average runtime on this specific worker
	workerRuntime := computeWorkerAvgRuntime(taskType, workerID, history)
	if workerRuntime <= 0 {
		return 0.0 // Worker never ran this task type
	}

	// Speed ratio: baseline / worker_runtime
	// If worker is faster, ratio > 1.0
	// If worker is slower, ratio < 1.0
	speed := baseline / workerRuntime

	return speed
}

// computeSLAReliability calculates the SLA success rate for a (taskType, workerID) pair.
//
// Formula:
//
//	SLA_reliability = 1 - (violations / total_tasks)
//
// Returns:
//   - 1.0: perfect SLA compliance (all tasks met deadline)
//   - 0.0: all tasks violated SLA
//   - 0.5: half of tasks violated SLA
func computeSLAReliability(taskType, workerID string, pairHistory []db.TaskHistory) float64 {
	if len(pairHistory) == 0 {
		return 0.0
	}

	violations := 0
	for _, record := range pairHistory {
		if !record.SLASuccess {
			violations++
		}
	}

	// SLA reliability = success rate
	reliability := 1.0 - (float64(violations) / float64(len(pairHistory)))

	return reliability
}

// computeOverloadRate calculates how often a worker was overloaded when executing
// a specific task type.
//
// Formula:
//
//	overload_rate = avg(LoadAtStart) for this (taskType, workerID) pair
//
// Returns:
//   - 0.0: worker was never loaded (bad - underutilized)
//   - 0.5: worker was moderately loaded (good)
//   - 1.0: worker was always at max capacity (bad - overloaded)
func computeOverloadRate(taskType, workerID string, pairHistory []db.TaskHistory) float64 {
	if len(pairHistory) == 0 {
		return 0.0
	}

	totalLoad := 0.0
	for _, record := range pairHistory {
		totalLoad += record.LoadAtStart
	}

	avgLoad := totalLoad / float64(len(pairHistory))

	return avgLoad
}

// computeBaselineRuntime computes the average runtime for a task type across ALL workers.
// This serves as the reference point for computing speed ratios.
func computeBaselineRuntime(taskType string, history []db.TaskHistory) float64 {
	filtered := filterHistoryByType(history, taskType)
	if len(filtered) == 0 {
		return 0.0
	}

	totalRuntime := 0.0
	for _, record := range filtered {
		totalRuntime += record.ActualRuntime
	}

	return totalRuntime / float64(len(filtered))
}

// computeWorkerAvgRuntime computes the average runtime for a specific (taskType, workerID) pair.
func computeWorkerAvgRuntime(taskType, workerID string, history []db.TaskHistory) float64 {
	filtered := filterHistory(history, taskType, workerID)
	if len(filtered) == 0 {
		return 0.0
	}

	totalRuntime := 0.0
	for _, record := range filtered {
		totalRuntime += record.ActualRuntime
	}

	return totalRuntime / float64(len(filtered))
}

// filterHistory returns TaskHistory records matching both taskType and workerID
func filterHistory(history []db.TaskHistory, taskType, workerID string) []db.TaskHistory {
	var filtered []db.TaskHistory
	for _, record := range history {
		if record.Type == taskType && record.WorkerID == workerID {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

// filterHistoryByType returns TaskHistory records matching a specific task type
func filterHistoryByType(history []db.TaskHistory, taskType string) []db.TaskHistory {
	var filtered []db.TaskHistory
	for _, record := range history {
		if record.Type == taskType {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

// getUniqueWorkers extracts all unique worker IDs from history
func getUniqueWorkers(history []db.TaskHistory) []string {
	workerSet := make(map[string]bool)
	for _, record := range history {
		workerSet[record.WorkerID] = true
	}

	workers := make([]string, 0, len(workerSet))
	for workerID := range workerSet {
		workers = append(workers, workerID)
	}

	return workers
}
