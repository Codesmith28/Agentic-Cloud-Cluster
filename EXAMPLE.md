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

To register this worker, run in master CLI:
  master> register <worker-id> <ip>:<port>
```

> **Note:** Copy the worker ID and address shown!

---

## Step 4: Register Worker with Master

**In Terminal 1 (master CLI):**
```bash
master> register <worker-id> <worker-ip:port>
```

**Example:**
```bash
master> register my-laptop 192.168.1.100:50052
```

Expected output:
```
✅ Worker my-laptop registered with address 192.168.1.100:50052
```

**Verify registration:**
```bash
master> workers
```

---

## Step 5: Send a Test Task

### Option A: Simple Public Image Test

```bash
master> task <worker-id> docker.io/library/hello-world:latest
```

**Example:**
```bash
master> task my-laptop docker.io/library/hello-world:latest
```

### Option B: Python Task with Custom Resources

```bash
master> task <worker-id> docker.io/library/python:3.9-slim -cpu_cores 2.0 -mem 1.0
```

**Example:**
```bash
master> task my-laptop docker.io/library/python:3.9-slim -cpu_cores 2.0 -mem 1.0
```

### Option C: Ubuntu Task

```bash
master> task <worker-id> docker.io/library/ubuntu:latest -cpu_cores 1.0 -mem 0.5
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

## Success Criteria ✅

If you see the following, the implementation is working correctly:

- ✅ Master CLI displays task details before sending
- ✅ Master server logs show task assignment with resources
- ✅ Worker receives task and prints Task ID, Docker image, and system requirements
- ✅ Worker successfully executes the Docker container
- ✅ Task completion is reported

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
| `unregister <id>` | Remove a worker |
| `exit` | Shutdown master |

---

## More Task Examples

### High CPU Task
```bash
master> task worker-1 docker.io/library/python:3.9-slim -cpu_cores 4.0 -mem 2.0
```

### High Memory Task
```bash
master> task worker-1 docker.io/library/ubuntu:latest -cpu_cores 2.0 -mem 8.0
```

### GPU Task
```bash
master> task worker-1 docker.io/tensorflow/tensorflow:latest -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
```

### Multiple Tasks
```bash
master> task worker-1 docker.io/library/hello-world:latest
master> task worker-1 docker.io/library/python:3.9-slim
master> task worker-1 docker.io/library/ubuntu:latest
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

🎉 **You're all set! Start testing the task sending/receiving functionality.**
