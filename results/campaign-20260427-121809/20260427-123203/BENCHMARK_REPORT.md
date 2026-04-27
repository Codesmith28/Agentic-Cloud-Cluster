# Benchmark Execution Report

**Generated**: 2026-04-27 07:02:04 UTC
**Master URL**: http://localhost:8080

## Model

- **path**: `agentic_scheduler/models/ppo_latest.pt`
- **size_bytes**: `234358`
- **sha256**: `76ac246538e57b37...`
- **version**: `v004`

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
| unassigned | 340 | 280 | 60 | cpu-heavy(100), cpu-light(90), memory-heavy(80), mixed(70) |
| **Total** | **340** | **280** | **60** | |

## Campaign Summary

- **Started**: 2026-04-27T06:48:09Z
- **Finished**: 2026-04-27T07:02:03Z
- **Duration**: 833.95s
- **Scenarios run**: 36

### Scheduler Comparison

| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |
|-----------|-------|-----------|--------|--------------|-------------|
| RR | 0 | 0 | 0 | 0.0% | 0.0s |
| RTS | 170 | 140 | 30 | 82.4% | 31.65s |
| PPO | 170 | 140 | 30 | 82.4% | 28.67s |

**Best scheduler**: **RTS**

### Scenario Details

#### baseline / RR / heterogeneous-smoke

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### baseline / RR / steady-cpu

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### baseline / RR / bursty

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### baseline / RR / memory-pressure

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
- Duration: 18.2s

#### baseline / RTS / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 73.36s

#### baseline / RTS / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 29.2s

#### baseline / RTS / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 47.79s

#### baseline / PPO / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.22s

#### baseline / PPO / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 19.16s

#### baseline / PPO / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 17.16s

#### baseline / PPO / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 11.64s

#### burst / RR / heterogeneous-smoke

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### burst / RR / steady-cpu

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### burst / RR / bursty

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### burst / RR / memory-pressure

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
- Duration: 15.23s

#### burst / RTS / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 12.17s

#### burst / RTS / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 21.23s

#### burst / RTS / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 12.15s

#### burst / PPO / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 24.19s

#### burst / PPO / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 15.18s

#### burst / PPO / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 18.24s

#### burst / PPO / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 9.11s

#### overload / RR / heterogeneous-smoke

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### overload / RR / steady-cpu

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### overload / RR / bursty

- Submitted: 0
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 0.0s
- Error: Failed to switch scheduler to RR

#### overload / RR / memory-pressure

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
- Duration: 51.96s

#### overload / RTS / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 33.32s

#### overload / RTS / bursty

- Submitted: 30
- Completed: 24
- Failed: 6
- Success Rate: 80.0%
- Duration: 39.35s

#### overload / RTS / memory-pressure

- Submitted: 18
- Completed: 6
- Failed: 12
- Success Rate: 33.3%
- Duration: 25.89s

#### overload / PPO / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 82.19s

#### overload / PPO / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 36.39s

#### overload / PPO / bursty

- Submitted: 30
- Completed: 24
- Failed: 6
- Success Rate: 80.0%
- Duration: 39.39s

#### overload / PPO / memory-pressure

- Submitted: 18
- Completed: 6
- Failed: 12
- Success Rate: 33.3%
- Duration: 53.13s

## Verdict

**⚠️  60/340 tasks failed (17.6%)**

- Total tasks seen by cluster: 340
- Completed: 280
- Failed: 60
- Workers used: 1
