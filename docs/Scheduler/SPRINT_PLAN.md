# **CLOUD-AI RTS+GA SCHEDULER — SPRINT PLAN**

---

## **MILESTONE 1: Foundation & Data Models**
**Goal**: Establish core data structures and interfaces for RTS without disrupting existing functionality.

---

### **Task 1.1: Create RTS Data Models**
**File(s)**:
- `master/internal/scheduler/rts_models.go` (NEW)

**Functions to add/modify**:
- `type TaskView struct`
- `type WorkerView struct`
- `type Theta struct`
- `type Risk struct`
- `type AffinityWeights struct`
- `type PenaltyWeights struct`
- `type GAParams struct`
- `func NewTaskViewFromProto(pbTask *pb.Task, now time.Time, tau float64, k float64) TaskView`
- `func InferTaskType(pbTask *pb.Task) string`
- `func ValidateTaskType(taskType string) bool`
- `const` for task type enums

**Implementation steps**:
1. Create `rts_models.go` with all structs as defined in EDD §2.2, §2.3, §5.2
2. Define task type constants:
   ```go
   const (
       TaskTypeCPULight      = "cpu-light"
       TaskTypeCPUHeavy      = "cpu-heavy"
       TaskTypeMemoryHeavy   = "memory-heavy"
       TaskTypeGPUInference  = "gpu-inference"
       TaskTypeGPUTraining   = "gpu-training"
       TaskTypeMixed         = "mixed"
   )
   ```
3. Implement `ValidateTaskType` that returns true only if taskType is one of the 6 valid types
4. Implement `TaskView` with fields: ID, Type, CPU, Mem, GPU, Storage, ArrivalTime, Tau, Deadline, UserID
5. Implement `WorkerView` with fields: ID, CPUAvail, MemAvail, GPUAvail, StorageAvail, Load
6. Implement `GAParams` with nested structs for Theta, Risk, AffinityWeights, PenaltyWeights, AffinityMatrix, PenaltyVector
7. Implement `NewTaskViewFromProto` that:
   - Extracts resource requirements from `pb.Task`
   - Uses explicit `pb.Task.TaskType` if present and valid
   - Calls `InferTaskType` ONLY if taskType is empty or invalid
   - Computes `Deadline = now + k * tau`
   - Returns populated `TaskView`
8. Implement `InferTaskType` with rules (used ONLY when user does not specify):
   - If `ReqGpu > 2.0 && ReqCPU > 4.0` → return "gpu-training"
   - If `ReqGpu > 0` → return "gpu-inference"
   - If `ReqMemory > 8.0` → return "memory-heavy"
   - If `ReqCPU > 4.0` → return "cpu-heavy"
   - If `ReqCPU > 0` → return "cpu-light"
   - Otherwise → return "mixed"

---

### **Task 1.2: Create Telemetry Data Models**
**File(s)**:
- `master/internal/db/history.go` (NEW)

**Functions to add/modify**:
- `type TaskHistory struct`
- `type WorkerStats struct`
- `func (db *HistoryDB) GetTaskHistory(ctx context.Context, since time.Time, until time.Time) ([]TaskHistory, error)`
- `func (db *HistoryDB) GetWorkerStats(ctx context.Context, since time.Time, until time.Time) ([]WorkerStats, error)`

**Implementation steps**:
1. Create `history.go` in `master/internal/db/`
2. Define `TaskHistory` struct with fields from EDD §2.4:
   - TaskID, WorkerID, Type (must be one of the 6 standardized task types), ArrivalTime, Deadline, ActualStart, ActualFinish, ActualRuntime, SLASuccess, CPUUsed, MemUsed, GPUUsed, LoadAtStart
3. Define `WorkerStats` struct with fields from EDD §2.4:
   - WorkerID, TasksRun, SLAViolations, TotalRuntime, CPUUsedTotal, MemUsedTotal, GPUUsedTotal, OverloadTime, TotalTime
4. Create `HistoryDB` struct with MongoDB client
5. Implement `GetTaskHistory` that:
   - Joins TASKS + ASSIGNMENTS + RESULTS collections
   - Filters by time range
   - Returns enriched TaskHistory records with standardized Type field
6. Implement `GetWorkerStats` that:
   - Aggregates data from TaskHistory
   - Computes per-worker statistics
   - Returns WorkerStats slice

---

### **Task 1.3: Extend Task Schema for SLA Tracking**
**File(s)**:
- `master/internal/db/tasks.go` (MODIFY)
- `master/internal/db/assignments.go` (MODIFY)

**Functions to add/modify**:
- Modify `type Task struct` to add:
  - `Deadline time.Time`
  - `Tau float64`
  - `TaskType string` (must be one of 6 valid types)
- Modify `type Assignment struct` to add:
  - `LoadAtStart float64`
- `func (db *TaskDB) UpdateTaskWithSLA(ctx context.Context, taskID string, deadline time.Time, tau float64, taskType string) error`

