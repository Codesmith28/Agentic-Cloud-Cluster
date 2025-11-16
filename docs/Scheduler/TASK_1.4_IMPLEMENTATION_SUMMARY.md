# Task 1.4 Implementation Summary: Add Task Type to Proto and API Layer

**Status**: ✅ Complete  
**Date**: November 16, 2025  
**Sprint Plan Reference**: Milestone 1, Task 1.4

---

## Overview

Task 1.4 extends the protobuf definitions to include an explicit `task_type` field in both `master_worker.proto` and `master_agent.proto`. This enables users to specify task types explicitly in their submissions, improving scheduling accuracy while maintaining backward compatibility through task type inference.

---

## Changes Made

### 1. Proto File Modifications

#### **master_worker.proto**
- **File**: `/proto/master_worker.proto`
- **Change**: Added `task_type` field to `Task` message
- **Field Number**: 11 (following `sla_multiplier` at field 10)
- **Type**: `string` (optional)
- **Comment**: "Task type: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed"

```protobuf
message Task {
  string task_id = 1;
  string docker_image = 2;
  string command = 3;
  double req_cpu = 4;
  double req_memory = 5;
  double req_storage = 6;
  double req_gpu = 7;
  string target_worker_id = 8;
  string user_id = 9;
  double sla_multiplier = 10;
  string task_type = 11; // NEW FIELD
}
```

#### **master_agent.proto**
- **File**: `/proto/master_agent.proto`
- **Change**: Added `task_type` field to `TaskInfo` message
- **Field Number**: 5 (following `req_gpu` at field 4)
- **Type**: `string` (optional)
- **Comment**: "Task type: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed"

```protobuf
message TaskInfo {
  string task_id = 1;
  double req_cpu = 2;
  double req_memory = 3;
  double req_gpu = 4;
  string task_type = 5; // NEW FIELD
}
```

---

### 2. Generated Code Updates

#### **Go Files** (Generated via protoc)
- **master_worker.pb.go**:
  - Added `TaskType string` field to `Task` struct
  - Added `GetTaskType() string` method
  - Field tag: `protobuf:"bytes,11,opt,name=task_type,json=taskType,proto3"`

- **master_agent.pb.go**:
  - Added `TaskType string` field to `TaskInfo` struct  
  - Added `GetTaskType() string` method
  - Field tag: `protobuf:"bytes,5,opt,name=task_type,json=taskType,proto3"`

#### **Python Files** (Generated via grpc_tools.protoc)
- **py/master_agent_pb2.py**:
  - Added `task_type` field to `TaskInfo` descriptor
  - Serialized in protobuf wire format at field 5

- **py/master_agent_pb2_grpc.py**:
  - Updated with correct relative import: `from . import master_agent_pb2`

---

### 3. Test Coverage

#### **proto_task_type_test.go**
Created comprehensive test suite with 7 test functions:

1. **TestProtoTaskTypeField**: Verifies basic field access
2. **TestProtoTaskTypeValidation**: Tests all 6 valid task types
3. **TestProtoTaskTypeEmpty**: Ensures empty task_type is allowed (for inference fallback)
4. **TestProtoTaskTypeGetter**: Validates `GetTaskType()` method
5. **TestProtoAgentTaskInfoType**: Tests `TaskInfo` message in master_agent.proto
6. **TestProtoTaskTypeSerialization**: Verifies field survives proto serialization
7. **TestProtoBackwardCompatibility**: Ensures old tasks without task_type still work

**Test Results**: All tests passing ✅

---

## Task Type Enum Values

The following 6 task types are valid and standardized across the system:

| Task Type | Description | Typical Use Case |
|-----------|-------------|------------------|
| `cpu-light` | Low CPU intensity | Web servers, I/O operations |
| `cpu-heavy` | High CPU intensity | Data processing, compression |
| `memory-heavy` | High memory usage | In-memory databases, caching |
| `gpu-inference` | GPU for inference | ML inference, image processing |
| `gpu-training` | GPU for training | Deep learning training |
| `mixed` | Mixed resources | Complex workloads |

---

## API Usage

### **Explicit Task Type Specification**

Users can now explicitly specify task type in task submissions:

```go
task := &pb.Task{
    TaskId:      "user-task-123",
    DockerImage: "tensorflow/tensorflow:latest-gpu",
    Command:     "python train.py",
    ReqCpu:      8.0,
    ReqMemory:   32.0,
    ReqGpu:      4.0,
    TaskType:    "gpu-training", // Explicit type
}
```

### **Inference Fallback**

If `task_type` is empty or invalid, the system falls back to inference using `InferTaskType()`:

```go
task := &pb.Task{
    TaskId:      "user-task-456",
    DockerImage: "nginx:latest",
    ReqCpu:      1.0,
    ReqMemory:   2.0,
    // TaskType empty - will be inferred as "cpu-light"
}
```

### **Validation Flow**

1. **Check if task_type is provided and non-empty**
2. **If yes**: Validate using `scheduler.ValidateTaskType(task.TaskType)`
   - If valid → use explicit type
   - If invalid → log warning, fall back to inference
3. **If no**: Call `scheduler.InferTaskType(task)` to determine type

---

## Integration Points

### **Current Integration**
- ✅ Proto definitions updated with task_type field
- ✅ Go code generated with proper getters/setters
- ✅ Python code generated for agent integration
- ✅ Backward compatibility maintained (empty task_type allowed)

### **Future Integration** (Milestone 2+)
These components will use the new `task_type` field:

1. **Task Submission Handler** (Task 2.2):
   - Validate explicit task_type if provided
   - Fall back to inference if empty/invalid
   - Store final task_type in TaskDB

