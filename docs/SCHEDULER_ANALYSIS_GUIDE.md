# Scheduler Performance Analysis Guide

## ✅ Fixed batch_submit.sh

### Changes Made:
1. **Reduced total tasks** from 40 to 40 (removed GPU tasks as you don't have GPU workers)
2. **K-values now in range 1.5-2.5**:
   - `cpu-heavy`: 2.5 (highest priority)
   - `memory-heavy`: 2.3 (high priority)
   - `mixed`: 2.0 (medium priority)
   - `cpu-light`: 1.5 (lowest priority)
3. **Better output formatting** with resource details
4. **Proper task type tags** that will display correctly in UI

### Task Distribution:
- 5 cpu-intensive tasks (4 CPU, 4GB RAM)
- 10 cpu-light tasks (1 CPU, 2GB RAM)
- 5 cpu-heavy tasks (4 CPU, 4GB RAM)
- 10 memory-heavy tasks (1 CPU, 8GB RAM)
- 5 mixed tasks (2 CPU, 4GB RAM)
- **Total: 35 tasks**

---

## 📊 Data Needed for Scheduler Analysis

### 1. **Database Collections to Export**

#### A. TASK_ASSIGNMENTS Collection
```javascript
// Export all assignments
db.TASK_ASSIGNMENTS.find({}).toArray()
```

**Key Fields:**
- `task_id` - Task identifier
- `worker_id` - Which worker was assigned
- `assigned_at` - Assignment timestamp
- `load_at_start` - Worker load at assignment time

**What This Tells Us:**
- Which tasks went to which workers
- Worker load distribution over time
- Assignment patterns (are tasks being distributed fairly?)

#### B. TASK_RESULTS Collection
```javascript
// Export completed tasks
db.TASK_RESULTS.find({}).toArray()
```

**Key Fields:**
- `task_id` - Task identifier
- `worker_id` - Worker that executed it
- `status` - completed/failed
- `execution_time_seconds` - How long it took
- `completed_at` - Completion timestamp
- `sla_success` - Whether SLA was met
- `logs` - Error messages if failed

**What This Tells Us:**
- Task success/failure rates
- Execution times per task type
- SLA compliance
- Which workers perform better for which task types

#### C. TASK_HISTORY Collection
```javascript
// Export task execution history
db.TASK_HISTORY.find({}).toArray()
```

**Key Fields:**
- `task_id`, `worker_id`, `task_type`
- `cpu_required`, `memory_required`, `gpu_required`
- `execution_time`
- `queued_time`, `assigned_time`, `started_time`, `completed_time`
- `sla_met`

**What This Tells Us:**
- Complete task lifecycle timing
- Queue wait times
- Resource utilization patterns

#### D. WORKER_REGISTRY Collection
```javascript
// Export worker information
db.WORKER_REGISTRY.find({}).toArray()
```

**Key Fields:**
- `worker_id`, `worker_ip`
- `total_cpu`, `total_memory`, `total_storage`, `total_gpu`
- `allocated_cpu`, `allocated_memory`, `available_cpu`, `available_memory`
- `is_active`, `last_heartbeat`

**What This Tells Us:**
- Worker capacity and availability
- Resource allocation accuracy

---

## 📈 Key Metrics to Calculate

### 1. **Task Distribution Metrics**

```javascript
// Count tasks per worker
db.TASK_ASSIGNMENTS.aggregate([
  { $group: { _id: "$worker_id", count: { $sum: 1 } } },
  { $sort: { count: -1 } }
])
```

**Good Sign:** Even distribution across workers  
**Bad Sign:** One worker getting 90% of tasks

### 2. **Task Type Affinity**

```javascript
// Check which task types went to which workers
db.TASK_HISTORY.aggregate([
  { $group: { 
      _id: { worker: "$worker_id", type: "$task_type" },
      count: { $sum: 1 },
      avg_exec_time: { $avg: "$execution_time" }
  }}
])
```

**What to Look For:**
- Are `cpu-heavy` tasks going to workers with more CPU?
- Are `memory-heavy` tasks going to workers with more RAM?
- Is RTS scheduler respecting task affinity?

### 3. **SLA Success Rate**

```javascript
// Calculate SLA success rate by scheduler
db.TASK_RESULTS.aggregate([
  { $group: {
      _id: null,
      total: { $sum: 1 },
      sla_met: { $sum: { $cond: ["$sla_success", 1, 0] } }
  }},
  { $project: {
      total: 1,
      sla_met: 1,
      sla_percentage: { $multiply: [{ $divide: ["$sla_met", "$total"] }, 100] }
  }}
])
```

**Goal:** Higher SLA success rate = better scheduler

### 4. **Average Execution Time by Task Type**

```javascript
db.TASK_RESULTS.aggregate([
  { $group: {
      _id: "$task_type",
      avg_time: { $avg: "$execution_time_seconds" },
      min_time: { $min: "$execution_time_seconds" },
      max_time: { $max: "$execution_time_seconds" },
      count: { $sum: 1 }
  }},
  { $sort: { avg_time: -1 } }
])
```

**What to Look For:**
- Consistency in execution times
- Outliers (very long/short executions)

### 5. **Queue Wait Time**

```javascript
db.TASK_HISTORY.aggregate([
  { $project: {
      task_id: 1,
      wait_time: { 
        $subtract: [
          { $dateFromString: { dateString: "$assigned_time" } },
          { $dateFromString: { dateString: "$queued_time" } }
        ]
      }
  }},
  { $group: {
      _id: null,
      avg_wait: { $avg: "$wait_time" },
      max_wait: { $max: "$wait_time" }
  }}
])
```

**Goal:** Lower wait time = better scheduler efficiency

### 6. **Worker Utilization**

```javascript
// Calculate average load per worker
db.TASK_ASSIGNMENTS.aggregate([
  { $group: {
      _id: "$worker_id",
      avg_load: { $avg: "$load_at_start" },
      task_count: { $sum: 1 }
  }},
  { $sort: { avg_load: -1 } }
])
```

**What to Look For:**
- Are workers being overloaded?
- Is load balanced across workers?

---

## 🔍 Issues Found & How to Detect Them

### Issue 1: "NullPointer" Worker Assignments

**Detection:**
```javascript
db.TASK_ASSIGNMENTS.find({ worker_id: "NullPointer" }).count()
```

**Root Cause:**
- Scheduler returning empty worker ID
- Task assignment happening before worker selection completes

**Fix Needed:**
1. Add validation in `selectWorkerForTask()` to never return null
2. Add logging when no suitable worker found
3. Ensure tasks stay queued if no worker available

### Issue 2: Wrong Task Types Going to Wrong Workers

**Detection:**
```javascript
// Find cpu-heavy tasks on low-CPU workers
db.TASK_HISTORY.find({
  task_type: "cpu-heavy",
  worker_id: { $in: ["Shehzada", "Kiwi"] }  // Assuming these have low CPU
})
```

**What to Check:**
1. Worker resource specifications in `WORKER_REGISTRY`
2. RTS scheduler affinity calculations
3. Round-robin fallback behavior

### Issue 3: All Tasks Going to Same Worker

**Detection:**
```javascript
// Count tasks per worker
db.TASK_ASSIGNMENTS.aggregate([
  { $group: { _id: "$worker_id", count: { $sum: 1 } } },
  { $project: { 
      worker: "$_id", 
      count: 1,
      percentage: { $multiply: [
        { $divide: ["$count", { $sum: "$count" }] }, 
        100
      ]}
  }}
])
```

**If one worker has >80%:** Scheduler is not distributing properly

**Possible Causes:**
1. Other workers marked as inactive
2. Scheduler always selecting same worker
3. Resource tracking not updating properly

### Issue 4: Docker Image Pull Failures

**Detection:**
```javascript
db.TASK_RESULTS.find({ 
  status: "failed",
  logs: /pull access denied/
}).count()
```

**Root Cause:**
- Images are private or don't exist
- Docker Hub authentication missing

**Fix:**
- Ensure images are public on Docker Hub
- Or add Docker credentials to workers

---

## 📋 Scheduler Comparison Methodology

### A. Run Tests with Round-Robin

```bash
# Set scheduler to Round-Robin
master> scheduler set round-robin

# Clear previous results
# In MongoDB:
db.TASK_ASSIGNMENTS.deleteMany({})
db.TASK_RESULTS.deleteMany({})
db.TASK_HISTORY.deleteMany({})

# Submit tasks
bash test/batch_submit.sh

# Wait for all tasks to complete (~30 minutes)

# Export results
mongoexport --db=cloudai --collection=TASK_RESULTS --out=results_roundrobin.json
mongoexport --db=cloudai --collection=TASK_HISTORY --out=history_roundrobin.json
```

### B. Run Tests with RTS

```bash
# Train RTS scheduler with AOD
master> aod train

# Set scheduler to RTS
master> scheduler set rts

# Clear previous results
db.TASK_ASSIGNMENTS.deleteMany({})
db.TASK_RESULTS.deleteMany({})
db.TASK_HISTORY.deleteMany({})

# Submit tasks
bash test/batch_submit.sh

# Wait for all tasks to complete

# Export results
mongoexport --db=cloudai --collection=TASK_RESULTS --out=results_rts.json
mongoexport --db=cloudai --collection=TASK_HISTORY --out=history_rts.json
```

### C. Compare Results

| Metric | Round-Robin | RTS | Winner |
|--------|------------|-----|--------|
| Total Tasks Completed | ? | ? | ? |
| SLA Success Rate (%) | ? | ? | ? |
| Avg Execution Time (s) | ? | ? | ? |
| Avg Queue Wait Time (s) | ? | ? | ? |
| Task Failures | ? | ? | ? |
| Load Balance (std dev) | ? | ? | ? |
| CPU Utilization (%) | ? | ? | ? |
| Memory Utilization (%) | ? | ? | ? |

---

## 🚨 Critical Logs to Check

### 1. Master Logs - Scheduler Selection

```bash
# Look for these patterns:
grep "RTS: Selected worker" master.log
grep "Round-robin selected" master.log
grep "No suitable worker" master.log
grep "Queue: Task.*still waiting" master.log
```

**What to Look For:**
- Is scheduler being called?
- Is it selecting different workers?
- Are tasks stuck in queue?

### 2. Master Logs - Worker Status

```bash
grep "Worker.*marked as inactive" master.log
grep "Worker.*registered" master.log
grep "Worker.*disconnected" master.log
```

**What to Look For:**
- Are workers staying connected?
- Frequent disconnects = network issues

### 3. Master Logs - Task Assignment

```bash
grep "Task.*assigned to" master.log
grep "Task.*queued" master.log
grep "Assignment failed" master.log
```

**What to Look For:**
- Assignment success rate
- Reasons for queuing

### 4. Worker Logs - Task Execution

```bash
# On worker machines:
grep "Received task" worker.log
grep "Task.*completed" worker.log
grep "Task.*failed" worker.log
grep "Error pulling image" worker.log
```

**What to Look For:**
- Are tasks being received?
- Why are they failing?

---

## 📊 Quick Analysis Script

Create `test/analyze_scheduler.sh`:

```bash
#!/bin/bash

echo "=== Scheduler Performance Analysis ==="
echo ""

echo "1. Task Distribution:"
mongosh cloudai --quiet --eval '
  db.TASK_ASSIGNMENTS.aggregate([
    { $group: { _id: "$worker_id", count: { $sum: 1 } } },
    { $sort: { count: -1 } }
  ]).forEach(doc => print(doc._id + ": " + doc.count + " tasks"))
'

echo ""
echo "2. Task Success Rate:"
mongosh cloudai --quiet --eval '
  var result = db.TASK_RESULTS.aggregate([
    { $group: {
        _id: null,
        total: { $sum: 1 },
        completed: { $sum: { $cond: [{ $eq: ["$status", "completed"] }, 1, 0] } },
        failed: { $sum: { $cond: [{ $eq: ["$status", "failed"] }, 1, 0] } }
    }}
  ]).toArray()[0];
  print("Total: " + result.total);
  print("Completed: " + result.completed + " (" + (result.completed/result.total*100).toFixed(2) + "%)");
  print("Failed: " + result.failed + " (" + (result.failed/result.total*100).toFixed(2) + "%)");
'

echo ""
echo "3. Average Execution Time by Type:"
mongosh cloudai --quiet --eval '
  db.TASK_HISTORY.aggregate([
    { $group: {
        _id: "$task_type",
        avg_time: { $avg: "$execution_time" },
        count: { $sum: 1 }
    }},
    { $sort: { avg_time: -1 } }
  ]).forEach(doc => print(doc._id + ": " + doc.avg_time.toFixed(2) + "s (n=" + doc.count + ")"))
'

echo ""
echo "4. NullPointer Assignments:"
mongosh cloudai --quiet --eval '
  print(db.TASK_ASSIGNMENTS.find({ worker_id: "NullPointer" }).count() + " tasks assigned to NullPointer")
'
```

---

## ✅ Action Items

### Immediate Fixes Needed:

1. **Fix "NullPointer" issue:**
   - Add validation in `selectWorkerForTask()`
   - Never return empty worker ID
   - Log when no workers available

2. **Fix Docker images:**
   - Make sure all images in batch_submit.sh exist and are public
   - Or add Docker Hub authentication to workers

3. **Improve logging:**
   - Log every task assignment with worker ID and reason
   - Log scheduler decision-making process
   - Log worker resource state at assignment time

4. **Add monitoring:**
   - Real-time dashboard showing task distribution
   - Worker load visualization
   - Queue depth over time

### For Scheduler Comparison:

1. **Run both schedulers** with same task set
2. **Export all data** from MongoDB
3. **Calculate metrics** (SLA success, exec time, wait time)
4. **Compare results** side-by-side
5. **Document findings** with graphs and tables

---

## 🎯 Expected Outcomes

### Good Scheduler Behavior:
✅ Tasks distributed evenly across workers  
✅ Task types matched to appropriate workers  
✅ High SLA success rate (>90%)  
✅ Low queue wait times (<5 seconds)  
✅ No "NullPointer" assignments  
✅ RTS performs better than Round-Robin for mixed workloads

### Red Flags:
❌ One worker getting >80% of tasks  
❌ High failure rate (>10%)  
❌ Long queue wait times (>30 seconds)  
❌ Tasks assigned to "NullPointer"  
❌ Wrong task types on wrong workers (cpu-heavy on low-CPU worker)

---

## 📝 Summary

**To compare RTS vs Round-Robin, you need:**

1. **Run both schedulers** with identical task sets
2. **Collect data** from 4 MongoDB collections
3. **Calculate 6 key metrics** (distribution, SLA, exec time, wait time, utilization, failures)
4. **Check logs** for errors and decision traces
5. **Compare results** in a table
6. **Document findings** with evidence

**Most Important Metrics:**
- **SLA Success Rate** - Higher is better
- **Task Distribution** - More even is better
- **Average Execution Time** - Lower is better
- **Queue Wait Time** - Lower is better

The scheduler that scores better on these metrics is the winner! 🏆
