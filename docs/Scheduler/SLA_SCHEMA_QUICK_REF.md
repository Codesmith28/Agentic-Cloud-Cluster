# SLA Schema Extension - Quick Reference

## Schema Changes

### Task Collection (NEW FIELDS)
```javascript
{
  deadline: Date,           // SLA deadline = arrival + k*tau
  tau: Double,              // Expected runtime (seconds)
  task_type: String,        // Must be one of 6 valid types
  sla_multiplier: Double    // k value [1.5-2.5]
}
```

### Assignment Collection (NEW FIELD)
```javascript
{
  load_at_start: Double     // Worker load [0-1] at assignment
}
```

## Valid Task Types (6 Types)

| Type | Default Tau | Use Case |
|------|-------------|----------|
| `cpu-light` | 5.0s | Light CPU workloads |
| `cpu-heavy` | 15.0s | Heavy CPU computation |
| `memory-heavy` | 20.0s | Memory-intensive tasks |
| `gpu-inference` | 10.0s | GPU inference |
| `gpu-training` | 60.0s | GPU training |
| `mixed` | 10.0s | Mixed resources |

## Key Formulas

### Deadline Calculation
```
deadline = arrival_time + k * tau
```

### SLA Success
```
sla_success = completed_at <= deadline
```

## API Method

### UpdateTaskWithSLA
```go
err := taskDB.UpdateTaskWithSLA(ctx, taskID, deadline, tau, taskType)
```

**Parameters**:
- `taskID` - Task identifier
- `deadline` - Computed deadline (time.Time)
- `tau` - Expected runtime in seconds
- `taskType` - Optional (empty = don't update)

**Validation**:
- Task type must be one of 6 valid types or empty
- Returns error if invalid type or task not found

## Usage Examples

### Update Task After Submission
```go
// Get tau from TauStore
tau := tauStore.GetTau("cpu-heavy")  // 15.0 seconds
k := 2.0  // SLA multiplier

// Calculate deadline
deadline := time.Now().Add(time.Duration(k * tau) * time.Second)

// Update task
err := taskDB.UpdateTaskWithSLA(ctx, taskID, deadline, tau, "cpu-heavy")
```

### Create Assignment with Load
```go
assignment := &db.Assignment{
    AssignmentID: "ass-123",
    TaskID:       "task-456",
    WorkerID:     "worker-1",
    AssignedAt:   time.Now(),
    LoadAtStart:  0.75,  // 75% load
}
err := assignmentDB.CreateAssignment(ctx, assignment)
```

### Check SLA Success
```go
task, _ := taskDB.GetTask(ctx, taskID)
slaSuccess := !task.CompletedAt.After(task.Deadline)

if !slaSuccess {
    missedBy := task.CompletedAt.Sub(task.Deadline)
    log.Printf("Missed SLA by %v", missedBy)
}
```

## Validation Rules

### Task Type
✅ Valid: `cpu-light`, `cpu-heavy`, `memory-heavy`, `gpu-inference`, `gpu-training`, `mixed`  
✅ Valid: Empty string (triggers inference)  
❌ Invalid: `cpu`, `gpu`, `dl`, `CPU-HEAVY`, `cpu_heavy`

### SLA Multiplier (k)
✅ Valid range: `1.5 ≤ k ≤ 2.5`  
❌ Invalid: `k < 1.5` or `k > 2.5`

### Load At Start
✅ Valid range: `0.0 ≤ load ≤ 1.0`  
❌ Invalid: `load < 0` or `load > 1`

## MongoDB Indexes (Recommended)

```javascript
// TASKS collection
db.TASKS.createIndex({ "deadline": 1 })
db.TASKS.createIndex({ "task_type": 1, "completed_at": 1 })
db.TASKS.createIndex({ "tau": 1 })

// ASSIGNMENTS collection
db.ASSIGNMENTS.createIndex({ "task_id": 1, "load_at_start": 1 })
```

## Backward Compatibility

✅ **No migration required**
- New fields use `omitempty` BSON tag
- Existing tasks: `deadline` and `tau` will be zero-valued
- Existing assignments: `load_at_start` will be 0.0

## Integration with Other Tasks

### Task 2.2: Enrich Task Submission
```go
// In SubmitTask handler
tau := tauStore.GetTau(taskType)
deadline := now.Add(time.Duration(k * tau) * time.Second)
taskDB.UpdateTaskWithSLA(ctx, taskID, deadline, tau, taskType)
```

### Task 2.3: Track Load at Assignment
```go
// In assignTaskToWorker
load := telemetryManager.GetWorkerLoad(workerID)
assignment.LoadAtStart = load
```

### Task 2.5: Store SLA Success
```go
// In ReportTaskCompletion
task, _ := taskDB.GetTask(ctx, taskID)
slaSuccess := !task.CompletedAt.After(task.Deadline)
resultDB.CreateWithSLA(ctx, result, slaSuccess)
```

## Testing

```bash
# Run all SLA tests
cd master/internal/db
go test -v -run "TestTask|TestAssignment|TestSLA"

# Run specific test
go test -v -run TestDeadlineCalculation
```

**Test Count**: 26 tests  
**Status**: ✅ All passing

## Common Patterns

### Pattern 1: Task Lifecycle with SLA
```go
// 1. Submit task
task := &db.Task{TaskID: "task-1", ...}
taskDB.CreateTask(ctx, task)

// 2. Enrich with SLA
tau := tauStore.GetTau(task.TaskType)
deadline := task.CreatedAt.Add(time.Duration(k * tau) * time.Second)
taskDB.UpdateTaskWithSLA(ctx, task.TaskID, deadline, tau, task.TaskType)

// 3. Assign with load tracking
assignment := &db.Assignment{
    TaskID: task.TaskID,
    WorkerID: selectedWorker,
    LoadAtStart: currentLoad,
}
assignmentDB.CreateAssignment(ctx, assignment)

// 4. Complete and check SLA
task, _ = taskDB.GetTask(ctx, task.TaskID)
slaSuccess := !task.CompletedAt.After(task.Deadline)
```

### Pattern 2: Query Tasks by SLA Status
```go
// Get all tasks with SLA violations
tasks, _ := taskDB.GetTasksByStatus(ctx, "completed")
violations := 0
for _, task := range tasks {
    if task.CompletedAt.After(task.Deadline) {
        violations++
    }
}
violationRate := float64(violations) / float64(len(tasks))
```

## Troubleshooting

### Issue: UpdateTaskWithSLA fails with "invalid task type"
**Solution**: Ensure taskType is one of the 6 valid types or empty string

### Issue: Deadline is zero/empty in database
**Solution**: Call `UpdateTaskWithSLA()` after `CreateTask()`

### Issue: LoadAtStart is always 0
**Solution**: Set `assignment.LoadAtStart` before calling `CreateAssignment()`

---

**File Locations**:
- Task struct: `master/internal/db/tasks.go`
- Assignment struct: `master/internal/db/assignments.go`
- Tests: `master/internal/db/sla_test.go`
- Full docs: `docs/Scheduler/TASK_1.3_IMPLEMENTATION_SUMMARY.md`

**Status**: ✅ Task 1.3 Complete
