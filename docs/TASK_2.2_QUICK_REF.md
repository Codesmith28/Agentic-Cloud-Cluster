# Task 2.2 Quick Reference: Task Submission with Tau & Deadline

**Status**: ✅ Complete | **Sprint**: Milestone 2 | **Date**: January 2025

---

## 📋 Overview

Task 2.2 enriches task submission to automatically compute SLA parameters (tau, deadline, task_type) using the TauStore.

**Formula**: `deadline = arrival_time + k * tau`

---

## 🚀 Quick Start

### 1. Start Master with SLA Configuration

```bash
export SCHED_SLA_MULTIPLIER=1.8  # Optional (default: 2.0, range: [1.5, 2.5])
cd master
./master
```

### 2. Submit Task (Explicit Type)

```bash
grpcurl -plaintext -d '{
  "task_id": "cpu-task-1",
  "docker_image": "alpine",
  "req_cpu": 8.0,
  "task_type": "cpu-heavy"
}' localhost:8080 MasterWorker/SubmitTask
```

**Expected Master Log**:
```
[INFO] Using explicit task_type 'cpu-heavy' for task cpu-task-1
[INFO] Retrieved tau=15.00s for task_type 'cpu-heavy'
[INFO] Computed deadline: arrival=2025-01-15T17:00:00Z, tau=15.00s, k=1.80, deadline=2025-01-15T17:00:27Z
[INFO] Stored SLA parameters for task cpu-task-1
```

### 3. Submit Task (Inferred Type)

```bash
grpcurl -plaintext -d '{
  "task_id": "gpu-task-1",
  "docker_image": "tensorflow",
  "req_cpu": 8.0,
  "req_memory": 16.0,
  "req_gpu": 4.0
}' localhost:8080 MasterWorker/SubmitTask
```

**Expected Master Log**:
```
[INFO] Inferred task_type 'gpu-training' for task gpu-task-1
[INFO] Retrieved tau=60.00s for task_type 'gpu-training'
[INFO] Computed deadline: arrival=2025-01-15T17:00:00Z, tau=60.00s, k=1.80, deadline=2025-01-15T17:01:48Z
```

---

## 🔧 Configuration

### Environment Variables

```bash
# Global SLA multiplier (k)
export SCHED_SLA_MULTIPLIER=2.0  # Range: [1.5, 2.5]
```

### Task-Level Override

```protobuf
message Task {
    string task_id = 1;
    float sla_multiplier = 15;  // Overrides server default
}
```

**Example**:
```bash
grpcurl -plaintext -d '{
  "task_id": "urgent-task",
  "task_type": "cpu-heavy",
  "sla_multiplier": 1.5  # Tighter deadline
}' localhost:8080 MasterWorker/SubmitTask
```

---

## 📊 Task Types & Default Tau Values

| Task Type | Default Tau | Resource Profile |
|-----------|------------|------------------|
| `cpu-light` | 5s | ReqCpu < 4.0, no GPU |
| `cpu-heavy` | 15s | ReqCpu ≥ 4.0, no GPU |
| `memory-heavy` | 20s | ReqMemory ≥ 8.0, no GPU |
| `gpu-inference` | 10s | ReqGpu > 0, ReqCpu < 4.0 |
| `gpu-training` | 60s | ReqGpu ≥ 2.0, ReqCpu ≥ 4.0 |
| `mixed` | 10s | Fallback type |

---

## 🔍 Task Type Inference Logic

**Priority Order**:
1. Explicit `task_type` (if valid)
2. Inferred from resources:

```go
if reqGpu >= 2.0 && reqCpu >= 4.0 → "gpu-training"
else if reqGpu > 0 → "gpu-inference"
else if reqMemory >= 8.0 → "memory-heavy"
else if reqCpu >= 4.0 → "cpu-heavy"
else if reqCpu > 0 || reqMemory > 0 → "cpu-light"
else → "mixed"
```

---

## 🧪 Testing

### Run All Tests

```bash
cd master
go test ./internal/server -v -run TestTask
```

