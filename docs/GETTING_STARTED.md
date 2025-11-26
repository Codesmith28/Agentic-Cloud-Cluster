# CloudAI - Getting Started Guide

**Quick guide to get CloudAI up and running in 5 minutes.**

---

## TL;DR (Ultra Quick Start)

```bash
# 1. Setup (one-time)
make setup

# 2. Build
make all

# 3. Start services (in separate terminals)
cd database && docker-compose up -d                     # Terminal 1
./runMaster.sh                                          # Terminal 2 (includes Web UI)
./runWorker.sh                                          # Terminal 3

# 4. Use it!
# In master CLI:
master> workers
master> task worker-1 hello-world:latest
master> monitor task-<id>

# Or open the Web UI at http://localhost:3000
```

---

## Prerequisites Checklist

Ensure you have:

- ✅ **Go 1.22+**: `go version`
- ✅ **Docker**: `docker --version`
- ✅ **Docker Compose**: `docker-compose --version`
- ✅ **Protocol Buffers**: `protoc --version`
- ✅ **Node.js 18+**: `node --version` (for Web UI)
- ✅ **Python 3.8+**: `python3 --version` (for future agent extensibility)

### Install Missing Tools

**Go:**
```bash
# Download from https://go.dev/dl/
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

**Docker:**
```bash
# Ubuntu/Debian
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER
# Logout and login
```

**Protocol Buffers:**
```bash
# Ubuntu/Debian
sudo apt-get install -y protobuf-compiler

# macOS
brew install protobuf
```

**Go gRPC Plugins:**
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH=$PATH:$(go env GOPATH)/bin
```

---

## Installation Steps

### Step 1: Clone Repository

```bash
git clone https://github.com/Codesmith28/CloudAI.git
cd CloudAI
```

### Step 2: Set Up Python Environment

```bash
# Create and activate virtual environment (for future Python agent extensibility)
python3 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -r requirements.txt
```

This installs gRPC dependencies needed for Python agents via `master_agent.proto`.

### Step 3: One-Time Setup

```bash
# Generates proto code, creates symlinks, installs dependencies
make setup
```

This command:
- Generates Go code from `.proto` files
- Creates symlinks in master/worker directories
- Installs Go module dependencies

### Step 4: Build Binaries

```bash
# Build both master and worker
make all

# Or build individually:
make master   # Creates master/masterNode
make worker   # Creates worker/workerNode
```

### Step 5: Install UI Dependencies (Optional)

```bash
# Only needed if you want to use the Web UI
cd ui
npm install
cd ..
```

---

## Running the System

### Terminal 1: Start MongoDB

```bash
cd database
docker-compose up -d

# Verify it's running
docker-compose ps
```

Expected output:
```
       Name                     Command             State           Ports
---------------------------------------------------------------------------------
mongodb            docker-entrypoint.sh mongod   Up      0.0.0.0:27017->27017/tcp
```

### Terminal 2: Start Master Node

```bash
# Use the convenience script (also starts Web UI)
./runMaster.sh

# Or run manually:
cd master && ./masterNode
```

Expected output:
```
═══════════════════════════════════════════════════════
  CloudAI Master Node - Interactive CLI
═══════════════════════════════════════════════════════
✓ Master node started successfully
✓ gRPC server listening on 192.168.1.10:50051
✓ HTTP server listening on :8080
✓ Database connected successfully

Type 'help' for available commands

master>
```

### Terminal 3: Start Worker Node

```bash
# Use the convenience script
./runWorker.sh

# Or run manually:
cd worker && ./workerNode
```

Expected output:
```
Worker ID:      hostname-abc123
Worker Address: 192.168.1.100:50052

✓ Worker gRPC server started on :50052
✓ Registered with master at localhost:50051
✓ Telemetry monitor started (5s interval)

Waiting for tasks...
```

---

## Your First Task

Now that everything is running, let's submit and monitor a task!

### Step 1: Check Cluster Status

```bash
# In master CLI (Terminal 2)
master> status
```

Output (live view - refreshes automatically):
```
╔═══════════════════════════════════════╗
║    Live Cluster Status Monitor        ║
║    Press any key to exit...           ║
╚═══════════════════════════════════════╝

╔═══ Cluster Status ═══
║ Total Workers: 1
║ Active Workers: 1
║ Running Tasks: 0
╚══════════════════════
```

### Step 2: List Workers

```bash
master> workers
```

Output:
```
╔═══ Registered Workers ═══
║ hostname-abc123
║   Status: 🟢 Active
║   IP: 192.168.1.100:50052
║   Resources: CPU=8.0, Memory=16.0GB, Storage=500.0GB, GPU=0.0
║   Running Tasks: 0
║   Last Heartbeat: 2s ago
╚═══════════════════════
```

### Step 3: Submit a Task (Scheduler Auto-selects Worker)

```bash
# The scheduler automatically picks the best available worker
master> task hello-world:latest
```

Output:
```
✓ Task created successfully!
  Task ID: task-1731677400
  Image: hello-world:latest
  Resources: CPU=1.0, Memory=0.5GB

Task queued for scheduling...
```

### Step 3 (Alternative): Direct Worker Assignment

```bash
# Use dispatch to manually select a worker
master> dispatch hostname-abc123 hello-world:latest
```

### Step 4: Monitor Task

```bash
master> monitor task-1731677400
```

