# Agentic Scheduler Metrics and Reward Function

This note documents the **current PPO/agentic scheduler implementation**. It explains what the scheduler observes, how every reward term is calculated, how PPO uses the reward, which metrics are used to evaluate the scheduler, and how the design evolved.

> **Code is the source of truth.** Some older project documentation describes a fixed 14-element state and a reward based on success, normalized wait, and a balance bonus. That description is stale. The current policy uses **5 task features plus 9 features for every worker**, an action mask, a multi-term offline trace reward, and a separate online outcome reward.

## 1. The metrics fall into four different groups

These groups answer different questions and should not be mixed:

1. **State features** — information available to the policy before it chooses a worker.
2. **Reward signals** — scalar feedback used to train the policy.
3. **PPO learning quantities** — return, advantage, losses, entropy, and probability ratios used during optimization.
4. **Evaluation and operational metrics** — success rate, queue wait, turnaround, feasibility, latency, utilization, and balance used to compare or monitor schedulers.

The reward is an optimization target. A benchmark metric is external evidence of whether optimizing that target produced useful real behavior. A high training reward is therefore not, by itself, proof that the live scheduler is better.

---

## 2. Metrics the PPO policy observes

At each scheduling decision, the state is:

```text
state = {
    task:        5 values,
    workers:     W × 9 values,
    action_mask: W booleans
}
```

`W` is the number of candidate workers. The model evaluates each task-worker pair, so it can work with different cluster sizes.

### 2.1 Task features

| Index | Feature | Calculation | Why it matters / expected impact |
|---:|---|---|---|
| 0 | CPU request | `req_cpu` | Allows the policy to avoid CPU-constrained workers and reserve CPU headroom for large tasks. |
| 1 | Memory request | `req_memory` | Makes memory-heavy tasks distinguishable and prevents memory becoming the hidden bottleneck. |
| 2 | Storage request | `req_storage` | Prevents placements that fit CPU/memory but not storage. |
| 3 | SLA multiplier | `sla_multiplier`, default `2.0` | Represents the allowed time budget. In the wider system, the deadline is based on `k × tau`; larger `k` means more slack. |
| 4 | Task type | `id(task_type) / 3`, where CPU-light=0, CPU-heavy=1, memory-heavy=2, mixed=3 | Gives the network a coarse workload class in addition to raw resource requests. |

Source: `agentic_scheduler/features.py`, lines 8-39.

Alibaba CPU values are converted to cores when needed (`100 → 1 core`). Alibaba memory stays in the trace's native normalized units because task memory and machine capacity use the same scale. Mixing these units previously made almost all actions appear infeasible.

### 2.2 Worker features

For every worker, the policy receives:

| Index | Feature | Calculation | Why it matters / expected impact |
|---:|---|---|---|
| 0 | CPU headroom ratio | `available_cpu / total_cpu` | Direct measure of remaining CPU capacity. |
| 1 | Memory headroom ratio | `available_memory / total_memory` | Direct measure of remaining memory capacity. |
| 2 | Storage headroom ratio | `available_storage / total_storage` | Direct measure of remaining storage capacity. |
| 3 | Total CPU | `total_cpu` | Lets the model distinguish small and large workers. |
| 4 | Total memory | `total_memory` | Captures worker heterogeneity. |
| 5 | Total storage | `total_storage` | Captures storage heterogeneity. |
| 6 | CPU utilization ratio | `(total_cpu - available_cpu) / total_cpu` | Represents current CPU pressure. |
| 7 | Memory utilization ratio | `(total_memory - available_memory) / total_memory` | Represents current memory pressure. |
| 8 | Storage utilization ratio | `(total_storage - available_storage) / total_storage` | Represents current storage pressure. |

All divisions are protected against a zero denominator. Used resources are clamped to at least zero.

Headroom and utilization are mathematically complementary in normal conditions, but both are supplied so the network can learn different gradients for “capacity left” and “pressure already present.” Total capacity is retained because a 50% free 4-core machine is not equivalent to a 50% free 32-core machine.

Source: `agentic_scheduler/features.py`, lines 42-80.

### 2.3 Feasibility / action mask

A worker is feasible only if:

