# CloudAI Master-Worker System - Setup & Testing Guide

This guide provides comprehensive step-by-step instructions to build, run, and test the CloudAI distributed system.

## 📋 Table of Contents

1. [Prerequisites](#prerequisites)
2. [Project Structure](#project-structure)
3. [Step 1: Generate gRPC Code](#step-1-generate-grpc-code)
4. [Step 2: Build Sample Docker Task](#step-2-build-sample-docker-task)
5. [Step 3: Start MongoDB](#step-3-start-mongodb)
6. [Step 4: Build and Run Master](#step-4-build-and-run-master)
7. [Step 5: Build and Run Worker](#step-5-build-and-run-worker)
8. [Step 6: Test the System](#step-6-test-the-system)
9. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Tools

Install the following tools before proceeding:

```bash
# 1. Go (version 1.22+)
go version  # Should output: go version go1.22.x ...

# 2. Protocol Buffers Compiler
sudo apt-get install -y protobuf-compiler  # Ubuntu/Debian
# OR
brew install protobuf  # macOS

protoc --version  # Should output: libprotoc 3.x.x or higher

# 3. Go gRPC plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 4. Python gRPC tools (for future agent development)
pip install grpcio grpcio-tools

# 5. Docker
docker --version  # Should output: Docker version 24.x.x or higher

# 6. Docker Hub account (for pushing sample task)
# Sign up at https://hub.docker.com if you don't have one
```

### Environment Setup

Ensure Go binaries are in your PATH:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

Add this to your `~/.bashrc` or `~/.zshrc` to make it permanent.

---

## Project Structure

```
CloudAI/
├── proto/                      # Protocol buffer definitions
│   ├── master_worker.proto    # Master ↔ Worker communication
│   ├── master_agent.proto     # Master ↔ Agent communication
│   ├── generate.sh            # Script to generate gRPC code
│   ├── pb/                    # Generated Go code (created by script)
│   └── py/                    # Generated Python code (created by script)
│
├── master/                     # Master node
│   ├── main.go                # Entry point
│   ├── go.mod                 # Go dependencies
│   ├── internal/
│   │   ├── server/            # gRPC server implementation
│   │   │   └── master_server.go
│   │   ├── cli/               # Command-line interface
│   │   │   └── cli.go
│   │   └── db/                # MongoDB integration
│   │       └── init.go
│   └── proto/                 # Symlink to ../proto/pb
│
├── worker/                     # Worker node
│   ├── main.go                # Entry point
│   ├── go.mod                 # Go dependencies
│   ├── internal/
│   │   ├── server/            # gRPC server implementation
│   │   │   └── worker_server.go
│   │   ├── executor/          # Docker task execution
│   │   │   └── executor.go
│   │   └── telemetry/         # Heartbeat & monitoring
│   │       └── telemetry.go
│   └── proto/                 # Symlink to ../proto/pb
│
├── sample_task/                # Sample Docker task
│   ├── task.py                # Python task script
│   ├── Dockerfile             # Container definition
│   └── README.md
│
└── database/                   # MongoDB setup
    └── docker-compose.yml
```

---

## Step 1: Generate gRPC Code

Generate Go and Python code from proto definitions:

```bash
cd proto
chmod +x generate.sh
./generate.sh
```

**Expected Output:**

```
Generating gRPC code from proto files...
→ Generating Go code for master_worker.proto (Go ↔ Go)...
→ Generating Go code for master_agent.proto (Master side - Go)...
→ Generating Python code for master_agent.proto (Agent side - Python)...
✓ gRPC code generation complete!
  Go files:     ./pb/
  Python files: ./py/
```

**Verify:**

```bash
ls -la proto/pb/
# Should show: master_worker.pb.go, master_worker_grpc.pb.go,
#              master_agent.pb.go, master_agent_grpc.pb.go

ls -la proto/py/
# Should show: master_agent_pb2.py, master_agent_pb2_grpc.py
```

---

## Step 2: Build Sample Docker Task

Create a sample task container for testing:

```bash
cd sample_task

# Build the Docker image
docker build -t <your-dockerhub-username>/cloudai-sample-task:latest .

# Test locally (optional but recommended)
docker run --rm <your-dockerhub-username>/cloudai-sample-task:latest
```

**Expected Output:**

```
==================================================
CloudAI Sample Task - Starting Execution
==================================================

[Step 1/3] Initializing task environment...
[Step 2/3] Processing data...
  → Processing 87 data points...
  → Progress: 0/87
  → Progress: 20/87
  ...
✓ Task completed successfully!
```

**Push to Docker Hub:**

```bash
docker login
# Enter your Docker Hub credentials

docker push <your-dockerhub-username>/cloudai-sample-task:latest
```

**Important:** Remember your full image name:
`docker.io/<your-dockerhub-username>/cloudai-sample-task:latest`

---

## Step 3: Start MongoDB

The master node optionally uses MongoDB to store worker registry and task data.

```bash
cd database
docker-compose up -d
```

**Verify:**

```bash
docker ps
# Should show MongoDB container running on port 27017
```

**Create `.env` file** in the project root with MongoDB credentials:

```bash
cd ..  # Back to project root
cat > .env << EOF
MONGODB_USERNAME=admin
MONGODB_PASSWORD=password123
EOF
```

---

## Step 4: Build and Run Master

### Create Symlink to Proto Files

```bash
cd master
ln -s ../proto/pb proto
```

### Install Dependencies

```bash
go mod tidy
```

This will download:

- `google.golang.org/grpc`
- `google.golang.org/protobuf`
- MongoDB driver
- Other dependencies

### Build the Master

```bash
go build -o master-node .
```

### Run the Master

```bash
./master-node
```

**Expected Output:**

```
✓ MongoDB collections ensured
Starting gRPC server on :50051...

✓ Master node started successfully
✓ gRPC server listening on :50051

═══════════════════════════════════════════════════════
  CloudAI Master Node - Interactive CLI
═══════════════════════════════════════════════════════
Type 'help' for available commands

master>
```

### Master CLI Commands

```bash
master> help          # Show available commands
master> status        # Show cluster status
master> workers       # List registered workers
master> task <worker_id> <docker_image>  # Assign task
master> exit          # Shutdown master
```

---

## Step 5: Build and Run Worker

**Open a new terminal** while keeping the master running.

### Create Symlink to Proto Files

```bash
cd worker
ln -s ../proto/pb proto
```

### Install Dependencies

```bash
go mod tidy
```

This will download:

- `google.golang.org/grpc`
- Docker SDK for Go
- Other dependencies

### Build the Worker

```bash
go build -o worker-node .
```

### Run the Worker

```bash
./worker-node -id worker-1 -ip localhost -master localhost:50051
```

**Command-line Flags:**

- `-id`: Worker identifier (default: worker-1)
- `-ip`: Worker IP address (default: localhost)
- `-master`: Master server address (default: localhost:50051)
- `-port`: Worker gRPC port (default: :50052)

**Expected Output:**

```
Starting Worker Node: worker-1
Master Address: localhost:50051
✓ Worker registered: Worker registered successfully
Starting telemetry monitor (interval: 5s)
✓ Worker worker-1 started successfully
✓ gRPC server listening on :50052
✓ Ready to receive tasks...
Heartbeat sent: CPU=30.0%, Memory=45.2MB, Tasks=0
```

**In the Master terminal**, you should see:

```
Worker registration: worker-1 (IP: localhost, CPU: 4.00, Memory: 8.00 GB)
Heartbeat from worker-1: CPU=30.00%, Memory=45.20MB, Running Tasks=0
```

---

## Step 6: Test the System

### Test 1: Check Worker Registration

In the master CLI:

```bash
master> workers
```

**Expected Output:**

```
╔═══ Registered Workers ═══
║ worker-1
║   Status: 🟢 Active
║   IP: localhost
║   Resources: CPU=4.0, Memory=8.0GB, GPU=0.0
║   Running Tasks: 0
║
╚═══════════════════════════
```

### Test 2: Assign a Task

In the master CLI:

```bash
master> task worker-1 docker.io/<your-dockerhub-username>/cloudai-sample-task:latest
```

**Expected Output (Master):**

```
Assigning task to worker worker-1...
Docker Image: docker.io/<username>/cloudai-sample-task:latest
✓ Task task-1697123456 assigned successfully!
```

**Expected Output (Worker Terminal):**

```
Received task assignment: task-1697123456 (Image: docker.io/...)
[Task task-1697123456] Starting execution...
[Task task-1697123456] Pulling image: docker.io/.../cloudai-sample-task:latest
[Task task-1697123456] Creating container...
[Task task-1697123456] Starting container: abc123def456
[Task task-1697123456] Waiting for completion...
[Task task-1697123456] ✓ Completed successfully
✓ Task result reported: Task result received
```

**Expected Output (Master Terminal):**

```
Task completion: task-1697123456 from worker worker-1 [Status: success]
Task logs:
==================================================
CloudAI Sample Task - Starting Execution
==================================================

[Step 1/3] Initializing task environment...
[Step 2/3] Processing data...
  → Processing 87 data points...
  → Progress: 0/87
  ...
✓ Task completed successfully!
```

### Test 3: Check Cluster Status

```bash
master> status
```

**Expected Output:**

```
╔═══ Cluster Status ═══
║ Total Workers: 1
║ Active Workers: 1
║ Running Tasks: 0
╚══════════════════════════
```

### Test 4: Multiple Workers (Optional)

Open another terminal and start a second worker:

```bash
cd worker
./worker-node -id worker-2 -ip localhost -master localhost:50051 -port :50053
```

Now try:

```bash
master> workers
master> task worker-2 docker.io/<username>/cloudai-sample-task:latest
```

---

## Troubleshooting

### Problem: "protoc: command not found"

**Solution:**

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y protobuf-compiler

# macOS
brew install protobuf
```

### Problem: "protoc-gen-go: program not found"

**Solution:**

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Add to PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

### Problem: "Failed to register with master: connection refused"

**Solution:**

- Ensure master is running first
- Check master is listening on `:50051`
- Check firewall settings if running on separate machines

### Problem: "Failed to pull image: unauthorized"

**Solution:**

- Ensure Docker image is public on Docker Hub
- OR login to Docker on worker machine: `docker login`

### Problem: "Worker not receiving heartbeat acknowledgments"

**Solution:**

- Check network connectivity between master and worker
- Verify master address in worker command: `-master localhost:50051`
- Check master logs for errors

### Problem: "Task fails with 'Failed to create docker client'"

**Solution:**

- Ensure Docker daemon is running: `sudo systemctl start docker`
- Add user to docker group: `sudo usermod -aG docker $USER`
- Restart terminal after adding to docker group

### Problem: "MongoDB connection failed"

**Solution:**

- Check MongoDB is running: `docker ps`
- Verify `.env` file has correct credentials
- Master will continue working without MongoDB (warning shown)

---

## Next Steps

### 1. Implement Result Storage

Currently results are logged but not stored. Add MongoDB integration:

- Store task results in `RESULTS` collection
- Update `master/internal/server/master_server.go` in `ReportTaskCompletion`

### 2. Add AI Agent

Create a Python agent that:

- Fetches cluster state from master
- Makes intelligent task assignment decisions
- Uses the `master_agent.proto` gRPC service

### 3. Enhanced Resource Monitoring

Improve worker telemetry:

- Real CPU usage (use `github.com/shirou/gopsutil`)
- Real memory monitoring
- Disk I/O stats
- GPU utilization (if available)

### 4. Task Queuing

Add task queue in master:

- Accept tasks even when no workers available
- Auto-assign when workers join
- Priority-based scheduling

### 5. Authentication & Security

- Add TLS for gRPC connections
- Implement authentication tokens
- Secure MongoDB credentials

---

## Architecture Overview

```
┌─────────────┐
│   Master    │  (Port 50051)
│   - gRPC    │
│   - MongoDB │
│   - CLI     │
└──────┬──────┘
       │
       │ gRPC: RegisterWorker, SendHeartbeat, ReportTaskCompletion
       │ gRPC: AssignTask (Master → Worker)
       │
    ┌──┴──────────┬──────────────┐
    │             │              │
┌───▼────┐   ┌───▼────┐    ┌───▼────┐
│Worker-1│   │Worker-2│    │Worker-N│  (Port 50052, 50053, ...)
│- Docker│   │- Docker│    │- Docker│
│- gRPC  │   │- gRPC  │    │- gRPC  │
└────────┘   └────────┘    └────────┘
     │            │              │
     └────────────┴──────────────┘
                  │
          Docker Hub / Registry
          (Task Container Images)
```

---

## Summary

You now have a working distributed task execution system with:

- ✅ gRPC communication between master and workers
- ✅ Docker-based task execution
- ✅ Real-time telemetry and heartbeat monitoring
- ✅ Interactive CLI for task management
- ✅ Modular, maintainable code structure

This is a solid foundation for building a production-grade distributed computing platform!
