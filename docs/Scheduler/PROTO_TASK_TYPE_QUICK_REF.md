# Task Type Proto API - Quick Reference

**Task 1.4**: Add Task Type to Proto and API Layer  
**Status**: ✅ Complete

---

## Proto Field Definitions

### master_worker.proto - Task Message
```protobuf
message Task {
  // ... existing fields ...
  string task_type = 11; // Optional: cpu-light, cpu-heavy, memory-heavy, 
                         //           gpu-inference, gpu-training, mixed
}
```

### master_agent.proto - TaskInfo Message
```protobuf
message TaskInfo {
  // ... existing fields ...
  string task_type = 5; // Optional: cpu-light, cpu-heavy, memory-heavy,
                        //           gpu-inference, gpu-training, mixed
}
```

---

## Valid Task Types

| Type | Description | Inference Trigger |
|------|-------------|-------------------|
| `cpu-light` | Low CPU tasks | `ReqCPU > 0 && ReqCPU <= 4.0` |
| `cpu-heavy` | High CPU tasks | `ReqCPU > 4.0 && ReqGPU == 0` |
| `memory-heavy` | High memory tasks | `ReqMemory > 8.0` |
| `gpu-inference` | GPU inference | `ReqGPU > 0 && ReqGPU <= 2.0` |
| `gpu-training` | GPU training | `ReqGPU > 2.0 && ReqCPU > 4.0` |
| `mixed` | Mixed workloads | Default fallback |

---

## Go API Usage

### Creating Task with Explicit Type
```go
task := &pb.Task{
    TaskId:      "task-123",
    DockerImage: "tensorflow/tensorflow:latest-gpu",
    Command:     "python train.py",
    ReqCpu:      8.0,
    ReqMemory:   32.0,
    ReqGpu:      4.0,
    TaskType:    "gpu-training", // Explicit
}
```

### Getting Task Type
```go
taskType := task.GetTaskType() // Returns "" if not set
```

### Validating Task Type (Task 1.1)
```go
import "github.com/Codesmith28/CloudAI/master/internal/scheduler"

if scheduler.ValidateTaskType(task.TaskType) {
    // Valid task type
} else {
    // Invalid - fall back to inference
    task.TaskType = scheduler.InferTaskType(task)
}
```

---

## Python API Usage (Agent Side)

### Creating TaskInfo with Type
```python
from proto.py import master_agent_pb2

task_info = master_agent_pb2.TaskInfo(
    task_id="task-456",
    req_cpu=4.0,
    req_memory=8.0,
    req_gpu=1.0,
    task_type="gpu-inference"
)
```

### Reading Task Type
```python
if task_info.task_type:
    print(f"Task type: {task_info.task_type}")
else:
    print("Task type not specified (will be inferred)")
```

---

## Submission Flow

```
User Submits Task
      ↓
task_type provided? ──NO──→ InferTaskType() ──→ Store inferred type
      ↓ YES
      ↓
ValidateTaskType() ──INVALID──→ Log warning + InferTaskType()
      ↓ VALID
      ↓
Store explicit type
      ↓
Continue with submission
```

---

## Backward Compatibility

### Old Client (No task_type)
```go
// This still works - task_type will be inferred
task := &pb.Task{
    TaskId:      "legacy-task",
    DockerImage: "nginx:latest",
    ReqCpu:      2.0,
    // task_type: ""  (empty, will be inferred as "cpu-light")
}
```

### New Client (With task_type)
```go
// Preferred approach - explicit type
task := &pb.Task{
    TaskId:      "new-task",
    DockerImage: "nginx:latest",
    ReqCpu:      2.0,
    TaskType:    "cpu-light", // Explicit
}
```

---

## Database Storage

Task type stored in MongoDB with `omitempty` tag:

```go
type Task struct {
    // ... other fields ...
    TaskType  string    `bson:"task_type,omitempty"`
    // ... other fields ...
}
```

