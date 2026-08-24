# Agentic Cloud Cluster — Developer Guide & Architecture Overview

Welcome to the **Agentic Cloud Cluster** codebase. This guide serves as the primary manual for developers and contributors to navigate and extend the cluster.

---

## 1. System Architecture

The project is structured according to **Clean Architecture** and **Domain-Driven Design (DDD)** principles:

```
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
├── Makefile                         # Unified make build & orchestration targets
├── pkg/                             # Shared Foundation Module
│   ├── domain/                      # Pure Business Entities (Task, Worker, Assignment, Attempt, Result)
│   ├── ports/                       # Interface Contracts (Scheduler, Repositories, TelemetrySink)
│   ├── constants/                   # Centralized string constants (env keys, defaults, collections, metrics)
│   └── envutil/                     # Type-safe environment helpers
├── master/                          # Master Node Service
│   ├── main.go                      # Minimal binary entrypoint
│   └── internal/
│       ├── app/                     # Application lifecycle & bootstrap orchestration
│       ├── config/                  # Master configuration struct & env loaders
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
│       ├── config/                  # Worker configuration struct & env loaders
│       ├── server/                  # gRPC server (MasterWorkerServer implementation)
│       ├── executor/                # Docker task execution engine with resource limits
│       ├── telemetry/               # Heartbeat transmitter & system load monitor
│       ├── logstream/               # Multiplexed live log broadcaster
│       └── system/                  # Hardware resource probing & persistent worker identity
├── agentic_scheduler/               # Python Reinforcement Learning Policy
│   ├── __main__.py                  # PPO gRPC service daemon
│   ├── constants.py                 # Feature dimensions, gRPC ports, model defaults
│   ├── model.py                     # PyTorch Actor-Critic neural network architecture
│   ├── features.py                  # Feature encoding (task dim 5, worker dim 9)
│   ├── service.py                   # PPO core inference & online replay buffer
│   ├── persistence.py               # MongoDB GridFS model checkpoint persistence
│   ├── train_ppo.py                 # Offline and trace replay training runner
│   └── tests/                       # Unit test suite
├── proto/                           # Protocol Buffer definitions (master_worker.proto, ppo_scheduler.proto)
├── scripts/                         # Organized Automation & Run Scripts
│   ├── _common.sh                   # Shared shell helpers & color formatters
│   ├── master/run.sh                # Master node launch script
│   ├── worker/run.sh                # Worker node launch script
│   ├── cluster/reset.sh             # Full cluster state wipe & reset
│   ├── testing/execute_tests.sh     # Host-master campaign runner
│   ├── testing/run_ppo_test.sh      # Clean-slate PPO benchmark runner
│   └── tools/                       # Dependencies, model promotion & reporting tools
├── docs/                            # Documentation
│   ├── CODEBASE_GUIDE.md            # This guide
│   ├── DOCUMENTATION.md             # Complete system reference
│   ├── USER_MANUAL.md               # User operational manual
│   ├── diagrams/                    # System diagrams and sequence flows
│   └── academic/                    # Academic papers, presentations & reports
├── ui/                              # Web UI React Dashboard
└── testbench/                       # Multi-worker simulation and validation testbench
```

---

## 3. Verification & Testing

```bash
# Run all Go unit tests
make test-unit

# Run Python scheduler tests
make test-python

# Compile check & vet
make check && make vet

# Build master & worker binaries
make build
```
