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
**Status**: ✅ COMPLETE
**Files Modified**: 1
**Documentation**: [TASK_4_7_MASTER_INTEGRATION.md](./TASK_4_7_MASTER_INTEGRATION.md), [MASTER_INTEGRATION_QUICK_REF.md](./MASTER_INTEGRATION_QUICK_REF.md)

**File(s)**:
- `master/main.go` (MODIFIED)

**Functions added/modified**:
- Modified `main()` to initialize HistoryDB and start GA epoch ticker
- Added AOD/GA epoch goroutine running every 60 seconds
- Added graceful shutdown for HistoryDB

**Implementation completed**:
1. In `main.go`, after starting queue processor:
   - Create `historyDB = db.NewHistoryDB(ctx, cfg)` with error handling
   - Load `gaConfig = aod.GetDefaultGAConfig()`
   - Log GA configuration details (population size, generations, mutation rate, etc.)
   - Start goroutine with 60-second ticker:
     ```go
     go func() {
         ticker := time.NewTicker(60 * time.Second)
         defer ticker.Stop()
         for range ticker.C {
             log.Println("🧬 Starting AOD/GA epoch...")
             if err := aod.RunGAEpoch(context.Background(), historyDB, gaConfig, paramsPath); err != nil {
                 log.Printf("❌ AOD/GA epoch error: %v", err)
             } else {
                 log.Println("✅ AOD/GA epoch completed successfully")
             }
         }
     }()
     ```
   - Add HistoryDB cleanup in shutdown handler

2. Graceful degradation:
   - If HistoryDB unavailable, logs warning and disables AOD/GA training
   - RTS continues using default parameters from `config/ga_output.json`
   - System remains operational

3. Complete feedback loop established:
   - RTS scheduling → Task execution → MongoDB history
   - GA epoch (60s) → Parameter optimization → JSON save
   - RTS hot-reload (30s) → Improved scheduling
   - Continuous learning and adaptation

**Test Results**: All 119 AOD tests passing (0.007s)

---

## **🎉 MILESTONE 4 COMPLETE: AOD/GA Module Implementation**

**Status**: ✅ ALL TASKS COMPLETE (7/7)

| Task | Status | Tests | Documentation |
|------|--------|-------|---------------|
| 4.1 AOD Data Models | ✅ | 13/13 | ✅ |
| 4.2 Theta Trainer | ✅ | 11/11 | ✅ |
| 4.3 Affinity Builder | ✅ | 13/13 | ✅ |
| 4.4 Penalty Builder | ✅ | 13/13 | ✅ |
| 4.5 Fitness Function | ✅ | 11/11 (38+ sub-tests) | ✅ |
| 4.6 GA Runner | ✅ | 14/14 | ✅ |
| 4.7 Master Integration | ✅ | Verified with all tests | ✅ |

**Total AOD Tests**: 119/119 passing ✅

**System Capabilities**:
- ✅ Real-time task scheduling with RTS
- ✅ Continuous parameter optimization via GA
- ✅ Self-improving performance over time
- ✅ Graceful fallback mechanisms
- ✅ Production-ready monitoring and logging
- ✅ Hot-reload of optimized parameters
- ✅ Complete feedback loop from execution to optimization

**Key Achievements**:
1. **Adaptive Scheduling**: System learns optimal parameters from execution history
2. **Zero-Downtime Optimization**: GA runs in background without blocking scheduling
3. **Automatic Tuning**: No manual parameter adjustment required
4. **Robust Fallback**: Works with defaults if insufficient data or MongoDB unavailable
5. **Comprehensive Testing**: All components validated with unit tests

**Next Milestone**: Testing & Validation (Milestone 5)

---

## **MILESTONE 5: Testing & Validation**
**Goal**: Verify RTS+GA system works end-to-end using Master CLI commands.

---

### **Task 5.1: CLI-Based Manual Testing Procedure**
**Status**: 🎯 CURRENT FOCUS
**Documentation**: `docs/Scheduler/TASK_5_1_CLI_TESTING_GUIDE.md` (NEW)

**Available Master CLI Commands** (from production logs):
```
Task Submission:
  task <docker_img> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>] [-k <1.5-2.5>] [-type <task_type>]
  dispatch <worker_id> <docker_img> [options]

Task Monitoring:
  list-tasks [status]           - List tasks (pending/running/completed/failed)
  monitor <task_id>             - Live logs for task
  cancel <task_id>              - Cancel running task
  queue                         - Show pending tasks in queue

Worker Management:
  workers                       - List all workers
  stats <worker_id>             - Detailed worker stats
  internal-state                - Dump all in-memory state
  fix-resources                 - Fix stale allocations
  register <id> <ip:port>       - Manual registration
  unregister <id>               - Unregister worker

System Status:
  status                        - Cluster status
  help                          - Command help
```

**Task Types Supported**:
- `cpu-light` - Light CPU workloads
- `cpu-heavy` - Heavy CPU workloads
- `memory-heavy` - Memory-intensive workloads
- `gpu-inference` - GPU inference workloads
- `gpu-training` - GPU training workloads
- `mixed` - Mixed workloads

**Implementation Plan**:

1. **Create CLI Testing Guide** (`docs/Scheduler/TASK_5_1_CLI_TESTING_GUIDE.md`):
   - Step-by-step manual testing procedures
   - Test scenarios for each of the 6 task types
   - Validation criteria for RTS vs Round-Robin
   - How to interpret outputs and verify SLA tracking
   - GA convergence verification steps

