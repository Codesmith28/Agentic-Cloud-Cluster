# Task 4.3: Affinity Builder - Implementation Summary

**Status:** ✅ COMPLETE  
**Date:** November 16, 2025  
**Component:** AOD (Affinity Offline Director) - Affinity Matrix Builder

## Overview
Implemented the Affinity Builder that computes affinity scores for each (taskType, workerID) pair based on historical performance metrics. The affinity matrix guides the RTS scheduler to prefer workers that have historically performed well for specific task types.

## Mathematical Foundation

### Affinity Formula (EDD §5.3)
```
A[taskType][workerID] = a₁ × speed + a₂ × SLA_reliability - a₃ × overload_rate
```

Where:
- **speed**: How fast the worker executes this task type relative to baseline
- **SLA_reliability**: Fraction of tasks that met their SLA deadline
- **overload_rate**: Average load when executing this task type
- **a₁, a₂, a₃**: Tunable weights from AffinityWeights

### Component Formulas

#### 1. Speed Ratio
```
speed = baseline_runtime / worker_avg_runtime
```
- **> 1.0**: Worker is faster than average (positive contribution)
- **< 1.0**: Worker is slower than average (negative contribution)
- **baseline_runtime**: Average runtime for task type across ALL workers

#### 2. SLA Reliability
```
SLA_reliability = 1 - (violations / total_tasks)
```
- **1.0**: Perfect compliance (all tasks met deadline)
- **0.0**: Complete failure (all tasks missed deadline)

#### 3. Overload Rate
```
overload_rate = avg(LoadAtStart) for (taskType, workerID) pairs
```
- **0.0**: Worker was idle (underutilized)
- **0.5**: Worker was moderately loaded (optimal)
- **1.0**: Worker was at max capacity (overloaded)

### Affinity Interpretation
- **Positive affinity** (+0.5 to +5.0): Worker is preferred
  - Fast execution
  - High SLA compliance
  - Moderate load
- **Neutral affinity** (~0.0): No strong preference
- **Negative affinity** (-5.0 to -0.5): Worker should be avoided
  - Slow execution
  - Frequent SLA violations
  - High overload

## Implementation

### Files Created

#### 1. `master/internal/aod/affinity_builder.go` (230 lines)

**Main Function:**
```go
func BuildAffinityMatrix(
    history []db.TaskHistory, 
    weights scheduler.AffinityWeights
) map[string]map[string]float64
```

**Core Algorithm:**
1. Extract all unique workers from history
2. For each of the 6 task types (cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed):
   - For each worker:
     - Filter history for (taskType, workerID) pairs
     - If < 3 data points → set affinity = 0.0 (neutral)
     - Compute speed ratio
     - Compute SLA reliability
     - Compute overload rate
     - Calculate: `affinity = a1*speed + a2*SLA - a3*overload`
     - Clip to [-5.0, +5.0]
3. Return nested map: `map[taskType][workerID]affinity`

**Helper Functions:**

1. **`computeSpeed(taskType, workerID, history) float64`**
   - Computes baseline runtime (avg across all workers for task type)
   - Computes worker-specific runtime
   - Returns speed ratio: baseline / worker_runtime

2. **`computeSLAReliability(taskType, workerID, pairHistory) float64`**
   - Counts SLA violations in filtered history
   - Returns: 1 - (violations / total)

3. **`computeOverloadRate(taskType, workerID, pairHistory) float64`**
   - Averages LoadAtStart across all records
   - Returns mean load value

4. **`computeBaselineRuntime(taskType, history) float64`**
   - Filters history by task type
   - Returns average runtime across all workers

5. **`computeWorkerAvgRuntime(taskType, workerID, history) float64`**
   - Filters history by (taskType, workerID)
   - Returns average runtime for this pair

6. **`filterHistory(history, taskType, workerID) []TaskHistory`**
   - Returns records matching both taskType AND workerID

