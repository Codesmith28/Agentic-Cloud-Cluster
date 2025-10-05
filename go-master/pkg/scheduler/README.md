# Scheduler Package

A modular scheduling system for CloudAI that orchestrates task scheduling using different algorithms.

## Overview

The scheduler package implements **US-2.1: As a master node, I need a greedy scheduler to assign pending tasks to available workers**. It's designed to be extensible for future scheduling algorithms (e.g., agentic/ML-based schedulers).

## Architecture

### Modular Design

```
scheduler/
├── scheduler.go      # Main scheduler - orchestrates and delegates to algorithms
├── greedy.go         # Greedy best-fit algorithm (helper/extension)
├── scheduler_test.go # All tests for the scheduler package
└── README.md         # This file
```

### Design Philosophy

- **`scheduler.go`**: Main scheduler that coordinates scheduling. It makes calls to different scheduling algorithms based on configuration (currently uses greedy, will support agentic in future)
- **`greedy.go`**: Extension/helper file containing the greedy algorithm implementation
- **Future**: `agentic.go` will contain ML-based scheduling logic

This design keeps the codebase modular and makes it easy to switch between or combine different scheduling strategies.

## Usage

### Basic Usage

```go
import (
    "context"
    "github.com/Codesmith28/CloudAI/pkg/scheduler"
    "github.com/Codesmith28/CloudAI/pkg/taskqueue"
    "github.com/Codesmith28/CloudAI/pkg/workerregistry"
)

// Create dependencies
taskQueue := taskqueue.NewTaskQueue()
registry := workerregistry.NewRegistry()

// Create scheduler (currently uses greedy algorithm)
sched := scheduler.NewScheduler(taskQueue, registry)

// Start scheduling loop
ctx := context.Background()
go sched.Start(ctx)
```

### Configuration

```go
// Change scheduling frequency
sched.SetInterval(10 * time.Second)

// Change batch size
sched.SetBatchSize(20)

// Run single scheduling cycle
err := sched.ScheduleOnce(ctx)
```

## Current Implementation: Greedy Algorithm

The scheduler currently uses a greedy best-fit algorithm implemented in `greedy.go`:

### Algorithm Strategy

For each pending task:
1. Get all workers that have sufficient resources (CPU, memory, GPU)
2. Among those, select the one with **maximum free CPU**
3. Reserve resources on that worker
4. Mark task as SCHEDULED

**Why maximum free CPU?** Reduces fragmentation - larger workers stay available for larger tasks.

### Example Flow

```
Task: CPU=2, Mem=4GB
Workers:
  - worker-1: Free CPU=2, Free Mem=8GB
  - worker-2: Free CPU=6, Free Mem=12GB  ← Selected (most free CPU)
  - worker-3: Free CPU=4, Free Mem=8GB

Result: Task assigned to worker-2
```

## Future: Agentic Scheduler

In future sprints, we'll add `agentic.go` with ML-based scheduling:

```go
// Future: agentic.go
func agenticFindBestWorker(task *pb.Task, workers []*pb.Worker) (string, error) {
    // Use ML model to predict best worker based on:
    // - Historical performance
    // - Task characteristics
    // - Worker capabilities
    // - System load patterns
}

// scheduler.go will be updated to:
func (s *Scheduler) ScheduleOnce(ctx context.Context) error {
    // ...
    
    // Check if agentic scheduler is available
    if s.useAgentic && agenticSchedulerReady() {
        workerID, err := agenticFindBestWorker(task, workers)
    } else {
        // Fallback to greedy
        workerID, err := greedyFindBestWorker(task, workers)
    }
    
    // ...
}
```

## Testing

Run all tests:
```bash
cd go-master
go test ./pkg/scheduler/ -v
```

Run specific test:
```bash
go test ./pkg/scheduler/ -run TestGreedy -v
```

### Test Coverage

All tests are in `scheduler_test.go`:
- Scheduler creation and configuration
- Resource checking (`canFit`)
- Greedy algorithm behavior
- Edge cases (no suitable workers, etc.)

## API Reference

### Main Scheduler

```go
type Scheduler struct {
    // private fields
}

// Create new scheduler
func NewScheduler(tq *TaskQueue, reg *Registry) *Scheduler

// Start scheduling loop
func (s *Scheduler) Start(ctx context.Context) error

// Run single scheduling cycle
func (s *Scheduler) ScheduleOnce(ctx context.Context) error

// Configure scheduling
func (s *Scheduler) SetInterval(interval time.Duration)
func (s *Scheduler) SetBatchSize(size int)
```

### Helper Functions

```go
// Check if task fits on worker (package-private)
func canFit(task *pb.Task, worker *pb.Worker) bool

// Greedy algorithm (package-private, in greedy.go)
func greedyFindBestWorker(task *pb.Task, workers []*pb.Worker) (string, error)
```

## Design Principles

1. **Main orchestrator**: `scheduler.go` is the main scheduler that delegates to different algorithms
2. **Modularity**: Algorithm implementations are in separate files as extensions
3. **Extensibility**: Easy to add new algorithms (just add new file + update scheduler.go)
4. **Minimal dependencies**: Only taskqueue and registry
5. **Simple and clear**: Easy to understand and maintain

## Performance

### Greedy Scheduler
- **Time Complexity**: O(n*m) per cycle, where n = pending tasks, m = workers
- **Space Complexity**: O(1) - no additional data structures
- **Throughput**: Configurable (default: 10 tasks per 5 seconds = 2 tasks/sec)

For a system with 100 workers and 1000 pending tasks:
- Each cycle: 10 tasks × 100 workers = 1000 comparisons
- Time: < 1ms for 1000 comparisons

## File Structure Summary

| File | Purpose | Lines |
|------|---------|-------|
| scheduler.go | Main scheduler (orchestrator) | ~130 |
| greedy.go | Greedy algorithm (extension) | ~35 |
| scheduler_test.go | All tests | ~260 |
| README.md | Documentation | This file |

**Total Implementation**: ~165 lines of code (excluding tests)
**Total Tests**: ~260 lines of code

## Adding New Scheduling Algorithms

To add a new algorithm (e.g., `agentic.go`):

1. Create `agentic.go` with your algorithm:
```go
func agenticFindBestWorker(task *pb.Task, workers []*pb.Worker) (string, error) {
    // Your ML-based logic here
}
```

2. Update `scheduler.go` to use it:
```go
func (s *Scheduler) ScheduleOnce(ctx context.Context) error {
    // ...
    if s.useAgenticScheduler {
        workerID, err := agenticFindBestWorker(task, workers)
    } else {
        workerID, err := greedyFindBestWorker(task, workers)
    }
    // ...
}
```

3. Add tests to `scheduler_test.go`

That's it! The modular design makes it easy to extend.
