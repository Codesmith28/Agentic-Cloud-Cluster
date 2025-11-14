# Task Cancellation Feature

**Date:** November 14, 2025  
**Branch:** sarthak/cancel_tasks  
**Status:** ✅ Implemented and Tested

---

## Overview

The task cancellation feature allows users to stop running tasks from the master CLI. When a task is cancelled, the Docker container is stopped, the task status is updated in the database, and the worker reports the cancellation back to the master.

---

## Architecture

### Flow Diagram

```
User (Master CLI)
    |
    | cancel <task_id>
    ↓
Master Server
    |
    | 1. Lookup which worker has the task
    | 2. Connect to worker via gRPC
    | 3. Call CancelTask RPC
    ↓
Worker Server
    |
    | 1. Stop Docker container (graceful then forceful)
    | 2. Remove container
    | 3. Clean up task tracking
    | 4. Report cancellation to master
    ↓
Docker Container
    | - SIGTERM (10s timeout)
    | - SIGKILL (if needed)
    | - Container removed
    ↓
Master Server
    |
    | 1. Update task status to 'cancelled' in DB
    | 2. Remove from worker's running tasks
    | 3. Store result in RESULTS collection
    ↓
MongoDB
    | - TASKS collection: status='cancelled'
    | - RESULTS collection: task result with logs
```

---

## Components Modified

### 1. Worker (`worker/internal/executor/executor.go`)

**Added `CancelTask` method:**
```go
func (e *TaskExecutor) CancelTask(ctx context.Context, taskID string) error
```

**Functionality:**
- Looks up container ID from task ID mapping
- Attempts graceful stop with 10-second timeout
- Falls back to SIGKILL if graceful stop fails
- Removes container forcefully
- Cleans up task tracking

**Error Handling:**
- Returns error if task not found
- Logs warnings for stop/remove failures
- Ensures cleanup even on partial failures

---

### 2. Worker Server (`worker/internal/server/worker_server.go`)

**Implemented `CancelTask` RPC handler:**
```go
func (s *WorkerServer) CancelTask(ctx context.Context, taskID *pb.TaskID) (*pb.TaskAck, error)
```

**Functionality:**
- Receives cancellation request from master
- Calls executor's CancelTask method
- Removes task from telemetry monitoring
- Reports cancellation to master asynchronously

**Added `reportCancellation` helper:**
```go
func (s *WorkerServer) reportCancellation(taskID string)
```

**Functionality:**
- Sends TaskResult with status="cancelled" to master
- Includes cancellation message in logs
- Runs in separate goroutine (non-blocking)

**Logging:**
- Beautiful formatted output with box drawing
- Success/failure indicators
- Task ID and operation status

---

### 3. Master Server (`master/internal/server/master_server.go`)

**Implemented `CancelTask` method:**
```go
func (s *MasterServer) CancelTask(ctx context.Context, taskID *pb.TaskID) (*pb.TaskAck, error)
```

**Functionality:**
1. **Task Lookup:**
   - First checks in-memory worker.RunningTasks map
   - Falls back to database (ASSIGNMENTS collection) if not found
   - Identifies which worker is running the task

2. **Worker Communication:**
   - Connects to worker via gRPC
   - Sends CancelTask RPC with task ID
   - Handles connection and RPC errors

3. **Database Updates:**
   - Updates task status to 'cancelled' in TASKS collection
   - Logs warnings if database update fails

4. **State Cleanup:**
   - Removes task from worker's RunningTasks map
   - Maintains consistency between memory and database

**Updated `ReportTaskCompletion`:**
- Now handles "cancelled" status in addition to "success" and "failed"
- Correctly updates database when worker reports cancellation
- Stores cancellation result in RESULTS collection

**Logging:**
- Comprehensive logging throughout cancellation flow
- Box-drawing characters for visual clarity
- Success/failure indicators
- Worker and task identification

---

### 4. Master CLI (`master/internal/cli/cli.go`)

**Added `cancel` command:**
```
Usage: cancel <task_id>
Example: cancel task-1234567890
```

**Implemented `cancelTask` method:**
```go
func (c *CLI) cancelTask(taskID string)
```

**Features:**
- Beautiful colored output with ANSI codes
- Progress indication
- Clear success/failure messages
- 10-second timeout for operation

**Updated `printHelp`:**
- Added cancel command documentation
- Included usage examples

---

### 5. Protocol Buffers (`proto/master_worker.proto`)

**Existing RPC (no changes needed):**
```protobuf
rpc CancelTask(TaskID) returns (TaskAck);
```

**Message Types Used:**
- `TaskID`: Contains task_id string
- `TaskAck`: Returns success boolean and message
- `TaskResult`: Used to report cancellation to master

