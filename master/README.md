# CloudAI Master Node

Central coordinator for the CloudAI distributed task execution system.

## Features

- **Interactive CLI**: Command-line interface for cluster management
- **gRPC Server**: Handles worker registration, heartbeats, task assignment, and log streaming
- **Worker Management**: Track worker health and capacity
- **Cluster Monitoring**: Real-time status and statistics
- **MongoDB Integration**: Persistent storage for tasks, users, and results
- **HTTP API Server**: REST API and WebSocket telemetry (port 8080)
- **JWT Authentication**: User registration, login, and protected endpoints
- **File Storage**: Secure file management with access control
- **Task Scheduler**: Pluggable scheduler with round-robin implementation
- **Task Queuing**: Queue tasks when no workers available

## Architecture

```
Master Node
├── gRPC Server (port 50051)
│   ├── RegisterWorker
│   ├── SendHeartbeat
│   ├── AssignTask
│   ├── CancelTask
│   ├── UploadTaskFiles
│   ├── StreamTaskLogs
│   └── ReportTaskCompletion
├── HTTP Server (port 8080)
│   ├── REST API (/api/*)
│   ├── WebSocket (/ws/telemetry)
│   └── Telemetry endpoint (/telemetry)
├── CLI Interface
│   ├── Task submission (scheduler-based)
│   ├── Worker listing
│   ├── Task monitoring
│   ├── Queue management
│   └── File management
├── Scheduler
│   └── Round-Robin (default)
└── Database Layer
    └── MongoDB (6 collections)
```

## Usage

### Start Master

```bash
./master-node
```

The interactive CLI will start automatically:

```
═══════════════════════════════════════════════════════
  CloudAI Master Node - Interactive CLI
═══════════════════════════════════════════════════════
Type 'help' for available commands

master>
```

## CLI Commands

### help

Show available commands and usage examples.

```bash
master> help
```

### status

Display current cluster status (workers, tasks).

```bash
master> status
```

Output:

```
╔═══ Cluster Status ═══
║ Total Workers: 2
║ Active Workers: 2
║ Running Tasks: 1
╚══════════════════════
```

### workers

List all registered workers with details.

```bash
master> workers
```

Output:

```
╔═══ Registered Workers ═══
║ worker-1
║   Status: 🟢 Active
║   IP: 192.168.1.100
║   Resources: CPU=4.0, Memory=8.0GB, GPU=0.0
║   Running Tasks: 1
║
║ worker-2
║   Status: 🟢 Active
║   IP: 192.168.1.101
║   Resources: CPU=8.0, Memory=16.0GB, GPU=1.0
║   Running Tasks: 0
╚═══════════════════════
```

### task

Submit a task to the cluster. The scheduler automatically selects an appropriate worker.

```bash
master> task <docker_image> [options]
```

**Parameters:**

- `docker_image`: Docker image to run
- `options`: Resource allocation and task flags
  - `-name <string>`: Task name (optional)
  - `-cpu_cores <num>`: CPU cores to allocate (default: 1.0)
  - `-mem <gb>`: Memory in GB (default: 0.5)
  - `-storage <gb>`: Storage in GB (default: 1.0)
  - `-gpu_cores <num>`: GPU cores to allocate (default: 0.0)

**Examples:**

```bash
# Basic task submission (scheduler picks worker)
master> task docker.io/username/sample-task:latest

# Task with name and resource allocation
master> task docker.io/username/gpu-task:latest -name ml-training -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
```

### dispatch

Assign a task directly to a specific worker (bypasses scheduler).

```bash
master> dispatch <worker_id> <docker_image> [options]
```

**Example:**

```bash
master> dispatch worker-1 ubuntu:latest -cpu_cores 2.0 -mem 4.0
```

### monitor

Stream live logs from a running task.

```bash
master> monitor <task_id>
```

### list-tasks

List all tasks with optional status filter.

```bash
master> list-tasks [status]
```

**Status filters:** `queued`, `pending`, `running`, `completed`, `failed`

### queue

Display tasks waiting in the queue.

```bash
master> queue
```

### files

List files for a specific user.

```bash
master> files <username>
```

### task-files

List files associated with a specific task.

```bash
master> task-files <task_id> <username>
```

### download

Download task output files.

```bash
master> download <task_id> <username> <output_dir>
```

### internal-state

Debug command to show internal state.

```bash
master> internal-state
```

### fix-resources

Reconcile worker resource allocations.

```bash
master> fix-resources
```

### exit / quit

Shutdown the master node.

```bash
master> exit
```

## Building

```bash
# Install dependencies
go mod tidy

# Build
go build -o master-node .

# Run
./master-node
```

## Components

### 1. gRPC Server (`internal/server/`)

Handles worker communication:

