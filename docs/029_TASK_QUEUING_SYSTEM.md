# Task Queuing and Scheduling System

## Overview

The CloudAI Master Node implements a comprehensive task queuing and scheduling system. **ALL tasks submitted to the system go through a queue first**, and then a built-in scheduler automatically selects the best available worker based on resource availability and utilization.

This design ensures optimal resource utilization, fair task distribution, and automatic handling of resource constraints.

## Status: ✅ Implemented

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Master Server                         │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  ┌──────────────────────────────────────────────────┐   │
│  │           SubmitTask() Entry Point                │   │
│  └────────────────┬─────────────────────────────────┘   │
│                   │                                       │
│                   │  ALL tasks go here first             │
│                   ▼                                       │
│           ┌──────────────────┐                           │
│           │  In-Memory Queue │                           │
│           │   [QueuedTask]   │                           │
│           │   Status: queued │                           │
│           └────────┬─────────┘                           │
│                    │                                      │
│                    │  Checked every 5s                    │
│                    ▼                                      │
│           ┌────────────────────┐                         │
│           │ Queue Processor     │                         │
│           │  (Background)       │                         │
│           └────────┬───────────┘                         │
│                    │                                      │
│                    │  For each task                       │
│                    ▼                                      │
│           ┌────────────────────┐                         │
│           │    Scheduler        │                         │
│           │ (First-Fit)         │                         │
│           │ • Find best worker  │                         │
│           │ • Check resources   │                         │
│           │ • Balance load      │                         │
│           └────────┬───────────┘                         │
│                    │                                      │
│                    │  Worker selected                     │
│                    ▼                                      │
│           ┌────────────────────┐                         │
│           │ assignTaskToWorker  │                         │
│           │ • Connect via gRPC  │                         │
│           │ • Allocate resources│                         │
│           │ • Update status     │                         │
│           └────────┬───────────┘                         │
│                    │                                      │
│                    ▼                                      │
│           Worker Executes Task                           │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

## Key Design Principles

### 1. **Queue-First Architecture**
- Users submit tasks to the system, not to specific workers
- Scheduler automatically selects optimal worker
- Abstracts worker management from users
- Enables intelligent load balancing

### 2. **Automatic Scheduling**
- First-Fit algorithm with utilization awareness
- Considers resource availability and current load
- Handles worker failures gracefully
- No user intervention required

### 3. **Resource Management**
- Prevents oversubscription
- Tracks allocated vs available resources
- Automatic retry on resource constraints
- Fair resource distribution

## Usage

### Submitting Tasks (New Workflow)

**Command**:
```bash
master> task <docker_image> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]
```

**Example**:
```bash
master> task docker.io/user/sample-task:latest -cpu_cores 2.0 -mem 1.0
```

**Note**: No worker ID required! The scheduler selects the best worker automatically.

### Viewing Queue

```bash
master> queue
```

Shows all pending tasks with:
- Task ID
- Assigned worker (or "Waiting for scheduler")
- Resource requirements
- Time in queue
- Retry attempts
- Status/error messages

## Benefits

✅ **Simplified User Experience** - No need to know worker IDs or capacities
✅ **Optimal Resource Utilization** - Scheduler balances load automatically
✅ **Fault Tolerance** - Automatic retry and failover
✅ **Scalability** - Easy to add new workers without configuration changes
✅ **Fairness** - FIFO processing with resource-aware assignment

## Related Documentation

- [Task Queuing Quick Reference](TASK_QUEUING_QUICK_REF.md)
- [Task Queuing Implementation Summary](TASK_QUEUING_IMPLEMENTATION_SUMMARY.md)
- [Task Queuing Testing Guide](TASK_QUEUING_TESTING.md)