7. **`filterHistoryByType(history, taskType) []TaskHistory`**
   - Returns records matching taskType only

8. **`getUniqueWorkers(history) []string`**
   - Extracts unique worker IDs from history

### Key Design Decisions

#### Why 3-Record Minimum?
Statistical reliability requires at least 3 data points to compute meaningful averages. With fewer records, variance is too high and affinity would be unreliable.

#### Why Clip to [-5, +5]?
- **Numerical stability**: Prevents extreme values from dominating GA fitness
- **Interpretability**: Bounded range makes affinity values comparable
- **Regularization**: Limits impact of outliers

#### Why All 6 Task Types?
The affinity matrix MUST have exactly 6 rows (one per task type) to match the standardized task type system. Even if no history exists for a task type, the row is created with neutral (0.0) affinities.

## Test Suite

### `master/internal/aod/affinity_builder_test.go` (365 lines)

**Test Coverage: 8 tests**

1. **TestBuildAffinityMatrixWithInsufficientData**
   - Verifies neutral affinity (0.0) when < 3 records
   - Ensures all 6 task types present in matrix

2. **TestBuildAffinityMatrixWithGoodWorker**
   - Creates history with fast, reliable worker
   - Verifies positive affinity for good worker
   - Verifies lower affinity for slower worker
   - Example output:
     - Worker-1: 3.050 (fast, reliable)
     - Worker-2: 1.817 (slower, less reliable)

3. **TestBuildAffinityMatrixAllTaskTypes**
   - Creates diverse history with all 6 task types
   - Verifies matrix has exactly 6 rows
   - Verifies appropriate workers have positive affinity
   - Validates task type mapping:
     - cpu-light → w1: 2.825
     - cpu-heavy → w1: 2.700
     - memory-heavy → w2: 2.738
     - gpu-inference → w2: 2.800
     - gpu-training → w3: 2.645
     - mixed → w3: 2.750

4. **TestComputeSpeed**
   - Tests speed ratio calculation
   - Fast worker (8s): speed = 1.25
   - Slow worker (12s): speed = 0.833
   - No data: speed = 0.0

5. **TestComputeSLAReliability**
   - Perfect SLA (100%): reliability = 1.0
   - Poor SLA (33%): reliability = 0.333
   - Empty history: reliability = 0.0

6. **TestComputeOverloadRate**
   - Low load (avg 0.3): overload = 0.3
   - High load (avg 0.8): overload = 0.8

7. **TestAffinityClipping**
   - Creates extreme affinity values using large weights
   - Verifies clipping to [-5, +5] range
   - Super fast worker with large weights → clipped to 5.0

8. **TestFilterHistory**
   - Tests filterHistory (taskType + workerID)
   - Tests filterHistoryByType (taskType only)
   - Tests getUniqueWorkers extraction

