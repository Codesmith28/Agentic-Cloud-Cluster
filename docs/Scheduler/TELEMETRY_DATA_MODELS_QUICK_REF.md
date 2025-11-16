# Telemetry Data Models - Quick Reference

## Data Structures

### TaskHistory
Enriched historical task execution data from TASKS + ASSIGNMENTS + RESULTS.

| Field | Type | Description |
|-------|------|-------------|
| `TaskID` | string | Unique task identifier |
| `WorkerID` | string | Worker that executed the task |
| `Type` | string | Task type (must be one of 6 valid types) |
| `ArrivalTime` | time.Time | When task was submitted |
| `Deadline` | time.Time | SLA deadline (arrival + k*tau) |
| `ActualStart` | time.Time | When execution started |
| `ActualFinish` | time.Time | When execution completed |
| `ActualRuntime` | float64 | Execution duration (seconds) |
| `SLASuccess` | bool | Whether task met deadline |
| `CPUUsed` | float64 | CPU cores allocated |
| `MemUsed` | float64 | Memory GB allocated |
| `GPUUsed` | float64 | GPU cores allocated |
| `StorageUsed` | float64 | Storage GB allocated |
| `LoadAtStart` | float64 | Worker load when started (0-1) |
| `Tau` | float64 | Expected runtime baseline |
| `SLAMultiplier` | float64 | k value used (1.5-2.5) |

### WorkerStats
Aggregated statistics per worker over time period.

| Field | Type | Description |
|-------|------|-------------|
| `WorkerID` | string | Worker identifier |
| `TasksRun` | int | Total tasks executed |
| `SLAViolations` | int | Number of missed deadlines |
| `TotalRuntime` | float64 | Sum of task runtimes (sec) |
| `CPUUsedTotal` | float64 | Sum of CPU-seconds |
| `MemUsedTotal` | float64 | Sum of Memory-GB-seconds |
| `GPUUsedTotal` | float64 | Sum of GPU-seconds |
| `OverloadTime` | float64 | Time at high load (sec) |
| `TotalTime` | float64 | Observation period (sec) |
| `AvgLoad` | float64 | Average normalized load |
| `PeriodStart` | time.Time | Start of period |
| `PeriodEnd` | time.Time | End of period |

## Valid Task Types

All queries validate against these 6 types:
- `cpu-light` - Light CPU workloads
- `cpu-heavy` - Heavy CPU workloads
- `memory-heavy` - Memory-intensive tasks
- `gpu-inference` - GPU inference workloads
- `gpu-training` - GPU training workloads
- `mixed` - Mixed resource usage

## HistoryDB Methods

```go
// Initialize
historyDB, err := db.NewHistoryDB(ctx, cfg)
defer historyDB.Close(ctx)

// Get all task history in time range
history, err := historyDB.GetTaskHistory(ctx, since, until)

// Get history for specific task type
cpuHistory, err := historyDB.GetTaskHistoryByType(ctx, "cpu-heavy", since, until)

// Get worker statistics
stats, err := historyDB.GetWorkerStats(ctx, since, until)

// Get stats for specific worker
workerStats, err := historyDB.GetWorkerStatsForWorker(ctx, "worker-1", since, until)

// Get overall SLA success rate
rate, err := historyDB.GetSLASuccessRate(ctx, since, until)

// Get per-type SLA success rate
rate, err := historyDB.GetSLASuccessRateByType(ctx, "cpu-heavy", since, until)
```

## Common Usage Patterns

### 1. Query Last 24 Hours
```go
since := time.Now().Add(-24 * time.Hour)
until := time.Now()
history, err := historyDB.GetTaskHistory(ctx, since, until)
```

