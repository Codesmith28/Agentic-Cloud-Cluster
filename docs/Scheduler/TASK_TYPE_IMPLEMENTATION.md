# Task Type Implementation Guide

## Overview
The system now supports explicit task type classification through user input. Users can specify the task type via a dropdown menu or CLI flag, which helps the RTS scheduler make better scheduling decisions.

## Task Type Options

### 1. **cpu-light**
- **Description**: Light CPU workloads
- **Typical Use Cases**: 
  - Web servers
  - API endpoints
  - Light data processing
  - Simple scripts
- **Resource Profile**: Low CPU, moderate memory

### 2. **cpu-heavy**
- **Description**: Heavy CPU workloads
- **Typical Use Cases**:
  - Data compression/decompression
  - Video encoding
  - Compilation
  - Complex calculations
- **Resource Profile**: High CPU (> 4 cores), moderate memory

### 3. **memory-heavy**
- **Description**: Memory-intensive workloads
- **Typical Use Cases**:
  - Large dataset processing
  - In-memory databases
  - Caching systems
  - Big data analytics
- **Resource Profile**: High memory (> 8GB), moderate CPU

### 4. **gpu-inference**
- **Description**: GPU inference workloads
- **Typical Use Cases**:
  - Model serving
  - Real-time predictions
  - Image/video processing
  - Natural language processing
- **Resource Profile**: GPU required, moderate CPU/memory

### 5. **gpu-training**
- **Description**: GPU training workloads  
- **Typical Use Cases**:
  - Deep learning model training
  - Neural network optimization
  - Large-scale ML experiments
- **Resource Profile**: High GPU (> 2), high CPU (> 4 cores), high memory

### 6. **mixed**
- **Description**: Mixed or unknown workloads
- **Typical Use Cases**:
  - Multi-stage pipelines
  - Hybrid workloads
  - Unknown resource patterns
- **Resource Profile**: Variable

## Implementation Details

### Protocol Buffers
**File**: `proto/master_worker.proto`

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
  string task_type = 11; // NEW: Task type field
}
```

### Database Schema
**File**: `master/internal/db/tasks.go`

```go
type Task struct {
    TaskID        string    `bson:"task_id"`
    UserID        string    `bson:"user_id"`
    DockerImage   string    `bson:"docker_image"`
    Command       string    `bson:"command"`
    ReqCPU        float64   `bson:"req_cpu"`
    ReqMemory     float64   `bson:"req_memory"`
    ReqStorage    float64   `bson:"req_storage"`
    ReqGPU        float64   `bson:"req_gpu"`
    TaskType      string    `bson:"task_type"`      // NEW
    SLAMultiplier float64   `bson:"sla_multiplier"`
    Status        string    `bson:"status"`
    CreatedAt     time.Time `bson:"created_at"`
    StartedAt     time.Time `bson:"started_at,omitempty"`
    CompletedAt   time.Time `bson:"completed_at,omitempty"`
}
```

### RTS Models
**File**: `master/internal/scheduler/rts_models.go`

The `NewTaskViewFromProto` function now:
1. **Checks for explicit task type** from user input
2. **Validates** the task type against allowed values
3. **Falls back to inference** if type is invalid or not provided
4. **Uses explicit type** for RTS scheduling decisions

```go
func NewTaskViewFromProto(pbTask *pb.Task, now time.Time, tau float64, k float64) TaskView {
    // Try to get explicit task type from user
    taskType := pbTask.GetTaskType()
    
    // Validate task type
    if taskType != "" && !ValidateTaskType(taskType) {
        taskType = "" // Invalid, will infer
    }
    
    // Fallback to inference if not provided
    if taskType == "" {
        taskType = InferTaskType(pbTask)
    }
    
    // ... rest of function
}
```

## Usage Examples

### CLI Usage

#### Basic Task Submission with Type
```bash
task myapp:latest -type cpu-heavy
```

#### Complete Example with All Parameters
```bash
task myapp:latest -cpu_cores 4 -mem 8 -storage 10 -k 1.8 -type cpu-heavy
```

#### GPU Training Example
```bash
task ml-model:latest -cpu_cores 8 -mem 16 -gpu_cores 2 -k 2.5 -type gpu-training
```

#### GPU Inference Example
```bash
task api-server:latest -cpu_cores 2 -mem 4 -gpu_cores 1 -k 1.5 -type gpu-inference
```

#### Memory-Heavy Example
```bash
task data-processor:latest -cpu_cores 4 -mem 32 -k 2.0 -type memory-heavy
```

### Automatic Inference (Fallback)
If you don't specify `-type`, the system will infer it:

```bash
# This will be inferred as "gpu-training" (GPU > 2, CPU > 4)
task ml-model:latest -cpu_cores 8 -gpu_cores 4 -mem 16

# This will be inferred as "memory-heavy" (Memory > 8)
task cache-system:latest -cpu_cores 2 -mem 16

# This will be inferred as "cpu-heavy" (CPU > 4)
task encoder:latest -cpu_cores 8 -mem 4
```

## CLI Help

The updated help command shows:

```
Task Types (-type flag):
  cpu-light                      - Light CPU workloads
  cpu-heavy                      - Heavy CPU workloads
  memory-heavy                   - Memory-intensive workloads
  gpu-inference                  - GPU inference workloads
  gpu-training                   - GPU training workloads
  mixed                          - Mixed workloads