```text
worker is active
AND available_cpu     >= task.req_cpu
AND available_memory  >= task.req_memory
AND available_storage >= task.req_storage
```

Infeasible worker logits are replaced with a very negative value (`-1e4`), giving them effectively zero selection probability. The Go master validates the returned worker again and falls back to RTS/RR if PPO returns an invalid or unsuitable worker.

Impact: the action mask turns hard capacity and liveness constraints into safety constraints instead of asking the neural network to learn them purely by trial and error. The infeasible reward remains useful during offline exploration and for diagnosing bad trace state, but production inference normally cannot deliberately select a masked worker.

Sources: `agentic_scheduler/features.py`, lines 83-117; `agentic_scheduler/model.py`, lines 111-113; `master/internal/scheduler/ppo_scheduler.go`, lines 196-200 and 327-343.

### 2.4 Feature normalization

Each task vector is concatenated with each worker vector to form a 14-value task-worker row. A running Welford normalizer maintains the mean and sample variance:

```text
normalized_feature = (x - running_mean) / sqrt(max(sample_variance, 1e-6))
```

Impact: CPU cores, storage capacity, SLA multipliers, and ratios have very different numerical scales. Normalization prevents large-unit features from dominating the network only because their numbers are larger. The normalizer state is saved in the checkpoint, so inference uses the learned scale.

Source: `agentic_scheduler/model.py`, lines 20-77 and 205-216.

---

## 3. Current offline trace-replay reward

The primary offline reward is implemented in `TraceReplayEnv._quality_reward()`.

### 3.1 Resource load used by the reward

After tentatively placing the task, the selected worker's load is:

```text
cpu_load     = used_cpu / total_cpu
memory_load  = used_memory / total_memory
storage_load = used_storage / total_storage

projected_load = min((cpu_load + memory_load + storage_load) / 3, 1.5)
cluster_load   = mean(projected load of every worker)
```

The cap of `1.5` prevents extreme oversubscription from producing an unbounded signal. Under normal feasible placement, each component should be at most 1, so the load is normally in `[0, 1]`.

### 3.2 SLA and latency proxies

The trace environment does not actually run containers, so it estimates the effect of current load:

```text
runtime          = max(task.runtime_seconds, 1)
k                = max(task.sla_multiplier, 1)
historical_wait  = max(task.queue_wait_seconds, 0)

queue_wait_proxy = historical_wait + runtime × projected_load
turnaround_proxy = queue_wait_proxy + runtime
sla_budget       = max(runtime × k, 1)
```

The normalized pressures are:

```text
queue_pressure      = min(queue_wait_proxy / sla_budget, 3)
turnaround_pressure = min(turnaround_proxy / sla_budget, 4)
tail_pressure       = max(turnaround_pressure - 1, 0)
```

`turnaround_pressure` is returned as a diagnostic metric, but it is **not directly subtracted from reward**. Only its amount above 1, `tail_pressure`, enters the reward. In other words, the tail term activates when estimated turnaround exceeds the SLA budget.

The caps limit the influence of extreme trace outliers and help keep gradients stable.

### 3.3 Balance, headroom, and recovery terms

```text
headroom_bonus    = 1 - projected_load
imbalance_penalty = max(projected_load - cluster_load, 0)

delta_imbalance = std(loads_after_placement)
                - std(loads_before_placement)

requeue_penalty = 0.05 × min(task.requeue_count, 4)
```

- `headroom_bonus` is higher for a lightly loaded selected worker.
- `imbalance_penalty` penalizes selecting a worker that ends above the cluster mean.
- `delta_imbalance` measures what this action changed. A positive value means the action made worker loads less balanced and is penalized. A negative value means the action improved balance and therefore adds reward because the formula subtracts it.
- `requeue_penalty` adds a small cost for tasks that have already required recovery, capped at `0.20`.

### 3.4 Full formula

For a feasible placement:

```text
R_offline = 1.40
          + 0.25 × headroom_bonus
          - 0.35 × queue_pressure
          - 0.55 × tail_pressure
          - 0.20 × imbalance_penalty
          - 0.40 × delta_imbalance
          - requeue_penalty
```

For an infeasible placement:

```text
R_offline = -1.80
```

Source: `agentic_scheduler/training/trace_replay_env.py`, lines 275-342.