**Implementation steps**:
1. Add `Deadline`, `Tau`, `TaskType` fields to `Task` struct with bson tags
2. Add validation in `TaskType` setter to ensure it's one of: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed
3. Add `LoadAtStart` field to `Assignment` struct
4. Implement `UpdateTaskWithSLA` to update these fields in MongoDB
5. No migration needed; new fields will be empty for existing tasks

---

### **Task 1.4: Add Task Type to Proto and API Layer (NEW)**
**File(s)**:
- `proto/master_worker.proto` (MODIFY)
- `proto/master_agent.proto` (MODIFY)

**Functions to add/modify**:
- Modify `message Task` to add:
  - `string task_type = 10;` (optional field for explicit task type)

**Implementation steps**:
1. Add `task_type` field to Task message in proto files
2. Regenerate proto code: `cd proto && ./generate.sh`
3. Add validation in gRPC handlers to ensure task_type is one of 6 valid enums or empty
4. Document that if task_type is provided, it must be one of: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed

---

## **MILESTONE 2: Tau Store & Telemetry Enrichment**
**Goal**: Implement runtime learning (τ) and enrich telemetry with SLA tracking.

---

### **Task 2.1: Implement Tau Store**
**File(s)**:
- `master/internal/telemetry/tau_store.go` (NEW)

**Functions to add/modify**:
- `type TauStore interface`
  - `GetTau(taskType string) float64`
  - `UpdateTau(taskType string, actualRuntime float64)`
  - `SetTau(taskType string, tau float64)`
- `type InMemoryTauStore struct`
- `func NewInMemoryTauStore() *InMemoryTauStore`
- `func (s *InMemoryTauStore) GetTau(taskType string) float64`
- `func (s *InMemoryTauStore) UpdateTau(taskType string, actualRuntime float64)`
- `func (s *InMemoryTauStore) SetTau(taskType string, tau float64)`

**Implementation steps**:
1. Create `tau_store.go` with `TauStore` interface
2. Implement `InMemoryTauStore` with:
   - `tauMap map[string]float64` (keyed by 6 task types)
   - `mu sync.RWMutex`
   - `lambda float64` (default 0.2)
3. Initialize with default tau values for all 6 task types in constructor
4. Implement `GetTau`:
   - Return task-type-specific default if not found (cpu-light: 5.0, cpu-heavy: 15.0, memory-heavy: 20.0, gpu-inference: 10.0, gpu-training: 60.0, mixed: 10.0)
   - Return stored value from map
5. Implement `UpdateTau` using EMA formula from EDD §4.2:
   - `tau_new = lambda * actualRuntime + (1-lambda) * tau_old`
   - Thread-safe update with mutex
   - Only update if taskType is one of the 6 valid types
6. Implement `SetTau` for explicit initialization

---

### **Task 2.2: Enrich Task Submission with Tau & Deadline**
**File(s)**:
- `master/internal/server/master_server.go` (MODIFY)

**Functions to add/modify**:
- Modify `type MasterServer struct` to add:
  - `tauStore *telemetry.TauStore`
  - `slaMultiplier float64`
- Modify `func NewMasterServer(...)` to accept `tauStore`
- Modify `func (s *MasterServer) SubmitTask(...)` to:
  - Validate explicit task_type if provided
  - Use explicit task_type if valid, otherwise infer
  - Get tau from TauStore
  - Compute deadline
  - Store in TaskDB

**Implementation steps**:
1. Add `tauStore` field to `MasterServer` struct
2. Add `slaMultiplier` field (read from env var, default 2.0)
3. Update `NewMasterServer` constructor to accept `tauStore` parameter
4. In `SubmitTask` RPC handler:
   - Check if `task.TaskType` is provided and non-empty
   - If provided, validate using `scheduler.ValidateTaskType(task.TaskType)`
   - If valid, use explicit type; if invalid or empty, call `scheduler.InferTaskType(task)`
   - Store final taskType in task struct
   - Get `tau = tauStore.GetTau(taskType)`
   - Compute `deadline = now + slaMultiplier * tau * time.Second`
   - Call `taskDB.UpdateTaskWithSLA(ctx, taskID, deadline, tau, taskType)`

---

### **Task 2.3: Track Load at Task Assignment**
**File(s)**:
- `master/internal/server/master_server.go` (MODIFY)

**Functions to add/modify**:
- Modify `func (s *MasterServer) assignTaskToWorker(ctx context.Context, task *pb.Task, workerID string) (*pb.TaskAck, error)`

**Implementation steps**:
1. In `assignTaskToWorker`, before calling gRPC:
   - Get current worker load from `telemetryManager.GetWorkerData(workerID)`
   - Extract `Load` value (normalized CPU+Mem+GPU usage)
2. When storing assignment in `assignmentDB.Create`:
   - Include `LoadAtStart` field with current load value