Examples:
  task myapp:latest -cpu_cores 4 -mem 8 -k 1.8 -type cpu-heavy
  task ml-model:latest -gpu_cores 2 -mem 16 -k 2.5 -type gpu-training
```

## Validation

### CLI Validation
The CLI validates task types before submission:
```go
validTypes := []string{"cpu-light", "cpu-heavy", "memory-heavy", 
                       "gpu-inference", "gpu-training", "mixed"}
```

If invalid type is provided:
```
⚠️  Warning: Invalid task type 'invalid-type'. Must be one of: [cpu-light cpu-heavy memory-heavy gpu-inference gpu-training mixed]
    Task type will be automatically inferred from resources.
```

### Server Validation
The master server also validates:
```go
validTypes := map[string]bool{
    "cpu-light": true, "cpu-heavy": true, "memory-heavy": true,
    "gpu-inference": true, "gpu-training": true, "mixed": true,
}
if !validTypes[taskType] {
    log.Printf("⚠️  Task %s: Invalid task type '%s'. Will be inferred from resources.", 
        task.TaskId, taskType)
    taskType = "" // Clear to trigger inference
}
```

## Display Output

When submitting a task with explicit type:
```
═══════════════════════════════════════════════════════
  📤 SUBMITTING TASK TO QUEUE
═══════════════════════════════════════════════════════
  Task ID:           task-1731753000
  Docker Image:      myapp:latest
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     4.00 cores
    • Memory:        8.00 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
───────────────────────────────────────────────────────
  Task Classification:
    • Type:          cpu-heavy (user-specified)
───────────────────────────────────────────────────────
  SLA Configuration:
    • SLA Multiplier (k): 1.8 (Deadline = k × τ)
───────────────────────────────────────────────────────
```

When task type will be inferred:
```
───────────────────────────────────────────────────────
  Task Classification:
    • Type:          (will be inferred from resources)
───────────────────────────────────────────────────────
```

## Master Server Logging

Tasks are logged with their type:
```
📋 Task task-1731753000 submitted and queued (position: 1, k=1.8, type=cpu-heavy)
```

## Database Queries

### Query tasks by type
```javascript
db.TASKS.find({ task_type: "gpu-training" })
```

### Count tasks by type
```javascript
db.TASKS.aggregate([
  { $group: { _id: "$task_type", count: { $sum: 1 } } }
])
```

### Find tasks with explicit vs inferred types
```javascript
// Tasks with user-specified types
db.TASKS.find({ task_type: { $ne: "" } })

// Tasks that will use inferred types
db.TASKS.find({ task_type: "" })
```

## Benefits for RTS Scheduler

### 1. **Better Affinity Matching**
- Scheduler can use `AffinityMatrix[taskType][workerID]` directly
- No need to wait for historical data to build preferences

### 2. **Improved Tau Estimation**
- More accurate base runtime estimates per type
- Better deadline calculation

### 3. **Workload Balancing**
- Distribute task types across workers based on performance
- Avoid overloading workers with incompatible task types

### 4. **SLA Optimization**
- Task types help predict execution time more accurately
- Lower SLA violation rates

## Integration with Web UI (Future)

For web-based task submission, implement dropdown:

```html
<select name="task_type">
  <option value="">Auto-detect from resources</option>
  <option value="cpu-light">CPU Light</option>
  <option value="cpu-heavy">CPU Heavy</option>
  <option value="memory-heavy">Memory Heavy</option>
  <option value="gpu-inference">GPU Inference</option>
  <option value="gpu-training">GPU Training</option>
  <option value="mixed">Mixed</option>
</select>
```

## Best Practices

1. **Specify type when known**: If you know the workload type, always specify it
2. **Use consistent types**: For similar tasks, use the same type for better scheduler learning
3. **Monitor accuracy**: Check if inferred types match your expectations
4. **Update based on performance**: Adjust task types if scheduling isn't optimal

## Backward Compatibility

- ✅ Tasks without explicit type will be inferred automatically
- ✅ Existing code continues to work
- ✅ Empty/invalid types fall back to inference
- ✅ No breaking changes to existing API

## Testing

### Test Different Types
```bash
# Test each task type
task test:v1 -cpu_cores 1 -mem 1 -type cpu-light
task test:v1 -cpu_cores 8 -mem 4 -type cpu-heavy
task test:v1 -cpu_cores 4 -mem 16 -type memory-heavy
task test:v1 -cpu_cores 2 -mem 4 -gpu_cores 1 -type gpu-inference
task test:v1 -cpu_cores 8 -mem 16 -gpu_cores 4 -type gpu-training
task test:v1 -cpu_cores 2 -mem 2 -type mixed
```

### Test Validation
```bash
# Test invalid type (should warn and infer)
task test:v1 -cpu_cores 2 -mem 4 -type invalid-type

# Test no type (should infer)
task test:v1 -cpu_cores 8 -mem 4
```

### Verify in Database
```javascript
db.TASKS.find().sort({ created_at: -1 }).limit(5)
```

## Summary

The task type feature provides:
- ✅ **Explicit user control** over task classification
- ✅ **Validation** at multiple layers (CLI, Server, Scheduler)
- ✅ **Automatic fallback** to inference when not specified
- ✅ **Better scheduling decisions** for RTS
- ✅ **Database persistence** for analytics
- ✅ **Backward compatibility** with existing code
