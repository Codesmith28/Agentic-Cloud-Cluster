# CloudAI Testing Checklist

Complete testing guide to verify your CloudAI installation.

## ✅ Pre-Flight Checks

### System Requirements

- [ ] Go 1.22+ installed: `go version`
- [ ] Docker installed: `docker --version`
- [ ] Docker daemon running: `docker ps`
- [ ] protoc installed: `protoc --version`
- [ ] protoc-gen-go installed: `which protoc-gen-go`
- [ ] protoc-gen-go-grpc installed: `which protoc-gen-go-grpc`

### Project Setup

- [ ] Proto code generated: `ls proto/pb/` shows .pb.go files
- [ ] Master symlink created: `ls -la master/proto`
- [ ] Worker symlink created: `ls -la worker/proto`
- [ ] Master dependencies installed: `cd master && go mod tidy`
- [ ] Worker dependencies installed: `cd worker && go mod tidy`
- [ ] .env file exists with MongoDB credentials

---

## 🗄️ Database Tests

### MongoDB

```bash
# Start MongoDB
cd database
docker-compose up -d

# Verify running
docker ps | grep mongodb

# Check logs
docker-compose logs mongodb
```

**Expected**: MongoDB container running on port 27017

- [ ] MongoDB container started
- [ ] No error messages in logs
- [ ] Can connect from master

---

## 🏗️ Build Tests

### Master Node

```bash
cd master
go build -o master-node .
```

**Expected**: No compilation errors, `master-node` binary created

- [ ] Master compiles without errors
- [ ] Binary is executable: `./master-node --help` (will start the CLI)

### Worker Node

```bash
cd worker
go build -o worker-node .
```

**Expected**: No compilation errors, `worker-node` binary created

- [ ] Worker compiles without errors
- [ ] Binary is executable: `./worker-node -h`

### Sample Task

```bash
cd sample_task
docker build -t test/cloudai-sample-task:latest .
docker run --rm test/cloudai-sample-task:latest
```

**Expected**: Container runs and completes successfully

- [ ] Docker image builds
- [ ] Container runs without errors
- [ ] Sees "Task completed successfully!" message

---

## 🚀 Runtime Tests

### Test 1: Master Startup

**Terminal 1:**

```bash
cd master
./master-node
```

**Expected Output:**

```
✓ MongoDB collections ensured
Starting gRPC server on :50051...
✓ Master node started successfully
✓ gRPC server listening on :50051

═══════════════════════════════════════════════════════
  CloudAI Master Node - Interactive CLI
═══════════════════════════════════════════════════════
Type 'help' for available commands

master>
```

**Checks:**

- [ ] No error messages
- [ ] gRPC server starts on port 50051
- [ ] CLI prompt appears
- [ ] MongoDB connection succeeds (or warning if optional)

### Test 2: Master CLI Commands

In the master CLI:

```bash
master> help
```

- [ ] Help message displays all commands

```bash
master> status
```

- [ ] Shows cluster status (0 workers, 0 tasks initially)

```bash
master> workers
```

- [ ] Shows "No workers registered yet" or empty list

### Test 3: Worker Startup

**Terminal 2:**

```bash
cd worker
./worker-node -id worker-1 -ip localhost -master localhost:50051
```

**Expected Output (Worker Terminal):**

```
Starting Worker Node: worker-1
Master Address: localhost:50051
✓ Worker registered: Worker registered successfully
Starting telemetry monitor (interval: 5s)
✓ Worker worker-1 started successfully
✓ gRPC server listening on :50052
✓ Ready to receive tasks...
Heartbeat sent: CPU=30.0%, Memory=45.2MB, Tasks=0
```

**Expected Output (Master Terminal):**

```
Worker registration: worker-1 (IP: localhost, CPU: 4.00, Memory: 8.00 GB)
Heartbeat from worker-1: CPU=30.00%, Memory=45.20MB, Running Tasks=0
```

**Checks:**

- [ ] Worker connects to master
- [ ] Registration succeeds
- [ ] Heartbeats appear every 5 seconds
- [ ] Both terminals show connection activity

### Test 4: Worker Registration

In master CLI:

```bash
master> workers
```