---

### **Task 2.4: Update Tau on Task Completion**
**File(s)**:
- `master/internal/server/master_server.go` (MODIFY)

**Functions to add/modify**:
- Modify `func (s *MasterServer) ReportTaskCompletion(ctx context.Context, result *pb.TaskResult) (*pb.TaskAck, error)`

**Implementation steps**:
1. In `ReportTaskCompletion`, after storing result:
   - Fetch task from TaskDB to get `TaskType` and `StartedAt`
   - Compute `actualRuntime = result.CompletedAt - task.StartedAt`
   - Call `tauStore.UpdateTau(task.TaskType, actualRuntime)`
2. Log the tau update for debugging

---

### **Task 2.5: Compute and Store SLA Success**
**File(s)**:
- `master/internal/db/results.go` (MODIFY)

**Functions to add/modify**:
- Modify `type TaskResult struct` to add:
  - `SLASuccess bool`
- `func (db *ResultDB) CreateWithSLA(ctx context.Context, result *db.TaskResult, slaSuccess bool) error`

**Implementation steps**:
1. Add `SLASuccess` field to `TaskResult` struct
2. In `MasterServer.ReportTaskCompletion`:
   - Fetch task deadline from TaskDB
   - Compare `result.CompletedAt` vs `task.Deadline`
   - Set `slaSuccess = result.CompletedAt <= task.Deadline`
   - Store result with SLA flag

---

## **MILESTONE 3: RTS Scheduler Implementation**
**Goal**: Implement core RTS scheduling logic as primary scheduler with Round-Robin fallback.

---

### **Task 3.1: Create TelemetrySource Adapter**
**File(s)**:
- `master/internal/scheduler/telemetry_source.go` (NEW)

**Functions to add/modify**:
- `type TelemetrySource interface`
  - `GetWorkerViews() []WorkerView`
  - `GetWorkerLoad(workerID string) float64`
- `type MasterTelemetrySource struct`
- `func NewMasterTelemetrySource(telemetryMgr *telemetry.TelemetryManager, workerDB *db.WorkerDB) *MasterTelemetrySource`
- `func (s *MasterTelemetrySource) GetWorkerViews() []WorkerView`
- `func (s *MasterTelemetrySource) GetWorkerLoad(workerID string) float64`

**Implementation steps**:
1. Create `telemetry_source.go` with `TelemetrySource` interface
2. Implement `MasterTelemetrySource` that bridges:
   - `telemetry.TelemetryManager` for current load/usage
   - `db.WorkerDB` for total capacities
3. Implement `GetWorkerViews`:
   - Iterate over all workers from TelemetryManager
   - For each worker, get total capacity from WorkerDB
   - Compute available resources (Total - Allocated)
   - Compute normalized load
   - Return `[]WorkerView`
4. Implement `GetWorkerLoad`:
   - Get worker telemetry data
   - Return normalized load value

---

### **Task 3.2: Implement GAParams Loader**
**File(s)**:
- `master/internal/scheduler/rts_params_loader.go` (NEW)
- `master/config/ga_output.json` (NEW)

**Functions to add/modify**:
- `func LoadGAParams(filePath string) (*GAParams, error)`
- `func (p *GAParams) SaveToFile(filePath string) error`
- `func GetDefaultGAParams() *GAParams`

**Implementation steps**:
1. Create `rts_params_loader.go` with JSON loading functions
2. Implement `LoadGAParams`:
   - Read JSON file
   - Unmarshal into `GAParams` struct
   - Validate ranges (e.g., Theta values reasonable)
   - Return error if file doesn't exist or invalid
3. Implement `GetDefaultGAParams`:
   - Return sensible defaults from EDD §6:
     - Theta: {0.1, 0.1, 0.3, 0.2}
     - Risk: {10.0, 1.0}
     - AffinityW: {1.0, 2.0, 0.5}
     - PenaltyW: {2.0, 1.0, 0.5}
     - Empty Affinity/Penalty maps
4. Create initial `config/ga_output.json` with defaults

---

### **Task 3.3: Implement RTS Core Logic**
**File(s)**:
- `master/internal/scheduler/rts_scheduler.go` (NEW)