2. **Create Test Workload Scripts** (`test/workloads/`):
   - `submit_cpu_light.sh` - 10 cpu-light tasks
   - `submit_cpu_heavy.sh` - 10 cpu-heavy tasks
   - `submit_memory_heavy.sh` - 10 memory-heavy tasks
   - `submit_gpu_inference.sh` - 10 gpu-inference tasks
   - `submit_gpu_training.sh` - 10 gpu-training tasks
   - `submit_mixed.sh` - 10 mixed tasks
   - `submit_full_workload.sh` - 60 tasks (10 of each type)

3. **Create Verification Scripts** (`test/verify/`):
   - `check_task_distribution.sh` - Verify tasks distributed across workers
   - `check_sla_violations.sh` - Query MongoDB for SLA success rates
   - `check_tau_updates.sh` - Verify tau values updated per task type
   - `check_ga_output.sh` - Parse ga_output.json for affinity matrix structure
   - `check_rts_fallback.sh` - Verify fallback to Round-Robin when needed

**Example Test Commands**:
```bash
# Submit CPU-light task with explicit type
master> task moinvinchhi/cloudai-cpu-intensive:1 -cpu_cores 1 -mem 2 -type cpu-light

# Submit GPU-training task
master> task moinvinchhi/cloudai-gpu-intensive:4 -gpu_cores 2 -mem 16 -k 2.5 -type gpu-training

# Monitor task execution
master> list-tasks running
master> monitor task-123
master> stats worker-1

# Check queue and system status
master> queue
master> status
master> internal-state
```

---

### **Task 5.2: RTS vs Round-Robin Comparison**
**Documentation**: `docs/Scheduler/TASK_5_2_SCHEDULER_COMPARISON.md` (NEW)

**Goal**: Manually compare RTS and Round-Robin schedulers using CLI commands.

**Test Procedure**:

1. **Baseline: Round-Robin Testing** (Requires code change to disable RTS):
   - Temporarily modify `master/main.go` to use `rrScheduler` instead of `rtsScheduler`
   - Rebuild: `cd master && go build -o masterNode .`
   - Start master: `./runMaster.sh`
   - Submit 60 mixed tasks using CLI (10 of each type)
   - Record metrics:
     - Task completion times (from MongoDB)
     - SLA violations per task type
     - Worker utilization (from `stats <worker_id>`)
     - Task distribution (from `internal-state`)

2. **Comparison: RTS Testing** (Default configuration):
   - Restore RTS scheduler (use git stash/unstash)
   - Rebuild and restart master
   - Submit same 60 tasks
   - Record same metrics
   - Compare results

3. **Metrics Collection**:
   - Create MongoDB queries to extract:
     - SLA success rate per task type
     - Average task completion time per type
     - Worker load distribution
     - Task assignment patterns

4. **Create Comparison Script** (`test/compare_schedulers.sh`):
   ```bash
   #!/bin/bash
   # Queries MongoDB and compares:
   # - SLA violation rates (RR vs RTS)
   # - Task makespan (RR vs RTS)
   # - Worker utilization (RR vs RTS)
   # - Generates comparison report
   ```

**Expected Outcomes**:
- RTS should have ≤ SLA violations vs Round-Robin
- RTS should achieve higher worker utilization
- RTS should assign tasks based on affinity (once GA trains)
- Fallback to Round-Robin should work when no feasible workers

---

### **Task 5.3: GA Convergence Verification**
**Documentation**: `docs/Scheduler/TASK_5_3_GA_CONVERGENCE.md` (NEW)

**Goal**: Verify GA learns correct parameters from execution history.

**Test Procedure**:

1. **Initial State Verification**:
   ```bash
   # Check default parameters
   cat master/config/ga_output.json
   # Should show empty AffinityMatrix and PenaltyVector
   ```

2. **Generate Training Data** (60+ tasks across 6 types):
   ```bash
   master> task moinvinchhi/cloudai-cpu-intensive:1 -type cpu-light -cpu_cores 1 -mem 2
   # (Repeat for 10 tasks)
   master> task moinvinchhi/cloudai-cpu-intensive:8 -type cpu-heavy -cpu_cores 8 -mem 4
   # (Repeat for 10 tasks)
   # ... continue for all 6 types
   ```

3. **Wait for GA Epoch** (60 seconds):
   - Watch master logs for: `🧬 Starting AOD/GA epoch...`
   - Should log: `✓ Retrieved X task history records and Y worker stats`
   - If X < 10: Will use defaults (insufficient data)
   - If X ≥ 10: GA will train and update parameters

4. **Verify GA Output**:
   ```bash
   # After GA epoch completes
   cat master/config/ga_output.json | jq .
   
   # Verify structure:
   # - AffinityMatrix should have 6 keys (task types)
   # - Each task type should have worker IDs as subkeys
   # - PenaltyVector should have worker IDs as keys
   # - Theta values should be updated from training
   ```

5. **Verify RTS Hot-Reload**:
   - Watch logs for: `✓ RTS: Reloaded GA parameters from config/ga_output.json`
   - Should occur every 30 seconds
   - New tasks should use updated parameters

6. **Create Verification Script** (`test/verify_ga_convergence.sh`):
   ```bash
   #!/bin/bash
   # 1. Submit 60 mixed tasks
   # 2. Wait for GA epoch (60s)
   # 3. Parse ga_output.json
   # 4. Verify affinity matrix structure
   # 5. Submit 60 more tasks
   # 6. Compare SLA violation rates (should improve)
   ```

**Expected Outcomes**:
- After 10+ completed tasks: GA should train successfully
- Affinity matrix should have 6 rows (one per task type)
- Subsequent tasks should have better SLA success rates
- Theta values should converge toward optimal values

---

