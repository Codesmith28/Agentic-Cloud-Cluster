# **Master CLI - Quick Reference Card**

## **Task Management**

### **Submit Task (Scheduler Selects Worker)**
```bash
task <docker_img> [options]
```

**Options**:
- `-cpu_cores <num>` - CPU cores required (e.g., 1, 2.5, 8)
- `-mem <gb>` - Memory in GB (e.g., 2, 8, 16)
- `-storage <gb>` - Storage in GB (e.g., 10, 50)
- `-gpu_cores <num>` - GPU cores required (e.g., 0.5, 1, 2)
- `-k <1.5-2.5>` - SLA multiplier for deadline (default: 2.0)
- `-type <task_type>` - Explicit task type (see types below)

**Examples**:
```bash
# CPU-light task (explicit type)
master> task moinvinchhi/cloudai-cpu-intensive:1 -cpu_cores 1 -mem 2 -type cpu-light

# CPU-heavy task with tight deadline
master> task myapp:latest -cpu_cores 8 -mem 4 -k 1.5 -type cpu-heavy

# GPU training task with loose deadline
master> task ml-model:latest -gpu_cores 2 -mem 16 -k 2.5 -type gpu-training

# Task without explicit type (will infer from resources)
master> task sample:v1 -cpu_cores 4 -mem 8
```

---

### **Direct Dispatch to Worker (Testing Only)**
```bash
dispatch <worker_id> <docker_img> [options]
```

**Example**:
```bash
master> dispatch worker-1 moinvinchhi/cloudai-cpu-intensive:1 -cpu_cores 2 -mem 4
```

---

### **List Tasks**
```bash
list-tasks [status]
```

**Status Filters**:
- `pending` - Tasks in queue
- `running` - Currently executing
- `completed` - Finished successfully
- `failed` - Execution failed
- _(no filter)_ - All tasks

**Examples**:
```bash
master> list-tasks                # All tasks
master> list-tasks running        # Only running tasks
master> list-tasks completed      # Only completed tasks
```

---

### **Monitor Task (Live Logs)**
```bash
monitor <task_id>
```

**Example**:
```bash
master> monitor task-123
# Press any key to exit monitoring
```

---

### **Cancel Task**
```bash
cancel <task_id>
```

**Example**:
```bash
master> cancel task-123
```

---

### **Show Queue**
```bash
queue
```

Shows all pending tasks waiting for assignment.

---

## **Worker Management**

### **List All Workers**
```bash
workers
```

Shows registered workers with status, resources, and load.

---

### **Worker Detailed Stats**
```bash
stats <worker_id>
```

**Example**:
```bash
master> stats worker-1
```

Shows detailed resource usage, running tasks, and telemetry.

---

### **Internal State Dump**
```bash
internal-state
```

Shows complete in-memory state of all workers including:
- Total resources
- Allocated resources
- Available resources
- Running tasks
- Load metrics

---

### **Fix Resource Allocations**
```bash
fix-resources
```

Reconciles stale resource allocations (run if workers show incorrect available resources).

---

### **Manual Registration**
```bash
register <worker_id> <ip:port>
```

**Example**:
```bash
master> register worker-2 192.168.1.100:50052
```

---

### **Unregister Worker**
```bash
unregister <worker_id>
```

**Example**:
```bash
master> unregister worker-2
```

---

## **System Status**

### **Cluster Status**
```bash
status
```

Shows overall cluster health, worker count, task statistics.

---

### **Help**
```bash
help
```

Shows all available commands.

---

### **Exit Master**
```bash
exit
# or
quit
```

Gracefully shuts down master node.

---

## **Task Types**

### **6 Standardized Task Types**:

| Type | Description | Typical Resources |
|------|-------------|-------------------|
| `cpu-light` | Light CPU workloads | < 4 cores, < 8GB RAM, no GPU |
| `cpu-heavy` | Heavy CPU workloads | ≥ 4 cores, < 8GB RAM, no GPU |
| `memory-heavy` | Memory-intensive | ≥ 8GB RAM, no GPU |
| `gpu-inference` | GPU inference | < 2 GPU cores |
| `gpu-training` | GPU training | ≥ 2 GPU cores |
| `mixed` | Mixed workloads | Complex resource combinations |

### **Task Type Inference Rules** (when `-type` not specified):
- If `gpu > 2.0 && cpu > 4.0` → `gpu-training`
- If `gpu > 0` → `gpu-inference`
- If `mem > 8.0` → `memory-heavy`
- If `cpu > 4.0` → `cpu-heavy`
- If `cpu > 0` → `cpu-light`
- Otherwise → `mixed`

### **Best Practice**: Always use explicit `-type` flag when you know the task type to ensure correct scheduling.

---

## **SLA Configuration**

### **Understanding `-k` Parameter**:
Deadline is computed as: **`deadline = now + k × τ`**

