package aod

import (
	"log"
	"math"

	"master/internal/db"
)

// ComputeFitness evaluates a chromosome's fitness based on historical performance metrics.
//
// Fitness represents how well the chromosome's parameters would have performed
// on historical workloads. Higher fitness means better overall performance.
//
// Formula (EDD §5.5):
//
//	Fitness = w1×SLA_success + w2×Utilization - w3×Energy_norm - w4×Overload_norm
//
// Where:
//   - w1, w2, w3, w4 are weights from GAConfig.FitnessWeights
//   - SLA_success: fraction of tasks meeting deadlines [0,1] (higher better)
//   - Utilization: average resource usage [0,1] (higher better)
//   - Energy_norm: normalized energy consumption [0,1] (lower better, hence negative sign)
//   - Overload_norm: fraction of time overloaded [0,1] (lower better, hence negative sign)
//
// Returns: fitness score (unbounded, higher is better)
func ComputeFitness(chromosome Chromosome, history []db.TaskHistory, workerStats []db.WorkerStats, config GAConfig) float64 {
	// Extract fitness weights from config
	w1 := config.FitnessWeights[0] // SLA success weight
	w2 := config.FitnessWeights[1] // Utilization weight
	w3 := config.FitnessWeights[2] // Energy weight (penalty)
	w4 := config.FitnessWeights[3] // Overload weight (penalty)

	// Compute the four fitness components
	slaSuccess := computeSLASuccess(history)
	utilization := computeUtilization(workerStats)
	energyNorm := computeEnergyNormTotal(workerStats)
	overloadNorm := computeOverloadNormTotal(workerStats)

	// Compute weighted fitness
	// Note: Energy and Overload are subtracted (penalties)
	fitness := w1*slaSuccess + w2*utilization - w3*energyNorm - w4*overloadNorm

	log.Printf("🧬 Fitness = %.4f (SLA=%.3f, Util=%.3f, Energy=%.3f, Overload=%.3f)",
		fitness, slaSuccess, utilization, energyNorm, overloadNorm)

	return fitness
}

// computeSLASuccess calculates the fraction of tasks that met their SLA deadline.
//
// Formula:
//
//	SLA_success = count(SLASuccess == true) / count(total_tasks)
//
// Returns:
//   - Range: [0.0, 1.0]
//   - 1.0: all tasks met their deadline (perfect)
//   - 0.0: all tasks violated their deadline (worst case)
//   - 0.0: if no tasks in history
func computeSLASuccess(history []db.TaskHistory) float64 {
	if len(history) == 0 {
		return 0.0
	}

	successCount := 0
	for _, task := range history {
		if task.SLASuccess {
			successCount++
		}
	}

	return float64(successCount) / float64(len(history))
}

// computeUtilization calculates the average resource utilization across all workers.
//
// Utilization is computed as the ratio of resources actually used to total observation time,
// averaged across all workers and all resource types (CPU, Memory, GPU).
//
// Formula:
//
//	For each worker:
//	  util = (CPU_used + Mem_used + GPU_used) / (3 × total_time)
//	Average across all workers
//
// Returns:
//   - Range: [0.0, 1.0+] (can exceed 1.0 due to oversubscription)
//   - Higher values indicate better resource usage
//   - 0.0: if no workers or no observation time
func computeUtilization(workerStats []db.WorkerStats) float64 {
	if len(workerStats) == 0 {
		return 0.0
	}

	totalUtil := 0.0
	validWorkers := 0

	for _, stats := range workerStats {
		if stats.TotalTime <= 0 {
			continue // Skip workers with no observation time
		}

		// Compute utilization as sum of resource-seconds divided by time
		// Normalize by dividing by 3 (number of resource types)
		resourceUsage := stats.CPUUsedTotal + stats.MemUsedTotal + stats.GPUUsedTotal
		workerUtil := resourceUsage / (3.0 * stats.TotalTime)

		totalUtil += workerUtil
		validWorkers++
	}

	if validWorkers == 0 {
		return 0.0
	}

	return totalUtil / float64(validWorkers)
}