### 3.5 Why these terms and weights are used

| Term | Weight | Design purpose | Behavioral impact |
|---|---:|---|---|
| Valid-placement baseline | `+1.40` | Make satisfying hard capacity constraints the first objective. | Feasible choices normally remain positive; infeasible choices are clearly worse. |
| Headroom | `+0.25` | Preserve spare capacity and reduce immediate saturation risk. | Favors workers that will remain less loaded after placement. |
| Queue pressure | `-0.35` | Optimize delay relative to the task's own SLA scale rather than raw seconds. | Avoids choices whose projected load is likely to increase wait. |
| Tail/SLA pressure | `-0.55` | Give estimated SLA breach the largest quality penalty. | Makes avoiding long-tail delay more important than small balance or headroom improvements. |
| Above-mean imbalance | `-0.20` | Discourage repeatedly selecting an already-hot worker. | Reduces hot spots while keeping the penalty secondary to SLA. |
| Change in imbalance | `-0.40` | Assign credit for the imbalance caused by this specific action. | Penalizes a new imbalance even when the cluster was balanced before; rewards corrective placements. |
| Requeue count | `-0.05` each, max `-0.20` | Reflect recovery cost without allowing old failures to dominate. | Slightly lowers the value of repeatedly recovered work. |
| Infeasible action | `-1.80` | Strong hard-constraint violation signal. | Teaches the policy that a superficially attractive but impossible placement is unacceptable. |

The priority encoded by the relative weights is:

```text
feasibility first
→ prevent SLA-tail failures
→ reduce queue pressure
→ avoid creating imbalance
→ preserve headroom and account for recovery history
```

The numerical coefficients are **engineering weights, not coefficients derived from a closed-form optimization or statistical regression**. They were chosen to keep the shaped reward bounded and smooth, keep valid placements generally positive, make SLA-tail pressure the strongest soft penalty, and keep balance/recovery terms secondary. Later PPO sweeps selected model/training configurations and the serving headroom bias using replay and live benchmarks; the repository does not show a grid search or ablation that independently proves `0.35`, `0.55`, `0.20`, and `0.40` are globally optimal reward weights.

### 3.6 Worked offline example

Assume, after placement:

```text
projected_load = 0.45
cluster_load = 0.35
std_before = 0.10, std_after = 0.12
runtime = 100 s, historical_wait = 10 s, k = 2
requeue_count = 1
```

Then:

```text
sla_budget       = 100 × 2 = 200 s
queue_wait_proxy = 10 + 100 × 0.45 = 55 s
queue_pressure   = 55 / 200 = 0.275
turnaround       = 155 s, so tail_pressure = max(155/200 - 1, 0) = 0
headroom_bonus   = 0.55
imbalance        = 0.45 - 0.35 = 0.10
delta_imbalance  = 0.12 - 0.10 = 0.02
requeue_penalty  = 0.05

reward = 1.40 + 0.25(0.55) - 0.35(0.275)
       - 0.20(0.10) - 0.40(0.02) - 0.05
       = 1.36325
```

This is a good feasible placement, but it loses some reward for queue pressure, selecting a worker above the cluster mean, slightly worsening balance, and recovery history.

### 3.7 Important interpretation limits

- Offline `sla_success` and `failure_reason` fields exist in `TraceTask`, but the current trace reward does not directly use them.
- Historical queue wait and requeue count belong to the task record. For a single state they are mostly action-independent; the action-dependent part of the queue proxy is `runtime × projected_load`.
- `projected_load` averages CPU, memory, and storage equally. A single nearly exhausted resource can be softened by two idle resources, although the action mask still enforces capacity feasibility.
- Reward is a training proxy. Actual queue wait and SLA success must still be checked in live benchmarks.

---

## 4. Synthetic-environment reward

When no real trace source is supplied, training uses `SchedulingEnv`, which has a deliberately simpler curriculum:

```text
if feasible:
    normalized_load = min(mean(cpu_usage, memory_usage, storage_usage), 1.5)
    R_synthetic = 1.2 - normalized_load
else:
    R_synthetic = -1.4
```

Impact: it teaches basic feasibility and low-load placement, but it does not model trace queue wait, tail SLA risk, requeues, or change in cluster imbalance. Results from synthetic and trace training therefore should not be compared as if their reward scales mean exactly the same thing.

