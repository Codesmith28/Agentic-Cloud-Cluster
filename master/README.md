# CloudAI Master Node

Central coordinator for CloudAI distributed execution.

## Features

- Interactive CLI for worker/task operations
- gRPC control plane for workers
- HTTP API + WebSocket telemetry server
- Task queue with automatic retry/requeue paths
- Scheduler stack: **RTS (default)**, **RR**, optional **PPO**
- MongoDB-backed persistence (workers, tasks, attempts, results, files, scheduler models)
- File metadata + auth endpoints when backing services are enabled

## Architecture

```text
Master Node
├── gRPC Server (default :50051)
│   ├── RegisterWorker
│   ├── SendHeartbeat
│   ├── AssignTask
│   ├── CancelTask
│   ├── UploadTaskFiles
│   ├── StreamTaskLogs
│   └── ReportTaskCompletion
├── HTTP Server (default :8080)
│   ├── REST API (/api/*)
│   ├── WebSocket (/ws/telemetry)
│   ├── Telemetry REST (/telemetry, /workers)
│   └── Prometheus metrics (/metrics)
├── CLI
│   ├── Worker registration/management
│   ├── Task submission/dispatch/cancel/monitor
│   └── Queue + state inspection
└── Scheduler Layer
    ├── RR
    ├── RTS (default)
    └── PPO (optional, Python gRPC service)
```

## Runtime Usage

### Start master

```bash
./masterNode
```

### Headless mode (no interactive CLI)

```bash
CLOUDAI_HEADLESS=true ./masterNode
```

### Non-interactive test workflow mode

```bash
./masterNode test list
./masterNode test run smoke -scheduler RR
./masterNode test cleanup
```

## Scheduler selection behavior

At startup, the scheduler is selected in this order:

1. `SCHED_ALGO` env override (if set)
2. Interactive prompt (TTY only)
3. Config default (`RTS`)

Supported values: `RR`, `RTS`, `PPO`.

## Worker registration behavior (current runtime flow)

Workers must be pre-registered with an endpoint before they can fully join:

1. Start worker (it listens and waits for master registration)
2. Register worker endpoint from master CLI or API:
   - CLI: `register <worker_id> <worker_ip:port>`
3. Master calls worker `MasterRegister`
4. Worker calls back `RegisterWorker` with capacity details
5. Worker heartbeats begin (every 5s)

If a worker is not pre-registered, `RegisterWorker` is rejected.

## Common CLI commands

- `help`
- `status`
- `workers`
- `stats <worker_id>`
- `register <id> <ip:port>` / `unregister <id>`
- `task <docker_image> [-name <name>] [-cpu_cores <n>] [-mem <gb>] [-storage <gb>] [-k <1.5-2.5>] [-type <cpu-light|cpu-heavy|memory-heavy|mixed>]`
- `dispatch <worker_id> <docker_image> [resource flags]`
- `monitor <task_id>`
- `cancel <task_id>`
- `list-tasks [queued|pending|running|completed|failed|cancelled]`
- `queue`
- `internal-state`
- `fix-resources`
- `files`, `task-files`, `download`

## Runtime Configuration

### Core env vars

| Variable | Default | Notes |
| --- | --- | --- |
| `GRPC_PORT` | `:50051` | gRPC listen port suffix/address |
| `HTTP_PORT` | `:8080` | HTTP API/telemetry port |
| `CLOUDAI_HEADLESS` | `false` | Disable interactive CLI |
| `MASTER_BIND_ADDR` | auto | gRPC bind address override |
| `MASTER_ADVERTISE_ADDR` | auto | Address advertised to workers |

### MongoDB env vars

| Variable | Default | Notes |
| --- | --- | --- |
| `MONGODB_HOST` | `localhost:27017` | Host/port used to build Mongo URI |
| `MONGODB_DATABASE` | `cluster_db` | Database name |
| `MONGODB_USERNAME` | _empty_ | Optional |
| `MONGODB_PASSWORD` | _empty_ | Optional |

### Scheduler env vars

| Variable | Default | Notes |
| --- | --- | --- |
| `SCHED_ALGO` | `RTS` | `RR`/`RTS`/`PPO` |
| `SCHED_GA_PARAMS_PATH` | `config/ga_output.json` | RTS parameter file |
| `SCHED_SLA_MULTIPLIER` | `2.0` | Validated to `1.5-2.5` |
| `PPO_GRPC_ADDR` | `127.0.0.1:50050` | PPO service address |
| `PPO_REQUEST_TIMEOUT_MS` | `1500` | PPO request timeout |
| `PPO_AUTOSTART` | `true` | Auto-start PPO Python service |
| `PPO_MODEL_PATH` | `latest` | PPO model selector |
| `PPO_DEPLOYMENT_MODE` | `active` | `active`, `shadow`, `fallback` |
| `PPO_ONLINE_UPDATES_ENABLED` | `true` | PPO online update toggle |

### API/Auth/Telemetry env vars

| Variable | Default | Notes |
| --- | --- | --- |
| `JWT_SECRET` | random at startup if unset | Set explicitly for stable auth sessions |
| `AUTH_COOKIE_SECURE` | auto | Optional `true/false` override for auth cookie `Secure`; auto keeps secure cookies except localhost/loopback HTTP |
| `WEBUI_ADMIN_NAME` | `Web UI Admin` | Name for startup-bootstrapped default Web UI admin user |
| `WEBUI_ADMIN_EMAIL` | `admin@localhost` | Email for startup-bootstrapped default Web UI admin user |
| `WEBUI_ADMIN_PASSWORD` | `ChangeMeAdmin123!` | Password used when creating missing default admin user |
| `WEBUI_ADMIN_RESET_PASSWORD` | `false` | If `true`, reset existing admin password at startup |
| `ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:3001` | CORS/WebSocket origin allowlist |

## Operational Notes

- Task queue processor runs every **5 seconds**.
- Worker reconnection/inactivity checks run every **5 seconds**.
- Workers are marked inactive after **~30s** without heartbeat.
- Heartbeat usage values are normalized in worker RPCs and displayed as percentages in logs/telemetry.
- File base directory is initialized at startup (`/var/cloudai/files`, fallback `~/.cloudai/files`).

## Build

```bash
go mod tidy
go build -o masterNode .
./masterNode
```

## Requirements

- Go 1.22+
- Network reachability between master and workers
- MongoDB (optional but recommended for persistence)
