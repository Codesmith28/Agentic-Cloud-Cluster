# CloudAI - AI-Driven Agentic Scheduler
## Complete Sprint Plan & Development Roadmap

**Last Updated:** October 5, 2025  
**Project:** CloudAI - Hybrid Go/Python Agentic Task Scheduler  
**Repository:** github.com/Codesmith28/CloudAI

This document provides a **complete, production-oriented sprint plan** for the CloudAI system: **Go** for the master node and workers, **Python** for the agentic planner, with **gRPC** communication between them. This plan is step-by-step, lists concrete files, function names, algorithms to implement in each sprint, tests, acceptance criteria, and immediate next actions.

---

## Table of Contents
1. [Project Overview](#project-overview)
2. [Current Folder Structure](#current-folder-structure)
3. [Technology Stack](#technology-stack)
4. [Assumptions & Constraints](#assumptions--constraints)
5. [Definition of Done](#definition-of-done)
6. [Sprint Plan (13 Sprints)](#sprint-plan)
7. [Planner Algorithms Roadmap](#planner-algorithms-roadmap)
8. [Testing Strategy](#testing-strategy)
9. [Learning Resources](#learning-resources)
10. [Immediate Next Actions](#immediate-next-actions)

---

## Project Overview

The CloudAI system is an **AI-driven agentic scheduler** that intelligently assigns computational tasks to workers using advanced planning algorithms. Unlike traditional schedulers (like Kubernetes) which use greedy, rule-based approaches, CloudAI employs:

- **Forward state-space planning** using A* search
- **Constraint-based scheduling** using OR-Tools CP-SAT solver
- **Incremental replanning** for fault tolerance
- **ML-based runtime prediction** for improved decision quality

### System Components

1. **Master Node (Go)** - Orchestrates task scheduling, manages workers, calls planner
2. **Worker Nodes (Go)** - Execute tasks in containers or VMs
3. **Planner Service (Python)** - Implements AI planning algorithms via gRPC

---

## Current Folder Structure

```
CloudAI/
├── proto/                          # Protocol buffer definitions
│   └── scheduler.proto             # Shared message definitions
├── go-master/                      # Go master and worker implementation
│   ├── go.mod                      # Go module definition
│   ├── go.sum                      # Go dependencies
│   ├── cmd/                        # Entry points
│   │   ├── master/
│   │   │   └── main.go            # Master node entry point
│   │   └── worker/
│   │       └── main.go            # Worker node entry point
│   └── pkg/                        # Go packages
│       ├── api/                    # Generated protobuf code + API layer
│       │   ├── api.go             # Custom API helpers
│       │   ├── scheduler.pb.go    # Generated protobuf messages
│       │   └── scheduler_grpc.pb.go # Generated gRPC stubs
│       ├── scheduler/              # Scheduler coordination
│       │   └── scheduler.go
│       ├── workerregistry/         # Worker tracking and management
│       │   └── registry.go
│       ├── taskqueue/              # Task queue management
│       │   └── queue.go
│       ├── execution/              # Task execution coordination
│       │   └── executor.go
│       ├── persistence/            # Data persistence (BoltDB)
│       │   └── persistence.go
│       ├── monitor/                # Health monitoring and fault detection
│       │   └── monitor.go
│       ├── container_manager/      # Docker container management
│       │   └── container.go
│       ├── vm_manager/             # KVM/VM management
│       │   └── vm.go
│       └── testbench/              # Testing and benchmarking tools
│           └── testbench.go
├── planner_py/                     # Python planner service
│   ├── __init__.py                # Package initialization
│   ├── planner_server.py          # gRPC server entry point
│   ├── requirements.txt           # Python dependencies
│   ├── scheduler_pb2.py           # Generated protobuf messages
│   ├── scheduler_pb2_grpc.py      # Generated gRPC stubs
│   ├── venv/                      # Virtual environment
│   └── planner/                   # Planning algorithms
│       ├── __init__.py
│       ├── a_star.py              # A* forward planner
│       ├── or_tools_scheduler.py  # CP-SAT solver
│       ├── replanner.py           # Incremental replanning
│       └── predictor.py           # ML runtime predictor
├── docs/                           # Documentation
│   ├── architecture.md            # High-level design
│   ├── low_level_design.md        # Detailed component design
│   ├── sequence_diagram.md        # Sequence diagrams
│   └── ai_scheduler_sequence.xml  # Diagram source
├── ci/                             # CI/CD configuration
│   ├── ci.yml                     # GitHub Actions workflow
│   └── Makefile                   # Build automation
├── README.md                       # Project readme
└── Sprint.md                       # This file
```

---

## Technology Stack

### Go Components
- **Go 1.23+** - System programming language
- **gRPC** - Communication protocol
- **Protocol Buffers** - Data serialization
- **BoltDB** - Embedded key-value database
- **Docker SDK** - Container management
- **libvirt-go** - VM management (optional)

### Python Components
- **Python 3.12+** - Planning implementation language
- **gRPC Tools** - Server implementation
- **OR-Tools** - Constraint programming solver
- **scikit-learn** - ML for runtime prediction
- **pyperplan** - Planning library (optional)

### Infrastructure
- **Docker** - Containerization
- **Docker Compose** - Local development
- **Kubernetes/Helm** - Production deployment (optional)
- **Prometheus** - Metrics and monitoring
- **GitHub Actions** - CI/CD

---

## Assumptions & Constraints — I’ll act as Scrum Master and give you a **complete, production-oriented sprint plan** for the hybrid system: **Go** for the master + workers, **Python** for the agentic planner, with **gRPC** between them. This plan is step-by-step, lists concrete files / function names, algorithms to implement in each sprint, tests, acceptance criteria, and immediate next actions so your team can start coding today.

---

# Assumptions (so plan is actionable)

## Assumptions & Constraints

### Timeline
* **Sprint Duration:** 2 weeks per sprint
* **Total Duration:** 13 sprints (≈ 26 weeks / 6 months)
* **Team Size:** Flexible (plan scales with team size)

### Technical Decisions
* **Communication Protocol:** gRPC with Protocol Buffers for all inter-service communication
* **Persistence:** BoltDB for master state (lightweight, embedded)
* **Task Execution:** Docker containers (primary), KVM VMs (optional for isolation)
* **Planner State:** Ephemeral (stateless service) with optional logging to disk
* **Development Environment:** Docker Compose for local development and testing
* **Production Deployment:** Kubernetes with Helm charts (Sprint 12+)

### Core Architecture Split
* **Go Master** - Handles orchestration, worker management, API, execution, persistence
* **Python Planner** - Implements planning algorithms, ML models, optimization
* **Go Workers** - Execute tasks, report metrics, manage resources

### Fallback Strategy
* Go master **MUST** always include a greedy scheduler fallback
* If planner is unavailable or exceeds time budget, master uses greedy scheduling
* This ensures system availability even when planner is down

---

## Definition of Done

Every sprint task must meet the following criteria before being marked complete:

### Code Quality
1. ✅ **Compiles Successfully** - All code compiles without errors
2. ✅ **Tests Pass** - Unit tests and integration tests pass in CI
3. ✅ **Code Coverage** - Minimum 70% coverage for new code
4. ✅ **Linting Clean** - No linting errors
   - Go: `gofmt`, `golangci-lint`
   - Python: `black`, `flake8`, `pylint`

### Documentation
5. ✅ **API Documentation** - Public functions have docstrings/comments
6. ✅ **Architecture Updates** - `docs/architecture.md` updated if structure changes
7. ✅ **README Updates** - Setup/usage instructions updated if needed

### Testing
8. ✅ **Unit Tests** - Core logic has unit tests
9. ✅ **Integration Tests** - End-to-end scenarios tested
10. ✅ **Manual Testing** - Feature manually tested by another team member

### Deployment
11. ✅ **CI Pipeline** - GitHub Actions pipeline passes
12. ✅ **Demo Ready** - Working demonstration available
13. ✅ **Code Review** - At least one peer review completed

---

## Sprint Plan

### Sprint 0 — Foundation & Infrastructure (2 weeks)
**Sprint Goal:** Establish repository structure, CI/CD pipeline, and basic protobuf communication

#### User Stories
- **US-0.1:** As a developer, I need a working repository with proper Go and Python project structure
- **US-0.2:** As a developer, I need CI/CD pipelines to automatically test my code
- **US-0.3:** As a developer, I need protobuf definitions to be generated automatically

#### Detailed Tasks

##### Task 0.1: Repository Setup (2 days)
**Assignee:** DevOps Lead

**Subtasks:**
1. Initialize Git repository with proper `.gitignore`
   - Go: `vendor/`, `*.exe`, `*.test`, `*.out`
   - Python: `venv/`, `__pycache__/`, `*.pyc`, `.pytest_cache/`
   
2. Create folder structure (already done ✅)

3. Initialize Go module
   ```bash
   cd go-master
   go mod init github.com/Codesmith28/CloudAI
   ```

4. Initialize Python virtual environment
   ```bash
   cd planner_py
   python -m venv venv
   source venv/bin/activate
   pip install --upgrade pip
   ```

**Deliverables:**
- ✅ Repository structure created
- ✅ Go module initialized (`go.mod` exists)
- ✅ Python venv created

---

##### Task 0.2: Protocol Buffer Definition (3 days)
**Assignee:** Backend Developer

**File:** `proto/scheduler.proto`

**Implementation Details:**

Create comprehensive protobuf definitions with all necessary messages:

```protobuf
syntax = "proto3";

package scheduler;

option go_package = "github.com/Codesmith28/CloudAI/pkg/api;proto";
option java_multiple_files = true;
option java_package = "com.cloudai.scheduler.proto";

import "google/protobuf/timestamp.proto";
import "google/protobuf/struct.proto";

// ============ Core Domain Messages ============

message Task {
  string id = 1;
  double cpu_req = 2;              // CPU cores required (can be fractional, e.g., 0.5)
  int32 mem_mb = 3;                // Memory in MB
  int32 gpu_req = 4;               // Number of GPUs
  int32 disk_mb = 5;               // Disk space in MB
  int32 network_mbps = 6;          // Network bandwidth in Mbps
  string task_type = 7;            // Type for prediction (e.g., "ml_training", "batch_job")
  int64 estimated_sec = 8;         // Estimated runtime in seconds
  int32 priority = 9;              // Higher = more urgent
  int64 deadline_unix = 10;        // Unix timestamp; 0 = no deadline
  map<string,string> meta = 11;    // Free-form metadata
  string container_image = 12;     // Docker image name
  repeated string command = 13;    // Command and arguments
  ExecPreference exec_pref = 14;   // Container vs VM preference
  bool checkpointable = 15;        // Can this task be checkpointed?
  string tenant = 16;              // Multi-tenancy support
  repeated string labels = 17;     // Labels for affinity/anti-affinity
  google.protobuf.Struct extra = 18; // Extensible metadata
}

enum ExecPreference {
  EXEC_ANY = 0;
  EXEC_CONTAINER = 1;
  EXEC_VM = 2;
}

message Worker {
  string id = 1;
  double total_cpu = 2;
  int32 total_mem = 3;             // MB
  int32 gpus = 4;
  int32 total_disk_mb = 5;
  double free_cpu = 6;
  int32 free_mem = 7;
  int32 free_gpus = 8;
  repeated string labels = 9;
  int64 last_seen_unix = 10;
  string status = 11;              // "active", "draining", "down"
  map<string,string> capabilities = 12;
}

// ============ Planning Messages ============

message Assignment {
  string task_id = 1;
  string worker_id = 2;
  int64 start_unix = 3;
  int64 est_duration_sec = 4;
}

message PlanRequest {
  repeated Task tasks = 1;
  repeated Worker workers = 2;
  double planning_time_budget_sec = 3;
  string previous_plan_id = 4;      // For incremental replanning
  repeated string failed_worker_ids = 5;
}

message PlanResponse {
  repeated Assignment assignments = 1;
  double cost = 2;
  string status_message = 3;
  string plan_id = 4;
  double planning_time_sec = 5;
}

// ============ Task Lifecycle Messages ============

enum TaskStatus {
  PENDING = 0;
  SCHEDULED = 1;
  RUNNING = 2;
  COMPLETED = 3;
  FAILED = 4;
  CANCELLED = 5;
}

message SubmitTaskRequest {
  Task task = 1;
}

message SubmitTaskResponse {
  string task_id = 1;
  string message = 2;
}

message GetTaskStatusRequest {
  string task_id = 1;
}

message GetTaskStatusResponse {
  string task_id = 1;
  TaskStatus status = 2;
  string worker_id = 3;
  string message = 4;
}

message CancelTaskRequest {
  string task_id = 1;
}

message CancelTaskResponse {
  bool success = 1;
  string message = 2;
}

// ============ Worker Messages ============

message RegisterWorkerRequest {
  Worker worker = 1;
}

message RegisterWorkerResponse {
  bool success = 1;
  string message = 2;
}

message HeartbeatRequest {
  Worker worker = 1;
  repeated string running_task_ids = 2;
}

message HeartbeatResponse {
  bool success = 1;
  repeated string tasks_to_cancel = 2;  // Master can request cancellation
}

message ListWorkersRequest {}

message ListWorkersResponse {
  repeated Worker workers = 1;
}

// ============ Task Assignment Messages ============

message AssignTaskRequest {
  Task task = 1;
  int64 start_unix = 2;
}

message AssignTaskResponse {
  bool accepted = 1;
  string message = 2;
}

message TaskCompletionRequest {
  string task_id = 1;
  string worker_id = 2;
  TaskStatus status = 3;
  string message = 4;
  int64 actual_duration_sec = 5;
  map<string,double> resource_usage = 6;  // For predictor training
}

message TaskCompletionResponse {
  bool acknowledged = 1;
}

// ============ Services ============

service SchedulerService {
  // Client-facing APIs
  rpc SubmitTask(SubmitTaskRequest) returns (SubmitTaskResponse);
  rpc GetTaskStatus(GetTaskStatusRequest) returns (GetTaskStatusResponse);
  rpc CancelTask(CancelTaskRequest) returns (CancelTaskResponse);
  
  // Worker APIs
  rpc RegisterWorker(RegisterWorkerRequest) returns (RegisterWorkerResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc ListWorkers(ListWorkersRequest) returns (ListWorkersResponse);
  
  // Task completion callback
  rpc ReportTaskCompletion(TaskCompletionRequest) returns (TaskCompletionResponse);
}

service WorkerService {
  // Master -> Worker
  rpc AssignTask(AssignTaskRequest) returns (AssignTaskResponse);
  rpc CancelTask(CancelTaskRequest) returns (CancelTaskResponse);
}

service PlannerService {
  // Master -> Planner
  rpc Plan(PlanRequest) returns (PlanResponse);
}
```

**Generate Code:**

```bash
# From repo root

# Generate Go code
protoc \
  -I proto \
  --go_out=go-master/pkg/api --go_opt=paths=source_relative \
  --go-grpc_out=go-master/pkg/api --go-grpc_opt=paths=source_relative \
  proto/scheduler.proto

# Generate Python code
python -m grpc_tools.protoc \
  -I proto \
  --python_out=planner_py --grpc_python_out=planner_py \
  proto/scheduler.proto
```

**Deliverables:**
- ✅ `proto/scheduler.proto` created with complete message definitions
- ✅ Go stubs generated in `go-master/pkg/api/`
- ✅ Python stubs generated in `planner_py/`

---

##### Task 0.3: CI/CD Pipeline Setup (3 days)
**Assignee:** DevOps Lead

**File:** `ci/ci.yml`

**Implementation:**

Create GitHub Actions workflow for automated testing:

```yaml
name: CI Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  go-tests:
    name: Go Tests and Linting
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
    
    - name: Install dependencies
      working-directory: ./go-master
      run: |
        go mod download
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    
    - name: Run gofmt
      working-directory: ./go-master
      run: |
        gofmt -d .
        test -z "$(gofmt -l .)"
    
    - name: Run golangci-lint
      working-directory: ./go-master
      run: golangci-lint run ./...
    
    - name: Run tests
      working-directory: ./go-master
      run: go test -v -race -coverprofile=coverage.out ./...
    
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        files: ./go-master/coverage.out

  python-tests:
    name: Python Tests and Linting
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Python
      uses: actions/setup-python@v4
      with:
        python-version: '3.12'
    
    - name: Install dependencies
      working-directory: ./planner_py
      run: |
        python -m pip install --upgrade pip
        pip install -r requirements.txt
        pip install pytest flake8 black pylint pytest-cov
    
    - name: Run black
      working-directory: ./planner_py
      run: black --check .
    
    - name: Run flake8
      working-directory: ./planner_py
      run: flake8 . --max-line-length=100
    
    - name: Run pylint
      working-directory: ./planner_py
      run: pylint planner/ planner_server.py || true
    
    - name: Run tests
      working-directory: ./planner_py
      run: pytest --cov=planner --cov-report=xml
    
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        files: ./planner_py/coverage.xml
```

**File:** `ci/Makefile`

```makefile
.PHONY: all proto go-build python-setup test clean

all: proto go-build python-setup

# Protocol buffer generation
proto:
	protoc -I proto \
		--go_out=go-master/pkg/api --go_opt=paths=source_relative \
		--go-grpc_out=go-master/pkg/api --go-grpc_opt=paths=source_relative \
		proto/scheduler.proto
	cd planner_py && python -m grpc_tools.protoc \
		-I ../proto \
		--python_out=. --grpc_python_out=. \
		../proto/scheduler.proto

# Go build
go-build:
	cd go-master && go mod tidy && go build -v ./...

# Python setup
python-setup:
	cd planner_py && pip install -r requirements.txt

# Run all tests
test: go-test python-test

go-test:
	cd go-master && go test -v -race ./...

python-test:
	cd planner_py && pytest -v

# Linting
lint: go-lint python-lint

go-lint:
	cd go-master && gofmt -l . && golangci-lint run ./...

python-lint:
	cd planner_py && black --check . && flake8 .

# Clean generated files
clean:
	rm -f go-master/pkg/api/*.pb.go
	rm -f planner_py/*_pb2.py planner_py/*_pb2_grpc.py
	cd go-master && go clean -testcache

# Docker compose
up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f
```

**Deliverables:**
- ✅ CI pipeline configured and passing
- ✅ Makefile for common tasks
- ✅ Automated proto generation

---

##### Task 0.4: Documentation & Onboarding (2 days)
**Assignee:** Tech Lead

**Files to Update:**
- `README.md` - Project overview and quick start
- `docs/architecture.md` - Already exists, verify completeness
- `docs/low_level_design.md` - Already exists, verify completeness

**Create:** `docs/CONTRIBUTING.md`

```markdown
# Contributing to CloudAI

## Development Setup

### Prerequisites
- Go 1.23+
- Python 3.12+
- Protocol Buffers compiler (protoc)
- Docker and Docker Compose

### Getting Started

1. Clone repository:
   ```bash
   git clone https://github.com/Codesmith28/CloudAI.git
   cd CloudAI
   ```

2. Generate protobuf code:
   ```bash
   make proto
   ```

3. Set up Go environment:
   ```bash
   cd go-master
   go mod download
   ```

4. Set up Python environment:
   ```bash
   cd planner_py
   python -m venv venv
   source venv/bin/activate  # On Windows: venv\Scripts\activate
   pip install -r requirements.txt
   ```

5. Run tests:
   ```bash
   make test
   ```

## Development Workflow

1. Create feature branch: `git checkout -b feature/your-feature`
2. Make changes
3. Run tests: `make test`
4. Run linting: `make lint`
5. Commit changes: `git commit -m "feat: your feature"`
6. Push and create PR

## Code Style

### Go
- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go)
- Run `golangci-lint` before committing

### Python
- Use `black` for formatting (line length: 100)
- Follow PEP 8
- Use type hints where possible
- Run `flake8` and `pylint` before committing

## Testing

- Unit tests are required for all new functions
- Integration tests for new features
- Minimum 70% code coverage

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `test:` Test changes
- `refactor:` Code refactoring
- `chore:` Build/CI changes
```

**Deliverables:**
- ✅ README.md updated with setup instructions
- ✅ CONTRIBUTING.md created
- ✅ Architecture docs verified

---

##### Task 0.5: Docker Compose Setup (2 days)
**Assignee:** DevOps Lead

**File:** `docker-compose.yml`

```yaml
version: '3.8'

services:
  master:
    build:
      context: ./go-master
      dockerfile: Dockerfile
    ports:
      - "50051:50051"  # gRPC port
      - "9090:9090"    # Metrics port
    environment:
      - PLANNER_ADDRESS=planner:50052
      - DB_PATH=/data/master.db
    volumes:
      - master-data:/data
    depends_on:
      - planner
    networks:
      - cloudai

  planner:
    build:
      context: ./planner_py
      dockerfile: Dockerfile
    ports:
      - "50052:50052"  # gRPC port
    environment:
      - GRPC_PORT=50052
    networks:
      - cloudai

  worker1:
    build:
      context: ./go-master
      dockerfile: Dockerfile.worker
    environment:
      - MASTER_ADDRESS=master:50051
      - WORKER_ID=worker-1
      - TOTAL_CPU=4
      - TOTAL_MEM=8192
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    depends_on:
      - master
    networks:
      - cloudai

  worker2:
    build:
      context: ./go-master
      dockerfile: Dockerfile.worker
    environment:
      - MASTER_ADDRESS=master:50051
      - WORKER_ID=worker-2
      - TOTAL_CPU=2
      - TOTAL_MEM=4096
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    depends_on:
      - master
    networks:
      - cloudai

volumes:
  master-data:

networks:
  cloudai:
    driver: bridge
```

**Files:** Create Dockerfiles

`go-master/Dockerfile`:
```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o master ./cmd/master

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/master .

EXPOSE 50051 9090
CMD ["./master"]
```

`go-master/Dockerfile.worker`:
```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o worker ./cmd/worker

FROM alpine:latest
RUN apk --no-cache add ca-certificates docker
WORKDIR /root/
COPY --from=builder /app/worker .

CMD ["./worker"]
```

`planner_py/Dockerfile`:
```dockerfile
FROM python:3.12-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 50052
CMD ["python", "planner_server.py"]
```

**Deliverables:**
- ✅ Docker Compose configuration
- ✅ Dockerfiles for all services
- ✅ Services can be started with `docker-compose up`

---

#### Sprint 0 Acceptance Criteria
- [ ] Repository structure is complete and documented
- [ ] Protocol buffers generate successfully for Go and Python
- [ ] CI pipeline runs and passes (even with minimal tests)
- [ ] Docker Compose brings up skeleton services
- [ ] All developers can clone repo and run `make test` successfully
- [ ] Documentation is complete and clear

#### Sprint 0 Demo
- Show repository structure
- Run `make proto` to generate code
- Run `make test` to show CI passing
- Run `docker-compose up` to show skeleton services starting

---

### Sprint 1 — Core Models, Worker Registry, Task Queue (2 weeks)
**Sprint Goal:** Implement fundamental data structures and in-memory storage for tasks and workers

#### User Stories
- **US-1.1:** As a master node, I need to store and track worker information
- **US-1.2:** As a master node, I need to queue incoming tasks
- **US-1.3:** As a master node, I need to accept task submissions via gRPC

#### Detailed Tasks

##### Task 1.1: Worker Registry Implementation (4 days)
**Assignee:** Backend Developer 1

**File:** `go-master/pkg/workerregistry/registry.go`

**Implementation:**

```go
package workerregistry

import (
	"sync"
	"time"
	
	pb "github.com/Codesmith28/CloudAI/pkg/api"
)

// Registry manages worker state and availability
type Registry struct {
	mu          sync.RWMutex
	workers     map[string]*pb.Worker
	reservations map[string]*Reservation  // taskID -> Reservation
	subscribers []chan<- RegistryEvent
}

type Reservation struct {
	TaskID    string
	WorkerID  string
	CPU       float64
	MemMB     int32
	GPU       int32
	ExpiresAt time.Time
}

type RegistryEvent struct {
	Type     string  // "worker_added", "worker_updated", "worker_removed"
	WorkerID string
	Worker   *pb.Worker
}

// NewRegistry creates a new worker registry
func NewRegistry() *Registry {
	return &Registry{
		workers:      make(map[string]*pb.Worker),
		reservations: make(map[string]*Reservation),
		subscribers:  make([]chan<- RegistryEvent, 0),
	}
}

// UpdateHeartbeat updates worker status from heartbeat
func (r *Registry) UpdateHeartbeat(worker *pb.Worker) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	worker.LastSeenUnix = time.Now().Unix()
	worker.Status = "active"
	
	existing, exists := r.workers[worker.Id]
	eventType := "worker_updated"
	if !exists {
		eventType = "worker_added"
	}
	
	r.workers[worker.Id] = worker
	
	// Notify subscribers
	r.notifySubscribers(RegistryEvent{
		Type:     eventType,
		WorkerID: worker.Id,
		Worker:   worker,
	})
	
	return nil
}

// GetSnapshot returns a copy of all active workers
func (r *Registry) GetSnapshot() []*pb.Worker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	snapshot := make([]*pb.Worker, 0, len(r.workers))
	now := time.Now().Unix()
	
	for _, w := range r.workers {
		// Only include workers seen in last 30 seconds
		if now - w.LastSeenUnix < 30 {
			// Create a copy
			workerCopy := *w
			snapshot = append(snapshot, &workerCopy)
		}
	}
	
	return snapshot
}

// Reserve resources for a task on a specific worker
func (r *Registry) Reserve(taskID string, workerID string, cpuReq float64, memMB, gpu int32, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	worker, exists := r.workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}
	
	// Check if worker has enough resources
	if worker.FreeCpu < cpuReq || worker.FreeMem < memMB || worker.FreeGpus < gpu {
		return fmt.Errorf("worker %s has insufficient resources", workerID)
	}
	
	// Create reservation
	r.reservations[taskID] = &Reservation{
		TaskID:    taskID,
		WorkerID:  workerID,
		CPU:       cpuReq,
		MemMB:     memMB,
		GPU:       gpu,
		ExpiresAt: time.Now().Add(ttl),
	}
	
	// Deduct resources optimistically
	worker.FreeCpu -= cpuReq
	worker.FreeMem -= memMB
	worker.FreeGpus -= gpu
	
	return nil
}

// Release resources when task completes
func (r *Registry) Release(taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	reservation, exists := r.reservations[taskID]
	if !exists {
		return fmt.Errorf("no reservation found for task %s", taskID)
	}
	
	worker, exists := r.workers[reservation.WorkerID]
	if exists {
		worker.FreeCpu += reservation.CPU
		worker.FreeMem += reservation.MemMB
		worker.FreeGpus += reservation.GPU
	}
	
	delete(r.reservations, taskID)
	return nil
}

// Subscribe to registry events
func (r *Registry) Subscribe() <-chan RegistryEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	ch := make(chan RegistryEvent, 100)
	r.subscribers = append(r.subscribers, ch)
	return ch
}

func (r *Registry) notifySubscribers(event RegistryEvent) {
	for _, ch := range r.subscribers {
		select {
		case ch <- event:
		default:
			// Skip if channel is full
		}
	}
}

// CleanupStaleWorkers removes workers that haven't sent heartbeat
func (r *Registry) CleanupStaleWorkers(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	now := time.Now().Unix()
	cutoff := now - int64(timeout.Seconds())
	
	for id, worker := range r.workers {
		if worker.LastSeenUnix < cutoff {
			delete(r.workers, id)
			r.notifySubscribers(RegistryEvent{
				Type:     "worker_removed",
				WorkerID: id,
				Worker:   worker,
			})
		}
	}
}

// CleanupExpiredReservations removes expired reservations
func (r *Registry) CleanupExpiredReservations() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	now := time.Now()
	for taskID, res := range r.reservations {
		if now.After(res.ExpiresAt) {
			// Return resources
			if worker, exists := r.workers[res.WorkerID]; exists {
				worker.FreeCpu += res.CPU
				worker.FreeMem += res.MemMB
				worker.FreeGpus += res.GPU
			}
			delete(r.reservations, taskID)
		}
	}
}
```

**Test File:** `go-master/pkg/workerregistry/registry_test.go`

```go
package workerregistry

import (
	"testing"
	"time"
	
	pb "github.com/Codesmith28/CloudAI/pkg/api"
)

func TestRegistryHeartbeat(t *testing.T) {
	registry := NewRegistry()
	
	worker := &pb.Worker{
		Id:       "worker-1",
		TotalCpu: 4.0,
		TotalMem: 8192,
		Gpus:     1,
		FreeCpu:  4.0,
		FreeMem:  8192,
		FreeGpus: 1,
	}
	
	err := registry.UpdateHeartbeat(worker)
	if err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}
	
	snapshot := registry.GetSnapshot()
	if len(snapshot) != 1 {
		t.Fatalf("Expected 1 worker, got %d", len(snapshot))
	}
	
	if snapshot[0].Id != "worker-1" {
		t.Errorf("Expected worker-1, got %s", snapshot[0].Id)
	}
}

func TestResourceReservation(t *testing.T) {
	registry := NewRegistry()
	
	worker := &pb.Worker{
		Id:       "worker-1",
		TotalCpu: 4.0,
		TotalMem: 8192,
		FreeCpu:  4.0,
		FreeMem:  8192,
	}
	registry.UpdateHeartbeat(worker)
	
	// Reserve resources
	err := registry.Reserve("task-1", "worker-1", 2.0, 4096, 0, time.Minute)
	if err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}
	
	// Check worker resources updated
	snapshot := registry.GetSnapshot()
	if snapshot[0].FreeCpu != 2.0 {
		t.Errorf("Expected FreeCpu=2.0, got %f", snapshot[0].FreeCpu)
	}
	
	// Release resources
	err = registry.Release("task-1")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	
	// Check resources returned
	snapshot = registry.GetSnapshot()
	if snapshot[0].FreeCpu != 4.0 {
		t.Errorf("Expected FreeCpu=4.0 after release, got %f", snapshot[0].FreeCpu)
	}
}

func TestStaleWorkerCleanup(t *testing.T) {
	registry := NewRegistry()
	
	worker := &pb.Worker{
		Id:           "worker-1",
		LastSeenUnix: time.Now().Add(-2 * time.Minute).Unix(),
	}
	registry.workers["worker-1"] = worker
	
	registry.CleanupStaleWorkers(time.Minute)
	
	snapshot := registry.GetSnapshot()
	if len(snapshot) != 0 {
		t.Errorf("Expected stale worker to be removed")
	}
}
```

**Deliverables:**
- ✅ Worker registry with thread-safe operations
- ✅ Resource reservation system
- ✅ Event subscription for monitoring
- ✅ Unit tests with >80% coverage

---

##### Task 1.2: Task Queue Implementation (4 days)
**Assignee:** Backend Developer 2

**File:** `go-master/pkg/taskqueue/queue.go`

**Implementation:**

```go
package taskqueue

import (
	"container/heap"
	"sync"
	"time"
	
	pb "github.com/Codesmith28/CloudAI/pkg/api"
)

// TaskQueue manages pending tasks with priority ordering
type TaskQueue struct {
	mu       sync.RWMutex
	tasks    map[string]*TaskEntry  // taskID -> TaskEntry
	pq       PriorityQueue
	cond     *sync.Cond
}

type TaskEntry struct {
	Task      *pb.Task
	Status    pb.TaskStatus
	CreatedAt time.Time
	index     int  // heap index
}

// PriorityQueue implements heap.Interface
type PriorityQueue []*TaskEntry

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// Higher priority first, then earliest deadline
	if pq[i].Task.Priority != pq[j].Task.Priority {
		return pq[i].Task.Priority > pq[j].Task.Priority
	}
	
	// If both have deadlines, earlier deadline first
	if pq[i].Task.DeadlineUnix > 0 && pq[j].Task.DeadlineUnix > 0 {
		return pq[i].Task.DeadlineUnix < pq[j].Task.DeadlineUnix
	}
	
	// Deadline task comes first
	if pq[i].Task.DeadlineUnix > 0 {
		return true
	}
	if pq[j].Task.DeadlineUnix > 0 {
		return false
	}
	
	// Otherwise FIFO
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	entry := x.(*TaskEntry)
	entry.index = n
	*pq = append(*pq, entry)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	*pq = old[0 : n-1]
	return entry
}

// NewTaskQueue creates a new task queue
func NewTaskQueue() *TaskQueue {
	tq := &TaskQueue{
		tasks: make(map[string]*TaskEntry),
		pq:    make(PriorityQueue, 0),
	}
	tq.cond = sync.NewCond(&tq.mu)
	heap.Init(&tq.pq)
	return tq
}

// Enqueue adds a task to the queue
func (tq *TaskQueue) Enqueue(task *pb.Task) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	
	if _, exists := tq.tasks[task.Id]; exists {
		return fmt.Errorf("task %s already exists", task.Id)
	}
	
	entry := &TaskEntry{
		Task:      task,
		Status:    pb.TaskStatus_PENDING,
		CreatedAt: time.Now(),
	}
	
	tq.tasks[task.Id] = entry
	heap.Push(&tq.pq, entry)
	
	// Signal waiting goroutines
	tq.cond.Signal()
	
	return nil
}

// DequeueBatch retrieves up to n highest priority tasks
func (tq *TaskQueue) DequeueBatch(n int) []*pb.Task {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	
	result := make([]*pb.Task, 0, n)
	
	for i := 0; i < n && tq.pq.Len() > 0; i++ {
		entry := heap.Pop(&tq.pq).(*TaskEntry)
		entry.Status = pb.TaskStatus_SCHEDULED
		result = append(result, entry.Task)
	}
	
	return result
}

// PeekPending returns pending tasks without removing them
func (tq *TaskQueue) PeekPending() []*pb.Task {
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	
	result := make([]*pb.Task, 0, tq.pq.Len())
	for _, entry := range tq.pq {
		result = append(result, entry.Task)
	}
	
	return result
}

// UpdateStatus updates task status
func (tq *TaskQueue) UpdateStatus(taskID string, status pb.TaskStatus) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	
	entry, exists := tq.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}
	
	entry.Status = status
	
	// If task failed or cancelled, put it back in queue
	if status == pb.TaskStatus_FAILED {
		if entry.index == -1 {  // Not in heap
			heap.Push(&tq.pq, entry)
		}
	}
	
	return nil
}

// GetStatus returns task status
func (tq *TaskQueue) GetStatus(taskID string) (pb.TaskStatus, error) {
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	
	entry, exists := tq.tasks[taskID]
	if !exists {
		return pb.TaskStatus_PENDING, fmt.Errorf("task %s not found", taskID)
	}
	
	return entry.Status, nil
}

// Remove removes a task from the queue
func (tq *TaskQueue) Remove(taskID string) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	
	entry, exists := tq.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}
	
	if entry.index >= 0 {
		heap.Remove(&tq.pq, entry.index)
	}
	
	delete(tq.tasks, taskID)
	return nil
}

// WaitForTasks blocks until tasks are available
func (tq *TaskQueue) WaitForTasks() {
	tq.mu.Lock()
	for tq.pq.Len() == 0 {
		tq.cond.Wait()
	}
	tq.mu.Unlock()
}

// Size returns the number of pending tasks
func (tq *TaskQueue) Size() int {
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	return tq.pq.Len()
}
```

**Test File:** `go-master/pkg/taskqueue/queue_test.go`

```go
package taskqueue

import (
	"testing"
	
	pb "github.com/Codesmith28/CloudAI/pkg/api"
)

func TestEnqueueDequeue(t *testing.T) {
	queue := NewTaskQueue()
	
	task1 := &pb.Task{Id: "task-1", Priority: 5}
	task2 := &pb.Task{Id: "task-2", Priority: 10}
	task3 := &pb.Task{Id: "task-3", Priority: 1}
	
	queue.Enqueue(task1)
	queue.Enqueue(task2)
	queue.Enqueue(task3)
	
	// Dequeue should return highest priority first
	batch := queue.DequeueBatch(2)
	
	if len(batch) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(batch))
	}
	
	if batch[0].Id != "task-2" {
		t.Errorf("Expected task-2 first (priority 10), got %s", batch[0].Id)
	}
	
	if batch[1].Id != "task-1" {
		t.Errorf("Expected task-1 second (priority 5), got %s", batch[1].Id)
	}
}

func TestDeadlinePriority(t *testing.T) {
	queue := NewTaskQueue()
	
	now := time.Now().Unix()
	
	task1 := &pb.Task{Id: "task-1", Priority: 5, DeadlineUnix: now + 3600}
	task2 := &pb.Task{Id: "task-2", Priority: 5, DeadlineUnix: now + 1800}
	
	queue.Enqueue(task1)
	queue.Enqueue(task2)
	
	batch := queue.DequeueBatch(1)
	
	// Earlier deadline should come first
	if batch[0].Id != "task-2" {
		t.Errorf("Expected task-2 with earlier deadline, got %s", batch[0].Id)
	}
}

func TestUpdateStatus(t *testing.T) {
	queue := NewTaskQueue()
	
	task := &pb.Task{Id: "task-1", Priority: 5}
	queue.Enqueue(task)
	
	err := queue.UpdateStatus("task-1", pb.TaskStatus_RUNNING)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	
	status, err := queue.GetStatus("task-1")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	
	if status != pb.TaskStatus_RUNNING {
		t.Errorf("Expected RUNNING status, got %v", status)
	}
}
```

**Deliverables:**
- ✅ Priority queue with deadline awareness
- ✅ Thread-safe operations
- ✅ Status tracking
- ✅ Unit tests with >80% coverage

---

##### Task 1.3: Master API Implementation (4 days)
**Assignee:** Backend Developer 1

**File:** `go-master/pkg/api/api.go`

**Implementation:**

```go
package proto

import (
	"context"
	"fmt"
	"log"
	
	"github.com/Codesmith28/CloudAI/go-master/pkg/taskqueue"
	"github.com/Codesmith28/CloudAI/go-master/pkg/workerregistry"
)

// MasterServer implements SchedulerServiceServer
type MasterServer struct {
	UnimplementedSchedulerServiceServer
	taskQueue *taskqueue.TaskQueue
	registry  *workerregistry.Registry
}

// NewMasterServer creates a new master server
func NewMasterServer(tq *taskqueue.TaskQueue, reg *workerregistry.Registry) *MasterServer {
	return &MasterServer{
		taskQueue: tq,
		registry:  reg,
	}
}

// SubmitTask handles task submission
func (s *MasterServer) SubmitTask(ctx context.Context, req *SubmitTaskRequest) (*SubmitTaskResponse, error) {
	log.Printf("Received task submission: %s (type=%s, cpu=%.2f, mem=%d MB)",
		req.Task.Id, req.Task.TaskType, req.Task.CpuReq, req.Task.MemMb)
	
	// Validate task
	if req.Task.Id == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	if req.Task.CpuReq <= 0 {
		return nil, fmt.Errorf("CPU requirement must be positive")
	}
	if req.Task.MemMb <= 0 {
		return nil, fmt.Errorf("memory requirement must be positive")
	}
	
	// Enqueue task
	err := s.taskQueue.Enqueue(req.Task)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}
	
	return &SubmitTaskResponse{
		TaskId:  req.Task.Id,
		Message: fmt.Sprintf("Task %s accepted and queued", req.Task.Id),
	}, nil
}

// GetTaskStatus retrieves task status
func (s *MasterServer) GetTaskStatus(ctx context.Context, req *GetTaskStatusRequest) (*GetTaskStatusResponse, error) {
	status, err := s.taskQueue.GetStatus(req.TaskId)
	if err != nil {
		return nil, fmt.Errorf("failed to get task status: %w", err)
	}
	
	return &GetTaskStatusResponse{
		TaskId: req.TaskId,
		Status: status,
	}, nil
}

// CancelTask cancels a task
func (s *MasterServer) CancelTask(ctx context.Context, req *CancelTaskRequest) (*CancelTaskResponse, error) {
	log.Printf("Cancelling task: %s", req.TaskId)
	
	err := s.taskQueue.UpdateStatus(req.TaskId, TaskStatus_CANCELLED)
	if err != nil {
		return &CancelTaskResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to cancel task: %v", err),
		}, nil
	}
	
	// TODO: If task is running, send cancellation to worker
	
	return &CancelTaskResponse{
		Success: true,
		Message: fmt.Sprintf("Task %s cancelled", req.TaskId),
	}, nil
}

// RegisterWorker handles worker registration
func (s *MasterServer) RegisterWorker(ctx context.Context, req *RegisterWorkerRequest) (*RegisterWorkerResponse, error) {
	log.Printf("Registering worker: %s (CPU=%.2f, Mem=%d MB, GPU=%d)",
		req.Worker.Id, req.Worker.TotalCpu, req.Worker.TotalMem, req.Worker.Gpus)
	
	// Initialize free resources to match total
	req.Worker.FreeCpu = req.Worker.TotalCpu
	req.Worker.FreeMem = req.Worker.TotalMem
	req.Worker.FreeGpus = req.Worker.Gpus
	
	err := s.registry.UpdateHeartbeat(req.Worker)
	if err != nil {
		return &RegisterWorkerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to register worker: %v", err),
		}, nil
	}
	
	return &RegisterWorkerResponse{
		Success: true,
		Message: fmt.Sprintf("Worker %s registered successfully", req.Worker.Id),
	}, nil
}

// Heartbeat handles worker heartbeats
func (s *MasterServer) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	err := s.registry.UpdateHeartbeat(req.Worker)
	if err != nil {
		log.Printf("Heartbeat error for worker %s: %v", req.Worker.Id, err)
		return &HeartbeatResponse{Success: false}, nil
	}
	
	// TODO: Check if there are tasks to cancel
	
	return &HeartbeatResponse{
		Success:        true,
		TasksToCancel: []string{},
	}, nil
}

// ListWorkers returns all active workers
func (s *MasterServer) ListWorkers(ctx context.Context, req *ListWorkersRequest) (*ListWorkersResponse, error) {
	workers := s.registry.GetSnapshot()
	
	return &ListWorkersResponse{
		Workers: workers,
	}, nil
}

// ReportTaskCompletion handles task completion reports
func (s *MasterServer) ReportTaskCompletion(ctx context.Context, req *TaskCompletionRequest) (*TaskCompletionResponse, error) {
	log.Printf("Task %s completed on worker %s: status=%v, duration=%ds",
		req.TaskId, req.WorkerId, req.Status, req.ActualDurationSec)
	
	// Update task status
	err := s.taskQueue.UpdateStatus(req.TaskId, req.Status)
	if err != nil {
		log.Printf("Error updating task status: %v", err)
	}
	
	// Release worker resources
	err = s.registry.Release(req.TaskId)
	if err != nil {
		log.Printf("Error releasing resources: %v", err)
	}
	
	// TODO: Send to predictor for training
	
	return &TaskCompletionResponse{
		Acknowledged: true,
	}, nil
}
```

**File:** `go-master/cmd/master/main.go`

```go
package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	pb "github.com/Codesmith28/CloudAI/pkg/api"
	"github.com/Codesmith28/CloudAI/go-master/pkg/taskqueue"
	"github.com/Codesmith28/CloudAI/go-master/pkg/workerregistry"
	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting CloudAI Master Node...")
	
	// Initialize components
	taskQueue := taskqueue.NewTaskQueue()
	registry := workerregistry.NewRegistry()
	
	// Create gRPC server
	grpcServer := grpc.NewServer()
	masterServer := pb.NewMasterServer(taskQueue, registry)
	pb.RegisterSchedulerServiceServer(grpcServer, masterServer)
	
	// Start listener
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen on port 50051: %v", err)
	}
	
	// Start cleanup goroutines
	go cleanupLoop(registry)
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Shutting down gracefully...")
		grpcServer.GracefulStop()
	}()
	
	log.Println("Master server listening on port 50051...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func cleanupLoop(registry *workerregistry.Registry) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		registry.CleanupStaleWorkers(30 * time.Second)
		registry.CleanupExpiredReservations()
	}
}
```

**Deliverables:**
- ✅ Master gRPC server implementation
- ✅ Task submission endpoint
- ✅ Worker registration and heartbeat handlers
- ✅ Main entry point with graceful shutdown

---

#### Sprint 1 Acceptance Criteria
- [ ] Worker registry tracks workers and manages resources
- [ ] Task queue orders tasks by priority and deadline
- [ ] Master accepts task submissions via gRPC
- [ ] Master accepts worker heartbeats
- [ ] All components have >70% test coverage
- [ ] Integration test: submit task -> appears in queue

#### Sprint 1 Demo
- Start master server
- Submit tasks via grpcurl
- Register workers via grpcurl
- Show tasks in queue (logs)
- Show workers in registry (ListWorkers API)

---

### Sprint 2 — Greedy Scheduler + Worker Stub + Basic Execution (2 weeks)
**Sprint Goal:** Implement end-to-end task execution with a simple greedy scheduler

This sprint will be detailed in the next section...

---

*[Sprint 2-13 will continue with similar level of detail. Would you like me to continue with the remaining sprints?]*

---

## Planner Algorithms Roadmap

1. Code compiles, unit tests pass, basic integration tests pass in CI.
2. Linting & formatting (gofmt/black, golangci-lint/flake8) pass.
3. Architecture doc updated for any structural changes.
4. Demoable artifact for the sprint goal (script + test harness).

---

# Top-level repo layout (suggested)

```
/proto                     # protobuf definitions (shared)
 /scheduler.proto
/go-master                 # Go master & worker agents
 /cmd/master
 /cmd/worker
 /pkg/api
 /pkg/scheduler
 /pkg/workerregistry
 /pkg/taskqueue
 /pkg/execution
 /pkg/persistence
 /pkg/monitor
 /pkg/vm_manager
 /pkg/container_manager
 /pkg/testbench
/planner_py                # Python planner service
 /planner_server.py
 /planner/
   a_star.py
   or_tools_scheduler.py
   replanner.py
   predictor.py
 /requirements.txt
/docs
/ci                        # CI scripts
```

---

# Shared protobuf (core messages & planner service)

Create `proto/scheduler.proto` and generate for Go & Python.

Example (short):

```proto
syntax = "proto3";
package scheduler;

message Task {
  string id = 1;
  double cpu_req = 2;
  int32 mem_mb = 3;
  int32 gpu_req = 4;
  string task_type = 5;
  int64 estimated_sec = 6;
  int32 priority = 7;
  int64 deadline_unix = 8; // 0 if none
  map<string,string> meta = 9;
}

message Worker {
  string id = 1;
  double total_cpu = 2;
  int32 total_mem = 3;
  int32 gpus = 4;
  repeated string labels = 5;
  double free_cpu = 6;
  int32 free_mem = 7;
  int32 free_gpus = 8;
  int64 last_seen_unix = 9;
}

message PlanRequest {
  repeated Task tasks = 1;
  repeated Worker workers = 2;
  // optional objectives etc
  double planning_time_budget_sec = 3;
}

message Assignment { string task_id = 1; string worker_id = 2; int64 start_unix = 3; int64 est_duration_sec = 4; }

message PlanResponse { repeated Assignment assignments = 1; double cost = 2; string status_message = 3; }

service Planner {
  rpc Plan(PlanRequest) returns (PlanResponse);
}
```

---

# Sprint-by-Sprint Plan (very detailed)

Each sprint shows: **goal**, **user stories**, **detailed subtasks** (code files, functions, algorithmic notes), **tests**, and **acceptance criteria**.

---

## Sprint 0 — Kickoff & infra (2 weeks)

**Goal:** Repo, CI, basic proto, skeleton services, dev onboarding.

**Subtasks**

* Create mono-repo and initialize `go.mod` and Python venv.
* Add `proto/scheduler.proto` (above). Generate Go & Python stubs (protoc + grpc plugins).
* CI skeleton:

  * Go: `go test ./...`, `golangci-lint run`
  * Python: `pytest`, `flake8`, `black --check`
* Create a `docs/architecture.md` with HLD diagram (master ↔ planner ↔ workers).
* Create `Makefile` / `run.sh` to bring up local Docker Compose with empty services.
* Create issue tracker backlog skeleton (epics for "Master", "Planner", "Workers", "Infra", "Tests").

**Files & functions to create**

* `/proto/scheduler.proto`
* `/go-master/cmd/master/main.go` (empty server skeleton)
* `/planner_py/planner_server.py` (gRPC server skeleton that returns a trivial plan)

**Tests**

* CI runs compile and linter.

**Acceptance**

* Repo initialised, proto generation works for both languages, CI green for skeleton tests.

---

## Sprint 1 — Core models, Worker Registry, TaskQueue, API (2 weeks)

**Goal:** Implement domain models in Go, worker registry, task queue, basic master gRPC endpoints to accept tasks and heartbeats.

**Subtasks**

* Implement Go structs:

  * `/go-master/pkg/models/task.go` → `type Task struct { ... }`
  * `/go-master/pkg/models/worker.go` → `type Worker struct { ... }`
* Implement `pkg/workerregistry`:

  * `func NewRegistry() *Registry`
  * `func (r *Registry) UpdateHeartbeat(ctx, hb *pb.Heartbeat)` (accepts Worker message)
  * `func (r *Registry) GetSnapshot() map[string]Worker`
  * Persist last-seen & capability in BoltDB (`pkg/persistence/registry.go`)
* Implement `pkg/taskqueue`:

  * `Enqueue(task models.Task) error`
  * `PeekBatch(n int) []Task`
  * `DequeueByID(id string)` etc.
* API handlers in master:

  * `SubmitTask(ctx, req)` → enqueue + ack
  * `WorkerHeartbeat(ctx, req)` → call Registry.UpdateHeartbeat
* Implement internal event bus for registry updates: `Subscribe()` returns channel.

**Tests**

* Unit tests for registry heartbeats and task queue ordering.
* Integration test: submit tasks -> queue contains them.

**Acceptance**

* Master can accept tasks and worker heartbeats; registry shows workers; tasks persist to DB.

---

## Sprint 2 — Greedy Scheduler + Execution stub + Testbench (2 weeks)

**Goal:** Build simple working pipeline: tasks scheduled by greedy algorithm and executed by worker stub. Create a test harness to measure baseline metrics.

**Subtasks**

* `pkg/scheduler`:

  * `func (s *Scheduler) Start(ctx)`: main event loop
  * `func (s *Scheduler) scheduleOne(task Task) (workerID string, err error)` (greedy best-fit: choose worker with enough resources and maximum free_cpu to reduce fragmentation)
* `pkg/execution`:

  * `func AssignTaskToWorker(task Task, worker Worker) error` → sends gRPC to worker agent (initially a stub) or calls stubbed local function
* `cmd/worker` stub:

  * `AssignTask` handler: print logs, sleep `task.EstimatedSec`, then send completion to master
* `pkg/testbench`:

  * script to spawn N worker stubs with varied capacities and submit M tasks; collect metrics: makespan, avg utilization, deadline misses.
* Add fallback timeout: if `AssignTask` does not ack in `X` seconds, mark worker problematic.

**Tests**

* Integration test: run small workload and ensure all tasks finish.

**Acceptance**

* End-to-end loop works; baseline metrics collected.

---

## Sprint 3 — Python Planner v1: A* Forward Planner (2 weeks)

**Goal:** Create Python planner service implementing a forward A* search over assignment states for small batches (N ≤ 20). Expose via gRPC `Plan`.

**Subtasks**

* Planner server skeleton: `/planner_py/planner_server.py`

  * `class PlannerServicer(planner_pb2_grpc.PlannerServicer)` with `Plan()` implemented.
* Implement `planner/a_star.py`:

  * `def plan(tasks: List[Task], workers: List[Worker], time_budget: float) -> PlanResponse`
  * **State representation:** bitmask or tuple `(assigned_task_ids, worker_free_resources)`.
  * **Action:** assign one unscheduled task to one worker that fits.
  * **Heuristic:** `h = sum(remaining_est_runtime) / max_total_cpu` (fast admissible lower bound) + deadline penalties.
  * **Task selection strategy:** pick task with largest CPU requirement or earliest deadline (try both as options).
  * Respect `time_budget`: return best-found plan on timeout.
* Add PDDL/STRIPS prototype if desired: small module to emit PDDL for debugging.
* Integrate planner server with gRPC: parse incoming pb messages -> plan -> return PlanResponse.

**Tests**

* Unit tests of `a_star.plan()` with toy problems.
* Planner integration test: Go master calls planner (mock) and receives plan.

**Acceptance**

* Planner returns valid plans for small batches; respects resource constraints; Go master can call planner and enact response.

---

## Sprint 4 — Planner Integration + Reservation & Enactment (2 weeks)

**Goal:** Master calls planner regularly; implement plan enactment with reservations to avoid races.

**Subtasks**

* Go master:

  * `pkg/scheduler` adds `PlanCoordinator`:

    * `func (pc *PlanCoordinator) RequestPlan(ctx) (Plan, error)` → marshals tasks+workers to pb and calls planner gRPC.
    * `func (pc *PlanCoordinator) EnactPlan(plan PlanResponse)`:

      * Reserve resources in registry (`Reserve(taskID, workerID, ttl)`).
      * Call `execution.AssignTaskToWorker` for each assignment.
  * Implement reservation DB table: `reservations(taskID->workerID, expiry)`.
  * Implement atomic assignment handshake: master sends Assign RPC and waits for `Ack` before committing reservation.
* Planner:

  * Add `plan_id` field in response for tracking.
* Worker:

  * Respond to Assign with ack containing `accepted` boolean (reject if cannot satisfy due to local drift).
* Conflict handling: if worker rejects, master marks the assignment failed and re-invokes planner for the remaining tasks.

**Tests**

* Simulate race: two planners (concurrent requests) — verify reservations prevent double-assignment.

**Acceptance**

* Planner → master → reservations → worker ack handshake works reliably.

---

## Sprint 5 — Worker Agent v1: Container Execution & Resource Monitoring (2 weeks)

**Goal:** Replace stubs with real container execution using Docker (or containerd) in worker agents; heartbeats include real resource snapshots.

**Subtasks**

* `/go-master/pkg/container_manager`:

  * `func LaunchContainer(task Task) (containerID string, err error)` (using Docker SDK).
  * Support CPU/memory limits.
* `cmd/worker`:

  * `AssignTask` handler now `LaunchContainer` and stream logs; on completion, send `TaskComplete` event to master.
  * `SampleResources()` using `gopsutil` reporting CPU% and mem usage.
  * Heartbeat includes `free_cpu`, `free_mem`, `running_tasks`.
* Add resource accounting: when container launched, update `FreeCPU/FreeMem` in registry (optimistic) and verify on heartbeat (correct drift).
* Master enforces `max_concurrent_tasks` per worker based on CPU budget.

**Tests**

* Integration: real containers run in testbench; monitor utilization.

**Acceptance**

* Workers can run tasks in containers, report resource usage, and master uses that to plan.

---

## Sprint 6 — Planner v2: Incremental Replanning & Plan Repair (2 weeks)

**Goal:** Make the planner and master robust to worker failures by implementing plan repair / incremental replanning.

**Subtasks**

* Python `planner/replanner.py`:

  * Two strategies:

    1. **Plan Repair**: when event (worker down) arrives, mark affected tasks unscheduled and re-run A* on that subset (fast).
    2. **Incremental Search (lightweight)**: reuse previous good partial plan as initial solution; only search for replacements for failed assignments.
  * `def repair(prev_plan, events, time_budget) -> PlanResponse`
* Go master:

  * Fault detector watches heartbeats; on `worker_down`:

    * Mark running tasks on that worker as `INCOMPLETE`.
    * Push `RepairRequest` to planner (RPC `Plan` with current state and previous plan id).
* Planner must accept optional `previous_plan` id and try to reuse it.
* Add plan-change diff protocol so master can apply minimal reassignments.

**Tests**

* Inject worker down event in testbench, assert quick replan and no double execution.

**Acceptance**

* Planner + master recover from worker failure within defined SLA (e.g., replan for affected tasks < 3s for small sets).

---

## Sprint 7 — Temporal Scheduling & Deadlines (2 weeks)

**Goal:** Add durations/deadlines. Move to OR-Tools CP-SAT solver for temporal/slot scheduling for medium-size batches.

**Subtasks**

* Python `planner/or_tools_scheduler.py`:

  * Formulate assignment as CP-SAT job-shop / resource-constrained scheduling:

    * Decision variables: `assign_task_to_worker[task,worker] ∈ {0,1}`
    * Start time variable `start[task]` with domain `[now, now + horizon]`
    * Resource capacity constraints per worker over time slices (discretize or use cumulatives)
    * Objective: minimize weighted sum of lateness + makespan + resource fragmentation
  * Use `time_budget` to limit solver runtime and return best feasible solution.
* Planner API extended: `PlanRequest` includes `time_horizon_sec` and `objectives`.
* Go master handles scheduled start times: `AssignRequest` includes `start_unix` and master enforces start (worker will not run before start).
* Scheduler supports `delayed assignments` (reservation until start).

**Tests**

* Temporal benchmark: mix of tasks with tight deadlines — planner must honor deadlines where feasible.

**Acceptance**

* Planner successfully returns temporal plans; master enacts delayed starts; deadline misses reduced vs greedy baseline.

---

## Sprint 8 — Checkpointing, Preemption & Reservations (2 weeks)

**Goal:** Add checkpointing mechanics for long tasks and safe preemption.

**Subtasks**

* Decide checkpoint approach by task type: if containerized apps support application-level checkpoint (CRIU) or implement periodic state dump (if internal to tasks).
* Worker agent:

  * `CheckpointTask(taskID) -> checkpointID` (store snapshot to shared storage).
  * `ResumeTask(taskID, checkpointID)` support.
* Master:

  * Preemption API: `Preempt(taskID, reason)` requests worker to checkpoint, stop, and return `checkpointID` to master.
  * Planner can schedule `Resume` on different worker.
* Planner:

  * Must be checkpoint-aware; treat checkpointable tasks as restartable with cost of resume.
* Tests:

  * Simulate long task preemption and resume on another worker; measure overhead.

**Acceptance**

* Checkpoint+resume works for at least one sample task type; preemption reduces time-to-failover.

---

## Sprint 9 — Runtime Predictor & Heuristic Learning (2 weeks)

**Goal:** Improve planning quality using runtime prediction models.

**Subtasks**

* Python `planner/predictor.py`:

  * Implement simple training pipeline (scikit-learn) for `predict_runtime(task_type, worker_profile)`.
  * Model persists to disk (`joblib`).
  * `Predictor.predict(task, worker)` used by planner heuristics & cost functions.
* Master:

  * After task completion, send telemetry to planner: actual duration, resource usage.
* Planner:

  * Use predictor in A* heuristics and CP-SAT estimated durations.
* Tests:

  * Train on synthetic data, validate reduced prediction error.

**Acceptance**

* Predictor improves heuristic accuracy and improves plan cost in benchmarks.

---

## Sprint 10 — VM / KVM Integration & Hardware Heterogeneity (2 weeks)

**Goal:** Add option to run tasks in KVM VMs for stronger isolation and heterogeneous hardware (GPU passthrough in lab).

**Subtasks**

* Implement `/go-master/pkg/vm_manager` (libvirt-go wrapper):

  * `func CreateVM(spec VMSpec) (vmID string, err error)`
  * `func RunCommandInVM(vmID, image, cmd) error` (or run container inside VM)
* Worker agent:

  * Support `run_in_vm` vs `run_in_container` based on assignment.
* Planner:

  * Include `worker_type` labels (e.g., `gpu=true`, `arch=arm`) and match task requirements.
* Test on dev machine (note: CI skip if KVM not available).

**Acceptance**

* Planner can assign to VM workers and KVM-based worker can execute a CPU-bound task.

---

## Sprint 11 — Scale & Optimizations (2 weeks)

**Goal:** Make planner and master scale: pruning, symmetry-breaking, batch strategies, and fallback policies.

**Subtasks**

* Planner:

  * Implement symmetry reduction: detect identical worker classes and treat them as groups to reduce branching.
  * Implement heuristic time budget enforcement and graceful degrade to greedy if budget exceeded.
  * Add caching / transposition table to A*.
* Master:

  * Implement batching policy for planner calls (e.g., trigger planner when pending tasks >= K or periodic).
  * Implement `PlannerHealthMonitor` to fallback to greedy scheduler if planner unhealthy or exceeded time.
* Tests:

  * Scale tests: 500 tasks, 100 workers (simulated) measuring plan latency.
* Acceptance:

  * Planner produces acceptable plans within configured time budget (e.g., 5s for medium batch); master can fallback.

---

## Sprint 12 — Observability, Security & Deployment (2 weeks)

**Goal:** Add Prometheus metrics, tracing, secure gRPC, Helm manifests / Docker Compose for demo cluster.

**Subtasks**

* Master & worker:

  * Expose `/metrics` (Prometheus) with key metrics: `plan_latency_seconds`, `assignments_total`, `deadline_misses_total`, `planner_fallbacks_total`.
  * Add OpenTelemetry traces for Plan→Enact→Execute flows.
* gRPC security:

  * Enable mTLS between master↔planner and master↔workers.
* Create Docker images and Helm chart: `master`, `worker`, `planner`.
* Add runbooks for operations (scaling workers, recovering master).
* Tests:

  * Integration tests in a Kubernetes test cluster (minikube or kind).
* Acceptance:

  * Metrics available; planner and master talk via secured gRPC; deployment manifests working.

---

## Sprint 13 — Final polish, benchmarking & release (2 weeks)

**Goal:** Polish, run final benchmarks vs Kubernetes-simulated baseline, produce release artifacts and handover docs.

**Subtasks**

* Final bug fixes & performance tuning.
* Benchmark suite:

  * Compare our system vs a simple simulated Kubernetes scheduler on identical workloads (makespan, util, deadline miss).
  * Produce plots & a results report.
* Document API, PDDL examples, planner knobs, and ops guide.
* Build release: docker images, binary artifacts, Helm chart, `docs/release-notes.md`.
* Demo & stakeholder walkthrough.

**Acceptance**

* MVP release artifacts published; benchmark report showing improvement vs baseline or clear explanation of trade-offs.

---

# Planner Algorithms: clear roadmap & why/when to use them

1. **Sprint 3 — A*** forward search (python `a_star.py`): quick to implement, good for small batches and exactness. Heuristics: relaxed estimates, runtime-based lower bounds.
2. **Sprint 6 — Plan Repair / Incremental**: reuse previous plan to quickly adapt to worker failure; far faster than full replan.
3. **Sprint 7 — OR-Tools CP-SAT (temporal)**: for temporal constraints, deadlines, and medium-sized problems — convert to job-shop/resource constrained model; use solver time budgets.
4. **Sprint 11 — Symmetry breaking & grouping**: prune search tree by collapsing identical workers.
5. (Post-MVP) **Hybrid ML/RL**: if workloads repeat, learn policies for specific patterns.

---

# Concrete function names & file locations (copy-paste ready checklist)

**Go (master)**

* `go-master/pkg/scheduler/scheduler.go`

  * `func NewScheduler(reg WorkerRegistry, tq TaskQueue, plannerClient PlannerClient) *Scheduler`
  * `func (s *Scheduler) Start(ctx context.Context) error`
  * `func (s *Scheduler) scheduleLoop(ctx context.Context)`
  * `func (s *Scheduler) callPlanner(ctx context.Context, batch []Task) (*pb.PlanResponse, error)`

* `go-master/pkg/workerregistry/registry.go`

  * `func (r *Registry) UpdateHeartbeat(w *pb.Worker)`
  * `func (r *Registry) Reserve(taskID string, workerID string, ttl time.Duration) error`

* `go-master/pkg/execution/executor.go`

  * `func (e *Executor) AssignTask(ctx context.Context, task models.Task, workerID string) error`
  * `func (e *Executor) CancelTask(ctx context.Context, taskID string) error`

**Python (planner)**

* `planner_py/planner_server.py`

  * `class PlannerServicer(planner_pb2_grpc.PlannerServicer):`

    * `def Plan(self, request, context) -> PlanResponse:`
* `planner_py/planner/a_star.py`

  * `def plan(tasks, workers, time_budget_sec) -> PlanResponse:`
* `planner_py/planner/or_tools_scheduler.py`

  * `def plan_cp_sat(tasks, workers, horizon_sec, time_budget_sec) -> PlanResponse:`
* `planner_py/planner/replanner.py`

  * `def repair(prev_plan, events, time_budget_sec) -> PlanResponse:`
* `planner_py/planner/predictor.py`

  * `class Predictor: predict(task, worker) -> float; update(task, actual)`

---

# Testing strategy (must run each sprint)

* Unit tests (Go & Python) for core logic.
* Planner-specific tests:

  * deterministic toy problems for A* & CP-SAT.
  * time-budget and fallback behavior tests.
* Integration tests:

  * Docker Compose scenario: master + 3 workers + planner + testbench.
* Performance tests:

  * Run nightly with varying sizes (small, medium, large).
* Acceptance tests:

  * End-to-end scenario scripts that act as a demo.

---

# Learning map (AIMA + tools)

* **AIMA chapters to read** (order):

  * Ch.2 Agents (agent architectures) — maps to master→planner→worker pattern.
  * Search chapters (A* and heuristics) — implement `planner/a_star.py`.
  * Ch.10 Classical Planning (STRIPS / PDDL) — good for formalization & PDDL prototypes.
  * Ch.11 Planning & Acting — replanning, partial-order, temporal issues.
* **Python libraries**:

  * `pyperplan` (toy planning & heuristics) or custom A* for full control.
  * `ortools` (Job-shop / CP-SAT) for temporal scheduling.
  * `scikit-learn` for predictor.

---

# Immediate next actions (what you can do this afternoon)

1. Create repository and push `proto/scheduler.proto` (copy provided above).
2. Run `protoc` to generate Go & Python stubs.
3. Bootstrap Go master skeleton (`main.go`) and Python planner skeleton (`planner_server.py`) using generated stubs.
4. Implement `WorkerRegistry` and `TaskQueue` simple in-memory versions so you can demo submit → master queue.
5. Implement a trivial python planner that returns a round-robin mapping so you can test gRPC end-to-end.
6. Add a simple Makefile + Docker Compose to run master + planner + a few worker stubs.

If you want, I can produce the **exact starter files** now:

* `proto/scheduler.proto` (full)
* Go `main.go` + registry skeleton
* Python `planner_server.py` (gRPC stub)
  Tell me if you want those generated immediately and I’ll output them (copy-paste friendly).

---

If you want, next I can:

* Export this plan as a **Jira/CSV backlog** (epics/stories/tasks with estimates), or
* Generate starter code for **Sprint 0–2** (Go master skeleton, Python planner stub, Docker Compose) so devs can begin right away.

Which would you like me to do next?