### **Task 5.4: End-to-End Integration Test**
**Documentation**: `docs/Scheduler/TASK_5_4_INTEGRATION_TEST.md` (NEW)

**Goal**: Validate complete RTS+GA workflow with real workers and Docker execution.

**Test Procedure**:

1. **Setup Multi-Worker Cluster**:
   ```bash
   # Terminal 1: Start master
   ./runMaster.sh
   
   # Terminal 2: Start worker-1
   ./runWorker.sh
   
   # Terminal 3: Start worker-2 (on different machine/laptop)
   ./runWorker.sh
   
   # Verify registration
   master> workers
   # Should show 2+ workers
   ```

2. **Test Task Type Inference**:
   ```bash
   # Submit without explicit type (should infer)
   master> task moinvinchhi/cloudai-cpu-intensive:1 -cpu_cores 1 -mem 2
   # Check MongoDB: task should have type="cpu-light"
   
   master> task moinvinchhi/cloudai-gpu-intensive:5 -gpu_cores 3 -mem 12
   # Should infer type="gpu-training"
   ```

3. **Test Explicit Task Type**:
   ```bash
   # Submit with explicit type
   master> task myapp:latest -cpu_cores 4 -mem 8 -type cpu-heavy
   # Verify type is preserved (not overwritten by inference)
   ```

4. **Test SLA Tracking**:
   ```bash
   # Submit tasks with different k values
   master> task sample:v1 -cpu_cores 2 -mem 4 -k 1.5 -type cpu-light
   master> task sample:v1 -cpu_cores 2 -mem 4 -k 2.5 -type cpu-light
   
   # Monitor completion
   master> list-tasks running
   master> list-tasks completed
   
   # Check MongoDB for SLA success/failure
   # Query: db.results.find({}, {task_id:1, sla_success:1, completed_at:1})
   ```

5. **Test Tau Updates**:
   ```bash
   # Before tasks: Check initial tau values in logs
   # Submit 10 cpu-light tasks
   # After completion: Tau should update
   # Logs should show: "Updated tau for cpu-light: X.XX seconds"
   ```

6. **Test Load Balancing**:
   ```bash
   # Submit 20 tasks rapidly
   master> internal-state
   # Verify tasks distributed across workers
   # RTS should balance based on load
   ```

7. **Test Fallback to Round-Robin**:
   ```bash
   # Submit task requiring 100 CPU cores (infeasible)
   master> task heavy:latest -cpu_cores 100 -mem 200
   # Should fallback to Round-Robin
   # Logs: "⚠️  RTS: No feasible workers, falling back to Round-Robin"
   ```

**Expected Outcomes**:
- All task types submit successfully
- Type inference works correctly
- Explicit types are preserved
- SLA tracking records success/failure
- Tau updates per task type
- RTS balances load across workers
- Fallback works for infeasible tasks

---

### **Task 5.5: Performance & Stress Testing**
**Documentation**: `docs/Scheduler/TASK_5_5_PERFORMANCE_TEST.md` (NEW)

**Goal**: Test system under load and measure performance characteristics.

**Test Scenarios**:

1. **High-Volume Task Submission** (100 tasks in 1 minute):
   ```bash
   # Create script: test/stress_test.sh
   for i in {1..100}; do
     echo "task moinvinchhi/cloudai-cpu-intensive:$((i % 12 + 1)) -cpu_cores $((i % 8 + 1)) -mem $((i % 16 + 2)) -type cpu-light"
   done | timeout 60 nc localhost 50051
   ```

2. **Concurrent Worker Load** (3+ workers, 200 tasks):
   - Start 3+ workers on different machines
   - Submit 200 tasks via CLI
   - Monitor: `master> status` (every 10s)
   - Verify: Tasks complete without deadlock

3. **GA Training Under Load**:
   - Submit 100 tasks
   - Let 20+ complete
   - Wait for GA epoch
   - Verify: GA completes without blocking new submissions
   - Check logs: AOD epoch duration should be < 5s

4. **Resource Reconciliation Test**:
   ```bash
   # Simulate stale allocations
   master> fix-resources
   # Should log: "✓ Resource reconciliation complete"
   
   # Check workers still responsive
   master> workers
   master> stats worker-1
   ```

5. **Long-Running Stability** (8 hours):
   - Start master + 2 workers
   - Submit 500 tasks over 8 hours (1 task/minute script)
   - Monitor for:
     - Memory leaks (check master process RSS)
     - GA epochs completing successfully
     - Worker disconnections/reconnections
     - Task queue not growing unbounded

**Performance Metrics to Collect**:
- Task submission latency (CLI response time)
- Scheduling decision time (from RTS logs)
- GA epoch duration (from AOD logs)
- Worker registration time
- Task completion rate (tasks/hour)

**Expected Outcomes**:
- System handles 100 tasks/minute without degradation
- RTS scheduling decision < 10ms
- GA epoch completes in < 5s with 100+ history records
- No memory leaks over 8 hours
- Queue drains correctly (no tasks stuck)

---

## **MILESTONE 6: Documentation & User Guides**
**Goal**: Complete documentation for operators and users.

---

### **Task 6.1: CLI User Guide**
**File(s)**:
- `docs/Scheduler/CLI_USER_GUIDE.md` (NEW)

**Content**:
- **Master CLI Commands Reference**:
  - Complete command syntax with examples
  - Task submission: `task` vs `dispatch`
  - Worker management: `register`, `unregister`, `stats`
  - Monitoring: `monitor`, `list-tasks`, `queue`, `status`
  - Resource management: `fix-resources`, `internal-state`

- **Task Type Guide**:
  - Explanation of 6 task types (cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed)
  - When to use explicit `-type` flag
  - How task type affects scheduling
  - Task type inference rules
  - Best practices for each type

