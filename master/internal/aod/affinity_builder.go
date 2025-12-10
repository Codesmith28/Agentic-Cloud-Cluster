package aod

import (
	"log"
	"math"

	"master/internal/db"
)

// BuildAffinityMatrix computes affinity scores for each (taskType, workerID) pair
// based on historical performance metrics.
//
// Affinity represents how well a worker performs for a specific task type:
//   - Positive affinity: worker is preferred (fast, reliable)
//   - Negative affinity: worker should be avoided (slow, unreliable)
//
// Formula (NO WEIGHTS):
//
//	Affinity = SpeedAdvantage + SLAReliability
//	SpeedAdvantage = τ / worker_avg_runtime
//	SLAReliability = sla_success_count / completed_tasks
//
// Returns a nested map: map[taskType][workerID]float64
// Affinity values are clipped to [-5.0, +5.0] for numerical stability
func BuildAffinityMatrix(history []db.TaskHistory) map[string]map[string]float64 {
	affinity := make(map[string]map[string]float64)

	// Define the 6 standardized task types
	taskTypes := []string{
		"cpu-light",
		"cpu-heavy",
		"memory-heavy",
		"gpu-heavy", // Updated from gpu-inference
		"gpu-training",
		"mixed",
	}

	// Get all unique workers from history
	workers := getUniqueWorkers(history)

	// For each task type, compute affinity with each worker
	for _, taskType := range taskTypes {
		affinity[taskType] = make(map[string]float64)

		// Get baseline tau for this task type
		baselineTau := getAverageTau(taskType, history)
		if baselineTau <= 0 {
			baselineTau = getDefaultTauForType(taskType)
		}

		for _, workerID := range workers {
			// Filter history for this (taskType, workerID) pair
			pairHistory := filterHistory(history, taskType, workerID)

			// Need at least 2 data points for meaningful statistics
			if len(pairHistory) < 2 {
				// Default to neutral affinity (1.0) for insufficient data
				affinity[taskType][workerID] = 1.0
				continue
			}

			// Compute SpeedAdvantage = τ / worker_avg_runtime
			workerAvgRuntime := computeWorkerAvgRuntime(taskType, workerID, history)
			speedAdvantage := 0.0
			if workerAvgRuntime > 0 {
				speedAdvantage = baselineTau / workerAvgRuntime
			}

			// Compute SLAReliability = sla_success_count / completed_tasks
			slaSuccessCount := 0
			for _, record := range pairHistory {
				if record.SLASuccess {
					slaSuccessCount++
				}
			}
			slaReliability := float64(slaSuccessCount) / float64(len(pairHistory))

			// Compute affinity: NO WEIGHTS, direct sum
			rawAffinity := speedAdvantage + slaReliability

			// Clip to [-5, +5] for numerical stability
			clippedAffinity := math.Max(-5.0, math.Min(5.0, rawAffinity))

			affinity[taskType][workerID] = clippedAffinity

			log.Printf("📊 Affinity[%s][%s] = %.3f (speed=%.3f, SLA=%.3f)",
				taskType, workerID, clippedAffinity, speedAdvantage, slaReliability)
		}
	}

	return affinity
}

// getAverageTau computes the average tau (baseline runtime) for a task type
func getAverageTau(taskType string, history []db.TaskHistory) float64 {
	filtered := filterHistoryByType(history, taskType)
	if len(filtered) == 0 {
		return 0.0
	}

	totalTau := 0.0
	for _, record := range filtered {
		totalTau += record.Tau
	}

	return totalTau / float64(len(filtered))
}

// getDefaultTauForType returns default tau values per task type
func getDefaultTauForType(taskType string) float64 {
	switch taskType {
	case "cpu-light":
		return 5.0
	case "cpu-heavy":
		return 60.0
	case "memory-heavy":
		return 30.0
	case "gpu-heavy":
		return 45.0
	case "gpu-training":
		return 120.0
	case "mixed":
		return 20.0
	default:
		return 30.0
	}
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
