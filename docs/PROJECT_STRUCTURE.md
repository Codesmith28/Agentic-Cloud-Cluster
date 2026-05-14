# CloudAI Project Structure Guide

This document explains the organization of the codebase as a monolithic system with specialized, focused directories. Each directory has a single, well-defined responsibility and does not venture beyond its scope.

---

## Monolithic Architecture Overview

The CloudAI project is organized as a **monolith with specialized subsystems**:

- **Master:** Central orchestration node (Go) — manages cluster, tasks, scheduling, persistence
- **Worker:** Distributed execution node (Go) — executes tasks in Docker, reports status
- **Scheduler:** AI-driven scheduling policy (Python) — runs as optional gRPC service
- **Protocols:** Shared gRPC definitions — enables cross-component communication
- **Infrastructure:** Testing, UI, database — supporting systems
- **Documentation:** Project guides, API reference, troubleshooting

Each subsystem is cleanly separated. Changes to one subsystem should have minimal impact on others.

---

## Top-Level Directory Structure

```
CloudAI/
├── master/               # Master control plane (Go monolith)
├── worker/               # Worker execution plane (Go monolith)
├── agentic_scheduler/    # AI scheduler (Python monolith)
├── proto/                # gRPC service definitions
├── testbench/            # Integration testing infrastructure
├── ui/                   # Web dashboard (React)
├── database/             # Database configuration
├── docs/                 # Documentation
├── scripts/              # Utility scripts
├── Makefile              # Central build orchestration
└── Configuration files   # .env, .gitignore, etc.
```

---

## Master Node (`/master/`)

### Purpose
Central orchestration and control plane for the entire cluster. Manages workers, schedules tasks, persists state, provides APIs, and serves the web dashboard.

### Monolithic Structure
```
master/
├── main.go                    # Entry point, service initialization
├── main_test.go               # Main integration tests
├── go.mod, go.sum             # Go module dependencies
├── .env.example               # Environment variable template
├── proto/                     # Symlink to generated gRPC code
├── config/                    # Configuration files
│   └── ga_output.json         # Genetic algorithm parameters (RTS)
└── internal/                  # Private implementation packages
    ├── cli/                   # RESPONSIBILITY: Interactive CLI
    │   ├── commands.go        # Command definitions & dispatch
    │   ├── worker_commands.go # Worker management (register, list, etc.)
    │   ├── task_commands.go   # Task operations (submit, monitor, cancel)
    │   ├── queue_commands.go  # Queue inspection
    │   └── *_test.go          # Tests for each command group
    │
    ├── server/                # RESPONSIBILITY: gRPC & HTTP servers
    │   ├── master_server.go   # Main server initialization (2,823 lines)
    │   ├── worker_handlers.go # Worker registration & heartbeat RPCs
    │   ├── task_handlers.go   # Task assignment & result collection RPCs
    │   ├── file_handlers.go   # File upload/download RPCs
    │   └── *_test.go          # Server tests
    │
    ├── http/                  # RESPONSIBILITY: HTTP REST API
    │   ├── handlers.go        # HTTP endpoint handlers
    │   ├── middleware.go      # Auth, CORS, logging middleware
    │   ├── websocket.go       # WebSocket telemetry streaming
    │   └── *_test.go          # API tests
    │
    ├── telemetry/             # RESPONSIBILITY: Real-time monitoring
    │   ├── aggregator.go      # Collect worker/task metrics
    │   ├── broadcaster.go     # Broadcast to WebSocket clients
    │   └── *_test.go          # Tests
    │
    ├── scheduler/             # RESPONSIBILITY: Task scheduling policy
    │   ├── scheduler.go       # Scheduler interface & registry
    │   ├── round_robin.go     # Round-robin implementation
    │   ├── rts_scheduler.go   # Risk-aware task scheduling
    │   ├── ppo_scheduler.go   # PPO RL-based scheduler
    │   └── *_test.go          # Scheduler tests
    │
    ├── aod/                   # RESPONSIBILITY: Adaptive optimization
    │   ├── optimizer.go       # Genetic algorithm for RTS parameters
    │   ├── fitness.go         # Fitness evaluation
    │   └── *_test.go          # Tests
    │
    ├── controlplane/          # RESPONSIBILITY: Task orchestration
    │   ├── executor.go        # Main orchestration logic (1,524 lines)
    │   ├── queue.go           # Task queue management
    │   ├── attempt.go         # Execution attempt tracking
    │   ├── recovery.go        # Failure recovery & retry logic
    │   └── *_test.go          # Tests
    │
    ├── benchmark/             # RESPONSIBILITY: Benchmark execution
    │   ├── benchmark.go       # Main benchmark logic (1,356 lines)
    │   ├── runner.go          # Benchmark run orchestration
    │   ├── reporter.go        # Result collection & reporting
    │   └── *_test.go          # Tests
    │
    ├── db/                    # RESPONSIBILITY: MongoDB persistence
    │   ├── init.go            # Database initialization & schema
    │   ├── tasks.go           # Task collection operations
    │   ├── workers.go         # Worker registry persistence
    │   ├── assignments.go     # Assignment tracking
    │   ├── attempts.go        # Execution attempt storage
    │   ├── results.go         # Task result persistence
    │   ├── file_metadata.go   # File tracking
    │   ├── scheduler_models.go# Scheduler checkpoint storage
    │   └── *_test.go          # Database tests
    │
    ├── storage/               # RESPONSIBILITY: File storage & access
    │   ├── file_storage.go    # File storage implementation
    │   ├── access_control.go  # Permission checking
    │   └── *_test.go          # Storage tests
    │
    ├── config/                # RESPONSIBILITY: Configuration
    │   ├── config.go          # Config structure & defaults
    │   ├── loader.go          # Environment variable loading
    │   ├── validator.go       # Config validation
    │   └── *_test.go          # Config tests
    │
    ├── metrics/               # RESPONSIBILITY: Prometheus metrics
    │   ├── metrics.go         # Metric definitions & registry
    │   └── *_test.go          # Tests
    │
    ├── system/                # RESPONSIBILITY: Utility functions
    │   ├── logger.go          # Structured logging
    │   ├── errors.go          # Error types & helpers
    │   └── *_test.go          # Tests
    │
    ├── tui/                   # RESPONSIBILITY: Terminal UI (optional)
    │   ├── screen.go          # TUI rendering
    │   └── *_test.go          # Tests
    │
    └── testworkflow/          # RESPONSIBILITY: Test workflow execution
        ├── runner.go          # Test suite runner
        ├── reporter.go        # Test result reporting
        └── *_test.go          # Tests
```

