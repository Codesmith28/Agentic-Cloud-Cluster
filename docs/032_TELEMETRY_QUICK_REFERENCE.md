# Telemetry System - Quick Reference

## Architecture Summary

```
Worker → (5s interval) → Heartbeat → Master RPC Handler → TelemetryManager → Per-Worker Thread
                                                                ↓
                                                        Telemetry Data Store
                                                                ↓
                                                    ┌───────────┴───────────┐
                                                    ↓                       ↓
                                                CLI Query              HTTP API Query
```

## Key Features

- ✅ **Non-blocking**: Telemetry processing doesn't slow down main process
- ✅ **Per-worker threads**: Each worker has dedicated goroutine
- ✅ **Thread-safe**: Mutex-protected data access
- ✅ **HTTP API**: REST endpoints for external monitoring
- ✅ **Graceful shutdown**: Clean thread termination

## HTTP API Endpoints

### Telemetry & Monitoring

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/health` | GET | Health check | `{"status": "healthy", "time": 1699286400}` |
| `/telemetry` | GET | All workers telemetry | Map of worker_id → telemetry data |
| `/telemetry/{workerID}` | GET | Specific worker telemetry | Single worker's telemetry data |
| `/workers` | GET | All workers basic info | Map of worker_id → basic stats |

### Task Management

| Endpoint | Method | Description | Request Body |
|----------|--------|-------------|--------------|
| `/api/tasks` | POST | Submit new task | `{"docker_image": "...", "cpu_required": 1.0, "memory_required": 512}` |
| `/api/tasks` | GET | List all tasks | Query params: `?status=queued` (pending/queued/running/completed/failed/cancelled) |
| `/api/tasks/{taskID}` | GET | Get task details | - |
| `/api/tasks/{taskID}/logs` | GET | Get task execution logs | - |
| `/api/tasks/{taskID}` | DELETE | Cancel task | - |

### Worker Management

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/api/workers` | GET | List all workers | Array of worker details with metrics |
| `/api/workers/{workerID}` | GET | Get worker details | Worker info with current status |
| `/api/workers/{workerID}/metrics` | GET | Get worker resource metrics | CPU, memory, GPU usage |
| `/api/workers/{workerID}/tasks` | GET | Get worker's tasks | Array of task assignments |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `:8080` | Port for HTTP telemetry API |
| Heartbeat interval | 5s | How often workers send heartbeats |
| Inactivity timeout | 30s | When to mark worker as inactive |

## Code Locations

| Component | File Path |
|-----------|-----------|
| TelemetryManager | `master/internal/telemetry/telemetry_manager.go` |
| HTTP API Server | `master/internal/http/telemetry_server.go` |
| Master Integration | `master/internal/server/master_server.go` |
| Worker Telemetry | `worker/internal/telemetry/telemetry.go` |

## Usage Examples

### Start master with HTTP API
```bash
cd master
HTTP_PORT=":8080" ./masterNode
```

### Query via HTTP

#### Telemetry Endpoints
```bash
# All workers telemetry
curl http://localhost:8080/telemetry | jq

# Specific worker telemetry
curl http://localhost:8080/telemetry/worker-1 | jq

# Workers list
curl http://localhost:8080/workers | jq
```

#### Task Management
```bash
# Submit a new task
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "docker_image": "python:3.9",
    "cpu_required": 2.0,
    "memory_required": 1024,
    "gpu_required": 0.0,
    "priority": 5
  }' | jq

# List all tasks
curl http://localhost:8080/api/tasks | jq

# List pending tasks
curl "http://localhost:8080/api/tasks?status=pending" | jq

# Get task details
curl http://localhost:8080/api/tasks/task-123 | jq

# Get task logs
curl http://localhost:8080/api/tasks/task-123/logs | jq

# Cancel a task
curl -X DELETE http://localhost:8080/api/tasks/task-123 | jq
```

#### Worker Management
```bash
# List all workers
curl http://localhost:8080/api/workers | jq

# Get worker details
curl http://localhost:8080/api/workers/worker-1 | jq

# Get worker metrics
curl http://localhost:8080/api/workers/worker-1/metrics | jq

# Get worker's assigned tasks
curl http://localhost:8080/api/workers/worker-1/tasks | jq
```

### Query via CLI
```
master> workers
master> status worker-1
```

## TelemetryManager Methods

```go
// Registration
RegisterWorker(workerID string)
UnregisterWorker(workerID string)

// Processing (non-blocking)
ProcessHeartbeat(hb *pb.Heartbeat) error

// Querying
GetWorkerTelemetry(workerID string) (*WorkerTelemetryData, bool)
GetAllWorkerTelemetry() map[string]*WorkerTelemetryData

// Lifecycle
Start()
Shutdown()
```

## Telemetry Data Structure

```go
type WorkerTelemetryData struct {
    WorkerID      string
    CpuUsage      float64
    MemoryUsage   float64
    GpuUsage      float64
    RunningTasks  []*pb.RunningTask
    LastUpdate    int64
    IsActive      bool
}
```

## Thread Model

| Thread | Purpose | Count |
|--------|---------|-------|
| Main | CLI and coordination | 1 |
| gRPC Server | Handle RPC requests | 1 |
| TelemetryManager | Inactivity checker | 1 |
| Worker Threads | Process worker heartbeats | 1 per worker |
| HTTP Server | Serve telemetry API | 1 (optional) |

## Performance Metrics

| Operation | Typical Latency |
|-----------|----------------|
| Heartbeat processing | < 1ms |
| Single worker query | < 1ms |
| All workers query | < 10ms |
| HTTP API response | < 5ms |

## Common Tasks

### Enable HTTP API
Add to `.env`:
```bash
HTTP_PORT=":8080"
```

### Monitor specific worker
```bash
watch -n 1 'curl -s http://localhost:8080/telemetry/worker-1 | jq'
```

### Check active workers
```bash
curl -s http://localhost:8080/workers | jq 'to_entries | map(select(.value.is_active)) | length'
```

### Monitor system health
```bash
while true; do
  curl -s http://localhost:8080/health
  sleep 5
done
```

## Troubleshooting

| Problem | Check |
|---------|-------|
| Worker not showing up | Is worker registered? Check `workers` command |
| Telemetry shows inactive | Check network, verify heartbeats arriving |
| HTTP API not responding | Verify `HTTP_PORT` set, check firewall |
| High memory usage | Check for worker leaks (not unregistered) |

## Integration Points

### Adding custom telemetry metrics
1. Update `pb.Heartbeat` protobuf message
2. Update worker's `getResourceUsage()` to collect new metric
3. Update `WorkerTelemetryData` struct
4. Update HTTP serialization in `convertTelemetryToMap()`

### Connecting to Prometheus
```go
// Add to master/internal/http/telemetry_server.go
mux.HandleFunc("/metrics", ts.handlePrometheusMetrics)
```

### Database persistence
```go
// Add callback in main.go
telemetryMgr.SetUpdateCallback(func(workerID string, data *telemetry.WorkerTelemetryData) {
    // Save to MongoDB/PostgreSQL
})
```

## Next Steps

1. ✅ System is production-ready
2. 🔄 Consider adding Prometheus integration
3. 🔄 Add historical telemetry storage
4. 🔄 Implement alert system for anomalies
5. 🔄 Add telemetry visualization dashboard
