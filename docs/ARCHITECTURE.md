# CloudAI System Architecture

**Last Updated:** November 26, 2025

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Interfaces                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │   CLI Tool      │  │   Web Dashboard │  │   REST API      │  │
│  │ (Interactive)   │  │   (React/Vite)  │  │ (HTTP + WS)     │  │
│  └─────────┬───────┘  └─────────┬───────┘  └─────────┬───────┘  │
│            │                    │                    │          │
└────────────┼────────────────────┼────────────────────┼──────────┘
             │                    │                    │
             ▼                    ▼                    ▼
┌────────────────────────────────────────────────────────────────┐
│                        Master Node (Go)                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │     CLI      │  │  HTTP Server │  │  Auth/JWT    │          │
│  │  Interface   │  │  (Port 8080) │  │   Handler    │          │
│  └──────────────┘  └──────┬───────┘  └──────────────┘          │
│                           │                                    │
│  ┌──────────────┐  ┌──────▼──────┐  ┌──────────────┐           │
│  │  gRPC Server │  │Telemetry Mgr│  │  Scheduler   │           │
│  │ (Port 50051) │  │ (WebSocket) │  │(Round-Robin) │           │
│  └──────┬───────┘  └─────────────┘  └──────────────┘           │
│                                                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │File Storage  │  │  Task Queue  │  │   Database   │          │
│  │  Service     │  │   Manager    │  │   (MongoDB)  │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└─────────┼──────────────────────────────────────────────────────┘
          │
          │ gRPC (Task Assignment, Heartbeats, File Upload)
          ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Worker Nodes (Go)                         │
│  ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐ │
│  │   Worker Node 1  │ │   Worker Node 2  │ │   Worker Node N  │ │
│  │                  │ │                  │ │                  │ │
│  │  ┌────────────┐  │ │  ┌────────────┐  │ │  ┌────────────┐  │ │
│  │  │gRPC Server │  │ │  │gRPC Server │  │ │  │gRPC Server │  │ │
│  │  │(Port 50052)│  │ │  │(Port 50053)│  │ │  │(Port 5005X)│  │ │
│  │  └─────┬──────┘  │ │  └─────┬──────┘  │ │  └─────┬──────┘  │ │
│  │        │         │ │        │         │ │        │         │ │
│  │  ┌─────▼─────┐   │ │  ┌─────▼─────┐   │ │  ┌─────▼─────┐   │ │
│  │  │  Docker   │   │ │  │  Docker   │   │ │  │  Docker   │   │ │
│  │  │ Executor  │   │ │  │ Executor  │   │ │  │ Executor  │   │ │
│  │  └─────┬──────┘  │ │  └─────┬──────┘  │ │  └─────┬──────┘  │ │
│  │        │         │ │        │         │ │        │         │ │
│  │  ┌─────▼─────┐   │ │  ┌─────▼─────┐   │ │  ┌─────▼─────┐   │ │
│  │  │Log Stream │   │ │  │Log Stream │   │ │  │Log Stream │   │ │
│  │  │  Manager  │   │ │  │  Manager  │   │ │  │  Manager  │   │ │
│  │  └─────┬──────┘  │ │  └─────┬──────┘  │ │  └─────┬──────┘  │ │
│  │        │         │ │        │         │ │        │         │ │
│  │  ┌─────▼─────┐   │ │  ┌─────▼─────┐   │ │  ┌─────▼─────┐   │ │
│  │  │ Telemetry │   │ │  │ Telemetry │   │ │  │ Telemetry │   │ │
│  │  │  Monitor  │   │ │  │  Monitor  │   │ │  │  Monitor  │   │ │
│  │  └────────────┘  │ │  └────────────┘  │ │  └────────────┘  │ │
│  │                  │ │                  │ │                  │ │
│  └────────┬─────────┘ └────────┬─────────┘ └────────┬─────────┘ │
│           │                    │                    │           │
└───────────┼────────────────────┼────────────────────┼───────────┘
            │                    │                    │
            └────────────────────┴────────────────────┘
                           │
                           ▼
                ┌────────────────────┐
                │  Docker Engine     │
                │  (Container Runtime)│
                └────────────────────┘
                           │
                           ▼
                ┌────────────────────┐
                │ Docker Hub/Registry│
                │  (Task Images)     │
                └────────────────────┘
