# Context and Persistence Fixes

## Overview
Fixed two critical issues preventing task execution and monitoring:
1. **Context Cancellation Issue**: Tasks were completing immediately with "context canceled" error
2. **Log Persistence Missing**: Completed task logs weren't being stored, preventing monitoring of finished tasks

## Problem 1: Context Cancellation

### Root Cause
The `executeTask` method in `worker/internal/server/worker_server.go` was using the RPC context from `AssignTask`. This context has a timeout and gets cancelled when the RPC call completes, causing any long-running container operations to be cancelled mid-execution.

### Symptoms
- Tasks complete immediately after starting
- Worker logs show "Failed to report task result: context canceled"
- Container execution interrupted before completion
- Monitoring shows "Task not found or not running on this worker"

### Fix Applied
Changed `executeTask` signature and implementation to use separate contexts:

**Before:**
```go
func (s *WorkerServer) executeTask(ctx context.Context, task *pb.Task) {
    // Uses ctx throughout, gets cancelled when RPC times out
    err := s.executor.PullImage(ctx, task.ContainerImage)
    containerID, err := s.executor.RunContainer(ctx, ...)
}
```

**After:**
```go
func (s *WorkerServer) executeTask(task *pb.Task) {
    // Create independent context for task execution
    ctx := context.Background()
    
    err := s.executor.PullImage(ctx, task.ContainerImage)
    containerID, err := s.executor.RunContainer(ctx, ...)
    
    // Separate context with timeout for reporting
    reportCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    _, err = s.masterClient.ReportTaskCompletion(reportCtx, &pb.TaskResult{...})
}
```

### Key Changes
- `executeTask` no longer takes context parameter
- Uses `context.Background()` for long-running operations
- Separate 10-second timeout context for result reporting
- Task execution lifecycle independent of RPC lifecycle

## Problem 2: Missing Log Persistence

### Root Cause
The `ReportTaskCompletion` method in `master/internal/server/master_server.go` had a TODO comment and wasn't storing task logs in the database. This prevented the monitoring system from retrieving logs for completed tasks.

### Symptoms
- Monitoring works for running tasks but fails for completed ones
- "Task not found" error when trying to monitor finished tasks
- Task logs only visible in master's console output
- No historical log retrieval capability

### Fix Applied

#### 1. Created Results Database Handler
**File:** `master/internal/db/results.go`

```go
type TaskResult struct {
    TaskID      string    `bson:"task_id"`
    WorkerID    string    `bson:"worker_id"`
    Status      string    `bson:"status"` // "success", "failed"
    Logs        string    `bson:"logs"`
    CompletedAt time.Time `bson:"completed_at"`
}

type ResultDB struct {
    client     *mongo.Client
    collection *mongo.Collection
}

// Key methods:
// - CreateResult: Store task result with logs
// - GetResult: Retrieve result by task ID
// - GetResultsByWorker: Get all results for a worker
```

#### 2. Updated Master Server
**File:** `master/internal/server/master_server.go`

Added `resultDB` field to `MasterServer` struct:
```go
type MasterServer struct {
    // ... existing fields
    resultDB      *db.ResultDB
}
```

Updated constructor:
```go
func NewMasterServer(workerDB, taskDB, assignmentDB, resultDB, telemetryMgr) {
    // Initialize with resultDB
}
```

Updated `ReportTaskCompletion`:
```go
func (s *MasterServer) ReportTaskCompletion(ctx, result) {
    // ... existing code
    
    // Store result with logs in RESULTS collection
    if s.resultDB != nil {
        taskResult := &db.TaskResult{
            TaskID:   result.TaskId,
            WorkerID: result.WorkerId,
            Status:   result.Status,
            Logs:     result.Logs,
        }
        s.resultDB.CreateResult(context.Background(), taskResult)
    }
}
```

#### 3. Enhanced Log Streaming
**File:** `master/internal/server/master_server.go`

Updated `StreamTaskLogsFromWorker` to check database first:
```go
func (s *MasterServer) StreamTaskLogsFromWorker(ctx, taskID, userID, logHandler) {
    s.mu.RLock()
    
    // First, check if task is completed and logs are in database
    if s.resultDB != nil {
        result, err := s.resultDB.GetResult(ctx, taskID)
        if err == nil && result != nil {
            // Task is completed, return stored logs
            s.mu.RUnlock()
            logHandler(result.Logs, true)
            return nil
        }
    }
    
    // Task might be running, try to stream from worker
    // ... existing worker streaming code
}
```

#### 4. Initialized in Main
**File:** `master/main.go`

