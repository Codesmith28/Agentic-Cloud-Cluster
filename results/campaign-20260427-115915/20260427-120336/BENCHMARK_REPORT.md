# Benchmark Execution Report

**Generated**: 2026-04-27 06:33:37 UTC
**Master URL**: http://localhost:8080

## Model

- **path**: `agentic_scheduler/models/ppo_latest.pt`
- **size_bytes**: `234358`
- **sha256**: `728b89b335b980fa...`
- **version**: `v003`

## Cluster Topology

- **Workers registered**: 3

| Worker ID | CPU | Memory (GB) | Storage (GB) | Active | Status |
|-----------|-----|-------------|-------------|--------|--------|
| worker-medium | ? | ? | ? | ✓ | unknown |
| worker-large | ? | ? | ? | ✓ | unknown |
| worker-small | ? | ? | ? | ✓ | unknown |

## Worker Utilisation

| Worker | Tasks Assigned | Completed | Failed | Task Types |
|--------|---------------|-----------|--------|------------|
| unassigned | 100 | 100 | 0 | cpu-heavy(30), cpu-light(30), memory-heavy(20), mixed(20) |
| **Total** | **100** | **100** | **0** | |

## Campaign Summary

- **Started**: 2026-04-27T06:29:15Z
- **Finished**: 2026-04-27T06:33:36Z
- **Duration**: 261.6s
- **Scenarios run**: 9

### Scheduler Comparison

| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |
|-----------|-------|-----------|--------|--------------|-------------|
| RR | 0 | 0 | 0 | 0.0% | 0.0s |
| RTS | 50 | 50 | 0 | 100.0% | 45.52s |
| PPO | 50 | 50 | 0 | 100.0% | 32.51s |

**Best scheduler**: **RTS**

### Scenario Details

#### baseline / RR / heterogeneous-smoke

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### baseline / RTS / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 27.21s

#### baseline / PPO / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.23s

#### burst / RR / heterogeneous-smoke

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### burst / RTS / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 63.36s

#### burst / PPO / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 39.36s

#### overload / RR / heterogeneous-smoke

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### overload / RTS / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 46.0s

#### overload / PPO / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 39.95s

## Verdict

**✅ ALL TASKS PASSED**

- Total tasks seen by cluster: 100
- Completed: 100
- Failed: 0
- Workers used: 1
