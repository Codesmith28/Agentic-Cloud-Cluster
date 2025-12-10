package aod

import (
	"log"
	"math"

	"master/internal/db"
)

// BuildPenaltyVector computes penalty scores for each worker based on
// their historical performance and reliability metrics.
//
// Penalty represents characteristics that should discourage task assignment:
//   - High SLA failure rate: worker frequently misses deadlines
//   - High overload rate: worker is frequently oversubscribed
//   - High energy consumption: worker is inefficient
//
// Formula (SIMPLIFIED - NO WEIGHTS):
//
//	P[workerID] = SLA_fail_rate + overload_rate + energy_norm
//
// Returns: map[workerID]penalty
// Penalty values are clipped to [0.0, 5.0] for numerical stability
func BuildPenaltyVector(workerStats []db.WorkerStats) map[string]float64 {
	penalty := make(map[string]float64)

	// Need to compute normalized energy values across all workers
	maxEnergy := findMaxEnergy(workerStats)
	if maxEnergy <= 0 {
		maxEnergy = 1.0 // Prevent division by zero
	}

	for _, stats := range workerStats {
		// Skip workers with no data
		if stats.TasksRun == 0 {
			penalty[stats.WorkerID] = 0.0
			continue
		}

		// Compute the three penalty components
		slaFailRate := computeSLAFailRate(stats)
		overloadRate := computeWorkerOverloadRate(stats)
		energyNorm := computeEnergyNorm(stats, maxEnergy)

		// Compute raw penalty: NO WEIGHTS, direct sum
		rawPenalty := slaFailRate + overloadRate + energyNorm

		// Clip to [0, 5] for numerical stability (penalties should be non-negative)
		clippedPenalty := math.Max(0.0, math.Min(5.0, rawPenalty))

		penalty[stats.WorkerID] = clippedPenalty

		log.Printf("⚠️  Penalty[%s] = %.3f (SLA_fail=%.3f, overload=%.3f, energy=%.3f)",
			stats.WorkerID, clippedPenalty, slaFailRate, overloadRate, energyNorm)
	}

	return penalty
}

// computeSLAFailRate calculates the fraction of tasks that violated SLA deadline
//
// Formula:
//
//	SLA_fail_rate = SLA_violations / tasks_run
//
// Returns:
//   - Range: [0.0, 1.0]
//   - 0.0: perfect SLA compliance (no violations)
//   - 1.0: all tasks missed their deadline
func computeSLAFailRate(stats db.WorkerStats) float64 {
	if stats.TasksRun == 0 {
		return 0.0
	}

	failRate := float64(stats.SLAViolations) / float64(stats.TasksRun)

	// Ensure within valid range
	return math.Max(0.0, math.Min(1.0, failRate))
}

// computeWorkerOverloadRate calculates the fraction of time the worker spent overloaded
//
// Formula:
//
//	overload_rate = overload_time / total_time
//
// Returns:
//   - Range: [0.0, 1.0]
//   - 0.0: worker was never overloaded
//   - 1.0: worker was constantly overloaded
func computeWorkerOverloadRate(stats db.WorkerStats) float64 {
	if stats.TotalTime <= 0 {
		return 0.0
	}

	overloadRate := stats.OverloadTime / stats.TotalTime

	// Ensure within valid range
	return math.Max(0.0, math.Min(1.0, overloadRate))
}

// computeEnergyNorm calculates normalized energy consumption for a worker
//
// Energy is estimated as the sum of resource-seconds used:
//   - CPU-seconds + Memory-GB-seconds + GPU-seconds
//
// Formula:
//
//	energy = (CPU_used_total + Mem_used_total + GPU_used_total) / max_energy
//
// Returns:
//   - Range: [0.0, 1.0]
//   - Lower values indicate more energy-efficient workers
func computeEnergyNorm(stats db.WorkerStats, maxEnergy float64) float64 {
	if maxEnergy <= 0 {
		return 0.0
	}

	// Aggregate energy metric: sum of all resource-seconds
	energy := stats.CPUUsedTotal + stats.MemUsedTotal + stats.GPUUsedTotal

	// Normalize by maximum energy across all workers
	energyNorm := energy / maxEnergy

	// Ensure within valid range
	return math.Max(0.0, math.Min(1.0, energyNorm))
}

// findMaxEnergy finds the maximum energy consumption across all workers
// This is used for normalization in computeEnergyNorm
func findMaxEnergy(workerStats []db.WorkerStats) float64 {
	maxEnergy := 0.0

	for _, stats := range workerStats {
		energy := stats.CPUUsedTotal + stats.MemUsedTotal + stats.GPUUsedTotal
		if energy > maxEnergy {
			maxEnergy = energy
		}
	}

	return maxEnergy
}

// GetDefaultPenaltyVector returns a penalty vector with all zeros (no penalties)
// This is used when there is insufficient historical data
func GetDefaultPenaltyVector(workerIDs []string) map[string]float64 {
	penalty := make(map[string]float64)

	for _, workerID := range workerIDs {
		penalty[workerID] = 0.0
	}

	return penalty
}