- **SLA Configuration**:
  - Understanding `-k` parameter (SLA multiplier: 1.5-2.5)
  - How deadlines are computed: `deadline = now + k × τ`
  - Default k value: 2.0
  - When to use tight vs loose SLAs

- **Common Workflows**:
  ```bash
  # Submit batch of CPU tasks
  task app:v1 -cpu_cores 4 -mem 8 -type cpu-heavy
  
  # Submit GPU training job
  task ml-model:latest -gpu_cores 2 -mem 16 -k 2.5 -type gpu-training
  
  # Monitor execution
  list-tasks running
  monitor task-123
  stats worker-1
  
  # Check cluster health
  status
  workers
  queue
  ```

---

### **Task 6.2: Operator Guide**
**File(s)**:
- `docs/Scheduler/OPERATOR_GUIDE.md` (NEW)

**Content**:
- **System Monitoring**:
  - Master logs interpretation
  - Key log messages to watch:
    - `✓ RTS: Reloaded GA parameters` (every 30s)
    - `🧬 Starting AOD/GA epoch` (every 60s)
    - `⚠️ RTS: No feasible workers, falling back to Round-Robin`
    - `✓ Retrieved X task history records and Y worker stats`
  - Using `internal-state` for debugging
  - MongoDB queries for telemetry analysis

- **GA Parameter Tuning**:
  - Understanding `ga_output.json` structure
  - Theta values (execution time prediction)
  - Risk parameters (Alpha for SLA, Beta for load)
  - Affinity weights (A1, A2, A3)
  - Penalty weights (G1, G2, G3)
  - When to manually adjust vs let GA learn

- **Troubleshooting Guide**:
  
  **Issue**: All tasks assigned to one worker
  - **Check**: `cat master/config/ga_output.json | jq .AffinityMatrix`
  - **Cause**: Affinity heavily favors that worker for common task types
  - **Solution**: Adjust AffinityW weights or add more diverse task history

  **Issue**: High SLA violations
  - **Check**: MongoDB query for SLA success rate per task type
  - **Cause**: Tight deadlines (low k) or poor θ prediction
  - **Solution**: Increase k parameter, let GA train more, check worker loads

  **Issue**: Workers underutilized
  - **Check**: `master> stats worker-1` for all workers
  - **Cause**: Beta parameter too high (avoids loaded workers)
  - **Solution**: Decrease Beta in Risk parameters

  **Issue**: GA not training
  - **Check**: Logs show "Insufficient data (X tasks < 10 required)"
  - **Cause**: Not enough completed tasks in last 24h
  - **Solution**: Submit more tasks, wait for completions

  **Issue**: Tasks stuck in queue
  - **Check**: `master> queue`
  - **Cause**: No feasible workers or all workers offline
  - **Solution**: Check worker status, use `fix-resources`

- **Performance Optimization**:
  - Ideal worker:task ratio (1:10 to 1:50)
  - GA training frequency tuning
  - MongoDB query optimization
  - Resource reconciliation schedule

---

### **Task 6.3: Configuration Reference**
**File(s)**:
- `docs/Scheduler/CONFIGURATION_REFERENCE.md` (NEW)

**Content**:
- **Environment Variables**:
  ```bash
  # Master Configuration
  MASTER_PORT=:50051                    # gRPC port
  HTTP_PORT=8080                        # HTTP/WebSocket port
  
  # Scheduler Configuration
  SCHED_SLA_MULTIPLIER=2.0              # Default k for SLA deadlines
  RTS_PARAMS_PATH=config/ga_output.json # GA parameters file
  RTS_RELOAD_INTERVAL=30s               # Parameter hot-reload frequency
  
  # GA Configuration
  GA_POPULATION_SIZE=20                 # Population size
  GA_GENERATIONS=10                     # Generations per epoch
  GA_MUTATION_RATE=0.1                  # Mutation probability
  GA_CROSSOVER_RATE=0.7                 # Crossover probability
  GA_ELITISM_COUNT=2                    # Elite chromosomes preserved
  GA_TOURNAMENT_SIZE=3                  # Tournament selection size
  GA_EPOCH_INTERVAL=60s                 # Training frequency
  GA_HISTORY_WINDOW=24h                 # Training data window
  
  # Database Configuration
  MONGO_URI=mongodb://localhost:27017
  MONGO_DB=cloudai
  ```

- **ga_output.json Schema**:
  ```json
  {
    "Theta": {
      "Theta1": 0.1,    // CPU ratio weight
      "Theta2": 0.1,    // Memory ratio weight
      "Theta3": 0.3,    // GPU ratio weight
      "Theta4": 0.2     // Load weight
    },
    "Risk": {
      "Alpha": 10,      // SLA penalty multiplier
      "Beta": 1         // Load penalty multiplier
    },
    "AffinityW": {
      "A1": 1,          // Speed weight
      "A2": 2,          // SLA reliability weight
      "A3": 0.5         // Overload rate weight
    },
    "PenaltyW": {
      "G1": 2,          // SLA failure weight
      "G2": 1,          // Overload weight
      "G3": 0.5         // Energy weight
    },
    "AffinityMatrix": {
      "cpu-light": {
        "worker-1": 2.5,
        "worker-2": 1.8
      },
      "gpu-training": {
        "worker-1": -1.0,
        "worker-2": 3.2
      }
      // ... 6 task types total
    },
    "PenaltyVector": {
      "worker-1": 0.5,
      "worker-2": 1.2
    }
  }
  ```