---

## Database Schema

### TASKS Collection

**Updated Status Field:**
```javascript
{
  task_id: "task-1234567890",
  status: "cancelled",  // New status value
  completed_at: ISODate("2025-11-14T..."),
  // ... other fields
}
```

**Possible Status Values:**
- `pending` - Task created but not assigned
- `running` - Task executing on worker
- `completed` - Task finished successfully
- `failed` - Task execution failed
- `cancelled` - Task was cancelled by user ✨ NEW

---

### RESULTS Collection

**Cancellation Result:**
```javascript
{
  task_id: "task-1234567890",
  worker_id: "worker-1",
  status: "cancelled",
  logs: "Task was cancelled by user request",
  completed_at: ISODate("2025-11-14T..."),
}
```

---

## Usage Examples

### 1. Cancel a Running Task

```bash
# Start master
./runMaster.sh

# In master CLI, assign a long-running task
master> task worker-1 ubuntu:latest -cpu_cores 1.0 -mem 0.5

# Output:
# ✅ Task task-1234567890 assigned successfully!

# Cancel the task while it's running
master> cancel task-1234567890

# Output:
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#   🛑 CANCELLING TASK
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#   Task ID: task-1234567890
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#
# ✅ Task cancelled successfully!
#    Task cancelled successfully
```

### 2. Cancel Non-Existent Task

```bash
master> cancel task-999999

# Output:
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#   🛑 CANCELLING TASK
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#   Task ID: task-999999
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#
# ❌ Failed to cancel task: Task not found or not assigned to any worker: ...
```

### 3. View Help

```bash
master> help

# Output includes:
#   cancel <task_id>               - Cancel a running task
# 
# Examples:
#   cancel task-123
```

---

## Logging Examples

### Master Logs

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  🛑 CANCELLING TASK
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Task ID: task-1234567890
  Target Worker: worker-1 (192.168.1.100:50052)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  ✓ Task status updated to 'cancelled' in database
  ✓ Task cancelled successfully
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Worker Logs

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  🛑 TASK CANCELLATION REQUEST
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Task ID: task-1234567890
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[Task task-1234567890] Cancelling task (container: a1b2c3d4e5f6)...
[Task task-1234567890] ✓ Task cancelled successfully
  ✓ Task cancelled successfully
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[Task task-1234567890] ✓ Cancellation reported to master
```

---

## Error Handling

### Scenarios Handled

1. **Task Not Found:**
   - Checked in-memory state
   - Checked database assignments
   - Clear error message to user

2. **Worker Unreachable:**
   - gRPC connection timeout (10 seconds)
   - Graceful error message
   - Task still marked for cancellation in DB

3. **Container Already Stopped:**
   - Worker handles gracefully
   - Returns success (idempotent operation)

4. **Database Errors:**
   - Logged as warnings
   - Operation continues (best effort)
   - User still gets success response

5. **Network Failures:**
   - Timeout protection
   - Clear error messages
   - No hanging operations

---

## Testing Guide

### Prerequisites

1. Master node running
2. At least one worker registered
3. MongoDB running
4. Docker daemon running on worker

### Test Cases

#### Test 1: Cancel Running Task ✅

```bash
# 1. Assign a long-running task
master> task worker-1 ubuntu:latest -cpu_cores 1.0 -mem 0.5

# 2. Immediately cancel it
master> cancel task-1234567890

# Expected: Task stops, status updated to 'cancelled'
```

#### Test 2: Cancel Completed Task ❌

```bash
# 1. Assign a quick task
master> task worker-1 hello-world:latest

# 2. Wait for completion, then try to cancel
master> cancel task-1234567890

# Expected: Error message "Task not found or not running"
```

#### Test 3: Cancel Non-Existent Task ❌

```bash
master> cancel task-999999

# Expected: Error message "Task not found..."
```

#### Test 4: Verify Database Updates ✅

```bash
# After cancelling task-1234567890:

# Check MongoDB
mongo cloudai_db
db.TASKS.find({task_id: "task-1234567890"})
# Expected: status: "cancelled", completed_at: <timestamp>

db.RESULTS.find({task_id: "task-1234567890"})
# Expected: status: "cancelled", logs: "Task was cancelled..."
```

#### Test 5: Worker Restart Scenario ✅

```bash
# 1. Assign task to worker-1
master> task worker-1 ubuntu:latest

# 2. Restart worker-1 (task keeps running)

# 3. Try to cancel task
master> cancel task-1234567890

