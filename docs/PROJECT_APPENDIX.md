# CloudAI Project Appendix: Scripts, Files, and Reference Guide

This document serves as a comprehensive reference guide for understanding where components are located, what each script does, and how to find specific functionality.

---

## Quick Reference: Root-Level Scripts & Files

| Script/File | Location | Purpose | When to Use |
|-------------|----------|---------|------------|
| `runMaster.sh` | `/` | Start master node with UI and optional PPO | Development/testing; alternative to `make run-master` |
| `runWorker.sh` | `/` | Start worker node | Development/testing; alternative to `make run-worker` |
| `execute-tests.sh` | `/` | End-to-end deployment with benchmark | Running integrated benchmarks; testing full pipeline |
| `Makefile` | `/` | Central build system | All build, test, and deployment operations |
| `requirements.txt` | `/` | Python dependencies | `pip install -r requirements.txt` for PPO module |
| `README.md` | `/` | Project overview | First read; contains quick start and architecture |
| `ARCHITECTURE.md` | `/` | Detailed system architecture | Understanding system design and data flow |
| `docker-compose.yml` (database/) | `/database/` | MongoDB container definition | Starting persistent database backend |

---

## Master Make Targets

Located in: `Makefile` (lines 1-399)

### Build & Setup
```bash
make setup            # One-time setup: install deps, generate proto, create symlinks
make build            # Compile master + worker binaries
make all              # Full setup + build
make proto            # Generate gRPC code from .proto files
make clean            # Remove binaries, generated code, venv
```

### Quality & Testing
```bash
make check            # Compile-check Go code without creating binaries
make vet              # Run go vet on master + worker
make fmt              # Run gofmt, flag files needing formatting
make test             # Verify toolchain (Go, Docker, protoc, Python)
make test-unit        # Run Go unit tests (master + worker)
make test-unit-verbose # Verbose unit test output
```

### Python & PPO
```bash
make venv             # Create Python virtualenv (if not exists)
make pip-install      # Install Python dependencies into venv
make ppo-server       # Start PPO gRPC server (requires venv)
```

### Database
```bash
make db-up            # Start MongoDB in Docker (port 27017)
make db-down          # Stop MongoDB container
```

### Runtime
```bash
make run-master       # Build + start master (RTS scheduler)
make run-master-ppo   # Build + start master (PPO scheduler)
make run-worker       # Build + start worker
```

### Testbench (Docker-based testing)
```bash
make testbench-up                    # Start full Docker stack
make testbench-host-up               # Start worker-only Docker stack (host-master topology)
make testbench-down                  # Tear down full stack
make testbench-host-down             # Tear down worker-only stack
make testbench-prepare-images        # Build workflow images into worker DinD
make testbench-register              # Register workers (full stack)
make testbench-host-register         # Register workers (host-master topology)
make testbench-workload              # Prepare images + submit workload
make testbench-suite SUITE_NAME=smoke # Run single test suite
make testbench-integration           # Full integration + benchmark pipeline
```

### Campaigns & Benchmarks
```bash
make campaign              # Smoke benchmark (heterogeneous-smoke workload)
make campaign-full         # Full campaign (all workloads + scenarios)
make campaign-final        # Heavy evaluation (50 tasks × 3 scenarios × 3 schedulers)
make campaign-comprehensive # Comprehensive benchmark (4 workloads × 3 scenarios)
```

### Model Management
```bash
make model-promote         # Promote latest trained model, archive old
make model-promote-dry     # Dry-run: preview without changes
make model-archive-list    # List archived model versions
```

### Pipelines
```bash
make deploy             # Build → promote model → benchmark
make benchmark          # Run execute-tests.sh end-to-end
```

---

## Testbench Scripts

Located in: `/testbench/scripts/`

| Script | Purpose | Called By |
|--------|---------|-----------|
| `run_integration.sh` | Execute full integration test suite | `make testbench-integration` |
| `run_suite.sh` | Run a specific test suite (smoke/reliability/ui-smoke/evidence/full) | `make testbench-suite` |
| `run_workload.py` | Submit test workload to cluster | `make testbench-workload` |
| `run_campaign.py` | Execute benchmark campaign with schedulers | `make campaign*` |
| `register_workers.sh` | Register Docker workers with master | testbench flows |
| `prepare_workflow_images.sh` | Build and push workflow images to Docker DinD | Integration tests |

