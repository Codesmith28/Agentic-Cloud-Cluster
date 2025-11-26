# CloudAI - Comprehensive Documentation

**Version:** 2.1  
**Last Updated:** November 26, 2025  
**Authors:** CloudAI Development Team

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [System Architecture](#2-system-architecture)
3. [Core Features](#3-core-features)
4. [Components Overview](#4-components-overview)
5. [Installation & Setup](#5-installation--setup)
6. [Usage Guide](#6-usage-guide)
7. [API Reference](#7-api-reference)
8. [Telemetry & Monitoring](#8-telemetry--monitoring)
9. [Database Schema](#9-database-schema)
10. [Development Guide](#10-development-guide)
11. [Troubleshooting](#11-troubleshooting)
12. [Performance Tuning](#12-performance-tuning)
13. [Security Considerations](#13-security-considerations)

---

## 1. Introduction

### 1.1 What is CloudAI?

CloudAI is a distributed computing platform designed for orchestrating Docker-based task execution across a cluster of worker nodes. Built with **Go** for high performance, it provides a robust, scalable foundation for distributed workload processing.

### 1.2 Key Capabilities

- **Distributed Task Execution**: Run containerized workloads across multiple worker nodes
- **Real-time Monitoring**: WebSocket-based telemetry streaming for cluster health and task status
- **Resource Management**: Track and optimize CPU, memory, storage, and GPU allocation
- **Interactive Management**: Command-line interface for cluster administration
- **Web Dashboard**: React-based UI for monitoring and management
- **Persistent Storage**: MongoDB-backed data persistence for tasks, workers, and results
- **Docker Integration**: Native Docker support for running any containerized application
- **File Storage**: Secure file upload and download for task inputs/outputs

### 1.3 Use Cases

- **Machine Learning Pipelines**: Distribute training tasks across GPU-enabled workers
- **Data Processing**: Batch processing of large datasets
- **CI/CD Workflows**: Parallel test execution and build processes
- **Microservices Testing**: Deploy and test services in isolated containers
- **Research Computing**: Academic and scientific computing workloads

---

## 2. System Architecture

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     User Interfaces                             │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐         │
│  │ Master CLI   │   │  HTTP API    │   │  WebSocket   │         │
│  │  (Terminal)  │   │  (REST)      │   │  (Streaming) │         │
│  └──────┬───────┘   └──────┬───────┘   └──────┬───────┘         │
└─────────┼──────────────────┼──────────────────┼─────────────────┘
          │                  │                  │
          └──────────────────┼──────────────────┘
                             ↓
┌────────────────────────────────────────────────────────────────┐
│                        Master Node (Go)                        │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  gRPC Server (Port 50051)                                │  │
│  │  • Worker Registration    • Task Assignment              │  │
│  │  • Heartbeat Processing   • Result Collection            │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  Telemetry   │  │   Database   │  │      CLI     │          │
│  │   Manager    │  │    Layer     │  │   Interface  │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└───────────────────────────┬────────────────────────────────────┘
                            │
          ┌─────────────────┼─────────────────┐
          │                 │                 │
          ↓                 ↓                 ↓
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│  Worker Node 1  │ │  Worker Node 2  │ │  Worker Node N  │
│  (Go)           │ │  (Go)           │ │  (Go)           │
│                 │ │                 │ │                 │
│ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │
│ │gRPC Server  │ │ │ │gRPC Server  │ │ │ │gRPC Server  │ │
│ │(Port 5005X) │ │ │ │(Port 5005X) │ │ │ │(Port 5005X) │ │
│ └─────────────┘ │ │ └─────────────┘ │ │ └─────────────┘ │
│ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │
│ │   Docker    │ │ │ │   Docker    │ │ │ │   Docker    │ │
│ │  Executor   │ │ │ │  Executor   │ │ │ │  Executor   │ │
│ └─────────────┘ │ │ └─────────────┘ │ │ └─────────────┘ │
│ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │
│ │ Telemetry   │ │ │ │ Telemetry   │ │ │ │ Telemetry   │ │
│ │  Monitor    │ │ │ │  Monitor    │ │ │ │  Monitor    │ │
│ └─────────────┘ │ │ └─────────────┘ │ │ └─────────────┘ │
└────────┬────────┘ └────────┬────────┘ └────────┬────────┘
         │                   │                   │
         └───────────────────┴───────────────────┘
                             ↓
                    ┌────────────────┐
                    │ Docker Engine  │
                    │   (Runtime)    │
                    └────────────────┘

┌───────────────────────────────────────────────────────────────┐
│                    Web Dashboard (React)                      │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  WebSocket Client (connects to Master :8080)             │ │
│  └──────────────────────────────────────────────────────────┘ │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Worker     │  │    Task      │  │   Real-time  │         │
│  │   Status     │  │  Management  │  │   Telemetry  │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└───────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│                    Database (MongoDB)                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │    TASKS     │  │   WORKERS    │  │   RESULTS    │          │
│  │  Collection  │  │  REGISTRY    │  │  Collection  │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└────────────────────────────────────────────────────────────────┘
```

### 2.2 Communication Protocols

#### gRPC Communication (Master ↔ Worker)

**Protocol:** gRPC over HTTP/2  
**Port:** 50051 (Master), 50052+ (Workers)  
**Definition:** `proto/master_worker.proto`

**Master → Worker RPCs:**
- `AssignTask`: Send task to worker for execution
- `CancelTask`: Request task cancellation

**Worker → Master RPCs:**
- `RegisterWorker`: Initial worker registration
- `SendHeartbeat`: Periodic health updates (every 5s)
- `ReportTaskCompletion`: Task execution results

#### HTTP/WebSocket API (Master → Clients)

**Port:** 8080 (configurable via `HTTP_PORT`)

**REST Endpoints - Telemetry:**
- `GET /health` - Health check
- `GET /telemetry` - All workers telemetry (JSON snapshot)
- `GET /telemetry/{workerID}` - Specific worker telemetry (JSON snapshot)
- `GET /workers` - Workers list with basic info

**REST Endpoints - Task Management:**
- `POST /api/tasks` - Submit new task
- `GET /api/tasks` - List all tasks (supports ?status= filter)
- `GET /api/tasks/{id}` - Get task details
- `DELETE /api/tasks/{id}` - Cancel task
- `GET /api/tasks/{id}/logs` - Get task logs

**REST Endpoints - Worker Management:**
- `GET /api/workers` - List all workers with telemetry
- `GET /api/workers/{id}` - Get worker details
- `GET /api/workers/{id}/metrics` - Get worker resource metrics
- `GET /api/workers/{id}/tasks` - Get tasks assigned to worker

**WebSocket Endpoints:**
- `WS /ws/telemetry` - Real-time telemetry stream (all workers)
- `WS /ws/telemetry/{workerID}` - Real-time telemetry stream (specific worker)

### 2.3 Data Flow

#### Task Submission Flow

```
1. User submits task via CLI, REST API, or Web UI
   ↓
2. Master validates task and stores in MongoDB
   ↓
3. Master determines target worker based on resources
   ↓
4. Master assigns task to worker via gRPC
   ↓
5. Worker pulls Docker image if needed
   ↓
6. Worker creates and starts container
   ↓
7. Worker streams logs and monitors execution
   ↓
8. Container completes (success or failure)
   ↓
9. Worker reports result back to master
   ↓
10. Master updates database and notifies user
```

#### Telemetry Flow

```
1. Worker collects system metrics (every 5s)
   ↓
2. Worker sends heartbeat to master via gRPC
   ↓
3. Master's TelemetryManager processes heartbeat
   ↓
4. Data stored in per-worker thread-safe structure
   ↓
5. WebSocket clients receive real-time updates
   ↓
6. CLI/HTTP clients can query current state
```

---

## 3. Core Features

### 3.1 Task Execution

**Docker-Based Execution:**
- Pull images from any Docker registry (Docker Hub, private registries)
- Support for any containerized application
- Automatic container lifecycle management
- Log collection and streaming
- Resource isolation and limits

**Task Cancellation:**
- Graceful shutdown with SIGTERM (10s timeout)
- Forceful termination with SIGKILL if needed
- Automatic container cleanup
- Database status updates

**Task Monitoring:**
- Real-time log streaming
- Exit code capture
- Execution time tracking
- Resource usage monitoring

### 3.2 Worker Management

**Auto-Registration:**
- Workers automatically register on startup
- Resource capacity reporting (CPU, memory, GPU, storage)
- Unique worker identification

**Health Monitoring:**
- Periodic heartbeats (5-second interval)
- Automatic inactive status on timeout (30s)
- Resource utilization tracking
- Running task inventory

**Manual Registration:**
- Admin can pre-register workers in database
- Workers auto-populate specs on first connection
- Persistent worker registry

### 3.3 Real-Time Telemetry

**WebSocket Streaming:**
- Push-based updates (no polling overhead)
- Sub-second latency
- Automatic reconnection
- Filter by worker ID

**Metrics Tracked:**
- CPU usage percentage
- Memory usage (GB)
- GPU utilization
- Storage usage
- Running task count and details
- Last update timestamp
- Worker active/inactive status

**Access Methods:**
- WebSocket for real-time streaming
- REST API for snapshot queries
- CLI commands for interactive monitoring

### 3.4 Database Persistence

**MongoDB Collections:**

1. **TASKS**: All task records with status and metadata
2. **WORKER_REGISTRY**: Registered workers with specifications
3. **RESULTS**: Task execution results and logs
4. **FILE_METADATA**: File storage metadata
5. **USERS**: User accounts and authentication

**Features:**
- Persistent task history
- Worker registry with capacity tracking
- Result storage for analysis
- Context preservation across restarts
- Secure file storage

### 3.5 Interactive CLI

**Command Categories:**

- **Task Management**: submit, cancel, monitor tasks
- **Worker Operations**: list, register, status workers
- **Cluster Monitoring**: overall status, statistics
- **Help System**: built-in documentation

**Features:**
- Command history
- Tab completion
- Color-coded output
- Real-time updates

---

## 4. Components Overview

### 4.1 Master Node (Go)

**Location:** `master/`

**Responsibilities:**
- Central coordination and control
- Worker registration and health monitoring
- Task assignment and tracking
- Database management
- HTTP/WebSocket API serving
- CLI interface

**Key Modules:**

```
master/
├── main.go                 # Entry point
├── internal/
│   ├── server/            # gRPC server implementation
│   │   └── master_server.go
│   ├── cli/               # Interactive CLI
│   │   └── cli.go
│   ├── db/                # MongoDB operations
│   │   └── init.go
│   ├── http/              # HTTP/WebSocket server
│   │   └── telemetry_server.go
│   ├── telemetry/         # Telemetry management
│   │   └── telemetry_manager.go
│   └── system/            # System utilities
│       └── resources.go
```

**Configuration:**
- Port: 50051 (gRPC), 8080 (HTTP)
- Database: MongoDB connection string
- Environment variables via `.env`

### 4.2 Worker Node (Go)

**Location:** `worker/`

**Responsibilities:**
- Task execution via Docker
- Telemetry collection and reporting
- Container lifecycle management
- Log streaming
- Result reporting

**Key Modules:**

```
worker/
├── main.go                 # Entry point
├── internal/
│   ├── server/            # gRPC server
│   │   └── worker_server.go
│   ├── executor/          # Docker task executor
│   │   └── executor.go
│   ├── telemetry/         # Heartbeat sender
│   │   └── telemetry.go
│   └── system/            # System metrics
│       └── resources.go
```

**Configuration:**
- Worker ID
- Worker IP address
- Master address (for gRPC connection)
- Server port (default: 50052)

### 4.3 Web UI

**Location:** `ui/`

**Responsibilities:**
- Real-time cluster monitoring dashboard
- Task submission and management
- Worker status visualization
- Telemetry display

**Technology:**
- React with Vite
- WebSocket for real-time updates
- Runs on port 3000

### 4.4 Protocol Definitions

**Location:** `proto/`

**Files:**
- `master_worker.proto`: Master-Worker communication
- `master_agent.proto`: Master-Agent communication

**Generated Code:**
- `proto/pb/`: Go generated code
- `proto/py/`: Python generated code (for future agent extensibility)

---

## 5. Installation & Setup

### 5.1 Prerequisites

**Required Software:**

```bash
# Go (1.22 or higher)
go version

# Docker
docker --version

# Docker Compose
docker-compose --version

# Protocol Buffers Compiler
protoc --version  # Should be 3.x or higher

# Python (3.8 or higher) - for future agent extensibility
python3 --version

# Node.js (18 or higher) - for Web UI
node --version

# MongoDB (via Docker)
# No separate installation needed if using docker-compose
```

**Install Go gRPC Plugins:**

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

**Set Up Python Environment:**

```bash
# Create virtual environment
python3 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate

# Install Python dependencies (gRPC for future agent support)
pip install -r requirements.txt
```

The `requirements.txt` includes gRPC dependencies for Python agents via `master_agent.proto`.

### 5.2 Quick Setup

**Using Makefile (Recommended):**

```bash
# Set up Python environment first
python3 -m venv venv && source venv/bin/activate && pip install -r requirements.txt

# One-time setup (generate proto code, create symlinks, install deps)
make setup

# Build master and worker
make all

# Or build individually
make master
make worker
```

**Manual Setup:**

```bash
# 1. Generate gRPC code
cd proto
chmod +x generate.sh
./generate.sh
cd ..

# 2. Create symlinks
cd master && ln -s ../proto/pb proto && cd ..
cd worker && ln -s ../proto/pb proto && cd ..

# 3. Install Go dependencies
cd master && go mod tidy && cd ..
cd worker && go mod tidy && cd ..

# 4. Build binaries
cd master && go build -o masterNode . && cd ..
cd worker && go build -o workerNode . && cd ..
```

### 5.3 Database Setup

**Start MongoDB:**

```bash
cd database
docker-compose up -d
```

**Verify MongoDB is running:**

```bash
docker-compose ps
# Should show mongodb service as "Up"
```

**MongoDB Configuration:**

- **Host:** localhost
- **Port:** 27017
- **Database:** cloudai
- **Collections:** TASKS, WORKER_REGISTRY, RESULTS, FILE_METADATA, USERS

### 5.4 Configuration

**Master Node Configuration:**

Create `.env` file in the root directory (optional - system works with defaults):

```bash
# Database (optional - defaults shown)
MONGO_URI=mongodb://localhost:27017
DB_NAME=cluster_db

# Server ports (optional - defaults shown)
GRPC_PORT=:50051
HTTP_PORT=:8080

# Logging (optional)
LOG_LEVEL=info  # debug|info|warn|error

# Note: TLS/SSL not yet implemented
# Future: TLS_ENABLED, TLS_CERT_FILE, TLS_KEY_FILE
```

**Note:** Currently, environment variables are optional. The system uses sensible defaults if no `.env` file is present.

**Worker Node Configuration:**

Environment variables or command-line flags:

```bash
export MASTER_ADDR=localhost:50051
export WORKER_ID=worker-1
export WORKER_IP=192.168.1.100
export WORKER_PORT=:50052
```

---

## 6. Usage Guide

### 6.1 Starting the System

**Terminal 1 - Start MongoDB:**

```bash
cd database
docker-compose up -d
```

**Terminal 2 - Start Master:**

```bash
cd master
./masterNode
# Or using the convenience script:
# ./runMaster.sh
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

**Terminal 3 - Start Worker:**

```bash
cd worker
./workerNode
# Or using the convenience script:
# ./runWorker.sh
```

Expected output:
```
Worker ID:      hostname-123
Worker Address: 192.168.1.100:50052

✓ Worker gRPC server started on :50052
✓ Registered with master at localhost:50051
✓ Telemetry monitor started (5s interval)

Waiting for tasks...
```

### 6.2 CLI Commands Reference

#### Help Command

```bash
master> help
```

Shows all available commands with usage examples.

#### Status Command

```bash
master> status
```

Output:
```
╔═══ Cluster Status ═══
║ Total Workers: 3
║ Active Workers: 3
║ Running Tasks: 5
║ Completed Tasks: 42
╚══════════════════════
```

#### Workers Command

```bash
master> workers
```

Output:
```
╔═══ Registered Workers ═══
║ worker-1
║   Status: 🟢 Active
║   IP: 192.168.1.100:50052
║   Resources: CPU=8.0, Memory=16.0GB, Storage=500.0GB, GPU=1.0
║   Running Tasks: 2
║   Last Heartbeat: 2s ago
║
║ worker-2
║   Status: 🟢 Active
║   IP: 192.168.1.101:50052
║   Resources: CPU=4.0, Memory=8.0GB, Storage=250.0GB, GPU=0.0
║   Running Tasks: 1
║   Last Heartbeat: 1s ago
╚═══════════════════════
```

#### Register Command

```bash
master> register <worker_id> <worker_address>

# Example
master> register worker-3 192.168.1.102:50052
```

Manually register a worker in the database before it connects.

#### Task Command

```bash
master> task <worker_id> <docker_image> [options]

# Options:
#   -cpu_cores <float>   CPU cores (default: 1.0)
#   -mem <float>         Memory in GB (default: 0.5)
#   -storage <float>     Storage in GB (default: 1.0)
#   -gpu_cores <float>   GPU count (default: 0.0)

# Examples:

# Simple task
master> task worker-1 hello-world:latest

# Task with resources
master> task worker-1 python:3.9 -cpu_cores 2.0 -mem 4.0

# GPU task
master> task worker-2 tensorflow/tensorflow:latest-gpu -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
```

Output:
```
✓ Task created successfully!
  Task ID: task-1731677400
  Worker: worker-1
  Image: python:3.9
  Resources: CPU=2.0, Memory=4.0GB

Task submitted to worker...
```

#### Monitor Command

```bash
master> monitor <task_id> [user_id]

# Examples
master> monitor task-1731677400
master> monitor task-1731677400 user-123

# Press any key to exit monitoring
```

Real-time output:
```
╔═══ Task Monitor ═══
║ Task ID: task-1731677400
║ Status: Running
║ Worker: worker-1
║ Image: python:3.9
╚════════════════════

─── Logs ───
Pulling image python:3.9...
Image pulled successfully
Starting container...
Container started: abc123def456

Hello from Python!
Processing data...
Task completed successfully

Exit Code: 0
Status: Completed
```

#### Cancel Command

```bash
master> cancel <task_id>

# Example
master> cancel task-1731677400
```

Output:
```
✓ Task task-1731677400 cancelled successfully
  Container stopped and removed
  Status updated in database
```

#### Exit Command

```bash
master> exit
# or
master> quit
```

Gracefully shuts down the master node.

### 6.3 Monitoring via HTTP API

**Get all workers telemetry:**

```bash
curl http://localhost:8080/telemetry | jq
```

**Get specific worker:**

```bash
curl http://localhost:8080/telemetry/worker-1 | jq
```

**WebSocket streaming (JavaScript):**

```javascript
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');

ws.onmessage = (event) => {
    const telemetry = JSON.parse(event.data);
    console.log('Telemetry update:', telemetry);
};
```

**WebSocket streaming (Python):**

```python
import asyncio
import websockets
import json

async def monitor():
    uri = "ws://localhost:8080/ws/telemetry"
    async with websockets.connect(uri) as ws:
        while True:
            data = await ws.recv()
            telemetry = json.loads(data)
            print(telemetry)

asyncio.run(monitor())
```

---

## 7. API Reference

### 7.1 gRPC API (Master-Worker)

**Service: MasterService**

```protobuf
service MasterService {
    rpc RegisterWorker(WorkerInfo) returns (RegisterAck);
    rpc SendHeartbeat(Heartbeat) returns (HeartbeatAck);
    rpc ReportTaskCompletion(TaskResult) returns (ResultAck);
}
```

**Service: WorkerService**

```protobuf
service WorkerService {
    rpc AssignTask(Task) returns (TaskAck);
    rpc CancelTask(TaskID) returns (TaskAck);
}
```

**Message Types:**

```protobuf
message WorkerInfo {
    string worker_id = 1;
    string worker_ip = 2;
    double total_cpu = 3;
    double total_memory = 4;
    double total_storage = 5;
    double total_gpu = 6;
}

message Task {
    string task_id = 1;
    string user_id = 2;
    string docker_image = 3;
    double cpu_cores = 4;
    double memory_gb = 5;
    double storage_gb = 6;
    double gpu_cores = 7;
    int64 created_at = 8;
}

message Heartbeat {
    string worker_id = 1;
    double cpu_usage = 2;
    double memory_usage = 3;
    double gpu_usage = 4;
    repeated TaskInfo running_tasks = 5;
    int64 timestamp = 6;
}

message TaskResult {
    string task_id = 1;
    string worker_id = 2;
    string status = 3;          // "completed", "failed", "cancelled"
    int32 exit_code = 4;
    string logs = 5;
    int64 completed_at = 6;
}
```

### 7.2 HTTP REST API

**Base URL:** `http://localhost:8080`

#### GET /health

Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "time": 1731677400,
  "active_clients": 2,
  "workers": 3,
  "active_workers": 3
}
```

#### GET /telemetry

Get telemetry for all workers.

**Response:**
```json
{
  "worker-1": {
    "worker_id": "worker-1",
    "cpu_usage": 45.2,
    "memory_usage": 62.1,
    "gpu_usage": 78.3,
    "running_tasks": [
      {
        "task_id": "task-123",
        "cpu_allocated": 2.0,
        "memory_allocated": 4096.0,
        "gpu_allocated": 1.0,
        "status": "running"
      }
    ],
    "last_update": 1731677400,
    "is_active": true
  },
  "worker-2": { ... }
}
```

#### GET /telemetry/{workerID}

Get telemetry for a specific worker.

**Response:** Same as single worker object above.

#### GET /workers

Get basic info for all workers.

**Response:**
```json
{
  "worker-1": {
    "worker_id": "worker-1",
    "is_active": true,
    "running_tasks_count": 2,
    "last_update": 1731677400
  },
  "worker-2": { ... }
}
```

---

#### POST /api/tasks

Submit a new task for execution.

**Request Body:**
```json
{
  "docker_image": "ubuntu:latest",
  "command": "echo 'Hello World'",
  "cpu_required": 1.0,
  "memory_required": 512.0,
  "gpu_required": 0.0,
  "storage_required": 1024.0,
  "user_id": "user123"
}
```

**Response:**
```json
{
  "task_id": "task-1731677400123456789",
  "status": "queued",
  "message": "Task submitted successfully. Queue position: 1. Scheduler will assign it to an available worker."
}
```

**Example:**
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

---

#### GET /api/tasks

List all tasks with optional status filtering.

**Query Parameters:**
- `status` (optional): Filter by task status (pending, queued, running, completed, failed)

**Response:**
```json
[
  {
    "task_id": "task-123",
    "docker_image": "ubuntu:latest",
    "command": "echo hello",
    "status": "running",
    "user_id": "user123",
    "cpu_required": 1.0,
    "memory_required": 512.0,
    "gpu_required": 0.0,
    "storage_required": 1024.0,
    "created_at": 1731677400
  }
]
```

**Examples:**
```bash
# List all tasks
curl http://localhost:8080/api/tasks | jq

# Filter by status
curl http://localhost:8080/api/tasks?status=running | jq
```

---

#### GET /api/tasks/{id}

Get detailed information about a specific task.

**Response:**
```json
{
  "task_id": "task-123",
  "docker_image": "ubuntu:latest",
  "command": "echo hello",
  "status": "completed",
  "user_id": "user123",
  "cpu_required": 1.0,
  "memory_required": 512.0,
  "gpu_required": 0.0,
  "storage_required": 1024.0,
  "created_at": 1731677400,
  "assignment": {
    "worker_id": "worker-1",
    "assigned_at": 1731677410
  },
  "result": {
    "status": "completed",
    "completed_at": 1731677420,
    "logs": "Hello World\n"
  }
}
```

**Example:**
```bash
curl http://localhost:8080/api/tasks/task-123 | jq
```

---

#### DELETE /api/tasks/{id}

Cancel a running or queued task.

**Response:**
```json
{
  "task_id": "task-123",
  "status": "cancelled",
  "message": "Task cancellation requested"
}
```

**Example:**
```bash
curl -X DELETE http://localhost:8080/api/tasks/task-123
```

---

#### GET /api/tasks/{id}/logs

Get stored logs for a completed task.

**Response:**
```json
{
  "task_id": "task-123",
  "logs": "Hello World\nTask completed successfully\n",
  "status": "completed",
  "completed_at": 1731677420
}
```

**Example:**
```bash
curl http://localhost:8080/api/tasks/task-123/logs | jq
```

---

#### GET /api/workers

List all workers with current telemetry.

**Response:**
```json
[
  {
    "worker_id": "worker-1",
    "is_active": true,
    "cpu_usage": 45.2,
    "memory_usage": 62.1,
    "gpu_usage": 15.3,
    "running_tasks_count": 2,
    "last_update": 1731677400
  }
]
```

**Example:**
```bash
curl http://localhost:8080/api/workers | jq
```

---

#### GET /api/workers/{id}

Get detailed information about a specific worker.

**Response:**
```json
{
  "worker_id": "worker-1",
  "is_active": true,
  "cpu_usage": 45.2,
  "memory_usage": 62.1,
  "gpu_usage": 15.3,
  "running_tasks": [
    {
      "task_id": "task-123",
      "cpu_allocated": 1.0,
      "memory_allocated": 512.0,
      "gpu_allocated": 0.0,
      "status": "running"
    }
  ],
  "last_update": 1731677400,
  "worker_info": {
    "worker_id": "worker-1",
    "worker_ip": "192.168.1.100",
    "total_cpu": 8.0,
    "total_memory": 16384.0,
    "total_storage": 512000.0,
    "total_gpu": 1.0,
    "registered_at": 1731600000,
    "last_heartbeat": 1731677400
  }
}
```

**Example:**
```bash
curl http://localhost:8080/api/workers/worker-1 | jq
```

---

#### GET /api/workers/{id}/metrics

Get current resource metrics for a specific worker.

**Response:**
```json
{
  "worker_id": "worker-1",
  "cpu_usage": 45.2,
  "memory_usage": 62.1,
  "gpu_usage": 15.3,
  "is_active": true,
  "last_update": 1731677400,
  "timestamp": 1731677400
}
```

**Example:**
```bash
curl http://localhost:8080/api/workers/worker-1/metrics | jq
```

---

#### GET /api/workers/{id}/tasks

Get all tasks assigned to a specific worker.

**Response:**
```json
{
  "worker_id": "worker-1",
  "tasks": [
    {
      "task_id": "task-123",
      "assigned_at": 1731677410
    },
    {
      "task_id": "task-456",
      "assigned_at": 1731677420
    }
  ],
  "count": 2
}
```

**Example:**
```bash
curl http://localhost:8080/api/workers/worker-1/tasks | jq
```

---

### 7.3 WebSocket API

**Base URL:** `ws://localhost:8080`

#### WS /ws/telemetry

Stream telemetry for all workers.

**Message Format:**
```json
{
  "worker-1": { /* full telemetry data */ },
  "worker-2": { /* full telemetry data */ }
}
```

Messages are sent whenever any worker sends a heartbeat (~5s interval).

#### WS /ws/telemetry/{workerID}

Stream telemetry for a specific worker.

**Message Format:**
```json
{
  "worker_id": "worker-1",
  "cpu_usage": 45.2,
  "memory_usage": 62.1,
  "gpu_usage": 78.3,
  "running_tasks": [...],
  "last_update": 1731677400,
  "is_active": true
}
```

Messages are sent only when the specified worker sends a heartbeat.

---

## 8. Telemetry & Monitoring

### 9.1 Telemetry Architecture

**Worker Side:**
- Dedicated goroutine collects metrics every 5 seconds
- Sends heartbeat to master via gRPC
- Includes: CPU, memory, GPU usage, running tasks

**Master Side:**
- TelemetryManager with per-worker threads
- Non-blocking heartbeat processing
- Thread-safe data access
- WebSocket push notifications
- HTTP API for queries

### 9.2 Metrics Collected

**System Metrics:**
- CPU usage (%)
- Memory usage (GB and %)
- GPU utilization (%)
- Storage usage (GB)

**Task Metrics:**
- Running task IDs
- Resource allocation per task
- Task status
- Task start time

**Worker Metrics:**
- Last heartbeat timestamp
- Active/inactive status
- Total capacity
- Available capacity

### 9.3 Monitoring Methods

#### Real-Time WebSocket

**Advantages:**
- Sub-second latency
- Push-based (no polling overhead)
- Automatic updates

**Use Cases:**
- Live dashboards
- Alerting systems
- Real-time analytics

**Example:**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');
ws.onmessage = (event) => {
    updateDashboard(JSON.parse(event.data));
};
```

#### HTTP API Polling

**Advantages:**
- Simple to implement
- Works with any HTTP client
- Snapshot queries

**Use Cases:**
- Periodic monitoring
- Integration with existing tools
- Debugging

**Example:**
```bash
watch -n 1 'curl -s http://localhost:8080/telemetry | jq'
```

#### CLI Commands

**Advantages:**
- Interactive
- Human-readable output
- Built-in formatting

**Use Cases:**
- Manual monitoring
- Troubleshooting
- Quick status checks

**Commands:**
```bash
master> status
master> workers
master> monitor task-123
```

### 9.4 Alerting (Future Feature)

Planned alerting capabilities:
- Worker down alerts
- High resource usage alerts
- Task failure notifications
- Custom threshold alerts

---

## 9. Database Schema

### 9.1 TASKS Collection

Stores all task submissions and their status.

**Schema:**

```javascript
{
  _id: ObjectId("..."),
  task_id: "task-1731677400",       // Unique task identifier
  user_id: "user-123",              // User who submitted
  docker_image: "python:3.9",       // Docker image name
  command: "",                      // Command to run (optional)
  cpu_cores: 2.0,                   // CPU allocation
  memory_gb: 4.0,                   // Memory allocation (GB)
  storage_gb: 10.0,                 // Storage allocation (GB)
  gpu_cores: 0.0,                   // GPU allocation
  status: "running",                // pending|running|completed|failed|cancelled
  assigned_worker: "worker-1",      // Worker executing task
  created_at: ISODate("..."),       // Submission time
  started_at: ISODate("..."),       // Execution start time
  completed_at: ISODate("..."),     // Execution end time
}
```

**Indexes:**
- `task_id`: Unique index
- `user_id`: Index for user queries
- `status`: Index for status filtering
- `assigned_worker`: Index for worker queries

### 9.2 WORKER_REGISTRY Collection

Stores registered workers and their specifications.

**Schema:**

```javascript
{
  _id: ObjectId("..."),
  worker_id: "worker-1",            // Unique worker identifier
  worker_ip: "192.168.1.100:50052", // Worker address (ip:port)
  total_cpu: 8.0,                   // Total CPU cores
  total_memory: 16.0,               // Total memory (GB)
  total_storage: 500.0,             // Total storage (GB)
  total_gpu: 1.0,                   // Total GPU count
  is_active: true,                  // Currently connected?
  last_heartbeat: 1731677400,       // Unix timestamp
  registered_at: ISODate("..."),    // Registration time
  updated_at: ISODate("..."),       // Last update time
}
```

**Indexes:**
- `worker_id`: Unique index
- `is_active`: Index for active worker queries

### 9.3 RESULTS Collection

Stores task execution results and logs.

**Schema:**

```javascript
{
  _id: ObjectId("..."),
  task_id: "task-1731677400",       // Reference to TASKS
  worker_id: "worker-1",            // Worker that executed
  status: "completed",              // completed|failed|cancelled
  exit_code: 0,                     // Container exit code
  logs: "...",                      // Execution logs (truncated if large)
  execution_time: 45.2,             // Execution time (seconds)
  completed_at: ISODate("..."),     // Completion timestamp
}
```

**Indexes:**
- `task_id`: Unique index
- `worker_id`: Index for worker queries
- `status`: Index for status filtering

---

## 10. Development Guide

### 10.1 Project Structure

```
CloudAI/
├── proto/                  # gRPC protocol definitions
│   ├── master_worker.proto
│   ├── master_agent.proto
│   ├── generate.sh
│   ├── pb/                # Generated Go code
│   └── py/                # Generated Python code
│
├── master/                # Master node (Go)
│   ├── main.go
│   ├── internal/
│   │   ├── server/        # gRPC server
│   │   ├── cli/           # Interactive CLI
│   │   ├── db/            # MongoDB layer
│   │   ├── http/          # HTTP/WebSocket server
│   │   ├── telemetry/     # Telemetry manager
│   │   └── system/        # System utilities
│   └── proto -> ../proto/pb
│
├── worker/                # Worker node (Go)
│   ├── main.go
│   ├── internal/
│   │   ├── server/        # gRPC server
│   │   ├── executor/      # Docker executor
│   │   ├── telemetry/     # Heartbeat sender
│   │   └── system/        # System metrics
│   └── proto -> ../proto/pb
│
├── ui/                    # Web Dashboard (React)
│   ├── src/
│   ├── package.json
│   └── vite.config.js
│
├── database/              # MongoDB setup
│   ├── docker-compose.yml
│   └── README.md
│
├── Makefile               # Build automation
├── README.md              # Project README
└── DOCUMENTATION.md       # This file
```

### 10.2 Adding New Features

#### Adding a New CLI Command

**File:** `master/internal/cli/cli.go`

```go
// 1. Add command to help text
const helpText = `
...
  mycommand <args>     - Description of my command
...
`

// 2. Add case in switch statement
switch parts[0] {
    // ... existing cases ...
    
    case "mycommand":
        if len(parts) < 2 {
            fmt.Println("Usage: mycommand <args>")
            continue
        }
        handleMyCommand(parts[1:])
}

// 3. Implement handler
func handleMyCommand(args []string) {
    // Your implementation
}
```

#### Adding a New gRPC Endpoint

**1. Define in proto file** (`proto/master_worker.proto`):

```protobuf
service MasterService {
    // ... existing RPCs ...
    rpc MyNewRPC(MyRequest) returns (MyResponse);
}

message MyRequest {
    string param1 = 1;
    int32 param2 = 2;
}

message MyResponse {
    bool success = 1;
    string message = 2;
}
```

**2. Regenerate code:**

```bash
cd proto
./generate.sh
```

**3. Implement in server** (`master/internal/server/master_server.go`):

```go
func (s *MasterServer) MyNewRPC(ctx context.Context, req *pb.MyRequest) (*pb.MyResponse, error) {
    // Your implementation
    return &pb.MyResponse{
        Success: true,
        Message: "RPC executed successfully",
    }, nil
}
```

### 10.3 Testing

**Unit Tests:**

```bash
# Go tests
cd master && go test ./... -v
cd worker && go test ./... -v
```

**Integration Tests:**

```bash
# Start all components
./runMaster.sh &
./runWorker.sh &

# Run test tasks
# In master CLI:
master> task worker-1 hello-world:latest
```

### 10.4 Debugging

**Enable verbose logging:**

```bash
# Go components
export LOG_LEVEL=debug
./masterNode
```

**Debug gRPC communication:**

```bash
export GRPC_GO_LOG_VERBOSITY_LEVEL=99
export GRPC_GO_LOG_SEVERITY_LEVEL=info
```

**Monitor database operations:**

```bash
# Connect to MongoDB
docker exec -it mongodb mongo

# Use cluster_db
use cluster_db

# Watch collection changes
db.TASKS.find().sort({created_at: -1}).limit(5)
```

---

## 11. Troubleshooting

### 11.1 Common Issues

#### Worker Not Connecting

**Symptoms:**
- Worker starts but doesn't appear in `master> workers`
- Registration timeout errors

**Solutions:**

1. Check network connectivity:
   ```bash
   # From worker machine
   telnet master-ip 50051
   ```

2. Verify master address:
   ```bash
   # Worker should use correct master IP
   export MASTER_ADDR=master-ip:50051
   ```

3. Check firewall rules:
   ```bash
   # Master machine should allow port 50051
   sudo ufw allow 50051/tcp
   ```

4. Verify master is running:
   ```bash
   # On master machine
   netstat -tuln | grep 50051
   ```

#### Task Execution Fails

**Symptoms:**
- Task status shows "failed"
- Container exits immediately

**Solutions:**

1. Check Docker image exists:
   ```bash
   docker pull <image-name>
   ```

2. Check container logs:
   ```bash
   master> monitor task-id
   ```

3. Verify resource availability:
   ```bash
   master> workers
   # Check if worker has enough CPU/memory/GPU
   ```

4. Test image locally:
   ```bash
   docker run --rm <image-name>
   ```

#### MongoDB Connection Issues

**Symptoms:**
- Master fails to start
- "MongoDB connection error" messages

**Solutions:**

1. Verify MongoDB is running:
   ```bash
   cd database
   docker-compose ps
   ```

2. Check connection string:
   ```bash
   # In .env file
   MONGO_URI=mongodb://localhost:27017
   ```

3. Test connection:
   ```bash
   docker exec -it mongodb mongo --eval "db.adminCommand('ping')"
   ```

4. Restart MongoDB:
   ```bash
   cd database
   docker-compose restart
   ```

#### High Memory Usage

**Symptoms:**
- System becomes slow
- Out of memory errors

**Solutions:**

1. Limit container resources:
   ```bash
   master> task worker-1 image:latest -mem 2.0
   ```

2. Clean up old containers:
   ```bash
   docker system prune -a
   ```

3. Monitor worker resources:
   ```bash
   curl http://localhost:8080/telemetry/worker-1 | jq .memory_usage
   ```

#### WebSocket Connection Drops

**Symptoms:**
- Telemetry streaming stops
- "Connection closed" errors

**Solutions:**

1. Implement reconnection logic:
   ```javascript
   function connect() {
       const ws = new WebSocket('ws://localhost:8080/ws/telemetry');
       ws.onclose = () => setTimeout(connect, 5000);
   }
   ```

2. Check network stability:
   ```bash
   ping master-ip
   ```

3. Increase timeout values:
   ```javascript
   ws.onopen = () => {
       setInterval(() => ws.send('ping'), 30000);
   };
   ```

### 11.2 Logging

**Master logs location:**
- Console output (stdout)
- Future: `/var/log/cloudai/master.log`

**Worker logs location:**
- Console output (stdout)
- Future: `/var/log/cloudai/worker.log`

**Enable debug logging:**

```bash
export LOG_LEVEL=debug
```

---

## 12. Performance Tuning

### 12.1 Master Node Optimization

**Increase database connection pool:**

```go
// master/internal/db/init.go
clientOptions := options.Client().
    ApplyURI(uri).
    SetMaxPoolSize(100)  // Default: 100
```

**Adjust telemetry buffer size:**

```go
// master/internal/telemetry/telemetry_manager.go
const heartbeatBufferSize = 1000  // Default: 100
```

**Tune HTTP server:**

```go
// master/internal/http/telemetry_server.go
server := &http.Server{
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
    IdleTimeout:  60 * time.Second,
}
```

### 12.2 Worker Node Optimization

**Adjust heartbeat interval:**

```go
// worker/internal/telemetry/telemetry.go
const heartbeatInterval = 5 * time.Second  // Default: 5s
// Increase for lower overhead, decrease for fresher data
```

**Docker pull optimization:**

```bash
# Pre-pull common images on workers
docker pull python:3.9
docker pull tensorflow/tensorflow:latest-gpu
docker pull node:18
```

**Resource monitoring frequency:**

```go
// worker/internal/system/resources.go
// Reduce sampling frequency if CPU usage is high
```

### 12.3 Scheduler Optimization

**Batch scheduling:**

```python
# Schedule multiple tasks at once instead of one-by-one
batch_size = 10
for batch in chunk_tasks(pending_tasks, batch_size):
    assignments = scheduler(batch, workers)
    submit_assignments(assignments)
```

**Caching:**

```python
# Cache worker capacity calculations
@lru_cache(maxsize=128)
def calculate_worker_score(worker_id, task_requirements):
    # Expensive calculation
    pass
```

### 12.4 Database Optimization

**Create indexes:**

```javascript
// In MongoDB shell
db.TASKS.createIndex({status: 1})
db.TASKS.createIndex({user_id: 1, created_at: -1})
db.WORKER_REGISTRY.createIndex({is_active: 1})
db.RESULTS.createIndex({task_id: 1}, {unique: true})
```

**Limit log size:**

```go
// Truncate logs before storing
const maxLogSize = 10000  // characters
if len(logs) > maxLogSize {
    logs = logs[:maxLogSize] + "\n... (truncated)"
}
```

---

## 13. Security Considerations

### 13.1 Network Security

**Current State:**
- gRPC communication currently uses **insecure connections** (no TLS)
- Suitable for development and trusted internal networks
- Should not be exposed to public internet without additional security layers

**Planned: TLS/SSL for gRPC (Future Feature)**

TLS support is planned for production deployments. The implementation will include:

```go
// Master (planned implementation)
creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
server := grpc.NewServer(grpc.Creds(creds))

// Worker (planned implementation)
creds, err := credentials.NewClientTLSFromFile(certFile, "")
conn, err := grpc.Dial(address, grpc.WithTransportCredentials(creds))
```

**Planned configuration:**

```bash
# .env file (future)
TLS_ENABLED=true
TLS_CERT_FILE=/path/to/cert.pem
TLS_KEY_FILE=/path/to/key.pem
```

**Current Security Recommendations:**
- Deploy within a private network or VPN
- Use firewall rules to restrict access to ports 50051, 50052+, 8080
- Consider SSH tunneling for remote connections:
  ```bash
  # SSH tunnel from remote worker to master
  ssh -L 50051:localhost:50051 user@master-host
  ```

### 13.2 Authentication & Authorization

**Current State:**
- No authentication or authorization implemented
- All workers and clients have full access
- User IDs in tasks are for tracking only, not enforced

**Future enhancements (Planned):**
- JWT-based authentication for API access
- Role-based access control (RBAC)
- API key management for programmatic access
- User quotas and resource limits
- Per-user task isolation

**Planned architecture:**

```
User → JWT Token → Master validates → Authorize action
```

**Current Security Recommendations:**
- Use network-level security (firewalls, VPNs)
- Restrict access to master CLI to trusted users
- Monitor task submissions via logs
- Run in isolated network segments

### 13.3 Container Security

**Best practices:**

1. **Use official images:**
   ```bash
   docker pull python:3.9-slim  # Official, minimal
   ```

2. **Scan images for vulnerabilities:**
   ```bash
   docker scan python:3.9
   ```

3. **Run containers as non-root:**
   ```dockerfile
   USER nonroot
   ```

4. **Limit container capabilities:**
   ```bash
   docker run --cap-drop=ALL --cap-add=NET_BIND_SERVICE ...
   ```

5. **Use read-only root filesystem:**
   ```bash
   docker run --read-only ...
   ```

### 13.4 Database Security

**MongoDB authentication:**

```yaml
# database/docker-compose.yml
services:
  mongodb:
    environment:
      MONGO_INITDB_ROOT_USERNAME: admin
      MONGO_INITDB_ROOT_PASSWORD: secure_password
```

**Connection string with auth:**

```bash
MONGO_URI=mongodb://admin:secure_password@localhost:27017/cluster_db?authSource=admin
```

**Network isolation:**

```yaml
# docker-compose.yml
services:
  mongodb:
    networks:
      - internal
networks:
  internal:
    internal: true
```

### 13.5 Input Validation

**Always validate user inputs:**

```go
func validateTask(task *Task) error {
    if task.DockerImage == "" {
        return errors.New("docker image is required")
    }
    if task.CPUCores < 0 || task.CPUCores > 64 {
        return errors.New("invalid CPU cores")
    }
    if task.MemoryGB < 0 || task.MemoryGB > 512 {
        return errors.New("invalid memory")
    }
    // ... more validation
    return nil
}
```

---

## Appendix A: Configuration Reference

### Master Node Environment Variables

| Variable | Default | Description | Status |
|----------|---------|-------------|--------|
| `MONGO_URI` | `mongodb://localhost:27017` | MongoDB connection string | ✅ Implemented |
| `DB_NAME` | `cluster_db` | Database name | ✅ Implemented |
| `GRPC_PORT` | `:50051` | gRPC server port | ✅ Implemented |
| `HTTP_PORT` | `:8080` | HTTP/WebSocket server port | ✅ Implemented |
| `LOG_LEVEL` | `info` | Logging level (debug/info/warn/error) | ✅ Implemented |
| `TLS_ENABLED` | - | Enable TLS for gRPC | 🔜 Planned |
| `TLS_CERT_FILE` | - | TLS certificate file path | 🔜 Planned |
| `TLS_KEY_FILE` | - | TLS private key file path | 🔜 Planned |

### Worker Node Environment Variables

| Variable | Default | Description | Status |
|----------|---------|-------------|--------|
| `WORKER_ID` | hostname | Worker unique identifier | ✅ Implemented |
| `WORKER_IP` | auto-detected | Worker IP address | ✅ Implemented |
| `WORKER_PORT` | `:50052` | Worker gRPC server port | ✅ Implemented |
| `MASTER_ADDR` | `localhost:50051` | Master server address | ✅ Implemented |
| `HEARTBEAT_INTERVAL` | `5s` | Heartbeat send interval | ✅ Implemented |
| `LOG_LEVEL` | `info` | Logging level | ✅ Implemented |
| `CLOUDAI_OUTPUT_DIR` | `/var/cloudai/outputs` | Task output directory | ✅ Implemented |

---

## Appendix B: Protocol Buffer Definitions

### master_worker.proto (Summary)

```protobuf
// Master-side services
service MasterService {
    rpc RegisterWorker(WorkerInfo) returns (RegisterAck);
    rpc SendHeartbeat(Heartbeat) returns (HeartbeatAck);
    rpc ReportTaskCompletion(TaskResult) returns (ResultAck);
}

// Worker-side services
service WorkerService {
    rpc AssignTask(Task) returns (TaskAck);
    rpc CancelTask(TaskID) returns (TaskAck);
}

// Key message types
message Task { ... }
message WorkerInfo { ... }
message Heartbeat { ... }
message TaskResult { ... }
```

See `proto/master_worker.proto` for full definitions.

---

## Appendix C: Performance Benchmarks

### Scheduler Performance Comparison

Based on test workload: 100 tasks, 5 workers, mixed resource requirements

| Scheduler | Makespan | Waiting Time | Utilization | Load Variance |
|-----------|----------|--------------|-------------|---------------|
| Greedy (baseline) | 180s | 45s | 62% | 0.25 |
| Round Robin | 195s | 52s | 58% | 0.18 |
| Balanced | 175s | 41s | 65% | 0.15 |
| AI Multi-Objective | 126s (-30%) | 29s (-36%) | 78% (+26%) | 0.12 |
| AI Aggressive | 108s (-40%) | 23s (-49%) | 91% (+47%) | 0.08 |
| AI Predictive | 115s (-36%) | 26s (-42%) | 82% (+32%) | 0.14 |
| AI Load-Balanced | 132s (-27%) | 31s (-31%) | 75% (+21%) | 0.06 |

### System Performance Metrics

- **Master throughput:** 1000+ tasks/minute
- **Worker execution latency:** <100ms (container start overhead excluded)
- **Heartbeat overhead:** <1% CPU per worker
- **WebSocket latency:** <50ms (LAN), <200ms (WAN)
- **Database write latency:** <10ms (local MongoDB)

---

## Appendix D: FAQ

**Q: Can I use private Docker registries?**  
A: Yes, workers authenticate using the host's Docker credentials. Run `docker login` on worker machines.

**Q: How do I scale to more workers?**  
A: Simply start more worker nodes with unique IDs. The master automatically handles them.

**Q: Can tasks communicate with each other?**  
A: Not directly. For inter-task communication, use external services (Redis, RabbitMQ, etc.).

**Q: What's the maximum cluster size?**  
A: Tested up to 100 workers. Theoretical limit is much higher (10,000+).

**Q: Can I run multiple tasks per worker?**  
A: Yes, if resources allow. Workers schedule multiple containers concurrently.

**Q: How do I upgrade without downtime?**  
A: Rolling upgrade: Upgrade workers one at a time, then master. Use load balancer for multiple masters (future feature).

**Q: Can I use Kubernetes instead of Docker?**  
A: Not currently. K8s support is planned for future releases.

**Q: How do I backup the database?**  
A: Use MongoDB backup tools: `mongodump -d cluster_db -o backup/`

---

## Conclusion

CloudAI provides a comprehensive, production-ready platform for distributed task execution. With AI-powered scheduling, real-time monitoring, and robust architecture, it's suitable for a wide range of use cases from research computing to production workloads.

For more information:
- GitHub: [Codesmith28/CloudAI](https://github.com/Codesmith28/CloudAI)
- Issues: Use GitHub Issues for bug reports and feature requests
- Contributing: See CONTRIBUTING.md (coming soon)

**Happy distributed computing! 🚀**