# Expected: Uses database to find worker, sends cancellation
```

---

## Performance Considerations

### Resource Cleanup

- **Container Stop:** 10-second graceful timeout
- **Container Kill:** Immediate forceful termination
- **Memory Cleanup:** Task removed from all tracking maps
- **Database:** Single update operation per cancellation

### Network Efficiency

- **Master → Worker:** Single gRPC call
- **Worker → Master:** Async result reporting
- **Timeout:** 10 seconds for entire operation

### Database Impact

- **Reads:** 0-1 (only if task not in memory)
- **Writes:** 2 (TASKS status update, RESULTS insert)
- **Indexes:** Uses task_id index for fast lookup

---

## Limitations

1. **No Batch Cancellation:**
   - Must cancel tasks one at a time
   - Could add `cancel all` or `cancel <worker_id>/*` in future

2. **No Undo:**
   - Cancellation is permanent
   - Task cannot be resumed

3. **No Partial Cancellation:**
   - Entire task is cancelled
   - No support for cancelling individual steps

4. **Worker Must Be Online:**
   - If worker is offline, cancellation fails
   - Task will be cleaned up when worker reconnects (future feature)

---

## Future Enhancements

### Priority: High

- [ ] **Bulk Cancellation:** Cancel all tasks on a worker
- [ ] **Force Cancel:** Skip graceful stop, immediate kill
- [ ] **Offline Worker Handling:** Queue cancellation for offline workers

### Priority: Medium

- [ ] **Cancel by User:** Cancel all tasks for a user
- [ ] **Cancel by Status:** Cancel all pending/running tasks
- [ ] **Cancellation Webhook:** Notify external systems

### Priority: Low

- [ ] **Cancellation Reason:** Allow user to specify reason
- [ ] **Cancellation Audit Trail:** Track who cancelled what
- [ ] **Auto-Retry:** Retry cancellation if worker doesn't respond

---

## API Reference

### Master CLI

```
Command: cancel <task_id>
Description: Cancel a running task
Parameters:
  - task_id: ID of the task to cancel (required)
Returns:
  - Success: "Task cancelled successfully!"
  - Failure: Error message with details
Example:
  master> cancel task-1234567890
```

### gRPC API

**Service:** `MasterWorker`

**RPC:** `CancelTask`

**Request:**
```protobuf
message TaskID {
  string task_id = 1;
}
```

**Response:**
```protobuf
message TaskAck {
  bool success = 1;
  string message = 2;
}
```

**Example:**
```go
client := pb.NewMasterWorkerClient(conn)
ack, err := client.CancelTask(ctx, &pb.TaskID{TaskId: "task-123"})
if err != nil {
    // Handle error
}
if ack.Success {
    // Task cancelled
}
```

---

## Troubleshooting

### Issue: "Task not found or not assigned to any worker"

**Causes:**
- Task ID is incorrect
- Task already completed
- Task never existed

**Solution:**
- Verify task ID with `monitor <task_id>`
- Check MongoDB for task status

---

### Issue: "Failed to connect to worker"

**Causes:**
- Worker is offline
- Network issues
- Firewall blocking connection

**Solution:**
- Check worker status: `master> workers`
- Verify worker is active (green indicator)
- Check network connectivity

---

### Issue: "Failed to cancel task: task not found or not running"

**Causes:**
- Container already stopped
- Task ID not in worker's tracking map
- Worker restarted recently

**Solution:**
- Check worker logs
- Verify container exists: `docker ps -a`
- Task might have already completed

---

### Issue: Database not updating

**Causes:**
- MongoDB connection lost
- Permissions issue
- Network timeout

**Solution:**
- Check MongoDB status
- Verify master can connect to DB
- Check master logs for database errors

---

## Related Documentation

- [Task Execution Guide](TASK_EXECUTION_QUICK_REFERENCE.md)
- [Task Monitoring](TASK_EXECUTION_MONITORING_SUMMARY.md)
- [Worker Registration](WORKER_REGISTRATION.md)
- [Database Schema](schema.md)
- [Architecture Overview](architecture.md)

---

## Changelog

### Version 1.0 - November 14, 2025

**Added:**
- ✅ Task cancellation feature
- ✅ CLI `cancel` command
- ✅ Database status updates
- ✅ Worker-side container termination
- ✅ Graceful shutdown with fallback to force kill
- ✅ Comprehensive logging
- ✅ Error handling

**Modified:**
- ✅ Worker executor to support cancellation
- ✅ Master server to route cancellations
- ✅ CLI help text
- ✅ Task status enum

**Tested:**
- ✅ Basic cancellation flow
- ✅ Error scenarios
- ✅ Database updates
- ✅ Container cleanup

---

**Status:** ✅ Feature Complete and Production Ready

**Last Updated:** November 14, 2025  
**Maintained by:** CloudAI Development Team
