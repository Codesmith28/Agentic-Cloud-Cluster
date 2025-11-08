# Task Execution & Monitoring - Testing Guide

## Pre-Test Setup

### 1. Environment Requirements

- **Go**: 1.19 or higher
- **Docker**: 20.10 or higher
- **MongoDB**: 4.4 or higher (optional, for persistence)
- **Operating System**: Linux (recommended), macOS, or Windows with WSL2

### 2. Build Components

```bash
# Build master
cd /home/codesmith28/Projects/CloudAI/master
go build -o masterNode .

# Build worker
cd /home/codesmith28/Projects/CloudAI/worker
go build -o workerNode .

# Verify builds
ls -lh masterNode
ls -lh ../worker/workerNode
```

### 3. Start MongoDB (Optional)

```bash
cd /home/codesmith28/Projects/CloudAI/database
docker-compose up -d

# Verify MongoDB is running
docker ps | grep mongo
```

### 4. Pull Test Images

```bash
# Pull images for testing
docker pull hello-world:latest
docker pull alpine:latest
docker pull python:3.9-slim
docker pull ubuntu:22.04
```

## Test Suite

### Test 1: Basic Task Execution

**Objective**: Verify task can be assigned and executed successfully

```bash
# Terminal 1: Start Master
cd /home/codesmith28/Projects/CloudAI/master
./masterNode

# Terminal 2: Start Worker
cd /home/codesmith28/Projects/CloudAI/worker
./workerNode

# Note the worker details from worker logs
# Example: Worker ID: hostname, Address: 192.168.1.100:50052

# Terminal 3: Master CLI
# Register worker (use actual address from worker logs)
master> register worker-1 <worker-address>

# Assign simple task
master> task worker-1 hello-world:latest -cpu_cores 0.5 -mem 0.25

# Expected: Task assigned successfully with task ID
```

**Success Criteria**:
- ✅ Worker registers successfully
- ✅ Task is assigned without errors
- ✅ Task ID is displayed
- ✅ Worker logs show task execution
- ✅ Master logs show task completion

**Verification**:
```bash
# Check worker logs for:
# - "Task accepted - Starting execution..."
# - "Pulling image: hello-world:latest"
# - "Creating container..."
# - "Completed successfully"

# Check master logs for:
# - "Task assigned to worker"
# - "Task completed" (from worker report)
```

---

### Test 2: Resource Constraints

**Objective**: Verify resource limits are applied to containers

```bash
# Terminal 1: Master CLI
master> task worker-1 ubuntu:22.04 -cpu_cores 1.5 -mem 2.0 -storage 10.0

# Note the task ID from output
```

**Verification**:
```bash
# In worker terminal, while task is running
docker ps

# Note container ID for the task, then:
docker inspect <container-id> | grep -A 10 "Resources"

# Expected output should show:
# - NanoCPUs: 1500000000 (1.5 CPUs)
# - Memory: 2147483648 (2 GB)
```

**Success Criteria**:
- ✅ Container created with correct resource limits
- ✅ CPU limit: 1500000000 nanoCPUs (1.5 CPUs)
- ✅ Memory limit: 2147483648 bytes (2 GB)

---

### Test 3: Live Log Monitoring

**Objective**: Verify live log streaming works correctly

```bash
# Create a test script that outputs logs over time
# Terminal 1: Master CLI

# Assign a long-running task
master> task worker-1 alpine:latest -cpu_cores 0.5 -mem 0.25

# Immediately monitor (using task ID from assignment)
master> monitor task-1730899200
```

**Expected Behavior**:
- Terminal clears and shows monitoring header
- Logs appear in real-time as container executes
- "Press any key to exit" message visible
- Pressing any key returns to CLI prompt

**Success Criteria**:
- ✅ Monitoring screen displays correctly
- ✅ Logs stream in real-time
- ✅ Task ID and user ID shown in header
- ✅ Can exit by pressing any key
- ✅ Returns to CLI prompt cleanly

---

### Test 4: Database Persistence

**Objective**: Verify tasks and assignments are stored in database

**Prerequisites**: MongoDB running

```bash
# Terminal 1: Master CLI
master> task worker-1 python:3.9-slim -cpu_cores 1.0 -mem 1.0

# Note task ID, e.g., task-1730899200

# Terminal 2: MongoDB Query
mongo cluster_db

# Query tasks
> db.TASKS.findOne({ task_id: "task-1730899200" })

# Expected output:
{
  task_id: "task-1730899200",
  user_id: "admin",
  docker_image: "python:3.9-slim",
  req_cpu: 1.0,
  req_memory: 1.0,
  status: "running",  // or "completed" if finished
  created_at: ISODate("..."),
  started_at: ISODate("...")
}

# Query assignment
> db.ASSIGNMENTS.findOne({ task_id: "task-1730899200" })

# Expected output:
{
  ass_id: "ass-task-1730899200",
  task_id: "task-1730899200",
  worker_id: "worker-1",
  assigned_at: ISODate("...")
}
```

