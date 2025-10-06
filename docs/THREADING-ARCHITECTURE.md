# Threading Architecture - CloudAI Scheduler

This document describes the concurrent threading architecture for each main service in the CloudAI distributed scheduling system.

---

## Overview

The CloudAI system consists of 4 main services, each with its own threading model:

1. **Master Node** (Go) - Central coordinator
2. **Worker Node** (Go) - Task executor
3. **Client** (Go) - Task submitter
4. **Planner Service** (Python) - AI-based scheduler

---

## 1. Master Node Threading Architecture

### Main Thread
- **Purpose**: Initializes components and manages graceful shutdown
- **Lifecycle**: Runs for entire process lifetime
- **Operations**:
  - Initialize Registry, TaskQueue, gRPC server
  - Listen for SIGINT/SIGTERM signals
  - Coordinate graceful shutdown

### gRPC Server Thread Pool
- **Purpose**: Handle incoming client and worker requests
- **Concurrency Model**: Multiple goroutines (one per request)
- **Operations**:
  - `SubmitTask()` - Accept task submissions
  - `GetTaskStatus()` - Query task state
  - `CancelTask()` - Cancel pending/running tasks
  - `RegisterWorker()` - Register new workers
  - `Heartbeat()` - Process worker heartbeats
  - `ListWorkers()` - Return active workers
  - `ReportTaskCompletion()` - Handle task completion

### Cleanup Loop (Background Goroutine)
- **Purpose**: Periodic maintenance of worker registry
- **Frequency**: Every 10 seconds
- **Operations**:
  - `CleanupStaleWorkers()` - Remove workers with no heartbeat for 30s
  - `CleanupExpiredReservations()` - Release expired resource reservations

### Scheduler Thread (Planned - Not Yet Implemented)
- **Purpose**: Periodic task scheduling cycle
- **Frequency**: Every 5 seconds (configurable)
- **Operations**:
  - Fetch pending tasks from queue
  - Get worker snapshot from registry
  - Call Planner service via gRPC
  - Assign tasks to workers based on plan
  - Update task status to SCHEDULED

### Thread Relationships & Synchronization

```
Main Thread
    ├── Spawns: gRPC Server Thread Pool
    │   └── Shares: TaskQueue (mutex-protected)
    │   └── Shares: Registry (RWMutex-protected)
    │
    ├── Spawns: Cleanup Loop
    │   └── Accesses: Registry (RWMutex-protected)
    │
    └── Spawns: Scheduler Thread (future)
        └── Accesses: TaskQueue (mutex-protected)
        └── Accesses: Registry (RWMutex-protected)
        └── Calls: Planner Service (gRPC)
```

### Shared Resources & Protection
- **TaskQueue**: Protected by `sync.Mutex` + `sync.Cond` for blocking operations
- **Registry**: Protected by `sync.RWMutex` (multiple readers, single writer)
- **gRPC Server**: Thread-safe by design

---

## 2. Worker Node Threading Architecture

### Main Thread
- **Purpose**: Initialize worker and coordinate lifecycle
- **Operations**:
  - Load worker configuration
  - Register with Master node
  - Manage graceful shutdown

### Heartbeat Thread (Background Goroutine)
- **Purpose**: Send periodic heartbeats to Master
- **Frequency**: Every 5-10 seconds
- **Operations**:
  - Collect current resource usage
  - Send heartbeat to Master via gRPC
  - Report running task IDs
  - Update free CPU/Memory/GPU metrics

### Task Execution Thread Pool
- **Purpose**: Execute assigned tasks concurrently
- **Concurrency Model**: One goroutine per task (configurable max)
- **Operations**:
  - Receive task assignment from Master
  - Launch container/VM for task execution
  - Monitor task progress
  - Capture stdout/stderr logs
  - Report completion/failure to Master

### Resource Monitor Thread (Background Goroutine)
- **Purpose**: Track local resource usage
- **Frequency**: Continuous monitoring
- **Operations**:
  - Monitor CPU utilization
  - Track memory consumption
  - Check GPU availability
  - Update free resource counters
  - Detect resource anomalies

### Container/VM Manager Thread
- **Purpose**: Manage Docker containers or VMs
- **Operations**:
  - Launch new containers/VMs
  - Monitor container health
  - Handle container cleanup
  - Collect execution metrics

