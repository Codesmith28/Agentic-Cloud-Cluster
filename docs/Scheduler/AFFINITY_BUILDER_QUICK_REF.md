# Affinity Builder - Quick Reference

## Core Function

```go
func BuildAffinityMatrix(
    history []db.TaskHistory, 
    weights scheduler.AffinityWeights
) map[string]map[string]float64
```

## Formula

```
Affinity[taskType][workerID] = a₁ × speed + a₂ × SLA_reliability - a₃ × overload_rate
```

## Components

| Component | Formula | Meaning |
|-----------|---------|---------|
| **Speed** | baseline_runtime / worker_runtime | > 1.0 = faster, < 1.0 = slower |
| **SLA Reliability** | 1 - (violations / total) | 1.0 = perfect, 0.0 = all failed |
| **Overload Rate** | avg(LoadAtStart) | 0.5 = optimal, 1.0 = overloaded |

## Default Weights

```go
AffinityWeights{
    A1: 1.0,  // Speed coefficient
    A2: 2.0,  // SLA reliability coefficient
    A3: 0.5,  // Overload penalty
}
```

## Output Structure

```go
map[string]map[string]float64
// 6 rows (task types) × N columns (workers)
{
    "cpu-light": {"worker-1": 3.05, "worker-2": 1.82, ...},
    "cpu-heavy": {"worker-1": 2.70, ...},
    "memory-heavy": {...},
    "gpu-inference": {...},
    "gpu-training": {...},
    "mixed": {...}
}
```

## Affinity Interpretation

| Range | Meaning | Action |
|-------|---------|--------|
| +2.0 to +5.0 | Strongly preferred | Schedule here first |
| +0.5 to +2.0 | Preferred | Good choice |
| -0.5 to +0.5 | Neutral | No strong preference |
| -2.0 to -0.5 | Avoid | Consider alternatives |
| -5.0 to -2.0 | Strongly avoid | Last resort only |

## Usage

```go
// Build affinity matrix
weights := scheduler.AffinityWeights{A1: 1.0, A2: 2.0, A3: 0.5}
affinity := aod.BuildAffinityMatrix(history, weights)

// Query affinity
score := affinity["cpu-light"]["worker-1"]

// Use in scheduling
if score > 1.0 {
    // Prefer this worker for this task type
}
```

## Data Requirements

- **Minimum**: 3 records per (taskType, workerID) pair
- **Recommended**: 10+ records for stable statistics
- **Insufficient data**: Affinity = 0.0 (neutral)

## Test Coverage

✅ 8 tests, all passing
- Insufficient data handling
- Speed ratio calculation
- SLA reliability computation
- Overload rate measurement
- Affinity clipping [-5, +5]
- All 6 task types present
- Good vs poor worker differentiation

## Performance

- **Time**: O(T × W × N) where T=6, typically < 10ms
- **Space**: O(6 × W) ≈ 480 bytes for 10 workers

## Files

- **Implementation**: `master/internal/aod/affinity_builder.go` (230 lines)
- **Tests**: `master/internal/aod/affinity_builder_test.go` (365 lines)
- **Docs**: `docs/Scheduler/TASK_4_3_AFFINITY_BUILDER.md`
