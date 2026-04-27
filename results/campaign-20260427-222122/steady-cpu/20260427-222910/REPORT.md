# CloudAI Evidence Benchmark Campaign Report

**Started**: 2026-04-27T16:55:36Z
**Finished**: 2026-04-27T16:59:10Z
**Duration**: 213.71s
**Scenarios executed**: 9

## Scheduler Comparison

| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration | Avg Queue Wait | Avg Turnaround | P95 Turnaround |
|-----------|-------|-----------|--------|-------------|-------------|---------------|---------------|----------------|
| RR | 40 | 40 | 0 | 100.0% | 19.55s | 0.0s | 0.0s | 0.0s |
| RTS | 40 | 40 | 0 | 100.0% | 21.57s | 0.0s | 0.0s | 0.0s |
| PPO | 40 | 40 | 0 | 100.0% | 20.87s | 0.0s | 0.0s | 0.0s |

**Best scheduler**: RR


## Scenario Details

### baseline / RR / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 16.16s
- Avg Queue Wait: 0.0s
- Avg Turnaround: 0.0s
- P95 Turnaround: 0.0s

### baseline / RTS / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 19.18s
- Avg Queue Wait: 0.0s
- Avg Turnaround: 0.0s
- P95 Turnaround: 0.0s

### baseline / PPO / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 16.21s
- Avg Queue Wait: 0.0s
- Avg Turnaround: 0.0s
- P95 Turnaround: 0.0s

### burst / RR / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 12.13s
- Avg Queue Wait: 0.0s
- Avg Turnaround: 0.0s
- P95 Turnaround: 0.0s

### burst / RTS / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 12.16s
- Avg Queue Wait: 0.0s
- Avg Turnaround: 0.0s
- P95 Turnaround: 0.0s

### burst / PPO / steady-cpu

- Submitted: 8
- Completed: 8
- Failed: 0
- Success Rate: 100.0%
- Duration: 16.01s
- Avg Queue Wait: 0.0s
- Avg Turnaround: 0.0s
- P95 Turnaround: 0.0s

### overload / RR / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 30.36s
- Avg Queue Wait: 0.0s
- Avg Turnaround: 0.0s
- P95 Turnaround: 0.0s

### overload / RTS / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 33.36s
- Avg Queue Wait: 0.0s
- Avg Turnaround: 0.0s
- P95 Turnaround: 0.0s

### overload / PPO / steady-cpu

- Submitted: 24
- Completed: 24
- Failed: 0
- Success Rate: 100.0%
- Duration: 30.38s
- Avg Queue Wait: 0.0s
- Avg Turnaround: 0.0s
- P95 Turnaround: 0.0s

## Comparison with Published Results

### Reference: SAC-CS (Soft Actor-Critic for Container Scheduling)

> Taha, A.; Maher, S.; Manimurugan, S.; Taha, M.; Amin, E. "Optimized Container
> Scheduling: A Soft Actor-Critic Deep Reinforcement Learning Approach."
> *Computers* 2025, 14, 560. https://doi.org/10.3390/computers14120560

**SAC-CS Key Claims:**

| Metric | SAC-CS (Paper) | Method |
|--------|---------------|--------|
| Optimization target | Min execution time + energy | Multi-objective reward |
| State space | 6 features/host (affinity, speed, idle, diff-CPU/mem/GPU) | Flattened N×6 vector |
| Action space | Discrete (host index) | Stochastic policy sampling |
| Discount factor (γ) | 0.01 | Short-horizon, immediate-reward focus |
| Batch size | 128 | Replay buffer training |
| Algorithm | SAC with twin critics + entropy regularization | Maximum entropy RL |

**Our Approach (PPO-based) vs SAC-CS:**

| Aspect | SAC-CS (Paper) | Our PPO Scheduler |
|--------|---------------|-------------------|
| RL Algorithm | Soft Actor-Critic | Proximal Policy Optimization |
| Policy type | Stochastic (entropy-regularized) | Stochastic (clipped surrogate) |
| Exploration | Entropy bonus (automatic temperature α) | GAE + entropy coefficient |
| Training data | Simulated datacenter tasks | Alibaba cluster-trace-v2018 (200K real tasks) |
| Online adaptation | Not described | Online PPO updates from live cluster feedback |
| Baselines | Random, Round-Robin, First-Fit | Round-Robin (RR), Risk-aware (RTS) |
| Evaluation | Simulated environment only | Live Docker cluster with real task execution |

**Our Benchmark Results Summary:**

| Scheduler | Success Rate | Avg Duration | Avg Queue Wait | Avg Turnaround |
|-----------|-------------|-------------|---------------|---------------|
| RR | 100.0% | 19.55s | 0.0s | 0.0s |
| RTS | 100.0% | 21.57s | 0.0s | 0.0s |
| PPO | 100.0% | 20.87s | 0.0s | 0.0s |

**PPO duration improvement over RR**: -6.8%

### Methodology Notes

1. **Different evaluation harnesses**: SAC-CS evaluates in simulation; our results
   come from a live Docker cluster with real container execution. Direct numeric
   comparison is not appropriate — the environments measure different things.
2. **Training data**: Our PPO model is pre-trained on the Alibaba cluster-trace-v2018
   dataset (199,614 real production tasks from 17,592 machines) [1], providing a more
   realistic training signal than synthetic workloads.
3. **Online learning**: Unlike SAC-CS, our PPO continues learning from live cluster
   feedback during deployment, adapting to the actual workload distribution.
4. **Cluster topology**: 3-node heterogeneous cluster (small: 1 CPU/1.5 GB,
   medium: 2 CPU/3 GB, large: 3 CPU/5 GB) with Docker-in-Docker task execution.

### References

[1] Alibaba Cluster Trace Program. "cluster-trace-v2018."
    https://github.com/alibaba/clusterdata, 2018.

[2] Schulman, J.; Wolski, F.; Dhariwal, P.; Radford, A.; Klimov, O.
    "Proximal Policy Optimization Algorithms." *arXiv preprint arXiv:1707.06347*, 2017.

[3] Taha, A. et al. "Optimized Container Scheduling: A Soft Actor-Critic Deep
    Reinforcement Learning Approach." *Computers* 2025, 14, 560.
    https://doi.org/10.3390/computers14120560
