# Agentic Scheduler - Training Report

## Hardware & Dataset Setup
- **Accelerator**: NVIDIA GeForce RTX 4060 Laptop GPU
- **Cluster Size**: 4034 machines
- **Workload**: 199614 tasks (Alibaba v2018 Trace)

## Training Progress
| Update | Avg Reward | Steps | Epoch |
|---|---|---|---|
| 1 | 1.6343 | 16384 | 0.08 |
| 10 | 1.6392 | 163840 | 0.82 |
| 20 | 1.6212 | 327680 | 1.64 |
| 30 | 1.6263 | 491520 | 2.46 |
| 40 | 1.6205 | 655360 | 3.28 |
| 50 | 1.6104 | 819200 | 4.10 |
| 60 | 1.5921 | 983040 | 4.92 |
| 70 | 1.5734 | 1146880 | 5.75 |
| 80 | 1.5632 | 1310720 | 6.57 |
| 90 | 1.5791 | 1474560 | 7.39 |
| 100 | 1.5713 | 1638400 | 8.21 |
| 110 | 1.5347 | 1802240 | 9.03 |
| 120 | 1.5841 | 1966080 | 9.85 |
| 130 | 1.5602 | 2129920 | 10.67 |
| 140 | 1.5657 | 2293760 | 11.49 |
| 150 | 1.5615 | 2457600 | 12.31 |

### Summary
The model has completed **150** PPO updates.
Final average reward: **1.5615**
Best average reward:  **1.6392** (update 10)
Worst average reward: **1.5347** (update 110)