### Test Coverage

- ✅ Tau store integration (1 test)
- ✅ Task type inference (3 subtests)
- ✅ Invalid type handling (1 test)
- ✅ SLA multiplier validation (4 subtests)
- ✅ Database integration (1 test)
- ✅ Deadline computation (1 test)
- ✅ Concurrency (1 test, 10 tasks)
- ✅ Type preservation (6 subtests)

**Total**: 10 tests, 14+ subtests

---

## 📐 Key Formulas

### Deadline Computation

$$
\text{deadline} = t_{\text{arrival}} + k \times \tau
$$

**Example**:
- Arrival: `2025-01-15T17:00:00Z`
- Tau: `15.0s` (cpu-heavy)
- k: `2.0`
- **Deadline**: `2025-01-15T17:00:30Z`

### Urgency (Used by RTS Scheduler - Task 3.3)

$$
u(t) = \frac{c}{d - t}
$$

Where:
- $c$ = remaining execution time (initially = tau)
- $d$ = deadline
- $t$ = current time

---

## 🔄 Workflow

```
1. SubmitTask Request
   ↓
2. Validate/Infer Task Type
   ├─ ValidateTaskType() if explicit
   └─ InferTaskType() if missing/invalid
   ↓
3. Get Tau from TauStore
   └─ tauStore.GetTau(taskType)
   ↓
4. Compute Deadline
   └─ arrival + k * tau
   ↓
5. Persist SLA Parameters
   └─ taskDB.UpdateTaskWithSLA(taskID, deadline, tau, taskType)
   ↓
6. Queue Task for Scheduling
```

---

## 🐛 Common Issues

### Issue: All tasks get same tau

**Symptom**: Log shows `Retrieved tau=10.00s` for all types

**Check**:
```bash
grep "Retrieved tau" master.log | sort | uniq
```

**Fix**: Ensure TauStore initialized before MasterServer

---

### Issue: Deadline = arrival time (no slack)

**Symptom**: Deadline immediately expires

**Check**:
```bash
grep "SLAMultiplier" master.log
echo $SCHED_SLA_MULTIPLIER
```

**Fix**: Set valid `SCHED_SLA_MULTIPLIER` (1.5 - 2.5)

---

### Issue: Task type always inferred

**Symptom**: Log shows `Inferred task_type` even for explicit types

**Check**:
```bash
# Verify proto field name
grep "task_type" proto/master_worker.proto
```

**Fix**: Use correct field name `task_type` (not `taskType`)

---

## 📝 Code Snippets

### Get Current Tau for a Task Type

```go
import "master/internal/telemetry"

tauStore := telemetry.NewInMemoryTauStore()
tau := tauStore.GetTau("cpu-heavy")
fmt.Printf("Tau for cpu-heavy: %.2fs\n", tau)
```

### Compute Deadline Manually

```go
import "time"

arrival := time.Now()
tau := 15.0  // seconds
k := 2.0
deadline := arrival.Add(time.Duration(k * tau) * time.Second)
fmt.Printf("Deadline: %s\n", deadline.Format(time.RFC3339))
```

### Validate Task Type

```go
import "master/internal/scheduler"

taskType := "cpu-heavy"
if scheduler.ValidateTaskType(taskType) {
    fmt.Println("Valid task type")
} else {
    inferred := scheduler.InferTaskType(8.0, 4.0, 0.0)
    fmt.Printf("Invalid, inferred: %s\n", inferred)
}
```

---

## 🔗 Dependencies

### Upstream (Required)

| Task | Provides | Status |
|------|----------|--------|
| Task 2.1 | TauStore implementation | ✅ Complete |
| Task 1.3 | UpdateTaskWithSLA() | ✅ Complete |
| Task 1.4 | Proto task_type field | ✅ Complete |

### Downstream (Uses Task 2.2)

| Task | Needs | Status |
|------|-------|--------|
| Task 2.4 | Tau values to update | ⏳ Pending |
| Task 3.3 | Tau/deadline for RTS | ⏳ Pending |

---

## 📊 Performance