Source: `agentic_scheduler/training/scheduler_env.py`, lines 76-97 and 182-187.

---

## 5. Production online-outcome reward

The master calculates a separate reward after a real task reaches a terminal state:

```text
status term:
    success   → +1.00
    cancelled → -0.50
    failed    → -1.00

SLA term:
    met SLA   → +0.50
    missed    → -0.25

runtime term:
    -min(runtime_seconds / 600, 0.50)
```

Therefore:

```text
R_online = status_term + SLA_term - min(runtime_seconds / 600, 0.50)
```

The theoretical range under these branches is approximately `[-1.75, +1.50]`.

Examples:

- Success, SLA met, 90 s runtime: `1.0 + 0.5 - 0.15 = 1.35`.
- Failure, SLA missed, runtime at or above 300 s: `-1.0 - 0.25 - 0.5 = -1.75`.
- Cancellation is penalized less than failure because it may be externally requested rather than caused by a bad placement.

Why this reward is different from offline reward:

- Production has the actual terminal status, measured runtime, and deadline outcome, so it does not need simulated queue/tail proxies.
- Success is the dominant outcome, SLA compliance is the next priority, and runtime provides a bounded preference for faster completion.
- The runtime cap prevents a single very long task from destabilizing a small online update.

The Go master sends this explicit reward to the Python service. The Python service contains the same formula as a fallback, used only if the received reward is exactly `0.0`.

Sources: `master/internal/server/master_server.go`, lines 1125-1223; `agentic_scheduler/service.py`, lines 281-349.

### Online reward impact and caveats

- A successful but slow task can still receive a positive reward; this correctly keeps completion more important than small speed differences.
- A successful SLA miss receives less reward than an SLA-compliant success.
- The reward does not directly include worker balance, queue wait, resource utilization, or requeue count. Online learning relies on their downstream effect on status, SLA, and runtime.
- Runtime is useful only if worker choice can change execution speed or contention. Intrinsically long jobs receive a penalty even when no alternative worker could make them faster.
- SLA success is calculated when the result is processed by comparing the current time with the stored deadline. It is close to completion time in normal flow, but it is not recomputed from an explicit completion timestamp in the reward function.

---

## 6. How the reward function evolved

The repository history shows an iterative, evidence-driven evolution rather than one formula derived in a single step:

1. **Initial synthetic reward:** feasible placement earned `1.2 - load`; infeasible placement earned `-1.4`. This established the feasibility/load-balancing baseline.
2. **Initial trace reward:** added runtime awareness to the basic load reward.
3. **Queue/tail shaping (commit `e8f42af`, April 2026):** introduced normalized queue pressure, turnaround-based tail pressure, headroom, imbalance, requeue cost, a `+1.4` baseline, and `-1.8` infeasibility. This aligned offline optimization with the live ranking priorities: success, turnaround, queue wait, and run duration.
4. **Correct lifecycle and units (commit `e9ea2b4`, April 2026):**
   - fixed Alibaba memory units;
   - deduplicated machine records;
   - replaced exponential resource decay with task lifecycle accounting;
   - added `delta_imbalance` because the older above-mean penalty produced no signal when a balanced cluster was made unbalanced.
5. **Training/model selection sweeps:** replay `mean_reward` and `feasible_action_rate` gated models; repeated live runs ranked them using success, turnaround, queue wait, and duration. Serving headroom bias was calibrated separately (the optimized comparison used `0.20`).

The lifecycle correction was especially important. The Alibaba slice had many simultaneous arrivals, so exponential decay did not release resources correctly. With explicit lifecycles, placement reserves resources until `arrival_time + runtime`, then releases them. This makes load, feasibility, and every load-derived reward term reflect concurrency rather than an arbitrary decay constant.

The memory-unit fix also demonstrates why reward trends require diagnosis: the earlier `-1.3` to `-1.6` average rewards were largely caused by a unit mismatch that made about 98% of later actions infeasible, not necessarily by a poor policy. Post-fix test runs documented in `TRAINING_DECISIONS.md` reached full feasibility and about `+1.64` average reward.

---

## 7. Serving-time headroom score (not a training reward)