### 2. Compute Metrics for GA Training
```go
// Get training data for last 48 hours
since := time.Now().Add(-48 * time.Hour)
until := time.Now()

// Get task history per type for Theta training
for _, taskType := range []string{"cpu-light", "cpu-heavy", "memory-heavy", "gpu-inference", "gpu-training", "mixed"} {
    history, _ := historyDB.GetTaskHistoryByType(ctx, taskType, since, until)
    // Train Theta for this task type
}

// Get worker stats for Penalty Vector
workerStats, _ := historyDB.GetWorkerStats(ctx, since, until)
```

### 3. Monitor SLA Performance
```go
// Overall cluster SLA
overallSLA, _ := historyDB.GetSLASuccessRate(ctx, since, until)
fmt.Printf("Cluster SLA: %.2f%%\n", overallSLA*100)

// Per-type SLA breakdown
types := []string{"cpu-light", "cpu-heavy", "memory-heavy", "gpu-inference", "gpu-training", "mixed"}
for _, t := range types {
    rate, _ := historyDB.GetSLASuccessRateByType(ctx, t, since, until)
    fmt.Printf("%s: %.2f%%\n", t, rate*100)
}
```

### 4. Identify Problem Workers
```go
stats, _ := historyDB.GetWorkerStats(ctx, since, until)

for _, s := range stats {
    violationRate := float64(s.SLAViolations) / float64(s.TasksRun)
    overloadPct := (s.OverloadTime / s.TotalTime) * 100
    
    if violationRate > 0.1 {
        fmt.Printf("⚠ Worker %s: High SLA violations (%.1f%%)\n", 
            s.WorkerID, violationRate*100)
    }
    if overloadPct > 20 {
        fmt.Printf("⚠ Worker %s: High overload (%.1f%%)\n",
            s.WorkerID, overloadPct)
    }
}
```

## MongoDB Collections Used

**TASKS** - Task metadata
- Must have: `task_id`, `task_type`, `created_at`, `started_at`, `completed_at`, `tau`, `sla_multiplier`
- Index: `{ "completed_at": 1, "status": 1 }`

**ASSIGNMENTS** - Task-worker mappings
- Must have: `task_id`, `worker_id`, `load_at_start`
- Index: `{ "task_id": 1 }`

**RESULTS** - Task completion results
- Must have: `task_id`, `status`, `completed_at`
- Index: `{ "task_id": 1 }`

## Aggregation Pipeline

GetTaskHistory uses 6-stage MongoDB aggregation:
1. **Match** - Filter by time + status
2. **Lookup** - Join ASSIGNMENTS
3. **Unwind** - Flatten assignments
4. **Lookup** - Join RESULTS
5. **Unwind** - Flatten results
6. **Project** - Compute fields (deadline, runtime, sla_success)
7. **Match** - Filter valid task types
8. **Sort** - Order by arrival_time

## Calculations

**SLA Success**: `completed_at <= deadline`

**Deadline**: `arrival_time + sla_multiplier * tau`

**Actual Runtime**: `completed_at - started_at` (seconds)

**Violation Rate**: `sla_violations / tasks_run`

**Overload Rate**: `overload_time / total_time`

**Avg Load**: `sum(load_at_start) / tasks_run`

## File Location

```
master/internal/db/
├── history.go       # Implementation
└── history_test.go  # Tests
```

## Integration

**Used by**:
- Task 2.1: Tau Store (runtime learning)
- Task 4.2: Theta Trainer (regression)
- Task 4.3: Affinity Builder (per-type affinity)
- Task 4.4: Penalty Builder (worker penalties)
- Task 4.5: Fitness Function (GA fitness)
- Task 4.6: GA Runner (main training loop)

**Requires**:
- Task 1.3: Extended task schema (deadline, tau, task_type fields)
- Task 2.3: Load tracking (load_at_start in assignments)
- Task 2.5: SLA tracking (sla_success in results)

## Testing

```bash
# Run all tests
cd master/internal/db
go test -v

# Run specific test
go test -v -run TestTaskHistoryStructure

# Build verification
cd master
go build -o masterNode .
```

---

**Status**: ✅ Task 1.2 Complete
