# Agentic Cloud Cluster — System Architecture

**Last Updated:** August 2026

---

## 1. High-Level Architecture

Agentic Cloud Cluster follows a decoupled, distributed master-worker architecture built upon Clean Architecture and Domain-Driven Design (DDD) principles:

- **User Interfaces**: Interactive CLI, BubbleTea TUI, React Web Dashboard, REST/WebSocket APIs.
- **Shared Foundation (`pkg/`)**: Zero-dependency Go module containing pure domain models (`domain/`), ports/interfaces (`ports/`), and runtime helpers (`envutil/`).
- **Master Node (`master/`)**: High-throughput Go coordinator handling scheduling, unified MongoDB connection pooling (`MongoStore`), task lifecycle, telemetry aggregation, and benchmarking.
- **Worker Node (`worker/`)**: Distributed execution nodes providing Docker-isolated container execution, telemetry monitors, and live log broadcasting.
- **Python RL Policy (`agentic_scheduler/`)**: PPO (Proximal Policy Optimization) reinforcement learning engine communicating with Master via gRPC for intelligent task placement.
- **MongoDB**: Central persistent storage for cluster state, attempts, results, user RBAC, and model checkpoints.

```text
                                 ┌───────────────────────────┐
                                 │   UI / Web Dashboard      │
                                 │   (React + WebSocket)     │
                                 └─────────────┬─────────────┘
                                               │ HTTP / WS
                                               ▼
    ┌────────────────────────────────────────────────────────────────────────┐
    │                              MASTER NODE                               │
    │  ┌──────────────────────┐  ┌────────────────────────────────────────┐  │
    │  │     Controlplane     │  │          master/internal/server        │  │
    │  │  (Modular Commands)  │  │ (gRPC, Worker/Task/Queue Managers)     │  │
    │  └──────────┬───────────┘  └───────────────────┬────────────────────┘  │
    │             │                                  │                       │
    │             ▼                                  ▼                       │
    │  ┌──────────────────────┐  ┌────────────────────────────────────────┐  │
    │  │      MongoStore      │  │           Scheduler Engine             │  │
    │  │ (Unified Connection) │  │       (RR / RTS / Python PPO)          │  │
    │  └──────────────────────┘  └───────────────────┬────────────────────┘  │
    └────────────────────────────────────────────────┼───────────────────────┘
                                                     │ gRPC
                                                     ▼
    ┌────────────────────────────────────────────────────────────────────────┐
    │                              WORKER NODE                               │
    │  ┌──────────────────────┐  ┌────────────────────────────────────────┐  │
    │  │    Executor Engine   │  │           Telemetry Monitor            │  │
    │  │  (Docker Isolation)  │  │        (Heartbeats & Metrics)          │  │
    │  └──────────────────────┘  └────────────────────────────────────────┘  │
    └────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Directory Layout & Module Structure

```text
Agentic-Cloud-Cluster/
├── go.work                          # Go multi-module workspace
├── pkg/                             # Shared Foundation Module
│   ├── domain/                      # Pure Business Entities (Task, Worker, Assignment, Attempt, Result)
│   ├── ports/                       # Interface Contracts (Scheduler, Repositories, TelemetrySink)
│   └── envutil/                     # Type-safe environment helpers
├── master/                          # Master Node Service
│   ├── main.go                      # Minimal binary entrypoint
│   └── internal/
│       ├── app/                     # Application lifecycle & bootstrap orchestration
│       ├── db/                      # MongoStore connection pool & collection repositories
│       ├── server/                  # gRPC handlers, WorkerManager, TaskManager, QueueProcessor
│       ├── controlplane/            # Modular CLI command executors (cluster, task, file, benchmark)
│       ├── scheduler/               # Schedulers (Round-Robin, RTS, PPO client)
│       ├── benchmark/               # Benchmark simulation engine, profiles & reporting
│       ├── storage/                 # Secure file storage & RBAC
│       ├── telemetry/               # Telemetry monitor & WebSocket streaming
│       ├── http/                    # REST API & static dashboard server
│       ├── cli/                     # Interactive Readline CLI
│       └── tui/                     # BubbleTea Terminal Dashboard
├── worker/                          # Worker Node Service
│   ├── main.go                      # Minimal binary entrypoint
│   └── internal/
│       ├── app/                     # Worker application lifecycle & shutdown hooks
│       ├── server/                  # gRPC server (MasterWorkerServer implementation)
│       ├── executor/                # Docker task execution engine with resource limits
│       ├── telemetry/               # Heartbeat transmitter & system load monitor
│       ├── logstream/               # Multiplexed live log broadcaster
│       └── system/                  # Hardware resource probing & persistent worker identity
├── agentic_scheduler/               # Python Reinforcement Learning Policy
│   ├── __main__.py                  # PPO gRPC service daemon
│   ├── model.py                     # PyTorch Actor-Critic neural network architecture
│   ├── features.py                  # Feature encoding (task dim 5, worker dim 9)
│   ├── service.py                   # PPO core inference & online replay buffer
│   ├── persistence.py               # MongoDB GridFS model checkpoint persistence
│   ├── train_ppo.py                 # Offline and trace replay training runner
│   └── tests/                       # Unit test suite
├── proto/                           # Protocol Buffer definitions (master_worker.proto)
├── ui/                              # Web UI React Dashboard
└── testbench/                       # Multi-worker simulation and validation testbench
```

---

## 3. Communication Protocols

| Interface | Protocol | Port | Description |
| :--- | :--- | :--- | :--- |
| **Master gRPC** | HTTP/2 (Protobuf) | `50051` | Worker registration, task assignment, heartbeat ingestion, log streaming |
| **Worker gRPC** | HTTP/2 (Protobuf) | `50052+` | Worker listener for `MasterRegister`, `AssignTask`, `CancelTask`, `StreamLogs` |
| **PPO Scheduler gRPC** | HTTP/2 (Protobuf) | `50050` | Master ↔ Python PPO decision exchange and outcome reporting |
| **Master HTTP & WS** | HTTP/1.1 & WS | `8080` | REST API for tasks, workers, file storage, and WebSocket `/ws/telemetry` |
| **Worker Metrics** | HTTP (Prometheus) | `9101+` | Prometheus `/metrics` endpoint on each worker |
| **Database** | MongoDB Wire Protocol | `27017` | Persistent data storage |

---

## 4. Key Subsystems

### Unified MongoStore (`master/internal/db/mongo.go`)
- Thread-safe singleton client managing MongoDB connection pools.
- Pre-configured minimum pool (5) and maximum pool (50) connections.
- Automatically initializes indexes across `TASKS`, `WORKERS`, `ASSIGNMENTS`, `ATTEMPTS`, `RESULTS`, `USERS`, and `SCHEDULER_MODELS`.

### Master Queue Processor (`master/internal/server/queue_processor.go`)
- Background scheduling loop that periodically drains queued tasks.
- Preserves FIFO submission order while applying placement algorithms (RTS or PPO).
- Coordinates with `TaskManager` for atomic worker reservations.

### Worker Docker Runtime (`worker/internal/executor/executor.go`)
- Allocates CPU cores using Docker `NanoCPUs` and memory quotas using `Memory`.
- Enforces container PID limits to prevent resource starvation.
- Mounts host output directories to `/output` and streams files back to Master upon completion.
