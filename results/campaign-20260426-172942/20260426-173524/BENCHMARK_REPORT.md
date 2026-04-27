    # Benchmark Execution Report

**Generated**: 2026-04-26 12:05:24 UTC
**Master URL**: http://localhost:8080

## Model

- **path**: `agentic_scheduler/models/ppo_latest.pt`
- **size_bytes**: `234806`
- **sha256**: `8bc7a2c5b3b25c0f...`
- **version**: `v001`

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
| unassigned | 200 | 200 | 0 | cpu-heavy(60), cpu-light(60), memory-heavy(40), mixed(40) |
| **Total** | **200** | **200** | **0** | |

## Campaign Summary

- **Started**: 2026-04-26T11:59:43Z
- **Finished**: 2026-04-26T12:05:24Z
- **Duration**: 341.07s
- **Scenarios run**: 12

### Scheduler Comparison

| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |
|-----------|-------|-----------|--------|--------------|-------------|
| RR | 50 | 50 | 0 | 100.0% | 30.16s |
| RTS | 50 | 50 | 0 | 100.0% | 26.16s |
| PPO-pretrained | 50 | 50 | 0 | 100.0% | 26.16s |
| PPO-adapted | 50 | 50 | 0 | 100.0% | 31.2s |

**Best scheduler**: **RR**

### Scenario Details

#### baseline / RR / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 15.06s

#### baseline / RTS / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.06s

#### baseline / PPO-pretrained / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.09s

#### baseline / PPO-adapted / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 30.1s

#### burst / RR / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 24.08s

#### burst / RTS / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 21.09s

#### burst / PPO-pretrained / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 21.08s

#### burst / PPO-adapted / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.07s

#### overload / RR / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 51.35s

#### overload / RTS / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 39.32s

#### overload / PPO-pretrained / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 39.32s

#### overload / PPO-adapted / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 45.43s

## Verdict

**✅ ALL TASKS PASSED**

- Total tasks seen by cluster: 200
- Completed: 200
- Failed: 0
- Workers used: 1
