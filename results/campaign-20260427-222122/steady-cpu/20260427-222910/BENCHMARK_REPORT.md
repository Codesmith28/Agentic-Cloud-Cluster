# Benchmark Execution Report

**Generated**: 2026-04-27 17:06:02 UTC
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

- **Started**: 2026-04-27T16:55:36Z
- **Finished**: 2026-04-27T16:59:10Z
- **Duration**: 213.71s
- **Scenarios run**: 9

### Scheduler Comparison

| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |
|-----------|-------|-----------|--------|--------------|-------------|
| RR | 40 | 40 | 0 | 100.0% | 19.55s |
| RTS | 40 | 40 | 0 | 100.0% | 21.57s |
| PPO | 40 | 40 | 0 | 100.0% | 20.87s |

**Best scheduler**: **RR**

### Scenario Details

#### baseline / RR / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 16.16s

#### baseline / RTS / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 19.18s

#### baseline / PPO / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 16.21s

#### burst / RR / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 12.13s

#### burst / RTS / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 12.16s

#### burst / PPO / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 16.01s

#### overload / RR / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 30.36s

#### overload / RTS / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 33.36s

#### overload / PPO / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 30.38s

## Verdict

**⚠️  90/510 tasks failed (17.6%)**

- Total tasks seen by cluster: 510
- Completed: 420
- Failed: 90
- Workers used: 1