```

## Component Details

### Master Node Components

```
┌─────────────────────────────────────────────────────────────────┐
│                       Master Node                               │
│                                                                 │
│  ┌────────────────────────────────────────────────────────┐     │
│  │              CLI Interface (internal/cli/)              │     │
│  │  - Interactive command prompt (readline-based)         │     │
│  │  - Task submission: task, dispatch commands            │     │
│  │  - Worker management: workers, register, unregister    │     │
│  │  - Monitoring: status, stats, monitor, queue           │     │
│  │  - File operations: files, task-files, download        │     │
│  │  - Resource management: fix-resources, internal-state  │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │         HTTP/WebSocket Server (internal/http/)          │     │
│  │  REST Endpoints:                                        │     │
│  │  - Authentication: /api/auth/register, /api/auth/login │     │
│  │  - Tasks: /api/tasks (CRUD operations)                 │     │
│  │  - Workers: /api/workers (list, details, metrics)      │     │
│  │  - Files: /api/files (list, download with access ctrl) │     │
│  │  - Telemetry: /telemetry, /health                      │     │
│  │  WebSocket:                                             │     │
│  │  - /ws/telemetry - Real-time streaming                 │     │
│  │  - /ws/telemetry/{workerID} - Per-worker streaming     │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │           gRPC Server (internal/server/)               │     │
│  │  Worker → Master RPCs:                                 │     │
│  │  - RegisterWorker() - Worker registration              │     │
│  │  - SendHeartbeat() - Periodic health updates           │     │
│  │  - ReportTaskCompletion() - Task results               │     │
│  │  - UploadTaskFiles() - File streaming from workers     │     │
│  │  Master → Worker RPCs:                                 │     │
│  │  - AssignTask() - Send task to worker                  │     │
│  │  - CancelTask() - Request task cancellation            │     │
│  │  - StreamTaskLogs() - Live log streaming               │     │
│  │  - MasterRegister() - Master registration with worker  │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │        Scheduler (internal/scheduler/)                 │     │
│  │  - Scheduler interface for pluggable algorithms        │     │
│  │  - Round-Robin scheduler (current implementation)      │     │
│  │  - Resource-aware worker selection                     │     │
│  │  - Active worker filtering                             │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │       Telemetry Manager (internal/telemetry/)          │     │
│  │  - Per-worker thread-safe data structures              │     │
│  │  - WebSocket connection management                     │     │
│  │  - Real-time data broadcasting                         │     │
│  │  - Telemetry aggregation and caching                   │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │        File Storage (internal/storage/)                │     │
│  │  - Secure file storage with access control             │     │
│  │  - Per-user, per-task file organization                │     │
│  │  - File upload/download handlers                       │     │
│  │  - File metadata tracking                              │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │          Database Layer (internal/db/)                 │     │
│  │  Collections:                                           │     │
│  │  - USERS: User accounts and authentication             │     │
│  │  - WORKER_REGISTRY: Worker registration data           │     │
│  │  - TASKS: Task definitions and status                  │     │
│  │  - ASSIGNMENTS: Task-to-worker assignments             │     │
│  │  - RESULTS: Task execution results and logs            │     │
│  │  - FILE_METADATA: File storage metadata                │     │
│  └────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

### Worker Node Components

