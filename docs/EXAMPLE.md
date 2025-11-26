# CloudAI - Usage Examples

**Quick examples for testing task sending and receiving functionality.**

---

## Prerequisites

1. Go 1.22+ installed
2. Docker installed and running
3. Python 3.8+ with virtual environment set up
4. Network connectivity between master and worker (if on different machines)

---

## Step 1: Start MongoDB

```bash
cd database
docker-compose up -d
```

---

## Step 2: Start Master Node

**Terminal 1:**
```bash
cd /path/to/CloudAI
source venv/bin/activate  # Activate Python venv
./runMaster.sh
```

Expected output:
```
✓ Master node started successfully
✓ gRPC server listening on <ip>:50051
✓ HTTP server listening on :8080
master>
```

---

## Step 3: Start Worker Node

**Terminal 2:**
```bash
cd /path/to/CloudAI
./runWorker.sh
```

Expected output:
```
Worker ID:      <hostname>
Worker Address: <ip>:<port>

✓ Worker gRPC server started on :50052
✓ Registered with master at localhost:50051
✓ Telemetry monitor started (5s interval)

Waiting for tasks...
```

> **Note:** Worker auto-registers with master on startup!

---

## Step 4: Verify Worker Registration

**In Terminal 1 (master CLI):**
```bash
master> workers
```

Expected output:
```
╔═══ Registered Workers ═══
║ my-laptop
║   Status: 🟢 Active
║   IP: 192.168.1.100:50052
║   Resources: CPU=8.0, Memory=16.0GB, Storage=500.0GB, GPU=0.0
║   Running Tasks: 0
╚═══════════════════════
```

---

## Step 5: Send a Test Task

### Option A: Simple Public Image Test (Scheduler Picks Worker)

```bash
master> task docker.io/library/hello-world:latest
```

**Example:**
```bash
master> task docker.io/library/hello-world:latest
```

### Option B: Task with Custom Name

```bash
master> task docker.io/library/python:3.9-slim -name my-python-test
```

### Option C: Direct Worker Assignment (Dispatch)

```bash
master> dispatch <worker-id> docker.io/library/python:3.9-slim -cpu_cores 2.0 -mem 1.0
```

**Example:**
```bash
master> dispatch my-laptop docker.io/library/python:3.9-slim -cpu_cores 2.0 -mem 1.0
```

### Option D: Ubuntu Task

```bash
master> task docker.io/library/ubuntu:latest -cpu_cores 1.0 -mem 0.5
```

---

## Step 6: Verify Task Execution

### Terminal 1 (Master) - Should show:
```
═══════════════════════════════════════════════════════
  📤 SENDING TASK TO WORKER
═══════════════════════════════════════════════════════
  Task ID:           task-1730918400
  Target Worker:     my-laptop
  Docker Image:      docker.io/library/hello-world:latest
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════

✅ Task task-1730918400 assigned successfully!
```

### Terminal 2 (Worker) - Should show:
```
═══════════════════════════════════════════════════════
  📥 TASK RECEIVED FROM MASTER
═══════════════════════════════════════════════════════
  Task ID:           task-1730918400
  Docker Image:      docker.io/library/hello-world:latest
  Target Worker:     my-laptop
───────────────────────────────────────────────────────
  System Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════
  ✓ Task accepted - Starting execution...
═══════════════════════════════════════════════════════

[Task task-1730918400] Starting execution...
[Task task-1730918400] Pulling image: docker.io/library/hello-world:latest
[Task task-1730918400] ✓ Completed successfully
```

---

## Success Criteria

If you see the following, the implementation is working correctly:

- Master CLI displays task details before sending
- Master server logs show task assignment with resources
- Worker receives task and prints Task ID, Docker image, and system requirements
- Worker successfully executes the Docker container
- Task completion is reported

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| **"Worker not found"** | Register the worker first using the `register` command |
| **"Worker is not active"** | Check that the worker is running and sending heartbeats. Wait a few seconds after registration |
| **"Master not registered yet"** | Wait for master to register with worker (automatic), or restart the worker after master is running |
| **"Failed to pull image"** | Check Docker is running on worker machine. Verify image name is correct. Check network connectivity |
| **"Cannot connect to worker"** | Verify worker IP and port are correct. Check firewall settings. Ensure worker is actually running |

---

## Additional CLI Commands

| Command | Description |
|---------|-------------|
| `help` | Show all available commands |
| `status` | Show cluster status (live view) |
| `workers` | List all workers |
| `stats <worker-id>` | Show detailed worker stats |
| `internal-state` | Dump in-memory state of all workers |
| `fix-resources` | Fix stale resource allocations |
| `list-tasks [status]` | List all tasks (optionally filter by status) |
| `task <image> [options]` | Submit task (scheduler selects worker) |
| `dispatch <worker> <image>` | Dispatch task to specific worker |
| `monitor <task-id>` | Monitor live logs for a task |
| `cancel <task-id>` | Cancel a running task |
| `queue` | Show pending tasks in queue |
| `files <user-id>` | List all files for a user |
| `task-files <task-id> <user>` | View files for a specific task |
| `download <task-id> <user>` | Download task output files |
| `unregister <id>` | Remove a worker |
| `exit` | Shutdown master |

---

## More Task Examples

### High CPU Task
```bash
master> task docker.io/library/python:3.9-slim -name cpu-test -cpu_cores 4.0 -mem 2.0
```

### High Memory Task
```bash
master> task docker.io/library/ubuntu:latest -name mem-test -cpu_cores 2.0 -mem 8.0
```

### GPU Task
```bash
master> task docker.io/tensorflow/tensorflow:latest -name gpu-train -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
```

### Multiple Tasks (Scheduler Distributes)
```bash
master> task docker.io/library/hello-world:latest
master> task docker.io/library/python:3.9-slim
master> task docker.io/library/ubuntu:latest
```

### Check Task Queue
```bash
master> queue
```

### List Tasks by Status
```bash
master> list-tasks            # All tasks categorically
master> list-tasks running    # Running tasks only
master> list-tasks completed  # Completed tasks only
```

---

## REST API Examples

### Submit Task via API
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "docker_image": "ubuntu:latest",
    "command": "echo hello",
    "cpu_required": 1.0,
    "memory_required": 512.0
  }'
```

### List Tasks
```bash
curl http://localhost:8080/api/tasks | jq
```

### Get Telemetry
```bash
curl http://localhost:8080/telemetry | jq
```

### WebSocket Real-Time Streaming
```bash
wscat -c ws://localhost:8080/ws/telemetry
```

---

## Documentation

- **[README.md](README.md)** - Project overview
- **[GETTING_STARTED.md](GETTING_STARTED.md)** - Setup guide
- **[DOCUMENTATION.md](DOCUMENTATION.md)** - Complete reference

---

**You're all set! Start testing the task sending/receiving functionality.**