**Usage Example:**
```bash
cd /testbench/scripts
./run_integration.sh                    # Full pipeline
python3 run_campaign.py --scenarios all --workloads heterogeneous-smoke
./register_workers.sh                   # Register workers
```

---

## Scripts Utilities

Located in: `/scripts/`

| Script | Purpose | Arguments | Output |
|--------|---------|-----------|--------|
| `model_promote.sh` | Promote trained model to active, archive previous | `[model_path]`, `--dry-run` | New active model at `agentic_scheduler/models/ppo_latest.pt` |
| `generate_benchmark_report.py` | Create markdown report from campaign results | `--campaign-dir`, `--master-url`, `--model-path` | HTML/markdown report |
| `install_deps.sh` | Install system-level dependencies (protoc, Go, etc.) | (none) | Configured toolchain |

**Example:**
```bash
./scripts/model_promote.sh                           # Auto-detect latest
./scripts/model_promote.sh path/to/checkpoint.pt     # Specific file
./scripts/model_promote.sh --dry-run                 # Preview changes
python3 scripts/generate_benchmark_report.py \
  --campaign-dir results/campaign-20260427-140000 \
  --master-url http://localhost:8080 \
  --model-path agentic_scheduler/models/ppo_latest.pt
```

---

## Master Node (Go)

Located in: `/master/`

### Entry Point
- **File:** `main.go` (lines 1-1000+)
- **Binary:** `master/masterNode` (built by `make master`)
- **Start:** `./runMaster.sh` or `make run-master`

### Key Internal Packages

#### Core Runtime
| Package | Location | Responsibility |
|---------|----------|-----------------|
| `internal/cli` | `/master/internal/cli/` | Interactive CLI command parser & execution |
| `internal/server` | `/master/internal/server/` | gRPC control plane server (port 50051) |
| `internal/http` | `/master/internal/http/` | HTTP API handlers (port 8080) |
| `internal/telemetry` | `/master/internal/telemetry/` | WebSocket telemetry streaming |

#### Task & Worker Management
| Package | Location | Responsibility |
|---------|----------|-----------------|
| `internal/controlplane` | `/master/internal/controlplane/` | Task assignment & orchestration (1,524 lines) |
| `internal/db` | `/master/internal/db/` | MongoDB persistence layer |
| `internal/storage` | `/master/internal/storage/` | File storage & access control |

#### Scheduling & Policy
| Package | Location | Responsibility |
|---------|----------|-----------------|
| `internal/scheduler` | `/master/internal/scheduler/` | Scheduler implementations (RR, RTS, PPO) |
| `internal/aod` | `/master/internal/aod/` | Adaptive optimization & parameter tuning |
| `internal/benchmark` | `/master/internal/benchmark/` | Benchmark execution logic (1,356 lines) |

#### Utilities & Support
| Package | Location | Responsibility |
|---------|----------|-----------------|
| `internal/config` | `/master/internal/config/` | Configuration loading & validation |
| `internal/system` | `/master/internal/system/` | System utilities & helpers |
| `internal/metrics` | `/master/internal/metrics/` | Prometheus metrics definitions |
| `internal/tui` | `/master/internal/tui/` | Terminal UI components |
| `internal/testworkflow` | `/master/internal/testworkflow/` | Test workflow execution engine |

### Configuration Files
- **`master/.env.example`** — Environment variable template
- **`master/config/ga_output.json`** — Genetic algorithm parameters (RTS scheduler)

---

## Worker Node (Go)

Located in: `/worker/`

### Entry Point
- **File:** `main.go`
- **Binary:** `worker/workerNode` (built by `make worker`)
- **Start:** `./runWorker.sh` or `make run-worker`

### Key Internal Packages

| Package | Location | Responsibility |
|---------|----------|-----------------|
| `internal/server` | `/worker/internal/server/` | gRPC server listening for master commands |
| `internal/executor` | `/worker/internal/executor/` | Docker task execution & lifecycle management |
| `internal/telemetry` | `/worker/internal/telemetry/` | Heartbeat telemetry sending |
| `internal/logstream` | `/worker/internal/logstream/` | Live log streaming to master |
| `internal/metrics` | `/worker/internal/metrics/` | Prometheus metrics & health checks |
| `internal/system` | `/worker/internal/system/` | System info detection, port allocation, resource overrides |