- **Task Type Definitions**:
  - `cpu-light`: < 4 CPU cores, < 8GB RAM, no GPU
  - `cpu-heavy`: ≥ 4 CPU cores, < 8GB RAM, no GPU
  - `memory-heavy`: ≥ 8GB RAM, no GPU
  - `gpu-inference`: Any GPU, < 2 GPU cores
  - `gpu-training`: ≥ 2 GPU cores
  - `mixed`: Complex workloads with multiple resource types

---

### **Task 6.4: Testing Documentation**
**File(s)**:
- `docs/Scheduler/TESTING_GUIDE.md` (NEW)

**Content**:
- Links to all testing documentation:
  - Task 5.1: CLI Testing Guide
  - Task 5.2: Scheduler Comparison
  - Task 5.3: GA Convergence Verification
  - Task 5.4: Integration Testing
  - Task 5.5: Performance Testing

- **Quick Test Checklist**:
  ```bash
  # 1. Start master and verify initialization
  ./runMaster.sh
  # Check logs for ✓ marks
  
  # 2. Register workers
  master> workers
  # Should show registered workers
  
  # 3. Submit test task
  master> task moinvinchhi/cloudai-cpu-intensive:1 -type cpu-light
  
  # 4. Monitor execution
  master> list-tasks running
  master> monitor <task_id>
  
  # 5. Verify completion
  master> list-tasks completed
  
  # 6. Check GA training (after 60s)
  cat master/config/ga_output.json | jq .
  
  # 7. Verify RTS hot-reload (after 90s)
  # Check logs for parameter reload message
  ```

- **Test Scripts Location**:
  - `test/workloads/` - Task submission scripts
  - `test/verify/` - Verification scripts
  - `test/compare_schedulers.sh` - RTS vs RR comparison
  - `test/verify_ga_convergence.sh` - GA validation

---

## **MILESTONE 7: Monitoring & Production Readiness**
**Goal**: Add observability and production-ready features using CLI-based monitoring.

---

### **Task 7.1: Enhanced CLI Monitoring Commands**
**File(s)**:
- `master/internal/cli/cli.go` (MODIFY)

**Functions to add/modify**:
- `func handleRTSStats(s *server.MasterServer)` (NEW)
- `func handleGAStats(s *server.MasterServer)` (NEW)
- `func handleTaskTypeStats(s *server.MasterServer)` (NEW)
- `func handleSLAReport(s *server.MasterServer)` (NEW)

**New CLI Commands**:
```bash
master> rts-stats                     # Show RTS scheduling statistics
master> ga-stats                      # Show GA training statistics
master> task-type-stats               # Show task distribution by type
master> sla-report [hours]            # SLA violation report (default: last 24h)
```

**Implementation**:
1. **rts-stats** command output:
   ```
   RTS Scheduler Statistics:
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Total Scheduling Decisions: 156
   RTS Selections: 142 (91.0%)
   Round-Robin Fallbacks: 14 (9.0%)
   
   Average Decision Time: 2.3ms
   Feasible Worker Rate: 95.2%
   
   Current Parameters:
   - Theta: (0.12, 0.15, 0.28, 0.22)
   - Risk: (α=10.0, β=1.2)
   - Affinity Types: 6/6 task types
   - Penalty Workers: 3/3 workers
   
   Last Parameter Reload: 15 seconds ago
   Next GA Epoch: 35 seconds
   ```

2. **ga-stats** command output:
   ```
   GA Training Statistics:
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Total Epochs Completed: 8
   Last Epoch: 45 seconds ago
   Next Epoch: 15 seconds
   
   Training Data (Last Epoch):
   - Task History Records: 67
   - Worker Stats Records: 3
   - Training Duration: 1.2s
   
   Best Fitness: 0.854 (+12.3% vs baseline)
   
   Task Type Distribution:
   - cpu-light: 15 tasks
   - cpu-heavy: 12 tasks
   - memory-heavy: 8 tasks
   - gpu-inference: 18 tasks
   - gpu-training: 10 tasks
   - mixed: 4 tasks
   
   Affinity Matrix: 6x3 (6 types, 3 workers)
   Penalty Vector: 3 workers
   ```

3. **task-type-stats** command output:
   ```
   Task Type Statistics:
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Task Type         | Total | Pending | Running | Completed | Failed | SLA Success
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   cpu-light         |   45  |    2    |    3    |    38     |   2    |   92.1%
   cpu-heavy         |   32  |    1    |    2    |    28     |   1    |   87.5%
   memory-heavy      |   18  |    0    |    1    |    16     |   1    |   88.9%
   gpu-inference     |   28  |    3    |    4    |    20     |   1    |   90.9%
   gpu-training      |   15  |    1    |    2    |    11     |   1    |   84.6%
   mixed             |   12  |    0    |    1    |    10     |   1    |   90.9%
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   TOTAL             |  150  |    7    |   13    |   123     |   7    |   89.1%
   
   Tau Values (Average Runtime):
   - cpu-light: 4.8s
   - cpu-heavy: 16.2s
   - memory-heavy: 22.5s
   - gpu-inference: 12.3s
   - gpu-training: 58.7s
   - mixed: 11.4s
   ```

4. **sla-report** command output:
   ```bash
   master> sla-report 24
   
   SLA Report (Last 24 Hours):
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Overall SLA Success Rate: 89.1% (123/138 tasks)
   
   By Task Type:
   - cpu-light: 92.1% (35/38)
   - cpu-heavy: 87.5% (28/32)
   - memory-heavy: 88.9% (16/18)
   - gpu-inference: 90.9% (20/22)
   - gpu-training: 84.6% (11/13)
   - mixed: 90.9% (10/11)
   
   By Worker:
   - worker-1: 91.2% (52/57)
   - worker-2: 88.5% (46/52)
   - worker-3: 86.2% (25/29)
   
   Recent SLA Violations (Last 10):
   1. task-145 (gpu-training) - worker-2 - Late by 15.2s
   2. task-138 (cpu-heavy) - worker-1 - Late by 8.7s
   3. task-129 (memory-heavy) - worker-3 - Late by 22.4s
   ...
   ```