### Test Results
```bash
$ go test ./internal/aod -v -run="Affinity|Speed|SLA|Overload|Filter" -count=1

=== RUN   TestBuildAffinityMatrixWithInsufficientData
--- PASS: TestBuildAffinityMatrixWithInsufficientData (0.00s)

=== RUN   TestBuildAffinityMatrixWithGoodWorker
📊 Affinity[cpu-light][worker-1] = 3.050 (speed=1.250, SLA=1.000, overload=0.400)
📊 Affinity[cpu-light][worker-2] = 1.817 (speed=0.833, SLA=0.667, overload=0.700)
    ✓ Worker-1 affinity: 3.050 (fast, reliable)
    ✓ Worker-2 affinity: 1.817 (slower, less reliable)
--- PASS: TestBuildAffinityMatrixWithGoodWorker (0.00s)

=== RUN   TestBuildAffinityMatrixAllTaskTypes
📊 Affinity[cpu-light][w1] = 2.825 (speed=1.000, SLA=1.000, overload=0.350)
📊 Affinity[cpu-heavy][w1] = 2.700 (speed=1.000, SLA=1.000, overload=0.600)
📊 Affinity[memory-heavy][w2] = 2.738 (speed=1.000, SLA=1.000, overload=0.523)
📊 Affinity[gpu-inference][w2] = 2.800 (speed=1.000, SLA=1.000, overload=0.400)
📊 Affinity[gpu-training][w3] = 2.645 (speed=1.000, SLA=1.000, overload=0.710)
📊 Affinity[mixed][w3] = 2.750 (speed=1.000, SLA=1.000, overload=0.500)
    ✓ All 6 task types present in affinity matrix
--- PASS: TestBuildAffinityMatrixAllTaskTypes (0.00s)

=== RUN   TestComputeSpeed
    ✓ Speed ratios: w1=1.250 (fast), w2=0.833 (slow), w3=0.000 (no data)
--- PASS: TestComputeSpeed (0.00s)

=== RUN   TestComputeSLAReliability
    ✓ SLA reliability: perfect=1.000, poor=0.333, empty=0.000
--- PASS: TestComputeSLAReliability (0.00s)

=== RUN   TestComputeOverloadRate
    ✓ Overload rates: low=0.300, high=0.800
--- PASS: TestComputeOverloadRate (0.00s)

=== RUN   TestAffinityClipping
📊 Affinity[cpu-light][w-baseline] = 5.000 (speed=0.600, SLA=1.000, overload=0.500)
📊 Affinity[cpu-light][w-fast] = 5.000 (speed=3.000, SLA=1.000, overload=0.100)
    ✓ Affinity properly clipped: 5.000 (within [-5, +5])
--- PASS: TestAffinityClipping (0.00s)

=== RUN   TestFilterHistory
    ✓ Filter functions working correctly
--- PASS: TestFilterHistory (0.00s)

PASS
ok      master/internal/aod     0.009s
```

**All 8 tests passing ✅**

## Usage Example

```go
package main

import (
    "context"
    "log"
    "time"
    
    "master/internal/aod"
    "master/internal/db"
    "master/internal/scheduler"
)

func main() {
    // Fetch historical data (last 24 hours)
    historyDB := db.NewHistoryDB(ctx, cfg)
    since := time.Now().Add(-24 * time.Hour)
    history, err := historyDB.GetTaskHistory(context.Background(), since, time.Now())
    if err != nil {
        log.Fatal(err)
    }
    
    // Define affinity weights (from GA or defaults)
    weights := scheduler.AffinityWeights{
        A1: 1.0,  // Speed weight
        A2: 2.0,  // SLA reliability weight
        A3: 0.5,  // Overload penalty
    }
    
    // Build affinity matrix
    affinity := aod.BuildAffinityMatrix(history, weights)
    
    // Query affinity for specific (taskType, workerID)
    cpuLightW1 := affinity["cpu-light"]["worker-1"]
    log.Printf("Affinity[cpu-light][worker-1] = %.3f", cpuLightW1)
    
    // Use in RTS scheduling decision
    // Higher affinity → prefer this worker
    if cpuLightW1 > 1.0 {
        log.Println("Worker-1 is strongly preferred for cpu-light tasks")
    }
    
    // Iterate over all task types
    for taskType, workerMap := range affinity {
        log.Printf("Task type: %s", taskType)
        for workerID, aff := range workerMap {
            log.Printf("  %s: %.3f", workerID, aff)
        }
    }
}
```

## Integration Points

### Input: TaskHistory
```go
type TaskHistory struct {
    TaskID        string
    WorkerID      string
    Type          string    // One of 6 standardized types
    ActualRuntime float64   // Observed runtime (seconds)
    SLASuccess    bool      // Whether task met deadline
    LoadAtStart   float64   // Worker load at start [0, 1]
    // ... other fields
}
```

### Input: AffinityWeights
```go
type AffinityWeights struct {
    A1 float64  // Speed coefficient
    A2 float64  // SLA reliability coefficient
    A3 float64  // Overload penalty coefficient
}
```