Production deterministic inference adds a small hand-designed score to policy logits for the policy's top feasible candidates. This is a serving calibration layer, not the environment reward.

For each worker it computes residual CPU/memory/storage after the task, projected peak utilization, projected resource spread, task size relative to the median worker, and an SLA-based factor. It then blends:

```text
risk_aware_score = 1.25 × min_residual
                 - 1.00 × projected_peak
                 - 0.35 × projected_spread

packing_score = -0.90 × mean_residual
              - 0.60 × projected_peak
              - 0.20 × projected_spread

score = urgency × risk_aware_score
      + (1 - urgency) × packing_score

selection_logit = policy_logit + headroom_bias × score
```

The default service `headroom_bias` is `0.25`; an optimized repeated comparison used `0.20`. High risk-aware weight preserves capacity for important/large tasks; packing puts less urgent tasks more tightly to improve utilization.

Current caveat: the code calculates `sla_urgency = clip((sla_multiplier - 1) / 2, 0, 1)`. In the rest of the system a larger SLA multiplier creates a later deadline and therefore more slack, but this formula treats larger multipliers as more “urgent.” The variable naming/rationale and formula are directionally inconsistent and should be reviewed before further tuning.

Source: `agentic_scheduler/model.py`, lines 324-379.

---

## 8. How PPO turns rewards into learning

### 8.1 Logged training reward

`avg_reward` is the arithmetic mean of the most recent **5,000 step rewards**, not necessarily one episode's return:

```text
avg_reward = mean(last 5000 environment rewards)
```

Impact: it smooths the curve and is useful for trend detection, but it reacts slowly to recent changes and must be compared only across runs using the same environment/reward version.

### 8.2 Discounted return and advantage

Offline defaults are `gamma = 0.99` and `GAE lambda = 0.95`. For each transition:

```text
delta_t = reward_t + gamma × V(s_t+1) × (1 - done_t) - V(s_t)

advantage_t = delta_t
            + gamma × lambda × (1 - done_t) × advantage_t+1

return_t = advantage_t + V(s_t)
```

- `gamma` controls how much future scheduling consequences matter.
- `lambda` controls the bias/variance trade-off in advantage estimates.
- Advantages are normalized before the offline PPO update.
- Online defaults are slightly shorter-horizon (`gamma=0.97`, `lambda=0.92`), and online advantages are clamped to `[-8, 8]` before normalization to limit outliers.

### 8.3 PPO optimization quantities

The actor is trained with the clipped surrogate:

```text
ratio = exp(new_log_probability - old_log_probability)

policy_loss = -mean(min(
    ratio × advantage,
    clip(ratio, 1-epsilon, 1+epsilon) × advantage
))
```

The critic predicts return using a clipped mean-squared-error value loss. Entropy rewards exploration:

```text
total_loss = policy_loss
           + value_coefficient × value_loss
           - entropy_coefficient × policy_entropy
```

Current offline defaults:

| Quantity | Default | Impact |
|---|---:|---|
| PPO clip epsilon | `0.20` | Limits abrupt changes to action probabilities. |
| Value clip | `0.20` | Limits abrupt critic changes. |
| Value coefficient | `0.50` | Balances actor and critic learning. |
| Entropy coefficient | `0.01` | Prevents premature collapse to one worker. |
| Gradient norm cap | `1.0` | Prevents unstable updates. |
| PPO epochs | `6` CLI default | Reuses each rollout while controlling overfitting. |

Online updates use clip `0.20`, entropy `0.01`, value coefficient `0.50`, and 4 epochs. They trigger after the replay buffer reaches the configured batch size (default 32).

Sources: `agentic_scheduler/train_ppo.py`, lines 25-46 and 295-323; `agentic_scheduler/model.py`, lines 252-321; `agentic_scheduler/service.py`, lines 351-453.

---

## 9. Offline model-selection metrics

The held-out trace replay uses:

| Metric | Calculation | Desired direction | What it tells us |
|---|---|---:|---|
| Evaluated steps | Number of replayed task decisions | Equal across policies | Fairness check; comparisons are invalid if policies saw different task counts. |
| Mean reward | `sum(step_rewards) / evaluated_steps` | Higher | Alignment with the complete shaped offline objective. |
| Feasible action rate | `feasible_steps / evaluated_steps` | Higher | How often the chosen worker could actually host the task. |