```
┌─────────────────────────────────────────────────────────────────┐
│                       Worker Node                               │
│                                                                 │
│  ┌────────────────────────────────────────────────────────┐     │
│  │          gRPC Server (internal/server/)                │     │
│  │  Inbound RPCs from Master:                             │     │
│  │  - AssignTask() - Receive task assignments             │     │
│  │  - CancelTask() - Handle cancellation requests         │     │
│  │  - StreamTaskLogs() - Stream live logs to master       │     │
│  │  Outbound RPCs to Master:                              │     │
│  │  - RegisterWorker() - Register on startup              │     │
│  │  - SendHeartbeat() - Periodic health updates           │     │
│  │  - ReportTaskCompletion() - Report task results        │     │
│  │  - UploadTaskFiles() - Upload output files             │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │       Task Executor (internal/executor/)               │     │
│  │  - Docker client management                            │     │
│  │  - Image pulling from registries                       │     │
│  │  - Container creation with resource limits             │     │
│  │  - Container lifecycle management                      │     │
│  │  - Output directory handling (/output)                 │     │
│  │  - Log streaming integration                           │     │
│  │  - Container cleanup                                   │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │      Log Stream Manager (internal/logstream/)          │     │
│  │  - Real-time log broadcasting                          │     │
│  │  - Log buffer management                               │     │
│  │  - Multi-subscriber support                            │     │
│  │  - Log persistence for completed tasks                 │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │       Telemetry Monitor (internal/telemetry/)          │     │
│  │  - System metrics collection (CPU, Memory, GPU)        │     │
│  │  - Periodic heartbeat sending (5s interval)            │     │
│  │  - Running task tracking                               │     │
│  │  - Master registration on startup                      │     │
│  │  - Result reporting                                    │     │
│  └────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

### Web UI Components

```
┌─────────────────────────────────────────────────────────────────┐
│                    Web Dashboard (React/Vite)                   │
│                                                                 │
│  ┌────────────────────────────────────────────────────────┐     │
│  │                Authentication Layer                     │     │
│  │  - JWT-based authentication                            │     │
│  │  - Login/Register pages                                │     │
│  │  - Protected route wrapper                             │     │
│  │  - Auth context provider                               │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │                   Page Components                       │     │
│  │  - Dashboard: Cluster overview and statistics          │     │
│  │  - TasksPage: Task list with filtering and logs        │     │
│  │  - WorkersPage: Worker status and resource usage       │     │
│  │  - SubmitTaskPage: Task submission with tag/k-value    │     │
│  └──────────────────────┬─────────────────────────────────┘     │
│                         │                                       │
│  ┌──────────────────────▼─────────────────────────────────┐     │
│  │                   API Integration                       │     │
│  │  - Axios HTTP client for REST APIs                     │     │
│  │  - WebSocket client for real-time telemetry            │     │
│  │  - Custom hooks: useRealTimeTasks, useTelemetry        │     │
│  └────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

## Communication Flow

### 1. Worker Registration

```
Worker                    Master
  │                         │
  │──RegisterWorker────────▶│
  │   (WorkerInfo:          │
  │    worker_id,           │
  │    worker_ip,           │──┐
  │    total_cpu,           │  │ Store worker info
  │    total_memory,        │  │ in DB + memory
  │    total_storage,       │◀─┘
  │    total_gpu)           │
  │                         │
  │◀────RegisterAck─────────│
  │   (success: true)       │
  │                         │
  │◀───MasterRegister───────│
  │   (master_id,           │
  │    master_address)      │
  │                         │
```

### 2. Heartbeat Monitoring

```
Worker                    Master                TelemetryManager
  │                         │                         │
  ├──┐                      │                         │
  │  │ Every 5s             │                         │
  │◀─┘                      │                         │
  │                         │                         │
  │──SendHeartbeat─────────▶│                         │
  │   (worker_id,           │                         │
  │    cpu_usage,           │─────────────────────────▶│
  │    memory_usage,        │   Update worker         │
  │    gpu_usage,           │   telemetry data        │
  │    running_tasks[])     │                         │
  │                         │◀────────────────────────│
  │                         │                         │
  │◀────HeartbeatAck────────│                         │
  │                         │                         │
  │                         │─────WebSocket Push─────▶│ Clients
  │                         │                         │
```

### 3. Task Assignment and Execution