### Output: Affinity Matrix
```go
map[string]map[string]float64
// Structure:
// {
//   "cpu-light": {
//     "worker-1": 3.05,
//     "worker-2": 1.82,
//     ...
//   },
//   "cpu-heavy": { ... },
//   "memory-heavy": { ... },
//   "gpu-inference": { ... },
//   "gpu-training": { ... },
//   "mixed": { ... }
// }
```

### Future Integration (Task 4.6: GA Runner)
The affinity builder will be called within the GA epoch:
```go
func RunGAEpoch(...) error {
    // ... fetch history
    
    // For each chromosome in population:
    for _, chromosome := range population {
        // Build affinity matrix using chromosome's weights
        affinity := aod.BuildAffinityMatrix(history, chromosome.AffinityW)
        
        // Store in chromosome for fitness evaluation
        chromosome.Affinity = affinity
    }
    
    // ... continue with fitness evaluation
}
```

### Future Integration (Task 3.3: RTS Scheduler)
The RTS scheduler will use affinity in risk computation:
```go
func (s *RTSScheduler) computeFinalRisk(baseRisk, taskType, workerID, params) float64 {
    // Lookup affinity for this (taskType, workerID) pair
    affinity := params.AffinityMatrix[taskType][workerID]
    
    // Reduce risk for positive affinity (prefer this worker)
    // Increase risk for negative affinity (avoid this worker)
    finalRisk := baseRisk - affinity + penalty
    
    return finalRisk
}
```

## Performance Characteristics

### Time Complexity
- **BuildAffinityMatrix**: O(T × W × N)
  - T = 6 task types (constant)
  - W = number of workers
  - N = number of history records
- **Typical**: ~10 workers, ~1000 records → O(60,000) operations
- **Fast**: < 10ms for typical datasets

### Space Complexity
- **Affinity matrix**: O(T × W) = O(6W)
- **Typical**: 10 workers → 60 float64 values ≈ 480 bytes
- **Negligible memory footprint**

### Scalability
- Handles 100+ workers efficiently
- Handles 10,000+ history records
- Linear scaling with data size
- Can be optimized with caching if needed

## Error Handling

### Insufficient Data
- **Condition**: < 3 records for (taskType, workerID) pair
- **Action**: Set affinity = 0.0 (neutral)
- **Rationale**: Avoid unreliable statistics from sparse data

### Missing Baseline
- **Condition**: No history for a task type
- **Action**: Return 0.0 for speed ratio
- **Impact**: Affinity depends only on SLA and overload

### Division by Zero
- **Condition**: worker_avg_runtime = 0 or baseline = 0
- **Action**: Return 0.0 for speed ratio
- **Safe**: Prevents NaN/Inf values

## Validation

### Affinity Range Check
All affinity values are clipped to [-5.0, +5.0]:
```go
clippedAffinity := math.Max(-5.0, math.Min(5.0, rawAffinity))
```

### Task Type Validation
Matrix includes exactly 6 rows (one per task type):
- cpu-light
- cpu-heavy
- memory-heavy
- gpu-inference
- gpu-training
- mixed

### Logging
Debug logs show affinity computation details:
```
📊 Affinity[cpu-light][worker-1] = 3.050 (speed=1.250, SLA=1.000, overload=0.400)
```

## Next Steps

### Task 4.4: Penalty Builder
- Implement penalty vector computation
- Use WorkerStats for per-worker penalties
- Penalties increase risk for unreliable workers

### Task 4.5: Fitness Function
- Use affinity in chromosome evaluation
- Combine with metrics (SLA, utilization, energy)
- Compute overall fitness score

### Task 4.6: GA Runner
- Call BuildAffinityMatrix during GA epochs
- Evolve AffinityWeights using genetic algorithm
- Save best affinity matrix to ga_output.json

---

**Task 4.3 Status: IMPLEMENTATION COMPLETE ✅**

All code written, 8 tests passing, ready for integration with GA Runner.
