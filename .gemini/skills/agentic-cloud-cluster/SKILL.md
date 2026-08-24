---
name: agentic-cloud-cluster
description: "Comprehensive guide and developer skill for navigating, understanding, and extending the Agentic Cloud Cluster codebase."
---

# Agentic Cloud Cluster — Codebase Skill & Developer Guide

This guide provides an end-to-end overview of the **Agentic Cloud Cluster** codebase, its architectural principles, component responsibilities, protocols, and workflows.

---

## 1. System Overview & Clean Architecture

Agentic Cloud Cluster is a distributed container orchestration platform featuring:
- **Clean Architecture & Domain-Driven Design (DDD)**
- **Decoupled Go Framework & Python PPO Reinforcement Learning Model**
- **Unified MongoDB Connection Pooling (`MongoStore`)**
- **Docker-backed isolated Worker Execution Engine**
- **Real-time Telemetry, WebSocket streaming, and Web UI Dashboard**

```
Agentic-Cloud-Cluster/
├── pkg/                     # Shared Go foundation (Zero dependency on master/worker)
│   ├── domain/              # Pure domain entities (Task, Worker, Assignment, etc.)
│   ├── ports/               # Domain interfaces (Scheduler, Repositories, Reporter)
│   ├── constants/           # Centralized string constants (env keys, defaults, collections, metrics)
│   └── envutil/             # Centralized environment variable utilities
├── master/                  # Master Node Coordinator
│   ├── main.go              # Minimal entrypoint (~15 lines)
│   └── internal/
│       ├── app/             # Application lifecycle, bootstrap & signal handling
│       ├── config/          # Master configuration struct & env loaders
│       ├── controlplane/    # Modular CLI & admin command handlers (cluster, task, file, bench)
│       ├── db/              # MongoStore & collection-specific repositories
│       ├── scheduler/       # Round-Robin, RTS, & PPO Scheduler implementations
│       ├── server/          # gRPC Handlers, Worker Manager, Task Manager, Queue Processor
│       ├── benchmark/       # Benchmarking engine, profiles, simulation, and reports
│       ├── storage/         # Secure artifact file storage service & RBAC
│       ├── telemetry/       # Live telemetry monitor & WebSocket streaming
│       ├── http/            # REST API, JWT auth, Web UI static server
│       ├── cli/             # Interactive Readline CLI
│       └── tui/             # BubbleTea terminal dashboard
├── worker/                  # Worker Node Executor
│   ├── main.go              # Minimal entrypoint (~10 lines)
│   └── internal/
│       ├── app/             # Worker application lifecycle & bootstrap
│       ├── config/          # Worker configuration struct & env loaders
│       ├── server/          # gRPC server implementing MasterWorkerServer
│       ├── executor/        # Docker container runtime & task runner
│       ├── telemetry/       # Heartbeat reporter & status monitor
│       ├── logstream/       # Broadcaster for multiplexed live container logs
│       └── system/          # Hardware resource detection & persistent identity
├── agentic_scheduler/       # Python Reinforcement Learning Policy (PPO)
│   ├── __main__.py          # PPO gRPC daemon entrypoint
│   ├── constants.py         # Feature dimensions, gRPC ports, model defaults
│   ├── model.py             # PyTorch Actor-Critic neural network architecture
│   ├── features.py          # State/action feature vector transformations
│   ├── service.py           # Core PPO scheduling service with replay buffer
│   ├── persistence.py       # MongoDB GridFS model checkpoint persistence
│   ├── train_ppo.py         # Offline & trace replay training script
│   └── tests/               # Unit and regression test suite
├── proto/                   # gRPC Protocol Buffer definitions (master_worker.proto)
├── ui/                      # React & Tailwind Web UI Dashboard
└── docs/                    # Architectural documents & guides
```

---

## 2. Key Domain Concepts (`pkg/`)

- **`pkg/domain`**: Contains core system entities decoupled from databases and transport protocols:
  - `Task`: Task request specification (image, command, requested resources, SLA multiplier).
  - `Worker`: Worker state, endpoint, capacity, resource utilization, and status.
  - `Assignment`: Task-to-Worker assignment metadata.
  - `TaskAttempt`: Lifecycle record of each retry attempt.
  - `TaskResult`: Execution outcome, status, logs, and output file artifacts.

- **`pkg/ports`**: Interface contracts defining boundaries:
  - `Scheduler`: `SelectWorker(ctx, task, workers) (*domain.Worker, error)`
  - `TaskRepository`, `WorkerRepository`, `AssignmentRepository`: Data persistence boundaries.
  - `OutcomeReporter`: Notifying the scheduler of execution outcomes for online learning.

---

## 3. Master Node Subsystems (`master/`)

- **`master/internal/app`**: Encapsulates application startup, configuration validation, graceful shutdown, and signal orchestration.
- **`master/internal/db/mongo.go`**: Provides `MongoStore`, a single thread-safe connection pool with pre-configured connection sizing (min pool 5, max pool 50).
- **`master/internal/server`**:
  - `grpc_handlers.go`: Implements gRPC endpoints (`RegisterWorker`, `SendHeartbeat`, `SubmitTask`, `StreamTaskLogs`, etc.).
  - `worker_manager.go`: Handles worker registration, address updates, heartbeat tracking, and health reconcilers.
  - `task_manager.go`: Manages task lifecycle, assignments, output file ingestion, and terminal status recording.
  - `queue_processor.go`: FIFO queue scheduler loop with automatic retry and capacity backoff.
- **`master/internal/controlplane`**: Split into focused command handlers:
  - `cmd_cluster.go`: Worker registration, status display, resource stats.
  - `cmd_task.go`: Task submission, queue inspection, task cancellation, log viewing.
  - `cmd_file.go`: Secure file management and listing.
  - `cmd_benchmark.go`: Benchmark suite execution.

---

## 4. Worker Node Subsystems (`worker/`)

- **`worker/internal/app`**: Worker node bootstrap, port detection, persistent worker ID resolution (`AGENTIC_WORKER_STATE_DIR`), and signal handling.
- **`worker/internal/server`**: Handles `MasterRegister`, `AssignTask`, `CancelTask`, and `StreamTaskLogs`.
- **`worker/internal/executor`**:
  - Pulls Docker images with retry.
  - Configures CPU quotas (`NanoCPUs`) and memory limits.
  - Bind-mounts `/output` for task artifacts.
  - Enforces PID limits to prevent fork bombs.
- **`worker/internal/logstream`**: Broadcaster pattern allowing concurrent log consumers without disk I/O bottlenecks.

---

## 5. Python RL Scheduler (`agentic_scheduler/`)

- **PPO Neural Network**: `PPOActorCritic` model taking task features (dim 5) and worker candidate features (dim 9).
- **Masked Policy Gradient**: Uses action masking (`-1e4`) for infeasible workers (insufficient CPU/memory).
- **Online Learning**: Maintains an experience replay buffer with GAE (Generalized Advantage Estimation) to update model weights live as task completions arrive.
- **Model Checkpointing**: Serializes models to GridFS and tracks the latest working baseline in `agentic_scheduler/models/ppo_latest.pt`.

---

## 6. Build, Test & Run Commands

```bash
# Build entire Go workspace
go build -o master/masterNode ./master
go build -o worker/workerNode ./worker

# Run all Go tests across all modules
(cd pkg && go test ./...)
(cd master && go test ./...)
(cd worker && go test ./...)

# Run Python scheduler tests
python3 -m unittest discover -s agentic_scheduler/tests/ -v

# Start Master Node
./master/masterNode

# Start Worker Node
./worker/workerNode
```
