# Task Queuing & Scheduling - Quick Reference

## Command Reference

### Submit Task (NEW - No Worker ID Required)
```bash
master> task <docker_image> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]
```

**Examples**:
```bash
# Basic submission (scheduler picks worker)
master> task docker.io/hello-world:latest

# With resource requirements
master> task docker.io/user/task:latest -cpu_cores 2.0 -mem 1.0

# With GPU requirements
master> task docker.io/ml-task:latest -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
```

### View Queue
```bash
master> queue
```

Shows:
- All pending tasks
- Assigned worker (or "Waiting for scheduler")
- Resource requirements
- Time in queue
- Retry attempts
- Status messages

## How It Works

### 1. Task Submission Flow
```
User submits task → Queue (status: queued) → Scheduler → Worker
                      ↓
                 Database persistence
```

### 2. Scheduler (Runs every 5s)
- Checks all queued tasks
- For each task:
  - Finds workers with sufficient resources
  - Selects worker with lowest utilization
  - Assigns task to selected worker
  - Removes from queue if successful

### 3. Selection Algorithm: First-Fit
```
For each worker:
  ✓ Is active?
  ✓ Has IP configured?
  ✓ Has sufficient CPU?
  ✓ Has sufficient Memory?
  ✓ Has sufficient Storage?
  ✓ Has sufficient GPU?
  
  Calculate score: allocated / total_capacity
  
Select worker with LOWEST score (most available resources)
```

## Task Status Flow

```
queued → running → completed/failed
  ↑         ↓
  └─ retry ─┘ (on failure)
```

## Key Concepts

### QueuedTask
```
{
  Task: {TaskID, DockerImage, Resources, ...}
  QueuedAt: timestamp
  Retries: attempt_count
  LastError: "reason or status"
}
```

### Scheduler Behavior
- **Frequency**: Every 5 seconds
- **Order**: FIFO (First-In-First-Out)
- **Strategy**: Lowest utilization first
- **Retry**: Automatic on failure

## Examples

### Example 1: Single Worker
```bash
# Worker-1: 8 CPU, 16 GB RAM available
master> task docker.io/task:latest -cpu_cores 4.0 -mem 8.0

Response:
✅ Task submitted successfully and queued for scheduling!
    Use 'queue' command to view queued tasks

# After ~5 seconds
Log:
✓ Queue: Task task-123 successfully assigned to worker-1 after 1 attempts
```

### Example 2: Multiple Workers (Load Balancing)
```bash
# Worker-1: 50% utilized
# Worker-2: 20% utilized

master> task docker.io/task:latest -cpu_cores 2.0

Result:
→ Scheduler selects worker-2 (less utilized)
✅ Task assigned to worker-2
```

### Example 3: Insufficient Resources
```bash
# All workers busy or lacking resources
master> task docker.io/task:latest -cpu_cores 16.0

Response:
✅ Task submitted successfully and queued for scheduling!

master> queue

Output:
[1] Task ID: task-123
    Assigned Worker: Waiting for scheduler
    ...
    Status:          No suitable worker available with sufficient resources
```

## Monitoring

### Check Queue Status
```bash
master> queue
```

### Check Worker Resources
```bash
master> workers
master> stats worker-1
master> internal-state
```

### Watch Logs
```bash
# Master logs show:
📋 Task task-123 queued: Task submitted to queue for scheduling
✓ Queue: Task task-123 successfully assigned to worker-2 after 1 attempts
```

## Troubleshooting

### Problem: Tasks Stuck in Queue

**Check**:
```bash
master> workers  # Are workers active?
master> queue    # What's the error?
```

**Common Causes**:
- All workers offline
- Insufficient resources across cluster
- No worker meets task requirements

**Solution**:
- Add more workers
- Reduce task resource requirements
- Wait for running tasks to complete

### Problem: Scheduler Not Running

**Symptoms**:
- Queue never processes
- Tasks never assigned

**Check Logs**:
```
✓ Task queue processor started (checking every 5s)
```

**Verify**: Master started successfully

### Problem: Wrong Worker Selected

**Explanation**: Scheduler uses First-Fit with utilization
- Not guaranteed to be optimal
- Prefers less utilized workers
- May not consider data locality or other factors

**Solution**: Future enhancement - configurable scheduling policies

## Configuration

### Change Queue Processor Interval

**File**: `master/internal/server/master_server.go`
```go
func (s *MasterServer) StartQueueProcessor() {
    s.queueTicker = time.NewTicker(5 * time.Second) // Change here
    go s.processQueue()
}
```

### Implement Custom Scheduler

**File**: `master/internal/server/master_server.go`
```go
func (s *MasterServer) selectWorkerForTask(task *pb.Task) string {
    // Your custom logic here
    // Examples: Round-Robin, Random, Priority-based
}
```

## API (For External Clients)

### gRPC Method
```protobuf
rpc AssignTask(Task) returns (TaskAck);
```

**Note**: Still named `AssignTask` for backward compatibility, but internally uses `SubmitTask` + scheduler

### Task Message (No TargetWorkerId Required)
```protobuf
message Task {
    string task_id = 1;
    string docker_image = 2;
    string command = 3;
    double req_cpu = 4;
    double req_memory = 5;
    double req_storage = 6;
    double req_gpu = 7;
    string user_id = 8;
    // target_worker_id not required (set by scheduler)
}
```

## Best Practices

✅ **Don't specify worker IDs** - Let scheduler decide
✅ **Use realistic resource requirements** - Prevents starvation
✅ **Monitor queue regularly** - Use `queue` command
✅ **Check worker health** - Use `workers` command
✅ **Review logs** - Watch for assignment patterns

## Related Commands

```bash
master> help          # All available commands
master> workers       # List all workers
master> stats <id>    # Worker details
master> status        # Cluster status
master> queue         # View queued tasks
master> cancel <id>   # Cancel a task
```

## Further Reading

- [Task Queuing System](TASK_QUEUING_SYSTEM.md) - Complete documentation
- [Implementation Summary](TASK_QUEUING_IMPLEMENTATION_SUMMARY.md) - Technical details
- [Testing Guide](TASK_QUEUING_TESTING.md) - Test scenarios