**Expected:**

```
╔═══ Registered Workers ═══
║ worker-1
║   Status: 🟢 Active
║   IP: localhost
║   Resources: CPU=4.0, Memory=8.0GB, GPU=0.0
║   Running Tasks: 0
║
╚═══════════════════════════
```

**Checks:**

- [ ] Worker-1 appears in list
- [ ] Status shows active (🟢)
- [ ] Resources displayed correctly
- [ ] Running tasks = 0

### Test 5: Cluster Status

In master CLI:

```bash
master> status
```

**Expected:**

```
╔═══ Cluster Status ═══
║ Total Workers: 1
║ Active Workers: 1
║ Running Tasks: 0
╚══════════════════════════
```

**Checks:**

- [ ] Total workers = 1
- [ ] Active workers = 1
- [ ] Running tasks = 0

### Test 6: Task Assignment

**Prerequisites:**

- Sample task image pushed to Docker Hub
- Worker is running and registered

In master CLI:

```bash
master> task worker-1 docker.io/<username>/cloudai-sample-task:latest
```

**Expected Output (Master):**

```
Assigning task to worker worker-1...
Docker Image: docker.io/<username>/cloudai-sample-task:latest
✅ Task task-1234567890 assigned successfully!
```

**Expected Output (Worker):**

```
Received task assignment: task-1234567890 (Image: docker.io/.../cloudai-sample-task:latest)
[Task task-1234567890] Starting execution...
[Task task-1234567890] Pulling image: docker.io/.../cloudai-sample-task:latest
[Task task-1234567890] Creating container...
[Task task-1234567890] Starting container: abc123def456
[Task task-1234567890] Waiting for completion...
[Task task-1234567890] ✓ Completed successfully
✓ Task result reported: Task result received
```

**Expected Output (Master - after completion):**

```
Task completion: task-1234567890 from worker worker-1 [Status: success]
Task logs:
==================================================
CloudAI Sample Task - Starting Execution
==================================================
...
✓ Task completed successfully!
```

**Checks:**

- [ ] Task assignment succeeds
- [ ] Worker receives task
- [ ] Docker image pulls successfully
- [ ] Container starts and runs
- [ ] Task completes with success status
- [ ] Logs appear in master terminal
- [ ] Result reported back to master

### Test 7: Multiple Tasks

Assign another task:

```bash
master> task worker-1 docker.io/<username>/cloudai-sample-task:latest
```

**Checks:**

- [ ] Second task executes successfully
- [ ] Each task gets unique ID
- [ ] Both tasks complete independently

### Test 8: Multiple Workers

**Terminal 3:**

```bash
cd worker
./worker-node -id worker-2 -ip localhost -master localhost:50051 -port :50053
```

In master CLI:

```bash
master> workers
```

**Expected:**

```
╔═══ Registered Workers ═══
║ worker-1
║   Status: 🟢 Active
║   ...
║
║ worker-2
║   Status: 🟢 Active
║   ...
╚═══════════════════════════
```

**Checks:**

- [ ] Both workers registered
- [ ] Both workers sending heartbeats
- [ ] Status shows 2 total workers
- [ ] Can assign tasks to worker-2

```bash
master> task worker-2 docker.io/<username>/cloudai-sample-task:latest
```

- [ ] Worker-2 executes task successfully

---

## 🔥 Stress Tests

### Test 9: Rapid Task Assignment

Assign 5 tasks quickly:

```bash
master> task worker-1 docker.io/<username>/cloudai-sample-task:latest
master> task worker-1 docker.io/<username>/cloudai-sample-task:latest
master> task worker-1 docker.io/<username>/cloudai-sample-task:latest
master> task worker-1 docker.io/<username>/cloudai-sample-task:latest
master> task worker-1 docker.io/<username>/cloudai-sample-task:latest
```

**Checks:**

- [ ] All tasks execute sequentially
- [ ] No crashes or errors
- [ ] All tasks complete successfully
- [ ] Logs for all tasks appear

### Test 10: Worker Disconnect/Reconnect

1. In worker terminal, press `Ctrl+C`
2. Wait 10-15 seconds
3. In master CLI: `master> workers`
   - [ ] Worker shows as inactive (or removed)
