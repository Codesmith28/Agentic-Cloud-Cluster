# CloudAI - Function-Level Architecture Documentation

**Generated:** November 15, 2025  
**System:** Distributed Docker-based Task Execution Platform

---

## Table of Contents

1. [System Overview](#system-overview)
2. [Master Node Architecture](#master-node-architecture)
3. [Worker Node Architecture](#worker-node-architecture)
4. [Database Layer Architecture](#database-layer-architecture)
5. [Communication Protocol](#communication-protocol)
6. [Data Flow Diagrams](#data-flow-diagrams)

---

## System Overview

CloudAI is a distributed computing platform that orchestrates Docker-based task execution across a cluster of worker nodes using Go and gRPC. The system follows a master-worker architecture with real-time telemetry, persistent storage, and WebSocket support for live monitoring.

### Core Components
- **Master Node**: Orchestrates workers, assigns tasks, collects telemetry
- **Worker Nodes**: Execute Docker containers, report status
- **MongoDB**: Persistent storage for workers, tasks, assignments, results
- **gRPC**: Bidirectional communication protocol
- **WebSocket Server**: Real-time telemetry streaming

---

## Master Node Architecture

### 1. Main Entry Point (`master/main.go`)

#### `main()`
**Purpose:** Application entry point and initialization orchestrator

**Input:** None (reads from environment variables)

**Output:** Running master node server

**Working:**
```
1. Load configuration from environment (.env)
2. Collect system information (hostname, IP, CPU cores, etc.)
3. Initialize MongoDB connection (10s timeout)
4. Create database handlers (WorkerDB, TaskDB, AssignmentDB, ResultDB)
5. Initialize TelemetryManager (30s inactivity timeout)
6. Create MasterServer with all dependencies
7. Set master ID and address
8. Load pre-registered workers from database
9. **Start task queue processor** (background scheduling)
10. Start gRPC server on port 50051 (background goroutine)
11. Start HTTP/WebSocket telemetry server on port 8080 (optional)
12. Set up graceful shutdown handlers (SIGINT, SIGTERM)
13. Start interactive CLI (blocking)
```

**Key Dependencies:**
- `config.LoadConfig()`
- `system.CollectSystemInfo()`
- `db.EnsureCollections()`
- `telemetry.NewTelemetryManager()`
- `server.NewMasterServer()`
- **`server.StartQueueProcessor()`**

---

### 2. Server Layer (`master/internal/server/master_server.go`)

#### `NewMasterServer(workerDB, taskDB, assignmentDB, resultDB, telemetryMgr) *MasterServer`
**Input:**
- `workerDB *db.WorkerDB` - Worker database handler
- `taskDB *db.TaskDB` - Task database handler
- `assignmentDB *db.AssignmentDB` - Assignment database handler
- `resultDB *db.ResultDB` - Result database handler
- `telemetryMgr *telemetry.TelemetryManager` - Telemetry manager

**Output:** Initialized MasterServer instance

**Working:**
- Creates in-memory worker registry (map)
- Initializes task channel (buffered, size 100)
- **Creates task queue** (slice of QueuedTask)
- **Initializes Round-Robin scheduler** (default)
- Stores database and telemetry manager references

---

#### **NEW: Task Queue System**

#### `QueuedTask` struct
**Fields:**
- `Task *pb.Task` - The actual task
- `QueuedAt time.Time` - When task was queued
- `Retries int` - Number of scheduling attempts
- `LastError string` - Last failure reason

**Purpose:** Track tasks waiting for resources

---

#### `StartQueueProcessor()`
**Purpose:** Start background task queue processor

**Working:**
```
1. Create 5-second ticker
2. Launch goroutine calling processQueue()
3. Log startup
```

**Background Processing:** Runs continuously, tries to schedule queued tasks

---

#### `StopQueueProcessor()`
**Purpose:** Stop the queue processor gracefully

**Working:**
```
1. Stop ticker
2. Goroutine exits on next tick
3. Log shutdown
```

---

#### `processQueue()`
**Purpose:** Main scheduler loop - assigns queued tasks to workers

**Working:**
```
1. On each 5-second tick:
   a. Lock task queue
   b. If queue empty: continue
   c. For each queued task:
      i. Call selectWorkerForTask() (uses scheduler)
      ii. If no worker available:
          - Increment retries
          - Keep in queue
          - Log every 1st and 10th retry
      iii. If worker found:
          - Set Task.TargetWorkerId
          - Call assignTaskToWorker()
          - If success: Remove from queue
          - If fail: Keep in queue, increment retries
   d. Update queue with remaining tasks
   e. Unlock queue
```

**Retry Logic:** Tasks stay in queue until resources available

---

#### `selectWorkerForTask(task *pb.Task) string`
**Purpose:** Use scheduler to find best worker for task

**Working:**
```
1. Get all workers from registry
2. Convert WorkerState → scheduler.WorkerInfo:
   - WorkerID, IsActive, WorkerIP
   - AvailableCPU, AvailableMemory, AvailableStorage, AvailableGPU
3. Call scheduler.SelectWorker(task, workerInfos)
4. Return selected worker ID (or "" if none suitable)
```

**Scheduler Integration:** Pluggable - currently uses Round-Robin

---

#### `addTaskToQueue(task *pb.Task)`
**Purpose:** Add task to queue for scheduling

**Working:**
```
1. Lock queue
2. Create QueuedTask:
   - Task = task
   - QueuedAt = now
   - Retries = 0
   - LastError = ""
3. Append to taskQueue
4. Unlock queue
5. Log task queued
```

---

#### `GetQueueStatus() []*QueuedTask`
**Purpose:** Get snapshot of current queue (for CLI)

**Working:**
```
1. Lock queue (read)
2. Create copy of taskQueue
3. Unlock
4. Return copy
```

---

#### **MODIFIED: WorkerState struct**
**NEW Fields added:**
```go
// Resource tracking
AllocatedCPU     float64  // CPU allocated to tasks
AllocatedMemory  float64  // Memory allocated to tasks
AllocatedStorage float64  // Storage allocated to tasks
AllocatedGPU     float64  // GPU allocated to tasks
AvailableCPU     float64  // CPU still available
AvailableMemory  float64  // Memory still available
AvailableStorage float64  // Storage still available
AvailableGPU     float64  // GPU still available
```

**Purpose:** Track resource allocation to prevent oversubscription

---

#### `assignTaskToWorker(ctx, task, workerID) (*pb.TaskAck, error)`
**Purpose:** Assign task to specific worker (called by queue processor)

**Input:**
- `ctx context.Context`
- `task *pb.Task`
- `workerID string`

**Output:** TaskAck and error

**Working:**
```
1. Lock worker registry
2. Get worker state
3. **Allocate resources:**
   a. worker.AllocatedCPU += task.ReqCpu
   b. worker.AllocatedMemory += task.ReqMemory
   c. worker.AllocatedStorage += task.ReqStorage
   d. worker.AllocatedGPU += task.ReqGpu
   e. worker.AvailableCPU -= task.ReqCpu
   f. worker.AvailableMemory -= task.ReqMemory
   g. worker.AvailableStorage -= task.ReqStorage
   h. worker.AvailableGPU -= task.ReqGpu
4. Unlock registry
5. Store task in TASKS database (status: pending)
6. Connect to worker via gRPC
7. Call worker.AssignTask RPC
8. If success:
   a. Update TASKS (status: running)
   b. Create ASSIGNMENT record
   c. Add to worker.RunningTasks
9. If failure:
   a. **Release allocated resources** (reverse step 3)
   b. Update TASKS (status: failed)
   c. Return error
10. Return acknowledgment
```

**Resource Management:** Ensures workers don't get oversubscribed

---

#### `SetMasterInfo(masterID, masterAddress string)`
**Input:**
- `masterID string` - Unique identifier for master
- `masterAddress string` - Network address (IP:PORT)

**Output:** None

**Working:**
- Thread-safe update of master identity
- Used for worker notification and registration

---

#### `LoadWorkersFromDB(ctx) error`
**Input:** `ctx context.Context` - Request context

**Output:** `error` - Success/failure status

**Working:**
```
1. Query all workers from WORKER_REGISTRY collection
2. For each worker document:
   - Create WorkerState struct
   - Populate WorkerInfo (ID, IP, resources)
   - Set LastHeartbeat, IsActive status
   - Initialize empty RunningTasks map
3. Load into in-memory workers map
4. Log count of loaded workers
```

---

#### `ManualRegisterWorker(ctx, workerID, workerIP string) error`
**Input:**
- `ctx context.Context` - Request context
- `workerID string` - Unique worker identifier
- `workerIP string` - Worker address (IP:PORT format)

**Output:** `error` - Registration status

**Working:**
```
1. Check if worker already exists in memory
2. Check database for existing entry
3. Insert worker into WORKER_REGISTRY with:
   - worker_id, worker_ip
   - is_active = false (not connected yet)
   - Zero resource allocations
   - Timestamp
4. Add to in-memory registry with minimal info
5. Wait for worker to connect and send full specs
```

**Authorization Model:** Manual pre-registration required (security feature)

---

#### `ManualRegisterAndNotify(ctx, workerID, workerIP, masterID, masterAddress string) error`
**Input:**
- All fields from `ManualRegisterWorker`
- `masterID string` - Master's identifier
- `masterAddress string` - Master's address

**Output:** `error` - Registration and notification status

**Working:**
```
1. Call ManualRegisterWorker() first
2. Launch async goroutine to notify worker:
   a. Establish gRPC connection to worker (5s timeout)
   b. Call worker's MasterRegister RPC
   c. Send MasterInfo (master ID and address)
   d. Log success/failure (no blocking)
```

---

#### `UnregisterWorker(ctx, workerID string) error`
**Input:**
- `ctx context.Context`
- `workerID string`

**Output:** `error`

**Working:**
```
1. Verify worker exists in registry
2. Delete from MongoDB WORKER_REGISTRY
3. Unregister from TelemetryManager
4. Remove from in-memory map
5. Log unregistration
```

---

#### `RegisterWorker(ctx, *pb.WorkerInfo) (*pb.RegisterAck, error)`
**Purpose:** Handle worker registration RPC (called BY worker)

**Input:** `*pb.WorkerInfo` containing:
- `worker_id string`
- `worker_ip string`
- `total_cpu, total_memory, total_storage, total_gpu float64`

**Output:** `*pb.RegisterAck` with success status

**Working:**
```
1. Check if worker is pre-registered (authorization check)
2. If NOT pre-registered → REJECT with error message
3. If pre-registered:
   a. Update worker info with full specifications
   b. Preserve IP from manual registration if worker IP empty
   c. Set IsActive = true
   d. Update LastHeartbeat timestamp
   e. Update in MongoDB
   f. Register with TelemetryManager (starts dedicated thread)
4. Return success acknowledgment
```

**Security:** Only pre-registered workers can connect (prevents unauthorized workers)

---

#### `SendHeartbeat(ctx, *pb.Heartbeat) (*pb.HeartbeatAck, error)`
**Purpose:** Process periodic heartbeats from workers

**Input:** `*pb.Heartbeat` containing:
- `worker_id string`
- `cpu_usage, memory_usage, gpu_usage float64`
- `running_tasks []*pb.RunningTask`

**Output:** `*pb.HeartbeatAck`

**Working:**
```
1. Verify worker exists in registry
2. Update LastHeartbeat timestamp
3. Set IsActive = true
4. Store latest resource metrics (CPU, Memory, GPU)
5. Count running tasks
6. Update heartbeat in database
7. Forward heartbeat to TelemetryManager for async processing
8. Return acknowledgment
```

**Performance:** Minimal processing in main thread, offloads to telemetry thread

---

#### `ReportTaskCompletion(ctx, *pb.TaskResult) (*pb.Ack, error)`
**Purpose:** Receive task completion from worker and release resources

**Input:** `*pb.TaskResult` containing:
- `task_id, worker_id string`
- `status string` (success/failed/cancelled)
- `logs string`
- `result_location string`

**Output:** `*pb.Ack`

**Working:**
```
1. Log task completion details
2. Get task info from TASKS (to retrieve resource requirements)
3. **Release allocated resources:**
   a. worker.AllocatedCPU -= task.ReqCPU
   b. worker.AllocatedMemory -= task.ReqMemory
   c. worker.AllocatedStorage -= task.ReqStorage
   d. worker.AllocatedGPU -= task.ReqGPU
   e. worker.AvailableCPU += task.ReqCPU
   f. worker.AvailableMemory += task.ReqMemory
   g. worker.AvailableStorage += task.ReqStorage
   h. worker.AvailableGPU += task.ReqGPU
   i. Ensure non-negative (safety check)
4. Update database with released resources
5. Remove from worker's RunningTasks map
6. Update task status in TASKS collection (completed/failed/cancelled)
7. Store result in RESULTS collection with:
   - task_id, worker_id, status
   - logs (full execution logs)
   - completed_at timestamp
8. Return acknowledgment
```

**Resource Management:** Frees resources so queue processor can assign new tasks

---

#### `AssignTask(ctx, *pb.Task) (*pb.TaskAck, error)`
**Purpose:** Assign task to specific worker

**Input:** `*pb.Task` containing:
- `task_id, docker_image, command string`
- `req_cpu, req_memory, req_storage, req_gpu float64`
- `target_worker_id, user_id string`

**Output:** `*pb.TaskAck`

**Working:**
```
1. Validate target worker exists and is active
2. Verify worker has configured IP address
3. Store task in TASKS collection (status: pending)
4. Establish gRPC connection to worker
5. Call worker's AssignTask RPC
6. If successful:
   a. Mark task as running on worker
   b. Create assignment in ASSIGNMENTS collection
   c. Update task status to "running"
   d. Log detailed task information
7. If failed:
   a. Update task status to "failed"
   b. Return error message
```

---

#### `StreamTaskLogsFromWorker(ctx, taskID, userID string, logHandler func(string, bool)) error`
**Purpose:** Stream live logs for a task from worker

**Input:**
- `ctx context.Context`
- `taskID string` - Task identifier
- `userID string` - User requesting logs
- `logHandler func(string, bool)` - Callback for log lines

**Output:** `error`

**Working:**
```
1. Check if task is completed:
   a. Query RESULTS collection
   b. If found, return stored logs via callback
2. If task running:
   a. Find worker assignment from ASSIGNMENTS
   b. Get worker IP from registry
   c. Establish gRPC connection to worker
   d. Call worker's StreamTaskLogs RPC
   e. Stream log chunks via callback
   f. Handle completion status
   g. Update task status when complete
```

---

#### `BroadcastMasterRegistration(masterID, masterAddress string)`
**Purpose:** Notify all workers of master's address

**Input:**
- `masterID string`
- `masterAddress string`

**Output:** None (async)

**Working:**
```
For each pre-registered worker:
  Launch goroutine:
    1. Connect to worker (5s timeout)
    2. Call worker's MasterRegister RPC
    3. Send MasterInfo
    4. Log success/failure
```

**Use Case:** Master restart, worker discovery

---

### 3. CLI Layer (`master/internal/cli/cli.go`)

#### `NewCLI(srv *server.MasterServer) *CLI`
**Input:** MasterServer instance

**Output:** CLI instance

**Working:** Creates CLI with stdin reader and server reference

---

#### `Run()`
**Purpose:** Main interactive command loop

**Input:** User commands from stdin

**Output:** None (interactive)

**Working:**
```
1. Display banner
2. Loop:
   a. Display "master> " prompt
   b. Read user input
   c. Parse command and arguments
   d. Route to appropriate handler:
      - help: Show command list
      - status: Live cluster status (2s refresh)
      - workers: List all workers
      - stats <id>: Live worker stats (2s refresh)
      - register <id> <ip:port>: Register worker
      - unregister <id>: Remove worker
      - task <worker> <image> [flags]: Assign task
      - monitor <task_id>: Stream task logs
      - exit/quit: Shutdown
   e. Handle errors and display results
```

---

#### `showStatus()`
**Purpose:** Live cluster status monitor

**Output:** Terminal display (refreshed every 2s)

**Working:**
```
1. Display header with instructions
2. Start 2-second ticker
3. Listen for keypress (exit signal)
4. On each tick:
   a. Get all workers from server
   b. Count active workers
   c. Count total running tasks
   d. Update display using ANSI codes (move cursor up)
   e. Redraw stats box
5. Exit on any keypress
```

**UI Features:** Live updating, ANSI cursor control, clean exit

---

#### `listWorkers()`
**Purpose:** Display all registered workers

**Output:** Formatted worker list

**Working:**
```
1. Get workers from server
2. For each worker:
   a. Determine status (🟢 Active / 🔴 Inactive)
   b. Display ID, status, IP
   c. Show resources (CPU, Memory, GPU)
   d. Show running task count
```

---

#### `showWorkerStats(workerID string)`
**Purpose:** Live worker statistics monitor

**Input:** `workerID string`

**Output:** Terminal display (refreshed every 2s)

**Working:**
```
1. Verify worker exists
2. Display header
3. Start 2-second ticker
4. On each tick:
   a. Get worker stats from server
   b. Calculate time since last heartbeat
   c. Format status indicator
   d. Update display using ANSI codes
   e. Show: Status, Address, Last Seen
   f. Show: CPU/Memory/GPU usage percentages
   g. Show: Running task count
5. Handle worker disconnection
6. Exit on keypress
```

---

#### `assignTask(parts []string)`
**Purpose:** Parse and assign task to worker

**Input:** Command arguments array

**Output:** Task assignment result

**Working:**
```
1. Parse arguments:
   - parts[1]: worker_id
   - parts[2]: docker_image
2. Parse optional flags:
   - -cpu_cores <float>: CPU allocation (default: 1.0)
   - -mem <float>: Memory in GB (default: 0.5)
   - -storage <float>: Storage in GB (default: 1.0)
   - -gpu_cores <float>: GPU allocation (default: 0.0)
3. Generate unique task_id (timestamp-based)
4. Create pb.Task struct
5. Call server.AssignTask()
6. Display formatted task details
7. Show success/failure message
```

---

#### `monitorTask(taskID, userID string)`
**Purpose:** Stream live task logs to terminal

**Input:**
- `taskID string`
- `userID string`

**Output:** Live log stream in terminal

**Working:**
```
1. Clear screen and show header
2. Create cancellable context
3. Start goroutine listening for keypress (exit)
4. Start goroutine streaming logs:
   a. Call server.StreamTaskLogsFromWorker()
   b. Display each log line
   c. Show completion status
5. Wait for either:
   - User keypress (stop stream)
   - Stream completion (wait for keypress)
6. Display appropriate exit message
```

**UI Features:** ANSI colors, clean layout, responsive exit

---

### 4. Telemetry Layer (`master/internal/telemetry/telemetry_manager.go`)

#### `NewTelemetryManager(inactivityTimeout time.Duration) *TelemetryManager`
**Input:** `inactivityTimeout time.Duration` - Time before marking worker inactive

**Output:** TelemetryManager instance

**Working:**
```
1. Create context for goroutine management
2. Initialize worker data map
3. Initialize worker channel map
4. Set inactivity timeout (default: 30s)
5. Enable quiet mode (suppress verbose logs)
```

**Architecture:** One dedicated goroutine per worker for telemetry processing

---

#### `RegisterWorker(workerID string)`
**Purpose:** Register worker and start dedicated telemetry thread

**Input:** `workerID string`

**Output:** None

**Working:**
```
1. Check if already registered (avoid duplicates)
2. Create buffered channel for worker heartbeats (size: 10)
3. Initialize WorkerTelemetryData with defaults
4. Launch dedicated goroutine for this worker:
   - Processes heartbeats from channel
   - Updates telemetry data
   - Calls update callback
   - Exits when channel closed
5. Store channel in map
```

**Concurrency:** Each worker has isolated telemetry processing thread

---

#### `UnregisterWorker(workerID string)`
**Purpose:** Stop telemetry for a worker

**Input:** `workerID string`

**Output:** None

**Working:**
```
1. Close worker's heartbeat channel (signals goroutine exit)
2. Remove channel from map
3. Remove worker data from map
```

---

#### `ProcessHeartbeat(*pb.Heartbeat) error`
**Purpose:** Receive and route heartbeat to worker's thread

**Input:** `*pb.Heartbeat`

**Output:** `error`

**Working:**
```
1. Find worker's channel
2. If not found: Auto-register worker
3. Send heartbeat to channel (non-blocking)
4. If channel full: Drop heartbeat (log warning)
```

**Performance:** Non-blocking, prevents main thread stalls

---

#### `processWorkerTelemetry(workerID string, heartbeatChan <-chan *pb.Heartbeat)`
**Purpose:** Dedicated goroutine for worker telemetry processing

**Input:**
- `workerID string`
- `heartbeatChan <-chan *pb.Heartbeat`

**Output:** None (long-running goroutine)

**Working:**
```
Loop:
  Select:
    Case heartbeat from channel:
      - Update worker telemetry data
      - Update CPU, Memory, GPU usage
      - Update running tasks list
      - Set LastUpdate timestamp
      - Set IsActive = true
      - Call update callback (if set)
    
    Case channel closed:
      - Log shutdown
      - Exit goroutine
    
    Case context cancelled:
      - Manager shutting down
      - Exit goroutine
```

---

#### `GetWorkerTelemetry(workerID string) (*WorkerTelemetryData, bool)`
**Purpose:** Get latest telemetry for specific worker

**Input:** `workerID string`

**Output:** `*WorkerTelemetryData, bool` (copy, exists flag)

**Working:**
```
1. Acquire read lock
2. Find worker data
3. Create deep copy (prevent race conditions)
4. Return copy and existence flag
```

**Thread Safety:** Returns copy, safe for concurrent access

---

#### `GetAllWorkerTelemetry() map[string]*WorkerTelemetryData`
**Purpose:** Get telemetry for all workers

**Output:** Map of worker ID to telemetry data (copies)

**Working:**
```
1. Acquire read lock
2. Create result map
3. For each worker:
   - Create deep copy of telemetry data
   - Add to result map
4. Return map
```

---

#### `Start()`
**Purpose:** Start telemetry manager and inactivity checker

**Output:** None

**Working:**
```
1. Launch goroutine for checkInactivity()
2. Log startup
```

---

#### `checkInactivity()`
**Purpose:** Periodic worker activity monitoring

**Output:** None (long-running goroutine)

**Working:**
```
1. Create ticker (half of inactivity timeout)
2. Loop:
   - On tick: Call markInactiveWorkers()
   - On context cancel: Exit
```

---

#### `markInactiveWorkers()`
**Purpose:** Mark stale workers as inactive

**Working:**
```
1. Get current timestamp
2. For each worker:
   a. Calculate time since LastUpdate
   b. If > inactivityTimeout:
      - Set IsActive = false
      - Log inactive status
```

**Use Case:** Detect crashed/disconnected workers

---

#### `Shutdown()`
**Purpose:** Graceful shutdown of telemetry manager

**Working:**
```
1. Cancel context (stops all goroutines)
2. Close all worker channels
3. Wait for all goroutines to exit
4. Log completion
```

---

### 5. HTTP/WebSocket Layer (`master/internal/http/telemetry_server.go`)

#### `NewTelemetryServer(port int, telemetryMgr *telemetry.TelemetryManager) *TelemetryServer`
**Input:**
- `port int` - HTTP server port
- `telemetryMgr *telemetry.TelemetryManager`

**Output:** TelemetryServer instance

**Working:**
```
1. Create context for shutdown management
2. Initialize WebSocket client map
3. Set up HTTP routes:
   - /ws/telemetry: All workers telemetry stream
   - /ws/telemetry/{worker_id}: Specific worker stream
   - /health: Health check endpoint
4. Set telemetry update callback
5. Enable quiet mode
```

---

#### `Start() error`
**Purpose:** Start HTTP server with WebSocket support

**Output:** `error`

**Working:**
```
1. Log endpoints
2. Start HTTP server (blocking)
3. Listen for connections
```

---

#### `handleAllWorkersWS(w http.ResponseWriter, r *http.Request)`
**Purpose:** WebSocket handler for all workers telemetry

**Working:**
```
1. Upgrade HTTP connection to WebSocket
2. Create WSClient (worker_id = "")
3. Register client
4. Send initial telemetry snapshot for all workers
5. Start write pump (goroutine for sending data)
6. Start read pump (handle pings/disconnects)
```

---

#### `handleWorkerTelemetryWS(w http.ResponseWriter, r *http.Request)`
**Purpose:** WebSocket handler for specific worker

**Working:**
```
1. Extract worker_id from URL path
2. Upgrade HTTP connection to WebSocket
3. Create WSClient with specific worker_id
4. Register client
5. Send initial telemetry snapshot for worker
6. Start write/read pumps
```

---

#### `onTelemetryUpdate(workerID string, data *WorkerTelemetryData)`
**Purpose:** Callback when telemetry updates

**Input:**
- `workerID string`
- `data *WorkerTelemetryData`

**Output:** None

**Working:**
```
1. Convert telemetry to JSON format
2. For each connected client:
   - If client watches all workers OR specific worker
   - Send telemetry update via client channel
```

**Broadcast Logic:** Selective routing based on client subscription

---

#### `writePump(client *WSClient)`
**Purpose:** Goroutine for writing to WebSocket

**Working:**
```
1. Create ticker for ping messages (54s interval)
2. Loop:
   Select:
     Case message from send channel:
       - Write message to WebSocket
     Case ticker:
       - Send ping message (keepalive)
     Case context cancelled:
       - Exit goroutine
```

**Keepalive:** Prevents connection timeout

---

#### `readPump(client *WSClient)`
**Purpose:** Goroutine for reading from WebSocket

**Working:**
```
1. Set read deadline (60s)
2. Set pong handler (resets deadline)
3. Loop:
   - Read message from WebSocket
   - If error: Log and exit
   - Handle connection close gracefully
```

---

### 6. Database Layer (`master/internal/db/`)

#### `EnsureCollections(ctx, cfg) error` (init.go)
**Purpose:** Initialize MongoDB collections

**Input:** Context and config

**Output:** Error

**Working:**
```
1. Connect to MongoDB with credentials
2. Ping database to verify connection
3. List existing collections
4. For each required collection:
   - USERS
   - WORKER_REGISTRY
   - TASKS
   - ASSIGNMENTS
   - RESULTS
5. Create missing collections
```

---

#### WorkerDB Functions (workers.go)

##### `NewWorkerDB(ctx, cfg) (*WorkerDB, error)`
**Purpose:** Create worker database handler

**Working:**
```
1. Load MongoDB credentials from .env
2. Connect to MongoDB
3. Ping to verify
4. Get WORKER_REGISTRY collection reference
5. Return WorkerDB instance
```

---

##### `RegisterWorker(ctx, workerID, workerIP string) error`
**Purpose:** Insert new worker into database

**Input:**
- `workerID string`
- `workerIP string` (format: "IP:PORT")

**Output:** `error`

**Working:**
```
1. Create WorkerDocument:
   - worker_id, worker_ip
   - total_cpu, total_memory, total_storage, total_gpu = 0.0
   - is_active = false
   - registered_at, updated_at = now
2. Insert into WORKER_REGISTRY collection
```

---

##### `UpdateWorkerInfo(ctx, *pb.WorkerInfo) error`
**Purpose:** Update worker specs when it connects

**Working:**
```
1. Find worker by worker_id
2. Update fields:
   - worker_ip, total_cpu, total_memory, total_storage, total_gpu
   - is_active = true
   - last_heartbeat = now
   - updated_at = now
3. Return error if worker not found
```

---

##### `UpdateHeartbeat(ctx, workerID string, timestamp int64) error`
**Purpose:** Update heartbeat timestamp

**Working:**
```
1. Find worker by worker_id
2. Set last_heartbeat = timestamp
3. Set is_active = true
4. Set updated_at = now
```

---

##### `GetWorker(ctx, workerID string) (*WorkerDocument, error)`
**Purpose:** Retrieve single worker

**Working:**
```
1. Query by worker_id
2. Decode into WorkerDocument
3. Return nil if not found (no error)
```

---

##### `GetAllWorkers(ctx) ([]WorkerDocument, error)`
**Purpose:** Retrieve all workers

**Working:**
```
1. Find all documents in WORKER_REGISTRY
2. Decode into slice
3. Return slice
```

---

#### TaskDB Functions (tasks.go)

##### `CreateTask(ctx, *Task) error`
**Purpose:** Insert new task

**Input:** Task with metadata

**Working:**
```
1. Set created_at = now
2. Set status = "pending"
3. Insert into TASKS collection
```

---

##### `GetTask(ctx, taskID string) (*Task, error)`
**Purpose:** Retrieve single task

**Working:**
```
1. Query by task_id
2. Decode into Task struct
3. Return error if not found
```

---

##### `UpdateTaskStatus(ctx, taskID, status string) error`
**Purpose:** Update task status

**Input:**
- `taskID string`
- `status string` (pending, running, completed, failed)

**Working:**
```
1. Find task by task_id
2. Set status field
3. If status = "running": Set started_at = now
4. If status = "completed" or "failed": Set completed_at = now
```

---

#### AssignmentDB Functions (assignments.go)

##### `CreateAssignment(ctx, *Assignment) error`
**Purpose:** Record task-worker assignment

**Working:**
```
1. Set assigned_at = now
2. Insert into ASSIGNMENTS collection
```

---

##### `GetAssignmentByTaskID(ctx, taskID string) (*Assignment, error)`
**Purpose:** Find which worker has the task

**Working:**
```
1. Query by task_id
2. Return Assignment with worker_id
```

---

##### `GetAssignmentsByWorker(ctx, workerID string) ([]*Assignment, error)`
**Purpose:** Get all tasks assigned to worker

**Working:**
```
1. Query by worker_id
2. Return slice of assignments
```

---

#### ResultDB Functions (results.go)

##### `CreateResult(ctx, *TaskResult) error`
**Purpose:** Store task execution result and logs

**Working:**
```
1. Set completed_at = now
2. Insert into RESULTS collection with:
   - task_id, worker_id, status
   - logs (full container output)
   - timestamp
```

---

##### `GetResult(ctx, taskID string) (*TaskResult, error)`
**Purpose:** Retrieve task result and logs

**Working:**
```
1. Query by task_id
2. Decode into TaskResult
3. Return nil if not found
```

---

### 7. Configuration Layer (`master/internal/config/config.go`)

#### `LoadConfig() *Config`
**Purpose:** Load configuration from environment

**Output:** Config struct

**Working:**
```
1. Try loading .env file from multiple paths:
   - ./.env
   - ../.env
   - ../../.env
2. Read environment variables with defaults:
   - MONGODB_USERNAME (required)
   - MONGODB_PASSWORD (required)
   - MONGODB_HOST (default: localhost:27017)
   - MONGODB_DATABASE (default: cluster_db)
   - GRPC_PORT (default: :50051)
   - HTTP_PORT (default: :8080)
3. Construct MongoDB URI
4. Return Config struct
```

---

### 8. System Layer (`master/internal/system/system.go`)

#### `CollectSystemInfo() (*SystemInfo, error)`
**Purpose:** Gather system metadata

**Output:** SystemInfo struct

**Working:**
```
1. Get system information:
   - OS (runtime.GOOS)
   - Architecture (runtime.GOARCH)
   - CPU cores (runtime.NumCPU())
   - Process ID (os.Getpid())
   - User ID (syscall.Getuid())
   - Group ID (syscall.Getgid())
   - Hostname (os.Hostname())
2. Get network interfaces
3. Extract non-loopback IPv4 addresses
4. Return SystemInfo
```

---

#### `GetMasterAddress() string`
**Purpose:** Select best IP for master

**Output:** IP address string

**Working:**
```
1. Iterate through IP addresses
2. Prefer non-localhost addresses
3. Fallback to first available
4. Default to "localhost"
```

---

### 9. Scheduler Layer (`master/internal/scheduler/`)

#### `Scheduler` Interface (`scheduler.go`)

**Purpose:** Define contract for all scheduling algorithms

**Methods:**
```go
SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string
GetName() string
Reset()
```

**Design Pattern:** Strategy pattern - allows swapping algorithms without changing master server

---

#### `WorkerInfo` struct
**Fields:**
- `WorkerID string`
- `IsActive bool`
- `WorkerIP string`
- `AvailableCPU float64`
- `AvailableMemory float64`
- `AvailableStorage float64`
- `AvailableGPU float64`

**Purpose:** Simplified worker view for scheduler (read-only)

---

#### `RoundRobinScheduler` (`round_robin.go`)

**Purpose:** Simple round-robin task distribution

**Fields:**
- `lastWorkerIndex int` - Index of last selected worker
- `mu sync.Mutex` - Thread-safe access

---

#### `NewRoundRobinScheduler() *RoundRobinScheduler`
**Output:** Initialized scheduler

**Working:**
```
1. Create scheduler
2. Set lastWorkerIndex = -1 (no selection yet)
```

---

#### `SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string`
**Purpose:** Select next worker in round-robin order

**Input:**
- `task *pb.Task` - Task to schedule
- `workers map[string]*WorkerInfo` - Available workers

**Output:** `string` - Selected worker ID (or "" if none suitable)

**Working:**
```
1. Lock mutex (thread-safe)
2. If no workers available: return ""
3. Create sorted list of worker IDs (deterministic ordering):
   - Extract all worker IDs
   - Sort alphabetically (bubble sort)
4. Calculate start index: (lastWorkerIndex + 1) % len(workers)
5. Try each worker in circular order:
   a. Get worker info
   b. Check if worker is suitable:
      - IsActive = true
      - WorkerIP not empty
      - AvailableCPU >= task.ReqCpu
      - AvailableMemory >= task.ReqMemory
      - AvailableStorage >= task.ReqStorage
      - AvailableGPU >= task.ReqGpu
   c. If suitable:
      - Update lastWorkerIndex
      - Log selection
      - Return worker ID
6. If no suitable worker: return ""
7. Unlock mutex
```

**Fairness:** Cycles through workers, checks resource availability

---

#### `isWorkerSuitable(worker *WorkerInfo, task *pb.Task) bool`
**Purpose:** Check if worker can handle task

**Working:**
```
1. Check IsActive
2. Check WorkerIP configured
3. Check each resource:
   - CPU: available >= required
   - Memory: available >= required
   - Storage: available >= required
   - GPU: available >= required
4. Return true only if all checks pass
```

---

#### `Reset()`
**Purpose:** Reset scheduler state (for testing)

**Working:**
```
1. Lock mutex
2. Set lastWorkerIndex = -1
3. Unlock mutex
```

---

#### `GetName() string`
**Output:** "Round-Robin"

**Purpose:** Identify scheduler algorithm (for logging/debugging)

---

#### `sortWorkerIDs(ids []string)`
**Purpose:** Sort worker IDs for deterministic ordering

**Working:**
```
1. Bubble sort implementation
2. Compare adjacent strings
3. Swap if out of order
4. Repeat until sorted
```

**Why Sorting:** Ensures consistent worker order across restarts

---

## Worker Node Architecture

### 1. Main Entry Point (`worker/main.go`)

#### `main()`
**Purpose:** Worker node initialization and startup

**Working:**
```
1. Collect system information
2. Find available port (starting from 50052):
   - Try binding to port
   - If occupied, try next port
   - Continue for up to 100 ports
3. Set worker ID = hostname
4. Display worker details banner:
   - Worker ID
   - Worker Address (IP:PORT)
   - Registration command for master
5. Create cancellable context
6. Initialize telemetry monitor (5s interval)
7. Start telemetry monitoring (goroutine)
8. Create WorkerServer
9. Start gRPC server
10. Set up graceful shutdown handlers
11. Wait for connections
```

**Key Details:**
- Auto-finds available port
- Uses hostname as worker ID
- Displays registration command for convenience

---

### 2. Server Layer (`worker/internal/server/worker_server.go`)

#### `NewWorkerServer(workerID string, monitor *telemetry.Monitor) (*WorkerServer, error)`
**Input:**
- `workerID string`
- `monitor *telemetry.Monitor`

**Output:** WorkerServer instance or error

**Working:**
```
1. Create TaskExecutor (Docker client)
2. Initialize WorkerServer struct:
   - workerID
   - executor
   - monitor
   - masterAddr = "" (not known yet)
   - masterRegistered = false
3. Return server instance
```

---

#### `MasterRegister(ctx, *pb.MasterInfo) (*pb.RegisterAck, error)`
**Purpose:** Accept master registration (called BY master)

**Input:** `*pb.MasterInfo` containing master ID and address

**Output:** RegisterAck

**Working:**
```
1. Log master registration request
2. Store master address
3. Set masterRegistered = true
4. Update monitor with master address
5. Launch goroutine to register with master:
   - Call registerWithMaster()
6. Return success acknowledgment
```

**Flow:** Master notifies worker → Worker registers back with master

---

#### `registerWithMaster()`
**Purpose:** Register worker with master after receiving master address

**Working:**
```
1. Check if master address is set
2. Connect to master via gRPC (10s timeout)
3. Collect worker system resources:
   - CPU cores (runtime.NumCPU())
   - Memory (simplified: 8GB)
   - Storage (simplified: 100GB)
   - GPU (simplified: 0)
4. Call master's RegisterWorker RPC with WorkerInfo
5. Log success or failure
```

**Note:** Production would use actual resource detection libraries

---

#### `AssignTask(ctx, *pb.Task) (*pb.TaskAck, error)`
**Purpose:** Receive and accept task from master

**Input:** `*pb.Task` with task details

**Output:** TaskAck

**Working:**
```
1. Verify master is registered
2. Log comprehensive task details:
   - Task ID, Docker Image, Command
   - Target Worker
   - Resource requirements (CPU, Memory, Storage, GPU)
3. Add task to monitoring
4. Launch executeTask() in goroutine (non-blocking)
5. Return immediate acknowledgment
```

**Design:** Non-blocking acceptance, async execution

---

#### `executeTask(*pb.Task)`
**Purpose:** Execute Docker task and report result

**Input:** Task to execute

**Output:** None (reports via RPC)

**Working:**
```
1. Create fresh context (not tied to RPC timeout)
2. Call executor.ExecuteTask():
   - Pull Docker image
   - Create container with resource limits
   - Start container
   - Collect logs
   - Wait for completion
3. Remove task from monitoring
4. Create TaskResult with:
   - task_id, worker_id
   - status (success/failed)
   - logs (container output)
5. Report result to master via ReportTaskCompletion RPC
6. Log success/failure
```

---

#### `StreamTaskLogs(req *pb.TaskLogRequest, stream pb.MasterWorker_StreamTaskLogsServer) error`
**Purpose:** Stream live logs for a running task

**Input:**
- `req *pb.TaskLogRequest` with task_id, user_id, follow flag
- `stream` for sending log chunks

**Output:** error

**Working:**
```
1. Get container ID for task
2. If not found: Send "not found" error chunk
3. Get container status
4. Call executor.StreamLogs():
   - Returns log channel and error channel
5. Loop:
   Select:
     Case log line:
       - Send LogChunk with content
     Case error:
       - Send error chunk
     Case logs complete:
       - Get final container status
       - Send completion chunk with status
     Case client disconnects:
       - Exit stream
```

---

### 3. Executor Layer (`worker/internal/executor/executor.go`)

#### `NewTaskExecutor() (*TaskExecutor, error)`
**Purpose:** Create Docker client wrapper

**Working:**
```
1. Create Docker client from environment
2. Negotiate API version
3. Initialize container tracking map
4. Return executor instance
```

---

#### `ExecuteTask(ctx, taskID, dockerImage, command string, reqCPU, reqMemory, reqGPU float64) *TaskResult`
**Purpose:** Execute Docker container with resource limits

**Input:**
- `taskID string`
- `dockerImage string`
- `command string` (optional)
- Resource requirements

**Output:** TaskResult with status and logs

**Working:**
```
1. Pull Docker image:
   - Use ImagePull API
   - Wait for pull completion
2. Create container with resource limits:
   - NanoCPUs = reqCPU * 1e9
   - Memory = reqMemory * 1GB (in bytes)
   - Runtime = "nvidia" if GPU > 0
   - Command = ["/bin/sh", "-c", command] if provided
3. Store container ID in map
4. Start container
5. Collect logs in background:
   - Stream stdout and stderr
   - Parse Docker log format (remove 8-byte header)
   - Accumulate in buffer
6. Wait for container completion:
   - Use ContainerWait API
   - Get exit code
7. Set result status:
   - exit code 0 → "success"
   - exit code != 0 → "failed"
8. Cleanup:
   - Stop container (5s timeout)
   - Remove container (force)
   - Delete from container map
9. Return TaskResult
```

**Resource Enforcement:**
- CPU: Docker CFS quotas
- Memory: cgroup limits
- GPU: Requires nvidia-docker runtime

---

#### `pullImage(ctx, imageName string) error`
**Purpose:** Pull Docker image from registry

**Working:**
```
1. Call Docker ImagePull API
2. Read response stream (required for completion)
3. Discard output (silent pull)
4. Return error on failure
```

---

#### `createContainer(ctx, image, command, taskID string, reqCPU, reqMemory, reqGPU float64) (string, error)`
**Purpose:** Create Docker container with specifications

**Working:**
```
1. Create ContainerConfig:
   - Image name
   - Command (if provided)
2. Create HostConfig with resource limits:
   - NanoCPUs for CPU limit
   - Memory for RAM limit
   - Runtime = "nvidia" for GPU
3. Call ContainerCreate API
4. Return container ID
```

---

#### `collectLogs(ctx, containerID string) (string, error)`
**Purpose:** Collect container logs after execution

**Working:**
```
1. Open ContainerLogs stream (follow mode)
2. Create scanner for line-by-line reading
3. For each line:
   - Remove Docker log header (first 8 bytes)
   - Append to buffer
4. Return accumulated logs
```

**Docker Log Format:** Each line prefixed with 8-byte header (stream type + length)

---

#### `StreamLogs(ctx, containerID string) (<-chan string, <-chan error)`
**Purpose:** Stream live logs from container

**Output:** Log channel and error channel

**Working:**
```
1. Create buffered channels (log: 100, error: 1)
2. Launch goroutine:
   a. Open ContainerLogs stream (follow, timestamps)
   b. Create scanner
   c. For each line:
      - Remove header
      - Send to log channel
   d. On completion or error:
      - Send to error channel
      - Close channels
3. Return channels for consumption
```

**Use Case:** Real-time log monitoring via CLI

---

#### `GetContainerStatus(ctx, containerID string) (string, error)`
**Purpose:** Get current container state

**Working:**
```
1. Call ContainerInspect API
2. Check State.Running:
   - true → return "running"
3. Check State.Status:
   - "exited" + ExitCode 0 → "completed"
   - "exited" + ExitCode != 0 → "failed"
4. Return raw status for other states
```

---

#### `GetContainerID(taskID string) (string, bool)`
**Purpose:** Look up container ID for task

**Working:**
```
1. Acquire read lock
2. Look up in containers map
3. Return ID and existence flag
```

---

#### `cleanup(ctx, containerID string)`
**Purpose:** Stop and remove container

**Working:**
```
1. Stop container (5s timeout)
2. Remove container (force flag)
3. Log warnings on failure
```

---

### 4. Telemetry Layer (`worker/internal/telemetry/telemetry.go`)

#### `NewMonitor(workerID string, interval time.Duration) *Monitor`
**Purpose:** Create telemetry monitor

**Input:**
- `workerID string`
- `interval time.Duration` (heartbeat frequency)

**Output:** Monitor instance

**Working:**
```
1. Initialize monitor:
   - workerID
   - masterAddr = "" (unknown initially)
   - interval (e.g., 5s)
   - runningTasks map
   - stopChan channel
```

---

#### `SetMasterAddress(masterAddr string)`
**Purpose:** Update master address after registration

**Working:**
```
1. Acquire write lock
2. Set masterAddr
3. Log address
```

---

#### `Start(ctx)`
**Purpose:** Begin periodic heartbeat transmission

**Working:**
```
1. Create ticker with interval
2. Loop:
   Select:
     Case tick:
       - Call sendHeartbeat()
     Case stop signal:
       - Exit loop
     Case context cancelled:
       - Exit loop
```

---

#### `sendHeartbeat(ctx) error`
**Purpose:** Send heartbeat to master

**Working:**
```
1. Check if master address is set (skip if not)
2. Connect to master via gRPC
3. Collect current resource usage:
   - getResourceUsage() for CPU, Memory, GPU
4. Create running tasks slice (thread-safe)
5. Create Heartbeat message
6. Call master's SendHeartbeat RPC
7. Log metrics (CPU%, Memory%, GPU%, task count)
8. Close connection
```

---

#### `getResourceUsage() (cpuPercent, memoryPercent, gpuPercent float64)`
**Purpose:** Measure actual system resource usage

**Working:**
```
1. CPU Usage:
   - Use gopsutil cpu.Percent(1s)
   - Returns percentage over 1-second sample
2. Memory Usage:
   - Use gopsutil mem.VirtualMemory()
   - Get UsedPercent
3. GPU Usage:
   - Call getGPUUsage()
4. Return percentages
```

---

#### `getGPUUsage() float64`
**Purpose:** Query NVIDIA GPU utilization

**Working:**
```
1. Execute shell command:
   nvidia-smi --query-gpu=utilization.gpu --format=csv,noheader,nounits | head -n 1
2. Parse output as float
3. Return GPU percentage
4. If nvidia-smi not available: Return 0.0
```

**Dependency:** Requires NVIDIA drivers and nvidia-smi utility

---

#### `AddTask(taskID string, cpuAlloc, memAlloc, gpuAlloc float64)`
**Purpose:** Register task in monitoring

**Working:**
```
1. Acquire write lock
2. Add to runningTasks map:
   - task_id
   - cpu_allocated, memory_allocated, gpu_allocated
   - status = "running"
3. Log addition
```

---

#### `RemoveTask(taskID string)`
**Purpose:** Unregister completed task

**Working:**
```
1. Acquire write lock
2. Delete from runningTasks map
3. Log removal
```

---

#### `RegisterWorker(ctx, masterAddr string, *pb.WorkerInfo) error`
**Purpose:** Helper function to register worker with master

**Working:**
```
1. Connect to master
2. Call RegisterWorker RPC
3. Check acknowledgment
4. Return error or success
```

---

#### `ReportTaskResult(ctx, masterAddr string, *pb.TaskResult) error`
**Purpose:** Helper function to report task completion

**Working:**
```
1. Connect to master
2. Call ReportTaskCompletion RPC
3. Check acknowledgment
4. Return error or success
```

---

### 5. System Layer (`worker/internal/system/system.go`)

#### `CollectSystemInfo() (*SystemInfo, error)`
**Purpose:** Gather worker system metadata

**Working:** Same as master (OS, arch, CPU, PID, UID, GID, hostname, IP addresses)

---

#### `FindAvailablePort(startPort int) (int, error)`
**Purpose:** Find open port for worker

**Input:** `startPort int` (e.g., 50052)

**Output:** Available port number

**Working:**
```
1. Loop from startPort to startPort + 100:
   a. Try binding TCP listener on port
   b. If success:
      - Close listener
      - Return port
   c. If failure:
      - Try next port
2. Return error if no port found
```

**Use Case:** Multiple workers on same machine

---

## Database Layer Architecture

### Collections

#### 1. USERS
**Purpose:** User authentication and authorization (future)

**Schema:**
```
{
  user_id: string (primary key)
  username: string
  email: string
  password_hash: string
  role: string (admin, user)
  created_at: timestamp
}
```

---

#### 2. WORKER_REGISTRY
**Purpose:** Registered worker nodes

**Schema:**
```
{
  worker_id: string (primary key)
  worker_ip: string (format: "IP:PORT")
  total_cpu: float64
  total_memory: float64
  total_storage: float64
  total_gpu: float64
  is_active: boolean
  last_heartbeat: int64 (unix timestamp)
  registered_at: timestamp
  updated_at: timestamp
}
```

**Indexes:** worker_id (unique), is_active

---

#### 3. TASKS
**Purpose:** Task definitions and metadata

**Schema:**
```
{
  task_id: string (primary key)
  user_id: string (foreign key)
  docker_image: string
  command: string
  req_cpu: float64
  req_memory: float64
  req_storage: float64
  req_gpu: float64
  status: string (pending, running, completed, failed)
  created_at: timestamp
  started_at: timestamp
  completed_at: timestamp
}
```

**Indexes:** task_id (unique), user_id, status

---

#### 4. ASSIGNMENTS
**Purpose:** Task-to-worker mappings

**Schema:**
```
{
  ass_id: string (primary key)
  task_id: string (foreign key, unique)
  worker_id: string (foreign key)
  assigned_at: timestamp
}
```

**Indexes:** task_id (unique), worker_id

---

#### 5. RESULTS
**Purpose:** Task execution results and logs

**Schema:**
```
{
  task_id: string (primary key)
  worker_id: string (foreign key)
  status: string (success, failed)
  logs: string (full container output)
  result_location: string (future: S3/storage path)
  completed_at: timestamp
}
```

**Indexes:** task_id (unique), worker_id, status

---

## Communication Protocol

### gRPC Services

#### MasterWorker Service (Bidirectional)

**Worker → Master RPCs:**
1. **RegisterWorker**
   - Input: WorkerInfo (ID, IP, resources)
   - Output: RegisterAck
   - Purpose: Worker announces itself
   - Security: Must be pre-registered

2. **SendHeartbeat**
   - Input: Heartbeat (resource usage, running tasks)
   - Output: HeartbeatAck
   - Purpose: Periodic status update
   - Frequency: Every 5 seconds

3. **ReportTaskCompletion**
   - Input: TaskResult (status, logs)
   - Output: Ack
   - Purpose: Notify task completion

**Master → Worker RPCs:**
1. **MasterRegister**
   - Input: MasterInfo (ID, address)
   - Output: RegisterAck
   - Purpose: Master announces itself to worker
   - Flow: Master notifies worker → Worker registers back

2. **AssignTask**
   - Input: Task (image, resources, command)
   - Output: TaskAck
   - Purpose: Assign task for execution
   - Response: Immediate acknowledgment (async execution)

3. **CancelTask**
   - Input: TaskID
   - Output: TaskAck
   - Purpose: Cancel running task
   - Status: Not implemented

4. **StreamTaskLogs**
   - Input: TaskLogRequest (task_id, user_id, follow)
   - Output: Stream of LogChunk
   - Purpose: Live log streaming
   - Protocol: Server-side streaming

---

### Message Types

#### WorkerInfo
```protobuf
{
  worker_id: string
  worker_ip: string
  total_cpu: float64
  total_memory: float64
  total_storage: float64
  total_gpu: float64
}
```

#### Heartbeat
```protobuf
{
  worker_id: string
  cpu_usage: float64
  memory_usage: float64
  storage_usage: float64
  gpu_usage: float64
  running_tasks: [RunningTask]
}
```

#### Task
```protobuf
{
  task_id: string
  docker_image: string
  command: string
  req_cpu: float64
  req_memory: float64
  req_storage: float64
  req_gpu: float64
  target_worker_id: string
  user_id: string
}
```

#### TaskResult
```protobuf
{
  task_id: string
  worker_id: string
  status: string
  logs: string
  result_location: string
}
```

---

## Data Flow Diagrams

### 1. Worker Registration Flow

```
1. Admin runs: master> register worker-1 192.168.1.100:50052
   ↓
2. Master.ManualRegisterAndNotify()
   - Inserts worker into WORKER_REGISTRY (is_active=false)
   - Launches goroutine to notify worker
   ↓
3. Master → Worker: MasterRegister RPC
   - Sends master ID and address
   ↓
4. Worker.MasterRegister()
   - Stores master address
   - Sets masterRegistered = true
   - Launches registerWithMaster()
   ↓
5. Worker → Master: RegisterWorker RPC
   - Sends full system specs
   ↓
6. Master.RegisterWorker()
   - Verifies worker is pre-registered (authorization)
   - Updates worker info in database
   - Sets is_active = true
   - Registers with TelemetryManager (starts thread)
   ↓
7. Worker.Monitor.Start()
   - Begins sending heartbeats every 5s
```

---

### 2. Task Execution Flow

```
1. User runs: master> task worker-1 ubuntu:latest
   ↓
2. CLI.assignTask()
   - Parses command and resource flags
   - Generates task_id
   - Creates Task struct
   ↓
3. MasterServer.AssignTask()
   - Validates worker exists and is active
   - Stores task in TASKS collection (status: pending)
   - Connects to worker via gRPC
   ↓
4. Master → Worker: AssignTask RPC
   ↓
5. WorkerServer.AssignTask()
   - Logs task details
   - Adds task to Monitor
   - Returns immediate acknowledgment
   - Launches executeTask() goroutine
   ↓
6. executeTask()
   a. Executor.ExecuteTask()
      - Pulls Docker image
      - Creates container with resource limits
      - Starts container
      - Collects logs
      - Waits for completion
   b. Creates TaskResult
   c. Removes task from Monitor
   ↓
7. Worker → Master: ReportTaskCompletion RPC
   - Sends status and logs
   ↓
8. Master.ReportTaskCompletion()
   - Updates task status in TASKS
   - Removes from worker's RunningTasks
   - Stores result in RESULTS with logs
```

---

### 3. Heartbeat Flow

```
1. Worker.Monitor ticker (every 5s)
   ↓
2. Monitor.sendHeartbeat()
   - Collects CPU usage (gopsutil)
   - Collects Memory usage (gopsutil)
   - Collects GPU usage (nvidia-smi)
   - Creates RunningTask list
   ↓
3. Worker → Master: SendHeartbeat RPC
   ↓
4. Master.SendHeartbeat()
   - Updates LastHeartbeat timestamp
   - Sets IsActive = true
   - Stores latest metrics (CPU, Memory, GPU, task count)
   - Updates database
   - Forwards to TelemetryManager
   ↓
5. TelemetryManager.ProcessHeartbeat()
   - Routes to worker's dedicated channel
   ↓
6. processWorkerTelemetry() goroutine
   - Updates WorkerTelemetryData
   - Calls onUpdate callback
   ↓
7. TelemetryServer.onTelemetryUpdate()
   - Broadcasts to WebSocket clients
   - JSON format over WebSocket
   ↓
8. Client receives real-time telemetry
```

---

### 4. Log Streaming Flow

```
1. User runs: master> monitor task-123
   ↓
2. CLI.monitorTask()
   - Clears screen, shows header
   - Starts keypress listener
   ↓
3. MasterServer.StreamTaskLogsFromWorker()
   - Checks if task is completed:
     a. If completed: Return stored logs from RESULTS
     b. If running: Continue below
   - Finds worker from ASSIGNMENTS
   - Connects to worker via gRPC
   ↓
4. Master → Worker: StreamTaskLogs RPC (server streaming)
   ↓
5. WorkerServer.StreamTaskLogs()
   - Gets container ID for task
   - Gets container status
   ↓
6. Executor.StreamLogs()
   - Opens Docker log stream (follow mode)
   - Returns log and error channels
   ↓
7. Loop: Read from channels
   - Log line → Send LogChunk to stream
   - Error → Send error chunk
   - Complete → Send completion chunk with status
   ↓
8. Master receives stream
   - Passes each chunk to CLI callback
   ↓
9. CLI displays logs in real-time
   - User can press any key to exit
```

---

### 5. Telemetry WebSocket Flow

```
1. Client connects: ws://localhost:8080/ws/telemetry
   ↓
2. TelemetryServer.handleAllWorkersWS()
   - Upgrades HTTP to WebSocket
   - Creates WSClient (worker_id = "")
   - Registers client
   ↓
3. Send initial snapshot
   - GetAllWorkerTelemetry()
   - Convert to JSON
   - Send to client
   ↓
4. Start write and read pumps (goroutines)
   ↓
5. On each heartbeat:
   TelemetryManager → onTelemetryUpdate callback
   ↓
6. TelemetryServer.onTelemetryUpdate()
   - For each connected client:
     - If client watches all workers OR specific worker
     - Send telemetry to client's send channel
   ↓
7. writePump() goroutine
   - Reads from send channel
   - Writes to WebSocket
   - Sends pings every 54s (keepalive)
   ↓
8. Client receives real-time telemetry updates
```

---

## Key Design Patterns

### 1. **Thread-per-Worker Telemetry**
- Each worker gets dedicated goroutine for telemetry processing
- Prevents slow telemetry from blocking heartbeat reception
- Buffered channels for non-blocking sends

### 2. **Authorization-First Worker Registration**
- Workers must be manually pre-registered by admin
- Prevents unauthorized workers from joining cluster
- Security measure for production environments

### 3. **Bidirectional Master-Worker Relationship**
- Master can discover workers (MasterRegister RPC)
- Workers can connect to master (RegisterWorker RPC)
- Supports dynamic network topologies

### 4. **Async Task Execution**
- AssignTask RPC returns immediately
- Actual execution in background goroutine
- Prevents RPC timeout for long-running tasks

### 5. **Persistent State with In-Memory Cache**
- MongoDB for durability
- In-memory maps for fast access
- Database writes are async (don't block operations)

### 6. **WebSocket Selective Broadcast**
- Clients subscribe to all workers or specific worker
- Updates only sent to interested clients
- Efficient network usage

### 7. **Resource-Constrained Execution**
- Docker cgroup limits for CPU and memory
- Prevents task resource overcommit
- Fair resource sharing

### 8. **Graceful Degradation**
- System continues if database unavailable
- Telemetry failures don't stop task execution
- Optional components (HTTP server)

---

## Performance Characteristics

### Latency
- **Heartbeat interval:** 5 seconds
- **Inactivity timeout:** 30 seconds
- **Task assignment:** < 100ms (network bound)
- **Log streaming:** Real-time (sub-second)
- **WebSocket updates:** < 50ms after heartbeat

### Scalability
- **Workers per master:** Limited by MongoDB, gRPC connections
- **Tasks per worker:** Limited by Docker and system resources
- **Concurrent log streams:** Limited by goroutine count
- **WebSocket clients:** Limited by memory and network

### Resource Usage
- **Master:** Low CPU, moderate memory (in-memory registry)
- **Worker:** Variable (depends on tasks)
- **Database:** Proportional to tasks and workers
- **Network:** ~1KB per heartbeat (5s interval)

---

## Security Considerations

### Authentication
- **Worker registration:** Pre-authorization required
- **User authentication:** Not implemented (future: USERS table)
- **gRPC:** Insecure credentials (production: use TLS)

### Authorization
- **Task assignment:** Admin-only via CLI
- **Log access:** User ID based (not enforced)
- **Worker operations:** Admin-only

### Network Security
- **gRPC:** Plain TCP (production: enable TLS)
- **WebSocket:** CORS wide-open (production: restrict origins)
- **MongoDB:** Username/password authentication

---

## Future Enhancements

### Mentioned in Code
1. **Task Cancellation:** CancelTask RPC not implemented
2. **User Authentication:** USERS table exists but unused
3. **GPU Support:** Simplified (needs nvidia-docker integration)
4. **Result Storage:** S3/shared volume integration
5. **AI Scheduler:** Agentic scheduler submodule (separate)

### Potential Improvements
1. **Load Balancing:** Automatic worker selection
2. **Task Queuing:** Priority-based scheduling
3. **Resource Prediction:** ML-based resource estimation
4. **Multi-Master:** High availability
5. **Worker Pools:** Dedicated worker groups per user
6. **Task Dependencies:** DAG-based execution
7. **Container Registry:** Private registry support
8. **Metrics Export:** Prometheus/Grafana integration

---

## Error Handling Strategies

### Master Node
- **Database errors:** Log warning, continue with in-memory state
- **Worker connection failures:** Mark worker inactive
- **Task assignment failures:** Update task status to "failed"
- **Telemetry errors:** Drop heartbeat (worker will retry)

### Worker Node
- **Image pull failures:** Return failed TaskResult
- **Container creation errors:** Return failed TaskResult
- **Master connection failures:** Retry on next heartbeat
- **Log streaming errors:** Send error chunk, close stream

### Database
- **Connection failures:** Retry with exponential backoff
- **Query errors:** Return error to caller
- **Document not found:** Return nil (not error)

---

## Monitoring and Observability

### Logs
- **Master:** Task assignments, worker registrations, errors
- **Worker:** Task executions, resource usage, errors
- **Structured format:** Timestamp, component, message

### Metrics (Available via Telemetry)
- Worker count (total, active)
- Task count (per worker)
- Resource usage (CPU, Memory, GPU %)
- Heartbeat status
- WebSocket client count

### Health Checks
- **Master:** `/health` HTTP endpoint
- **Worker:** Heartbeat indicates health
- **Database:** Connection ping

---

## Deployment Considerations

### Prerequisites
- **Go 1.22+**
- **Docker** with daemon running
- **MongoDB** 4.0+
- **Protocol Buffers** compiler (protoc)
- **NVIDIA drivers** (for GPU tasks)

### Environment Variables
```
MONGODB_USERNAME
MONGODB_PASSWORD
MONGODB_HOST (default: localhost:27017)
MONGODB_DATABASE (default: cluster_db)
GRPC_PORT (default: :50051)
HTTP_PORT (default: :8080)
```

### Network Ports
- **Master gRPC:** 50051
- **Master HTTP/WebSocket:** 8080
- **Worker gRPC:** 50052+ (auto-increments)
- **MongoDB:** 27017

### Running
```bash
# Start MongoDB
cd database && docker-compose up -d

# Start Master
cd master && go run main.go

# Start Worker (repeat for multiple workers)
cd worker && go run main.go

# Use CLI
master> help
master> register worker-1 192.168.1.100:50052
master> task worker-1 ubuntu:latest
```

---

## Conclusion

This architecture provides a robust, scalable foundation for distributed Docker task execution with:
- ✅ **Strong separation of concerns** (CLI, Server, Executor, DB, Telemetry)
- ✅ **Concurrent processing** (goroutines for async operations)
- ✅ **Real-time monitoring** (WebSocket telemetry streaming)
- ✅ **Persistent state** (MongoDB for durability)
- ✅ **Resource enforcement** (Docker cgroup limits)
- ✅ **Security measures** (pre-authorization, authentication ready)
- ✅ **Operational visibility** (CLI, logs, telemetry)

The function-level design demonstrates production-ready patterns while maintaining simplicity and extensibility for future enhancements.

---

**End of Function-Level Architecture Documentation**