```
User      Master         Scheduler        Worker          Docker
  │         │               │               │                │
  │─task───▶│               │               │                │
  │  cmd    │               │               │                │
  │         │──SelectWorker▶│               │                │
  │         │   (task,      │               │                │
  │         │    workers)   │               │                │
  │         │◀──workerID────│               │                │
  │         │               │               │                │
  │         │───────AssignTask─────────────▶│                │
  │         │   (Task: task_id,             │                │
  │         │    docker_image,              │                │
  │         │    command,                   │                │
  │         │    req_cpu, req_memory,       │                │
  │         │    req_gpu, user_id,          │                │
  │         │    task_name)                 │                │
  │         │                               │                │
  │         │◀──────────TaskAck─────────────│                │
  │         │                               │                │
  │◀─ack────│                               │──Pull Image───▶│
  │         │                               │                │
  │         │                               │◀──Image────────│
  │         │                               │                │
  │         │                               │──Create Container─▶│
  │         │                               │  (with resource     │
  │         │                               │   limits)           │
  │         │                               │                │
  │         │                               │◀──Container ID──│
  │         │                               │                │
  │         │◀──StreamTaskLogs──────────────│◀──Logs─────────│
  │         │   (live streaming)            │                │
  │         │                               │                │
  │         │                               │◀──Exit Code────│
  │         │                               │                │
  │         │                               │──Cleanup───────▶│
  │         │                               │                │
  │         │◀─ReportTaskCompletion─────────│                │
  │         │   (TaskResult: task_id,       │                │
  │         │    worker_id, status,         │                │
  │         │    logs, output_files[])      │                │
  │         │                               │                │
  │         │◀──UploadTaskFiles─────────────│                │
  │         │   (FileChunk stream)          │                │
  │         │                               │                │
  │         │──────Ack─────────────────────▶│                │
  │         │                               │                │
  │◀─logs───│                               │                │
  │  output │                               │                │
  │         │                               │                │
```

### 4. Task Queuing Flow

```
User         Master              TaskQueue         Worker
  │            │                     │               │
  │──Submit───▶│                     │               │
  │   Task     │                     │               │
  │            │─Check Resources────▶│               │
  │            │                     │               │
  │            │◀─No Worker Avail────│               │
  │            │                     │               │
  │            │──Add to Queue──────▶│               │
  │            │                     │               │
  │◀─Queued────│                     │               │
  │  Position  │                     │               │
  │            │                     │               │
  │            │     [Worker becomes available]      │
  │            │                     │               │
  │            │◀──Process Queue─────│               │
  │            │                     │               │
  │            │────────AssignTask──────────────────▶│
  │            │                     │               │
```

## Data Flow

### Task Lifecycle

```
┌──────────────┐
│ Task Created │ (CLI/REST API/Web UI)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│Task Queued   │ (If no worker available)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│Task Assigned │ (Scheduler selects worker → gRPC AssignTask)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Image Pulled │ (Worker → Docker Registry)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Container    │ (Created with resource limits)
│   Created    │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Running    │ (Docker Container + Log Streaming)
└──────┬───────┘
       │
       ├─────────────┐
       │             │
       ▼             ▼
┌──────────┐  ┌──────────┐
│ Success  │  │  Failed  │
└────┬─────┘  └────┬─────┘
     │             │
     └──────┬──────┘
            │
            ▼
   ┌────────────────┐
   │ Collect Output │ (Files from /output directory)
   └────────┬───────┘
            │
            ▼
   ┌────────────────┐
   │ Upload Files   │ (Worker → Master via gRPC stream)
   └────────┬───────┘
            │
            ▼
   ┌────────────────┐
   │ Report Result  │ (Worker → Master gRPC)
   └────────┬───────┘
            │
            ▼
   ┌────────────────┐
   │ Store in DB    │ (Master → MongoDB)
   └────────────────┘
```

### File Storage Flow

```
┌─────────────────┐
│ Task Completes  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Worker collects │
│ /output files   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ UploadTaskFiles │ (gRPC streaming)
│ FileChunk:      │
│  - task_id      │
│  - user_id      │
│  - file_path    │
│  - data chunks  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Master stores   │
│ files in:       │
│ /files/{user}/  │
│   {task_id}/    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ File metadata   │
│ saved to DB     │
└─────────────────┘
```

## Network Architecture

### Port Assignments