Output:
```
╔═══ Task Monitor ═══
║ Task ID: task-1731677400
║ Status: Running
║ Worker: hostname-abc123
║ Image: hello-world:latest
╚════════════════════

─── Logs ───
Pulling image hello-world:latest...
Image pulled successfully
Starting container...
Container started: abc123def456

Hello from Docker!
This message shows that your installation appears to be working correctly.

Exit Code: 0
Status: Completed

Press any key to exit...
```

**Success!** You've just run your first distributed task!

---

## Try More Examples

### Example 1: Task with Custom Name and Resources

```bash
master> task python:3.9 -name my-python-task -cpu_cores 2.0 -mem 4.0
```

### Example 2: View Task Queue

```bash
master> queue
```

### Example 3: List Tasks by Status

```bash
master> list-tasks            # Show all tasks categorically
master> list-tasks running    # Filter by status
master> list-tasks completed
```

### Example 4: Cancel a Task

```bash
master> task ubuntu:latest
master> cancel task-<id>
```

### Example 5: Multiple Workers

```bash
# Start another worker in Terminal 4
cd worker
./workerNode -id worker-2 -port :50053

# Back in master CLI
master> workers
# You should see two workers now!

# Submit tasks - scheduler distributes them
master> task python:3.9
master> task node:18
```

### Example 6: File Management

```bash
# List files for a user
master> files alice

# View files for a specific task
master> task-files task-123 alice

# Download task output files
master> download task-123 alice ./output-dir
```

---

## Web Dashboard

The Web UI is available at **http://localhost:3000** when using `./runMaster.sh`.

Features:
- **Dashboard**: Real-time cluster overview
- **Tasks**: Submit, monitor, and manage tasks
- **Workers**: View worker status and resources
- **Authentication**: Login/Register for user accounts

---

## HTTP API Usage

### Authentication

```bash
# Register a new user
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"John","email":"john@example.com","password":"secret123"}'

# Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"john@example.com","password":"secret123"}'
```

### Task Management

```bash
# Submit a new task (with tag and k-value for scheduling)
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "docker_image": "ubuntu:latest",
    "command": "echo hello",
    "cpu_required": 1.0,
    "memory_required": 512.0,
    "tag": "cpu-light",
    "k_value": 2.0
  }' | jq

# List all tasks
curl http://localhost:8080/api/tasks | jq

# Filter by status
curl http://localhost:8080/api/tasks?status=running | jq

# Get task details
curl http://localhost:8080/api/tasks/task-123 | jq

# Get task logs
curl http://localhost:8080/api/tasks/task-123/logs | jq

# Cancel task
curl -X DELETE http://localhost:8080/api/tasks/task-123 | jq
```

### Worker Management

```bash
# List all workers with telemetry
curl http://localhost:8080/api/workers | jq

# Get worker details
curl http://localhost:8080/api/workers/worker-1 | jq

# Get worker metrics
curl http://localhost:8080/api/workers/worker-1/metrics | jq
```

### Telemetry

```bash
# Health check
curl http://localhost:8080/health

# All workers telemetry
curl http://localhost:8080/telemetry | jq

# Specific worker telemetry
curl http://localhost:8080/telemetry/worker-1 | jq
```

### WebSocket Real-Time Streaming

```bash
# Install wscat
npm install -g wscat

# Connect to telemetry stream
wscat -c ws://localhost:8080/ws/telemetry

# Specific worker
wscat -c ws://localhost:8080/ws/telemetry/worker-1
```

---

## Troubleshooting

### Problem: "protoc: command not found"

**Solution:**
```bash
# Ubuntu/Debian
sudo apt-get install -y protobuf-compiler

# macOS
brew install protobuf
```

### Problem: "Worker not connecting"

**Solution:**
```bash
# 1. Check master is running
ps aux | grep masterNode

# 2. Check port is open
netstat -tuln | grep 50051

# 3. Check firewall
sudo ufw allow 50051/tcp

# 4. Verify master address
export MASTER_ADDR=localhost:50051
./workerNode
```

### Problem: "MongoDB connection failed"

**Solution:**
```bash
# 1. Check MongoDB is running
cd database
docker-compose ps

# 2. Restart MongoDB
docker-compose restart

# 3. Check logs
docker-compose logs mongodb
```

### Problem: "Docker permission denied"

**Solution:**
```bash
# Add your user to docker group
sudo usermod -aG docker $USER
# Logout and login again
```

### Problem: "Task execution failed"

**Solution:**
```bash
# 1. Check if image exists
docker pull <image-name>

# 2. View task logs
master> monitor task-<id>

# 3. Test image locally
docker run --rm <image-name>
```

---

## Next Steps

1. **[DOCUMENTATION.md](DOCUMENTATION.md)** - Complete API and feature reference
2. **[EXAMPLE.md](EXAMPLE.md)** - More usage examples
3. **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture details

### Advanced Features to Explore

1. **Scale horizontally** - Add more workers
2. **Use the Web UI** - Full dashboard at http://localhost:3000
3. **File management** - Upload/download task outputs
4. **Task queuing** - Automatic queuing when resources are full
5. **Resource management** - `fix-resources` and `internal-state` commands

---

## Congratulations!

You have a fully functional distributed task execution system!

**What you can do:**
- ✅ Submit Docker-based tasks (with automatic scheduling)
- ✅ Monitor tasks in real-time
- ✅ Scale with multiple workers
- ✅ Use the Web Dashboard
- ✅ Access via CLI, HTTP, and WebSocket APIs
- ✅ Manage task output files
- ✅ User authentication

**Happy distributed computing!**