**Success Criteria**:
- ✅ Task record created in TASKS collection
- ✅ Task status is "pending" then "running"
- ✅ Assignment record created in ASSIGNMENTS collection
- ✅ Timestamps are correctly set
- ✅ All fields populated correctly

---

### Test 5: Task Status Updates

**Objective**: Verify task status updates throughout lifecycle

```bash
# Terminal 1: Master CLI
master> task worker-1 alpine:latest -cpu_cores 0.5 -mem 0.25

# Terminal 2: Monitor database
watch -n 1 'mongo cluster_db --quiet --eval "db.TASKS.findOne({task_id: \"task-1730899200\"}, {task_id:1, status:1, created_at:1, started_at:1, completed_at:1})"'
```

**Expected Status Progression**:
1. `status: "pending"` - Just created
2. `status: "running"` - Worker started execution
3. `status: "completed"` or `"failed"` - Task finished

**Success Criteria**:
- ✅ Status starts as "pending"
- ✅ Status changes to "running" when task starts
- ✅ Status changes to "completed" on success
- ✅ Timestamps updated at each stage
- ✅ `started_at` set when running
- ✅ `completed_at` set when finished

---

### Test 6: Command Execution

**Objective**: Verify custom commands are executed in containers

```bash
# Terminal 1: Master CLI
master> task worker-1 python:3.9-slim -cpu_cores 1.0 -mem 0.5

# Monitor immediately to see command output
master> monitor <task-id>
```

**Expected**:
- Python command executes in container
- Command is run as: `/bin/sh -c "<command>"`
- Output appears in monitoring logs

**Success Criteria**:
- ✅ Command executes successfully
- ✅ Output visible in logs
- ✅ Exit code 0 on success

---

### Test 7: Error Handling

**Objective**: Verify graceful error handling

#### Test 7a: Non-existent Image
```bash
master> task worker-1 nonexistent/image:latest -cpu_cores 1.0
```

**Expected**:
- Worker attempts to pull image
- Pull fails
- Task status: "failed"
- Error logged

**Success Criteria**:
- ✅ Error message displayed
- ✅ Task marked as failed in database
- ✅ Worker continues running
- ✅ Master continues running

#### Test 7b: Monitor Non-existent Task
```bash
master> monitor task-nonexistent
```

**Expected**:
- Error message: "Task not found" or similar
- Returns to CLI prompt

**Success Criteria**:
- ✅ Error message displayed
- ✅ No crash
- ✅ Returns to CLI cleanly

#### Test 7c: Worker Disconnect During Task
```bash
# Start task
master> task worker-1 alpine:latest

# Kill worker
# In worker terminal: Ctrl+C

# Try to monitor
master> monitor <task-id>
```

**Expected**:
- Connection error when trying to stream logs
- Error message displayed

**Success Criteria**:
- ✅ Error handled gracefully
- ✅ Master doesn't crash
- ✅ Error message is user-friendly

---

### Test 8: Multiple Tasks

**Objective**: Verify system handles multiple concurrent tasks

```bash
# Terminal 1: Master CLI
master> task worker-1 alpine:latest -cpu_cores 0.5 -mem 0.25
master> task worker-1 python:3.9-slim -cpu_cores 0.5 -mem 0.5
master> task worker-1 ubuntu:22.04 -cpu_cores 1.0 -mem 1.0

# Check worker can handle multiple tasks
```

**Success Criteria**:
- ✅ All tasks assigned successfully
- ✅ Each gets unique task ID
- ✅ All tasks execute (may be sequential or parallel)
- ✅ All tasks tracked in database
- ✅ Can monitor any task independently

---

### Test 9: Monitor After Completion

**Objective**: Verify can view logs of completed tasks

```bash
# Terminal 1: Master CLI
master> task worker-1 hello-world:latest -cpu_cores 0.5

# Wait for task to complete (watch worker logs)

# Then monitor
master> monitor <task-id>
```

**Expected**:
- Shows stored logs from completed task
- Displays "Task Completed" message
- Shows final status

**Success Criteria**:
- ✅ Can monitor completed tasks
- ✅ Shows final logs
- ✅ Shows completion status
- ✅ Doesn't show as "running"

---

### Test 10: Long-Running Task Monitoring

**Objective**: Verify monitoring handles long-running tasks

```bash
# Create test script that outputs periodically
# Terminal 1: Master CLI
master> task worker-1 ubuntu:22.04 -cpu_cores 1.0 -mem 1.0

# Monitor immediately
master> monitor <task-id>

# Let it run for 30 seconds
# Press key to exit before completion
```