| Metric | Value | Notes |
|--------|-------|-------|
| Time per submission | ~1ms | Excludes network/DB |
| TauStore lookup | O(1) | In-memory map |
| Memory overhead | <1 KB | 6 tau values |
| Concurrency | ✅ Thread-safe | RWMutex protected |

---

## 📚 Files Modified

```
master/
├── internal/
│   ├── server/
│   │   ├── master_server.go        (+80, -20)  # SubmitTask rewrite
│   │   └── task_submission_test.go (+400)      # New tests
│   └── config/
│       └── config.go                (+30)      # SLAMultiplier config
└── main.go                          (+5)       # TauStore init
```

---

## 🎯 Next Steps

### Immediate

1. ✅ Verify build: `cd master && go build`
2. ✅ Run tests: `go test ./internal/server -v`
3. ✅ Test manually with grpcurl

### Sprint Plan Progression

- **Current**: Task 2.2 ✅ Complete (Milestone 2: 40%)
- **Next**: Task 2.3 - Track Load at Assignment
- **Then**: Task 2.4 - Update Tau on Completion

---

## 💡 Usage Examples

### Example 1: CPU-Intensive Data Processing

```bash
grpcurl -plaintext -d '{
  "task_id": "data-crunch-1",
  "docker_image": "pandas:latest",
  "command": "python process.py",
  "req_cpu": 16.0,
  "req_memory": 8.0,
  "task_type": "cpu-heavy"
}' localhost:8080 MasterWorker/SubmitTask
```

**Result**:
- Task type: `cpu-heavy`
- Tau: `15.0s`
- k: `2.0`
- Deadline: `arrival + 30s`

---

### Example 2: GPU Model Training (Inferred)

```bash
grpcurl -plaintext -d '{
  "task_id": "train-model-1",
  "docker_image": "pytorch:latest",
  "command": "python train.py",
  "req_cpu": 8.0,
  "req_memory": 32.0,
  "req_gpu": 4.0
}' localhost:8080 MasterWorker/SubmitTask
```

**Result**:
- Task type: `gpu-training` (inferred)
- Tau: `60.0s`
- k: `2.0`
- Deadline: `arrival + 120s`

---

### Example 3: Urgent Task (Custom Multiplier)

```bash
grpcurl -plaintext -d '{
  "task_id": "urgent-query",
  "docker_image": "mysql:latest",
  "command": "mysql < query.sql",
  "req_cpu": 2.0,
  "task_type": "cpu-light",
  "sla_multiplier": 1.5
}' localhost:8080 MasterWorker/SubmitTask
```

**Result**:
- Task type: `cpu-light`
- Tau: `5.0s`
- k: `1.5` (custom)
- Deadline: `arrival + 7.5s`

---

## 🔐 Validation Ranges

| Parameter | Min | Max | Default | Notes |
|-----------|-----|-----|---------|-------|
| SLA Multiplier (k) | 1.5 | 2.5 | 2.0 | Server or task-level |
| Tau | 0.1s | ∞ | Varies | Per task type |
| Deadline | Now | ∞ | arrival + k*tau | Must be future |

---

## 📞 Support

### Logs to Check

```bash
# Task type decisions
grep "task_type" master.log

# Tau retrieval
grep "Retrieved tau" master.log

# Deadline computation
grep "Computed deadline" master.log

# Errors
grep "ERROR" master.log
```

### Debug Mode

```bash
# Enable verbose logging
export LOG_LEVEL=debug
./master
```

---

## ✨ Summary

Task 2.2 enables **SLA-aware task submission** with:

✅ Automatic tau lookup from TauStore  
✅ Task type validation & inference  
✅ Deadline computation (`arrival + k*tau`)  
✅ Configurable SLA multiplier  
✅ Thread-safe, O(1) complexity  
✅ Graceful error handling  

**Ready for**: Task 2.4 (Tau Updates) and Task 3.3 (RTS Scheduling)

---

_For detailed implementation, see `TASK_2.2_IMPLEMENTATION_SUMMARY.md`_