### Key Design Principles for Master

1. **CLI & Server are Separate**
   - `cli/` handles interactive user commands
   - `server/` handles gRPC/HTTP for programmatic access
   - Both can run in the same process or separately (headless mode)

2. **Scheduler is Pluggable**
   - `scheduler/` defines a `Scheduler` interface
   - Multiple implementations (RR, RTS, PPO) can coexist
   - Scheduler selection via `SCHED_ALGO` env var

3. **Persistence is Abstracted**
   - `db/` handles all MongoDB operations
   - Other packages call `db.GetTask()`, not direct MongoDB queries
   - Easy to swap database backend if needed

4. **Task Lifecycle is Centralized**
   - `controlplane/` orchestrates: assign → execute → monitor → complete → store
   - Other packages call `controlplane` APIs, not direct state manipulation

---

## Worker Node (`/worker/`)

### Purpose
Executes tasks on behalf of the master. Manages Docker containers, reports heartbeats, streams logs, and uploads results/artifacts.

### Monolithic Structure
```
worker/
├── main.go                    # Entry point, service initialization
├── go.mod, go.sum             # Go module dependencies
├── proto/                     # Symlink to generated gRPC code
└── internal/                  # Private implementation packages
    ├── server/                # RESPONSIBILITY: gRPC server
    │   ├── worker_server.go   # Main server listening for master RPCs
    │   ├── task_handler.go    # Handle task assignment
    │   ├── cancel_handler.go  # Handle task cancellation
    │   ├── file_handler.go    # Handle file uploads
    │   └── *_test.go          # Tests
    │
    ├── executor/              # RESPONSIBILITY: Docker task execution
    │   ├── executor.go        # Main executor (Docker API calls)
    │   ├── container.go       # Container lifecycle management
    │   ├── resource_config.go # Resource limit configuration
    │   ├── output_handler.go  # Capture & bind-mount output
    │   └── *_test.go          # Tests
    │
    ├── telemetry/             # RESPONSIBILITY: Heartbeat & monitoring
    │   ├── heartbeat.go       # Periodic status reporting to master
    │   ├── resource_monitor.go# Real-time resource usage
    │   └── *_test.go          # Tests
    │
    ├── logstream/             # RESPONSIBILITY: Log streaming
    │   ├── log_manager.go     # Log collection from containers
    │   ├── log_broadcaster.go # Stream logs to master
    │   └── *_test.go          # Tests
    │
    ├── metrics/               # RESPONSIBILITY: Prometheus metrics
    │   ├── metrics.go         # Metric definitions & registry
    │   └── *_test.go          # Tests
    │
    └── system/                # RESPONSIBILITY: System utilities
        ├── system.go          # System info collection
        ├── runtime_config.go  # Environment variable parsing
        ├── worker_identity.go # Persistent worker ID
        ├── memory_linux.go    # Linux memory detection
        ├── memory_nonlinux.go # macOS/Windows memory detection
        ├── port_allocation.go # Find available ports
        └── *_test.go          # Tests
```