2. **Tau Store** (Task 2.1):
   - Use task_type as key for per-type tau values
   - Update tau separately for each of the 6 types

3. **RTS Scheduler** (Task 3.3):
   - Use task_type to retrieve type-specific tau
   - Compute deadline based on task_type tau
   - Apply affinity matrix row for specific task_type

4. **GA Module** (Task 4.3):
   - Build 6-row affinity matrix (one row per task_type)
   - Compute per-type performance metrics
   - Train separate theta values per task_type if beneficial

---

## Backward Compatibility

### **Design Decisions**
1. **Optional Field**: `task_type` is optional (proto3 default behavior)
2. **Empty String Default**: Missing task_type defaults to `""`
3. **Inference Fallback**: Empty task_type triggers `InferTaskType()` logic
4. **No Breaking Changes**: Existing task submissions continue to work

### **Migration Path**
- **Old Tasks**: Work as before, task_type inferred from resources
- **New Tasks**: Can specify explicit task_type for better accuracy
- **Database**: No migration needed, field stored as `omitempty` in BSON

---

## Validation Rules

### **At Submission** (Task 2.2 will implement)
```go
// Pseudo-code for SubmitTask handler
if task.TaskType != "" {
    if !scheduler.ValidateTaskType(task.TaskType) {
        log.Warnf("Invalid task type '%s', falling back to inference", task.TaskType)
        task.TaskType = scheduler.InferTaskType(task)
    }
} else {
    task.TaskType = scheduler.InferTaskType(task)
}
```

### **Valid Types Check**
```go
func ValidateTaskType(taskType string) bool {
    validTypes := []string{
        "cpu-light", "cpu-heavy", "memory-heavy",
        "gpu-inference", "gpu-training", "mixed",
    }
    for _, valid := range validTypes {
        if taskType == valid {
            return true
        }
    }
    return false
}
```

---

## Proto Regeneration Commands

To regenerate proto files after modifications:

```bash
cd proto

# Generate Go code for master_worker.proto
protoc --go_out=./pb --go_opt=paths=source_relative \
    --go-grpc_out=./pb --go-grpc_opt=paths=source_relative \
    master_worker.proto

# Generate Go code for master_agent.proto
protoc --go_out=./pb --go_opt=paths=source_relative \
    --go-grpc_out=./pb --go-grpc_opt=paths=source_relative \
    master_agent.proto

# Generate Python code for master_agent.proto
python3 -m grpc_tools.protoc \
    --python_out=./py \
    --grpc_python_out=./py \
    --proto_path=. \
    master_agent.proto

# Fix Python imports
sed -i 's/^import master_agent_pb2/from . import master_agent_pb2/g' \
    ./py/master_agent_pb2_grpc.py
```

Or simply run:
```bash
cd proto && ./generate.sh
```

---

## Testing

### **Run Proto Tests**
```bash
cd master/internal/db
go test -v -run TestProto
```

### **Run All DB Tests**
```bash
cd master/internal/db
go test -v
```

### **Verify Build**
```bash
cd master
go build -o masterNode .
```

---

## Files Modified

### **Proto Definitions**
- `proto/master_worker.proto` - Added task_type field (line 11)
- `proto/master_agent.proto` - Added task_type field (line 5)

### **Generated Files** (Auto-regenerated)
- `proto/pb/master_worker.pb.go` - Task struct with TaskType field
- `proto/pb/master_worker_grpc.pb.go` - gRPC service stubs
- `proto/pb/master_agent.pb.go` - TaskInfo struct with TaskType field
- `proto/pb/master_agent_grpc.pb.go` - gRPC service stubs
- `proto/py/master_agent_pb2.py` - Python message descriptors
- `proto/py/master_agent_pb2_grpc.py` - Python gRPC stubs

### **Test Files** (New)
- `master/internal/db/proto_task_type_test.go` - 7 comprehensive tests

---

## Benefits

1. **User Control**: Users can explicitly specify task types for better scheduling
2. **Improved Accuracy**: Reduces reliance on inference, which may not always be perfect
3. **Type-Aware Scheduling**: Enables RTS to use type-specific tau and affinity values
4. **GA Training**: Allows GA to learn separate performance characteristics per task type
5. **Backward Compatible**: Existing code continues to work without modifications
6. **Future-Proof**: Supports 6 standardized task types across the entire system

---

## Next Steps (Milestone 2)

1. **Task 2.1**: Implement TauStore with per-type tau values
2. **Task 2.2**: Integrate task_type validation in SubmitTask handler
3. **Task 2.3**: Use task_type when tracking load at assignment
4. **Task 2.4**: Update tau per-type on task completion
5. **Task 2.5**: Compute SLA success per task type

---

## Dependencies

### **Required For**
- Task 2.1 (Tau Store): Uses task_type as key
- Task 2.2 (Task Enrichment): Validates and stores task_type
- Task 3.3 (RTS Scheduler): Uses task_type for tau lookup and affinity
- Task 4.3 (Affinity Builder): Builds 6-row affinity matrix

### **Dependencies On**
- Task 1.1 (RTS Models): Uses task type constants and ValidateTaskType
- Proto compiler tools: protoc, protoc-gen-go, grpc_tools.protoc

---

## Conclusion

Task 1.4 successfully extends the proto API layer with explicit task type support while maintaining full backward compatibility. The implementation provides a clean foundation for type-aware scheduling in Milestone 2 and beyond, enabling both explicit user control and intelligent inference fallback.

**Milestone 1 Progress**: 4/4 tasks complete ✅ (100%)
