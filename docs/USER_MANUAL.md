# CloudAI — Comprehensive User Manual

> **Agentic Cloud Cluster**: A distributed task scheduling platform with intelligent, ML-driven schedulers, real-time monitoring, and a modern web UI.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [Prerequisites](#3-prerequisites)
4. [Installation & Setup](#4-installation--setup)
5. [Database Setup (MongoDB)](#5-database-setup-mongodb)
6. [Protobuf Code Generation](#6-protobuf-code-generation)
7. [Environment Variables Reference](#7-environment-variables-reference)
8. [Starting the Master Node](#8-starting-the-master-node)
9. [Starting Workers](#9-starting-workers)
10. [Starting the PPO Agentic Scheduler](#10-starting-the-ppo-agentic-scheduler)
11. [Master CLI Reference](#11-master-cli-reference)
12. [TUI (Terminal UI) Reference](#12-tui-terminal-ui-reference)
13. [HTTP REST API Reference](#13-http-rest-api-reference)
14. [Web UI (React Dashboard)](#14-web-ui-react-dashboard)
15. [Scheduler Algorithms](#15-scheduler-algorithms)
16. [Testbench & Integration Testing](#16-testbench--integration-testing)
17. [Observability (Prometheus & Grafana)](#17-observability-prometheus--grafana)
18. [Makefile Reference](#18-makefile-reference)
19. [CI/CD Pipeline](#19-cicd-pipeline)
20. [Troubleshooting](#20-troubleshooting)

---

## 1. Overview

CloudAI is a distributed task scheduling system that orchestrates Docker-based workloads across a cluster of worker nodes. It features:

- **Master Node** — Central control plane with CLI, TUI, HTTP API, and gRPC server
- **Worker Nodes** — Execute Docker containers with resource isolation and telemetry
- **PPO Agentic Scheduler** — Reinforcement learning-based scheduler using Proximal Policy Optimization
- **Web UI** — React-based dashboard for real-time monitoring and task management
- **Testbench** — Docker Compose-based integration testing with observability

### Core Workflow

```
User → (CLI / TUI / Web UI / REST API) → Master Node → Scheduler → Worker Node → Docker Container
                                              ↑                          │
                                              └── Heartbeats / Results ──┘
```

1. A user submits a task (Docker image + resource requirements)
2. The master's scheduler selects the best worker based on the active algorithm (RR, RTS, or PPO)
3. The worker pulls the Docker image, creates a container with resource limits, and executes it
4. Results, logs, and output files are streamed back to the master
5. Real-time telemetry is available via WebSocket, Prometheus, and the Web UI

---

## 2. Architecture

### Components

| Component | Language | Port(s) | Purpose |
|-----------|----------|---------|---------|
| **Master** | Go | gRPC `:50051`, HTTP `:8080` | Control plane, scheduling, API |
| **Worker** | Go | gRPC `:50052`, Metrics `:9101` | Task execution, telemetry |
| **PPO Scheduler** | Python | gRPC `:50061` | ML-based worker selection |
| **Web UI** | React/Vite | `:3001` (dev) | Dashboard, task/worker management |
| **MongoDB** | — | `:27017` | Persistence for tasks, workers, models |
| **Prometheus** | — | `:9090` | Metrics collection |
| **Grafana** | — | `:3000` | Metrics visualization |

### Directory Structure

```
BTEP/
├── master/              # Go master node
│   ├── main.go          # Entry point
│   ├── internal/
│   │   ├── cli/         # Interactive CLI
│   │   ├── tui/         # Bubble Tea TUI
│   │   ├── http/        # REST API & WebSocket server
│   │   ├── server/      # gRPC server
│   │   ├── scheduler/   # RR, RTS, PPO schedulers
│   │   ├── db/          # MongoDB data access
│   │   ├── controlplane/# Task execution orchestration
│   │   ├── storage/     # File storage & access control
│   │   ├── telemetry/   # Worker telemetry management
│   │   └── metrics/     # Prometheus metrics
│   └── config/          # GA parameters
├── worker/              # Go worker node
│   ├── main.go          # Entry point
│   └── internal/
│       ├── executor/    # Docker task execution
│       ├── server/      # gRPC server
│       ├── telemetry/   # Heartbeat & resource monitor
│       ├── logstream/   # Live log broadcasting
│       ├── metrics/     # Prometheus metrics
│       └── system/      # System resource detection
├── agentic_scheduler/   # Python PPO scheduler
│   ├── server.py        # gRPC server
│   ├── service.py       # Core scheduling logic
│   ├── model.py         # Actor-Critic neural network
│   ├── features.py      # Feature extraction
│   ├── persistence.py   # MongoDB model storage
│   ├── train_ppo.py     # Offline training script
│   └── training/        # Environments & trace loaders
├── ui/                  # React web dashboard
│   └── src/
│       ├── pages/       # Dashboard, Tasks, Workers, Submit
│       ├── components/  # Reusable UI components
│       ├── api/         # API clients & WebSocket
│       ├── hooks/       # Custom React hooks
│       └── context/     # Auth context
├── proto/               # Protobuf definitions
├── database/            # MongoDB Docker Compose
├── testbench/           # Integration testing framework
│   ├── docker/          # Dockerfiles
│   ├── scenarios/       # Test scenarios
│   ├── scripts/         # Test runners
│   ├── workloads/       # Workload definitions
│   └── observability/   # Prometheus + Grafana
├── Makefile             # Build & test targets
├── runMaster.sh         # Master startup script
└── runWorker.sh         # Worker startup script
```

---

## 3. Prerequisites

| Tool | Minimum Version | Purpose |
|------|----------------|---------|
| **Go** | 1.24+ | Master & Worker compilation |
| **Python** | 3.12+ | PPO Agentic Scheduler |
| **Node.js** | 18+ | Web UI |
| **Docker** | 24+ | Task execution, testbench |
| **Docker Compose** | v2+ | Multi-container environments |
| **MongoDB** | 7.0+ | Data persistence |
| **protoc** | 3.21+ | Protobuf code generation (optional) |
| **Make** | Any | Build automation |

---

## 4. Installation & Setup

### Quick Start (from scratch)

```bash
# 1. Clone the repository
git clone https://github.com/Codesmith28/Agentic-Cloud-Cluster.git
cd Agentic-Cloud-Cluster

# 2. Install all dependencies
make install-deps
# Or manually:
bash scripts/install_deps.sh

# 3. Start MongoDB
cd database && docker compose up -d && cd ..

# 4. Configure environment
cp master/.env.example master/.env
# Edit master/.env with your settings (see Section 7)

# 5. Build master and worker
make build-master
make build-worker

# 6. Start the master
./runMaster.sh
# Or: cd master && go run main.go --mode cli

# 7. Start a worker (in another terminal)
./runWorker.sh
# Or: cd worker && go run main.go

# 8. (Optional) Start the Web UI
cd ui && npm install && npm run dev

# 9. (Optional) Start the PPO Scheduler
cd agentic_scheduler
pip install -r ../requirements.txt
python -m agentic_scheduler --port 50061
```

### Using the Makefile

```bash
make install-deps      # Install Go, Python, Node.js dependencies
make build             # Build master + worker binaries
make run-master        # Start the master node
make run-worker        # Start a worker node
make run-ui            # Start the web UI
make run-all           # Start everything
```

---

## 5. Database Setup (MongoDB)

### Using Docker Compose (Recommended)

```bash
cd database
docker compose up -d
```

This starts MongoDB on `localhost:27017` with authentication.

**`database/docker-compose.yml`** provisions:
- MongoDB 7.0 on port `127.0.0.1:27017`
- Health checks every 10s
- Persistent volume `mongodb_data`
- Isolated Docker network

### Required Collections

The master auto-creates these collections on startup:

| Collection | Purpose |
|-----------|---------|
| `USERS` | User accounts (email, password hash) |
| `WORKER_REGISTRY` | Worker metadata (ID, IP, resources) |
| `TASKS` | Task definitions and status |
| `ASSIGNMENTS` | Task-to-worker assignments |
| `ATTEMPTS` | Task execution attempts (retries) |
| `RESULTS` | Execution results (exit code, duration, logs) |
| `SCHEDULER_MODELS` | PPO model checkpoints |
| `RTS_WEIGHTS` | Learned RTS scheduler parameters |

### Connection String

```
MONGODB_URI=mongodb://admin:password@localhost:27017
MONGODB_DATABASE=cloudai
```

---

## 6. Protobuf Code Generation

The project defines three `.proto` files:

| File | Purpose |
|------|---------|
| `proto/master_worker.proto` | Master ↔ Worker communication |
| `proto/master_agent.proto` | Master ↔ PPO Agent communication |
| `proto/ppo_scheduler.proto` | PPO Scheduler service definition |

### Regenerate (only needed if protos change)

```bash
cd proto
bash generate.sh
```

This generates:
- Go code in `proto/pb/` (for master and worker)
- Python code in `proto/py/` (for agentic_scheduler)

**Requirements:** `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`, `grpcio-tools` (Python)

---

## 7. Environment Variables Reference

### Master Node (`master/.env`)

#### MongoDB
| Variable | Default | Description |
|----------|---------|-------------|
| `MONGODB_URI` | `mongodb://localhost:27017` | Full MongoDB connection URI |
| `MONGODB_HOST` | `localhost:27017` | MongoDB host (fallback if URI not set) |
| `MONGODB_USERNAME` | _(empty)_ | MongoDB username |
| `MONGODB_PASSWORD` | _(empty)_ | MongoDB password |
| `MONGODB_DATABASE` | `cluster_db` | Database name |

#### Server Ports
| Variable | Default | Description |
|----------|---------|-------------|
| `GRPC_PORT` | `:50051` | Master gRPC listen port |
| `HTTP_PORT` | `:8080` | HTTP API + WebSocket + Prometheus port |
| `MASTER_BIND_ADDR` | _(auto)_ | Override gRPC bind address |
| `MASTER_ADVERTISE_ADDR` | _(auto)_ | Override gRPC advertise address |

#### Authentication
| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | _(random per run)_ | Base64-encoded JWT signing key. Generate: `openssl rand -base64 32` |

#### Scheduler
| Variable | Default | Description |
|----------|---------|-------------|
| `SCHED_ALGO` | `RTS` | Scheduler algorithm: `RR`, `RTS`, or `PPO` |
| `SCHED_SLA_MULTIPLIER` | `2.0` | SLA deadline multiplier k (range: 1.5–2.5) |
| `SCHED_GA_PARAMS_PATH` | `config/ga_output.json` | Path to GA-optimized parameters for RTS |

#### PPO Scheduler
| Variable | Default | Description |
|----------|---------|-------------|
| `PPO_GRPC_ADDR` | `127.0.0.1:50061` | PPO service gRPC address |
| `PPO_REQUEST_TIMEOUT_MS` | `1500` | Request timeout (milliseconds) |
| `PPO_AUTOSTART` | `true` | Auto-start the Python PPO service |
| `PPO_MODEL_PATH` | `latest` | Model file path or `latest` for most recent |
| `PPO_DEPLOYMENT_MODE` | `active` | Mode: `shadow`, `active`, or `fallback` |
| `PPO_ONLINE_UPDATES_ENABLED` | `true` | Enable online learning from outcomes |

#### UI & Runtime
| Variable | Default | Description |
|----------|---------|-------------|
| `CLOUDAI_UI_MODE` | `cli` | Startup mode: `cli` or `tui` |
| `CLOUDAI_HEADLESS` | `false` | `true` disables CLI (for containerized mode) |
| `CLOUDAI_FILES_DIR` | `/var/cloudai/files` | File storage base directory |
| `ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:3001` | CORS allowed origins |

### Worker Node

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKER_GRPC_PORT` | `:50052` | Worker gRPC listen port |
| `WORKER_METRICS_PORT` | `:9101` | Prometheus metrics port |
| `WORKER_OUTPUT_DIR` | `./output` | Task output directory |
| `WORKER_ID` | _(auto-generated)_ | Persistent worker identifier |
| `MASTER_ADDR` | _(set on registration)_ | Master gRPC address |

### Web UI

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_BASE_URL` | `http://localhost:8080` | Backend API URL |
| `VITE_WS_BASE_URL` | `ws://localhost:8080` | Backend WebSocket URL |
| `WEBUI_PORT` | `3001` | Dev server port |

---

## 8. Starting the Master Node

### Option A: Shell Script

```bash
./runMaster.sh
```

### Option B: Direct Go Command

```bash
cd master

# CLI mode (default)
go run main.go
go run main.go --mode cli

# TUI mode
go run main.go --mode tui
```

### Option C: Built Binary

```bash
make build-master
./master/masterNode --mode tui
```

### Startup Sequence

1. Load `.env` configuration
2. Initialize file storage (`/var/cloudai/files` or `~/.cloudai/files`)
3. Collect system hardware info
4. Connect to MongoDB and ensure collections exist
5. Initialize database handlers (Workers, Tasks, Assignments, Attempts, Results, Files, Users)
6. Start Telemetry Manager (30s inactivity timeout)
7. Create scheduler stack (RR → RTS → optionally PPO)
8. Start gRPC server on `GRPC_PORT`
9. Start HTTP server on `HTTP_PORT` (REST API + WebSocket + Prometheus `/metrics`)
10. Launch CLI or TUI interface

---

## 9. Starting Workers

### Option A: Shell Script

```bash
./runWorker.sh
```

### Option B: Direct Go Command

```bash
cd worker
go run main.go
```

### Startup Sequence

1. Validate Docker is running
2. Create output directory with secure permissions (0700)
3. Detect system resources (CPU cores, memory, storage)
4. Resolve network address (auto-detect IP)
5. Generate or load persistent Worker ID
6. Start gRPC server on `WORKER_GRPC_PORT`
7. Start Prometheus metrics server on `WORKER_METRICS_PORT`
8. Wait for master registration (master initiates via `MasterRegister` RPC)
9. Begin heartbeat loop (every 5 seconds): CPU%, memory%, storage%, running tasks

### Worker Registration Flow

Workers don't register themselves — the master initiates:

```
CLI/API:  register worker-1 192.168.1.100:50052
             │
Master:   calls worker's MasterRegister(masterAddr)
             │
Worker:   stores master address, begins heartbeats
             │
Worker:   calls master's RegisterWorker(workerInfo)
             │
Master:   stores worker in MongoDB, marks active
```

### Task Execution Lifecycle

```
1. Master → AssignTask(taskID, image, resources) → Worker
2. Worker validates task ID, image name, resource requirements
3. Worker pulls Docker image (with offline fallback)
4. Worker creates container:
   - CPU limit (NanoCPUs)
   - Memory limit (bytes)
   - Capabilities dropped (ALL), minimal CapAdd
   - PID limit: 512 (fork bomb prevention)
   - no-new-privileges security option
   - Network mode: bridge
5. Worker starts container and log streaming
6. Worker waits for container exit
7. Worker collects container stats (CPU seconds, peak memory, IO bytes)
8. Worker gathers output files from /output mount
9. Worker uploads files to master
10. Worker reports result (exit code, duration, logs)
11. Worker cleans up container
```

---

## 10. Starting the PPO Agentic Scheduler

### Start the Server

```bash
cd agentic_scheduler
python -m agentic_scheduler --port 50061
```

### Server CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `50061` | gRPC listen port |
| `--host` | `127.0.0.1` | Bind address |
| `--mongo-uri` | `mongodb://localhost:27017` | MongoDB connection |
| `--mongo-db` | `cloudai` | Database name |
| `--model-path` | `latest` | Model file or `latest` to load from MongoDB |
| `--device` | `auto` | Compute device: `cpu`, `cuda`, or `auto` |
| `--log-level` | `INFO` | Logging level |
| `--online-updates` | `true` | Enable online learning |

### gRPC Methods

| Method | Purpose |
|--------|---------|
| `Ping()` | Health check, returns status |
| `LoadModelForFingerprint(fingerprint)` | Initialize model for cluster configuration |
| `SelectWorker(task, workers[])` | Choose best worker for a task |
| `ReportOutcome(task, worker, result)` | Report task result for online learning |

### PPO Model Architecture

- **Input**: 14 features (5 task + 9 per worker)
- **Network**: 2-layer ReLU (14 → 128 → 128)
- **Actor head**: Outputs action probabilities over workers
- **Critic head**: Outputs state value estimate
- **Action masking**: Infeasible workers masked (insufficient resources)

### Task Features (5 dimensions)
| Feature | Description |
|---------|-------------|
| CPU required | Normalized CPU cores |
| Memory required | Normalized memory (GB) |
| Storage required | Normalized storage (GB) |
| SLA multiplier | k-value (1.5–2.5) |
| Task type | One-hot encoded (cpu-light, cpu-heavy, memory-heavy, mixed) |

### Worker Features (9 dimensions per worker)
| Feature | Description |
|---------|-------------|
| Available CPU % | Fraction of CPU available |
| Available Memory % | Fraction of memory available |
| Available Storage % | Fraction of storage available |
| Total CPU | Absolute CPU cores |
| Total Memory | Absolute memory (GB) |
| Total Storage | Absolute storage (GB) |
| CPU utilization | Current usage ratio |
| Memory utilization | Current usage ratio |
| Running task count | Number of active tasks |

### Training the PPO Model

#### Offline Training

```bash
python -m agentic_scheduler.train_ppo \
  --num-workers 8 \
  --rollout-steps 256 \
  --updates 500 \
  --output models/ppo_trained.pt \
  --trace-source cloudai \
  --mongo-uri mongodb://localhost:27017
```

#### Training CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--num-workers` | `8` | Simulated worker count |
| `--rollout-steps` | `256` | Steps per rollout |
| `--updates` | `500` | Number of PPO updates |
| `--output` | `models/ppo_model.pt` | Output model path |
| `--trace-source` | `synthetic` | Source: `synthetic`, `cloudai`, `alibaba`, `google` |
| `--mongo-uri` | `mongodb://localhost:27017` | MongoDB for trace loading |
| `--lr` | `3e-4` | Learning rate |
| `--gamma` | `0.99` | Discount factor |
| `--clip-eps` | `0.2` | PPO clipping epsilon |
| `--epochs` | `4` | PPO epochs per update |
| `--batch-size` | `64` | Mini-batch size |

### Deployment Modes

| Mode | Behavior |
|------|----------|
| `shadow` | PPO makes decisions but RTS is used for actual scheduling (for validation) |
| `active` | PPO decisions drive actual task scheduling |
| `fallback` | RTS used normally; PPO only if RTS fails |

Configure via `PPO_DEPLOYMENT_MODE` environment variable.

---

## 11. Master CLI Reference

When running in CLI mode (`--mode cli`), the master provides an interactive command prompt.

### Worker Management

```bash
# Register a worker
register <worker_id> <worker_ip:port>
register worker-1 192.168.1.100:50052

# Unregister a worker
unregister <worker_id>
unregister worker-1

# List all workers
workers
```

### Task Management

```bash
# Submit a task
task <docker_image> -cpu_cores <n> -mem <n> -storage <n> [-k <value>] [-type <type>] [-name <name>]
task ubuntu:20.04 -cpu_cores 2 -mem 4 -storage 10 -k 2.0 -type cpu-heavy -name my-job

# Manually dispatch a task to specific workers (scheduler candidates)
dispatch <task_id> <worker_id> [other_worker_ids...]
dispatch task-abc123 worker-1 worker-2 worker-3

# Cancel a running task
cancel <task_id>
cancel task-abc123

# List tasks (optionally filtered by status)
list-tasks [status]
list-tasks               # all tasks
list-tasks running       # only running
list-tasks pending       # only pending
list-tasks completed     # only completed
list-tasks failed        # only failed

# Show scheduling queue
queue
```

### Monitoring

```bash
# Live cluster status (refreshes every 2s, press any key to exit)
status

# Live worker statistics
stats <worker_id>
stats worker-1

# Internal state dump
internal-state

# Live task log streaming
monitor <task_id>
monitor task-abc123
```

### File Operations

```bash
# List files for a user
files <user_id> [requesting_user]
files alice

# List files for a specific task
task-files <task_id> <user_id> [requesting_user]
task-files task-abc123 alice

# Download task files
download <task_id> <user_id> [requesting_user] [output_dir]
download task-abc123 alice alice ./my-outputs
```

### Diagnostics

```bash
# Reconcile allocated vs available resources
fix-resources

# Show help
help
```

### Benchmarking & Testing

```bash
# Run a benchmark workload
benchmark <workload> [-seed <seed>] [-out <dir>] [-speed <factor>] [-limit <n>] [-dry-run]
benchmark high-compute -seed 42 -out /tmp/results -speed 1.0

# Submit a custom workload from file
workload-submit <config_file>

# List available test suites
test list

# Run a test suite
test run <suite> [-profile <name>] [-scheduler <name>]
test run e2e -profile hetero-small -scheduler RTS

# Clean up test environment
test cleanup
```

### Session

```bash
# Graceful shutdown
exit
quit
```

---

## 12. TUI (Terminal UI) Reference

Start TUI mode:

```bash
go run main.go --mode tui
# Or:
export CLOUDAI_UI_MODE=tui
go run main.go
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Tab` | Cycle to next pane |
| `Shift+Tab` | Cycle to previous pane |
| `1`–`6` | Jump directly to pane by number |
| `/` | Focus command bar (type CLI commands) |
| `Enter` | Execute command in command bar |
| `Esc` | Dismiss command bar / back |
| `r` | Refresh data |
| `m` | Monitor (pre-fills `monitor <task_id>` or `stats <worker_id>`) |
| `c` | Cancel task (pre-fills `cancel <task_id>`) |
| `u` | Unregister worker (pre-fills `unregister <worker_id>`) |
| `q` | Quit subview / close modal |
| `Ctrl+C` | Exit TUI |

### Panes

#### 1: Overview
- Cluster status summary (worker count, task count, queue depth)
- Resource utilization bars (CPU, Memory, Storage)
- Active scheduler name
- Master host resources

#### 2: Workers
- Table: Worker ID, IP, Status, Last Heartbeat, CPU/Memory/Storage Usage, Task Count
- Running task sub-lines per worker
- Select a worker and press `u` to unregister, `m` for stats

#### 3: Tasks
- Table: Task ID, Name, User ID, Docker Image, Tag, Status, Worker, CPU/Mem/Storage, Created, SLA k
- Status grouped by priority (running → pending → queued → completed → failed → cancelled)
- Status emojis: 🔄 running, ⏳ pending, 📋 queued, ✅ completed, ❌ failed, 🚫 cancelled
- Select a task and press `m` to stream logs, `c` to cancel

#### 4: Queue
- Table: Task ID, Image, User, CPU/Mem/Storage Req, Queued At, Time in Queue, Retries, Last Error

#### 5: Logs
- Real-time log streaming for a monitored task
- Terminal-style output display
- Press `q` to exit log view

#### 6: Activity
- Chronological event feed
- Categories: worker, task, scheduler, system
- Includes timestamps and severity levels

### Command Bar

Press `/` to open the command bar. You can type any CLI command (from Section 11) and press `Enter` to execute. The TUI renders the result inline.

---

## 13. HTTP REST API Reference

The master exposes a REST API on `HTTP_PORT` (default `:8080`).

### Authentication

#### `POST /api/auth/register`

Register a new user account.

```json
// Request
{
  "name": "Alice",
  "email": "alice@example.com",
  "password": "secure_password_123"
}

// Response 201
{
  "message": "User registered successfully",
  "user": { "name": "Alice", "email": "alice@example.com" }
}
```

#### `POST /api/auth/login`

Authenticate and receive a JWT cookie.

```json
// Request
{ "email": "alice@example.com", "password": "secure_password_123" }

// Response 200 (sets auth_token cookie)
{ "message": "Login successful", "user": { "name": "Alice", "email": "alice@example.com" } }
```

#### `POST /api/auth/logout`

Clears the `auth_token` cookie.

#### `GET /api/auth/me` _(requires auth)_

Returns current authenticated user info.

### Tasks

#### `POST /api/tasks`

Submit a new task for scheduling.

```json
// Request
{
  "docker_image": "python:3.9",
  "command": "python -c 'print(42)'",
  "cpu_required": 2.0,
  "memory_required": 4.0,
  "storage_required": 10.0,
  "user_id": "alice",
  "tag": "cpu-light",
  "k_value": 2.0
}

// Response 201
{
  "task_id": "task-abc123",
  "status": "queued",
  "created_at": 1714050000,
  "details": { "position": 1, "message": "Task queued for execution" }
}
```

#### `GET /api/tasks`

List all tasks. Optional query param: `?status=running`

#### `GET /api/tasks/{task_id}`

Get details for a specific task.

#### `DELETE /api/tasks/{task_id}`

Cancel a task.

#### `GET /api/tasks/{task_id}/logs`

Get task logs (HTTP).

### Workers

#### `GET /api/workers`

List all registered workers with resource info.

#### `POST /api/workers`

Register a worker via API.

```json
{ "worker_id": "worker-1", "address": "192.168.1.100:50052" }
```

#### `GET /api/workers/{worker_id}`

Get specific worker details.

#### `GET /api/workers/{worker_id}/metrics`

Get worker metrics.

### Files

#### `GET /api/files?user_id={user}&requesting_user={user}`

List all files for a user.

#### `GET /api/files/{task_id}?user_id={user}`

List files for a specific task.

```json
// Response
{
  "task_id": "task-abc123",
  "task_name": "my-task",
  "files": [
    { "path": "output.txt", "size": 1024 },
    { "path": "result.json", "size": 512 }
  ],
  "total_size": 1536
}
```

#### `GET /api/files/{task_id}/download/{file_path}?user_id={user}`

Download a specific output file.

#### `DELETE /api/files/{task_id}?user_id={user}`

Delete all files for a task.

### Telemetry

#### `GET /health`

Health check endpoint.

```json
{ "status": "healthy", "time": 1714050000, "active_clients": 2, "workers": 3, "active_workers": 3 }
```

#### `GET /telemetry`

All worker telemetry (REST snapshot).

#### `GET /telemetry/{worker_id}`

Specific worker telemetry.

#### `GET /metrics`

Prometheus metrics endpoint (scraped by Prometheus).

### WebSocket Endpoints

#### `WS /ws/telemetry`

Streams real-time telemetry for all workers. JSON messages pushed on every heartbeat update.

#### `WS /ws/telemetry/{worker_id}`

Streams telemetry for a specific worker.

#### `WS /ws/tasks/{task_id}`

Streams live task logs as the container executes.

**Message types:**
- `connected` — Connection established
- `log` — Container stdout/stderr line
- `error` — Task error
- `complete` — Task finished (includes exit code)

---

## 14. Web UI (React Dashboard)

### Setup

```bash
cd ui
npm install
npm run dev     # Starts on http://localhost:3001
```

### Build for Production

```bash
npm run build      # Output in dist/
npm run preview    # Preview production build on :4173
```

### Pages

#### Login (`/login`)
- Email + password authentication
- Redirects to Dashboard on success

#### Register (`/register`)
- Name, email, password (6+ characters)
- Redirects to login on success

#### Dashboard (`/`)
- **Overview Cards**: Total tasks, running tasks, active workers, cluster saturation %
- **Resource Capacity**: CPU/Memory/Storage bars with allocated vs available
- **Live Benchmarking** (5-min rolling window): Throughput, failure rate, success rate, queue pressure, avg queue age
- **Charts** (real-time):
  - Resource Utilization Trend (line chart)
  - Worker Load Distribution (bar chart, top 8 workers)
  - Task State Trend (line chart)
- Auto-refreshes every 3–5 seconds via polling + WebSocket

#### Tasks (`/tasks`)
- Table: Task ID, Image, Tag (color chip), K-Value, Status (color chip), Resources, Created
- Auto-refreshes every 3 seconds
- Click a task row to view live streaming logs
- Cancel tasks with confirmation dialog

#### Submit Task (`/submit`)
- **Docker Image** — e.g., `python:3.9` (validated format)
- **Task Tag** — `cpu-light`, `cpu-heavy`, `memory-heavy`, `mixed` (color-coded selector)
- **K-Value** — Priority slider 1.5–2.5 (0.1 increments)
  - 1.5 = Low priority (background tasks)
  - 2.0 = Normal priority (default)
  - 2.5 = High priority (time-sensitive)
- **CPU** — 0.1–64 cores
- **Memory** — 0.5–256 GB
- **Storage** — 1–1000 GB (default: 5)
- **Command** — Optional Docker command override

#### Workers (`/workers`)
- Table: Worker ID, Address, Status (Active/Inactive chip), CPU/Memory/Storage bars
- Register Worker dialog (Worker ID + IP:Port)
- Real-time updates via WebSocket telemetry

### Real-time Features

- **WebSocket telemetry** (`/ws/telemetry`) — Worker resource updates, auto-reconnects (10 attempts, 3s delay)
- **Task log streaming** (`/ws/tasks/{id}/logs`) — Live container output in terminal-style dialog
- **Polling** — Task list refreshes every 3 seconds

---

## 15. Scheduler Algorithms

The master supports three scheduling algorithms, configured via `SCHED_ALGO`.

### Round-Robin (RR)

```
SCHED_ALGO=RR
```

- Cycles through available workers in order
- Stateless, no resource awareness
- Best for: Testing, homogeneous clusters

### Risk-aware Task Scheduling (RTS)

```
SCHED_ALGO=RTS
```

- Uses GA-optimized parameters for risk-based worker selection
- Predicts task runtime from historical data (tau values via EMA)
- Computes risk score per worker based on resource utilization
- Selects the worker with the lowest risk

**Key parameters** (from `config/ga_output.json`):
- **Theta1–4**: Resource utilization weights
- **Alpha**: Risk penalty coefficient
- **Beta**: Risk success bonus
- **AffinityMatrix**: Task-type to worker affinity (optional)
- **PenaltyVector**: Worker-specific penalties (optional)

**Adaptive Online Decision (AOD)**:
- Background thread runs every 60 seconds
- Learns from 24-hour historical window
- Updates weights via linear regression
- Persists to MongoDB (`RTS_WEIGHTS` collection)

**Default tau values** (expected runtime in seconds):
| Task Type | Default Tau |
|-----------|------------|
| `cpu-light` | 5.0s |
| `cpu-heavy` | 15.0s |
| `memory-heavy` | 20.0s |
| `mixed` | 10.0s |

Tau is updated via EMA: `tau_new = 0.2 * actual_runtime + 0.8 * tau_old`

### Proximal Policy Optimization (PPO)

```
SCHED_ALGO=PPO
PPO_DEPLOYMENT_MODE=active
```

- Neural network-based scheduler trained via reinforcement learning
- Considers 14 features per scheduling decision (5 task + 9 per worker)
- Learns optimal placement through reward signals
- Falls back to RTS if PPO service is unavailable

See [Section 10](#10-starting-the-ppo-agentic-scheduler) for full details.

---

## 16. Testbench & Integration Testing

### Overview

The testbench provides a fully containerized environment for integration testing with observability.

### Quick Start

```bash
# Fully containerized (master + 3 workers in Docker)
cd testbench
docker compose up -d
# Access: Grafana at http://localhost:3000, Master HTTP at http://localhost:8080

# Host-master mode (master on host, workers in Docker)
docker compose -f docker-compose.host-master.yml up -d
cd ../master && go run main.go --mode cli
```

### Docker Compose Services

#### Fully Containerized (`docker-compose.yml`)

| Service | Image | Ports | Resources |
|---------|-------|-------|-----------|
| `master` | `cloudai/master` | `8080`, `50051` | — |
| `worker-small` | `cloudai/worker` | `9101` | 1 CPU, 1.5 GB |
| `worker-medium` | `cloudai/worker` | `9101` | 2 CPU, 3 GB |
| `worker-large` | `cloudai/worker` | `9101` | 3 CPU, 5 GB |
| `mongo` | `mongo:7` | `127.0.0.1:27017` | — |
| `prometheus` | `prom/prometheus` | `9090` | — |
| `grafana` | `grafana/grafana` | `3000` | — |

#### Host-Master Variant (`docker-compose.host-master.yml`)

Same as above but master runs on the host. Workers connect via `host.docker.internal`.

### Running Integration Tests

```bash
# Run full integration suite
make test-integration

# Run specific test suite
cd testbench/scripts
bash run_integration.sh

# Run test campaign (multiple scenarios)
python run_campaign.py --config ../scenarios/evidence.json

# Run a specific suite via master CLI
cd master
go run main.go test run e2e -profile hetero-small -scheduler RTS
```

### Available Test Suites

| Suite | Purpose |
|-------|---------|
| `smoke` | Basic functionality: worker registration, task submit, completion |
| `reliability` | Failure injection, worker timeout recovery, task requeuing |
| `e2e` | Full end-to-end: submit → schedule → execute → result |
| `ui-smoke` | Web UI auth and page loading |
| `evidence` | Scheduler comparison benchmarks for research |

### Workload Types

| Workload | CPU | Memory | Duration | Description |
|----------|-----|--------|----------|-------------|
| `cpu-light` | 0.5 | 0.5 GB | ~5s | Light computation |
| `cpu-heavy` | 2.0 | 1.0 GB | ~15s | Intensive computation |
| `memory-heavy` | 0.5 | 2.0 GB | ~20s | Memory-intensive |
| `mixed` | 1.0 | 1.0 GB | ~10s | Balanced workload |
| `quick-pass` | 0.25 | 0.25 GB | ~2s | Fast success |
| `quick-fail` | 0.25 | 0.25 GB | ~2s | Intentional failure |
| `bad-image` | 0.25 | 0.25 GB | — | Invalid image (tests error handling) |
| `high-compute` | 4.0 | 4.0 GB | ~30s | Heavy resource usage |
| `stress` | 2.0 | 2.0 GB | ~60s | Long-running stress test |

### Failure Injection

The testbench supports 7 failure injection actions for chaos testing:

1. **Kill worker container** — Simulates worker crash
2. **Pause worker** — Simulates network partition
3. **Unpause worker** — Resume after partition
4. **Restart worker** — Clean restart
5. **Network disconnect** — Docker network isolation
6. **Network reconnect** — Restore connectivity
7. **Stress injection** — CPU/memory pressure on worker

### Running Go Unit Tests

```bash
# All master tests
cd master && go test ./...

# Specific test packages
go test ./internal/server/ -v        # Server lifecycle, scheduling, recovery
go test ./internal/scheduler/ -v     # Scheduler unit tests
go test ./internal/db/ -v            # Database tests
go test ./internal/cli/ -v           # CLI test workflows

# Worker tests
cd worker && go test ./...
go test ./internal/telemetry/ -v     # Telemetry tests

# With race detection
go test -race ./...

# With coverage
go test -cover ./...
```

### Exporting Metrics

```bash
# Export Prometheus metrics to JSON
python testbench/scripts/export_metrics.py \
  --prometheus-url http://localhost:9090 \
  --output results/metrics.json
```

---

## 17. Observability (Prometheus & Grafana)

### Prometheus

**URL:** `http://localhost:9090`

Prometheus scrapes metrics from:
- Master: `master:8080/metrics` (or `host.docker.internal:8080`)
- Workers: `worker-{small,medium,large}:9101/metrics`

#### Master Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `cloudai_master_queue_depth` | Gauge | — | Current scheduling queue size |
| `cloudai_master_tasks_enqueued_total` | Counter | `reason` | Tasks added to queue |
| `cloudai_master_tasks_dequeued_total` | Counter | `outcome` | Tasks removed from queue |
| `cloudai_master_scheduling_latency_seconds` | Histogram | `scheduler` | Worker selection time |
| `cloudai_master_task_queue_wait_seconds` | Histogram | `scheduler`, `task_type` | Time in queue |
| `cloudai_master_scheduler_selections_total` | Counter | `scheduler`, `task_type`, `worker_id` | Selection counts |
| `cloudai_master_task_terminal_total` | Counter | `status`, `task_type` | Tasks reaching terminal state |
| `cloudai_master_worker_timeouts_total` | Counter | `worker_id` | Heartbeat timeouts |
| `cloudai_master_task_requeues_total` | Counter | `failure_reason`, `task_type` | Recovery requeues |
| `cloudai_master_stale_results_total` | Counter | `reason` | Late/stale results ignored |
| `cloudai_master_recovery_duration_seconds` | Histogram | `failure_reason` | Recovery time |

#### Worker Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `cloudai_worker_last_heartbeat_unix` | Gauge | — | Last heartbeat timestamp |
| `cloudai_worker_running_tasks` | Gauge | — | Current running task count |
| `cloudai_worker_resource_usage_ratio` | Gauge | `resource` | CPU/memory/storage ratio |
| `cloudai_worker_task_image_pull_seconds` | Histogram | `task_type` | Image pull duration |
| `cloudai_worker_task_container_create_seconds` | Histogram | `task_type` | Container creation time |
| `cloudai_worker_task_runtime_seconds` | Histogram | `task_type`, `status` | Task execution time |
| `cloudai_worker_container_cpu_seconds_total` | Counter | `task_type` | Cumulative CPU usage |
| `cloudai_worker_container_memory_peak_bytes` | Histogram | `task_type` | Peak memory usage |
| `cloudai_worker_container_io_bytes_total` | Counter | `task_type` | I/O bytes consumed |
| `cloudai_worker_docker_errors_total` | Counter | `stage`, `task_type` | Docker errors by stage |
| `cloudai_worker_task_starts_total` | Counter | `task_type` | Task attempt starts |

### Grafana

**URL:** `http://localhost:3000`

Three dashboards are provisioned automatically:

#### Dashboard 1: CloudAI Overview

**At-a-glance stats:**
- Current Queue Depth
- Total Running Tasks
- Active Workers
- Task Success Rate (%)
- Avg Scheduling Latency

**Timeseries panels:**
- Queue depth over time
- Task throughput (enqueued vs dequeued rate)
- Worker resource usage (CPU/memory/storage by instance)
- Running tasks per worker
- Error rates (requeues, timeouts)

**Template variables:** `$scheduler`, `$task_type` for filtering

#### Dashboard 2: CloudAI Scheduler & Queue

**Stats:** P50 scheduling latency, P95 scheduling latency, P99 queue wait

**Heatmaps:**
- Scheduling latency distribution
- Queue wait time distribution

**Timeseries:**
- Worker selections by scheduler/task_type
- Queue throughput
- Terminal task states (stacked by status)
- Benchmark flow counters

#### Dashboard 3: CloudAI Worker Runtime

**Stats:** Total task starts, avg task runtime, Docker error rate

**Timeseries:**
- Task runtime breakdown (image pull, container create, execution)
- Docker errors by stage
- Container CPU consumption by task type
- Container memory peak (p95)
- Container I/O by task type
- Worker heartbeat freshness (`time() - last_heartbeat_unix`)

---

## 18. Makefile Reference

Key targets (run from project root):

### Build

```bash
make build              # Build master + worker binaries
make build-master       # Build master only
make build-worker       # Build worker only
make build-ui           # Build React UI for production
```

### Run

```bash
make run-master         # Start master node
make run-worker         # Start worker node
make run-ui             # Start React UI dev server
make run-all            # Start everything
make run-ppo            # Start PPO scheduler
```

### Test

```bash
make test               # Run all Go tests
make test-master        # Run master tests only
make test-worker        # Run worker tests only
make test-race          # Run tests with race detector
make test-cover         # Run tests with coverage report
make test-integration   # Run integration test suite
```

### Docker & Testbench

```bash
make docker-build       # Build Docker images
make testbench-up       # Start testbench environment
make testbench-down     # Stop testbench
make testbench-logs     # View testbench logs
```

### Utilities

```bash
make proto              # Regenerate protobuf code
make install-deps       # Install all dependencies
make clean              # Clean build artifacts
make lint               # Run linters
make fmt                # Format code
```

---

## 19. CI/CD Pipeline

The GitHub Actions workflow (`.github/workflows/testbench-integration.yml`) runs on pushes and PRs:

### Pipeline Steps

1. **Checkout** — Clone repository
2. **Setup Go** — Install Go toolchain
3. **Setup Python** — Install Python + dependencies
4. **Build** — Compile master and worker
5. **Unit Tests** — Run `go test ./...` for master and worker
6. **Docker Build** — Build container images
7. **Testbench Up** — Start Docker Compose environment
8. **Integration Tests** — Run integration suite
9. **Metrics Export** — Export Prometheus metrics as artifacts
10. **Testbench Down** — Tear down environment
11. **Artifacts** — Upload test results and metrics

---

## 20. Troubleshooting

### MongoDB Connection Failed

```
Error: failed to connect to MongoDB
```
- Ensure MongoDB is running: `docker compose -f database/docker-compose.yml up -d`
- Verify `MONGODB_URI` in `.env`
- Check credentials match Docker Compose configuration

### Worker Not Registering

```
Error: worker not responding
```
- Ensure Docker is running on the worker host
- Check worker gRPC port is accessible from master
- Verify firewall rules allow the connection
- Run `register <worker_id> <ip:port>` from master CLI

### Task Stuck in "Pending"

- Check available workers: `workers` command
- Verify workers have sufficient resources for the task
- Check queue: `queue` command
- Try `fix-resources` to reconcile resource accounting

### Docker Image Pull Failed

```
Error: image pull failed
```
- Verify Docker is running on the worker
- Check internet connectivity
- Ensure image name is valid: `registry/image:tag`
- For private registries, ensure Docker is authenticated

### PPO Scheduler Not Connecting

```
Error: PPO service unavailable
```
- Check PPO server is running: `python -m agentic_scheduler --port 50061`
- Verify `PPO_GRPC_ADDR` matches the running server
- Check MongoDB is accessible for model loading
- The system will fall back to RTS automatically

### Web UI Not Loading

- Ensure master HTTP server is running on port 8080
- Check `npm run dev` is running in `ui/`
- Verify Vite proxy settings in `vite.config.js`
- Check browser console for CORS errors
- Ensure `ALLOWED_ORIGINS` includes the UI URL

### WebSocket Disconnections

- WebSocket auto-reconnects 10 times with 3-second delays
- Check network stability between client and master
- Verify WebSocket endpoints are not blocked by proxy/firewall
- Monitor browser Network tab for connection status

### Grafana Shows No Data

- Ensure Prometheus is scraping targets: check `http://localhost:9090/targets`
- Verify master and worker metrics ports are accessible
- Check Grafana datasource: Settings → Data Sources → Prometheus
- Ensure time range is appropriate (default: last 15 minutes)
- Submit some tasks to generate metric data

### Port Conflicts

```bash
# Find process using a port
lsof -i :8080
lsof -i :50051

# Override ports via environment
GRPC_PORT=:50055 HTTP_PORT=:8085 go run main.go
WEBUI_PORT=3002 npm run dev
```

---

## 21. Data Lifecycle — Provisioning, Processing & Consumption

This section traces the end-to-end lifecycle of data through the system, from raw inputs to trained scheduling models and production decisions.

### 21.1 Data Flow Overview

```
                     ┌─────────────────────────────────────────────────────────┐
                     │                  DATA SOURCES                          │
                     │                                                         │
                     │  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐  │
                     │  │ Alibaba  │  │ Google   │  │ CloudAI Live Cluster │  │
                     │  │ Traces   │  │ Traces   │  │ (MongoDB history)    │  │
                     │  └────┬─────┘  └────┬─────┘  └──────────┬───────────┘  │
                     └───────┼─────────────┼───────────────────┼──────────────┘
                             │             │                   │
                     ┌───────▼─────────────▼───────────────────▼──────────────┐
                     │              TRACE LOADERS (Provisioning)               │
                     │  Normalize → TraceTask → TraceCluster                  │
                     └────────────────────────┬───────────────────────────────┘
                                              │
               ┌──────────────────────────────┼──────────────────────────────┐
               │                              │                              │
      ┌────────▼─────────┐          ┌─────────▼──────────┐       ┌───────────▼──────────┐
      │  SchedulingEnv   │          │  TraceReplayEnv    │       │   AOD Trainer        │
      │  (Synthetic)     │          │  (Real traces)     │       │   (RTS learning)     │
      │  Gymnasium env   │          │  Gymnasium env     │       │   Linear regression  │
      └────────┬─────────┘          └─────────┬──────────┘       └───────────┬──────────┘
               │                              │                              │
               │ observations + rewards       │ observations + rewards       │ Theta, Affinity,
               │                              │                              │ Penalty
      ┌────────▼──────────────────────────────▼──────────┐       ┌───────────▼──────────┐
      │             PPO Training Loop                     │       │   GA Params (JSON)   │
      │  rollout → GAE → policy gradient → checkpoint    │       │   + MongoDB store    │
      └───────────────────────┬───────────────────────────┘       └───────────┬──────────┘
                              │                                               │
                     ┌────────▼──────────┐                        ┌───────────▼──────────┐
                     │  Model (.pt file) │                        │  RTS Scheduler       │
                     │  + MongoDB GridFS │                        │  (risk scoring)      │
                     └────────┬──────────┘                        └───────────┬──────────┘
                              │                                               │
                     ┌────────▼──────────────────────────────────────────────▼──┐
                     │                PRODUCTION SCHEDULING                     │
                     │  Task arrives → Feature extraction → Model inference →   │
                     │  Worker selected → Task dispatched → Outcome reported    │
                     └──────────────────────────┬──────────────────────────────┘
                                                │
                                       ┌────────▼────────┐
                                       │  Online Learning │
                                       │  (PPO updates   │
                                       │   from outcomes) │
                                       └─────────────────┘
```

### 21.2 Data Provisioning — Where Does the Data Come From?

The system uses three categories of data:

#### A) External Cluster Traces (Offline Training)

Used to train the PPO model before deployment.

**Alibaba cluster-trace-v2018** (`trace_loader.load_alibaba_trace`):
- **Source**: Public dataset from Alibaba's production cluster
- **Files needed**: `batch_task.csv` (task definitions) + `machine_meta.csv` (machine specs)
- **Raw fields**: `task_name`, `start_time`, `end_time`, `plan_cpu`, `plan_mem`, `task_type`, `status`
- **Normalization**:
  - CPU: Alibaba uses centi-cores (100 = 1 core). Values >10 are divided by 100, clamped to ≥0.1
  - Memory: Alibaba uses fractions of host budget. Values ≤1.0 are multiplied by 64 GB
  - Task types: 12 Alibaba types mapped to 4 canonical types (`cpu-light`, `cpu-heavy`, `memory-heavy`, `mixed`)
  - Timestamps: Offset-normalized so first task arrival = 0.0 seconds

**Google ClusterData2019** (`trace_loader.load_google_trace`):
- **Source**: Public dataset from Google's Borg cluster
- **Files needed**: `instance_events.json` + `machine_events.json` (or CSV equivalents)
- **Normalization**:
  - Timestamps: Google uses microseconds — divided by 1e6 for seconds
  - Memory: Normalized fraction — multiplied by 32 GB for approximate absolute value
  - Task types: Auto-classified from CPU/memory ratio using `_classify_task_type()`

**CloudAI Own History** (`trace_loader.load_cloudai_trace`):
- **Source**: The system's own MongoDB (`TASKS`, `ATTEMPTS`, `RESULTS` collections)
- **Loading**: Directly from MongoDB via `mongo_uri` or from exported JSON/CSV files
- **Fields**: `task_id`, `req_cpu`, `req_memory`, `req_storage`, `runtime_seconds`, `task_type`, `sla_multiplier`, `queue_wait_seconds`, `sla_success`, `failure_reason`
- **Time windowing**: Can filter by `trace_window_start` / `trace_window_end` (ISO 8601)
- **File formats**: Supports `.json`, `.jsonl`, and `.csv`

#### B) Live Telemetry Data (Runtime)

Collected continuously from workers during operation:

| Source | Data | Collection Method | Frequency |
|--------|------|-------------------|-----------|
| Worker heartbeats | CPU%, memory%, storage% usage | gRPC `SendHeartbeat` | Every 5 seconds |
| Task execution | Runtime, exit code, CPU seconds, peak memory, I/O bytes | gRPC `ReportTaskCompletion` | On task completion |
| Container stats | `docker stats` metrics | Docker API | Per task execution |
| System resources | Total CPU cores, memory, storage | `gopsutil` library | On worker startup |

#### C) GA-Optimized Parameters (AOD Training)

Generated by the Adaptive Online Decision (AOD) trainer from historical data:

- **Input**: Last 24 hours of task history + worker stats from MongoDB
- **Output**: `GAParams` (Theta weights, Affinity matrix, Penalty vector)
- **Training frequency**: Every 60 seconds (background goroutine)
- **Minimum data**: At least 2 completed tasks required; defaults used otherwise

### 21.3 Data Processing — How Is Data Transformed?

#### Stage 1: Trace Loading & Normalization

Raw traces are loaded into a canonical `TraceCluster` structure:

```python
@dataclass
class TraceTask:
    task_id: str
    arrival_time: float       # seconds from trace start (normalized)
    req_cpu: float            # CPU cores
    req_memory: float         # GB
    req_storage: float        # GB
    runtime_seconds: float    # actual duration
    task_type: str            # cpu-light | cpu-heavy | memory-heavy | mixed
    sla_multiplier: float     # k-value (1.5–2.5)

@dataclass
class TraceCluster:
    workers: List[Dict]       # [{worker_id, total_cpu, total_memory, total_storage}]
    tasks: List[TraceTask]    # chronologically sorted
    source: str               # alibaba-v2018 | google-2019 | cloudai-history
```

Processing steps:
1. Parse raw CSV/JSON/MongoDB records
2. Normalize resource units to canonical format (CPU cores, GB memory)
3. Classify task types using resource ratio heuristics
4. Sort tasks by arrival time
5. Offset-normalize timestamps (first task = time 0)
6. Cap at `max_tasks` (default: 5000) to limit memory

#### Stage 2: Feature Extraction

Every scheduling decision transforms raw task + worker data into neural network input:

**Task features** (5 dimensions):
```
[req_cpu, req_memory, req_storage, sla_multiplier, task_type_scalar]
                                                        ↑
                                               Encoded: cpu-light=0.0,
                                               cpu-heavy=0.33, memory-heavy=0.67, mixed=1.0
```

**Worker features** (9 dimensions per worker):
```
[avail_cpu/total_cpu,    # Available CPU ratio
 avail_mem/total_mem,    # Available memory ratio
 avail_stor/total_stor,  # Available storage ratio
 total_cpu,              # Absolute CPU capacity
 total_memory,           # Absolute memory capacity
 total_storage,          # Absolute storage capacity
 used_cpu/total_cpu,     # CPU utilization ratio
 used_mem/total_mem,     # Memory utilization ratio
 used_stor/total_stor]   # Storage utilization ratio
```

**Action mask** (1 dimension per worker):
```
mask[i] = 1 if worker[i] has enough resources AND is active, else 0
```

**Running normalization** (`RunningNormalizer`):
- Maintains online mean/variance using Welford's algorithm
- Normalizes pairwise (task+worker) features: `(x - mean) / std`
- State is persisted with model checkpoints so normalization is consistent

#### Stage 3: Training Environments

Two Gymnasium environments convert trace data into RL training experiences:

**SchedulingEnv** (Synthetic):
- Generates random tasks and workers each episode
- Task types sampled uniformly; resource requirements sampled from distributions
- Worker loads decay by 15% (CPU/memory) and 10% (storage) per step
- **Reward**: `+1.2 - load_penalty` for feasible placement, `-1.4` for infeasible
- Episode length: 96 steps

**TraceReplayEnv** (Real traces):
- Replays `TraceCluster` tasks in chronological order
- Worker loads decay proportional to inter-arrival gap between tasks
- Can loop traces for longer training
- **Reward signal** (multi-component):
  - Feasibility: `+1.0` feasible, `-2.0` infeasible
  - Load balance: `+0.3 × (1 - load)` — reward low-load workers
  - SLA proxy: `+0.4` if predicted headroom > 0.5
  - Tail-risk: `-0.2 × max(0, load - 0.8)` — penalize overloading
  - Trace feedback: `+0.3` if trace recorded SLA success

#### Stage 4: PPO Training Loop

The training loop (`train_ppo.py`) executes:

```
for update in range(num_updates):
    # Rollout: collect experiences
    for step in range(rollout_steps):
        obs = env.observe()                    # task + worker features + mask
        action, log_prob, value = policy(obs)  # forward pass
        reward, done = env.step(action)        # execute action
        buffer.store(obs, action, reward, value, log_prob, done)

    # Compute advantages using GAE (Generalized Advantage Estimation)
    advantages = GAE(rewards, values, dones, gamma=0.99, lambda=0.95)
    returns = advantages + values

    # PPO update (4 epochs per batch)
    for epoch in range(4):
        for minibatch in buffer.sample(batch_size=64):
            new_log_prob, new_value, entropy = policy.evaluate(minibatch)
            ratio = exp(new_log_prob - old_log_prob)
            clipped = clip(ratio, 1-eps, 1+eps)
            policy_loss = -min(ratio * advantages, clipped * advantages)
            value_loss = MSE(new_value, returns)
            loss = policy_loss + 0.5 * value_loss - 0.01 * entropy
            optimizer.step(loss)

    # Checkpoint
    if update % checkpoint_interval == 0:
        save_model(policy, f"ppo_checkpoint_{update}.pt")
```

**Output**: `.pt` file containing `PPOActorCritic` weights + `RunningNormalizer` state

#### Stage 5: AOD Training (RTS Parameters)

The AOD trainer runs every 60 seconds in the master's background:

```
1. Fetch last 24h of task history from MongoDB
   → (task_id, worker_id, req_cpu, req_memory, runtime, sla_success, ...)

2. Train Theta (linear regression):
   → runtime_predicted = θ₁·(cpu_ratio) + θ₂·(mem_ratio) + θ₃·(storage_ratio) + θ₄·(load_factor)
   → Minimize: Σ(runtime_actual - runtime_predicted)²

3. Build Affinity Matrix (direct computation):
   → For each (task_type, worker_id) pair:
      affinity = SpeedAdvantage + SLAReliability
   → SpeedAdvantage = (avg_runtime_others - avg_runtime_this_worker) / avg_runtime_others
   → SLAReliability = sla_success_rate_this_worker - sla_success_rate_global

4. Build Penalty Vector (direct computation):
   → For each worker: penalty = f(failure_rate, timeout_rate, overload_time)

5. Persist GAParams → MongoDB (RTS_WEIGHTS collection) + JSON file (config/ga_output.json)
```

### 21.4 Data Consumption — How Is Data Used in Production?

#### RTS Scheduler Decision Flow

```
Task arrives → get_tau(task_type)            # EMA-smoothed runtime estimate
            → get_telemetry(all_workers)     # live CPU/mem/storage usage
            → load_ga_params()               # Theta, Affinity, Penalty from store

For each candidate worker:
  predicted_runtime = tau × (1 + θ₁·cpu_ratio + θ₂·mem_ratio + θ₃·stor_ratio + θ₄·load)
  sla_deadline = sla_multiplier × tau
  risk = α × max(0, predicted_runtime - sla_deadline)
       + β × worker_load
       - affinity[task_type][worker_id]
       + penalty[worker_id]

Select worker with minimum risk score
```

After task completion, tau is updated:
```
tau_new = 0.2 × actual_runtime + 0.8 × tau_old    # EMA with λ=0.2
```

#### PPO Scheduler Decision Flow

```
Task arrives → extract_task_features(task)         # 5-dim vector
            → extract_worker_features(workers)     # 9-dim × N workers
            → compute_action_mask(task, workers)   # feasibility check
            → normalize(task ⊕ worker features)    # RunningNormalizer
            → policy.forward(features, mask)        # Neural network inference
            → sample action (or argmax with headroom bias)
            → store DecisionRecord in pending_decisions[task_id]

Task completes → pop DecisionRecord
              → derive_reward(status, runtime, sla_success)
              → append to replay_buffer
              → if buffer ≥ batch_size: run PPO update (online learning)
              → periodically persist model to MongoDB GridFS
```

**Reward derivation** (when master doesn't provide explicit reward):
```python
reward = 0.0
if status in {success, completed}: reward += 1.0
elif status == cancelled:           reward -= 0.5
else:                               reward -= 1.0

if sla_success:  reward += 0.5
else:            reward -= 0.25

if runtime > 0:  reward -= min(runtime / 600, 0.5)  # penalize long tasks
```

#### Online Learning Cycle

```
                     ┌──────────────────────────┐
                     │    Production Traffic     │
                     │  (task submissions)       │
                     └────────────┬─────────────┘
                                  │
                     ┌────────────▼─────────────┐
                     │  PPO SelectWorker()       │
                     │  Store DecisionRecord     │
                     │  (features, action,       │
                     │   log_prob, value)         │
                     └────────────┬─────────────┘
                                  │
                     ┌────────────▼─────────────┐
                     │  Task Executes on Worker  │
                     └────────────┬─────────────┘
                                  │
                     ┌────────────▼─────────────┐
                     │  ReportOutcome()          │
                     │  Match DecisionRecord     │
                     │  Derive reward            │
                     │  Append to replay_buffer  │
                     └────────────┬─────────────┘
                                  │
                     ┌────────────▼─────────────┐
                     │  buffer ≥ batch_size?     │
                     │  YES → PPO mini-update    │
                     │  • Compute GAE advantages │
                     │  • Clip policy gradient   │
                     │  • Update network weights │
                     │  • Clear buffer           │
                     └────────────┬─────────────┘
                                  │
                     ┌────────────▼─────────────┐
                     │  Persist checkpoint       │
                     │  → MongoDB GridFS         │
                     │  (versioned, SHA-256)      │
                     └───────────────────────────┘
```

### 21.5 Model Persistence & Versioning

#### PPO Models (MongoDB GridFS)

```
Collection: SCHEDULER_MODELS
{
  scheduler_type: "PPO",
  fingerprint_hash: "<cluster-config-hash>",     # identifies cluster topology
  version: 3,                                     # monotonically increasing
  active: true,                                   # only one active per fingerprint
  file_id: ObjectId("..."),                       # GridFS reference
  file_size: 524288,
  file_sha256: "a1b2c3...",
  framework: "pytorch",
  created_at: ISODate("2026-04-25T..."),
  activated_at: ISODate("2026-04-25T...")
}

GridFS Bucket: scheduler_models (files + chunks collections)
Filename pattern: ppo_<fingerprint>_v<version>.ckpt
```

**Checkpoint contents** (`.pt` file via `torch.save`):
- `actor_critic_state_dict` — Neural network weights
- `optimizer_state_dict` — Adam optimizer state
- `normalizer_state` — RunningNormalizer mean/variance/count
- `model_version` — Version string
- `fingerprint_hash` — Cluster configuration hash

#### RTS Parameters (MongoDB + JSON)

```
Collection: RTS_WEIGHTS
{
  _id: "active",
  params: {
    Theta: { Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2 },
    Risk: { Alpha: 10.0, Beta: 1.0 },
    AffinityMatrix: { "cpu-heavy": { "worker-large": 0.35, ... }, ... },
    PenaltyVector: { "worker-small": 0.12, ... }
  },
  updated_at: ISODate("2026-04-25T...")
}

File fallback: config/ga_output.json (same structure)
```

### 21.6 Data Retention & Buffers

| Buffer | Max Size | Eviction Policy | Purpose |
|--------|----------|-----------------|---------|
| `pending_decisions` | 8,192 | FIFO (oldest evicted) | Awaiting task outcome for online learning |
| `replay_buffer` | 4,096 | Tail trim (keep newest) | Completed outcomes awaiting PPO update |
| Tau store | 4 entries | One per task type | Runtime EMA estimates |
| Telemetry data | Per-worker | Overwritten on heartbeat | Latest worker resource state |
| Worker heartbeat channel | 10 buffered | Non-blocking drop | Incoming heartbeats per worker |

### 21.7 Summary: Data Types at Each Stage

| Stage | Input Data | Processing | Output |
|-------|-----------|------------|--------|
| **Provisioning** | Raw CSV/JSON/MongoDB traces | Parse, normalize units, classify types | `TraceCluster` (tasks + workers) |
| **Env Simulation** | `TraceCluster` or random sampling | Step-by-step task scheduling with rewards | (obs, action, reward) rollout tuples |
| **Feature Extraction** | Task object + Worker objects | Vectorize resources, ratios, type encoding | 5-dim task + 9-dim×N worker tensors |
| **PPO Training** | Rollout buffer | GAE → PPO clip loss → gradient descent | `.pt` model checkpoint |
| **AOD Training** | 24h MongoDB history | Linear regression + direct computation | `GAParams` (Theta, Affinity, Penalty) |
| **Production Inference** | Live task + live workers | Feature extraction → model forward pass | Worker ID selection |
| **Online Learning** | Task outcome (status, runtime, SLA) | Reward derivation → replay buffer → PPO update | Updated policy weights |
| **Telemetry** | Worker heartbeats (every 5s) | Store, broadcast via WebSocket, expose via Prometheus | Real-time dashboards & scheduling input |

---

_Last updated: April 2026_