4. Restart worker: `./worker-node -id worker-1`
   - [ ] Worker re-registers successfully
   - [ ] Heartbeats resume
   - [ ] Can assign new tasks

---

## 🧹 Cleanup Tests

### Test 11: Graceful Shutdown

**Master:**

```bash
master> exit
```

- [ ] Master shuts down cleanly
- [ ] No error messages

**Worker:**
Press `Ctrl+C`

- [ ] Worker receives shutdown signal
- [ ] "Shutting down worker..." message
- [ ] gRPC server stops gracefully
- [ ] Process exits cleanly

### Test 12: Database Persistence

1. Stop master: `master> exit`
2. Stop worker: `Ctrl+C`
3. Restart master: `./master-node`
4. Check MongoDB collections (optional):
   ```bash
   docker exec -it mongodb mongosh -u admin -p password123
   use cluster_db
   show collections
   ```
   - [ ] Collections exist: USERS, WORKER_REGISTRY, TASKS, ASSIGNMENTS, RESULTS

---

## 🐛 Error Handling Tests

### Test 13: Invalid Docker Image

```bash
master> task worker-1 docker.io/nonexistent/image:latest
```

**Expected:**

- [ ] Task starts
- [ ] Worker reports pull failure
- [ ] Master receives failed status
- [ ] Error logged, system remains stable

### Test 14: Non-existent Worker

```bash
master> task worker-99 docker.io/<username>/cloudai-sample-task:latest
```

**Expected:**

- [ ] Master shows error: "Worker 'worker-99' not found"
- [ ] Task not assigned
- [ ] Master remains functional

### Test 15: Master Unavailable

1. Stop master
2. Try starting worker: `./worker-node -id worker-1`

**Expected:**

- [ ] Worker fails to register
- [ ] Error message: "Failed to register with master"
- [ ] Worker exits gracefully

---

## 📊 Performance Tests

### Test 16: Heartbeat Accuracy

Watch master logs for 30 seconds, count heartbeats from worker-1:

**Expected:**

- [ ] ~6 heartbeats in 30 seconds (every 5 seconds)
- [ ] Heartbeats are regular
- [ ] No missed heartbeats

### Test 17: Task Execution Time

Assign a task and measure time:

**Expected:**

- [ ] Task completes in ~10-15 seconds (for sample task)
- [ ] Time includes: image pull + container start + execution + cleanup

---

## 🎯 Integration Tests

### Test 18: End-to-End Workflow

Complete workflow test:

1. Start MongoDB ✓
2. Start Master ✓
3. Start Worker-1 ✓
4. Check worker registration ✓
5. Assign task ✓
6. Wait for completion ✓
7. Verify logs in master ✓
8. Verify result reported ✓
9. Check cluster status ✓
10. Start Worker-2 ✓
11. Assign task to Worker-2 ✓
12. Both workers complete tasks ✓
13. Graceful shutdown ✓

**All steps should complete without errors**

---

## 🏆 Success Criteria

Your CloudAI system is fully functional if:

- [x] All pre-flight checks pass
- [x] Master starts and CLI works
- [x] Worker registers and sends heartbeats
- [x] Tasks can be assigned and execute
- [x] Logs are collected and displayed
- [x] Results are reported back
- [x] Multiple workers can operate simultaneously
- [x] Error handling works correctly
- [x] Graceful shutdown works

---

## 📝 Test Results Log

Date: ******\_******
Tester: ******\_******

| Test                | Status        | Notes |
| ------------------- | ------------- | ----- |
| Pre-flight checks   | ☐ Pass ☐ Fail |       |
| Master startup      | ☐ Pass ☐ Fail |       |
| Worker registration | ☐ Pass ☐ Fail |       |
| Task assignment     | ☐ Pass ☐ Fail |       |
| Task execution      | ☐ Pass ☐ Fail |       |
| Multiple workers    | ☐ Pass ☐ Fail |       |
| Error handling      | ☐ Pass ☐ Fail |       |
| Graceful shutdown   | ☐ Pass ☐ Fail |       |

**Overall Result:** ☐ PASS ☐ FAIL

**Additional Notes:**

---

---

---