### Environment Variables
- `WORKER_ID` — Unique worker identifier (default: auto-generated from hostname)
- `WORKER_PORT` — gRPC port (default: first free port from 50052)
- `WORKER_BIND_IP` — Bind address (default: auto-detected)
- `WORKER_TOTAL_CPU`, `WORKER_TOTAL_MEMORY_GB`, `WORKER_TOTAL_STORAGE_GB` — Resource overrides
- `WORKER_CONTAINER_NETWORK_MODE` — Docker network mode (bridge/host/none, default: bridge)
- `WORKER_METRICS_PORT` — Prometheus metrics port (default: 9101)

---

## Protocol Buffers (gRPC)

Located in: `/proto/`

### Proto Files
| File | Defines |
|------|---------|
| `master_worker.proto` | Master ↔ Worker communication (RegisterWorker, AssignTask, etc.) |
| `master_agent.proto` | Master ↔ Agent communication (optional) |
| `ppo_scheduler.proto` | Master ↔ PPO scheduler communication |

### Generated Code
- **Go:** `proto/pb/*.pb.go`, `*_grpc.pb.go` (generated by `proto/generate.sh`)
- **Python:** `proto/py/*_pb2.py`, `*_pb2_grpc.py` (generated by `proto/generate.sh`)

### Generation Script
```bash
cd proto && ./generate.sh          # Regenerate all proto code
```

---

## Agentic Scheduler (Python PPO)

Located in: `/agentic_scheduler/`

### Entry Point
```bash
python3 -m agentic_scheduler.server   # Start PPO gRPC server (port 50050)
```

### Key Modules

| Module | Location | Responsibility |
|--------|----------|-----------------|
| `model.py` | `/agentic_scheduler/` | Core PPO policy neural network |
| `features.py` | `/agentic_scheduler/` | Feature encoding for task/worker state |
| `service.py` | `/agentic_scheduler/` | PPO service core (model lifecycle, online updates) |
| `server.py` | `/agentic_scheduler/` | gRPC server exposing PPO decisions |
| `persistence.py` | `/agentic_scheduler/` | MongoDB checkpoint storage |
| `train_ppo.py` | `/agentic_scheduler/` | Offline training script (1,200+ lines) |

### Training Scripts
Located in: `/agentic_scheduler/training/`

| Script | Purpose | Input | Output |
|--------|---------|-------|--------|
| `train_ppo.py` | Main offline PPO training pipeline | Alibaba trace CSV | Trained `.pt` model |
| Various loaders | Load training traces | Trace files | Structured numpy arrays |

### Documentation
- **`TRAINING_ARCHITECTURE.md`** — Training system design & decisions
- **`TRAINING_DECISIONS.md`** — Hyperparameter choices & rationale

### Configuration
- `PPO_MODEL_PATH` env var — Path to model checkpoint (default: auto-detect latest from `agentic_scheduler/models/`)
- `PPO_AUTOSTART` — Auto-start Python gRPC service (default: true)
- `PPO_DEPLOYMENT_MODE` — active/shadow/fallback
- `PPO_ONLINE_UPDATES_ENABLED` — Enable online learning (default: true)

---

## Database

Located in: `/database/`

### Docker Compose Configuration
- **File:** `database/docker-compose.yml`
- **Service:** MongoDB (image: mongo:7.0)
- **Port:** 27017 (default)
- **Start:** `docker compose -f database/docker-compose.yml up -d`

### Collections (Created by Master)
- `TASKS` — Task definitions & metadata
- `ASSIGNMENTS` — Worker → task assignments
- `ATTEMPTS` — Individual execution attempts
- `RESULTS` — Task completion results
- `WORKER_REGISTRY` — Worker availability & capacity
- `FILE_METADATA` — Uploaded/output file tracking
- `SCHEDULER_MODELS` — Archived scheduler checkpoints

---

## UI (Web Dashboard)

Located in: `/ui/`

### Setup
```bash
cd ui && npm install && npm run dev   # Start dev server (port 3001)
npm run build                         # Build production dist
```

### Key Files
- `package.json` — Dependencies & build scripts
- `vite.config.js` — Vite build configuration
- `index.html` — Entry HTML
- `src/` — React components & pages

### Integration
- Started automatically by `runMaster.sh`
- Connects to master HTTP API on port 8080
- Serves on port 3001 (configurable via `WEBUI_PORT`)