**Effect**: 
- New tasks: `task_type` field present
- Old tasks: `task_type` field absent (backward compatible)
- Query works for both: defaults to empty string

---

## Testing

### Proto Field Test
```go
task := &pb.Task{TaskType: "cpu-heavy"}
assert.Equal(t, "cpu-heavy", task.GetTaskType())
```

### Validation Test
```go
valid := scheduler.ValidateTaskType("gpu-training")
assert.True(t, valid)

invalid := scheduler.ValidateTaskType("invalid-type")
assert.False(t, invalid)
```

### Inference Test
```go
task := &pb.Task{ReqCpu: 8.0, ReqGpu: 0.0}
taskType := scheduler.InferTaskType(task)
assert.Equal(t, "cpu-heavy", taskType)
```

---

## Common Patterns

### Pattern 1: Always Use Explicit Type (Recommended)
```go
func submitTask(resources TaskResources, explicitType string) {
    task := &pb.Task{
        ReqCpu:    resources.CPU,
        ReqMemory: resources.Memory,
        ReqGpu:    resources.GPU,
        TaskType:  explicitType, // User-specified
    }
    // Submit...
}
```

### Pattern 2: Validate Then Submit
```go
func submitTaskWithValidation(task *pb.Task) error {
    if task.TaskType != "" {
        if !scheduler.ValidateTaskType(task.TaskType) {
            return fmt.Errorf("invalid task type: %s", task.TaskType)
        }
    } else {
        task.TaskType = scheduler.InferTaskType(task)
    }
    // Submit validated task...
}
```

### Pattern 3: Inference Only (Legacy)
```go
func submitLegacyTask(task *pb.Task) {
    // No task_type specified
    // Server will infer during processing
    // Submit...
}
```

---

## Error Handling

### Invalid Task Type
```go
if !scheduler.ValidateTaskType(task.TaskType) {
    log.Warnf("Invalid task type '%s', falling back to inference", 
              task.TaskType)
    task.TaskType = scheduler.InferTaskType(task)
}
```

### Empty Task Type
```go
if task.GetTaskType() == "" {
    // This is OK - will be inferred
    log.Debug("Task type not specified, will infer from resources")
}
```

---

## Proto Regeneration

### Quick Regeneration
```bash
cd proto && ./generate.sh
```

### Manual Steps
```bash
# Go code
protoc --go_out=./pb --go_opt=paths=source_relative \
       --go-grpc_out=./pb --go-grpc_opt=paths=source_relative \
       master_worker.proto master_agent.proto

# Python code
python3 -m grpc_tools.protoc --python_out=./py \
        --grpc_python_out=./py --proto_path=. master_agent.proto

# Fix imports
sed -i 's/^import master_agent_pb2/from . import master_agent_pb2/g' \
       ./py/master_agent_pb2_grpc.py
```

---

## Integration Checklist

- [x] Proto definitions updated (master_worker.proto, master_agent.proto)
- [x] Go code generated (pb/master_worker.pb.go, pb/master_agent.pb.go)
- [x] Python code generated (py/master_agent_pb2.py)
- [x] Tests created (proto_task_type_test.go)
- [x] Build verification (master compiles successfully)
- [ ] SubmitTask handler validation (Task 2.2)
- [ ] TauStore per-type tau lookup (Task 2.1)
- [ ] RTS scheduler type-aware selection (Task 3.3)
- [ ] GA affinity matrix per-type training (Task 4.3)

---

## Quick Lookup

**Validate Type**: `scheduler.ValidateTaskType(taskType)`  
**Infer Type**: `scheduler.InferTaskType(task)`  
**Get Type**: `task.GetTaskType()`  
**Set Type**: `task.TaskType = "cpu-heavy"`  
**Valid Types**: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed

---

**Last Updated**: November 16, 2025  
**Related Docs**: TASK_1.4_IMPLEMENTATION_SUMMARY.md, SPRINT_PLAN.md (Task 1.4)