These are useful screening metrics, not final deployment proof. The documented hierarchy makes live cluster success and latency metrics primary, with replay reward/feasibility as secondary gates.

Source: `docs/PPO_TRACE_REPLAY.md`, evaluation example around lines 179-215; `docs/PPO_PERFORMANCE_OPTIMIZATION.md`.

---

## 10. Live campaign metrics

The end-to-end campaign runs real tasks on workers and calculates:

| Metric | Current calculation | Desired direction | Impact / interpretation |
|---|---|---:|---|
| Tasks submitted | Count of workload tasks sent | Same across schedulers | Fairness denominator. |
| Tasks completed | Count whose final status is `completed` | Higher | Successful useful work. |
| Tasks failed | Count with `failed` or `cancelled` | Lower | Reliability/capacity failure indicator. |
| Success rate | `completed / submitted × 100` | Higher | Primary ranking metric. Hardware-infeasible tasks can cap every scheduler equally. |
| Run duration | Wall time from submission start through polling completion | Lower | End-to-end campaign completion time. Includes submission/polling overhead. |
| Queue wait per task | `assigned_at - created_at` | Lower | How quickly scheduler/capacity assigned work. |
| Mean queue wait | Arithmetic mean of available task waits | Lower | Typical assignment delay. |
| Turnaround per task | `completed_at - created_at` | Lower | Full user-perceived latency. |
| Mean turnaround | Arithmetic mean of task turnaround | Lower | Typical end-to-end task latency. |
| P95 turnaround | Sorted nearest-rank-like element at `int(n × 0.95)` | Lower | Tail user latency. |

Campaign summaries aggregate success using total completed/total submitted, but average duration/wait/turnaround by taking an unweighted mean of per-scenario values. Zero/missing wait and turnaround values are excluded from those aggregate averages.

Source: `testbench/scripts/run_campaign.py`, lines 42-57, 152-187, 341-351, and 438-480.

The optimization reports rank schedulers in this order:

1. maximize success rate;
2. minimize mean turnaround;
3. minimize mean queue wait;
4. minimize total duration.

This hierarchy is sensible because a scheduler should not appear “fast” by failing work, and once success ties, user-visible latency becomes the discriminator.

Historical repeated-comparison reports also recorded `p95_queue_wait_seconds`, `mean_attempts_per_task`, `assignments_by_worker`, `assignment_balance_std`, and `assignment_max_share`. Their meanings are, respectively: tail assignment delay; average execution attempts per logical task; assignment count per worker; population standard deviation of those counts; and `max(worker_assignment_count) / total_assignments`. Lower balance standard deviation and lower maximum share indicate less concentration. These fields came from the one-off optimization evidence pipeline; they are not all produced by the current `run_campaign.py` data model.

---

## 11. Go simulation benchmark metrics

The built-in Go benchmark records a broader set. These are benchmark metrics, not PPO reward terms.

| Metric | Calculation |
|---|---|
| Total tasks | Number of generated tasks. |
| Completed tasks | Number with a simulated completion record. |
| Unschedulable tasks | Tasks left queued when no task is running and no more arrivals can change feasibility. |
| SLA success | Per task: `finish_time <= deadline`. |
| SLA success rate | `SLA-successful completed tasks / total tasks × 100`; unschedulable tasks therefore count against the rate. |
| Average queue wait | Mean of `start_time - arrival_time` over completed tasks. |
| P95 queue wait | Nearest-rank percentile: `ceil(0.95 × n) - 1` after sorting. |
| Average runtime | Mean of `finish_time - start_time`. |
| Makespan | `latest finish - earliest arrival`. |
| Throughput | `completed / (makespan / 60)` tasks per minute. |
| CPU utilization | `busy CPU-seconds / available CPU-seconds × 100`. |
| Memory utilization | `busy memory-seconds / available memory-seconds × 100`. |
| Average/P95 decision latency | Wall-clock duration of `SelectWorker`, in milliseconds. |
| Assignments by worker | Count of scheduled tasks for each worker. |
| Worker balance score | `1 - clamp(stddev(assignments) / mean(assignments), 0, 1)`, clamped to `[0,1]`. |