**Success Criteria**:
- ✅ Logs stream continuously
- ✅ Can exit monitoring mid-stream
- ✅ Task continues running after exit
- ✅ No memory leaks or performance issues

---

## Integration Tests

### Integration Test 1: Full Workflow

**Test the complete lifecycle**:

```bash
# 1. Start services
# Start master, worker, MongoDB

# 2. Register worker
master> register worker-1 <address>

# 3. Check status
master> workers
master> status

# 4. Submit task
master> task worker-1 python:3.9-slim -cpu_cores 2.0 -mem 4.0

# 5. Monitor task
master> monitor <task-id>

# 6. Verify database
mongo cluster_db --eval "db.TASKS.find().pretty()"
mongo cluster_db --eval "db.ASSIGNMENTS.find().pretty()"

# 7. Check completion
master> status
```

**Success Criteria**: All steps complete without errors

---

### Integration Test 2: Recovery After Failure

```bash
# 1. Start task
master> task worker-1 alpine:latest

# 2. Kill worker (simulate crash)
# Ctrl+C in worker terminal

# 3. Restart worker
./workerNode

# 4. Check system recovery
master> workers
# Worker should re-register automatically
```

---

## Performance Tests

### Performance Test 1: Rapid Task Submission

```bash
# Submit 10 tasks rapidly
for i in {1..10}; do
  echo "task worker-1 alpine:latest -cpu_cores 0.5 -mem 0.25" | timeout 5 ./masterNode
done

# Check all tasks tracked
mongo cluster_db --eval "db.TASKS.count()"
```

**Success Criteria**:
- ✅ All tasks created
- ✅ No race conditions
- ✅ All tasks in database

### Performance Test 2: Log Streaming Performance

```bash
# Task with high log output
master> task worker-1 ubuntu:22.04 -cpu_cores 1.0 -mem 1.0
master> monitor <task-id>

# Observe:
# - Log streaming latency
# - CPU usage on master/worker
# - Memory usage
```

---

## Cleanup After Tests

```bash
# Stop services
# Ctrl+C in master and worker terminals

# Clean up MongoDB
mongo cluster_db --eval "db.TASKS.deleteMany({})"
mongo cluster_db --eval "db.ASSIGNMENTS.deleteMany({})"
mongo cluster_db --eval "db.WORKER_REGISTRY.deleteMany({})"

# Or drop database
mongo cluster_db --eval "db.dropDatabase()"

# Clean up Docker
docker ps -a | grep task- | awk '{print $1}' | xargs docker rm -f

# Remove test images (optional)
docker rmi hello-world:latest alpine:latest python:3.9-slim ubuntu:22.04
```

---

## Test Results Template

```markdown
## Test Execution Report
Date: _________________
Tester: _______________

### Environment
- OS: _________________
- Go Version: _________
- Docker Version: _____
- MongoDB Version: ____

### Test Results

| Test # | Test Name | Status | Notes |
|--------|-----------|--------|-------|
| 1 | Basic Task Execution | ☐ Pass ☐ Fail | |
| 2 | Resource Constraints | ☐ Pass ☐ Fail | |
| 3 | Live Log Monitoring | ☐ Pass ☐ Fail | |
| 4 | Database Persistence | ☐ Pass ☐ Fail | |
| 5 | Task Status Updates | ☐ Pass ☐ Fail | |
| 6 | Command Execution | ☐ Pass ☐ Fail | |
| 7a | Non-existent Image | ☐ Pass ☐ Fail | |
| 7b | Monitor Non-existent | ☐ Pass ☐ Fail | |
| 7c | Worker Disconnect | ☐ Pass ☐ Fail | |
| 8 | Multiple Tasks | ☐ Pass ☐ Fail | |
| 9 | Monitor After Complete | ☐ Pass ☐ Fail | |
| 10 | Long-Running Monitor | ☐ Pass ☐ Fail | |

### Issues Found
1. ___________________________________
2. ___________________________________
3. ___________________________________

### Overall Assessment
☐ All tests passed - Ready for production
☐ Minor issues - Acceptable for development
☐ Major issues - Needs fixes before use
```

---

## Troubleshooting Common Test Failures

### Issue: Worker won't register
**Solution**: Check network connectivity, verify ports are open

### Issue: Tasks stuck in "pending"
**Solution**: Check Docker daemon on worker, verify image exists

### Issue: Log monitoring shows nothing
**Solution**: Verify task is running, check gRPC connection

### Issue: Database not updating
**Solution**: Check MongoDB is running, verify connection string

### Issue: Resource limits not applied
**Solution**: Update Docker version, check cgroup support

---

*Testing Guide - Version 1.0*
*Last updated: November 6, 2025*