```go
var resultDB *db.ResultDB

// Initialize ResultDB
resultDB, err = db.NewResultDB(ctx, cfg)
if err != nil {
    log.Printf("Warning: Failed to create ResultDB: %v")
    resultDB = nil
} else {
    log.Println("✓ ResultDB initialized")
    defer resultDB.Close(context.Background())
}

masterServer := server.NewMasterServer(workerDB, taskDB, assignmentDB, resultDB, telemetryMgr)
```

## Data Flow

### Task Execution Flow (Fixed)
1. Master calls `AssignTask` RPC on worker
2. Worker receives task in `AssignTask` handler
3. Worker calls `executeTask(task)` in goroutine (NO context from RPC)
4. `executeTask` creates `context.Background()` for operations
5. Container pulls and runs without interruption
6. Task completes naturally, logs collected
7. Worker reports completion with logs to master

### Log Persistence Flow (New)
1. Worker sends `TaskResult` with logs via `ReportTaskCompletion`
2. Master receives result in `ReportTaskCompletion` handler
3. Master updates task status in TASKS collection
4. Master stores complete result in RESULTS collection via `resultDB.CreateResult`
5. Result includes: taskID, workerID, status, logs, completedAt

### Log Retrieval Flow (Enhanced)
1. User runs `monitor <task_id>` command
2. Master checks `resultDB.GetResult(taskID)` first
3. **If found:** Returns stored logs immediately (completed task)
4. **If not found:** Connects to worker and streams live logs (running task)
5. Live logs displayed in real-time until completion
6. Completed logs stored for future retrieval

## Testing Steps

### Test Context Fix
1. Start master and worker
2. Register worker with master
3. Assign a task that takes some time (e.g., hello-world container)
4. Check worker logs - should see:
   - "Pulling image..."
   - "Starting container..."
   - "Container output: [actual output]"
   - "Container completed successfully"
   - NO "context canceled" errors
5. Task should complete normally, not immediately

### Test Log Persistence
1. Complete the task execution above
2. Wait for task to finish and logs to be stored
3. Run `monitor <task_id>` on completed task
4. Should see:
   - Stored logs displayed immediately
   - "Task Completed" message
   - No "Task not found" error
5. Check MongoDB RESULTS collection:
   ```bash
   mongosh
   use cloudai
   db.RESULTS.find({task_id: "<task_id>"}).pretty()
   ```
   Should show the stored result with logs

### Test Live Monitoring
1. Assign a new task
2. Immediately run `monitor <task_id>` while task is running
3. Should see logs streaming in real-time
4. After task completes, run `monitor <task_id>` again
5. Should retrieve stored logs from database

## Files Modified

### Worker
- `worker/internal/server/worker_server.go`
  - Changed `executeTask` signature (removed context parameter)
  - Uses `context.Background()` for task operations

### Master
- `master/internal/db/results.go` (NEW)
  - Created ResultDB handler
  - TaskResult struct with BSON tags
  - CRUD operations for results collection

- `master/internal/server/master_server.go`
  - Added `resultDB` field
  - Updated constructor signature
  - Enhanced `ReportTaskCompletion` to store logs
  - Enhanced `StreamTaskLogsFromWorker` to check database first

- `master/main.go`
  - Added `resultDB` variable declaration
  - Initialize ResultDB with error handling
  - Pass resultDB to NewMasterServer

## Database Schema

### RESULTS Collection
```javascript
{
  task_id: "task-1762629077",
  worker_id: "worker-1",
  status: "success",  // or "failed"
  logs: "full container output...",
  completed_at: ISODate("2024-01-15T10:30:00Z")
}
```

### Indexes (Recommended)
```javascript
db.RESULTS.createIndex({ task_id: 1 }, { unique: true })
db.RESULTS.createIndex({ worker_id: 1 })
db.RESULTS.createIndex({ completed_at: -1 })
```

## Benefits

1. **Reliable Task Execution**: Tasks run to completion without premature cancellation
2. **Complete Audit Trail**: All task logs stored permanently in database
3. **Historical Monitoring**: Can view logs of any completed task, any time
4. **Improved Debugging**: Full visibility into what happened during task execution
5. **Better User Experience**: Monitoring works for both running and completed tasks

## Future Enhancements

1. **Log Rotation**: Implement TTL or size limits for old logs
2. **Log Pagination**: Stream large logs in chunks for better performance
3. **Search Capability**: Add text search across stored logs
4. **Analytics**: Aggregate statistics from stored results
5. **Export**: Allow exporting task logs to files
