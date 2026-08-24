# Proto Definitions

This directory contains the gRPC protocol buffer definitions for Agentic Cloud Cluster.

## Proto Files

- **`master_worker.proto`**: Communication between Master (Go) ↔ Worker (Go)
  - Worker registration
  - Heartbeat/telemetry reporting
  - Task assignment & cancellation
  - Task completion results & log streaming

- **`ppo_scheduler.proto`**: Communication between Master (Go) ↔ PPO Scheduler (Python)
  - Scheduling decision exchange
  - Real-time task outcome reporting

## Generate Code

```bash
make proto
```

This generates:
- **Go code** in `./pb/` directory
- **Python code** in `agentic_scheduler/proto/` directory
