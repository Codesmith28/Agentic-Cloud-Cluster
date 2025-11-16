# Direct Task Dispatch Command

## Overview

The `dispatch` command allows you to directly dispatch a task to a specific worker, bypassing the scheduler entirely. This is useful for **testing and debugging purposes**.

## Command Syntax

```bash
dispatch <worker_id> <docker_image> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]
```

### Parameters

- `worker_id` (required): The specific worker ID to dispatch the task to
- `docker_image` (required): Docker image to run
- `-cpu_cores` (optional): CPU cores to allocate (default: 1.0)
- `-mem` (optional): Memory in GB (default: 0.5)
- `-storage` (optional): Storage in GB (default: 1.0)
- `-gpu_cores` (optional): GPU cores to allocate (default: 0.0)

## Examples

### Basic dispatch
```bash
dispatch worker-1 docker.io/user/sample-task:latest
```

### Dispatch with custom resources
```bash
dispatch worker-2 docker.io/user/ml-task:latest -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
```

### Dispatch with all options
```bash
dispatch worker-1 alpine:latest -cpu_cores 2.0 -mem 2.0 -storage 5.0 -gpu_cores 0.5
```

## How It Works

### Behind the Scenes

1. **CLI Validation**: The CLI validates the command arguments and packages the task specification

2. **Task Creation**: A unique task ID is generated (format: `task-<timestamp>`)

3. **Database Storage**: The task is stored in the database with status "queued"

4. **Direct Assignment**: The master calls `DispatchTaskToWorker()` which:
   - Skips the task queue entirely
   - Skips the scheduler
   - Directly calls `assignTaskToWorker()` with the specified worker ID

5. **Worker Validation**: Before assignment, the system checks:
   - Worker exists and is active
   - Worker has sufficient CPU resources
   - Worker has sufficient memory
   - Worker has sufficient storage
   - Worker has sufficient GPU resources

6. **Task Dispatch**: If validation passes:
   - Master connects to the worker via gRPC
   - Task is sent to the worker
   - Worker resources are allocated
   - Task status is updated to "running"

7. **Error Handling**: If dispatch fails:
   - Task status is set to "failed"
   - Error message is returned to CLI
   - Resources are not allocated

## Difference from `task` Command

| Feature | `task` Command | `dispatch` Command |
|---------|----------------|-------------------|
| **Worker Selection** | Scheduler selects best worker | User specifies worker |
| **Queue** | Task enters queue | Bypasses queue |
| **Scheduling** | Goes through scheduler | Bypasses scheduler |
| **Use Case** | Production workload | Testing/debugging |
| **Resource Check** | Done by scheduler | Done at dispatch time |

## Use Cases

### Testing Specific Workers
```bash
# Test if worker-1 can handle a specific task
dispatch worker-1 nginx:latest -cpu_cores 1.0 -mem 0.5
```

### Debugging Worker Issues
```bash
# Send a diagnostic task to a problematic worker
dispatch worker-3 busybox:latest -cpu_cores 0.5 -mem 0.25
```

### Load Testing Specific Workers
```bash
# Send multiple tasks to the same worker to test capacity
dispatch worker-2 stress-ng:latest -cpu_cores 2.0 -mem 4.0
```

### Development Testing
```bash
# Test a new worker immediately after registration
register worker-4 192.168.1.104:50052
dispatch worker-4 hello-world:latest
```

## Monitoring Dispatched Tasks

After dispatching a task, you can monitor it using:

```bash
monitor <task_id>
```

Example:
```bash
master> dispatch worker-1 alpine:latest
✅ Task task-1731629400 dispatched directly to worker worker-1!

master> monitor task-1731629400
```

## Error Scenarios

### Worker Not Found
```bash
master> dispatch worker-999 alpine:latest
❌ Failed to dispatch task: Worker worker-999 not found
```

### Insufficient Resources
```bash
master> dispatch worker-1 alpine:latest -cpu_cores 100.0
❌ Failed to dispatch task: Insufficient CPU: worker has 4.0 available, task requires 100.0
```

### Worker Not Active
```bash
master> dispatch worker-2 alpine:latest
❌ Failed to dispatch task: Worker worker-2 is not active
```

### Connection Failure
```bash
master> dispatch worker-1 alpine:latest
❌ Failed to dispatch task: Failed to connect to worker: connection refused
```

## Best Practices

1. **Use for Testing Only**: The `dispatch` command bypasses important scheduling logic and should primarily be used for testing and debugging

2. **Check Worker Status First**: Before dispatching, check worker status:
   ```bash
   workers
   stats worker-1
   ```

3. **Monitor Resource Usage**: Ensure the worker has sufficient resources:
   ```bash
   stats <worker_id>
   ```

4. **Avoid Oversubscription**: Be careful not to oversubscribe workers by dispatching too many tasks directly

5. **Use `task` for Production**: For production workloads, use the `task` command which leverages the scheduler for optimal placement

## Implementation Details

### Code Flow

```
CLI (cli.go)
  ↓
dispatchTask() - Parse arguments and create task
  ↓
dispatchTaskToWorker() - Call master server
  ↓
MasterServer.DispatchTaskToWorker() - Store in DB, bypass queue/scheduler
  ↓
assignTaskToWorker() - Validate resources and assign to worker
  ↓
Worker receives task via gRPC
```

### Key Functions

- **CLI**: `dispatchTask()` in `master/internal/cli/cli.go`
- **Server**: `DispatchTaskToWorker()` in `master/internal/server/master_server.go`
- **Assignment**: `assignTaskToWorker()` in `master/internal/server/master_server.go`

## Related Commands

- `task` - Submit task with scheduler selection
- `workers` - List all registered workers
- `stats <worker_id>` - Show worker resource usage
- `monitor <task_id>` - Monitor task execution
- `cancel <task_id>` - Cancel a running task
- `queue` - Show pending tasks in queue
