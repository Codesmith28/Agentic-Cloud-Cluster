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

- **Started**: 2026-04-27T16:59:15Z
- **Finished**: 2026-04-27T17:02:51Z
- **Duration**: 216.23s
- **Scenarios run**: 9

### Scheduler Comparison

| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |
|-----------|-------|-----------|--------|--------------|-------------|
| RR | 50 | 40 | 10 | 80.0% | 20.92s |
| RTS | 50 | 40 | 10 | 80.0% | 20.92s |
| PPO | 50 | 40 | 10 | 80.0% | 20.93s |

**Best scheduler**: **RR**

### Scenario Details

#### baseline / RR / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 17.16s

#### baseline / RTS / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 17.18s

#### baseline / PPO / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 17.21s

#### burst / RR / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 12.2s

#### burst / RTS / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 12.22s

#### burst / PPO / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 9.21s

#### overload / RR / bursty

- Submitted: 30
- Completed: 24
- Failed: 6
- Success Rate: 80.0%
- Duration: 33.41s

#### overload / RTS / bursty

- Submitted: 30
- Completed: 24
- Failed: 6
- Success Rate: 80.0%
- Duration: 33.37s

#### overload / PPO / bursty

- Submitted: 30
- Completed: 24
- Failed: 6
- Success Rate: 80.0%
- Duration: 36.36s

## Verdict

**⚠️  90/510 tasks failed (17.6%)**

- Total tasks seen by cluster: 510
- Completed: 420
- Failed: 90
- Workers used: 1
