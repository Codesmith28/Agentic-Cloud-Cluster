# CloudAI - Getting Started Guide

**Quick guide to get CloudAI up and running in 5 minutes.**

---

## ⚡ TL;DR (Ultra Quick Start)

```bash
# 1. Setup (one-time)
make setup

# 2. Build
make all

# 3. Start services (in separate terminals)
cd database && docker-compose up -d                     # Terminal 1
cd master && ./masterNode                               # Terminal 2
cd worker && ./workerNode                               # Terminal 3

# 4. Use it!
# In master CLI:
master> workers
master> task worker-1 hello-world:latest
master> monitor task-<id>
```

---

## 📋 Prerequisites Checklist

Ensure you have:

- ✅ **Go 1.22+**: `go version`
- ✅ **Docker**: `docker --version`
- ✅ **Docker Compose**: `docker-compose --version`
- ✅ **Protocol Buffers**: `protoc --version`
- ✅ **Python 3.8+**: `python3 --version` (for AI Scheduler)

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

## 🚀 Installation Steps

### Step 1: Clone Repository

```bash
git clone --recurse-submodules https://github.com/Codesmith28/CloudAI.git
cd CloudAI
```

### Step 2: One-Time Setup

```bash
# Generates proto code, creates symlinks, installs dependencies
make setup
```

This command:
- Generates Go code from `.proto` files
- Generates Python code from `.proto` files
- Creates symlinks in master/worker/agentic_scheduler
- Installs Go module dependencies

### Step 3: Build Binaries

```bash
# Build both master and worker
make all

# Or build individually:
make master   # Creates master/masterNode
make worker   # Creates worker/workerNode
```

### Step 4: Install Python Dependencies (Optional)

```bash
# Only needed if using AI Scheduler
cd agentic_scheduler
pip install -r requirements.txt
cd ..
```

---

## 🎮 Running the System

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
cd master
./masterNode

# Or use convenience script
cd ..
./runMaster.sh
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
cd worker
./workerNode

# Or use convenience script with custom settings
cd ..
./runWorker.sh
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

## 🎯 Your First Task

Now that everything is running, let's submit and monitor a task!

### Step 1: Check Cluster Status

```bash
# In master CLI (Terminal 2)
master> status
```

Output:
```
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

### Step 3: Submit a Task

```bash
master> task hostname-abc123 hello-world:latest
```

Output:
```
✓ Task created successfully!
  Task ID: task-1731677400
  Worker: hostname-abc123
  Image: hello-world:latest
  Resources: CPU=1.0, Memory=0.5GB

Task submitted to worker...
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

**Success!** You've just run your first distributed task! 🎉

---

## 🧪 Try More Examples

### Example 1: Python Task with Resources

```bash
master> task hostname-abc123 python:3.9 -cpu_cores 2.0 -mem 4.0
```

### Example 2: Long-Running Task

```bash
master> task hostname-abc123 ubuntu:latest -cpu_cores 1.0 -mem 1.0
master> monitor task-<id>
# Press Ctrl+C to exit monitoring (task continues running)
```

### Example 3: Cancel a Task

```bash
master> task hostname-abc123 ubuntu:latest
master> cancel task-<id>
```

### Example 4: Multiple Workers

```bash
# Start another worker in Terminal 4
cd worker
./workerNode -id worker-2 -port :50053

# Back in master CLI
master> workers
# You should see two workers now!

# Submit tasks to different workers
master> task worker-1 python:3.9
master> task worker-2 node:18
```

---

## 🤖 Using the AI Scheduler (Optional)

The AI Scheduler provides intelligent task assignment using multiple optimization strategies.

### Step 1: Prepare Test Data

```bash
cd agentic_scheduler

# Test data is already provided in tests/ directory:
# - tests/workers.csv
# - tests/tasks1.csv
```

### Step 2: Run the Scheduler

```bash
# Multi-Objective (recommended)
python main.py ai

# Aggressive Utilization (max resources)
python main.py ai_aggressive

# Predictive (minimize completion time)
python main.py ai_predictive

# Compare all strategies
python test_schedulers.py
```

