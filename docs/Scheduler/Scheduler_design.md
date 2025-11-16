Below is the **final, context-aware Copilot-ready EDD**, adapted to your **current CloudAI implementation** (master/worker, Round-Robin scheduler, telemetry, MongoDB, task queue, gRPC protos).

It keeps:

* Existing **master–worker RPCs** and message types unchanged. 
* Existing **task queue + Round-Robin scheduler** in `master/internal/server` + `master/internal/scheduler`.
* Existing **telemetry and worker registry** with per-worker goroutines and resource tracking. 

And adds:

* **RTS as the primary scheduler**, integrated inside `internal/scheduler`,
* **Round-Robin as a fallback** when RTS cannot decide,
* **AOD/GA** as a new background module under `master/internal/aod`.

You can drop this EDD + the sprint prompt into Copilot.

---

# ================================================================

# CLOUD-AI SCHEDULER — CONTEXT-AWARE COPILOT EDD

# ================================================================

> This EDD extends the existing CloudAI architecture (master/worker, telemetry, DB, Round-Robin scheduler)
> It does **not** change existing proto files, RPCs, or basic master–worker communication.

---

## 0. Integration Strategy (Option B)

* Keep the **current pluggable scheduler interface** in `master/internal/scheduler` with the existing **Round-Robin implementation**. 
* Introduce a new **RTSScheduler** inside the same package.
* Master will **call RTSScheduler first**.
* Inside RTSScheduler, if RTS cannot pick a worker (no feasible workers, missing params, internal error), it **falls back to the existing Round-Robin scheduler**.
* Existing task queue + worker registry + heartbeat processing remain intact.

---

## 1. System Overview

CloudAI already has:

* Master node with:

  * gRPC server (`MasterServer`)
  * **Task queue** and **Round-Robin scheduler**
  * TelemetryManager with **thread-per-worker heartbeats**
  * MongoDB (WORKER_REGISTRY, TASKS, ASSIGNMENTS, RESULTS)
* Worker nodes with:

  * Docker executor
  * Telemetry sender (CPU/MEM/GPU usage)
  * AssignTask RPC handling

We add:

1. **RTS (Real-Time Scheduler)**

   * Runs **inside master**, in `master/internal/scheduler/rts.go`
   * Called from existing **queue processor / AssignTaskToWorker flow**
   * Uses in-memory worker state + GA parameters

2. **AOD (Async Optimizer with GA)**

   * New module `master/internal/aod/`
   * Periodically reads telemetry from MongoDB
   * Computes:

     * θ₁…θ₄ (predictor weights)
     * α, β (RTS risk weights)
     * Affinity[type][worker]
     * Penalty[worker]
     * α₁…₃, γ₁…₃ (affinity/penalty weights)
   * Writes JSON to: **`config/ga_output.json`**

RTS periodically reloads `ga_output.json`.

---

## 2. Data Models (Aligned with Current System)

### 2.1 Protobuf `Task` (unchanged)

From `master_worker.proto`: 

```protobuf
message Task {
  string task_id       = 1;
  string docker_image  = 2;
  string command       = 3;
  double req_cpu       = 4;
  double req_memory    = 5;
  double req_storage   = 6;
  double req_gpu       = 7;
  string target_worker_id = 8;
  string user_id       = 9;
}
```

This stays as-is. RTS works on a **derived view**.

### 2.2 Scheduler-Level `TaskView`

New Go struct (in `master/internal/scheduler/models.go`):

```go
type TaskView struct {
    ID          string
    Type        string    // e.g. "cpu", "gpu", "dl", "mixed" (derived from metadata or labels)
    CPU         float64
    Mem         float64
    GPU         float64
    Storage     float64
    ArrivalTime time.Time
    Tau         float64   // base runtime (learned from telemetry)
    Deadline    time.Time // D_i = ArrivalTime + k * Tau
    UserID      string
}
```

Construction:

```go
func NewTaskViewFromProto(pbTask *pb.Task, now time.Time, tau float64, k float64) TaskView {
    return TaskView{
        ID:          pbTask.TaskId,
        Type:        InferTaskType(pbTask), // simple rules: any GPU -> "gpu"/"dl"
        CPU:         pbTask.ReqCpu,
        Mem:         pbTask.ReqMemory,
        GPU:         pbTask.ReqGpu,
        Storage:     pbTask.ReqStorage,
        ArrivalTime: now,
        Tau:         tau,
        Deadline:    now.Add(time.Duration(k * tau * float64(time.Second))),
        UserID:      pbTask.UserId,
    }
}
```

