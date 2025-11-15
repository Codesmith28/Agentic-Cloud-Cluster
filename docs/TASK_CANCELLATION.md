# Task Cancellation Feature - Complete Guide

## Table of Contents
- [Quick Reference](#quick-reference)
- [Overview](#overview)
- [Architecture & Flow](#architecture--flow)
- [Implementation Details](#implementation-details)
- [Database Schema](#database-schema)
- [Error Handling](#error-handling)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)

---

## Quick Reference

### Cancel a Task
```bash
master> cancel <task_id>

# Example
master> cancel task-1234567890
```

### What Happens When You Cancel
1. ✅ Master finds which worker has the task
2. ✅ Master updates task status to "cancelled" in database (immediately)
3. ✅ Master updates in-memory worker state
4. ✅ Master returns success to user (instant response)
5. ✅ Master sends cancellation to worker asynchronously
6. ✅ Worker stops Docker container (gracefully, 10s timeout)
7. ✅ Worker force-kills container if needed
8. ✅ Worker removes container and cleans up

### Common Commands
```bash
# Check if task is running
master> workers                    # List all workers and their tasks
master> stats <worker_id>         # Show detailed worker stats

# Monitor task before cancelling
master> monitor <task_id>         # See live logs

# Cancel task
master> cancel <task_id>          # Stop the task immediately
```

### Error Messages

| Message | Meaning | Solution |
|---------|---------|----------|
| "Task not found or not running" | Task doesn't exist or already completed | Check task ID with `workers` command |
| "Failed to update database" | Database connection issue | Check MongoDB status |
| Timeout warning (in logs) | Worker slow to respond | Normal - task still cancelled in DB |

---

## Overview

The task cancellation feature allows users to stop running tasks from the master CLI. The implementation uses an **optimistic, asynchronous approach** to ensure:
- Instant user feedback
- Database consistency
- Graceful handling of slow/offline workers

### Key Features
- **Immediate database update** - Task marked cancelled right away
- **Asynchronous worker notification** - User doesn't wait for container stop
- **Protected cancelled status** - Once cancelled, status never changes
- **Graceful degradation** - Works even if worker is offline/slow

---

## Architecture & Flow

### High-Level Flow

```
User CLI → Master Server → Worker Server → Task Executor → Docker Container
```

### Detailed Step-by-Step Flow

```
STEP 1: CLI Command
  ↓ User: master> cancel task-123
  ↓ Parse task ID
  ↓ Call masterServer.CancelTask()

STEP 2: Find Worker (Master Server)
  ↓ Search in-memory worker.RunningTasks
  ↓ Fallback: Query ASSIGNMENTS collection
  ↓ Result: Found worker "worker-1" at 192.168.1.55:50052

STEP 3: Update Database (Synchronous)
  ↓ Call taskDB.UpdateTaskStatus(taskID, "cancelled")
  ↓ MongoDB Update: TASKS collection
  ↓   • status: "cancelled"
  ↓   • completed_at: <timestamp>
  ↓ ✅ Database is now source of truth

STEP 4: Update Memory (Synchronous)
  ↓ Remove from worker.RunningTasks map
  ↓ Decrement worker.TaskCount

STEP 5: Return Success to User (Immediate)
  ↓ Unlock master mutex
  ↓ Return: "Task marked as cancelled..."
  ↓ 👤 USER SEES SUCCESS HERE

STEP 6: Worker Notification (Asynchronous Goroutine)
  ↓ Launch background goroutine (30s timeout)
  ↓ Connect to worker via gRPC
  ↓ Send CancelTask RPC
  
STEP 7: Worker Receives Cancellation
  ↓ WorkerServer.CancelTask() called
  ↓ Call executor.CancelTask(taskID)

STEP 8: Stop Docker Container
  ↓ Get container ID from task ID
  ↓ Try graceful stop (SIGTERM, 10s timeout)
  ↓ Force kill if needed (SIGKILL)
  ↓ Remove container
  ↓ Clean up tracking maps

STEP 9: Worker Cleanup
  ↓ Remove from telemetry monitoring
  ↓ Return success ACK to master

STEP 10: Protected Status (When Worker Reports Completion)
  ↓ Worker eventually calls ReportTaskCompletion
  ↓ Master checks: Is status already "cancelled"?
  ↓   YES → Keep "cancelled" status (don't overwrite)
  ↓   NO → Update to "completed" or "failed"
  ↓ ✅ Cancelled status is preserved!
```

### Why Asynchronous?

**Problem:** Container stopping can take time, causing timeouts

**Solution:** Optimistic update with async notification
- Database updated immediately (source of truth)
- User gets instant response
- Worker processes in background with 30s timeout
- System works even if worker is slow/offline

### Data Flow

```
MongoDB (TASKS Collection)
  status: "running" → "cancelled" ✅
  completed_at: null → "2025-11-16..." ✅

Master In-Memory State
  worker.RunningTasks[taskID] → deleted ✅
  worker.TaskCount → decremented ✅

Worker In-Memory State
  executor.containers[taskID] → deleted ✅
  monitor.tasks[taskID] → deleted ✅

Docker Engine
  Container → SIGTERM → wait 10s → SIGKILL → removed ✅
```

## Implementation Details

### Files Modified

#### 1. `worker/internal/executor/executor.go`
- **Added**: `CancelTask(ctx context.Context, taskID string) error`
- **Purpose**: Stop and remove the Docker container for a task
- **Features**:
  - Graceful stop with 10-second timeout
  - Force kill fallback if graceful stop fails
  - Container removal
  - Task tracking cleanup

#### 2. `worker/internal/server/worker_server.go`
- **Updated**: `CancelTask(ctx context.Context, taskID *pb.TaskID) (*pb.TaskAck, error)`
- **Purpose**: Handle cancellation requests from master
- **Features**:
  - Call executor's CancelTask
  - Remove task from telemetry monitoring
  - Return success/failure acknowledgment
  - Comprehensive logging

#### 3. `master/internal/server/master_server.go`
- **Updated**: `CancelTask(ctx context.Context, taskID *pb.TaskID) (*pb.TaskAck, error)`
- **Purpose**: Orchestrate task cancellation
- **Features**:
  - Find worker running the task (in-memory + database fallback)
  - Update task status to "cancelled" in database
  - Send cancellation request to worker via gRPC
  - Update in-memory worker state
  - Comprehensive error handling and logging

#### 4. `master/internal/cli/cli.go`
- **Added**: `cancel` command case in CLI switch
- **Added**: `cancelTask(taskID string)` method
- **Updated**: `printHelp()` to include cancel command
- **Features**:
  - Beautiful formatted output with ANSI colors
  - Clear success/failure messages
  - 10-second timeout for operation

#### 5. `master/internal/db/tasks.go`
- **Updated**: `UpdateTaskStatus()` to handle "cancelled" status
- **Change**: Added "cancelled" to statuses that set `completed_at` timestamp

## Database Schema

### Task Status Values
- `pending` - Task created but not yet assigned
- `running` - Task is currently executing
- `completed` - Task finished successfully
- `failed` - Task failed with error
- `cancelled` - Task was cancelled by user

### Fields Updated on Cancellation
- `status` → "cancelled"
- `completed_at` → current timestamp

## Error Handling

### Async Processing Model

The cancellation system uses **asynchronous worker notification** for reliability:
- ✅ Database updated immediately (synchronous)
- ✅ User gets instant success response
- ✅ Worker notification in background (30s timeout)
- ✅ System works even if worker times out

### Common Scenarios

#### 1. Task Not Found
**Error:** "Task not found or not running"

**Causes:**
- Task ID doesn't exist
- Task already completed
- Task was never assigned

**Solution:** Verify task ID with `workers` or `stats` command

#### 2. Database Update Failed
**Error:** "Failed to update database: <error>"

**Impact:** Critical - cancellation will not proceed

**Causes:**
- MongoDB connection lost
- Database permissions issue
- Network problem

**Solution:** Check MongoDB status and logs

#### 3. Worker Timeout (Gracefully Handled)
**Log Message:** "⚠ Async: Failed to send cancellation to worker: context deadline exceeded"

**What Happens:**
- Task still marked "cancelled" in database ✅
- User already saw success message ✅
- Worker will sync on next heartbeat
- Container may continue running temporarily

**Impact:** None for user, minimal for system

**Note:** This is expected behavior for slow containers

#### 4. Cancelled Status Overwrite Protection
**Scenario:** Worker reports task completion after cancellation

**Protection:** Master checks existing status before updating
- If status is "cancelled", it's preserved
- Worker's completion report is logged but ignored
- Database remains consistent

**Log Message:** "Task X was cancelled, not updating status to failed"

## Testing

### Manual Testing Steps

1. **Start the system**
   ```bash
   # Terminal 1: Start database
   cd database && docker-compose up -d
   
   # Terminal 2: Start master
   ./runMaster.sh
   
   # Terminal 3: Start worker
   ./runWorker.sh
   ```

2. **Assign a long-running task**
   ```bash
   master> task worker-1 ubuntu:latest
   # Note the task ID
   ```

3. **Cancel the task**
   ```bash
   master> cancel task-1234567890
   ```

4. **Verify cancellation**
   ```bash
   # Check worker logs for container stop messages
   # Check master logs for status update
   # Verify task status in database:
   mongo cloudai_db
   db.TASKS.find({task_id: "task-1234567890"})
   # Should show status: "cancelled"
   ```

### Expected Log Output

**Master:**
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  🛑 CANCELLING TASK
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Task ID: task-1234567890
  Target Worker: worker-1 (localhost:50052)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  ✓ Task status updated to 'cancelled' in database
  ✓ Task cancelled successfully on worker
  ✓ Task removed from worker's running tasks
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**Worker:**
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  🛑 TASK CANCELLATION REQUEST
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Task ID: task-1234567890
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[Task task-1234567890] Cancelling task (container: a1b2c3d4e5f6)...
[Task task-1234567890] ✓ Container stopped gracefully
[Task task-1234567890] ✓ Container removed
[Task task-1234567890] ✓ Task cancelled successfully
  ✓ Task cancelled successfully
  ✓ Container stopped and removed
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Troubleshooting

### Issue: Task Shows "failed" Instead of "cancelled"

**Problem:** Database shows status as "failed" even though you cancelled it

**Root Cause:** Worker reported completion before/after cancellation, overwriting status

**Fix Applied:** Added status protection in `ReportTaskCompletion`
- Master now checks if task is already "cancelled"
- If yes, preserves "cancelled" status
- Rebuild master with latest code

**Verification:**
```bash
# Rebuild master
cd master && go build -o masterNode .

# Restart master and test
```

### Issue: "Failed to send cancellation to worker: context deadline exceeded"

**Problem:** Timeout error when cancelling

**Status:** ✅ **This is now handled gracefully**

**What Happens:**
1. Database updated to "cancelled" ✅
2. User sees success message ✅
3. Worker notification times out (logged)
4. Container will stop eventually

**Action Required:** None - system is working correctly

### Issue: Container Still Running After Cancel

**Check:**
```bash
# On worker machine
docker ps | grep task-123

# Check worker logs
tail -f worker.log
```

**Possible Causes:**
1. Worker offline when cancellation sent
2. Docker daemon slow/unresponsive
3. Container ignoring SIGTERM

**Solution:**
```bash
# Manual cleanup if needed
docker stop -t 0 <container_id>
docker rm -f <container_id>
```

### Issue: Task Not Found

**Error:** "Task not found or not running"

**Debugging:**
```bash
# Check task exists
mongo cloudai_db
db.TASKS.find({task_id: "task-123"})

# Check assignment
db.ASSIGNMENTS.find({task_id: "task-123"})

# Check master state
master> workers
master> stats <worker_id>
```

## Configuration

| Setting | Value | Location | Purpose |
|---------|-------|----------|---------|
| DB Update Timeout | Parent context | `master_server.go` | Database operation timeout |
| Worker Notification Timeout | 30 seconds | `master_server.go` | Async gRPC call timeout |
| Graceful Stop Timeout | 10 seconds | `executor.go` | Container SIGTERM timeout |

## Limitations

1. **No Batch Cancellation**: Must cancel tasks one at a time
2. **No Rollback**: Cancelled tasks cannot be resumed
3. **No Partial Cancellation**: Entire task is cancelled, not individual steps
4. **Container May Run Briefly**: If worker offline, container stops on next sync

## Future Enhancements

1. **Batch Cancellation**: `cancel all` or `cancel worker-1/*`
2. **Offline Worker Handling**: Queue cancellation requests for offline workers
3. **Cancellation Webhooks**: Notify external systems when tasks are cancelled
4. **Cancellation Reasons**: Allow users to specify why task was cancelled
5. **Cancellation History**: Track who cancelled tasks and when

## Related Documentation

- [Task Execution](TASK_EXECUTION_MONITORING_SUMMARY.md)
- [Worker Architecture](worker.md)
- [Database Schema](schema.md)
- [CLI Commands](QUICK_REFERENCE.md)