Where:
- `k` = SLA multiplier (1.5 to 2.5)
- `τ` = Expected runtime (learned per task type)

### **Recommended Values**:
- `k = 1.5` - Tight deadline (low slack, higher risk)
- `k = 2.0` - **Default** (balanced)
- `k = 2.5` - Loose deadline (high slack, safer)

### **Examples**:
```bash
# Tight deadline for fast tasks
master> task quick:v1 -cpu_cores 1 -mem 2 -k 1.5 -type cpu-light

# Loose deadline for long GPU training
master> task train:v2 -gpu_cores 4 -mem 32 -k 2.5 -type gpu-training
```

---

## **Common Workflows**

### **1. Submit and Monitor Single Task**
```bash
master> task myapp:latest -cpu_cores 2 -mem 4 -type cpu-light
Task submitted: task-123

master> monitor task-123
# Watch live logs...

master> list-tasks completed
# Verify completion
```

---

### **2. Batch Task Submission**
```bash
# Submit multiple tasks
master> task app:v1 -cpu_cores 2 -mem 4 -type cpu-light
master> task app:v2 -cpu_cores 4 -mem 8 -type cpu-heavy
master> task model:v1 -gpu_cores 1 -mem 16 -type gpu-inference

# Check queue
master> queue

# Monitor all running tasks
master> list-tasks running
```

---

### **3. Check Cluster Health**
```bash
# Overall status
master> status

# Worker details
master> workers
master> stats worker-1
master> stats worker-2

# Task distribution
master> internal-state

# Queue status
master> queue
```

---

### **4. Troubleshooting Worker Issues**
```bash
# Check worker status
master> workers
# If worker shows stale allocations...

# Fix resource allocations
master> fix-resources

# Verify fix
master> internal-state
```

---

### **5. Testing Different Schedulers**
```bash
# Submit task (RTS scheduler)
master> task test:v1 -cpu_cores 2 -mem 4 -type cpu-light

# Direct dispatch (bypass scheduler, testing only)
master> dispatch worker-1 test:v1 -cpu_cores 2 -mem 4
```

---

## **Monitoring Key Logs**

### **RTS Scheduler Logs** (every 30s):
```
✓ RTS: Reloaded GA parameters from config/ga_output.json
```

### **GA Training Logs** (every 60s):
```
🧬 Starting AOD/GA epoch...
📊 Fetching task history from <start> to <end>
✓ Retrieved X task history records and Y worker stats
✓ GA parameters saved to config/ga_output.json
✅ AOD/GA epoch completed successfully
```

### **Worker Registration**:
```
✓ Worker worker-1 registered successfully
```

### **Task Assignment**:
```
[RTS] [task=task-123] [type=cpu-light] Selected worker=worker-1 (risk=2.34)
```

### **Fallback to Round-Robin**:
```
⚠️ RTS: No feasible workers, falling back to Round-Robin
```

---

## **Quick Troubleshooting**

| Problem | Command | Solution |
|---------|---------|----------|
| Tasks stuck in queue | `queue` + `workers` | Check worker availability |
| Worker shows wrong resources | `internal-state` + `fix-resources` | Reconcile allocations |
| High task failures | `list-tasks failed` | Check worker logs |
| Want to see GA parameters | `cat master/config/ga_output.json \| jq .` | Inspect learned parameters |
| Check SLA violations | MongoDB query | `db.results.find({sla_success:false})` |
| Verify tau updates | Check master logs | Look for tau update messages |

---

## **File Locations**

- **GA Parameters**: `master/config/ga_output.json`
- **Master Logs**: `master.log` (if backgrounded)
- **Environment Config**: `.env`
- **MongoDB Database**: `cloudai` (collections: tasks, assignments, results, workers)

---

## **Useful MongoDB Queries**

### **Check SLA Success Rate**:
```javascript
mongo cloudai --eval "
db.results.aggregate([
  { \$group: {
    _id: '\$task_type',
    total: { \$sum: 1 },
    success: { \$sum: { \$cond: ['\$sla_success', 1, 0] } }
  }},
  { \$project: {
    task_type: '\$_id',
    sla_rate: { \$multiply: [{ \$divide: ['\$success', '\$total'] }, 100] }
  }}
])
"
```

### **Find Recent SLA Violations**:
```javascript
mongo cloudai --eval "
db.results.find(
  { sla_success: false },
  { task_id: 1, task_type: 1, worker_id: 1, completed_at: 1 }
).sort({ completed_at: -1 }).limit(10)
"
```

### **Check Task Distribution by Type**:
```javascript
mongo cloudai --eval "
db.tasks.aggregate([
  { \$group: { _id: '\$task_type', count: { \$sum: 1 } } }
])
"
```

---

**Tip**: Keep this reference handy while testing or operating the CloudAI cluster!
