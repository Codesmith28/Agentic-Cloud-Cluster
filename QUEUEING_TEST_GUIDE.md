# Task Queueing Test Guide

## Overview

This guide will help you test the task queueing behavior in CloudAI by simulating resource exhaustion scenarios and observing how the system handles multiple tasks.

## Prerequisites

✅ MongoDB running  
✅ At least 1 worker registered and active  
✅ Master node running  
✅ Worker node running  

---

## Test Scenario 1: Resource Exhaustion

### Goal
Submit multiple tasks that exceed worker capacity and observe queueing behavior.

### Steps

#### 1. Start the System

```bash
# Terminal 1: Start MongoDB
cd database && docker-compose up -d

# Terminal 2: Start Master
cd master && ./runMaster.sh

# Terminal 3: Start Worker
cd worker && ./runWorker.sh
```

#### 2. Check Worker Resources

```bash
# In master CLI
master> workers
```

**Note the available resources:**
- Total CPU
- Total Memory  
- Total GPU
- Total Storage

Example output:
```
Worker: worker-1
  CPU: 4.0 cores
  Memory: 8.0 GB
  GPU: 0.0 cores
  Storage: 50.0 GB
```

#### 3. Submit First Task (Should Succeed Immediately)

```bash
master> task worker-1 ubuntu:latest -cpu_cores 2.0 -mem 2.0
```

**Expected Result:**
- ✅ Task assigned immediately
- Status: "running"
- Worker shows task in running tasks

#### 4. Submit Second Task (May Queue if Resources Low)

```bash
master> task worker-1 ubuntu:latest -cpu_cores 2.0 -mem 2.0
```

**Expected Result:**
- If worker has resources: Assigns immediately
- If no resources: May fail or queue (depending on implementation)

#### 5. Submit Third Task (Should Queue or Fail)

```bash
master> task worker-1 ubuntu:latest -cpu_cores 2.0 -mem 2.0
```

**Expected Result:**
- Worker exhausted
- Task either queued or error returned

---

## Test Scenario 2: Multiple Workers

### Goal
Test task distribution across multiple workers.

### Steps

#### 1. Register Second Worker

```bash
# Terminal 4: Start second worker on different machine/port
# Edit worker config first to use different port

# In master CLI
master> register worker-2 192.168.1.56:50052
```

#### 2. Verify Both Workers Active

```bash
master> workers
```

Should see:
- worker-1: Active
- worker-2: Active

#### 3. Submit Multiple Tasks

```bash
master> task worker-1 ubuntu:latest -cpu_cores 1.0 -mem 1.0
master> task worker-2 ubuntu:latest -cpu_cores 1.0 -mem 1.0
master> task worker-1 ubuntu:latest -cpu_cores 1.0 -mem 1.0
master> task worker-2 ubuntu:latest -cpu_cores 1.0 -mem 1.0
```

#### 4. Monitor Distribution

```bash
master> stats worker-1
master> stats worker-2
```

**Expected:** Tasks distributed across both workers

---

## Test Scenario 3: Task Cancellation While Queueing

### Goal
Test cancelling tasks in various states.

### Steps

#### 1. Submit Long-Running Task

```bash
master> task worker-1 ubuntu:latest sleep 300
```

#### 2. Note Task ID

Example: `task-1234567890`

#### 3. Cancel Task

```bash
master> cancel task-1234567890
```

#### 4. Verify Cancellation

```bash
# Check MongoDB
mongo cloudai_db
db.TASKS.find({task_id: "task-1234567890"})
```

**Expected:**
- status: "cancelled"
- completed_at: <timestamp>

---

## Test Scenario 4: Worker Failure Recovery

### Goal
Test system behavior when worker goes offline.

### Steps

#### 1. Assign Task to Worker

```bash
master> task worker-1 ubuntu:latest -cpu_cores 1.0
```

#### 2. Stop Worker Mid-Execution

```bash
# In worker terminal
Ctrl+C
```

#### 3. Observe Master Behavior

```bash
master> workers
```

**Expected:**
- Worker-1 shows as inactive
- Task may show as "failed" or "running" depending on timing

#### 4. Restart Worker

```bash
cd worker && ./runWorker.sh
```

#### 5. Verify Worker Re-registers

```bash
master> workers
```

**Expected:**
- Worker-1 shows as active again

---

## Test Scenario 5: Database Persistence

### Goal
Verify task data persists across restarts.

### Steps

#### 1. Submit Multiple Tasks