**Functions to add/modify**:
- `type RTSScheduler struct`
- `func NewRTSScheduler(rrScheduler Scheduler, tauStore *telemetry.TauStore, telemetrySource TelemetrySource, paramsPath string, slaMultiplier float64) *RTSScheduler`
- `func (s *RTSScheduler) GetName() string`
- `func (s *RTSScheduler) Reset()`
- `func (s *RTSScheduler) SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string`
- `func (s *RTSScheduler) buildTaskView(task *pb.Task, now time.Time) TaskView`
- `func (s *RTSScheduler) buildWorkerViews(workers map[string]*WorkerInfo) []WorkerView`
- `func (s *RTSScheduler) filterFeasible(task TaskView, workers []WorkerView) []WorkerView`
- `func (s *RTSScheduler) predictExecTime(t TaskView, w WorkerView, theta Theta) float64`
- `func (s *RTSScheduler) computeBaseRisk(t TaskView, w WorkerView, eHat float64, alpha, beta float64) float64`
- `func (s *RTSScheduler) computeFinalRisk(base float64, taskType string, workerID string, params *GAParams) float64`
- `func (s *RTSScheduler) getGAParamsSafe() *GAParams`
- `func (s *RTSScheduler) lookupWorkerInfo(workerID string, workers map[string]*WorkerInfo) *WorkerInfo`
- `func (s *RTSScheduler) startParamsReloader()`

**Implementation steps**:
1. Create `rts_scheduler.go` with `RTSScheduler` struct containing:
   - `rrScheduler Scheduler`
   - `tauStore *telemetry.TauStore`
   - `telemetrySource TelemetrySource`
   - `params *GAParams`
   - `paramsMu sync.RWMutex`
   - `paramsPath string`
   - `slaMultiplier float64`

2. Implement `NewRTSScheduler`:
   - Initialize all fields
   - Load initial GAParams from file (or defaults if missing)
   - Start background params reloader goroutine

3. Implement `SelectWorker` following EDD §3.9 algorithm:
   - Build `TaskView` from `pb.Task`
   - Build `[]WorkerView` from workers + telemetry
   - Filter feasible workers
   - If none feasible → fallback to Round-Robin
   - Load GAParams (thread-safe)
   - For each feasible worker:
     - Compute `E_hat` using `predictExecTime`
     - Compute base risk using `computeBaseRisk`
     - Compute final risk using `computeFinalRisk`
     - Track best (lowest risk)
   - If best invalid (inf/NaN) → fallback to Round-Robin
   - Return selected worker ID

4. Implement `buildTaskView`:
   - Extract task type using `InferTaskType`
   - Get tau from `tauStore.GetTau(taskType)`
   - Compute deadline using `slaMultiplier`
   - Return `TaskView`

5. Implement `buildWorkerViews`:
   - Call `telemetrySource.GetWorkerViews()`
   - Filter to only include workers in the `workers` map
   - Return filtered slice

6. Implement `filterFeasible` per EDD §3.3:
   - Check: `w.CPUAvail >= t.CPU && w.MemAvail >= t.Mem && w.GPUAvail >= t.GPU && w.StorageAvail >= t.Storage`
   - Return only feasible workers

7. Implement `predictExecTime` per EDD §3.5:
   - Formula: `tau * (1 + theta1*(C/Cavail) + theta2*(M/Mavail) + theta3*(G/Gavail) + theta4*Load)`
   - Handle division by zero (use 1.0 if avail=0)

8. Implement `computeBaseRisk` per EDD §3.7:
   - `fHat = arrivalTime + eHat`
   - `delta = max(0, fHat - deadline)`
   - `risk = alpha * delta + beta * load`

9. Implement `computeFinalRisk` per EDD §3.8:
   - `affinity = params.AffinityMatrix[taskType][workerID]` (default 0)
   - `penalty = params.PenaltyVector[workerID]` (default 0)
   - `finalRisk = baseRisk - affinity + penalty`

10. Implement `getGAParamsSafe`:
    - Thread-safe read of `params` with RWMutex

11. Implement `startParamsReloader`:
    - Ticker every 30 seconds
    - Reload GAParams from file
    - Update with write lock

---

### **Task 3.4: Integrate RTS into Master Server** ✅
**Status**: COMPLETE
**Files Modified**: 2
**Documentation**: [TASK_3_4_RTS_INTEGRATION.md](./TASK_3_4_RTS_INTEGRATION.md), [RTS_INTEGRATION_QUICK_REF.md](./RTS_INTEGRATION_QUICK_REF.md)

**File(s)**:
- `master/main.go` (MODIFIED)
- `master/internal/server/master_server.go` (MODIFIED)

**Functions added/modified**:
- Modified `main()` in `main.go` to initialize RTS
- Added `NewMasterServerWithScheduler` constructor
- Modified `NewMasterServer` to use default Round-Robin (backward compatible)

**Implementation completed**:
1. In `main.go`, after creating TelemetryManager:
   - Create `tauStore = telemetry.NewInMemoryTauStore()`
   - Load SLA multiplier from env var (default 2.0)
   - Create `rrScheduler = scheduler.NewRoundRobinScheduler()`
   - Create telemetry source adapter
   - Create `rtsScheduler = scheduler.NewRTSScheduler(rrScheduler, tauStore, telemetrySource, "config/ga_output.json", slaMultiplier)`
   - Pass `rtsScheduler` and `tauStore` to `NewMasterServer`

2. In `NewMasterServer`:
   - Accept `scheduler Scheduler` parameter
   - Accept `tauStore *telemetry.TauStore` parameter
   - Store both in MasterServer struct
   - Load SLA multiplier from env