### 2.3 Worker State View

Master already has:

* In-memory worker registry
* Resource tracking (allocated/available CPU/MEM/GPU/Storage)
* TelemetryManager with per-worker load

We define a **read-only view** used by RTS (in `master/internal/scheduler/models.go`):

```go
type WorkerView struct {
    ID           string
    CPUAvail     float64
    MemAvail     float64
    GPUAvail     float64
    StorageAvail float64
    Load         float64 // normalized; may exceed 1 due to oversubscription
}
```

RTS obtains `[]WorkerView` from a small adapter around:

* `WorkerDB` (total capacities, static) 
* `TelemetryManager` / resource tracking (current usage/allocated, load) 

### 2.4 Telemetry Logical Models

We **reuse existing collections** (`TASKS`, `ASSIGNMENTS`, `RESULTS`, `WORKER_REGISTRY`)  but define clear logical structs:

```go
type TaskHistory struct {
    TaskID        string
    WorkerID      string
    Type          string
    ArrivalTime   time.Time
    Deadline      time.Time
    ActualStart   time.Time
    ActualFinish  time.Time
    ActualRuntime float64
    SLASuccess    bool
    CPUUsed       float64
    MemUsed       float64
    GPUUsed       float64
    LoadAtStart   float64
}
```

```go
type WorkerStats struct {
    WorkerID      string
    TasksRun      int
    SLAViolations int
    TotalRuntime  float64
    CPUUsedTotal  float64
    MemUsedTotal  float64
    GPUUsedTotal  float64
    OverloadTime  float64 // time with Load > 1.0
    TotalTime     float64
}
```

Implementation detail:
These can be **built by queries** over existing collections or materialized into new collections (e.g. `TASK_HISTORY`, `WORKER_STATS`) but that choice is left to the implementation.

---

## 3. RTS (Real-Time Scheduler) — Inside `master/internal/scheduler`

### 3.1 High-Level Behavior

* RTS runs **inside the existing scheduler package**.
* Flow:

  1. Task is queued in `MasterServer` as `QueuedTask`. 
  2. `StartQueueProcessor()` pops a queued task and calls the scheduler. 
  3. Scheduler implementation now is **RTSScheduler**, which:

     * builds `TaskView` and `[]WorkerView`
     * computes scores (`R_{ij}^{final}`)
     * selects best worker
     * if fail → falls back to existing Round-Robin scheduler
  4. Master uses selected worker ID to call existing `AssignTask` RPC to that worker. 

### 3.2 Interface and Structs

Existing interface (from `internal/scheduler`): 

```go
type Scheduler interface {
    SelectWorker(task *pb.Task, workers []WorkerState) (*WorkerState, error)
}
```

We introduce:

```go
type RTSScheduler struct {
    rrScheduler     Scheduler      // existing RoundRobinScheduler
    params          *GAParams      // θ, α, β, affinity, penalty
    paramsMu        sync.RWMutex
    telemetrySource TelemetrySource // adapter to TelemetryManager + WorkerDB
    tauStore        TauStore        // interface to get/update tau by type
}
```

`GAParams` is loaded from `config/ga_output.json` (see §5).

RTSScheduler implements `Scheduler` and is plugged into `MasterServer` in place of pure Round-Robin.

### 3.3 Feasibility Filter

Worker is feasible iff:

[
CPU_j^{avail} \ge CPU_i,\quad
MEM_j^{avail} \ge MEM_i,\quad
GPU_j^{avail} \ge GPU_i,\quad
STR_j^{avail} \ge STR_i
]

Implementation:

```go
func (s *RTSScheduler) filterFeasible(task TaskView, workers []WorkerView) []WorkerView
```

If no feasible workers → return error; `MasterServer` keeps task in queue.

### 3.4 Deadline Computation

Deadline is computed when building `TaskView`:

[
D_i = A_i + k \cdot \tau_i,\quad k \in {1.5, 2.0}
]

`k` is a config (e.g. env var `SCHED_SLA_MULTIPLIER`).