```bash
master> task worker-1 ubuntu:latest
master> task worker-1 python:3.9
```

#### 2. Query Database

```bash
mongo cloudai_db
db.TASKS.find().pretty()
db.ASSIGNMENTS.find().pretty()
```

**Expected:** All tasks visible in database

#### 3. Restart Master

```bash
# Stop master (Ctrl+C)
# Start again
./runMaster.sh
```

#### 4. Verify Data Persists

```bash
# In MongoDB
db.TASKS.find().count()
```

**Expected:** Task count unchanged

---

## Test Scenario 6: Concurrent Task Submission

### Goal
Test system under concurrent load.

### Method 1: Manual (Simple)

```bash
# Submit tasks rapidly
master> task worker-1 ubuntu:latest
master> task worker-1 ubuntu:latest
master> task worker-1 ubuntu:latest
master> task worker-1 ubuntu:latest
master> task worker-1 ubuntu:latest
```

### Method 2: Script (Advanced)

Create test script:

```bash
#!/bin/bash
# test_concurrent_tasks.sh

for i in {1..10}; do
    echo "Submitting task $i"
    docker exec -it cloudai-master sh -c "echo 'task worker-1 ubuntu:latest' | nc localhost <port>"
    sleep 0.5
done
```

---

## Expected Behaviors

### Current System (Without Queue)

✅ **Tasks Assigned Directly**
- Task goes straight to specified worker
- If worker busy: Error returned
- If worker offline: Error returned

❌ **No Automatic Queuing**
- Tasks don't wait for resources
- No automatic retry
- No load balancing

### With Queueing System (If Implemented)

✅ **Automatic Queuing**
- Insufficient resources → Task queued
- Worker offline → Task queued
- FIFO order maintained

✅ **Background Processing**
- Queue checked every 5 seconds
- Tasks auto-assigned when resources free
- Automatic retry on failure

✅ **Queue Commands**
```bash
master> queue          # List queued tasks
master> queue clear    # Clear queue
```

---

## Monitoring During Tests

### Master Logs

Watch for:
```
✓ Task assigned successfully
✗ Insufficient resources
✓ Worker registered
✗ Worker offline
```

### Worker Logs

Watch for:
```
[Task X] Starting execution
[Task X] Container started
[Task X] Task completed
```

### Database Queries

```javascript
// Check task statuses
db.TASKS.aggregate([
  { $group: { _id: "$status", count: { $sum: 1 } } }
])

// Check running tasks
db.TASKS.find({ status: "running" }).pretty()

// Check failed tasks
db.TASKS.find({ status: "failed" }).pretty()

// Check assignments
db.ASSIGNMENTS.aggregate([
  { $group: { _id: "$worker_id", count: { $sum: 1 } } }
])
```

---

## Troubleshooting

### Issue: Task Stuck in "pending"

**Check:**
```bash
master> workers  # Is worker active?
master> stats worker-1  # Does worker have resources?
```

### Issue: "Worker not found"

**Solution:**
```bash
# Verify worker registration
master> workers

# Re-register if needed
master> register worker-1 <ip:port>
```

### Issue: Database Connection Failed

**Check:**
```bash
# Verify MongoDB running
docker ps | grep mongo

# Check connection
mongo cloudai_db --eval "db.stats()"
```

---

## Success Criteria

✅ **Task Assignment**
- Tasks assign to specified worker
- Resources tracked correctly
- Database updated

✅ **Task Execution**
- Containers start successfully
- Logs stream properly
- Results stored

✅ **Task Cancellation**
- Containers stop
- Status updated to "cancelled"
- Resources freed

✅ **System Recovery**
- Worker re-registration works
- Tasks persist across restarts
- No data loss

---

## Next Steps: Implementing Full Queueing

If you want to implement automatic queueing, you would need:

1. **Queue Data Structure** (`master/internal/server/master_server.go`)
   - Add queue storage
   - Add queue lock for thread safety

2. **Background Processor**
   - Goroutine checking queue every 5s
   - Automatic task assignment

3. **CLI Commands**
   - `queue` - List queued tasks
   - `queue clear` - Clear queue

4. **Status Updates**
   - "queued" status in database
   - Queue position tracking

Would you like help implementing a full queueing system?

---

## Quick Test Checklist

- [ ] MongoDB running
- [ ] Master started
- [ ] Worker started and registered
- [ ] Submit test task
- [ ] Verify task runs
- [ ] Check database records
- [ ] Test cancellation
- [ ] Monitor logs
- [ ] Restart components
- [ ] Verify persistence