3. Verify that `StartQueueProcessor` → `processQueue` → `selectWorkerForTask` uses `s.scheduler` (which is now RTS)

---

## **MILESTONE 4: AOD/GA Module Implementation**
**Goal**: Implement offline optimization with Genetic Algorithm.

---

### **Task 4.1: Create AOD Data Models**
**File(s)**:
- `master/internal/aod/models.go` (NEW)

**Functions to add/modify**:
- `type Chromosome struct`
- `type Population []Chromosome`
- `type Metrics struct`
- `type GAConfig struct`

**Implementation steps**:
1. Create `master/internal/aod/` directory
2. Create `models.go` with:
   - `Chromosome` containing all GA-evolvable parameters:
     - `Theta Theta`
     - `Risk Risk`
     - `AffinityW AffinityWeights`
     - `PenaltyW PenaltyWeights`
     - `Fitness float64`
   - `Metrics` struct for fitness computation:
     - `SLASuccess float64`
     - `Utilization float64`
     - `EnergyNorm float64`
     - `OverloadNorm float64`
   - `GAConfig` for tunable parameters:
     - `PopulationSize int`
     - `Generations int`
     - `MutationRate float64`
     - `CrossoverRate float64`
     - `FitnessWeights [4]float64` (w1, w2, w3, w4)

---

### **Task 4.2: Implement Theta Trainer (Regression)**
**File(s)**:
- `master/internal/aod/theta_trainer.go` (NEW)

**Functions to add/modify**:
- `func TrainTheta(history []db.TaskHistory) Theta`
- `func buildRegressionMatrix(history []db.TaskHistory) (X [][]float64, y []float64)`
- `func solveLinearRegression(X [][]float64, y []float64) []float64`

**Implementation steps**:
1. Create `theta_trainer.go`
2. Implement `TrainTheta`:
   - Call `buildRegressionMatrix` to create features and targets
   - Call `solveLinearRegression` to get θ₁..θ₄
   - Return `Theta` struct
3. Implement `buildRegressionMatrix`:
   - For each TaskHistory record:
     - Features: [CPU_ratio, Mem_ratio, GPU_ratio, Load]
     - Target: (ActualRuntime / Tau) - 1.0
   - Return X matrix and y vector
4. Implement `solveLinearRegression`:
   - Use simple normal equation: θ = (X^T X)^(-1) X^T y
   - Or use gradient descent if dataset large
   - Return theta vector [θ₁, θ₂, θ₃, θ₄]

---

### **Task 4.3: Implement Affinity Builder**
**File(s)**:
- `master/internal/aod/affinity_builder.go` (NEW)

**Functions to add/modify**:
- `func BuildAffinityMatrix(history []db.TaskHistory, weights AffinityWeights) map[string]map[string]float64`
- `func computeSpeed(taskType, workerID string, history []db.TaskHistory) float64`
- `func computeSLAReliability(taskType, workerID string, history []db.TaskHistory) float64`
- `func computeOverloadRate(taskType, workerID string, history []db.TaskHistory) float64`

**Implementation steps**:
1. Create `affinity_builder.go`
2. Implement `BuildAffinityMatrix` per EDD §5.3:
   - Get all unique (taskType, workerID) pairs from history
   - For each of the 6 task types (cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed):
     - For each worker:
       - Compute baseline runtime for that specific type
       - Compute speed ratio for (type, worker) pair
       - Compute SLA reliability for (type, worker) pair
       - Compute overload rate for (type, worker) pair
       - Compute raw affinity: `a1*speed + a2*SLA - a3*overload`
       - Clip to [-5, +5]
   - Return nested map[taskType][workerID] with 6 task type rows

3. Implement helper functions:
   - `computeSpeed`: mean(baseline for type) / mean(runtime_on_worker for type)
   - `computeSLAReliability`: 1 - (violations for type on worker / tasks for type on worker)
   - `computeOverloadRate`: sum(overload_time for type on worker) / sum(total_time for type on worker)

---

### **Task 4.4: Implement Penalty Builder**
**File(s)**:
- `master/internal/aod/penalty_builder.go` (NEW)

**Functions to add/modify**:
- `func BuildPenaltyVector(workerStats []db.WorkerStats, weights PenaltyWeights) map[string]float64`
- `func computeSLAFailRate(stats db.WorkerStats) float64`
- `func computeWorkerOverloadRate(stats db.WorkerStats) float64`
- `func computeEnergyNorm(stats db.WorkerStats) float64`

**Implementation steps**:
1. Create `penalty_builder.go`
2. Implement `BuildPenaltyVector` per EDD §5.4:
   - For each worker in workerStats:
     - Compute SLA fail rate
     - Compute overload rate
     - Compute energy norm
     - Compute penalty: `g1*SLAfail + g2*overload + g3*energy`
   - Return map[workerID]penalty

