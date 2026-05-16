# Benchmark Execution Report

**Generated**: 2026-04-27 17:06:03 UTC
**Master URL**: http://localhost:8080

## Model

- **path**: `agentic_scheduler/models/ppo_latest.pt`
- **size_bytes**: `234358`
- **sha256**: `8ff873842c604c92...`
- **version**: `v012`

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
| unassigned | 510 | 420 | 90 | cpu-heavy(150), cpu-light(135), memory-heavy(120), mixed(105) |
| **Total** | **510** | **420** | **90** | |

## Campaign Summary

- **Started**: 2026-04-27T17:02:58Z
- **Finished**: 2026-04-27T17:06:02Z
- **Duration**: 183.49s
- **Scenarios run**: 9

### Scheduler Comparison

| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |
|-----------|-------|-----------|--------|--------------|-------------|
| RR | 30 | 10 | 20 | 33.3% | 17.63s |
| RTS | 30 | 10 | 20 | 33.3% | 17.63s |
| PPO | 30 | 10 | 20 | 33.3% | 16.63s |

**Best scheduler**: **RR**

### Scenario Details

#### baseline / RR / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 14.69s

#### baseline / RTS / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 11.68s

#### baseline / PPO / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 11.66s

#### burst / RR / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 12.2s

#### burst / RTS / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 12.2s

#### burst / PPO / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 12.16s

#### overload / RR / memory-pressure

- Submitted: 18
- Completed: 6
- Failed: 12
- Success Rate: 33.3%
- Duration: 25.99s

#### overload / RTS / memory-pressure

- Submitted: 18
- Completed: 6
- Failed: 12
- Success Rate: 33.3%
- Duration: 29.02s

#### overload / PPO / memory-pressure

- Submitted: 18
- Completed: 6
- Failed: 12
- Success Rate: 33.3%
- Duration: 26.06s

## Verdict

**⚠️  90/510 tasks failed (17.6%)**

- Total tasks seen by cluster: 510
- Completed: 420
- Failed: 90
- Workers used: 1
