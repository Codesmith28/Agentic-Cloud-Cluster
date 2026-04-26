# CloudAI Evidence Benchmark Campaign Report

**Started**: 2026-04-26T13:04:08Z
**Finished**: 2026-04-26T16:15:38Z
**Duration**: 11489.76s
**Scenarios executed**: 72

## Scheduler Comparison

| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |
|-----------|-------|-----------|--------|-------------|-------------|
| RR | 240 | 156 | 36 | 65.0% | 218.39s |
| RTS | 240 | 161 | 37 | 67.1% | 210.55s |
| PPO-pretrained | 240 | 195 | 45 | 81.2% | 22.25s |
| PPO-adapted | 240 | 166 | 36 | 69.2% | 187.13s |

**Best scheduler**: PPO-pretrained


## Scenario Details

### baseline / RR / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 24.09s

### baseline / RR / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 19.06s

### baseline / RR / steady-mixed

- Submitted: 8
- Completed: 6
- Failed: 2
- Success Rate: 75.0%
- Duration: 17.3s

### baseline / RR / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 14.55s

### baseline / RR / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 23.06s

### baseline / RR / long-tail

- Submitted: 6
- Completed: 5
- Failed: 1
- Success Rate: 83.3%
- Duration: 23.57s

### baseline / RTS / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.07s

### baseline / RTS / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 19.06s

### baseline / RTS / steady-mixed

- Submitted: 8
- Completed: 6
- Failed: 2
- Success Rate: 75.0%
- Duration: 14.31s

### baseline / RTS / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 17.57s

### baseline / RTS / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 23.06s

### baseline / RTS / long-tail

- Submitted: 6
- Completed: 5
- Failed: 1
- Success Rate: 83.3%
- Duration: 23.57s

### baseline / PPO-pretrained / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 24.08s

### baseline / PPO-pretrained / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 16.05s

### baseline / PPO-pretrained / steady-mixed

- Submitted: 8
- Completed: 6
- Failed: 2
- Success Rate: 75.0%
- Duration: 17.32s

### baseline / PPO-pretrained / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 14.55s

### baseline / PPO-pretrained / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 20.06s

### baseline / PPO-pretrained / long-tail

- Submitted: 6
- Completed: 5
- Failed: 1
- Success Rate: 83.3%
- Duration: 20.57s

### baseline / PPO-adapted / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 39.13s

### baseline / PPO-adapted / steady-cpu

- Submitted: 8
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 609.62s

### baseline / PPO-adapted / steady-mixed

- Submitted: 8
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 607.99s

### baseline / PPO-adapted / memory-pressure

- Submitted: 6
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 604.66s

### baseline / PPO-adapted / bursty

- Submitted: 10
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 608.2s

### baseline / PPO-adapted / long-tail

- Submitted: 6
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 604.65s

### burst / RR / heterogeneous-smoke

- Submitted: 10
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 600.03s

### burst / RR / steady-cpu

- Submitted: 8
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 602.54s

### burst / RR / steady-mixed

- Submitted: 8
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 602.47s

### burst / RR / memory-pressure

- Submitted: 6
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 602.08s

### burst / RR / bursty

- Submitted: 10
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 602.99s

### burst / RR / long-tail

- Submitted: 6
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 602.06s

### burst / RTS / heterogeneous-smoke

- Submitted: 10
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 600.09s

### burst / RTS / steady-cpu

- Submitted: 8
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 602.6s

### burst / RTS / steady-mixed

- Submitted: 8
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 602.55s

### burst / RTS / memory-pressure

- Submitted: 6
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 602.07s

### burst / RTS / bursty

- Submitted: 10
- Completed: 0
- Failed: 0
- Success Rate: 0.0%
- Duration: 600.07s

### burst / RTS / long-tail

- Submitted: 6
- Completed: 5
- Failed: 1
- Success Rate: 83.3%
- Duration: 472.72s

### burst / PPO-pretrained / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.1s

### burst / PPO-pretrained / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 15.07s

### burst / PPO-pretrained / steady-mixed

- Submitted: 8
- Completed: 6
- Failed: 2
- Success Rate: 75.0%
- Duration: 15.07s

### burst / PPO-pretrained / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 12.07s