**Implementation Steps**:
1. Add new command handlers in `cli.go`
2. Query MongoDB for statistics
3. Format output with aligned tables
4. Add to help menu

---

### **Task 7.2: Automated Health Checks**
**File(s)**:
- `master/internal/health/health_check.go` (NEW)
- `master/internal/cli/cli.go` (MODIFY for new command)

**Functions to add/modify**:
- `func RunHealthCheck(ctx context.Context, s *server.MasterServer) *HealthReport`
- `func CheckWorkerHealth(workers map[string]*WorkerInfo) []WorkerHealthStatus`
- `func CheckGAHealth(historyDB *db.HistoryDB) GAHealthStatus`
- `func CheckRTSHealth(scheduler *scheduler.RTSScheduler) RTSHealthStatus`

**New CLI Command**:
```bash
master> health-check                  # Run comprehensive health check
```

**Health Check Output**:
```
System Health Check:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ Master Node: HEALTHY
  - Uptime: 4h 23m
  - Memory: 234MB / 8GB (2.9%)
  - gRPC: Listening on :50051
  - HTTP: Listening on :8080

✓ Workers: HEALTHY (3/3 online)
  - worker-1: HEALTHY (Load: 42%, CPU: 8/16, Mem: 12/32GB)
  - worker-2: HEALTHY (Load: 35%, CPU: 6/16, Mem: 8/32GB)
  - worker-3: HEALTHY (Load: 18%, CPU: 3/16, Mem: 4/32GB)

✓ Task Queue: HEALTHY
  - Pending: 7 tasks
  - Processing: 13 tasks
  - Average Wait Time: 2.3s

✓ RTS Scheduler: HEALTHY
  - Decision Time: 2.1ms (avg)
  - Fallback Rate: 8.2%
  - Parameters Loaded: 35s ago

⚠ GA Training: WARNING
  - Last Epoch: 45s ago (GOOD)
  - Training Data: 67 tasks (GOOD)
  - Missing Task Types: 0 (GOOD)
  - Fitness Trend: Declining ⚠️

✓ Database: HEALTHY
  - MongoDB: Connected
  - Collections: 5/5 accessible
  - Avg Query Time: 12ms

✓ SLA Performance: GOOD
  - Success Rate: 89.1%
  - Target: > 85%

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Overall Status: HEALTHY (1 warning)
```

---

### **Task 7.3: Logging Enhancements**
**File(s)**:
- `master/internal/scheduler/rts_scheduler.go` (MODIFY)
- `master/internal/aod/ga_runner.go` (MODIFY)

**Improvements**:
1. **Structured Logging** with context:
   ```go
   log.Printf("[RTS] [task=%s] [type=%s] Selected worker=%s (risk=%.2f, feasible=%d/%d)",
       taskID, taskType, selectedWorker, bestRisk, feasibleCount, totalWorkers)
   ```

2. **Debug Mode** (env var: `LOG_LEVEL=debug`):
   ```go
   if debugEnabled {
       log.Printf("[RTS] [DEBUG] Risk calculation for worker=%s: base=%.2f, affinity=%.2f, penalty=%.2f, final=%.2f",
           workerID, baseRisk, affinity, penalty, finalRisk)
   }
   ```

3. **GA Training Logs**:
   ```go
   log.Printf("[GA] [epoch=%d] Generation %d/%d: best_fitness=%.4f, avg_fitness=%.4f",
       epochNum, gen, totalGens, bestFitness, avgFitness)
   ```

4. **Performance Logs**:
   ```go
   log.Printf("[PERF] RTS decision took %dms for task=%s (feasible=%d)",
       duration.Milliseconds(), taskID, feasibleCount)
   ```

---

### **Task 7.4: Alerting & Notifications**
**File(s)**:
- `master/internal/alerts/alert_manager.go` (NEW)

**Functions to add/modify**:
- `func CheckSLAThreshold(ctx context.Context, db *db.HistoryDB) error`
- `func CheckWorkerAvailability(workers map[string]*WorkerInfo) error`
- `func CheckGATrainingFailures(gaStatus GAStatus) error`

**Alert Conditions**:
1. **Critical Alerts** (log ERROR + could send webhook):
   - SLA success rate < 70% for any task type
   - All workers offline
   - GA training failed 3+ consecutive epochs
   - Task queue > 100 pending tasks

2. **Warning Alerts** (log WARN):
   - SLA success rate < 85% for any task type
   - Worker offline for > 5 minutes
   - GA training data < 10 tasks
   - Tau values not updating (stale > 1 hour)

3. **Info Alerts** (log INFO):
   - New worker registered
   - GA training completed successfully
   - RTS parameters reloaded
   - Worker resource reconciliation

**Log Format**:
```
2025/11/17 10:15:23 ⚠️ [ALERT] [WARNING] SLA success rate below threshold: gpu-training=82.3% (target: >85%)
2025/11/17 10:15:45 ❌ [ALERT] [CRITICAL] Worker worker-2 offline for 6 minutes
2025/11/17 10:16:12 ✓ [ALERT] [INFO] GA training completed: fitness=0.854 (+12.3%)
```

---

