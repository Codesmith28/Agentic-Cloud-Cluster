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

## 2. Go Workspace Layout (`go.work`)

The Go codebase is managed as a unified Go workspace:

1. **`pkg/`** (`github.com/Codesmith28/Agentic-Cloud-Cluster/pkg`):
   - `domain/`: Pure data models (`Task`, `Worker`, `Assignment`, `TaskAttempt`, `TaskResult`).
   - `ports/`: Standard interfaces for schedulers, repositories, and metric sinks.
   - `envutil/`: Clean helpers for typed environment variable loading.

2. **`master/`**:
   - `main.go`: Minimal entrypoint forwarding to `internal/app`.
   - `internal/app/`: Application bootstrap, signal handling, and subsystem orchestration.
   - `internal/db/`: `MongoStore` with single connection pool powering all collections.
   - `internal/server/`: Modular server components (`grpc_handlers.go`, `worker_manager.go`, `task_manager.go`, `queue_processor.go`).
   - `internal/controlplane/`: Focused CLI command dispatchers.
   - `internal/benchmark/`: Simulation, profiles, and artifact generation.

3. **`worker/`**:
   - `main.go`: Minimal entrypoint.
   - `internal/app/`: Worker lifecycle and graceful shutdown.
   - `internal/executor/`: Docker container executor with memory/CPU limits and safety checks.
   - `internal/server/`: gRPC server handling task assignments and cancellations.
   - `internal/telemetry/`: Heartbeat and node health reporter.
   - `internal/logstream/`: Non-blocking log broadcaster.

---

## 3. Python PPO Reinforcement Learning Policy (`agentic_scheduler/`)

- **`model.py`**: Contains `PPOActorCritic` neural network architecture.
- **`features.py`**: State representation (5-dim task vector, 9-dim worker feature matrix).
- **`service.py`**: Inference engine, action masking, and online replay buffer updates.
- **`train_ppo.py`**: Offline and trace replay training runner.
- **`tests/test_scheduler.py`**: Native unit test suite verifying neural network layers, serialization, and model loading.

---

## 4. Verification & Testing

### Go Tests
```bash
# Run all master, worker, and pkg tests
(cd pkg && go test -v ./...)
(cd master && go test -v ./...)
(cd worker && go test -v ./...)
```

### Python Tests
```bash
# Run Python scheduler tests
python3 -m unittest discover -s agentic_scheduler/tests/ -v
```