- **RegisterWorker**: Accept new workers
- **SendHeartbeat**: Monitor worker health
- **AssignTask**: Send tasks to workers
- **CancelTask**: Cancel running tasks
- **UploadTaskFiles**: Receive task output files
- **StreamTaskLogs**: Stream task execution logs
- **ReportTaskCompletion**: Receive task results

### 2. HTTP Server (`internal/http/`)

REST API and WebSocket server:

- **auth_handler.go**: JWT authentication (register, login)
- **task_handler.go**: Task CRUD operations
- **worker_handler.go**: Worker listing
- **file_handler.go**: File upload/download with access control
- **middleware.go**: JWT validation middleware
- **telemetry_server.go**: WebSocket telemetry streaming

### 3. CLI (`internal/cli/`)

Interactive command interface:

- Task submission (scheduler-based)
- Direct task dispatch
- Worker management
- Task monitoring with log streaming
- Queue management
- File operations
- Status display

### 4. Scheduler (`internal/scheduler/`)

Pluggable task scheduling:

- **scheduler.go**: Scheduler interface
- **round_robin.go**: Round-robin implementation

### 5. Database (`internal/db/`)

MongoDB integration for:

- Worker registry
- Task tracking
- Result storage
- User management
- File metadata
- Task assignments

### 6. Storage (`internal/storage/`)

File storage with access control:

- **file_storage.go**: Basic file operations
- **file_storage_secure.go**: Secure file operations
- **access_control.go**: User-based access control

## Configuration

### Environment Variables

Create `.env` in project root:

```bash
MONGODB_USERNAME=admin
MONGODB_PASSWORD=password123
JWT_SECRET=your-secret-key
```

### Ports

- **gRPC Server**: `50051`
- **HTTP Server**: `8080`
- **MongoDB**: `27017` (via docker-compose)

## Worker Registration Flow

```
1. Worker starts and connects to master
   ↓
2. Worker sends RegisterWorker request
   ↓
3. Master validates and stores worker info
   ↓
4. Worker added to active pool
   ↓
5. Worker begins sending heartbeats
```

## Heartbeat Monitoring

Workers send heartbeat every 5 seconds containing:

- CPU usage percentage
- Memory usage
- Storage usage
- List of running tasks

Master logs heartbeat activity:

```
Heartbeat from worker-1: CPU=45.00%, Memory=120.50MB, Running Tasks=2
```

## Task Assignment (Scheduler-Based)

```
1. User enters: task docker.io/image:tag
   ↓
2. Scheduler selects best available worker
   ↓
3. Master generates unique task ID
   ↓
4. Master sends task via gRPC to selected worker
   ↓
5. Worker acknowledges receipt
   ↓
6. CLI shows success message with task ID
```

## Direct Task Dispatch

```
1. User enters: dispatch worker-1 docker.io/image:tag
   ↓
2. Master validates worker exists
   ↓
3. Master generates unique task ID
   ↓
4. Master sends task via gRPC to worker
   ↓
5. Worker acknowledges receipt
   ↓
6. CLI shows success message
```

## Task Result Handling

When a task completes:

```
1. Worker sends ReportTaskCompletion
   ↓
2. Master logs completion status
   ↓
3. Master prints task logs
   ↓
4. Master updates database (optional)
   ↓
5. Task removed from worker's running list
```

## MongoDB Collections

| Collection        | Purpose                          |
| ----------------- | -------------------------------- |
| `USERS`           | User accounts and authentication |
| `WORKER_REGISTRY` | Worker nodes and capacities      |
| `TASKS`           | Task definitions and status      |
| `ASSIGNMENTS`     | Task-to-worker mappings          |
| `RESULTS`         | Task execution results           |
| `FILE_METADATA`   | File storage metadata            |

## Requirements

- Go 1.22+
- MongoDB 5.0+ (optional)
- Network access for workers to connect

## Monitoring

Master logs show:

- Worker registrations
- Heartbeat activity
- Task assignments
- Task completions
- Error conditions

Example output:

```
✓ MongoDB collections ensured
Starting gRPC server on :50051...
✓ Master node started successfully
✓ gRPC server listening on :50051

Worker registration: worker-1 (IP: localhost, CPU: 4.00, Memory: 8.00 GB)
Heartbeat from worker-1: CPU=30.00%, Memory=45.20MB, Running Tasks=0
Task completion: task-123 from worker worker-1 [Status: success]
```

## Error Handling

- MongoDB unavailable → Warning logged, continues without DB
- Worker connection lost → Marked inactive after missed heartbeats
- Invalid task assignment → Error shown in CLI
- Worker not found → CLI shows error message

## Future Enhancements

- [ ] Advanced scheduling algorithms (priority-based, resource-aware)
- [ ] Multi-master setup for HA
- [ ] Metrics and analytics dashboard
- [ ] Task dependencies and workflows
- [ ] Container networking configuration