```
┌─────────────────────────────────────────────────────────────────┐
│                  Network Layout                                 │
│                                                                 │
│  Master Node:                                                   │
│    ├─ gRPC Server:     0.0.0.0:50051                           │
│    ├─ HTTP/WS Server:  0.0.0.0:8080                            │
│    │   ├─ REST API:    /api/tasks, /api/workers, /api/files   │
│    │   ├─ Auth:        /api/auth/login, /api/auth/register    │
│    │   ├─ Telemetry:   /telemetry, /health                    │
│    │   └─ WebSocket:   /ws/telemetry, /ws/telemetry/{id}      │
│    └─ MongoDB:         localhost:27017                         │
│                                                                 │
│  Worker Node 1:                                                 │
│    └─ gRPC Server:     0.0.0.0:50052                           │
│                                                                 │
│  Worker Node 2:                                                 │
│    └─ gRPC Server:     0.0.0.0:50053                           │
│                                                                 │
│  Worker Node N:                                                 │
│    └─ gRPC Server:     0.0.0.0:5005N                           │
│                                                                 │
│  Web UI (Development):                                          │
│    └─ Vite Dev Server: localhost:3000                          │
│                                                                 │
│  External Services:                                             │
│    ├─ Docker Hub:      registry.hub.docker.com:443             │
│    └─ Docker Engine:   unix:///var/run/docker.sock             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Protocol Stack

```
┌─────────────────────────────────────────────────────────────────┐
│                    Protocol Architecture                        │
│                                                                 │
│  Layer 7 (Application):                                         │
│    ├─ gRPC (Master ↔ Worker)                                   │
│    │   └─ Protobuf messages (master_worker.proto)              │
│    ├─ HTTP/REST (Clients → Master)                             │
│    │   └─ JSON payloads                                        │
│    ├─ WebSocket (Real-time telemetry)                          │
│    │   └─ JSON messages                                        │
│    └─ JWT (Authentication tokens)                               │
│                                                                 │
│  Layer 4 (Transport):                                           │
│    ├─ HTTP/2 (gRPC)                                            │
│    └─ HTTP/1.1 + WebSocket Upgrade                             │
│                                                                 │
│  Future: TLS/mTLS for secure communication                      │
└─────────────────────────────────────────────────────────────────┘
```

## Scalability Model

### Horizontal Scaling

```
                    ┌──────────────┐
                    │    Master    │
                    │  (Stateful)  │
                    └──────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
   ┌─────────┐       ┌─────────┐       ┌─────────┐
   │Worker-1 │       │Worker-2 │  ...  │Worker-N │
   │         │       │         │       │         │
   │ 4 CPU   │       │ 8 CPU   │       │ 16 CPU  │
   │ 8 GB    │       │ 16 GB   │       │ 32 GB   │
   └─────────┘       └─────────┘       └─────────┘

   Scale by adding more workers:
   - Each worker independent
   - No inter-worker communication
   - Linear scalability
```

### Multi-Master (Future)

```
   ┌──────────┐         ┌──────────┐
   │ Master-1 │◀───────▶│ Master-2 │
   │ (Active) │  Sync   │(Standby) │
   └────┬─────┘         └────┬─────┘
        │                    │
        │     Shared State   │
        │          │         │
        └──────────┼─────────┘
                   ▼
            ┌────────────┐
            │  MongoDB   │
            │ (Replica)  │
            └────────────┘
```

## Security Architecture

### Current Implementation

```
┌─────────────────────────────────────────────────────────────────┐
│               Current Security Features                         │
│                                                                 │
│  ┌────────────────────────────────────────────────────────┐     │
│  │         JWT Authentication (Implemented)                │     │
│  │  - User registration and login                         │     │
│  │  - JWT token generation and validation                 │     │
│  │  - Configurable JWT_SECRET via environment             │     │
│  │  - Token expiration handling                           │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                 │
│  ┌────────────────────────────────────────────────────────┐     │
│  │         File Access Control (Implemented)               │     │
│  │  - Per-user file isolation                             │     │
│  │  - Task ownership verification                         │     │
│  │  - Access control on file operations                   │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                 │
│  ┌────────────────────────────────────────────────────────┐     │
│  │         CORS Configuration (Implemented)                │     │
│  │  - Configurable allowed origins                        │     │
│  │  - Preflight request handling                          │     │
│  └────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

### Future Security Enhancements