### Key Design Principles for Worker

1. **Single Responsibility per Package**
   - `executor/` only executes tasks; doesn't handle networking
   - `server/` only handles RPCs; doesn't execute tasks
   - `telemetry/` only reports status; doesn't make decisions

2. **Stateless Task Execution**
   - No persistent state about tasks (worker restarts don't lose task assignments)
   - All state is in master
   - Worker can be restarted without recovery logic

3. **Graceful Degradation**
   - Worker starts before master registration (no crash)
   - Heartbeat failures don't stop task execution
   - Missing resources default to safe fallbacks

---

## Agentic Scheduler (`/agentic_scheduler/`)

### Purpose
Provides an AI-driven (PPO) scheduling policy as an optional gRPC microservice. Can be deployed separately or co-located with master.

### Monolithic Structure
```
agentic_scheduler/
├── __init__.py                # Package initialization
├── __main__.py                # Entry point (python -m agentic_scheduler)
├── server.py                  # gRPC server (port 50050)
├── service.py                 # PPO service core (policy lifecycle)
├── model.py                   # PPO neural network architecture
├── features.py                # Feature encoding (task/worker → tensors)
├── persistence.py             # MongoDB checkpoint storage
├── train_ppo.py               # Offline training pipeline (1,200+ lines)
│
├── training/                  # RESPONSIBILITY: Training utilities
│   ├── trace_loader.py        # Load Alibaba trace CSV
│   ├── alibaba_loader.py      # Alibaba-specific trace parsing
│   └── (other loaders)
│
├── scripts/                   # RESPONSIBILITY: Utility scripts
│   ├── preprocess_traces.py   # Prepare training data
│   └── (other scripts)
│
├── docs/                      # RESPONSIBILITY: PPO documentation
│   ├── TRAINING_ARCHITECTURE.md
│   └── TRAINING_DECISIONS.md
│
├── requirements.txt           # Python dependencies
├── setup.py                   # Package configuration
└── __pycache__/ (generated)
```

### Key Design Principles for Agentic Scheduler

1. **Model & Training Separated**
   - `model.py` is pure ML (neural network definitions)
   - `train_ppo.py` is training logic (doesn't know about gRPC)
   - `service.py` is runtime (loads model, serves decisions)

2. **Persistence is Optional**
   - Model can be trained offline and deployed
   - Checkpoint loading is graceful (defaults to random if no checkpoint found)
   - Online learning persists to both local disk and MongoDB

3. **Inference is Deterministic**
   - `service.py` caches model state
   - Multiple calls with same fingerprint return same policy
   - Allows safe online adaptation without breaking master assumptions

---

## Protocol Buffers (`/proto/`)

### Purpose
Defines all gRPC service interfaces and message types used for communication between master, worker, and scheduler.

### Structure
```
proto/
├── generate.sh                # Script to regenerate code
├── README.md                  # Proto documentation
├── master_worker.proto        # Master ↔ Worker RPCs
├── master_agent.proto         # Master ↔ Agent RPCs (optional)
├── ppo_scheduler.proto        # Master ↔ PPO Scheduler RPCs
├── pb/                        # Generated Go code
│   ├── master_worker_pb.go
│   ├── master_worker_grpc.pb.go
│   └── (other *.pb.go files)
└── py/                        # Generated Python code
    ├── master_worker_pb2.py
    ├── master_worker_pb2_grpc.py
    └── (other *_pb2.py files)
```

### Key Design Principles for Proto

1. **No Business Logic in Protos**
   - Protos only define interfaces & message types
   - Business logic stays in Go/Python code

2. **Versioning & Backward Compatibility**
   - Only add fields; never remove (gRPC allows forward/backward compat)
   - Use field numbers carefully

3. **Clear Domain Boundaries**
   - `master_worker.proto` only for master ↔ worker communication
   - `ppo_scheduler.proto` only for master ↔ scheduler communication
   - Makes it clear what services interact

---

## Testbench (`/testbench/`)

### Purpose
Integration testing infrastructure for validating the full system under various load scenarios.

### Structure
```
testbench/
├── README.md                  # Testbench documentation
├── docker-compose.yml         # Full Docker stack (master+workers)
├── docker-compose.host-master.yml # Host-master topology
├── scripts/                   # RESPONSIBILITY: Test orchestration
│   ├── run_integration.sh     # Full integration pipeline
│   ├── run_suite.sh           # Run single test suite (smoke/reliability/etc.)
│   ├── run_campaign.py        # Run benchmark campaign
│   ├── run_workload.py        # Submit workload to cluster
│   ├── register_workers.sh    # Register Docker workers
│   ├── prepare_workflow_images.sh # Build test images
│   ├── shared_polling.py      # Shared polling utilities
│   └── (other scripts)
│
├── docker/                    # RESPONSIBILITY: Docker images
│   ├── Dockerfile.master      # Master image
│   ├── Dockerfile.worker      # Worker image
│   └── (other Dockerfiles)
│
└── observability/             # RESPONSIBILITY: Monitoring config
    ├── prometheus/
    │   ├── prometheus.yml
    │   └── prometheus.host-master.yml
    ├── grafana/
    │   └── dashboards/
    └── (other observability configs)
```

### Key Design Principles for Testbench

1. **Tests Are Reproducible**
   - Docker ensures consistent environment
   - Scripts are idempotent (can run multiple times)

2. **Topology Options**
   - Full stack: master in Docker
   - Host-master: master on host, workers in Docker (for model updates)
   - Each mode defined in separate docker-compose file

3. **Campaign Scripts**
   - `run_campaign.py` is the single entry point for benchmarks
   - Supports multiple schedulers, workloads, scenarios
   - Results are timestamped and comparable

---

## UI (`/ui/`)

### Purpose
Web-based dashboard for monitoring cluster status, task progress, and worker health.

### Structure
```
ui/
├── package.json               # Node.js dependencies & build scripts
├── package-lock.json          # Dependency lock file
├── vite.config.js             # Vite build configuration
├── index.html                 # HTML entry point
├── src/                       # RESPONSIBILITY: React components
│   ├── App.jsx                # Main app component
│   ├── pages/                 # Page components
│   ├── components/            # Reusable components
│   ├── services/              # API client
│   ├── hooks/                 # React hooks
│   ├── styles/                # CSS/styling
│   └── main.jsx               # React entry point
│
├── public/                    # RESPONSIBILITY: Static assets
│   └── (images, favicon, etc.)
│
└── dist/ (generated)          # Built distribution
```

### Key Design Principles for UI

1. **Separate from Backend**
   - UI is a React SPA (Single Page Application)
   - Communicates with master via REST/WebSocket APIs
   - Can be deployed independently

2. **Real-time Updates**
   - Uses WebSocket for live telemetry
   - Falls back to polling if WebSocket unavailable

---

## Database (`/database/`)

### Purpose
Configures and manages MongoDB persistence layer.

### Structure
```
database/
├── docker-compose.yml         # MongoDB container definition
└── README.md                  # Setup instructions
```

### Key Design Principles

1. **Externalized Configuration**
   - Database runs in Docker (easily swappable)
   - Master connects via configurable URI

2. **Schema Initialization**
   - `master/internal/db/init.go` creates collections on startup
   - No manual schema setup needed

---

## Scripts (`/scripts/`)

### Purpose
Utility scripts for model management, benchmarking, and dependency installation.

### Structure
```
scripts/
├── model_promote.sh           # Promote trained model to active
├── generate_benchmark_report.py # Create benchmark report
├── install_deps.sh            # Install system dependencies
└── (other utilities)
```

### Key Design Principles

1. **Idempotent**
   - Scripts can be run multiple times safely
   - Check before overwriting files

2. **Self-Documented**
   - Each script has `--help` option
   - Usage examples in comments

---

## Documentation (`/docs/`)

### Purpose
Comprehensive guides for developers, users, and operators.

### Structure
```
docs/
├── README.md                  # → See ../README.md
├── GETTING_STARTED.md         # 5-minute setup guide
├── DOCUMENTATION.md           # Complete API reference
├── ARCHITECTURE.md            # → See ../ARCHITECTURE.md
├── TESTBENCH_RUNBOOK.md       # Testing procedures
├── USER_MANUAL.md             # CLI command reference
├── PROJECT_STRUCTURE.md       # This file's sister (detailed)
├── PROJECT_APPENDIX.md        # Scripts & file locations
├── BENCHMARK_RESULTS.md       # Historical benchmark data
├── PPO_OPTIMIZATION_CONSOLIDATED_REPORT_2026-04-11.md
└── (other documentation)
```

### Key Design Principles

1. **Hierarchical Documentation**
   - README: Overview
   - GETTING_STARTED: Quick walkthrough
   - DOCUMENTATION: Complete reference
   - PROJECT_STRUCTURE: Code organization
   - PROJECT_APPENDIX: Scripts & utilities

2. **Searchable & Maintainable**
   - Each doc has a clear purpose
   - Cross-references link related docs
   - Updated alongside code changes

---

## Build System (`Makefile`)

### Purpose
Centralized orchestration of all build, test, and deployment operations.

### Key Targets
```makefile
make setup          # One-time: install deps, generate proto, create symlinks
make build          # Compile master + worker
make test-unit      # Run Go unit tests
make run-master     # Start master (RTS scheduler)
make run-master-ppo # Start master (PPO scheduler)
make run-worker     # Start worker
make testbench-up   # Start full Docker testbench
make campaign       # Run benchmark campaign
```

### Key Design Principles

1. **Single Entry Point**
   - All build operations go through Make
   - Reduces learning curve for new contributors

2. **Composable Targets**
   - Targets can be combined (`make setup build test-unit`)
   - Enables CI/CD integration

---

## Root-Level Files

| File | Purpose |
|------|---------|
| `README.md` | Project overview & quick start |
| `ARCHITECTURE.md` | System design & component interaction |
| `Makefile` | Central build orchestration |
| `requirements.txt` | Python dependencies (PPO module) |
| `runMaster.sh` | Master launch script (alternative to Make) |
| `runWorker.sh` | Worker launch script (alternative to Make) |
| `execute-tests.sh` | E2E test runner |
| `.gitignore` | Git ignore patterns |
| `.dockerignore` | Docker ignore patterns |
| `.github/` | GitHub Actions CI/CD workflows |

---

## Design Philosophy Summary

1. **Monolithic but Modular**
   - Single codebase, organized by subsystem
   - Each subsystem is self-contained

2. **Clear Boundaries**
   - Master, Worker, Scheduler are distinct
   - Communication via well-defined protocols (gRPC)
   - Easy to test & deploy independently if needed

3. **Separation of Concerns**
   - CLI ≠ Server ≠ Business Logic
   - Persistence ≠ Domain Logic
   - Testing ≠ Production

4. **Specialized Directories**
   - Each directory has ONE responsibility
   - No "util" packages or god-objects
   - Easy to navigate & contribute

5. **Documentation-Driven**
   - Code is self-documenting
   - README in each major directory
   - Makefile shows common workflows

---

## Adding New Features

### Adding a New Scheduler
1. Create `/master/internal/scheduler/your_scheduler.go`
2. Implement `Scheduler` interface (see `scheduler.go`)
3. Register in `main.go`
4. Add tests in `your_scheduler_test.go`
5. Update `docs/DOCUMENTATION.md`

### Adding a New CLI Command
1. Create `/master/internal/cli/your_command.go`
2. Implement command handler
3. Register in `cli/commands.go`
4. Add tests in `your_command_test.go`
5. Update `docs/USER_MANUAL.md`

### Adding a New gRPC Service
1. Define in `/proto/your_service.proto`
2. Run `proto/generate.sh`
3. Implement service in appropriate package
4. Add tests
5. Update `docs/DOCUMENTATION.md`

---

**Last Updated:** 2026-05-13  
**Version:** 2.0 (Post-Cleanup Reorganization)
