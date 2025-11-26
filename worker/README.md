# CloudAI Worker Node

Distributed task executor with Docker integration and gRPC communication.

## Features

- **Task Execution**: Runs Docker containers for assigned tasks
- **Telemetry**: Sends periodic heartbeat with resource stats
- **Auto-Registration**: Registers with master on startup
- **Log Streaming**: Real-time log streaming to master via gRPC
- **File Upload**: Upload task output files to master
- **Async Execution**: Non-blocking task processing
- **Task Cancellation**: Cancel running tasks on request

## Architecture

```
Worker Node
├── gRPC Server (receives tasks from master)
│   ├── AssignTask
│   └── CancelTask
├── Telemetry Monitor (sends heartbeats)
├── Task Executor (runs Docker containers)
├── Log Manager (streams logs to master)
├── File Uploader (uploads task files)
└── Result Reporter (sends completion status)
```

## Usage

### Basic

```bash
./worker-node
```

### With Configuration

```bash
./worker-node \
  -id worker-1 \
  -ip 192.168.1.100 \
  -master master.example.com:50051 \
  -port :50052
```

### Flags

| Flag      | Description           | Default           |
| --------- | --------------------- | ----------------- |
| `-id`     | Worker identifier     | `worker-1`        |
| `-ip`     | Worker IP address     | `localhost`       |
| `-master` | Master server address | `localhost:50051` |
| `-port`   | gRPC server port      | `:50052`          |

## Building

```bash
# Install dependencies
go mod tidy

# Build
go build -o worker-node .

# Run
./worker-node
```

## Components

### 1. Task Executor (`internal/executor/`)

Handles Docker container lifecycle:

- Pull images from registry
- Create and start containers
- Stream logs in real-time
- Monitor completion
- Upload output files
- Cleanup resources

### 2. Log Manager (`internal/logstream/`)

Manages log streaming:

- **log_manager.go**: Coordinates log collection
- **log_broadcaster.go**: Broadcasts logs to master via gRPC

### 3. Telemetry Monitor (`internal/telemetry/`)

Manages communication with master:

- Send periodic heartbeat (every 5s)
- Report CPU/memory usage
- Track running tasks
- Register on startup

### 4. Worker Server (`internal/server/`)

gRPC server handling:

- `AssignTask` - Receive task assignments
- `CancelTask` - Handle cancellation requests

## Task Flow

```
1. Master assigns task via gRPC
   ↓
2. Worker accepts and acknowledges
   ↓
3. Executor pulls Docker image
   ↓
4. Container starts and runs
   ↓
5. Logs are streamed to master in real-time
   ↓
6. Output files are uploaded to master
   ↓
7. Result is reported to master
   ↓
8. Container is cleaned up
```

## Requirements

- Docker daemon running
- Go 1.22+
- Network access to master node
- Access to Docker registry for images

## Monitoring

Worker logs show:

- Registration status
- Heartbeat transmissions
- Task assignments
- Execution progress
- Completion results

Example output:

```
Starting Worker Node: worker-1
Master Address: localhost:50051
✓ Worker registered: Worker registered successfully
Starting telemetry monitor (interval: 5s)
✓ Worker worker-1 started successfully
✓ gRPC server listening on :50052
✓ Ready to receive tasks...
Heartbeat sent: CPU=30.0%, Memory=45.2MB, Tasks=0
Received task assignment: task-123 (Image: docker.io/...)
[Task task-123] Starting execution...
[Task task-123] ✓ Completed successfully
✓ Task result reported: Task result received
```

## Error Handling

- Docker connection failures → Logged, task fails
- Image pull failures → Reported to master
- Container crashes → Exit code captured
- Master unreachable → Heartbeat retries

## Resource Management

The worker automatically:

- Stops containers after completion
- Removes containers to free disk space
- Reports resource usage to master
- Tracks allocated vs. available resources

## Future Enhancements

- [ ] Parallel task execution
- [ ] GPU support
- [ ] Resource limits per container
- [ ] Container networking configuration
- [ ] Volume mounting for shared storage
- [ ] Enhanced resource monitoring