### Step 3: View Results

```bash
ls -la reports/

# Each strategy has a directory with:
# - performance_metrics.txt
# - assignments.csv
# - plots (if matplotlib installed)
```

---

## 🌐 Monitoring & API Usage

### HTTP REST API - Telemetry

```bash
# Health check
curl http://localhost:8080/health

# All workers telemetry
curl http://localhost:8080/telemetry | jq

# Specific worker
curl http://localhost:8080/telemetry/worker-1 | jq

# Workers list
curl http://localhost:8080/workers | jq
```

### HTTP REST API - Task Management

```bash
# Submit a new task
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "docker_image": "ubuntu:latest",
    "command": "echo hello",
    "cpu_required": 1.0,
    "memory_required": 512.0
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

### HTTP REST API - Worker Management

```bash
# List all workers
curl http://localhost:8080/api/workers | jq

# Get worker details
curl http://localhost:8080/api/workers/worker-1 | jq

# Get worker metrics
curl http://localhost:8080/api/workers/worker-1/metrics | jq

# Get worker's tasks
curl http://localhost:8080/api/workers/worker-1/tasks | jq
```

### WebSocket Real-Time Streaming

**Install wscat (Node.js):**
```bash
npm install -g wscat
```

**Connect to telemetry stream:**
```bash
# All workers
wscat -c ws://localhost:8080/ws/telemetry

# Specific worker
wscat -c ws://localhost:8080/ws/telemetry/worker-1
```

You'll see real-time telemetry updates every 5 seconds!

---

## 🔍 Troubleshooting

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
# In worker terminal, set explicitly:
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

# 4. Verify port
docker-compose port mongodb 27017
```

### Problem: "Docker permission denied"

**Solution:**
```bash
# Add your user to docker group
sudo usermod -aG docker $USER

# Logout and login again (or restart)
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

# 4. Check worker logs
# In worker terminal, look for error messages
```

---

## 📚 Next Steps

Now that you have CloudAI running, explore:

1. **[README.md](README.md)** - Full feature overview
2. **[DOCUMENTATION.md](DOCUMENTATION.md)** - Comprehensive documentation (50+ pages)
3. **[docs/architecture.md](docs/architecture.md)** - System architecture
4. **[docs/TELEMETRY_QUICK_REFERENCE.md](docs/TELEMETRY_QUICK_REFERENCE.md)** - Telemetry system
5. **[agentic_scheduler/AI_SCHEDULER_USAGE.md](agentic_scheduler/AI_SCHEDULER_USAGE.md)** - AI scheduler guide

### Learn More

- **Submitting tasks**: See [docs/TASK_EXECUTION_QUICK_REFERENCE.md](docs/TASK_EXECUTION_QUICK_REFERENCE.md)
- **Monitoring**: See [docs/WEBSOCKET_TELEMETRY.md](docs/WEBSOCKET_TELEMETRY.md)
- **Database**: See [docs/DATABASE_WORKER_REGISTRY.md](docs/DATABASE_WORKER_REGISTRY.md)
- **Development**: See [DOCUMENTATION.md](DOCUMENTATION.md) Section 11

### Try Advanced Features

1. **Add more workers** to scale horizontally
2. **Use AI Scheduler** for optimal task placement
3. **Build custom Docker tasks** for your workload
4. **Integrate WebSocket API** into your dashboard
5. **Test task cancellation** functionality

---

## 🎉 Congratulations!

You now have a fully functional distributed task execution system! 

**What you can do:**
- ✅ Submit Docker-based tasks
- ✅ Monitor tasks in real-time
- ✅ Scale with multiple workers
- ✅ Use AI-powered scheduling
- ✅ Access via CLI, HTTP, and WebSocket APIs

**Need help?**
- 📖 Check [DOCUMENTATION.md](DOCUMENTATION.md)
- 🐛 Report issues on [GitHub](https://github.com/Codesmith28/CloudAI/issues)
- 💬 Join discussions on [GitHub Discussions](https://github.com/Codesmith28/CloudAI/discussions)

**Happy distributed computing! 🚀**
