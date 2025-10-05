# Task Queue Implementation

## Overview

This package implements a **priority-based task queue** with deadline awareness for the CloudAI scheduler. It uses an in-memory heap data structure for efficient priority ordering.

## Why In-Memory Queue (Not Kafka)?

### Decision Rationale

✅ **In-Memory Priority Queue is the right choice because:**

1. **Architecture Fit**
   - Single Master Node coordination pattern
   - CouchDB already provides persistence
   - Queue is internal to Master - not a distributed messaging layer

2. **Performance**
   - O(log n) enqueue/dequeue with heap
   - Priority ordering with deadline awareness
   - Synchronous, transactional task flow

3. **Complexity**
   - Simple to implement and maintain
   - No additional infrastructure dependencies
   - Fits educational/research project scope

❌ **Kafka would be overkill because:**
- Designed for high-throughput streaming (millions of msgs/sec)
- Adds operational complexity (ZooKeeper, brokers, topics)
- Doesn't naturally support priority ordering
- This system needs <1000 tasks/sec with priority scheduling

### When You WOULD Need Kafka:
- Multiple Master nodes requiring distributed consensus
- Event streaming across many services
- High-throughput task ingestion (>100k tasks/sec)
- Complex event-driven workflows

## Design

### Priority Rules (implemented in `Less()`)

Tasks are ordered by:
1. **Priority** (higher priority first)
2. **Deadline** (earlier deadline first, if same priority)
3. **FIFO** (first-in-first-out, if same priority and no deadlines)

### Thread Safety

- All operations are protected by `sync.RWMutex`
- Read operations use `RLock()` for concurrent reads
- Write operations use `Lock()` for exclusive access
- Condition variable (`sync.Cond`) for blocking `WaitForTasks()`

### Task Lifecycle

```
PENDING → SCHEDULED → RUNNING → COMPLETED/FAILED
```

Failed tasks can be automatically re-queued for retry.

## API

### Core Operations

```go
// Create new queue
queue := NewTaskQueue()

// Add task
err := queue.Enqueue(task)

// Get batch of highest priority tasks
tasks := queue.DequeueBatch(10)

// Check pending tasks (non-destructive)
pending := queue.PeekPending()

// Update task status
err := queue.UpdateStatus("task-123", pb.TaskStatus_RUNNING)

// Remove task
err := queue.Remove("task-123")

// Get queue size
size := queue.Size()

// Block until tasks available
queue.WaitForTasks()
```

## Persistence Strategy

The queue is **ephemeral** (in-memory), but durability is achieved through:

1. **CouchDB Persistence** - All tasks are persisted to CouchDB
2. **Recovery on Restart** - Master reloads pending tasks from CouchDB on startup
3. **Working Set** - The in-memory queue acts as a hot working set for the scheduler

This hybrid approach provides:
- Fast in-memory operations during normal operation
- Durability through CouchDB
- Recovery after Master node restarts

## Testing

Run tests:
```bash
cd go-master
go test -v ./pkg/taskqueue
```

Test coverage includes:
- Priority ordering
- Deadline awareness
- FIFO for same priority
- Thread safety (race detector)
- Status updates
- Task removal
- Failed task re-queuing

## Performance Characteristics

| Operation | Time Complexity | Notes |
|-----------|----------------|-------|
| Enqueue | O(log n) | Heap insertion |
| DequeueBatch | O(k log n) | k tasks removed |
| PeekPending | O(n) | Full scan |
| UpdateStatus | O(1) | Hash map lookup |
| Remove | O(log n) | Heap removal |
| GetStatus | O(1) | Hash map lookup |
| Size | O(1) | Direct access |

## Future Enhancements (if needed)

If you later need distributed queues:

1. **Phase 1: Add CouchDB Sync**
   - Write-through to CouchDB on enqueue
   - Periodic sync of status updates

2. **Phase 2: Multi-Master Support**
   - Use CouchDB changes feed for queue synchronization
   - Leader election for single scheduler

3. **Phase 3: External Queue (only if scaling beyond 10k tasks/sec)**
   - Consider Redis with sorted sets (simpler than Kafka)
   - Or cloud-native solutions (AWS SQS, Google Pub/Sub)

## Integration with Master

```go
// In master/main.go
taskQueue := taskqueue.NewTaskQueue()

// Client submits task
masterAPI.SubmitTask(task)
  → taskQueue.Enqueue(task)
  → persistence.SaveTask(task)

// Scheduler loop
tasks := taskQueue.DequeueBatch(100)
plan := planner.Plan(tasks, workers)
executor.EnactPlan(plan)

// Worker reports completion
worker.ReportCompletion(taskID, status)
  → taskQueue.UpdateStatus(taskID, status)
  → persistence.UpdateTask(taskID, status)
```

## References

- Sprint Plan: `Sprint.md` - Task 1.2
- Architecture: `docs/architecture.md`
- Low-Level Design: `docs/low_level_design.md`
