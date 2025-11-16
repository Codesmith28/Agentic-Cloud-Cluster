# Tau Store - Quick Reference

**Task 2.1**: Implement Tau Store  
**Status**: ✅ Complete

---

## Quick Start

```go
import "master/internal/telemetry"

// Create store
store := telemetry.NewInMemoryTauStore()

// Get tau
tau := store.GetTau("cpu-light") // 5.0

// Update tau (learning)
store.UpdateTau("cpu-light", 7.5)

// Set tau explicitly
store.SetTau("gpu-training", 75.0)
```

---

## Interface

```go
type TauStore interface {
    GetTau(taskType string) float64
    UpdateTau(taskType string, actualRuntime float64)
    SetTau(taskType string, tau float64)
}
```

---

## Default Tau Values

| Task Type | Default (s) |
|-----------|-------------|
| cpu-light | 5.0 |
| cpu-heavy | 15.0 |
| memory-heavy | 20.0 |
| gpu-inference | 10.0 |
| gpu-training | 60.0 |
| mixed | 10.0 |

---

## EMA Formula

```
tau_new = λ * actualRuntime + (1-λ) * tau_old
```

**Default λ = 0.2** (20% new, 80% old)

**Example**:
```
tau_old = 5.0, actual = 10.0, λ = 0.2
tau_new = 0.2×10.0 + 0.8×5.0 = 6.0
```

---

## Common Operations

### Basic Usage
```go
store := telemetry.NewInMemoryTauStore()

// Get current tau
tau := store.GetTau("cpu-heavy") // Returns 15.0

// Learn from actual runtime
store.UpdateTau("cpu-heavy", 18.0) // Updates via EMA

// Check new value
newTau := store.GetTau("cpu-heavy") // Returns 15.6
```

### Custom Lambda
```go
// Create with custom EMA weight
store := telemetry.NewInMemoryTauStoreWithLambda(0.3)

// Or change later
store.SetLambda(0.25)
lambda := store.GetLambda() // Returns 0.25
```

### Bulk Operations
```go
// Get all tau values
allTau := store.GetAllTau()
for taskType, tau := range allTau {
    fmt.Printf("%s: %.2fs\n", taskType, tau)
}

// Reset to defaults
store.ResetToDefaults()
```

---

## Integration Patterns

### Pattern 1: Task Submission (Task 2.2)
```go
// Get tau for task type
tau := tauStore.GetTau(task.TaskType)

// Compute deadline
deadline := arrivalTime.Add(
    time.Duration(slaMultiplier * tau) * time.Second
)

// Store in task
taskDB.UpdateTaskWithSLA(ctx, taskID, deadline, tau, task.TaskType)
```

### Pattern 2: Task Completion (Task 2.4)
```go
// Calculate actual runtime
actualRuntime := completedAt.Sub(startedAt).Seconds()

// Update tau for task type
tauStore.UpdateTau(task.TaskType, actualRuntime)

// Log for monitoring
log.Printf("Updated tau for %s: %.2fs (actual: %.2fs)",
    task.TaskType, tauStore.GetTau(task.TaskType), actualRuntime)
```

### Pattern 3: RTS Scheduler (Task 3.3)
```go
// Get learned tau for task type
tau := tauStore.GetTau(taskView.Type)

// Use in prediction
predictedTime := tau * (1 + 
    theta.CPU*(taskCPU/workerCPU) +
    theta.Mem*(taskMem/workerMem) +
    theta.GPU*(taskGPU/workerGPU) +
    theta.Load*workerLoad
)
```

---

## Validation

### Valid Task Types (Case-Sensitive)
- ✅ `cpu-light`
- ✅ `cpu-heavy`
- ✅ `memory-heavy`
- ✅ `gpu-inference`
- ✅ `gpu-training`
- ✅ `mixed`