### **Task 7.5: MongoDB Query Optimization**
**File(s)**:
- `master/internal/db/history.go` (MODIFY)
- `master/internal/db/tasks.go` (MODIFY)

**Optimizations**:
1. **Add Database Indexes**:
   ```go
   // In InitHistoryDB:
   db.Collection("tasks").Indexes().CreateMany(ctx, []mongo.IndexModel{
       {Keys: bson.D{{"task_type", 1}}},
       {Keys: bson.D{{"completed_at", -1}}},
       {Keys: bson.D{{"worker_id", 1}, {"task_type", 1}}},
       {Keys: bson.D{{"sla_success", 1}, {"task_type", 1}}},
   })
   ```

2. **Optimize GA Training Query**:
   ```go
   // Use aggregation pipeline for faster stats computation
   pipeline := mongo.Pipeline{
       {{"$match", bson.D{{"completed_at", bson.D{{"$gte", since}, {"$lte", until}}}}}},
       {{"$group", bson.D{
           {"_id", bson.D{{"task_type", "$task_type"}, {"worker_id", "$worker_id"}}},
           {"count", bson.D{{"$sum", 1}}},
           {"sla_success_count", bson.D{{"$sum", bson.D{{"$cond", bson.A{"$sla_success", 1, 0}}}}}},
       }}},
   }
   ```

3. **Add Query Timeouts**:
   ```go
   ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
   defer cancel()
   ```

---

### **Task 7.6: Production Deployment Script**
**File(s)**:
- `deploy_production.sh` (NEW)
- `check_production_health.sh` (NEW)

**deploy_production.sh**:
```bash
#!/bin/bash
set -e

echo "🚀 Deploying CloudAI Master (Production Mode)"

# 1. Backup current ga_output.json
if [ -f master/config/ga_output.json ]; then
    cp master/config/ga_output.json master/config/ga_output.json.backup
    echo "✓ Backed up GA parameters"
fi

# 2. Build master
cd master
go build -o masterNode .
cd ..
echo "✓ Master built successfully"

# 3. Verify MongoDB connection
echo "Checking MongoDB connection..."
if ! mongo --eval "db.adminCommand('ping')" > /dev/null 2>&1; then
    echo "❌ MongoDB not accessible"
    exit 1
fi
echo "✓ MongoDB accessible"

# 4. Verify configuration
if [ ! -f .env ]; then
    echo "❌ Missing .env file"
    exit 1
fi
echo "✓ Configuration file present"

# 5. Start master in background
echo "Starting master node..."
nohup ./runMaster.sh > master.log 2>&1 &
MASTER_PID=$!
echo "✓ Master started (PID: $MASTER_PID)"

# 6. Wait for initialization
echo "Waiting for master to initialize..."
sleep 5

# 7. Health check
if ! grep -q "✓ Master node started successfully" master.log; then
    echo "❌ Master failed to start. Check master.log"
    exit 1
fi
echo "✓ Master initialized successfully"

# 8. Run health check
echo "Running health check..."
./check_production_health.sh

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Deployment complete!"
echo "Master PID: $MASTER_PID"
echo "Logs: tail -f master.log"
echo "CLI: ./runMaster.sh (will connect to existing instance)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
```

**check_production_health.sh**:
```bash
#!/bin/bash

echo "🏥 CloudAI Production Health Check"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check master process
if pgrep -f "masterNode" > /dev/null; then
    echo "✓ Master process running"
else
    echo "❌ Master process not running"
    exit 1
fi

# Check MongoDB
if mongo --eval "db.adminCommand('ping')" > /dev/null 2>&1; then
    echo "✓ MongoDB accessible"
else
    echo "❌ MongoDB not accessible"
    exit 1
fi

# Check gRPC port
if netstat -tuln | grep -q ":50051"; then
    echo "✓ gRPC port listening (:50051)"
else
    echo "❌ gRPC port not listening"
    exit 1
fi

# Check HTTP port
if netstat -tuln | grep -q ":8080"; then
    echo "✓ HTTP port listening (:8080)"
else
    echo "❌ HTTP port not listening"
    exit 1
fi

# Check recent logs for errors
if tail -100 master.log | grep -q "ERROR\|CRITICAL"; then
    echo "⚠️  Recent errors found in logs"
else
    echo "✓ No recent critical errors"
fi

# Check GA output file
if [ -f master/config/ga_output.json ]; then
    echo "✓ GA parameters file exists"
    
    # Check if affinity matrix has data
    if jq -e '.AffinityMatrix | length > 0' master/config/ga_output.json > /dev/null; then
        echo "✓ GA has trained (affinity matrix populated)"
    else
        echo "⚠️  GA not yet trained (empty affinity matrix)"
    fi
else
    echo "❌ GA parameters file missing"
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Health check complete"
```

---

## **Summary & Dependencies**

### **Critical Path**:
1. ✅ M1 (Foundation) → ✅ M2 (Telemetry) → ✅ M3 (RTS) → ✅ M4 (AOD) → 🎯 M5 (Testing)
2. M6 (Documentation) can proceed in parallel with M5
3. M7 (Monitoring) should be done after M5 is validated

### **Current Status**:
- **Milestone 1**: ✅ COMPLETE (Foundation & Data Models)
- **Milestone 2**: ✅ COMPLETE (Tau Store & Telemetry Enrichment)
- **Milestone 3**: ✅ COMPLETE (RTS Scheduler Implementation)
- **Milestone 4**: ✅ COMPLETE (AOD/GA Module - 119/119 tests passing)
- **Milestone 5**: 🎯 IN PROGRESS (CLI-Based Testing & Validation)
- **Milestone 6**: 📋 PENDING (Documentation & User Guides)
- **Milestone 7**: 📋 PENDING (Monitoring & Production Readiness)