3. Implement helpers:
   - `computeSLAFailRate`: violations / tasks
   - `computeWorkerOverloadRate`: overloadTime / totalTime
   - `computeEnergyNorm`: normalize energy usage relative to baseline

---

### **Task 4.5: Implement Fitness Function**
**File(s)**:
- `master/internal/aod/fitness.go` (NEW)

**Functions to add/modify**:
- `func ComputeFitness(chromosome Chromosome, history []db.TaskHistory, workerStats []db.WorkerStats, config GAConfig) float64`
- `func computeSLASuccess(history []db.TaskHistory) float64`
- `func computeUtilization(workerStats []db.WorkerStats) float64`
- `func computeEnergyNormTotal(workerStats []db.WorkerStats) float64`
- `func computeOverloadNormTotal(workerStats []db.WorkerStats) float64`

**Implementation steps**:
1. Create `fitness.go`
2. Implement `ComputeFitness` per EDD §5.5:
   - Extract weights w1..w4 from config
   - Compute SLA success rate from history
   - Compute avg utilization from workerStats
   - Compute energy norm
   - Compute overload norm
   - Return: `w1*SLA + w2*Util - w3*Energy - w4*Overload`

3. Implement helpers:
   - `computeSLASuccess`: count(SLASuccess=true) / count(total)
   - `computeUtilization`: avg across workers of (used / total)
   - `computeEnergyNormTotal`: sum of energy metrics normalized
   - `computeOverloadNormTotal`: sum of overload durations normalized

---

### **Task 4.6: Implement GA Runner**
**File(s)**:
- `master/internal/aod/ga_runner.go` (NEW)

**Functions to add/modify**:
- `func RunGAEpoch(ctx context.Context, historyDB *db.HistoryDB, config GAConfig, paramsOutputPath string) error`
- `func initializePopulation(config GAConfig) Population`
- `func evaluatePopulation(pop Population, history []db.TaskHistory, workerStats []db.WorkerStats, config GAConfig) Population`
- `func selection(pop Population) Population`
- `func crossover(parent1, parent2 Chromosome, rate float64) (Chromosome, Chromosome)`
- `func mutate(c Chromosome, rate float64) Chromosome`
- `func getBestChromosome(pop Population) Chromosome`

**Implementation steps**:
1. Create `ga_runner.go`
2. Implement `RunGAEpoch`:
   - Fetch TaskHistory and WorkerStats from DB (last 24h)
   - If insufficient data, use default params and return
   - Train Theta using `TrainTheta`
   - Initialize population with random variations around current params
   - For each generation:
     - Evaluate fitness for all chromosomes
     - Selection (tournament or roulette)
     - Crossover
     - Mutation
   - Get best chromosome
   - Build Affinity matrix using best AffinityWeights
   - Build Penalty vector using best PenaltyWeights
   - Create GAParams from best chromosome
   - Save to JSON file

3. Implement population operations:
   - `initializePopulation`: create N random chromosomes
   - `evaluatePopulation`: compute fitness for each
   - `selection`: pick top 50% by fitness
   - `crossover`: combine two parents
   - `mutate`: random perturbations

---

### **Task 4.7: Integrate AOD into Master**
**File(s)**:
- `master/main.go` (MODIFY)

**Functions to add/modify**:
- Modify `main()` to start AOD ticker

**Implementation steps**:
1. In `main.go`, after starting queue processor:
   - Create `historyDB = db.NewHistoryDB(ctx, cfg)`
   - Create `gaConfig = aod.GetDefaultGAConfig()` (from env vars if set)
   - Start goroutine with ticker:
     ```go
     go func() {
         ticker := time.NewTicker(60 * time.Second)
         defer ticker.Stop()
         for range ticker.C {
             if err := aod.RunGAEpoch(context.Background(), historyDB, gaConfig, "config/ga_output.json"); err != nil {
                 log.Printf("AOD/GA epoch error: %v", err)
             } else {
                 log.Printf("✓ AOD/GA epoch completed")
             }
         }
     }()
     ```

---

## **MILESTONE 5: Testing & Validation**
**Goal**: Verify RTS+GA system works end-to-end with real workloads.

---

### **Task 5.1: Create Test Workload Generator**
**File(s)**:
- `test/workload_generator.go` (NEW)

**Functions to add/modify**:
- `func GenerateMixedWorkload(count int) []*pb.Task`
- `func GenerateCPULightTask() *pb.Task`
- `func GenerateCPUHeavyTask() *pb.Task`
- `func GenerateMemoryHeavyTask() *pb.Task`
- `func GenerateGPUInferenceTask() *pb.Task`
- `func GenerateGPUTrainingTask() *pb.Task`
- `func GenerateMixedTask() *pb.Task`