---

## Documentation Structure

Located in: `/docs/`

| Document | Purpose | Audience |
|----------|---------|----------|
| `README.md` | Project overview, quick start | Everyone |
| `ARCHITECTURE.md` | System design, component interaction | Developers, architects |
| `GETTING_STARTED.md` | Step-by-step setup guide | New users |
| `DOCUMENTATION.md` | Complete API & configuration reference | Developers |
| `TESTBENCH_RUNBOOK.md` | Testing & benchmarking procedures | QA/performance engineers |
| `USER_MANUAL.md` | CLI command reference | End users |
| `PROJECT_STRUCTURE.md` (NEW) | Directory guide & package responsibilities | Developers |
| `PROJECT_APPENDIX.md` (NEW) | This file — scripts & file reference | Everyone |

---

## Key Environment Variables

### Master
```bash
GRPC_PORT=:50051              # gRPC listen port
HTTP_PORT=:8080               # HTTP API port
CLOUDAI_HEADLESS=true         # Disable interactive CLI
SCHED_ALGO=RTS|RR|PPO         # Scheduler selection
MONGODB_HOST=localhost:27017  # Database connection
PPO_GRPC_ADDR=127.0.0.1:50050 # PPO service address
JWT_SECRET=<random>           # API authentication
```

### Worker
```bash
WORKER_ID=worker-1            # Worker identifier
WORKER_PORT=50052             # gRPC port
WORKER_BIND_IP=auto           # Bind address
WORKER_TOTAL_CPU=4            # Override detected CPU
WORKER_TOTAL_MEMORY_GB=8      # Override detected memory
WORKER_CONTAINER_NETWORK_MODE=bridge  # Container network
```

### Python/PPO
```bash
PPO_MODEL_PATH=latest         # Model checkpoint path
PPO_AUTOSTART=true            # Auto-start gRPC service
PPO_ONLINE_UPDATES_ENABLED=true   # Enable online learning
PPO_DEPLOYMENT_MODE=active    # Deployment mode
```

---

## Useful Commands

### Development Workflow
```bash
# Full setup & build
make all

# Run tests
make test-unit

# Start all services (3 terminals)
Terminal 1: make db-up
Terminal 2: make run-master
Terminal 3: make run-worker

# Register worker (in master CLI)
master> register worker-1 127.0.0.1:50052

# Submit task
master> task ubuntu:latest echo hello

# Monitor task
master> monitor task-abc123

# View cluster status
master> status
master> workers
master> list-tasks
```

### Benchmarking
```bash
# Quick smoke test
./execute-tests.sh

# Full campaign
./execute-tests.sh --full

# Isolated workload mode
./execute-tests.sh --isolated-workloads

# With specific model
./execute-tests.sh --model path/to/model.pt
```

### Database Inspection
```bash
# Connect to MongoDB
docker exec -it cloudai-mongo mongosh

# Inside mongosh:
use cluster_db
db.TASKS.find().limit(5)
db.WORKER_REGISTRY.find()
db.RESULTS.find().count()
```

### Debugging
```bash
# Enable debug logging
export LOG_LEVEL=debug
./runMaster.sh

# Check master health
curl http://localhost:8080/health

# View Prometheus metrics
curl http://localhost:9090

# Connect to Grafana (if running testbench)
http://localhost:3300 (admin/password)
```

---

## Directory Tree (High-Level Monolith View)