The simulator estimates task runtime as:

```text
runtime = tau × worker_task_speed_factor × (1 + 0.35 × worker_load) × jitter
```

so worker selection can change completion time, SLA success, queueing, utilization, and throughput.

Sources: `master/internal/benchmark/benchmark.go`, lines 72-104, 416-428, 572-587, 611-625, 656-709, 1235-1266, and 1330-1355.

The Go benchmark also has a composite RTS-vs-RR winner score:

```text
score = 0.45 × SLA_success_rate
      + 0.20 × normalized_throughput
      + 0.15 × inverse_P95_wait_score
      + 0.10 × CPU_utilization
      + 0.10 × worker_balance_score
```

That score belongs to the legacy RTS/RR benchmark comparison; it is **not the PPO training reward**.

---

## 12. Operational Prometheus metrics relevant to PPO

The master currently exports scheduler-neutral metrics labeled by scheduler where appropriate:

| Prometheus metric | Type / calculation | Operational use |
|---|---|---|
| `cloudai_master_queue_depth` | Gauge of tasks waiting | Detect backlog and saturation. |
| `cloudai_master_tasks_enqueued_total{reason}` | Counter | Understand why tasks enter the queue. |
| `cloudai_master_tasks_dequeued_total{outcome}` | Counter | Measure assignment/cancellation throughput. |
| `cloudai_master_scheduling_latency_seconds{scheduler}` | Histogram of worker-selection wall time | Compare PPO RPC/inference overhead with local schedulers. |
| `cloudai_master_task_queue_wait_seconds{scheduler,task_type}` | Histogram of queue residence time | Compare assignment latency by scheduler/workload. |
| `cloudai_master_scheduler_selections_total{scheduler,task_type,worker_id}` | Counter | Analyze decision distribution and hot spots. |
| `cloudai_master_task_terminal_total{status,task_type}` | Counter | Compute outcome rates. |
| `cloudai_master_task_requeues_total{failure_reason,task_type}` | Counter | Measure recovery pressure. |
| `cloudai_master_worker_timeouts_total{worker_id}` | Counter | Detect worker instability affecting scheduler results. |
| `cloudai_master_stale_results_total{reason}` | Counter | Detect late/duplicate attempts. |
| `cloudai_master_recovery_duration_seconds{failure_reason}` | Histogram | Quantify recovery time. |

The observability exporter derives maximum queue depth, counter increases, P95 scheduling latency, and P95 queue wait with PromQL over the run window.

Source: `master/internal/metrics/metrics.go`; `testbench/scripts/export_metrics.py`, lines 16-28 and 88-109.

Some documentation mentions `scheduler_decisions_total{algorithm="ppo"}`, `ppo_online_updates_total`, and `ppo_model_version`. Those exact PPO-specific Prometheus instruments are **not defined in the current metrics recorder**. Current selection counting uses `cloudai_master_scheduler_selections_total`, and online update/model version are available through service state/logs rather than those claimed gauges/counters.

---

## 13. What each metric changes in scheduler behavior

| Signal increases when... | Policy pressure / practical effect |
|---|---|
| Feasibility is high | The policy learns to stay within hard CPU, memory, and storage limits. |
| Headroom is high | The policy preserves spare capacity and reduces immediate saturation. |
| Queue pressure is high | Reward falls; the policy is pushed toward less-loaded workers for long/SLA-sensitive tasks. |
| Tail pressure is high | Reward falls sharply; estimated SLA breaches dominate soft objectives. |
| Above-mean imbalance is high | Reward falls; hot workers become less attractive. |
| Delta imbalance is positive | Reward falls; actions that newly worsen cluster-wide balance are discouraged. |
| Requeue count is high | Reward falls slightly; recovered work is treated more cautiously. |
| Live task succeeds and meets SLA | Online reward rises; the selected task-worker pattern is reinforced. |
| Live runtime grows | Online reward falls, up to a bounded penalty. |
| Entropy grows | PPO loss rewards exploration, preventing early overcommitment to one worker. |
| Scheduling latency grows | It does not alter reward, but flags PPO service/RPC overhead operationally. |
| Success rate ties but turnaround/wait improves | The PPO policy is judged better in deployment comparisons. |