**Implementation steps**:
1. Create `test/` directory
2. Implement workload generators that create tasks with:
   - Explicit task_type field set to one of the 6 valid types
   - Different resource requirements matching each type
   - Realistic patterns (burst, steady, mixed)
3. Test that explicit task_type is preserved through submission

---

### **Task 5.2: Create Scheduler Comparison Test**
**File(s)**:
- `test/scheduler_comparison_test.go` (NEW)

**Functions to add/modify**:
- `func TestRTSvsRoundRobin(t *testing.T)`
- `func measureSLAViolations(results []TaskResult) float64`
- `func measureUtilization(telemetry []WorkerTelemetry) float64`

**Implementation steps**:
1. Create test that:
   - Submits same workload to both schedulers
   - Measures SLA violations, utilization, makespan
   - Compares results
2. Verify RTS never breaks existing RoundRobin behavior (fallback works)

---

### **Task 5.3: Create GA Convergence Test**
**File(s)**:
- `test/ga_convergence_test.go` (NEW)

**Functions to add/modify**:
- `func TestGAConvergence(t *testing.T)`
- `func verifyAffinityMatrix(affinity map[string]map[string]float64) error`
- `func verifyPenaltyVector(penalty map[string]float64) error`

**Implementation steps**:
1. Create synthetic telemetry data with all 6 task types
2. Run GA for multiple epochs
3. Verify:
   - Theta converges to expected values
   - Affinity matrix has exactly 6 rows (one per task type)
   - Affinity correctly identifies fast workers for each task type
   - Penalty correctly penalizes unreliable workers
   - Fitness improves over generations

---

### **Task 5.4: Integration Test with Real Workers**
**File(s)**:
- `test/integration_test.go` (NEW)

**Functions to add/modify**:
- `func TestEndToEndRTSScheduling(t *testing.T)`
- `func TestExplicitTaskTypeSubmission(t *testing.T)` (NEW)

**Implementation steps**:
1. Start master with RTS scheduler
2. Start 3+ mock workers with different capacities
3. Submit 100+ tasks of mixed types with explicit task_type field
4. Verify:
   - All tasks complete successfully
   - Explicit task_type is preserved and not overwritten by inference
   - SLA violations are tracked correctly per task type
   - Tau values are updated separately for each of the 6 task types
   - GA params file is generated with 6-row affinity matrix
   - RTS uses GA params after first epoch
5. Test that tasks without explicit task_type use inference correctly

---

### **Task 5.5: Load Test & Performance Benchmarks**
**File(s)**:
- `test/load_test.go` (NEW)

**Functions to add/modify**:
- `func BenchmarkRTSScheduler(b *testing.B)`
- `func BenchmarkRoundRobinScheduler(b *testing.B)`

**Implementation steps**:
1. Benchmark SelectWorker call latency
2. Verify RTS overhead is < 10ms per decision
3. Test with 1000+ workers and 10000+ tasks across all 6 task types
4. Measure memory usage growth

---

### **Task 5.6: Task Type Migration Utility (NEW)**
**File(s)**:
- `master/internal/db/migration.go` (NEW)
- `master/cmd/migrate_task_types.go` (NEW)

**Functions to add/modify**:
- `func MigrateTaskTypes(ctx context.Context, taskDB *TaskDB) error`
- `func normalizeOldTaskType(oldType string) string`

**Implementation steps**:
1. Create migration utility that:
   - Reads all existing tasks from MongoDB TASKS collection
   - For tasks with empty or invalid TaskType:
     - Apply `InferTaskType` logic based on resource requirements
     - Update TaskType to one of the 6 valid types
   - For tasks with old type labels (e.g., "cpu", "gpu", "dl"):
     - Map to new standardized types:
       - "cpu" → "cpu-light"
       - "gpu" → "gpu-inference"
       - "dl" → "gpu-training"
       - Others → "mixed"
   - Update all affected records in MongoDB
2. Create standalone CLI tool for running migration
3. Log migration statistics (records updated, type distribution)

---

## **MILESTONE 6: Documentation & Deployment**
**Goal**: Complete documentation and production readiness.

---

### **Task 6.1: Configuration Documentation**
**File(s)**:
- `docs/RTS_CONFIGURATION.md` (NEW)

**Content**:
- Environment variables:
  - `SCHED_SLA_MULTIPLIER` (default 2.0)
  - `GA_POPULATION_SIZE` (default 20)
  - `GA_GENERATIONS` (default 10)
  - `GA_MUTATION_RATE` (default 0.1)
  - `GA_CROSSOVER_RATE` (default 0.7)
  - `GA_EPOCH_INTERVAL` (default 60s)
  - `RTS_PARAMS_RELOAD_INTERVAL` (default 30s)
- Task type enums and validation:
  - Valid task types: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed
  - How to specify explicit task_type in submission
  - Task type inference rules when not specified
- GAParams JSON schema with 6-row affinity matrix structure
- How to disable RTS (use Round-Robin only)