### 3.5 Execution Time Prediction

[
\widehat{E}_{ij} =
\tau_i\left(
1 +
\theta_1\frac{C_i}{C_j^{avail}} +
\theta_2\frac{M_i}{M_j^{avail}} +
\theta_3\frac{G_i}{G_j^{avail}} +
\theta_4 L_j
\right)
]

Implementation:

```go
func (s *RTSScheduler) predictExecTime(t TaskView, w WorkerView, theta Theta) float64
```

### 3.6 Finish Time and SLA Margin

[
\widehat{F}*{ij} = A_i + \widehat{E}*{ij}
]

[
\Delta_{ij} = \max(0,\ \widehat{F}_{ij} - D_i)
]

### 3.7 Base Risk

[
R_{ij} = \alpha \cdot \Delta_{ij} + \beta \cdot L_j
]

Implementation:

```go
func (s *RTSScheduler) computeBaseRisk(
    t TaskView,
    w WorkerView,
    eHat float64,
    alpha, beta float64,
) float64
```

### 3.8 GA-Enhanced Final Risk (Affinity + Penalty)

From GA output:

* `Affinity[type][workerID]` ∈ [-5, +5]
* `Penalty[workerID]` ≥ 0

Final score:

[
R^{final}*{ij} = R*{ij} - Affinity[type_i][w_j] + Penalty(w_j)
]

Implementation:

```go
func (s *RTSScheduler) computeFinalRisk(
    base float64,
    taskType string,
    workerID string,
    params *GAParams,
) float64
```

### 3.9 Selection and Fallback

Algorithm inside `SelectWorker`:

```go
func (s *RTSScheduler) SelectWorker(task *pb.Task, workers []WorkerState) (*WorkerState, error) {
    // 1. Build TaskView (including Tau, Deadline)
    tv := s.buildTaskView(task)

    // 2. Build WorkerView slice from workers + telemetry
    wviews := s.buildWorkerViews(workers)

    // 3. Filter feasible
    feasible := s.filterFeasible(tv, wviews)
    if len(feasible) == 0 {
        // Fallback to existing Round-Robin (which already does resource-aware check)
        return s.rrScheduler.SelectWorker(task, workers)
    }

    // 4. Load GA params
    params := s.getGAParamsSafe()

    // 5. Score each feasible worker
    bestIdx := -1
    bestScore := math.Inf(1)

    for i, wv := range feasible {
        eHat := s.predictExecTime(tv, wv, params.Theta)
        baseRisk := s.computeBaseRisk(tv, wv, eHat, params.Risk.Alpha, params.Risk.Beta)
        finalRisk := s.computeFinalRisk(baseRisk, tv.Type, wv.ID, params)

        if finalRisk < bestScore {
            bestScore = finalRisk
            bestIdx = i
        }
    }

    if bestIdx == -1 || math.IsInf(bestScore, 1) || math.IsNaN(bestScore) {
        // Safety fallback
        return s.rrScheduler.SelectWorker(task, workers)
    }

    // 6. Map best WorkerView back to *WorkerState (existing type in internal/scheduler)
    selected := s.lookupWorkerState(feasible[bestIdx].ID, workers)
    if selected == nil {
        return s.rrScheduler.SelectWorker(task, workers)
    }

    return selected, nil
}
```

So **RTS is primary**, Round-Robin is fallback.

---

## 4. Telemetry and Learning (Using Existing System)

### 4.1 Existing Telemetry

CloudAI already has:

* Heartbeats with CPU/MEM/GPU usage and running tasks.
* TelemetryManager per worker.
* MongoDB collections for WORKER_REGISTRY, TASKS, ASSIGNMENTS, RESULTS.

We add:

1. **Task completion enrichment** in `MasterServer.ReportTaskCompletion`:

   * Compute `ActualRuntime = ActualFinish - ActualStart`.
   * Determine `SLASuccess` by comparing `ActualFinish` vs `Deadline` from TASKS/ASSIGNMENTS.
2. **Worker load sampling**:

   * TelemetryManager tracks `Load` per worker over time and increments `OverloadTime` when `Load > 1.0`.

### 4.2 Updating τᵢ (per task type)

For each type `c`:

[
\tau_c^{new} = \lambda E_{actual} + (1-\lambda)\tau_c^{old},\quad \lambda \approx 0.2
]