### **Revised Testing Approach (Milestones 5-7)**:

**Why CLI-Based Testing?**
Based on the production logs, the master node provides a rich interactive CLI with comprehensive commands for:
- Task submission with explicit task types (`-type` flag)
- Real-time monitoring (`monitor`, `list-tasks`, `queue`)
- Worker management (`workers`, `stats`, `internal-state`)
- System status (`status`, `fix-resources`)

This CLI-based approach is **more practical** than programmatic testing because:
1. ✅ Tests actual production workflow users will use
2. ✅ No need to write complex gRPC client code
3. ✅ Easier to debug and visualize results
4. ✅ Scripts can automate CLI commands
5. ✅ Validates end-to-end system behavior

**Testing Workflow**:
```
M5: Manual Testing
├── Task 5.1: CLI Testing Procedures (guides + scripts)
├── Task 5.2: RTS vs RR Comparison (CLI-based)
├── Task 5.3: GA Convergence Verification (CLI + MongoDB queries)
├── Task 5.4: End-to-End Integration (multi-worker CLI testing)
└── Task 5.5: Performance & Stress Testing (scripted CLI load)

M6: Documentation
├── Task 6.1: CLI User Guide (command reference)
├── Task 6.2: Operator Guide (troubleshooting, tuning)
├── Task 6.3: Configuration Reference (env vars, JSON schema)
└── Task 6.4: Testing Documentation (links to M5 guides)

M7: Production Features
├── Task 7.1: Enhanced CLI Commands (rts-stats, ga-stats, sla-report)
├── Task 7.2: Automated Health Checks (health-check command)
├── Task 7.3: Logging Enhancements (structured logs, debug mode)
├── Task 7.4: Alerting & Notifications (threshold monitoring)
├── Task 7.5: MongoDB Query Optimization (indexes, timeouts)
└── Task 7.6: Production Deployment Scripts (deploy, health check)
```

### **Estimated Timeline** (Revised):
- **M5 (CLI Testing)**: 4 days
  - Day 1: Create testing guides and scripts (Task 5.1)
  - Day 2: RTS vs RR comparison testing (Task 5.2)
  - Day 3: GA convergence and integration tests (Task 5.3, 5.4)
  - Day 4: Performance and stress testing (Task 5.5)
- **M6 (Documentation)**: 2 days (can parallel with M5)
  - Day 1: CLI user guide and operator guide (Task 6.1, 6.2)
  - Day 2: Configuration and testing docs (Task 6.3, 6.4)
- **M7 (Production)**: 3 days
  - Day 1: Enhanced CLI monitoring commands (Task 7.1, 7.2)
  - Day 2: Health checks and alerting (Task 7.3, 7.4)
  - Day 3: Optimization and deployment scripts (Task 7.5, 7.6)
- **Total**: ~9 days (~2 weeks with buffer)

### **Key Deliverables**:
1. **M5 Deliverables**:
   - ✅ CLI testing guide with step-by-step procedures
   - ✅ Bash scripts for automated workload submission
   - ✅ MongoDB queries for metrics collection
   - ✅ Scheduler comparison report (RTS vs RR)
   - ✅ GA convergence verification report
   - ✅ Performance benchmarks

2. **M6 Deliverables**:
   - ✅ Complete CLI command reference
   - ✅ Operator troubleshooting guide
   - ✅ Configuration schema documentation
   - ✅ Testing procedures documentation

3. **M7 Deliverables**:
   - ✅ Enhanced CLI monitoring commands (4 new commands)
   - ✅ Automated health check system
   - ✅ Production deployment scripts
   - ✅ Structured logging with debug mode
   - ✅ Alert system for critical conditions
   - ✅ Optimized database queries

### **Risk Mitigation**:
1. **Risk**: RTS performs worse than Round-Robin
   - **Mitigation**: ✅ Fallback mechanism ensures no regression (already implemented)
   - **Validation**: Task 5.2 will measure and compare both schedulers

2. **Risk**: GA doesn't converge with real data
   - **Mitigation**: ✅ Default parameters used if insufficient data (already implemented)
   - **Validation**: Task 5.3 will verify convergence with real task history

3. **Risk**: CLI-based testing is manual and slow
   - **Mitigation**: Create bash scripts to automate CLI command sequences
   - **Validation**: Task 5.1 includes script templates for automation

4. **Risk**: Performance issues under load
   - **Mitigation**: Task 5.5 stress tests and Task 7.5 optimizes queries
   - **Validation**: Benchmark against targets (< 10ms RTS decision, < 5s GA epoch)

### **Success Criteria**:
- ✅ **M5**: All 5 test scenarios pass with documented results
- ✅ **M6**: Complete documentation published (4 guides)
- ✅ **M7**: Production features deployed and validated

### **Next Immediate Actions**:
1. 🎯 **Start Task 5.1**: Create CLI testing guide
   - Document test procedures for each of 6 task types
   - Write bash scripts for workload submission
   - Create verification scripts for MongoDB queries

2. 📋 Delete test directory (as user requested):
   - Remove `test/generate_workload.go` (no longer needed)
   - Remove `test/README.md` and `test/QUICK_START.md`
   - Remove `test/build.sh`
   - Keep directory structure for new bash scripts

3. 📝 Begin M6 documentation in parallel:
   - Start CLI_USER_GUIDE.md with command examples from master logs
   - Document task type usage patterns
   - Create configuration reference from env vars

---

**This revised sprint plan is production-focused and aligns with the actual master CLI capabilities shown in the logs. All testing will use real CLI commands that users will actually use in production.**