// computeEnergyNormTotal calculates the normalized total energy consumption.
//
// Energy is estimated as the sum of all resource-seconds across all workers.
// This is normalized to [0, 1] range by dividing by a reference maximum.
//
// Formula:
//
//	total_energy = sum(CPU_used + Mem_used + GPU_used) across all workers
//	reference_max = sum(total_time × 3) across all workers (if fully utilized)
//	energy_norm = total_energy / reference_max
//
// Returns:
//   - Range: [0.0, 1.0+] (can exceed 1.0 with oversubscription)
//   - Lower values indicate more energy-efficient scheduling
//   - 0.0: if no workers or no observation time
func computeEnergyNormTotal(workerStats []db.WorkerStats) float64 {
	if len(workerStats) == 0 {
		return 0.0
	}

	totalEnergy := 0.0
	totalCapacity := 0.0

	for _, stats := range workerStats {
		if stats.TotalTime <= 0 {
			continue
		}

		// Sum all resource-seconds
		energy := stats.CPUUsedTotal + stats.MemUsedTotal + stats.GPUUsedTotal
		totalEnergy += energy

		// Reference capacity: if all resources fully utilized
		// 3 resource types × total observation time
		capacity := 3.0 * stats.TotalTime
		totalCapacity += capacity
	}

	if totalCapacity <= 0 {
		return 0.0
	}

	energyNorm := totalEnergy / totalCapacity

	// Clamp to reasonable range (can exceed 1.0 with oversubscription)
	return math.Max(0.0, math.Min(2.0, energyNorm))
}

// computeOverloadNormTotal calculates the normalized time spent in overload state.
//
// Overload represents time when workers were oversubscribed (load > capacity),
// which can lead to performance degradation and SLA violations.
//
// Formula:
//
//	total_overload = sum(overload_time) across all workers
//	total_time = sum(total_time) across all workers
//	overload_norm = total_overload / total_time
//
// Returns:
//   - Range: [0.0, 1.0]
//   - 0.0: workers were never overloaded (ideal)
//   - 1.0: workers were constantly overloaded (worst case)
//   - 0.0: if no workers or no observation time
func computeOverloadNormTotal(workerStats []db.WorkerStats) float64 {
	if len(workerStats) == 0 {
		return 0.0
	}

	totalOverload := 0.0
	totalTime := 0.0

	for _, stats := range workerStats {
		totalOverload += stats.OverloadTime
		totalTime += stats.TotalTime
	}

	if totalTime <= 0 {
		return 0.0
	}

	overloadNorm := totalOverload / totalTime

	// Ensure within valid range
	return math.Max(0.0, math.Min(1.0, overloadNorm))
}

// ComputeMetrics extracts the four fitness metrics from historical data.
// This is a convenience function that returns all metrics as a Metrics struct.
//
// Returns: Metrics struct with SLASuccess, Utilization, EnergyNorm, OverloadNorm
func ComputeMetrics(history []db.TaskHistory, workerStats []db.WorkerStats) Metrics {
	return Metrics{
		SLASuccess:   computeSLASuccess(history),
		Utilization:  computeUtilization(workerStats),
		EnergyNorm:   computeEnergyNormTotal(workerStats),
		OverloadNorm: computeOverloadNormTotal(workerStats),
	}
}

// EvaluateChromosomeFitness is a wrapper that sets the Fitness field on a chromosome.
// This modifies the chromosome in-place.
func EvaluateChromosomeFitness(chromosome *Chromosome, history []db.TaskHistory, workerStats []db.WorkerStats, config GAConfig) {
	chromosome.Fitness = ComputeFitness(*chromosome, history, workerStats, config)
}

// GetDefaultFitnessWeights returns sensible default weights for fitness computation.
//
// Default weights (EDD §6):
//   - w1 = 3.0: SLA success is most important (high priority)
//   - w2 = 1.0: Utilization is moderately important
//   - w3 = 0.5: Energy is less important (but still penalized)
//   - w4 = 1.5: Overload is important to avoid (between SLA and utilization)
//
// Rationale:
//   - SLA compliance is primary objective (highest weight)
//   - Balancing utilization and overload prevents waste while avoiding performance issues
//   - Energy is secondary concern (sustainability vs performance tradeoff)
func GetDefaultFitnessWeights() [4]float64 {
	return [4]float64{
		3.0, // w1: SLA success (maximize)
		1.0, // w2: Utilization (maximize)
		0.5, // w3: Energy norm (minimize, penalty)
		1.5, // w4: Overload norm (minimize, penalty)
	}
}