### Invalid Examples
- ❌ `cpu` (too generic)
- ❌ `CPU-LIGHT` (wrong case)
- ❌ `cpu_light` (underscore)
- ❌ `""` (empty)
- ❌ `invalid-type`

### Value Constraints
- **Tau**: Must be > 0
- **Runtime**: Must be > 0
- **Lambda**: Must be in [0, 1]

---

## Thread Safety

```go
// Safe for concurrent use
var wg sync.WaitGroup

// Multiple readers
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        tau := store.GetTau("cpu-light")
        // Use tau...
    }()
}

// Multiple writers
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(runtime float64) {
        defer wg.Done()
        store.UpdateTau("cpu-light", runtime)
    }(float64(i + 5))
}

wg.Wait()
```

---

## Error Handling

**No errors returned** - methods silently ignore invalid input:

```go
// Invalid type - ignored
store.UpdateTau("invalid", 10.0)

// Invalid value - ignored
store.SetTau("cpu-light", 0)
store.SetTau("cpu-light", -5.0)

// Out of range lambda - ignored
store.SetLambda(-0.1)
store.SetLambda(1.5)
```

**Rationale**: Scheduling must continue even with bad data.

---

## Testing

```bash
# Run tests
cd master/internal/telemetry
go test -v -run TestTau

# With race detector
go test -race -v

# Coverage
go test -cover
```

---

## Performance

- **GetTau**: O(1), ~10-50ns
- **UpdateTau**: O(1), ~50-200ns
- **Memory**: ~200 bytes (6 entries)
- **Thread-safe**: Full RWMutex protection

---

## Monitoring (Future - Milestone 7)

```go
// Expose as Prometheus metrics
tauGauge := prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "tau_store_values",
        Help: "Current tau values per task type",
    },
    []string{"task_type"},
)

// Update metrics
for taskType, tau := range store.GetAllTau() {
    tauGauge.WithLabelValues(taskType).Set(tau)
}
```

---

## Troubleshooting

### Issue: Tau not updating
**Check**:
1. Task type is valid (case-sensitive)
2. Runtime value is positive
3. UpdateTau is being called

### Issue: Unexpected tau values
**Check**:
1. Lambda value (default 0.2)
2. Number of updates (check logs)
3. Actual runtime values being passed

### Issue: Tau too volatile
**Solution**: Decrease lambda
```go
store.SetLambda(0.1) // More stable
```

### Issue: Tau too stable
**Solution**: Increase lambda
```go
store.SetLambda(0.3) // More responsive
```

---

## Best Practices

1. **Initialize once** at master startup
2. **Share single instance** across components
3. **Don't reset** in production (loses learning)
4. **Monitor** tau values for anomalies
5. **Log** significant changes (future)
6. **Tune lambda** based on workload volatility

---

## Lambda Tuning Guide

| Lambda | Behavior | Use Case |
|--------|----------|----------|
| 0.1 | Very stable | Consistent workloads |
| 0.2 | **Default** | **General use** |
| 0.3 | More responsive | Variable workloads |
| 0.5 | Highly responsive | Development/testing |
| 1.0 | No memory | Not recommended |

---

## Quick Lookup

**Create**: `NewInMemoryTauStore()`  
**Get**: `GetTau(taskType)`  
**Update**: `UpdateTau(taskType, runtime)`  
**Set**: `SetTau(taskType, tau)`  
**Lambda**: `GetLambda()`, `SetLambda(value)`  
**Reset**: `ResetToDefaults()`  
**All**: `GetAllTau()`

---

## Next Steps

1. ✅ Task 2.1 complete (Tau Store)
2. ⏭️ Task 2.2: Integrate into SubmitTask handler
3. ⏭️ Task 2.4: Update tau on task completion
4. ⏭️ Task 3.3: Use in RTS scheduler

---

**Last Updated**: November 16, 2025  
**Related**: TASK_2.1_IMPLEMENTATION_SUMMARY.md, SPRINT_PLAN.md