---

## 14. Current gaps and cautions

1. **Reward weights lack a coefficient ablation.** The priorities are defensible, and model/hyperparameter sweeps were run, but the repository does not contain an experiment varying each reward coefficient independently.
2. **Offline and online objectives differ.** Offline training explicitly rewards headroom and balance; online updates optimize status, SLA, and runtime. Extended online learning can therefore shift the policy away from the offline balance objective.
3. **SLA urgency direction in serving bias should be reviewed.** Larger `k` means a looser deadline, while the serving score currently gives it higher “urgency.”
4. **Current PPO state has no queue-depth feature.** Queue effects are represented indirectly through available resources/load in state and through reward feedback.
5. **Live CPU/memory telemetry fields are transmitted but ignored by Python feature extraction.** PPO derives utilization from total minus available capacity; `current_cpu_usage` and `current_memory_usage` in the gRPC candidate are not part of the 9 worker features.
6. **Equal averaging of resource ratios can hide a dominant bottleneck.** Feasibility protects correctness, but reward quality may still understate a worker whose one resource is nearly full.
7. **No dedicated agentic scheduler unit tests are present.** Formula regressions currently depend mainly on integration runs and documentation evidence. Unit tests for reward terms, lifecycle release, online reward branches, feature parity, and percentile calculations would make future changes safer.
8. **Old documentation is inconsistent with code.** In particular, the 14-fixed-feature/reward block in `docs/BENCHMARK_RESULTS.md` should be treated as historical, not current.
9. **Some optimization evidence is no longer in this checkout.** `docs/PPO_PERFORMANCE_OPTIMIZATION.md` names JSON files under `results/`, but the current worktree has no `results/` directory. The artifacts remain visible in Git history, but restoring or regenerating them would improve reproducibility.

---

## 15. Concise answer to “how did we come up with the reward?”

We started from the minimum scheduling requirement—choose a worker that can fit the task—and used a positive feasible-placement reward plus a strong infeasible penalty. Real trace replay then exposed the objectives that feasibility alone misses: queue delay, turnaround beyond the SLA budget, hot-spotting, available headroom, and recovery history. These were normalized or capped so tasks with different durations and a few outliers would not dominate learning. SLA-tail risk received the strongest soft penalty because live scheduler ranking prioritizes successful, timely completion. Balance terms were kept secondary so the scheduler does not sacrifice SLA merely to distribute assignments evenly.

The function was then corrected using empirical trace behavior: memory units were fixed, resource decay was replaced by explicit lifecycles, and change in load standard deviation was added after the original imbalance term failed to penalize actions that turn a balanced cluster into an unbalanced one. Finally, candidate models were screened using held-out replay reward and feasibility and validated with repeated live success, turnaround, queue-wait, and duration measurements.

So the reward is best described as a **hierarchical, engineered multi-objective reward refined through trace diagnosis and live validation**. Its structure has strong project-specific reasoning; its exact coefficients are heuristic and should be treated as tunable hyperparameters rather than mathematically unique constants.

## 16. Primary implementation references

- State features and feasibility: `agentic_scheduler/features.py`
- Actor-critic model, normalization, PPO losses, serving bias: `agentic_scheduler/model.py`
- Synthetic reward: `agentic_scheduler/training/scheduler_env.py`
- Trace reward and lifecycle accounting: `agentic_scheduler/training/trace_replay_env.py`
- Trace normalization and task metrics: `agentic_scheduler/training/trace_loader.py`
- Offline training, GAE, and reward logging: `agentic_scheduler/train_ppo.py`
- Online outcomes and updates: `agentic_scheduler/service.py`
- Master-side outcome reward: `master/internal/server/master_server.go`
- PPO integration/fallback: `master/internal/scheduler/ppo_scheduler.go`
- Live campaign metrics: `testbench/scripts/run_campaign.py`
- Simulation benchmark metrics: `master/internal/benchmark/benchmark.go`
- Prometheus metrics: `master/internal/metrics/metrics.go`
- Reward-design history: `agentic_scheduler/TRAINING_DECISIONS.md`
- Optimization evidence and metric hierarchy: `docs/PPO_PERFORMANCE_OPTIMIZATION.md`