### Thread Relationships & Synchronization

```
Main Thread
    ├── Spawns: Heartbeat Thread
    │   └── Reads: Resource Metrics (atomic or mutex)
    │   └── Calls: Master gRPC
    │
    ├── Spawns: Resource Monitor Thread
    │   └── Updates: Resource Metrics (atomic or mutex)
    │
    ├── Spawns: Task Execution Pool
    │   └── Uses: Container/VM Manager
    │   └── Updates: Running Tasks List (mutex)
    │   └── Calls: Master gRPC (completion reports)
    │
    └── Spawns: Container/VM Manager Thread
        └── Manages: Docker/KVM processes
```

### Shared Resources & Protection
- **Resource Metrics**: Protected by atomic operations or mutex
- **Running Tasks Map**: Protected by `sync.RWMutex`
- **Container Manager**: Thread-safe with internal locking

---

## 3. Test Client Threading Architecture

### Main Thread
- **Purpose**: Sequential test execution
- **Operations**:
  - Connect to Master via gRPC
  - Execute test cases sequentially
  - Verify responses
  - Report test results

### No Concurrent Threads
- The test client is **single-threaded** and executes operations **sequentially**
- Each test waits for completion before proceeding to the next

### Operations Flow (Sequential)
1. Submit Task
2. Query Task Status
3. Register Worker
4. Send Heartbeat
5. List Workers
6. Submit Another Task
7. Report Task Completion
8. Verify Status Update
9. Cancel Task

### Why Single-Threaded?
- Simplifies test execution and debugging
- Ensures deterministic test order
- Easy to verify causal relationships
- No race conditions in tests

---

## 4. Planner Service Threading Architecture (Python)

### Main Thread
- **Purpose**: Initialize gRPC server and manage lifecycle
- **Operations**:
  - Load AI models (A*, OR-Tools, ML predictor)
  - Start gRPC server
  - Handle shutdown signals (Ctrl+C)

### gRPC Thread Pool (ThreadPoolExecutor)
- **Purpose**: Handle concurrent planning requests from Master
- **Pool Size**: 8 worker threads (configurable via `max_workers=8`)
- **Operations**:
  - `Plan()` - Compute optimal task-to-worker assignments
  - Process planning requests in parallel

### Planning Algorithm Threads (Within Request Handler)
- **A* Search Thread**: Explores state space for optimal schedule
- **OR-Tools Solver Thread**: Runs constraint programming solver
- **Predictor Thread**: Runs ML inference for task duration prediction
- **Replanner Thread**: Handles replanning on failures

### Thread Relationships & Synchronization

```
Main Thread
    └── Spawns: gRPC Server
        └── Uses: ThreadPoolExecutor(max_workers=8)
            └── Thread 1: Plan() request handler
            │   ├── Calls: A* Planner
            │   ├── Calls: OR-Tools Solver
            │   └── Calls: ML Predictor
            │
            └── Thread 2-8: Additional concurrent requests
```

### Concurrent Request Handling
- **Multiple Masters**: Can handle requests from multiple Master nodes
- **Parallel Planning**: Up to 8 planning requests processed simultaneously
- **Thread Safety**: Each planning request is independent (no shared state)

### Why ThreadPoolExecutor?
- Python gRPC server uses threading for I/O-bound operations
- AI planning algorithms (A*, OR-Tools) release GIL during computation
- Allows parallel request processing despite Python's GIL

---

## Inter-Service Threading Relationships

### Master ↔ Planner Communication
```
[Master Scheduler Thread] 
    └── gRPC Call → [Planner Thread Pool]
        └── Returns Plan → [Master Scheduler Thread]
            └── Updates TaskQueue (mutex-protected)
```

### Client ↔ Master Communication
```
[Client Main Thread]
    └── gRPC Call → [Master gRPC Thread Pool]
        └── Accesses TaskQueue (mutex)
        └── Returns Response → [Client Main Thread]
```

### Worker ↔ Master Communication
```
[Worker Heartbeat Thread]
    └── gRPC Call → [Master gRPC Thread Pool]
        └── Updates Registry (RWMutex)
        └── Returns ACK → [Worker Heartbeat Thread]

[Worker Task Thread]
    └── gRPC Call (completion) → [Master gRPC Thread Pool]
        └── Updates TaskQueue (mutex)
        └── Releases Registry resources (RWMutex)
```

