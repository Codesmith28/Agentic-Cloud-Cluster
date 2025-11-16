# Task Sending and Receiving - Quick Reference

## Overview
This document describes how tasks are sent from the master node to worker nodes, including all system requirement specifications.

## Task Flow

```
Master Node (CLI) → Master Server → gRPC → Worker Server → Task Execution
```

## Sending Tasks from Master

### Command Syntax
```bash
task <worker_id> <docker_image> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]
```

### Parameters
- `worker_id`: The specific worker to assign the task to (e.g., `worker-1`)
- `docker_image`: Docker image to run (e.g., `docker.io/user/sample-task:latest`)
- `-cpu_cores`: CPU cores to allocate (default: 1.0)
- `-mem`: Memory in GB (default: 0.5)
- `-storage`: Storage in GB (default: 1.0)
- `-gpu_cores`: GPU cores to allocate (default: 0.0)

### Examples

#### Basic Task (default resources)
```bash
master> task worker-1 docker.io/user/sample-task:latest
```
This assigns a task with:
- CPU: 1.0 cores
- Memory: 0.5 GB
- Storage: 1.0 GB
- GPU: 0.0 cores

#### Task with Custom Resources
```bash
master> task worker-2 docker.io/tensorflow/tensorflow:latest -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
```
This assigns a task with:
- CPU: 4.0 cores
- Memory: 8.0 GB
- Storage: 1.0 GB (default)
- GPU: 1.0 cores

#### High-Memory Task
```bash
master> task worker-1 docker.io/user/data-processing:v1 -cpu_cores 2.0 -mem 16.0 -storage 50.0
```

## Task Information Sent

When a task is sent, the following information is transmitted via gRPC:

### Task Message Fields (protobuf)
```protobuf
message Task {
  string task_id = 1;              // Auto-generated: task-<timestamp>
  string docker_image = 2;         // Docker image to run
  string command = 3;              // Docker run command with resource limits
  double req_cpu = 4;              // CPU cores required
  double req_memory = 5;           // Memory in GB required
  double req_storage = 6;          // Storage in GB required
  double req_gpu = 7;              // GPU cores required
  string target_worker_id = 8;     // Target worker ID
}
```

## Master Output

When sending a task, the master displays:

```
═══════════════════════════════════════════════════════
  📤 SENDING TASK TO WORKER
═══════════════════════════════════════════════════════
  Task ID:           task-1730918400
  Target Worker:     worker-1
  Docker Image:      docker.io/user/sample-task:latest
  Command:           docker run --rm --cpus=1.0 --memory=0.5g docker.io/user/sample-task:latest
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════

✅ Task task-1730918400 assigned successfully!
```

The master server also logs:

```
═══════════════════════════════════════════════════════
  📤 TASK ASSIGNED TO WORKER
═══════════════════════════════════════════════════════
  Task ID:           task-1730918400
  Target Worker:     worker-1
  Docker Image:      docker.io/user/sample-task:latest
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════
```

## Worker Reception

When a worker receives a task, it displays:

```
═══════════════════════════════════════════════════════
  📥 TASK RECEIVED FROM MASTER
═══════════════════════════════════════════════════════
  Task ID:           task-1730918400
  Docker Image:      docker.io/user/sample-task:latest
  Command:           docker run --rm --cpus=1.0 --memory=0.5g docker.io/user/sample-task:latest
  Target Worker:     worker-1
───────────────────────────────────────────────────────
  System Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════
  ✓ Task accepted - Starting execution...
═══════════════════════════════════════════════════════
```

Then it proceeds to execute the task:
```
[Task task-1730918400] Starting execution...
[Task task-1730918400] Pulling image: docker.io/user/sample-task:latest
[Task task-1730918400] Creating container...
[Task task-1730918400] Starting container: abc123def456
[Task task-1730918400] Waiting for completion...
[Task task-1730918400] ✓ Completed successfully
```

## Implementation Files

### Master Side
- **CLI Interface**: `master/internal/cli/cli.go`
  - `assignTask()` method parses user input and builds task
  - Displays formatted task information before sending
  
- **Master Server**: `master/internal/server/master_server.go`
  - `AssignTask()` method handles gRPC call to worker
  - Validates worker existence and connectivity
  - Logs task assignment details

### Worker Side
- **Worker Server**: `worker/internal/server/worker_server.go`
  - `AssignTask()` method receives task via gRPC
  - Displays comprehensive task details
  - Adds task to monitoring system
  - Triggers background execution

- **Task Executor**: `worker/internal/executor/executor.go`
  - `ExecuteTask()` method runs Docker container
  - Manages container lifecycle (pull, create, start, wait)
  - Collects logs and reports results

## Protocol Definition

The task sending/receiving protocol is defined in:
- `proto/master_worker.proto`

The gRPC service includes:
```protobuf
service MasterWorker {
  // Master -> Worker
  rpc AssignTask(Task) returns (TaskAck);
}
```

## Testing

### Prerequisites
1. Master node running: `./runMaster.sh`
2. Worker node running: `./runWorker.sh`
3. Worker registered with master

### Test Steps
1. Start master node
2. Start worker node
3. In master CLI, register the worker:
   ```bash
   master> register <worker_id> <worker_ip:port>
   ```
4. Verify worker is active:
   ```bash
   master> workers
   ```
5. Send a test task:
   ```bash
   master> task worker-1 docker.io/library/hello-world:latest
   ```
6. Observe:
   - Master CLI shows task details
   - Master server logs task assignment
   - Worker logs show task reception with all details
   - Worker executes the task

### Expected Result
- Task details are displayed on both master and worker
- All system requirements (CPU, memory, storage, GPU) are shown
- Task executes successfully on the worker
- Completion is reported back to master

## Error Handling

### Common Issues

1. **Worker not registered**
   ```
   ❌ Failed to assign task: Worker worker-1 not found
   ```
   Solution: Register the worker first using `register` command

2. **Worker not active**
   ```
   ❌ Failed to assign task: Worker worker-1 is not active
   ```
   Solution: Ensure worker is running and has sent heartbeats

3. **Master not registered on worker**
   ```
   ❌ Task assignment failed: Master not registered yet
   ```
   Solution: Wait for master registration or restart worker

4. **Connection failed**
   ```
   ❌ Failed to assign task: Failed to connect to worker
   ```
   Solution: Check network connectivity and firewall settings

## Next Steps

- Implement resource availability checking before task assignment
- Add task scheduling algorithms (currently direct assignment only)
- Implement task cancellation support
- Add task result storage and retrieval
- Implement task queuing for busy workers