```
┌─────────────────────────────────────────────────────────────────┐
│            Planned Security Features                            │
│                                                                 │
│  Client ◀──TLS 1.3──▶ Master ◀──TLS 1.3──▶ Worker              │
│           (mTLS)                 (mTLS)                         │
│                                                                 │
│  ┌────────────────────────────────────────────────────────┐     │
│  │         Enhanced Authentication                         │     │
│  │  - API keys for programmatic access                    │     │
│  │  - Worker certificates                                 │     │
│  │  - OAuth2 integration                                  │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                 │
│  ┌────────────────────────────────────────────────────────┐     │
│  │         Authorization Layer                             │     │
│  │  - Role-based access control (RBAC)                    │     │
│  │  - Resource quotas per user                            │     │
│  │  - Task permissions and isolation                      │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Deployment Topologies

### Single Machine (Development)

```
┌────────────────────────────────────────────┐
│          localhost                          │
│                                            │
│  ┌──────────┐                              │
│  │  Master  │  127.0.0.1:50051            │
│  └──────────┘                              │
│       ▲                                    │
│       │                                    │
│  ┌────┴──────┐                             │
│  │  Worker   │  127.0.0.1:50052           │
│  └───────────┘                             │
│       │                                    │
│  ┌────▼──────┐                             │
│  │  Docker   │  /var/run/docker.sock      │
│  └───────────┘                             │
│                                            │
└────────────────────────────────────────────┘
```

### Multi-Machine (Production)

```
┌──────────────────┐
│  Master Server   │
│  10.0.1.10:50051│
└────────┬─────────┘
         │
    ┌────┴────────────────┐
    │                     │
┌───▼────────┐    ┌──────▼──────┐
│  Worker-1  │    │  Worker-2   │
│ 10.0.1.20  │    │ 10.0.1.30   │
│ :50052     │    │ :50052      │
└────────────┘    └─────────────┘
```

### Cloud Deployment

```
┌─────────────────────────────────────────────┐
│              Cloud Provider                  │
│  ┌──────────────────────────────────────┐  │
│  │         Virtual Network              │  │
│  │  ┌────────────────────────────────┐  │  │
│  │  │      Master VM                 │  │  │
│  │  │  - Public IP: xxx.xxx.xxx.xxx │  │  │
│  │  │  - Private IP: 10.0.1.10      │  │  │
│  │  └────────────────────────────────┘  │  │
│  │              ▲                        │  │
│  │              │                        │  │
│  │  ┌───────────┴───────────┐          │  │
│  │  │                       │          │  │
│  │  ▼                       ▼          │  │
│  │  ┌──────────┐     ┌──────────┐    │  │
│  │  │Worker VM1│     │Worker VM2│    │  │
│  │  │10.0.1.20 │     │10.0.1.30 │    │  │
│  │  └──────────┘     └──────────┘    │  │
│  └────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

## Future Architecture Extensions

### With AI Agent (proto/master_agent.proto)

```
┌────────────┐
│  Master    │◀─────gRPC─────┐
│   (Go)     │  FetchCluster │
└─────┬──────┘  State        │
      │                ┌─────┴──────┐
      │                │ AI Agent   │
      │                │  (Python)  │
      │                │            │
      │                │ Analyzes:  │
      │                │ - Workers  │
      │                │ - Tasks    │
      │                │ - Resources│
      │                └─────┬──────┘
      │                      │
      │◀─SubmitAssignments───┘
      │   (Task → Worker)
      ▼
┌────────────┐
│  Workers   │
│ (Pool of N)│   AI-Optimized
└────────────┘   Scheduling
```

### Web Dashboard (Implemented)

```
┌────────────────────┐
│     Browser        │
│  (React + Vite)    │
└─────────┬──────────┘
          │ HTTP/WebSocket
          ▼
┌────────────────────┐
│   Master Node      │
│   HTTP Server      │
│   (Port 8080)      │
│                    │
│  - REST API        │
│  - WebSocket       │
│  - Auth endpoints  │
└─────────┬──────────┘
          │ gRPC
          ▼
┌────────────────────┐
│   Workers          │
└────────────────────┘
```

## Monitoring Architecture (Future)

```
┌─────────────────────────────────────────┐
│          Monitoring Stack                │
│                                         │
│  Workers ──▶ Prometheus ──▶ Grafana    │
│    │            │              │        │
│    │            ▼              ▼        │
│    │         AlertManager   Dashboard   │
│    │                                    │
│    └──▶ Logs ──▶ Loki ──▶ Grafana     │
│                                         │
└─────────────────────────────────────────┘
```