### burst / PPO-pretrained / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 12.07s

### burst / PPO-pretrained / long-tail

- Submitted: 6
- Completed: 5
- Failed: 1
- Success Rate: 83.3%
- Duration: 18.09s

### burst / PPO-adapted / heterogeneous-smoke

- Submitted: 10
- Completed: 10
- Failed: 0
- Success Rate: 100.0%
- Duration: 18.09s

### burst / PPO-adapted / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 15.1s

### burst / PPO-adapted / steady-mixed

- Submitted: 8
- Completed: 6
- Failed: 2
- Success Rate: 75.0%
- Duration: 12.07s

### burst / PPO-adapted / memory-pressure

- Submitted: 6
- Completed: 2
- Failed: 4
- Success Rate: 33.3%
- Duration: 12.06s

### burst / PPO-adapted / bursty

- Submitted: 10
- Completed: 8
- Failed: 2
- Success Rate: 80.0%
- Duration: 15.11s

### burst / PPO-adapted / long-tail

- Submitted: 6
- Completed: 5
- Failed: 1
- Success Rate: 83.3%
- Duration: 18.07s

### overload / RR / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 30.38s

### overload / RR / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 36.15s

### overload / RR / steady-mixed

- Submitted: 24
- Completed: 18
- Failed: 6
- Success Rate: 75.0%
- Duration: 24.89s

### overload / RR / memory-pressure

- Submitted: 18
- Completed: 6
- Failed: 12
- Success Rate: 33.3%
- Duration: 28.7s

### overload / RR / bursty

- Submitted: 30
- Completed: 24
- Failed: 6
- Success Rate: 80.0%
- Duration: 36.24s

### overload / RR / long-tail

- Submitted: 18
- Completed: 15
- Failed: 3
- Success Rate: 83.3%
- Duration: 40.78s

### overload / RTS / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 39.44s

### overload / RTS / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 33.14s

### overload / RTS / steady-mixed

- Submitted: 24
- Completed: 18
- Failed: 6
- Success Rate: 75.0%
- Duration: 24.91s

### overload / RTS / memory-pressure

- Submitted: 18
- Completed: 6
- Failed: 12
- Success Rate: 33.3%
- Duration: 25.68s

### overload / RTS / bursty

- Submitted: 30
- Completed: 24
- Failed: 6
- Success Rate: 80.0%
- Duration: 36.21s

### overload / RTS / long-tail

- Submitted: 18
- Completed: 15
- Failed: 3
- Success Rate: 83.3%
- Duration: 34.81s

### overload / PPO-pretrained / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 39.56s

### overload / PPO-pretrained / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 30.18s

### overload / PPO-pretrained / steady-mixed

- Submitted: 24
- Completed: 18
- Failed: 6
- Success Rate: 75.0%
- Duration: 27.91s

### overload / PPO-pretrained / memory-pressure

- Submitted: 18
- Completed: 6
- Failed: 12
- Success Rate: 33.3%
- Duration: 25.73s

### overload / PPO-pretrained / bursty

- Submitted: 30
- Completed: 24
- Failed: 6
- Success Rate: 80.0%
- Duration: 36.18s

### overload / PPO-pretrained / long-tail

- Submitted: 18
- Completed: 15
- Failed: 3
- Success Rate: 83.3%
- Duration: 37.79s

### overload / PPO-adapted / heterogeneous-smoke

- Submitted: 30
- Completed: 30
- Failed: 0
- Success Rate: 100.0%
- Duration: 39.58s

### overload / PPO-adapted / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 33.19s

### overload / PPO-adapted / steady-mixed

- Submitted: 24
- Completed: 18
- Failed: 6
- Success Rate: 75.0%
- Duration: 24.93s

### overload / PPO-adapted / memory-pressure

- Submitted: 18
- Completed: 6
- Failed: 12
- Success Rate: 33.3%
- Duration: 28.84s

### overload / PPO-adapted / bursty

- Submitted: 30
- Completed: 24
- Failed: 6
- Success Rate: 80.0%
- Duration: 36.28s

### overload / PPO-adapted / long-tail

- Submitted: 18
- Completed: 15
- Failed: 3
- Success Rate: 83.3%
- Duration: 40.84s