```
CloudAI/
├── master/                    # Master control plane
│   ├── main.go
│   ├── internal/
│   │   ├── cli/               # CLI commands
│   │   ├── server/            # gRPC & HTTP servers
│   │   ├── scheduler/         # RR, RTS, PPO schedulers
│   │   ├── controlplane/      # Task orchestration
│   │   ├── db/                # MongoDB persistence
│   │   ├── storage/           # File storage
│   │   ├── aod/               # Parameter optimization
│   │   ├── benchmark/         # Benchmark engine
│   │   └── (6 other packages)
│   └── config/, proto/, proto/

├── worker/                    # Worker execution nodes
│   ├── main.go
│   ├── internal/
│   │   ├── executor/          # Docker execution
│   │   ├── server/            # gRPC server
│   │   ├── telemetry/         # Heartbeat
│   │   ├── logstream/         # Log streaming
│   │   ├── metrics/           # Prometheus
│   │   └── system/            # System utilities
│   └── proto/

├── agentic_scheduler/         # PPO scheduler (Python)
│   ├── model.py, service.py, server.py
│   ├── train_ppo.py, features.py, persistence.py
│   ├── training/              # Training utilities
│   ├── scripts/               # Utility scripts
│   └── docs/

├── proto/                     # gRPC definitions
│   ├── *.proto
│   ├── pb/ (Go generated)
│   ├── py/ (Python generated)
│   └── generate.sh

├── testbench/                 # Integration testing
│   ├── scripts/               # Test runners
│   ├── docker-compose.yml
│   ├── docker-compose.host-master.yml
│   └── (test configs)

├── ui/                        # React web dashboard
│   ├── src/, public/
│   ├── package.json
│   ├── vite.config.js
│   └── index.html

├── database/                  # MongoDB setup
│   └── docker-compose.yml

├── docs/                      # Documentation
│   ├── README.md
│   ├── ARCHITECTURE.md
│   ├── PROJECT_STRUCTURE.md (NEW)
│   ├── PROJECT_APPENDIX.md (NEW)
│   └── (other docs)

├── scripts/                   # Utilities
│   ├── model_promote.sh
│   ├── generate_benchmark_report.py
│   └── install_deps.sh

├── Makefile                   # Central build system
├── README.md                  # Project overview
├── ARCHITECTURE.md            # System design
├── runMaster.sh               # Master launch script
├── runWorker.sh               # Worker launch script
└── execute-tests.sh           # E2E test runner
```

---

## File Locations by Use Case

### "I want to add a new scheduler"
- **Edit:** `/master/internal/scheduler/your_scheduler.go`
- **Interface:** See `scheduler.go` for `Scheduler` interface
- **Test:** Add tests in `*_test.go`
- **Register:** Update `master/main.go` scheduler selection logic

### "I want to modify the CLI"
- **Edit:** `/master/internal/cli/commands.go` and related command files
- **Test:** `/master/internal/cli/*_test.go`
- **Verify:** `make run-master` and try commands

### "I want to adjust PPO training"
- **Edit:** `/agentic_scheduler/train_ppo.py`
- **Hyperparameters:** See `argparse` section for tunable parameters
- **Model:** Check `/agentic_scheduler/model.py` for architecture changes

### "I want to troubleshoot task execution"
- **Worker logs:** Check `./runWorker.sh` output
- **Executor:** `/worker/internal/executor/executor.go`
- **Docker:** Verify Docker is running and containers can be created

### "I want to add a new test"
- **Integration tests:** `/testbench/scripts/`
- **Unit tests:** Add `*_test.go` in relevant package
- **Run:** `make test-unit` or appropriate testbench command

---

## Quick Lookup: "Where do I find X?"

| What | Where |
|------|-------|
| gRPC service definitions | `/proto/*.proto` |
| Master CLI commands | `/master/internal/cli/` |
| Worker Docker execution | `/worker/internal/executor/executor.go` |
| Scheduler implementations | `/master/internal/scheduler/` |
| PPO model & training | `/agentic_scheduler/model.py`, `train_ppo.py` |
| HTTP API handlers | `/master/internal/http/`, `/master/internal/server/` |
| Task persistence | `/master/internal/db/tasks.go` |
| Worker registration | `/master/internal/server/worker_server.go` |
| UI components | `/ui/src/` |
| Makefile targets | `Makefile` (lines 1-399) |
| Configuration defaults | `/master/internal/config/` |
| Metrics definitions | `/master/internal/metrics/`, `/worker/internal/metrics/` |
| Test workflows | `/master/internal/testworkflow/`, `/testbench/scripts/` |
| Database schema | `/master/internal/db/init.go` |

---

## Getting Help

1. **Quick start:** See `README.md` — "Quick Start" section
2. **Architecture overview:** Read `ARCHITECTURE.md`
3. **Detailed reference:** Check `docs/DOCUMENTATION.md`
4. **Find a specific component:** Use "Quick Lookup" table above
5. **Understand testbench:** Read `docs/TESTBENCH_RUNBOOK.md`
6. **Troubleshoot:** Check logs in `./runMaster.sh`, `./runWorker.sh` output or use `docker logs`

---

**Last Updated:** 2026-05-13  
**Version:** 2.0 (Post-Cleanup Reorganization)