---

## Project Directory Structure

```
CloudAI/
├── master/                      # Master Node (Go)
│   ├── main.go                  # Entry point
│   ├── go.mod                   # Go module definition
│   └── internal/
│       ├── cli/                 # Interactive CLI (readline-based)
│       ├── config/              # Configuration management
│       ├── db/                  # MongoDB database operations
│       │   ├── init.go          # Collection setup
│       │   ├── tasks.go         # Task CRUD
│       │   ├── workers.go       # Worker registry
│       │   ├── assignments.go   # Task assignments
│       │   ├── results.go       # Task results
│       │   ├── users.go         # User management
│       │   └── file_metadata.go # File tracking
│       ├── http/                # HTTP/WebSocket handlers
│       │   ├── task_handler.go  # Task REST API
│       │   ├── worker_handler.go# Worker REST API
│       │   ├── file_handler.go  # File REST API
│       │   ├── auth_handler.go  # JWT authentication
│       │   ├── middleware.go    # CORS, auth middleware
│       │   └── telemetry_server.go # WebSocket telemetry
│       ├── scheduler/           # Task scheduling
│       │   ├── scheduler.go     # Scheduler interface
│       │   └── round_robin.go   # Round-robin implementation
│       ├── server/              # gRPC server implementation
│       │   ├── master_server.go # Main server logic
│       │   └── log_streaming_helper.go # Log stream helpers
│       ├── storage/             # File storage service
│       │   ├── file_storage.go  # File operations
│       │   ├── file_storage_secure.go # Secure file storage
│       │   └── access_control.go# File access control
│       ├── system/              # System utilities
│       └── telemetry/           # Telemetry management
│
├── worker/                      # Worker Node (Go)
│   ├── main.go                  # Entry point
│   ├── go.mod                   # Go module definition
│   └── internal/
│       ├── executor/            # Docker task executor
│       ├── logstream/           # Log streaming
│       │   ├── log_manager.go   # Log management
│       │   └── log_broadcaster.go # Multi-client broadcast
│       ├── server/              # gRPC server
│       ├── system/              # System metrics
│       └── telemetry/           # Heartbeat sending
│
├── proto/                       # Protocol Buffers
│   ├── master_worker.proto      # Master ↔ Worker communication
│   ├── master_agent.proto       # Master ↔ AI Agent (future)
│   ├── generate.sh              # Code generation script
│   ├── pb/                      # Generated Go code
│   └── py/                      # Generated Python code
│
├── ui/                          # Web Dashboard (React)
│   ├── src/
│   │   ├── api/                 # API clients
│   │   ├── components/          # React components
│   │   ├── context/             # Auth context
│   │   ├── hooks/               # Custom hooks
│   │   ├── pages/               # Page components
│   │   ├── styles/              # CSS styles
│   │   └── utils/               # Utilities
│   ├── package.json             # npm dependencies
│   └── vite.config.js           # Vite configuration
│
├── database/                    # MongoDB Docker setup
│   └── docker-compose.yml       # MongoDB container
│
├── docs/                        # Documentation
│   ├── ARCHITECTURE.md          # This file
│   ├── DOCUMENTATION.md         # Complete reference
│   ├── GETTING_STARTED.md       # Quick start guide
│   └── EXAMPLE.md               # Usage examples
│
├── Makefile                     # Build automation
├── requirements.txt             # Python dependencies (for future agent)
├── runMaster.sh                 # Master startup script
└── runWorker.sh                 # Worker startup script
```

---

This architecture supports:

- Horizontal scaling (add more workers)
- Fault tolerance (worker failures handled gracefully)
- Real-time monitoring (heartbeat system + WebSocket)
- Flexible deployment (local to cloud)
- Authentication (JWT-based user auth)
- File management (secure upload/download)
- Task queuing (when no workers available)
- Web dashboard (React-based UI)
- High availability (multi-master) - Planned
- TLS encryption (secure communication) - Planned
- Advanced scheduling (AI agent integration) - Planned
