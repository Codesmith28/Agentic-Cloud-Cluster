# CloudAI Master Node

Central coordinator for the CloudAI distributed task execution system.

## Features

- 🎮 **Interactive CLI**: Command-line interface for cluster management
- 📡 **gRPC Server**: Handles worker registration, heartbeats, and results
- 👥 **Worker Management**: Track worker health and capacity
- 📊 **Cluster Monitoring**: Real-time status and statistics
- 🗃️ **MongoDB Integration**: Persistent storage for tasks and results

## Architecture

```
Master Node
├── gRPC Server (port 50051)
│   ├── RegisterWorker
│   ├── SendHeartbeat
│   └── ReportTaskCompletion
├── CLI Interface
│   ├── Task assignment
│   ├── Worker listing
│   └── Status monitoring
└── Database Layer
    └── MongoDB (optional)
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

Assign a task to a specific worker. The target worker must be explicitly specified.

```bash
master> task <worker_id> <docker_image> [options]
```

**Parameters:**

- `worker_id`: ID of the worker to assign the task to (required)
- `docker_image`: Docker image to run
- `options`: Resource allocation flags
  - `-cpu_cores <num>`: CPU cores to allocate (default: 1.0)
  - `-mem <gb>`: Memory in GB (default: 0.5)
  - `-storage <gb>`: Storage in GB (default: 1.0)
  - `-gpu_cores <num>`: GPU cores to allocate (default: 0.0)

**Examples:**

```bash
# Basic task assignment
master> task worker-1 docker.io/username/sample-task:latest

# Task with custom resource allocation
master> task worker-2 docker.io/username/gpu-task:latest -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
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
- **ReportTaskCompletion**: Receive task results

### 2. CLI (`internal/cli/`)

Interactive command interface:

- Task assignment
- Worker management
- Cluster monitoring
- Status display

### 3. Database (`internal/db/`)

MongoDB integration for:

- Worker registry
- Task tracking
- Result storage
- User management

## Configuration

### Environment Variables

Create `.env` in project root:

```bash
MONGODB_USERNAME=admin
MONGODB_PASSWORD=password123
```

### Ports

- **gRPC Server**: `50051`
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

## Task Assignment

```
1. User enters: task worker-1 docker.io/image:tag
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

- [ ] Task queue management
- [ ] Automatic worker load balancing
- [ ] Task scheduling algorithms
- [ ] Authentication and authorization
- [ ] Web dashboard
- [ ] Metrics and analytics
- [ ] Multi-master setup for HA
- [ ] Task priority levels
