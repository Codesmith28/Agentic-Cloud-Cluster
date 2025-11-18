# CloudAI API Documentation

Complete API reference for all HTTP REST and WebSocket endpoints.

## Table of Contents
1. [Base URL](#base-url)
2. [Health & Status Endpoints](#health--status-endpoints)
3. [Worker Endpoints](#worker-endpoints)
4. [Task Endpoints](#task-endpoints)
5. [File Management Endpoints](#file-management-endpoints)
6. [WebSocket Endpoints](#websocket-endpoints)
7. [Data Models](#data-models)
8. [Error Handling](#error-handling)

---

## Base URL

```
HTTP API: http://localhost:8080
WebSocket: ws://localhost:8080
```

**Default Port:** 8080 (configurable via `HTTP_PORT` environment variable)

---

## Health & Status Endpoints

### GET /health
Check if the API server is running.

**Handler Location:** `master/internal/http/telemetry_server.go` → `handleHealth()`

**Response:**
```json
{
  "status": "ok"
}
```

**Example:**
```bash
curl http://localhost:8080/health
```

---

### GET /telemetry
Get telemetry data for all workers.

**Handler Location:** `master/internal/http/telemetry_server.go` → `handleTelemetryREST()`

**Response:**
```json
{
  "workers": {
    "worker-1": {
      "worker_id": "worker-1",
      "is_active": true,
      "cpu_usage": 45.2,
      "memory_usage": 60.5,
      "gpu_usage": 20.0,
      "running_tasks": [],
      "last_update": 1700000000
    }
  }
}
```

---

### GET /telemetry/{worker_id}
Get telemetry data for a specific worker.

**Handler Location:** `master/internal/http/telemetry_server.go` → `handleWorkerTelemetryREST()`

**URL Parameters:**
- `worker_id` (string): Worker identifier

**Response:**
```json
{
  "worker_id": "worker-1",
  "is_active": true,
  "cpu_usage": 45.2,
  "memory_usage": 60.5,
  "gpu_usage": 20.0,
  "running_tasks": [],
  "last_update": 1700000000
}
```

---

### GET /workers
Legacy endpoint - alias for `/api/workers`.

**Handler Location:** `master/internal/http/telemetry_server.go` → `handleWorkersREST()`

---

## Worker Endpoints

### GET /api/workers
List all registered workers with their resource information.

**Handler Location:** `master/internal/http/worker_handler.go` → `HandleListWorkers()`

**Logic:**
1. Fetches workers from database (`WorkerDB.GetAllWorkers()`)
2. Merges with real-time telemetry data (`TelemetryManager.GetAllWorkerTelemetry()`)
3. Calculates active status based on last_heartbeat (active if < 30 seconds old)
4. Returns combined data with resources and usage metrics

**Response:**
```json
{
  "workers": [
    {
      "worker_id": "worker-1",
      "address": "192.168.1.100:50052",
      "worker_ip": "192.168.1.100:50052",
      "is_active": true,
      "total_resources": {
        "cpu": 8.0,
        "memory": 16.0,
        "storage": 100.0,
        "gpu": 2.0
      },
      "allocated_resources": {
        "cpu": 2.0,
        "memory": 4.0,
        "storage": 10.0,
        "gpu": 0.0
      },
      "available_resources": {
        "cpu": 6.0,
        "memory": 12.0,
        "storage": 90.0,
        "gpu": 2.0
      },
      "last_heartbeat": 1700000000,
      "registered_at": 1699000000,
      "cpu_usage": 45.2,
      "memory_usage": 60.5,
      "gpu_usage": 20.0,
      "running_tasks_count": 2
    }
  ]
}
```

**Active Status Logic:**
```go
currentTime := time.Now().Unix()
isActive := (currentTime - dbWorker.LastHeartbeat) < 30 // Active if heartbeat within 30 seconds
```

---

### POST /api/workers
Register a new worker manually.

**Handler Location:** `master/internal/http/worker_handler.go` → `HandleRegisterWorker()`

**Logic:**
1. Validates worker_id and worker_ip/worker_port
2. Gets master info (master_id, master_address)
3. Calls `MasterServer.ManualRegisterAndNotify()` which:
   - Registers worker in database with minimal info (resources = 0)
   - Adds to in-memory worker map
   - Sends gRPC `MasterRegister` to worker to notify it
4. Worker receives notification and connects back via gRPC `RegisterWorker`
5. Worker sends full resource specs (CPU, memory, storage, GPU)
6. Master updates worker with actual resources

**Request Body:**
```json
{
  "worker_id": "worker-1",
  "worker_ip": "192.168.1.100:50052"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Worker registered successfully. Master is notifying worker to connect and send resource information.",
  "worker_id": "worker-1",
  "worker": {
    "worker_id": "worker-1",
    "worker_ip": "192.168.1.100:50052",
    "is_active": false
  }
}
```

**Related Functions:**
- `master/internal/server/master_server.go` → `ManualRegisterAndNotify()`
- `master/internal/server/master_server.go` → `RegisterWorker()` (gRPC handler)
- `master/internal/db/workers.go` → `RegisterWorker()`

---

### GET /api/workers/{worker_id}
Get detailed information for a specific worker.

**Handler Location:** `master/internal/http/worker_handler.go` → `HandleGetWorker()`

**URL Parameters:**
- `worker_id` (string): Worker identifier

**Response:**
```json
{
  "worker_id": "worker-1",
  "is_active": true,
  "cpu_usage": 45.2,
  "memory_usage": 60.5,
  "gpu_usage": 20.0,
  "running_tasks": [
    {
      "task_id": "task-123",
      "cpu_allocated": 1.0,
      "memory_allocated": 2.0,
      "gpu_allocated": 0.0,
      "status": "running"
    }
  ],
  "last_update": 1700000000,
  "worker_info": {
    "worker_id": "worker-1",
    "worker_ip": "192.168.1.100:50052",
    "total_cpu": 8.0,
    "total_memory": 16.0,
    "total_storage": 100.0,
    "total_gpu": 2.0,
    "registered_at": 1699000000,
    "last_heartbeat": 1700000000
  }
}
```

---

### GET /api/workers/{worker_id}/tasks
Get all tasks assigned to a specific worker.

**Handler Location:** `master/internal/http/worker_handler.go` → `HandleGetWorkerTasks()`

**Logic:**
1. Queries `AssignmentDB.GetAssignmentsByWorker()`
2. Returns list of task_id and assigned_at timestamp

**Response:**
```json
{
  "worker_id": "worker-1",
  "tasks": [
    {
      "task_id": "task-123",
      "assigned_at": 1700000000
    }
  ],
  "count": 1
}
```

---

## Task Endpoints

### GET /api/tasks
List all tasks (with optional status filter).

**Handler Location:** `master/internal/http/task_handler.go` → `HandleListTasks()`

**Query Parameters:**
- `status` (optional): Filter by status (pending, running, completed, failed, cancelled)

**Logic:**
1. If status provided: `TaskDB.GetTasksByStatus(status)`
2. If no status: `TaskDB.GetAllTasks()`
3. Returns wrapped response with `tasks` array

**Response:**
```json
{
  "tasks": [
    {
      "task_id": "task-123",
      "docker_image": "nginx:latest",
      "status": "running",
      "cpu_required": 1.0,
      "memory_required": 2.0,
      "storage_required": 5.0,
      "gpu_required": 0.0,
      "user_id": "admin",
      "tag": "web-server",
      "k_value": 1.8,
      "submitted_at": 1700000000,
      "assigned_at": 1700000010,
      "started_at": 1700000020,
      "completed_at": 0,
      "assigned_worker": "worker-1"
    }
  ]
}
```

**Task Status Flow:**
```
pending → running → completed/failed
              ↓
           cancelled
```

---

### POST /api/tasks
Submit a new task to the scheduler.

**Handler Location:** `master/internal/http/task_handler.go` → `HandleCreateTask()`

**Logic:**
1. Parses and validates request (supports both string and number types for flexibility)
2. Validates k_value range (1.5 - 2.5)
3. Generates task_id: `task-{timestamp}`
4. Calls `MasterServer.SubmitTask()` which:
   - Stores task in database with status="pending"
   - Adds to task queue
   - Scheduler picks up task asynchronously (runs every 5s)
5. Scheduler finds best worker and assigns task

**Request Body:**
```json
{
  "docker_image": "nginx:latest",
  "command": "",
  "cpu_required": 1.0,
  "memory_required": 2.0,
  "storage_required": 5.0,
  "gpu_required": 0.0,
  "user_id": "admin",
  "tag": "web-server",
  "k_value": 1.8
}
```

**Field Details:**
- `docker_image` (required): Docker image to run
- `command` (optional): Command to override container CMD
- `cpu_required` (required): CPU cores needed
- `memory_required` (required): Memory in GB
- `storage_required` (optional): Storage in GB (default: 1.0)
- `gpu_required` (optional): GPU cores (default: 0.0)
- `user_id` (optional): User identifier (default: "guest")
- `tag` (optional): Task category (web-server, data-processing, ml-training, batch-job, api-service, background-task)
- `k_value` (optional): Priority weight 1.5-2.5 (default: 2.0)

**Response:**
```json
{
  "success": true,
  "message": "Task submitted successfully",
  "task_id": "task-1700000000",
  "task": {
    "task_id": "task-1700000000",
    "docker_image": "nginx:latest",
    "status": "pending",
    "tag": "web-server",
    "k_value": 1.8
  }
}
```

**Related Functions:**
- `master/internal/server/master_server.go` → `SubmitTask()`
- `master/internal/server/master_server.go` → `StartQueueProcessor()`
- `master/internal/server/scheduler.go` → `FindBestWorker()`
- `master/internal/db/tasks.go` → `InsertTask()`

---

### GET /api/tasks/{task_id}
Get detailed information for a specific task.

**Handler Location:** `master/internal/http/task_handler.go` → `HandleGetTask()`

**URL Parameters:**
- `task_id` (string): Task identifier

**Response:**
```json
{
  "task_id": "task-123",
  "docker_image": "nginx:latest",
  "command": "",
  "status": "completed",
  "cpu_required": 1.0,
  "memory_required": 2.0,
  "storage_required": 5.0,
  "gpu_required": 0.0,
  "user_id": "admin",
  "tag": "web-server",
  "k_value": 1.8,
  "submitted_at": 1700000000,
  "assigned_at": 1700000010,
  "started_at": 1700000020,
  "completed_at": 1700000100,
  "assigned_worker": "worker-1"
}
```

---

### DELETE /api/tasks/{task_id}
Cancel a running or pending task.

**Handler Location:** `master/internal/http/task_handler.go` → `HandleCancelTask()`

**Logic:**
1. Calls `MasterServer.CancelTask()` which:
   - Updates task status to "cancelled" in database
   - If task is running on worker, sends gRPC `CancelTask` to worker
   - Worker stops the Docker container
   - Releases allocated resources
2. Returns acknowledgment

**Response:**
```json
{
  "success": true,
  "message": "Task task-123 cancelled successfully"
}
```

**Related Functions:**
- `master/internal/server/master_server.go` → `CancelTask()`
- `master/internal/db/tasks.go` → `UpdateTaskStatus()`

---

### GET /api/tasks/{task_id}/logs
Get logs for a completed task.

**Handler Location:** `master/internal/http/task_handler.go` → `HandleGetTaskLogs()`

**Logic:**
1. Queries `ResultDB.GetResult(task_id)`
2. Returns logs from result document

**Response:**
```json
{
  "task_id": "task-123",
  "logs": "Container started\nRunning nginx\nContainer stopped\n",
  "status": "completed",
  "completed_at": 1700000100
}
```

---

### PUT /api/tasks/{task_id}/retry
Retry a failed task.

**Handler Location:** `master/internal/http/task_handler.go` → `HandleRetryTask()`

**Logic:**
1. Gets original task from database
2. Creates new task with same parameters but new task_id
3. Submits to queue for scheduling

**Response:**
```json
{
  "success": true,
  "message": "Task retry initiated",
  "new_task_id": "task-1700000200",
  "original_task_id": "task-123"
}
```

---

## File Management Endpoints

### GET /api/files
List all files for a user.

**Handler Location:** `master/internal/http/file_handler.go` → `HandleListFiles()`

**Query Parameters:**
- `user_id` (required): User identifier
- `requesting_user` (optional): User making the request (for access control)

**Response:**
```json
{
  "user_id": "admin",
  "files": [
    {
      "file_id": "file-123",
      "task_id": "task-123",
      "file_name": "output.txt",
      "file_path": "results/output.txt",
      "file_size": 1024,
      "uploaded_at": 1700000000
    }
  ],
  "count": 1
}
```

---

### GET /api/files/{task_id}
Get all files for a specific task.

**Handler Location:** `master/internal/http/file_handler.go` → `HandleGetTaskFiles()`

**Query Parameters:**
- `user_id` (required): User who owns the task

**Response:**
```json
{
  "task_id": "task-123",
  "user_id": "admin",
  "files": [
    {
      "file_id": "file-123",
      "file_name": "output.txt",
      "file_path": "results/output.txt",
      "file_size": 1024,
      "content_type": "text/plain",
      "uploaded_at": 1700000000
    }
  ]
}
```

---

### GET /api/files/{task_id}/download/{file_path}
Download a specific file from a task.

**Handler Location:** `master/internal/http/file_handler.go` → `HandleDownloadFile()`

**Query Parameters:**
- `user_id` (required): User who owns the task

**Response:** Binary file stream with appropriate Content-Type header

---

### DELETE /api/files/{task_id}
Delete all files associated with a task.

**Handler Location:** `master/internal/http/file_handler.go` → `HandleDeleteTaskFiles()`

**Query Parameters:**
- `user_id` (required): User who owns the task

**Response:**
```json
{
  "success": true,
  "message": "All files for task task-123 deleted successfully"
}
```

---

## WebSocket Endpoints

### WS /ws/telemetry
Real-time telemetry stream for all workers.

**Handler Location:** `master/internal/http/telemetry_server.go` → `handleAllWorkersWS()`

**Connection:** `ws://localhost:8080/ws/telemetry`

**Update Frequency:** Every 5 seconds

**Message Format:**
```json
{
  "workers": {
    "worker-1": {
      "worker_id": "worker-1",
      "is_active": true,
      "cpu_usage": 45.2,
      "memory_usage": 60.5,
      "gpu_usage": 20.0,
      "total_resources": {
        "cpu": 8.0,
        "memory": 16.0,
        "storage": 100.0,
        "gpu": 2.0
      },
      "allocated_resources": {
        "cpu": 2.0,
        "memory": 4.0,
        "storage": 10.0,
        "gpu": 0.0
      },
      "available_resources": {
        "cpu": 6.0,
        "memory": 12.0,
        "storage": 90.0,
        "gpu": 2.0
      },
      "running_tasks": [],
      "last_update": 1700000000
    }
  }
}
```

**Logic:**
1. Client connects to WebSocket
2. Server adds client to broadcast list
3. `TelemetryManager` calls `onTelemetryUpdate()` callback every 5s
4. Server broadcasts latest telemetry to all connected clients
5. Clients receive real-time updates without polling

**Frontend Usage:**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  // Update UI with real-time worker data
};
```

---

### WS /ws/telemetry/{worker_id}
Real-time telemetry stream for a specific worker.

**Handler Location:** `master/internal/http/telemetry_server.go` → `handleWorkerTelemetryWS()`

**Connection:** `ws://localhost:8080/ws/telemetry/worker-1`

**Message Format:** Same as `/ws/telemetry` but only for specified worker

---

### WS /ws/tasks/{task_id}/logs
Real-time log streaming for a task.

**Handler Location:** `master/internal/http/task_handler.go` → `HandleTaskLogsStream()`

**Connection:** `ws://localhost:8080/ws/tasks/task-123/logs`

**Logic:**
1. Client connects and provides task_id
2. Server calls `MasterServer.GetUserIDForTask()` to get user_id
3. Server calls `MasterServer.StreamTaskLogsUnified()` which:
   - Connects to worker via gRPC `StreamLogs`
   - Worker streams Docker container logs
   - Master forwards logs to WebSocket client
4. Logs stream in real-time until task completes

**Message Types:**

**Connected:**
```json
{
  "type": "connected",
  "task_id": "task-123",
  "user_id": "admin",
  "message": "Connected to task log stream"
}
```

**Log Line:**
```json
{
  "type": "log",
  "line": "Container started successfully",
  "task_id": "task-123"
}
```

**Task Complete:**
```json
{
  "type": "complete",
  "task_id": "task-123",
  "status": "completed",
  "message": "Task completed with status: completed"
}
```

**Error:**
```json
{
  "type": "error",
  "task_id": "task-123",
  "error": "Failed to stream logs: connection lost"
}
```

**Frontend Usage:**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/tasks/task-123/logs');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'log') {
    appendLog(data.line);
  } else if (data.type === 'complete') {
    showCompletionMessage(data.status);
  }
};
```

**Related Functions:**
- `master/internal/server/log_streaming_helper.go` → `StreamTaskLogsUnified()`
- `master/internal/cli/cli.go` → `monitorTask()` (CLI equivalent)

---

## Data Models

### Worker Document
```go
type WorkerDocument struct {
    WorkerID         string    // Unique worker identifier
    WorkerIP         string    // Address (ip:port)
    TotalCPU         float64   // Total CPU cores
    TotalMemory      float64   // Total memory in GB
    TotalStorage     float64   // Total storage in GB
    TotalGPU         float64   // Total GPU cores
    AllocatedCPU     float64   // Currently allocated CPU
    AllocatedMemory  float64   // Currently allocated memory
    AllocatedStorage float64   // Currently allocated storage
    AllocatedGPU     float64   // Currently allocated GPU
    AvailableCPU     float64   // Available CPU (Total - Allocated)
    AvailableMemory  float64   // Available memory
    AvailableStorage float64   // Available storage
    AvailableGPU     float64   // Available GPU
    IsActive         bool      // Active status (from DB field)
    LastHeartbeat    int64     // Unix timestamp of last heartbeat
    RegisteredAt     time.Time // Registration timestamp
    UpdatedAt        time.Time // Last update timestamp
}
```

**Active Status Determination:**
```go
// From API response (worker_handler.go)
currentTime := time.Now().Unix()
isActive := (currentTime - LastHeartbeat) < 30 // Active if heartbeat within 30 seconds
```

---

### Task Document
```go
type Task struct {
    TaskID           string    // Unique task identifier
    DockerImage      string    // Docker image to run
    Command          string    // Override container CMD
    Status           string    // pending/running/completed/failed/cancelled
    ReqCPU           float64   // Required CPU cores
    ReqMemory        float64   // Required memory in GB
    ReqStorage       float64   // Required storage in GB
    ReqGPU           float64   // Required GPU cores
    UserID           string    // User who submitted task
    Tag              string    // Task category
    KValue           float64   // Priority weight (1.5-2.5)
    SubmittedAt      int64     // Unix timestamp
    AssignedAt       int64     // Unix timestamp
    StartedAt        int64     // Unix timestamp
    CompletedAt      int64     // Unix timestamp
    AssignedWorker   string    // Worker ID
}
```

**Task Tags:**
- `web-server` - Web servers and APIs
- `data-processing` - Data transformation tasks
- `ml-training` - Machine learning training
- `batch-job` - Batch processing
- `api-service` - API backend services
- `background-task` - Background workers

---

### Telemetry Data
```go
type TelemetryData struct {
    WorkerID     string
    IsActive     bool
    CpuUsage     float64      // Percentage 0-100
    MemoryUsage  float64      // Percentage 0-100
    GpuUsage     float64      // Percentage 0-100
    RunningTasks []TaskInfo   // Currently running tasks
    LastUpdate   int64        // Unix timestamp
}
```

---

## Error Handling

### Standard Error Response
```json
{
  "error": "Error message describing what went wrong"
}
```

### HTTP Status Codes

**2xx Success:**
- `200 OK` - Request succeeded
- `201 Created` - Resource created successfully

**4xx Client Errors:**
- `400 Bad Request` - Invalid request parameters
- `404 Not Found` - Resource not found
- `405 Method Not Allowed` - HTTP method not supported

**5xx Server Errors:**
- `500 Internal Server Error` - Server-side error
- `503 Service Unavailable` - Service temporarily unavailable

---

## Code Organization

### Backend Structure

**HTTP Handlers:**
```
master/internal/http/
├── telemetry_server.go    - WebSocket server & telemetry endpoints
├── worker_handler.go       - Worker CRUD operations
├── task_handler.go         - Task CRUD operations
└── file_handler.go         - File management operations
```

**Core Server Logic:**
```
master/internal/server/
├── master_server.go        - Main server logic & worker management
├── scheduler.go            - Task scheduling algorithm
├── log_streaming_helper.go - Log streaming utilities
└── queue_processor.go      - Task queue processing
```

**Database Layer:**
```
master/internal/db/
├── workers.go      - Worker database operations
├── tasks.go        - Task database operations
├── assignments.go  - Task-Worker assignment tracking
├── results.go      - Task results storage
└── files.go        - File metadata storage
```

**CLI Interface:**
```
master/internal/cli/
└── cli.go - Command-line interface (mirrors API functionality)
```

### Frontend Structure

**API Clients:**
```
ui/src/api/
├── client.js    - Axios client configuration
├── workers.js   - Worker API calls
├── tasks.js     - Task API calls
└── files.js     - File API calls
```

**Hooks:**
```
ui/src/hooks/
├── useWebSocket.js      - Base WebSocket connection
├── useTelemetry.js      - Worker telemetry updates
└── useRealTimeTasks.js  - Task polling with change detection
```

**Pages:**
```
ui/src/pages/
├── Dashboard.jsx        - Cluster overview with resource stats
├── WorkersPage.jsx      - Worker management
├── TasksPage.jsx        - Task list with live logs
└── SubmitTaskPage.jsx   - Task submission form
```

---

## Quick Reference

### Most Common API Calls

**Get All Workers:**
```bash
curl http://localhost:8080/api/workers
```

**Register Worker:**
```bash
curl -X POST http://localhost:8080/api/workers \
  -H "Content-Type: application/json" \
  -d '{"worker_id":"worker-1","worker_ip":"localhost:50052"}'
```

**Submit Task:**
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "docker_image":"nginx:latest",
    "cpu_required":1.0,
    "memory_required":2.0,
    "tag":"web-server",
    "k_value":1.8
  }'
```

**Get All Tasks:**
```bash
curl http://localhost:8080/api/tasks
```

**Get Running Tasks:**
```bash
curl http://localhost:8080/api/tasks?status=running
```

**Cancel Task:**
```bash
curl -X DELETE http://localhost:8080/api/tasks/task-123
```

**WebSocket Telemetry:**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');
ws.onmessage = (e) => console.log(JSON.parse(e.data));
```

**WebSocket Task Logs:**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/tasks/task-123/logs');
ws.onmessage = (e) => {
  const data = JSON.parse(e.data);
  if (data.type === 'log') console.log(data.line);
};
```

---

## Environment Variables

```bash
# MongoDB Configuration
MONGODB_URI=mongodb://localhost:27017
MONGODB_DATABASE=cloudai

# Server Ports
GRPC_PORT=:50051      # gRPC server port
HTTP_PORT=:8080       # HTTP API server port

# File Storage
CLOUDAI_FILES_DIR=/var/cloudai/files  # File storage directory
```

---

## Related Documentation

- [Complete Implementation Summary](047_COMPLETE_IMPLEMENTATION_SUMMARY.md)
- [WebSocket Telemetry](036_WEBSOCKET_TELEMETRY.md)
- [Live Log Streaming](014_LIVE_LOG_STREAMING.md)
- [Worker Registration](046_WORKER_REGISTRATION_QUICK_REF.md)
- [Task Queuing System](028_TASK_QUEUING_QUICK_REF.md)

---

**Last Updated:** November 2025  
**API Version:** 1.0  
**Server:** CloudAI Master Node