Implementation (e.g. in `master/internal/telemetry/tau_store.go`):

```go
type TauStore interface {
    GetTau(taskType string) float64
    UpdateTau(taskType string, actualRuntime float64)
}
```

### 4.3 Updating θ₁…θ₄

We perform a simple regression offline (inside AOD):

Minimize:

[
\sum (E_{actual} - \widehat{E})^2
]

Using features:

* CPU ratio = Cᵢ / Cⱼ⁽ᵃᵛᵃᶦˡ⁾
* MEM ratio = Mᵢ / Mⱼ⁽ᵃᵛᵃᶦˡ⁾
* GPU ratio = Gᵢ / Gⱼ⁽ᵃᵛᵃᶦˡ⁾
* Load = Lⱼ at start

Implementation details are in §5 (AOD); RTS just reads θ from GAParams.

---

## 5. GA / AOD — New `master/internal/aod` Module

### 5.1 Role

* Runs every 30–60 seconds (ticker in `main.go` or dedicated goroutine).

* Pulls `TaskHistory` and `WorkerStats` from MongoDB.

* Computes:

  * θ₁…θ₄ (Theta)
  * α, β (RTS risk weights)
  * Affinity[type][worker]
  * Penalty[worker]
  * α₁…₃ (affinity weights)
  * γ₁…₃ (penalty weights)

* Writes to `config/ga_output.json`.

### 5.2 GAParams Struct

```go
type Theta struct {
    Theta1 float64
    Theta2 float64
    Theta3 float64
    Theta4 float64
}

type Risk struct {
    Alpha float64
    Beta  float64
}

type AffinityWeights struct {
    A1 float64
    A2 float64
    A3 float64
}

type PenaltyWeights struct {
    G1 float64
    G2 float64
    G3 float64
}

type GAParams struct {
    Theta          Theta
    Risk           Risk
    AffinityW      AffinityWeights
    PenaltyW       PenaltyWeights
    AffinityMatrix map[string]map[string]float64 // type->workerID->score
    PenaltyVector  map[string]float64           // workerID->score
}
```

### 5.3 Affinity(c, j)

For task type `c` and worker `j`:

1. Baseline:

[
baseline(c) = \text{mean runtime of type }c
]

[
speed(c,j) = \frac{baseline(c)}{mean_runtime(c,j)}
]

2. SLA reliability:

[
SLAok(c,j) = 1 - \frac{violations(c,j)}{tasks(c,j)+\epsilon}
]

3. Overload rate (per (c,j)):

[
OverloadRate(c,j) = \frac{\text{time }L_j > 1 \text{ when running type }c}{\text{total time running type }c+\epsilon}
]

4. Raw affinity:

[
Affinity^{raw}(c,j) =
\alpha_1 speed(c,j)

* \alpha_2 SLAok(c,j)

- \alpha_3 OverloadRate(c,j)
  ]

5. Clip:

[
Affinity[c][j] = clip(Affinity^{raw}(c,j), -5, +5)
]

### 5.4 Penalty(j)

[
SLAFailRate(j) = \frac{\text{violations}(j)}{\text{tasks}(j)+\epsilon}
]

[
OverloadRate(j) = \frac{\text{time }L_j > 1}{\text{total time}+\epsilon}
]

Energy proxy for worker j over window:

[
E_j = a\cdot CPU^{used}_j + b\cdot MEM^{used}_j + c\cdot GPU^{used}_j
]

Normalize:

[
EnergyNorm(j) = \frac{E_j}{E_{ref,j}}
]

Penalty:

[
Penalty(j) =
\gamma_1 SLAFailRate(j)

* \gamma_2 OverloadRate(j)
* \gamma_3 EnergyNorm(j)
  ]

### 5.5 GA Fitness Function

For a candidate configuration:

[
Fitness =
w_1 SLA_Success

* w_2 Utilization

- w_3 EnergyNorm
- w_4 OverloadNorm
  ]

Where:

* **SLA_Success**:

[
SLA_Success = 1 - \frac{violations}{tasks}
]

* **Utilization**:

[
U_j = \frac{CPU^{used}_j + MEM^{used}_j + GPU^{used}_j}{CPU^{tot}_j + MEM^{tot}_j + GPU^{tot}_j}
]

