# Low-Level Design (LLD) - AI-Driven Agentic Scheduler

This document provides the Low-Level Design (LLD) of the system.  
It details the modules, key functions, and how they interact.

---

## 1. Master Node (Go)

### 1.1 Entry Points
- **File**: `go-master/cmd/master/main.go`
- **Responsibility**:  
  - Bootstraps the Master service.
  - Initializes registry, queue, scheduler, executor, persistence.
  - Starts gRPC server for client submissions and worker heartbeats.

---

### 1.2 API Layer
- **File**: `pkg/api/api.go`
- **Responsibility**: Exposes gRPC/REST APIs.
- **Functions**:
  - `SubmitTask(task Task) -> Ack`
  - `WorkerHeartbeat(worker Worker) -> Ack`
  - `ReportTaskCompletion(taskID string, status string) -> Ack`

---

### 1.3 Task Queue
- **File**: `pkg/taskqueue/queue.go`
- **Responsibility**: In-memory + persistent queue for tasks.
- **Functions**:
  - `Enqueue(task Task) error`
  - `DequeueBatch(n int) []Task`
  - `PeekPending() []Task`

---

### 1.4 Worker Registry
- **File**: `pkg/workerregistry/registry.go`
- **Responsibility**: Tracks workers and their resources.
- **Functions**:
  - `UpdateHeartbeat(worker Worker)`
  - `GetSnapshot() []Worker`
  - `ReserveResources(workerID string, task Task) error`
  - `ReleaseResources(workerID string, taskID string)`

---

### 1.5 Scheduler Interface
- **File**: `pkg/scheduler/scheduler.go`
- **Responsibility**: Coordinates with planner service.
- **Functions**:
  - `RequestPlan(tasks []Task, workers []Worker) -> PlanResponse`
  - `EnactPlan(plan PlanResponse)`
  - `FallbackGreedy(tasks, workers)`

---

### 1.6 Execution Engine
- **File**: `pkg/execution/executor.go`
- **Responsibility**: Dispatches tasks to workers.
- **Functions**:
  - `AssignTaskToWorker(task Task, worker Worker) error`
  - `CancelTask(taskID string)`
  - `HandleAck(worker Worker, taskID string)`

---

### 1.7 Persistence Layer
- **File**: `pkg/persistence/persistence.go`
- **Responsibility**: Stores tasks, workers, plans.
- **Functions**:
  - `SaveTask(task Task)`
  - `SaveWorker(worker Worker)`
  - `SavePlan(plan PlanResponse)`

---

### 1.8 Monitoring
- **File**: `pkg/monitor/monitor.go`
- **Responsibility**: Fault detection and metrics.
- **Functions**:
  - `DetectWorkerFailure(workerID string)`
  - `CollectMetrics()`
  - `TriggerReplanOnFailure(workerID string)`

---

### 1.9 Container Manager
- **File**: `pkg/container_manager/container.go`
- **Responsibility**: Run tasks in Docker containers.
- **Functions**:
  - `LaunchContainer(task Task) -> containerID`
  - `StopContainer(containerID string)`

---

### 1.10 VM Manager
- **File**: `pkg/vm_manager/vm.go`
- **Responsibility**: Run tasks inside KVM/Xen VMs.
- **Functions**:
  - `CreateVM(task Task) -> vmID`
  - `DestroyVM(vmID string)`

---

### 1.11 Testbench
- **File**: `pkg/testbench/testbench.go`
- **Responsibility**: Simulates workloads for testing.
- **Functions**:
  - `GenerateTasks(n int)`
  - `SimulateWorkers(n int)`
  - `RunBenchmark()`

---

## 2. Worker Node (Go)

### 2.1 Entry Point
- **File**: `go-master/cmd/worker/main.go`
- **Responsibility**: Starts worker agent, registers with master, executes tasks.

### 2.2 Worker Functions
- **Functions**:
  - `ReceiveAssignment(task Task)`
  - `ExecuteTask(task Task)`
  - `SendHeartbeat()`
  - `ReportCompletion(taskID string, status string)`

---

## 3. Planner Service (Python)

### 3.1 Entry Point
- **File**: `planner_py/planner_server.py`
- **Responsibility**: gRPC service exposing planning.
- **Functions**:
  - `Plan(PlanRequest) -> PlanResponse`

---

### 3.2 Planner Modules

#### 3.2.1 A* Planner
- **File**: `planner/a_star.py`
- **Responsibility**: Forward state-space planning with heuristics.
- **Functions**:
  - `plan(tasks, workers, time_budget) -> PlanResponse`
- **Algorithm**: A* with heuristic = total remaining runtime / max available CPU.

---

#### 3.2.2 OR-Tools Scheduler
- **File**: `planner/or_tools_scheduler.py`
- **Responsibility**: Constraint-based scheduling with deadlines/durations.
- **Functions**:
  - `plan_cp_sat(tasks, workers, horizon) -> PlanResponse`
- **Algorithm**: CP-SAT job-shop scheduling.

---

#### 3.2.3 Replanner
- **File**: `planner/replanner.py`
- **Responsibility**: Incremental plan repair.
- **Functions**:
  - `repair(prev_plan, failed_tasks, workers) -> PlanResponse`
- **Algorithm**: Plan repair or partial replanning.

---

#### 3.2.4 Predictor
- **File**: `planner/predictor.py`
- **Responsibility**: Predict runtime based on task type + worker type.
- **Functions**:
  - `predict(task, worker) -> float`
  - `update(task, actual_runtime)`

---

## 4. Data Flow (Detailed)

1. **Client → Master**
   - Submits task → stored in `taskqueue`.

2. **Master → Planner**
   - Scheduler calls `RequestPlan` with current tasks + worker snapshot.
   - Planner computes schedule and returns `PlanResponse`.

3. **Master → Worker**
   - Executor dispatches assignments.
   - Worker runs container/VM task.

4. **Worker → Master**
   - Sends completion/failure report.
   - Sends heartbeats.

5. **Master → Planner (on failure)**
   - Monitoring detects failure.
   - Sends replan request with failed tasks.
   - Planner returns updated plan.

---

## 5. Interaction Summary

- **Master**
  - Talks to **Planner** (gRPC).
  - Talks to **Workers** (gRPC).
  - Uses **Persistence** for state saving.
  - Uses **Monitor** for failure detection.

- **Planner**
  - Purely reactive to Master requests.
  - Uses algorithms (A*, CP-SAT, Replanner).
  - Consults **Predictor** for runtime estimates.

- **Workers**
  - Passive executors.
  - Report resource usage and task results.

---
