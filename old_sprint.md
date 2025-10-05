Great — I’ll act as Scrum Master and give you a **complete, production-oriented sprint plan** for the hybrid system: **Go** for the master + workers, **Python** for the agentic planner, with **gRPC** between them. This plan is step-by-step, lists concrete files / function names, algorithms to implement in each sprint, tests, acceptance criteria, and immediate next actions so your team can start coding today.

---

# Assumptions (so plan is actionable)

* Sprint length = **2 weeks**. Total plan = **13 sprints** (≈ 26 weeks). Adjust if you have more devs / want shorter schedule.
* Core split:

  * **Go** — master node, worker agent, registry, execution, metrics, persistence.
  * **Python** — agentic scheduler (planner). Uses planning libraries (pyperplan / OR-Tools / custom A*), ML predictors (scikit-learn) later.
* Communication: **gRPC (protobuf)** between Go master and Python planner. Shared proto files.
* Fallback: Go master contains a **greedy scheduler** fallback (must always exist for safety).
* CI: GitHub Actions (or GitLab CI) for both Go & Python. Unit tests + integration tests run in CI.
* Persistence: start with **BoltDB** (Go) for master state; planner keeps ephemeral state (persist planner logs/data to a file/DB if needed).
* Dev infra: Developers run everything locally with Docker Compose (master + worker + planner containers) before deploying to real VMs/KVM.

---

# Definition of Done (applies to every sprint)

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