[
Utilization = mean_j(U_j)
]

* **EnergyNorm**:

[
EnergyNorm = \frac{\sum_j E_j}{E_{ref}}
]

* **OverloadNorm**:

[
OverloadNorm = \frac{\sum_j Overload_j}{Overload_{ref}},\quad Overload_j = \max(0,L_j - 1.0)
]

GA evolves:

* θ
* Risk (α, β)
* AffinityW (α₁..₃)
* PenaltyW (γ₁..₃)

and, indirectly, Affinity and Penalty.

---

## 6. Weight Justification Rules (Same Logic, Now Contextual)

* θ₁…θ₄: trained via regression on `TaskHistory`.
* α, β:

  * Initial: α = 10, β = 1
  * SLA miss by 1s ≈ 10× cost of sending to heavily loaded node.
* α₁…₃:

  * α₂ (SLA) > α₁ (speed) > α₃ (overload).
* γ₁…₃:

  * γ₁ (SLA failure) > γ₂ (overload) > γ₃ (energy).
* w₁…₄:

  * w₁ (SLA) largest, then w₂ (utilization), then w₃ & w₄.

These are **not fixed**; GA explores local variations and picks sets that maximize fitness.

---

## 7. File / Module Mapping to Existing Repo

Adjust previous EDD file list to match CloudAI structure:

### Master

* `master/internal/scheduler/`

  * `scheduler.go` (existing interface + Round-Robin)
  * `rts_scheduler.go` (new RTSScheduler, primary)
  * `rts_models.go` (TaskView, WorkerView, GAParams)
  * `rts_params_loader.go` (load `config/ga_output.json`)

* `master/internal/telemetry/`

  * `telemetry_manager.go` (existing; extend to expose WorkerView and load timeline)
  * `tau_store.go` (new helper for τ updates)

* `master/internal/aod/`

  * `ga_runner.go` (RunGAEpoch)
  * `fitness.go`
  * `affinity_builder.go`
  * `penalty_builder.go`
  * `theta_trainer.go` (regression)
  * `models.go` (Chromosome, Metrics, etc.)

* `master/internal/db/`

  * Reuse `tasks.go`, `assignments.go`, `results.go`, `workers.go`
  * Optionally add:

    * `history.go` (TaskHistory queries)
    * `worker_stats.go` (WorkerStats aggregation)

* `master/config/ga_output.json`

  * GA output file consumed by RTS.

### Worker

* No structural changes; telemetry and execution paths are reused.

---

## 8. Execution Loop (Concrete, CloudAI-Specific)

1. **Master startup** (`master/main.go`):

   * Initialize DB, TelemetryManager, WebSocket.
   * Initialize Round-Robin scheduler.
   * Initialize RTSScheduler(rrScheduler, telemetrySource, tauStore).
   * Start `StartQueueProcessor()` using RTSScheduler as the active scheduler.
   * Start AOD ticker → `aod.RunGAEpoch()` every 30–60s.

2. **Worker startup**:

   * Registers with master via existing `RegisterWorker`. 
   * Starts sending heartbeats.

3. **Task submission** (CLI or future API):

   * Master enqueues task (`QueuedTask`) in task queue. 

4. **Queue processor**:

   * Pulls `QueuedTask`.
   * Calls `scheduler.SelectWorker` (RTSScheduler).
   * RTSScheduler:

     * Builds TaskView + WorkerViews.
     * Filters feasible.
     * Computes `E_ij`, `R_ij`, `R_ij^final`.
     * If failure: fallback to Round-Robin.
   * Returns selected worker.
   * Master calls `AssignTask` RPC to worker.

5. **Execution + completion**:

   * Worker runs Docker container and reports `TaskResult`. 
   * Master updates TASKS/ASSIGNMENTS/RESULTS.
   * Master computes `ActualRuntime`, `SLASuccess`, updates τ and stats.

6. **AOD / GA**:

   * Reads TaskHistory + WorkerStats from DB.
   * Trains θ, computes Affinity, Penalty, weights.
   * Computes best GAParams via fitness.
   * Writes `ga_output.json`.

7. **RTS reload**:

   * RTSScheduler reloads GAParams every N seconds.
   * Next tasks scheduled with improved parameters.

---
