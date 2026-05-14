# CloudAI Worker Node

Distributed task executor with Docker-backed task runtime and gRPC integration.

**See Also:** [Project Structure Guide](../docs/PROJECT_STRUCTURE.md#worker-node-worker) | [Project Appendix](../docs/PROJECT_APPENDIX.md) | [ARCHITECTURE](../ARCHITECTURE.md)

## Features

- Receives tasks from master and executes them in containers
- Supports cancellation and live log streaming
- Reports completion/results and uploads `/output` artifacts
- Sends heartbeat telemetry every 5s once master is known
- Supports runtime capacity overrides for heterogeneous fleets
- Configurable task container network mode (`bridge`/`host`/`none`)

## Architecture

```text
Worker Node
├── gRPC Server
│   ├── MasterRegister
│   ├── AssignTask
│   ├── CancelTask
│   └── StreamTaskLogs
├── Telemetry Monitor
│   └── SendHeartbeat (every 5s, after master registration)
├── Task Executor (Docker)
│   ├── Pull / Create / Start / Wait / Cleanup
│   └── Bind-mount host output dir to container /output
└── Result/File Reporter
    ├── ReportTaskCompletion
    └── UploadTaskFiles
```

## Runtime Usage

### Start worker

```bash
./workerNode
```

On startup, worker resolves:

- Stable worker ID (`WORKER_ID` or persisted state)
- gRPC port (`WORKER_PORT` or first free port from 50052)
- Bind IP (`WORKER_BIND_IP` or detected non-loopback IP)
- Metrics port (`WORKER_METRICS_PORT`, default 9101)

It then waits for master registration (`MasterRegister`) before sending heartbeats and registering back.

## Registration behavior (current flow)

1. Worker starts and listens for RPCs
2. Admin/operator registers worker endpoint on master (`register <id> <ip:port>`)
3. Master calls worker `MasterRegister`
4. Worker stores master address and sends `RegisterWorker` back with capacity
5. Worker sends periodic heartbeats to master

## Runtime Configuration

| Variable | Default | Notes |
| --- | --- | --- |
| `WORKER_ID` | persisted hostname/ID | Stable identity override |
| `CLOUDAI_WORKER_STATE_DIR` | `~/.cloudai/worker` | Stores persisted worker ID |
| `WORKER_PORT` | first free from `50052` | Worker gRPC port |
| `WORKER_BIND_IP` | detected worker IP | gRPC bind address |
| `WORKER_METRICS_PORT` | `9101` | Prometheus endpoint port (`/metrics`) |
| `WORKER_TOTAL_CPU` | detected | Advertised CPU cores override |
| `WORKER_TOTAL_MEMORY_GB` | detected | Advertised memory override |
| `WORKER_TOTAL_STORAGE_GB` | detected | Advertised storage override |
| `WORKER_CONTAINER_NETWORK_MODE` | `bridge` | `bridge`, `host`, or `none` |

## Container runtime behavior

- Each task runs in a dedicated container with CPU/memory limits from task request.
- Host output directory is mounted to container `/output` and collected after completion.
- Task container network mode is selected via `WORKER_CONTAINER_NETWORK_MODE`.
- Worker output base directory is initialized to `/var/cloudai/outputs` (fallback `~/.cloudai/outputs`).

## Monitoring

Worker logs include:

- master registration status
- heartbeat delivery status
- task assignment/execution lifecycle
- cancellation handling
- task result + file upload outcomes

Heartbeat logs are emitted as percentages (CPU/memory/storage), not absolute memory sizes.

## Build

```bash
go mod tidy
go build -o workerNode .
./workerNode
```

## Requirements

- Docker daemon running
- Go 1.22+
- Network reachability between master and worker
- Registry access (unless image is already available locally)
