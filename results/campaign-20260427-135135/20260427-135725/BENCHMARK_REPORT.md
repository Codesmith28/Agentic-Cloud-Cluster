# Benchmark Execution Report

**Generated**: 2026-04-27 08:27:26 UTC
**Master URL**: http://localhost:8080

## Model

- **path**: `agentic_scheduler/models/ppo_latest.pt`
- **size_bytes**: `234358`
- **sha256**: `509097eeae3f85a0...`
- **version**: `v007`

## Cluster Topology

- **Workers registered**: 3

| Worker ID | CPU | Memory (GB) | Storage (GB) | Active | Status |
|-----------|-----|-------------|-------------|--------|--------|
| worker-small | ? | ? | ? | ✓ | unknown |
| worker-medium | ? | ? | ? | ✓ | unknown |
| worker-large | ? | ? | ? | ✓ | unknown |

## Worker Utilisation

| Worker | Tasks Assigned | Completed | Failed | Task Types |
|--------|---------------|-----------|--------|------------|
| unassigned | 150 | 150 | 0 | cpu-heavy(45), cpu-light(45), memory-heavy(30), mixed(30) |
| **Total** | **150** | **150** | **0** | |

## Campaign Summary

- **Started**: 2026-04-27T08:21:35Z
- **Finished**: 2026-04-27T08:27:25Z
- **Duration**: 349.96s
- **Scenarios run**: 9

### Scheduler Comparison

| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |
|-----------|-------|-----------|--------|--------------|-------------|
| RR | 50 | 50 | 0 | 100.0% | 46.51s |
| RTS | 50 | 50 | 0 | 100.0% | 25.41s |
| PPO | 50 | 50 | 0 | 100.0% | 35.5s |

**Best scheduler**: **RR**

### Scenario Details

#### baseline / RR / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.23s

#### baseline / RTS / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 27.29s

#### baseline / PPO / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.2s

#### burst / RR / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 36.28s

#### burst / RTS / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 12.16s

#### burst / PPO / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.23s

#### overload / RR / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 85.02s

#### overload / RTS / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 36.79s

#### overload / PPO / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 70.08s

## Verdict

**✅ ALL TASKS PASSED**

- Total tasks seen by cluster: 150
- Completed: 150
- Failed: 0
- Workers used: 1