---

## Critical Concurrency Patterns

### 1. Producer-Consumer (Master TaskQueue)
- **Producers**: Client gRPC handlers → Enqueue tasks
- **Consumers**: Scheduler thread → Dequeue tasks
- **Synchronization**: Mutex + Condition variable for blocking dequeue

### 2. Reader-Writer (Master Registry)
- **Readers**: Scheduler, gRPC handlers (GetSnapshot)
- **Writers**: Heartbeat handlers, Cleanup loop
- **Synchronization**: RWMutex allows multiple concurrent readers

### 3. Worker Pool (Planner Service)
- **Pool**: ThreadPoolExecutor with fixed size
- **Tasks**: Planning requests from Master
- **Synchronization**: Internal queue managed by ThreadPoolExecutor

### 4. Parallel Task Execution (Worker)
- **Pool**: Goroutine per task (dynamic)
- **Resource Limiting**: Semaphore or channel to limit concurrency
- **Cleanup**: WaitGroup for graceful shutdown

---

## Thread Safety Guarantees

### Master Node
✅ **TaskQueue**: Mutex-protected, safe for concurrent access  
✅ **Registry**: RWMutex-protected, optimized for read-heavy workloads  
✅ **gRPC Handlers**: Stateless, no shared mutable state  
✅ **Cleanup Loop**: Uses proper locking when accessing Registry  

### Worker Node
✅ **Resource Metrics**: Atomic operations or mutex-protected  
✅ **Running Tasks**: RWMutex-protected map  
✅ **Heartbeat**: No shared state, independent goroutine  
✅ **Task Execution**: Isolated goroutines with proper cleanup  

### Planner Service
✅ **Request Handlers**: Stateless, independent threads  
✅ **Thread Pool**: Managed by ThreadPoolExecutor  
✅ **Planning Algorithms**: Pure functions, no shared state  

### Client
✅ **Single-threaded**: No concurrency issues  

---

## Performance Considerations

### Master Node
- **gRPC Thread Pool**: Auto-scales based on request load
- **Registry RWMutex**: Optimized for frequent reads (GetSnapshot)
- **TaskQueue Mutex**: Minimized lock contention with batch operations

### Worker Node
- **Heartbeat Interval**: Balances responsiveness vs. overhead (5-10s)
- **Task Concurrency**: Limited to prevent resource exhaustion
- **Resource Monitoring**: Low-frequency polling to reduce CPU usage

### Planner Service
- **Thread Pool Size**: Fixed at 8 to prevent oversubscription
- **GIL Release**: Heavy computation (NumPy, OR-Tools) releases GIL
- **Request Timeout**: Prevents slow planning from blocking others

---

## Future Enhancements

### Master Node
- [ ] Implement scheduler thread with configurable interval
- [ ] Add metrics collection thread for monitoring
- [ ] Implement distributed coordination (Raft/etcd) for HA

### Worker Node
- [ ] Dynamic thread pool sizing based on load
- [ ] Priority-based task scheduling within worker
- [ ] Advanced resource isolation with cgroups

### Planner Service
- [ ] Async/await with asyncio for better concurrency
- [ ] GPU-accelerated planning with CUDA threads
- [ ] Distributed planning across multiple planner instances

---

## Debugging & Monitoring

### Thread Inspection
- **Go**: Use `runtime.NumGoroutine()` to track goroutine count
- **Python**: Use `threading.active_count()` to track threads

### Deadlock Detection
- **Go**: Enable race detector with `-race` flag
- **Python**: Use `faulthandler` to dump thread stacks

### Performance Profiling
- **Go**: Use `pprof` for CPU/memory/goroutine profiling
- **Python**: Use `cProfile` or `py-spy` for thread profiling

---

## Summary

| Service | Main Threads | Concurrency Model | Shared Resources |
|---------|-------------|-------------------|------------------|
| **Master** | gRPC Pool + Cleanup + Scheduler | Goroutines + Channels | TaskQueue (Mutex), Registry (RWMutex) |
| **Worker** | Heartbeat + Tasks + Monitor | Goroutine Pool | Resource Metrics, Running Tasks |
| **Client** | Main only | Sequential | None |
| **Planner** | ThreadPoolExecutor (8 threads) | Thread Pool | None (stateless) |