---

### **Task 6.2: Operator Guide**
**File(s)**:
- `docs/RTS_OPERATOR_GUIDE.md` (NEW)

**Content**:
- How to monitor RTS performance
- How to interpret ga_output.json
- How to tune GA weights
- Task type best practices:
  - When to use explicit task_type vs inference
  - How task types affect scheduling decisions
  - Reviewing task type distribution in telemetry
- Troubleshooting common issues:
  - All tasks going to one worker (check affinity for specific task type)
  - High SLA violations (increase alpha, adjust theta, check per-type tau values)
  - Poor utilization (decrease beta, adjust affinity)
  - Invalid task_type errors

---

### **Task 6.3: API Documentation**
**File(s)**:
- `docs/RTS_API_REFERENCE.md` (NEW)

**Content**:
- Scheduler interface documentation
- RTSScheduler public methods
- TauStore interface
- GAParams structure
- How to implement custom schedulers

---

### **Task 6.4: Migration Guide**
**File(s)**:
- `docs/RTS_MIGRATION.md` (NEW)

**Content**:
- Step-by-step migration from Round-Robin to RTS
- Backward compatibility notes
- How to rollback if issues occur
- Database schema changes required
- Task type migration procedure:
  - Running the migration utility
  - Validating task type distribution
  - Handling tasks with old type labels
- Performance considerations

---

## **MILESTONE 7: Monitoring & Observability**
**Goal**: Add metrics and logging for production operations.

---

### **Task 7.1: Add Prometheus Metrics**
**File(s)**:
- `master/internal/scheduler/rts_metrics.go` (NEW)

**Functions to add/modify**:
- `var rtsDecisionDuration prometheus.Histogram`
- `var rtsFallbackCounter prometheus.Counter`
- `var rtsSelectedWorkerGauge prometheus.GaugeVec`
- `func InitRTSMetrics()`
- `func recordSchedulingDecision(duration time.Duration, workerID string, fallback bool)`

**Implementation steps**:
1. Create Prometheus metrics for:
   - Scheduling decision latency
   - Fallback to Round-Robin count
   - Selected worker distribution
   - SLA violations per worker
   - Tau values per task type
2. Export metrics on `/metrics` endpoint

---

### **Task 7.2: Add Structured Logging**
**File(s)**:
- `master/internal/scheduler/rts_scheduler.go` (MODIFY)

**Functions to add/modify**:
- Enhance logging in `SelectWorker` with structured fields

**Implementation steps**:
1. Add detailed logs for:
   - Task type inference
   - Feasibility filtering results
   - Risk scores for all workers
   - Selected worker and reason
   - Fallback triggers
2. Use log levels appropriately (DEBUG, INFO, WARN, ERROR)

---

### **Task 7.3: Add GA Training Metrics**
**File(s)**:
- `master/internal/aod/ga_runner.go` (MODIFY)

**Functions to add/modify**:
- Add metrics for GA epochs:
  - `var gaEpochDuration prometheus.Histogram`
  - `var gaBestFitness prometheus.Gauge`
  - `var gaDataPointsUsed prometheus.Gauge`
  - `var gaAffinityMatrixSize prometheus.GaugeVec` (per task type)

**Implementation steps**:
1. Track and log:
   - Epoch duration
   - Best fitness achieved
   - Number of training samples per task type
   - Theta convergence
   - Affinity matrix size (should be 6 rows for 6 task types)
   - Penalty vector size
2. Alert if GA fails or data insufficient for any task type

---

## **Summary & Dependencies**

### **Critical Path**:
1. M1 (Foundation) → M2 (Telemetry) → M3 (RTS) → M4 (AOD) → M5 (Testing)
2. M6 (Documentation) can proceed in parallel with M5
3. M7 (Monitoring) should be done after M3 is stable

### **Estimated Timeline**:
- **M1**: 3 days
- **M2**: 4 days
- **M3**: 5 days
- **M4**: 6 days
- **M5**: 4 days
- **M6**: 2 days
- **M7**: 2 days
- **Total**: ~26 days (1 sprint = ~4 weeks with buffer)

### **Team Allocation** (suggested):
- **Developer 1**: M1, M3 (RTS core)
- **Developer 2**: M2, M4 (Telemetry + GA)
- **Developer 3**: M5, M7 (Testing + Monitoring)
- **Technical Writer**: M6 (Documentation)

### **Risk Mitigation**:
1. **Risk**: RTS performs worse than Round-Robin
   - **Mitigation**: Fallback mechanism ensures no regression
2. **Risk**: GA doesn't converge
   - **Mitigation**: Use sensible defaults, make GA optional
3. **Risk**: Performance overhead
   - **Mitigation**: Benchmark early (Task 5.5), optimize hot paths

---

**This sprint plan is complete and ready for implementation. All tasks map directly to existing codebase structure and follow EDD specifications exactly.**
