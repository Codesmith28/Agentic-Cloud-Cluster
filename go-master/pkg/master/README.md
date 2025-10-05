# CloudAI Master API

This package implements the Master node's gRPC API server for the CloudAI system.

## Quick Start

```bash
# Build the master server
cd go-master
go build -o master ./cmd/master

# Run the server
./master
```

The server will listen on port 50051 by default (configurable via `MASTER_PORT` env variable).

## API Endpoints

### Client-Facing APIs

#### SubmitTask
Submit a new task to the scheduler.

```bash
grpcurl -plaintext -d '{
  "task": {
    "task_type": "compute",
    "cpu_req": 2.0,
    "mem_mb": 1024,
    "gpu_req": 0,
    "priority": 5,
    "estimated_sec": 60
  }
}' localhost:50051 scheduler.SchedulerService/SubmitTask
```

#### GetTaskStatus
Query the status of a submitted task.

```bash
grpcurl -plaintext -d '{
  "task_id": "your-task-id-here"
}' localhost:50051 scheduler.SchedulerService/GetTaskStatus
```

#### CancelTask
Cancel a pending task.

```bash
grpcurl -plaintext -d '{
  "task_id": "your-task-id-here"
}' localhost:50051 scheduler.SchedulerService/CancelTask
```

### Worker-Facing APIs

#### RegisterWorker
Register a new worker node.

```bash
grpcurl -plaintext -d '{
  "worker": {
    "id": "worker-1",
    "total_cpu": 8.0,
    "total_mem": 16384,
    "gpus": 1
  }
}' localhost:50051 scheduler.SchedulerService/RegisterWorker
```

#### Heartbeat
Send periodic heartbeat to update worker status.

```bash
grpcurl -plaintext -d '{
  "worker": {
    "id": "worker-1",
    "free_cpu": 4.0,
    "free_mem": 8192,
    "free_gpus": 1
  },
  "running_task_ids": []
}' localhost:50051 scheduler.SchedulerService/Heartbeat
```

#### ListWorkers
List all active workers.

```bash
grpcurl -plaintext localhost:50051 scheduler.SchedulerService/ListWorkers
```

#### ReportTaskCompletion
Report task completion from a worker.

```bash
grpcurl -plaintext -d '{
  "task_id": "your-task-id",
  "worker_id": "worker-1",
  "success": true,
  "actual_duration_sec": 45,
  "resource_usage": {
    "avg_cpu": "1.8",
    "peak_mem_mb": "900"
  }
}' localhost:50051 scheduler.SchedulerService/ReportTaskCompletion
```

## Components

- **`pkg/master/server.go`**: Main API server implementation
- **`go-master/cmd/master/main.go`**: Entry point and server initialization
- **`proto/scheduler.proto`**: Protocol buffer definitions

## Dependencies

- TaskQueue (Sprint 1.2) - For managing pending tasks
- WorkerRegistry (Sprint 1.1) - For tracking worker nodes

## Testing

```bash
# Run unit tests
go test -v ./pkg/master

# Run with coverage
go test -v -cover ./pkg/master
```

## Implementation Notes

- **Import Cycle Prevention**: The MasterServer is in `pkg/master` (not `pkg/api`) to avoid circular imports
- **Auto ID Generation**: Tasks without IDs are automatically assigned UUIDs
- **Resource Tracking**: Initial free resources are set equal to total resources during registration
- **Cleanup**: Background goroutine cleans up stale workers (30s timeout) and expired reservations
- **Graceful Shutdown**: SIGINT/SIGTERM triggers graceful server shutdown
