# Agentic Scheduler — PPO Training Architecture

> **Purpose of this document.** This is the single, comprehensive technical reference
> for the reinforcement-learning subsystem of the `agentic_scheduler`. It covers
> every formula, every tensor shape, every hyperparameter, and every code path —
> from raw CSV traces through neural-network inference to online weight updates in
> production. If you are preparing for a viva, interview, or deep code review,
> this document should answer every question you encounter.

---

## Table of Contents

1. [Introduction & Philosophy](#1-introduction--philosophy)
2. [Neural Network Architecture](#2-neural-network-architecture)
3. [Feature Engineering](#3-feature-engineering)
4. [Data Sources & Trace Loading](#4-data-sources--trace-loading)
5. [Training Environments](#5-training-environments)
6. [Offline Training Pipeline](#6-offline-training-pipeline)
7. [Online Learning Pipeline](#7-online-learning-pipeline)
8. [Model Persistence & Versioning](#8-model-persistence--versioning)
9. [gRPC Server & Protocol](#9-grpc-server--protocol)
10. [Reward Curve Interpretation](#10-reward-curve-interpretation)
11. [Hyperparameter Reference](#11-hyperparameter-reference)
12. [End-to-End Worked Example](#12-end-to-end-worked-example)
13. [Thread Safety & Concurrency](#13-thread-safety--concurrency)
14. [Frequently Asked Questions](#14-frequently-asked-questions)

---

## 1. Introduction & Philosophy

### 1.1 What is the `agentic_scheduler`?

The `agentic_scheduler` is a **neural-network-based cluster scheduler** that
decides, in real time, which physical worker machine should execute each incoming
computational task (container, batch job, training run). Instead of relying on
hand-written heuristics (`round-robin`, `least-loaded`, `best-fit`), it uses a
learned policy trained with Reinforcement Learning to discover non-obvious
scheduling strategies directly from data.

### 1.2 Why Reinforcement Learning?

Cluster scheduling is a **sequential decision problem under uncertainty**: every
placement changes the state of the cluster and affects the quality of all future
placements. This makes it a natural fit for RL, where an agent learns by
interacting with an environment and maximizing cumulative reward.

Key advantages over rule-based schedulers:

| Property | Rules-Based | RL (PPO) |
|---|---|---|
| Adapts to workload drift | ❌ Manual re-tuning | ✅ Continuous online learning |
| Multi-objective balancing | ❌ Fixed priority ordering | ✅ Learns non-linear trade-offs |
| Handles combinatorial state | ❌ Exponential rules | ✅ Neural network generalises |
| Tail-latency awareness | ❌ Ignored or ad-hoc | ✅ Built into reward signal |

### 1.3 Why PPO Specifically?

We chose **Proximal Policy Optimization** (Schulman et al., 2017) for three reasons:

1. **Stability.** PPO's clipped surrogate objective prevents catastrophic policy
   updates — critical for a system that will be deployed to production.
2. **Sample efficiency.** PPO reuses rollout data for multiple gradient updates
   (controlled by `--ppo-epochs`), extracting more learning per environment step.
3. **Simplicity.** Compared to SAC, TD3, or IMPALA, PPO has fewer moving parts
   (no replay buffer in offline mode, no target networks), making it easier to
   debug and audit in a thesis/production context.

### 1.4 The Sim-to-Real Transfer Strategy

```
 ┌────────────────────────────────────────────────────────┐
 │                   OFFLINE PHASE                        │
 │  Alibaba/Google traces ──► TraceReplayEnv ──► PPO      │
 │  Millions of simulated decisions, zero risk            │
 │  Output: trained checkpoint (.pt file)                 │
 └────────────────────────┬───────────────────────────────┘
                          │  deploy checkpoint
                          ▼
 ┌────────────────────────────────────────────────────────┐
 │                   ONLINE PHASE                         │
 │  Live gRPC requests ──► PPOServiceCore ──► decisions   │
 │  Real outcomes ──► replay buffer ──► mini PPO updates  │
 │  Model continuously adapts to production workloads     │
 └────────────────────────────────────────────────────────┘
```

1. **Train offline** on historical cluster traces (Alibaba, Google, or own CloudAI
   history) inside a safe simulator (`TraceReplayEnv`). The agent makes millions
   of mistakes without affecting real users.
2. **Deploy the checkpoint** to the live gRPC service. The model operates with
   pre-learned expertise from day one.
3. **Continue learning online** using real task outcomes reported back through
   `ReportOutcome`. A small replay buffer and lightweight PPO updates keep the
   model adapting to workload drift, new hardware, and changing SLA requirements.

### 1.5 Actor-Critic Architecture Rationale

The model follows the **Actor-Critic** paradigm:

* **Actor (Policy Head)** — outputs a probability distribution over workers for
  each task. Trained to maximise expected cumulative reward.
* **Critic (Value Head)** — estimates the expected future return from the
  current state. Provides a *baseline* that reduces gradient variance, making
  training dramatically more stable than pure policy-gradient methods (REINFORCE).

Both heads share a common encoder, which forces the network to learn a general
"cluster understanding" representation rather than two independent feature sets.

---

## 2. Neural Network Architecture

**Source:** `model.py` — class `PPOActorCritic` (~line 80)

### 2.1 Layer Structure

```
 Input: task[5] ⊕ worker[9] = pairwise[14]   (per worker)
 ────────────────────────────────────────────────────────
                        │
              ┌─────────▼──────────┐
              │  Linear(14, 128)   │   1,920 params (14×128 + 128)
              │       ReLU         │
              │  Linear(128, 128)  │  16,512 params (128×128 + 128)
              │       ReLU         │
              └──────┬──────┬──────┘
                     │      │
         ┌───────────▼──┐   │
         │  Policy Head │   │  hidden: [B, W, 128]
         │ Linear(128,1)│   │     129 params
         │  squeeze(-1) │   │
         │  → logits    │   │  output: [B, W]
         │  [B, W]      │   │
         └──────────────┘   │
                            │
              ┌─────────────▼──────────────┐
              │  mean-pool over W dim      │
              │  pooled: [B, 128]          │
              │  Value Head                │
              │  Linear(128, 1) → squeeze  │
              │     129 params             │
              │  output: scalar [B]        │
              └────────────────────────────┘
```

### 2.2 Parameter Count

| Component | Weight Shape | Bias Shape | Parameters |
|---|---|---|---|
| Encoder layer 1 | (14, 128) | (128,) | 1,920 |
| Encoder layer 2 | (128, 128) | (128,) | 16,512 |
| Policy head | (128, 1) | (1,) | 129 |
| Value head | (128, 1) | (1,) | 129 |
| **Total** | | | **18,690** |

This is intentionally tiny. A small model ensures sub-millisecond inference
(critical for real-time scheduling), fits entirely in CPU cache during rollout,
and is resistant to overfitting on limited trace data.

### 2.3 Forward Pass — Tensor Shapes

```python
def forward(self, task_features, worker_features, action_mask=None):
    # task_features:    [B, 5]          (batch of tasks)
    # worker_features:  [B, W, 9]       (batch of worker sets)
    # action_mask:      [B, W]          (boolean feasibility)

    expanded_task = task_features.unsqueeze(1).expand(B, W, 5)   # [B, W, 5]
    pairwise = cat([expanded_task, worker_features], dim=-1)      # [B, W, 14]
    hidden = self.encoder(pairwise)                               # [B, W, 128]
    logits = self.policy_head(hidden).squeeze(-1)                 # [B, W]
    pooled = hidden.mean(dim=1)                                   # [B, 128]
    values = self.value_head(pooled).squeeze(-1)                  # [B]

    if action_mask is not None:
        logits = logits.masked_fill(~action_mask.bool(), -1e9)    # [B, W]

    return logits, values  # [B, W], [B]
```

**Why pairwise encoding?** Each (task, worker) pair is independently encoded,
producing a per-worker hidden representation. The policy head scores each worker
*in context of the task*, while the value head aggregates all workers via
mean-pooling to estimate the overall cluster state quality. This design naturally
handles variable numbers of workers — the model never assumes a fixed cluster size.

### 2.4 RunningNormalizer — Welford's Online Algorithm

**Source:** `model.py` — class `RunningNormalizer` (~line 20)

Features arrive with wildly different scales (CPU in 0.1–32, memory in 0.1–128,
storage in 50–1500). The `RunningNormalizer` maintains streaming statistics and
normalizes features to approximately zero mean and unit variance.

**Algorithm (Welford's online method for batched updates):**

```
For each batch of N samples with batch_mean and batch_var:
    δ = batch_mean - running_mean
    new_count = old_count + N
    running_mean += δ × (N / new_count)
    M₂ += batch_var × N + δ² × (old_count × N / new_count)
    running_count = new_count

Normalize:
    variance = M₂ / max(count - 1, 1)
    std = sqrt(max(variance, 1e-6))
    normalized = (x - running_mean) / std
```

**Why Welford's?** Unlike accumulating a global sum-of-squares, Welford's method
is numerically stable even after billions of samples. It handles single samples
and batches equally well, and the running statistics are persisted as part of
the model checkpoint so normalization is consistent between training and inference.

**State persistence:** The normalizer stores `{size, count, mean, m2}` in the
checkpoint dict. When a checkpoint is loaded (offline resume or online
deployment), the normalizer continues where it left off — no re-warming required.

---

## 3. Feature Engineering

**Source:** `features.py`

### 3.1 Task Features (TASK_FEATURE_DIM = 5)

```python
extract_task_features(task) → np.ndarray of shape [5]
```

| Index | Feature | Description | Typical Range |
|---|---|---|---|
| 0 | `req_cpu` | CPU cores requested | 0.1 – 32.0 |
| 1 | `req_memory` | Memory in GB | 0.1 – 128.0 |
| 2 | `req_storage` | Storage in GB | 0.1 – 1500.0 |
| 3 | `sla_multiplier` | SLA budget = runtime × sla_mult | 1.0 – 5.0 |
| 4 | `task_type_scalar` | Encoded task type | 0.0 – 1.0 |

**Task type encoding** (via `task_type_to_scalar`):

| Task Type | Integer ID | Scalar |
|---|---|---|
| `cpu-light` | 0 | 0.000 |
| `cpu-heavy` | 1 | 0.333 |
| `memory-heavy` | 2 | 0.667 |
| `mixed` | 3 | 1.000 |

### 3.2 Worker Features (WORKER_FEATURE_DIM = 9)

```python
extract_worker_features(worker) → np.ndarray of shape [9]
```

| Index | Feature | Description |
|---|---|---|
| 0 | `available_cpu / total_cpu` | CPU headroom fraction |
| 1 | `available_memory / total_memory` | Memory headroom fraction |
| 2 | `available_storage / total_storage` | Storage headroom fraction |
| 3 | `total_cpu` | Absolute CPU capacity |
| 4 | `total_memory` | Absolute memory capacity |
| 5 | `total_storage` | Absolute storage capacity |
| 6 | `used_cpu / total_cpu` | CPU utilization fraction |
| 7 | `used_memory / total_memory` | Memory utilization fraction |
| 8 | `used_storage / total_storage` | Storage utilization fraction |

**Safe division:** All ratios use `_safe_div(a, max(b, 1e-6))` to prevent
division by zero when a worker reports zero capacity.

**Design note:** Features 0–2 and 6–8 are complementary (headroom + utilization
= 1.0 ideally). Both are included because after normalization they carry
different gradient signals — the model can learn to weight headroom vs.
utilization differently depending on task urgency.

### 3.3 Action Mask

```python
is_worker_feasible(task, worker) → bool
```

A worker is feasible if and only if:
1. `worker.is_active == True`
2. `worker.available_cpu >= task.req_cpu`
3. `worker.available_memory >= task.req_memory`
4. `worker.available_storage >= task.req_storage`

Infeasible workers are masked to `-1e9` in the logits, making them effectively
impossible to select through softmax. This is a hard constraint, not a learned
penalty — the model *cannot* place a task on a machine that lacks resources.

### 3.4 Pairwise Concatenation & Normalization Pipeline

```
 Raw task[5]  ──────────────────────────────────────────┐
                                                        │ repeat W times
 Raw workers[W, 9] ──────────────────────────────────── ┤
                                                        │
              ┌─────────────────────────────────────────▼─────┐
              │  Concatenate: pairwise_rows[W, 14]            │
              │  = [task[5] | worker_i[9]] for each worker i  │
              └────────────────┬──────────────────────────────┘
                               │
              ┌────────────────▼──────────────────────────────┐
              │  RunningNormalizer.update(pairwise_rows)       │
              │  → updates streaming mean/variance             │
              │  RunningNormalizer.normalize(pairwise_rows)    │
              │  → normalized_rows[W, 14]                      │
              └────────────────┬──────────────────────────────┘
                               │
              ┌────────────────▼──────────────────────────────┐
              │  Split back:                                   │
              │    normalized_task = normalized_rows[0, :5]     │
              │    normalized_workers = normalized_rows[:, 5:]  │
              └────────────────────────────────────────────────┘
```

### 3.5 EncodedRequest Dataclass

```python
@dataclass
class EncodedRequest:
    task_features:   np.ndarray   # shape [5]
    worker_features: np.ndarray   # shape [W, 9]
    action_mask:     np.ndarray   # shape [W], dtype=bool
```

Created by `encode_request(task, workers)` which extracts features for all
workers, stacks them, computes the action mask, and returns the bundle ready for
`choose_action()`.

---

## 4. Data Sources & Trace Loading

**Source:** `training/trace_loader.py`

### 4.1 Canonical Data Model

**TraceTask** — a single scheduling event:

| Field | Type | Default | Description |
|---|---|---|---|
| `task_id` | str | — | Unique identifier |
| `arrival_time` | float | — | Seconds from trace start |
| `req_cpu` | float | — | CPU cores requested |
| `req_memory` | float | — | Memory GB requested |
| `req_storage` | float | 1.0 | Storage GB requested |
| `runtime_seconds` | float | 30.0 | Actual execution duration |
| `task_type` | str | "mixed" | One of 4 canonical types |
| `sla_multiplier` | float | 2.0 | SLA budget multiplier |
| `queue_wait_seconds` | float | 0.0 | Time spent queued |
| `sla_success` | Optional[bool] | None | Whether SLA was met |
| `failure_reason` | str | "" | Why the task failed |
| `requeue_count` | int | 0 | Number of re-queues |
| `worker_snapshot` | Dict | {} | Worker state at scheduling time |

**TraceCluster** — a complete trace dataset:

| Field | Type | Description |
|---|---|---|
| `workers` | List[Dict] | List of {worker_id, total_cpu, total_memory, total_storage} |
| `tasks` | List[TraceTask] | Tasks sorted by arrival_time |
| `source` | str | Origin identifier (e.g., "alibaba-v2018") |
| `description` | str | Human-readable summary |
| `trace_window` | str | Time window label |
| `metadata` | Dict | Additional trace metadata |

### 4.2 Alibaba cluster-trace-v2018

**Function:** `load_alibaba_trace(trace_dir, max_tasks=5000)`

**Input files:**
- `batch_task.csv` — task definitions (columns: task_name, instance_num, job_name, task_type, status, start_time, end_time, plan_cpu, plan_mem)
- `machine_meta.csv` — machine specifications (columns: machine_id, time_stamp, failure_domain_1, failure_domain_2, cpu_num, mem_size, status)

**CPU normalization** (`_normalize_alibaba_cpu`):
```
if plan_cpu <= 0:     return 0.1         (floor)
if plan_cpu > 10.0:   return plan_cpu / 100.0   (centi-cores → cores)
else:                 return plan_cpu     (already in cores)
All values clamped ≥ 0.1
```

**Memory normalization** (`_normalize_alibaba_memory`):
```
if plan_mem <= 0:     return 0.1
if plan_mem <= 1.0:   return plan_mem × 64.0 GB   (fraction of 64 GB host)
else:                 return plan_mem              (already in GB)
All values clamped ≥ 0.1
```

**Alibaba task type mapping** (12 Alibaba types → 4 canonical types):

| Alibaba Type | Canonical Type |
|---|---|
| 1, 8 | cpu-light |
| 2, 4, 10 | cpu-heavy |
| 3, 5 | memory-heavy |
| 6, 7, 9, 11, 12 | mixed |

**Timestamps:** Already in seconds (no conversion needed). The Alibaba v2018
trace covers 8 days of a 4,000-machine cluster.

### 4.3 Google ClusterData2019

**Function:** `load_google_trace(trace_dir, max_tasks=5000)`

**Input files:**
- `instance_events.json` (or `instance_events.csv`) — task instance events
- `machine_events.json` (or `machine_events.csv`) — machine specifications

**Key differences from Alibaba:**
- **Timestamps:** microseconds — divided by `1e6` to get seconds
- **Memory:** normalized fraction — multiplied by `32 GB` to get approximate GB
- **Task types:** auto-classified via `_classify_task_type(cpu, memory)` since
  Google traces don't include our canonical type labels

### 4.4 CloudAI Own History

**Function:** `load_cloudai_trace(trace_path, max_tasks, mongo_uri, ...)`

**Data sources (in order of preference):**
1. **MongoDB** (when `mongo_uri` is provided):
   - `TASKS` collection — with optional time windowing via `submitted_at` queries
   - `RESULTS` collection — SLA success, status, completion timestamps
   - `WORKER_REGISTRY` collection — machine specifications
2. **File fallback** (when `trace_path` is provided):
   - Searches for: `cloudai_trace.{json,jsonl,csv}`, `history.{json,jsonl,csv}`, `tasks.{json,jsonl,csv}`
   - Worker data from: `workers.json`, `worker_registry.json`, `machine_meta.csv`

**Field precedence** — CloudAI data uses multiple fallback field names to handle
different schema versions:
```
task_id:        task_id → id
arrival_time:   submitted_at → arrival_time → created_at → time
req_cpu:        req_cpu → cpu_request → cpu
req_memory:     req_memory → memory_request → memory
runtime:        actual_runtime → runtime_seconds → (completed_at - started_at)
queue_wait:     queue_wait_seconds → queue_wait → (started_at - arrival_time)
requeue_count:  recovery_count → requeue_count
```

### 4.5 Task Type Classification Heuristic

When trace data doesn't include task types (Google, some CloudAI records), the
`_classify_task_type(req_cpu, req_memory)` function auto-classifies:

```
cpu_mem_ratio = req_cpu / max(req_memory, 0.01)

if ratio > 3.0:                           → "cpu-heavy"
if ratio < 0.3:                           → "memory-heavy"
if req_cpu < 1.0 AND req_memory < 1.0:    → "cpu-light"
else:                                      → "mixed"
```

### 4.6 Post-Processing (All Loaders)

Every loader applies `_normalise_tasks()`:
1. **Sort** by `arrival_time` ascending
2. **Cap** at `max_tasks` (default 5000) — keeps only the earliest N tasks
3. **Offset-normalize** timestamps — subtract the first task's arrival time so
   the trace starts at `t = 0.0`

### 4.7 Unified Loader

```python
load_trace(trace_path, source="alibaba"|"google"|"cloudai", max_tasks=5000, ...)
→ TraceCluster
```

A single entry point that dispatches to the appropriate loader based on the
`source` argument. Used by `train_ppo.py` via `--trace-source`.

---

## 5. Training Environments

### 5a. SchedulingEnv (Synthetic)

**Source:** `training/scheduler_env.py`

A Gymnasium environment for **bootstrapping** when no real traces are available.
Useful for smoke tests, CI/CD validation, and initial policy exploration.

**Random task generation:**
```
req_cpu:     Uniform(0.5, 12.0)    — boosted for cpu-heavy tasks to Uniform(6.0, 20.0)
req_memory:  Uniform(0.5, 32.0)    — boosted for memory-heavy tasks to Uniform(16.0, 64.0)
req_storage: Uniform(0.5, 80.0)
sla_mult:    Uniform(1.5, 2.5)
task_type:   Uniform random from {cpu-light, cpu-heavy, memory-heavy, mixed}
```

**Random worker generation:**
```
total_cpu:     Uniform(4.0, 32.0)
total_memory:  Uniform(8.0, 128.0)
total_storage: Uniform(100.0, 1500.0)
```

**Episode:** 96 steps. After each step, all worker loads decay:
```
used_cpu     *= 0.85
used_memory  *= 0.85
used_storage *= 0.90
```

**Reward function:**
```
If feasible:
    load_penalty = min((cpu_util + mem_util + stor_util) / 3.0,  1.5)
    reward = 1.2 - load_penalty
If infeasible:
    reward = -1.4

Range: [-1.4, 1.2]
```

The reward is simple by design — it teaches the agent the basics (don't overload,
prefer balanced placement) before graduating to the complex trace-based reward.

### 5b. TraceReplayEnv (Real Traces)

**Source:** `training/trace_replay_env.py`

Replays a `TraceCluster`'s tasks in chronological order. This is where the agent
encounters realistic workload distributions, bursty arrivals, and SLA pressures.

**Time-based load decay:**
```
dt = next_task.arrival_time - prev_task.arrival_time
decay_factor = e^(-dt / 30)       // τ = 30 seconds, half-life ≈ 20.8 seconds

worker.used_cpu     *= decay_factor
worker.used_memory  *= decay_factor
worker.used_storage *= decay_factor
```

Unlike the synthetic env's fixed 15% decay per step, the trace env decays
resources proportionally to real elapsed time, so long gaps between arrivals
release more capacity naturally.

**Looping:** When `loop=True` (the default), the trace restarts from the
beginning when exhausted, resetting all worker loads. This allows training runs
longer than the trace itself.

**Reward function** (multi-component, `_quality_reward`):
```
If feasible:
    projected_load = normalised_load(selected_worker)     # avg util fraction
    cluster_load   = mean(normalised_load(all_workers))

    # Queue & turnaround proxies
    queue_wait_proxy     = trace_queue_wait + (runtime × projected_load)
    turnaround_proxy     = queue_wait_proxy + runtime
    sla_budget           = max(runtime × sla_multiplier, 1.0)

    # Pressure signals
    queue_pressure       = min(queue_wait_proxy / sla_budget, 3.0)
    turnaround_pressure  = min(turnaround_proxy / sla_budget, 4.0)
    tail_pressure        = max(turnaround_pressure - 1.0, 0.0)
    imbalance_penalty    = max(projected_load - cluster_load, 0.0)
    headroom_bonus       = 1.0 - projected_load
    requeue_penalty      = min(requeue_count, 4) × 0.05

    reward = 1.4
           + 0.25 × headroom_bonus          # prefer workers with capacity
           - 0.35 × queue_pressure           # penalise estimated queue delay
           - 0.55 × tail_pressure            # heavily penalise SLA breaches
           - 0.20 × imbalance_penalty        # penalise load imbalance
           - requeue_penalty                 # penalise re-queued tasks

If infeasible:
    reward = -1.8

Approximate range: [-2.3, 1.65]
```

**Why so many terms?** Each component targets a different scheduling pathology:
- `headroom_bonus` encourages choosing workers with spare capacity
- `queue_pressure` penalises placements that will cause queuing delays
- `tail_pressure` is the "SLA cop" — strongly penalises turnaround > SLA budget
- `imbalance_penalty` prevents the agent from always choosing the same few workers
- `requeue_penalty` discourages placements on workers with a history of failures

The coefficients (0.25, 0.35, 0.55, 0.20, 0.05) were tuned empirically to
produce a smooth reward landscape that guides the agent without creating
degenerate local optima.

---

## 6. Offline Training Pipeline

**Source:** `train_ppo.py` — function `main()` (~line 345)

### 6.1 The Two-Phase Training Loop

The training process operates in a continuous, cyclic loop comprising two
distinct phases. This cycle repeats for `--updates` iterations (default 200).

#### Phase 1: Rollout (Data Collection)

**Hardware profile:** CPU-bound. GPU mostly idle.

1. `TraceReplayEnv` (or `SchedulingEnv`) presents the next task.
2. `choose_action()` runs the neural network on a **single** (task, workers) pair
   (batch size 1). During offline training, a CPU-resident copy of the model
   (`rollout_model`) is used to avoid GPU transfer overhead.
3. The environment calculates the consequences — updates simulated worker loads,
   computes the reward.
4. The transition `{task_features, worker_features, action_mask, action, old_log_prob, old_value, reward, done}` is appended to the rollout buffer.
5. Repeat for `--rollout-steps` iterations (default 1024).

**Why a CPU rollout model?** Transferring a single sample to/from GPU is slower
than computing the forward pass on CPU for this tiny network. The model weights
are synced from GPU after each PPO update:
```python
rollout_model.load_state_dict({k: v.cpu() for k, v in state.model.state_dict().items()})
```

#### Phase 2: PPO Update (Optimization)

**Hardware profile:** GPU-bound. CPU mostly idle.

1. Compute **GAE** (Generalized Advantage Estimation) over the collected rollout.
2. Normalize advantages to zero mean and unit variance.
3. Pack transitions into GPU tensors.
4. Run `ppo_update()` — shuffle and slice into minibatches, perform `--ppo-epochs`
   (default 6) full sweeps over the data.
5. Sync the updated weights back to the CPU rollout model.

**Summary of a single update cycle:**
```
 ┌─────────────────────────────────┐
 │  Phase 1: Rollout               │  ~5-40s depending on rollout_steps
 │  1024 sequential env.step()     │  CPU-bound
 │  choose_action() on CPU model   │
 └───────────────┬─────────────────┘
                 │ transitions buffer
 ┌───────────────▼─────────────────┐
 │  GAE computation                │  ~1ms (vectorized numpy)
 │  Advantage normalization        │
 └───────────────┬─────────────────┘
                 │
 ┌───────────────▼─────────────────┐
 │  Phase 2: PPO Update            │  ~1-10s depending on minibatch/epochs
 │  ppo_update() on GPU            │  GPU-bound
 │  6 epochs × (1024/256) = 24     │
 │  minibatch gradient steps       │
 └───────────────┬─────────────────┘
                 │
 ┌───────────────▼─────────────────┐
 │  Sync CPU model, log metrics    │
 │  Optional checkpoint save       │
 └─────────────────────────────────┘
```

### 6.2 CLI Arguments

| Argument | Default | Description |
|---|---|---|
| `--trace-source` | "" | `alibaba`, `google`, `cloudai`, or empty for synthetic |
| `--trace-path` | "" | Path to trace data directory |
| `--max-trace-tasks` | 5000 | Max tasks to load from trace |
| `--trace-window` | "" | Optional trace window label |
| `--trace-window-start` | "" | Window start (unix or ISO-8601) |
| `--trace-window-end` | "" | Window end (unix or ISO-8601) |
| `--num-workers` | 4 | Number of simulated workers |
| `--episode-length` | 96 | Steps per episode (synthetic only) |
| `--rollout-steps` | 1024 | Transitions per rollout |
| `--minibatch-size` | 256 | Minibatch size for PPO update |
| `--ppo-epochs` | 6 | PPO epochs per update |
| `--updates` | 200 | Total PPO update cycles |
| `--gamma` | 0.99 | Discount factor |
| `--gae-lambda` | 0.95 | GAE smoothing factor |
| `--learning-rate` | 3e-4 | Initial learning rate (Adam) |
| `--clip-ratio` | 0.2 | PPO clipping epsilon |
| `--entropy-coeff` | 0.01 | Entropy bonus coefficient |
| `--value-coeff` | 0.5 | Value loss coefficient |
| `--value-clip-range` | 0.2 | Value function clipping range |
| `--lr-anneal` | True | Enable linear LR annealing |
| `--seed` | 42 | Random seed |
| `--output` | `agentic_scheduler/models/ppo_trained.pt` | Final checkpoint path |
| `--checkpoint-dir` | `agentic_scheduler/models/checkpoints` | Periodic checkpoint directory |
| `--checkpoint-prefix` | `ppo_offline` | Checkpoint filename prefix |
| `--checkpoint-every` | 10 | Save every N updates (≤0 disables) |
| `--resume-from` | "" | Resume from explicit checkpoint |
| `--resume-latest` | False | Resume from latest checkpoint |
| `--resume-mongo` | False | Resume from Mongo checkpoint |
| `--mongo-uri` | "" | MongoDB connection URI |
| `--mongo-db` | `cluster_db` | MongoDB database name |
| `--fingerprint-hash` | "" | Cluster fingerprint hash |
| `--fingerprint-payload` | "" | Cluster fingerprint payload |
| `--mongo-checkpoint-every` | 0 | Mongo save interval (≤0 disables) |
| `--mongo-save-final` | True | Persist final checkpoint to Mongo |
| `--log-every` | 10 | Log metrics every N updates |

**Upper-bound safety limits:** `--num-workers ≤ 256`, `--rollout-steps ≤ 65536`, `--updates ≤ 100000`, `--episode-length ≤ 10000`.

### 6.3 Generalized Advantage Estimation (GAE)

**Code:** `train_ppo.py` → `generalized_advantage_estimation()` (~line 295)

The Advantage answers: *"Was the actual outcome better or worse than what the Critic predicted?"*

**Step 1 — TD-delta (Temporal Difference Error):**
```
δ_t = r_t + γ × V(s_{t+1}) × (1 - done_t) - V(s_t)
```

- `r_t` — actual reward at step t
- `γ × V(s_{t+1})` — discounted estimate of all future rewards. `γ = 0.99` means
  a reward 100 steps away is worth `0.99^100 ≈ 0.37` of an immediate reward.
- `V(s_t)` — what the Critic predicted. If `δ > 0`, outcome was better than
  expected (positive advantage). If `δ < 0`, it was worse.

**Step 2 — GAE accumulation (reverse scan):**
```
Â_t = δ_t + (γ × λ) × (1 - done_t) × Â_{t+1}
```

Rather than using raw one-step deltas, GAE accumulates an **exponentially-weighted
sum of deltas** going backwards through the rollout. `λ = 0.95` controls the
bias/variance trade-off:
- `λ = 0` → pure one-step TD (low variance, high bias)
- `λ = 1` → full Monte Carlo returns (high variance, low bias)
- `λ = 0.95` → empirically optimal middle ground

**Step 3 — Returns (Critic training targets):**
```
R_t = Â_t + V(s_t)
```

Returns represent "what the Critic *should* have said." The gap between `V(s_t)`
(prediction) and `R_t` (target) is what the Critic learns to close over time.

**Step 4 — Normalization:**
```
Â = (Â - mean(Â)) / (std(Â) + 1e-8)
```

Prevents one extremely good or bad decision from dominating the entire gradient
update and destabilizing training.

### 6.4 PPO Loss Functions

**Code:** `model.py` → `ppo_update()` (~line 252)

The total loss combines three components:

```
L_total = L_policy + value_coeff × L_value - entropy_coeff × H(π)
```

#### Policy Loss (Clipped Surrogate)

```
r_t = π_new(a|s) / π_old(a|s) = exp(log_prob_new - log_prob_old)

surrogate_1 = r_t × Â_t
surrogate_2 = clip(r_t, 1 - ε, 1 + ε) × Â_t       where ε = 0.2

L_policy = -E[min(surrogate_1, surrogate_2)]
```

This is the heart of PPO. The clipped surrogate prevents the policy from
changing too drastically in a single update:

- When `Â > 0` (good action): `min` caps the ratio at `1 + ε = 1.2`. The model
  gets rewarded for improving, but not too aggressively.
- When `Â < 0` (bad action): `min` caps the ratio at `1 - ε = 0.8`. The model
  gets punished, but not catastrophically.

**Why is this critical?** RL has no ground truth — a policy that changes too
drastically corrupts its own training data for future updates. PPO's clipping
is the guardrail that prevents this.

#### Value Loss (Clipped MSE)

```
V_clip = V_old + clip(V_new - V_old, -ε_v, ε_v)     where ε_v = 0.2

L_value_unclipped = (V_new - R_t)²
L_value_clipped   = (V_clip - R_t)²

L_value = E[max(L_value_unclipped, L_value_clipped)]
```

The `max` is pessimistic — it ensures the Critic only gets updated when the
clipped version is not conservative enough. This prevents the Critic from making
wild jumps that would distort advantage estimates.

#### Entropy Bonus

```
H(π) = -Σ_a π(a|s) log π(a|s)
```

High entropy = uncertain policy (good for exploration early in training).
Subtracting `-entropy_coeff × H(π)` from the loss effectively *rewards*
exploration, preventing the model from prematurely collapsing to a single
worker for every task.

### 6.5 Learning Rate Annealing

```python
frac = 1.0 - ((update_idx - 1) / (updates - 1))
current_lr = max(learning_rate × frac, 1e-6)
```

Linear decay from `3e-4` to `1e-6`. Early large steps escape random
initialization quickly; later small steps fine-tune without overshooting.

### 6.6 Gradient Clipping

```python
nn.utils.clip_grad_norm_(state.model.parameters(), max_norm=1.0)
```

Prevents any single minibatch from making huge weight changes. Essential with
chaotic real-world workloads where one anomalous batch could corrupt the policy.

### 6.7 Mixed Precision (Optional AMP)

When CUDA is available, training uses `torch.amp.GradScaler`:
```python
grad_scaler = torch.amp.GradScaler(device="cuda") if device.type == "cuda" else None
```

The forward pass runs in FP16 for speed, while gradients are scaled and
unscaled to prevent underflow. This approximately doubles GPU throughput
for the PPO update phase.

### 6.8 Checkpointing

- **Periodic local:** Every `--checkpoint-every` updates (default 10),
  saves `{prefix}_u{update:06d}_s{steps:06d}.pt` plus a `_latest.pt` symlink.
  Uses atomic write (temp file + `os.replace`) to prevent corruption.
- **Periodic Mongo:** Every `--mongo-checkpoint-every` updates (when configured),
  persists to MongoDB with version tracking.
- **Final:** Always saves to `--output` and optionally to Mongo.

---

## 7. Online Learning Pipeline

**Source:** `service.py` — class `PPOServiceCore`

### 7.1 Overview

The online pipeline runs inside the production gRPC service. It takes scheduling
decisions based on real requests, collects task outcomes, and performs lightweight
PPO updates to adapt the policy to the live cluster.

### 7.2 Decision Flow

```
 SelectWorkerRequest
       │
       ▼
 encode_request(task, workers)    → EncodedRequest
       │
       ▼
 choose_action(state, ...)        → action_info dict
       │                            {action_index, log_prob, value,
       │                             normalized_worker/task_features}
       ▼
 Store DecisionRecord             → pending_decisions[task_id]
       │
       ▼
 Return SelectWorkerResponse      → worker_id, reason, model_version
```

### 7.3 DecisionRecord

```python
@dataclass
class DecisionRecord:
    task_features:   np.ndarray   # [5]   normalized
    worker_features: np.ndarray   # [W, 9] normalized
    action_mask:     np.ndarray   # [W]   bool
    action_index:    int          # chosen worker index
    old_log_prob:    float        # log π(a|s) at decision time
    old_value:       float        # V(s) at decision time
    worker_id:       str          # selected worker's ID
```

### 7.4 Headroom Bias

**Parameter:** `deterministic_bias` (default 0.25)

In production (deterministic mode), the policy doesn't just `argmax` the
logits. Instead, it applies a **headroom bias** that blends the learned policy
with a capacity-aware heuristic:

1. Compute `_projected_headroom_scores()` for each worker — a composite score
   based on residual capacity, projected peak utilization, utilization spread,
   task size relative to median worker, and SLA urgency.
2. The score blends two strategies based on urgency:
   - **High urgency (tight SLA):** risk-aware score — prefer workers with the
     most headroom to minimize tail latency.
   - **Low urgency:** packing score — prefer workers that are already partially
     loaded to improve cluster utilization.
3. Take the policy's top-3 candidates, re-rank them by
   `policy_logits + bias × headroom_scores`, and select the winner.

This gives the learned policy final say (it determines the candidate set), while
the heuristic provides a tie-breaking safety net that prevents egregious
placements during early online learning when the policy may still be uncertain.

### 7.5 Outcome Reporting & Reward Derivation

When a task completes, `ReportOutcome` is called:

1. Pop the `DecisionRecord` from `pending_decisions`.
2. Derive reward (if not provided explicitly):

```python
def _derive_reward(status, runtime_seconds, sla_success):
    reward = 0.0

    if status in {"success", "completed"}:   reward += 1.0
    elif status == "cancelled":              reward -= 0.5
    else (failed/error/timeout):             reward -= 1.0

    if sla_success:   reward += 0.5
    else:             reward -= 0.25

    if runtime_seconds > 0:
        reward -= min(runtime_seconds / 600.0, 0.5)

    return reward    # Range: approximately [-1.75, 1.25]
```

3. Append transition to replay buffer with `done=True` for terminal outcomes
   (cancelled, failed, error, timeout, rejected).

### 7.6 Replay Buffer & Training Trigger

| Parameter | Value |
|---|---|
| Max replay buffer size | 4,096 transitions |
| Max pending decisions | 8,192 entries |
| Eviction policy (both) | FIFO — oldest evicted on overflow |
| Training trigger | `len(replay_buffer) >= update_batch_size` (default 32) |

### 7.7 Online PPO Update (`_train_from_replay_locked`)

When the replay buffer reaches `update_batch_size`:

1. **Stack** buffer into tensors (task_features, worker_features, masks, actions,
   old_log_probs, old_values, rewards, dones).
2. Force `dones[-1] = 1.0` (treat the buffer as a complete episode).
3. **Compute GAE** with online-tuned hyperparameters:
   - `online_gamma = 0.97` (lower than offline 0.99 — shorter horizon for
     adaptation speed)
   - `online_gae_lambda = 0.92` (lower than offline 0.95 — more bias, less
     variance for small batch sizes)
4. **Clamp** advantages to `[-8, 8]` (prevents outlier rewards from causing
   gradient explosions in the small online batch).
5. **Normalize** advantages (zero mean, unit variance).
6. **PPO update:** `clip_ratio=0.2, entropy_coeff=0.01, value_coeff=0.5, epochs=4`
7. **Clear** replay buffer.
8. **Persist** checkpoint to MongoDB and/or local disk.

---

## 8. Model Persistence & Versioning

**Source:** `persistence.py` — class `MongoSchedulerModelStore`

### 8.1 Storage Backend

- **MongoDB collection:** `SCHEDULER_MODELS` — version metadata
- **GridFS bucket:** `scheduler_models` — binary checkpoint storage

### 8.2 MongoDB Indexes

| Index Name | Fields | Properties |
|---|---|---|
| `sched_model_lookup_idx` | (scheduler_type ASC, fingerprint_hash ASC, version DESC) | Fast lookup |
| `sched_model_one_active_idx` | (scheduler_type ASC, fingerprint_hash ASC, active ASC) | Unique, partial filter `{active: true}` |
| `sched_model_created_idx` | (scheduler_type ASC, fingerprint_hash ASC, created_at DESC) | Timeline queries |

### 8.3 Document Schema

```javascript
{
    "scheduler_type":     "PPO",
    "fingerprint_hash":   "a1b2c3d4...",
    "fingerprint_payload": "<cluster topology JSON>",
    "version":            42,
    "active":             true,
    "file_id":            ObjectId("..."),        // GridFS reference
    "file_size":          245760,                 // bytes
    "file_sha256":        "e3b0c44298fc...",      // integrity check
    "framework":          "pytorch-ppo",
    "metadata":           { ... },                // extra: reason, training_steps, etc.
    "created_at":         ISODate("..."),
    "updated_at":         ISODate("..."),
    "activated_at":       ISODate("...")           // null until activated
}
```

**File naming:** `ppo_{fingerprint_hash}_v{version}.ckpt`

### 8.4 Save Flow

```
 state.checkpoint_payload()        → serialize model + optimizer + normalizer to bytes
       │
       ▼
 SHA-256 hash of checkpoint bytes  → integrity verification
       │
       ▼
 GridFS upload                     → file_id
       │
       ▼
 Insert metadata document          → active=False initially
       │
       ▼
 Deactivate all previous active    → update_many({active: True, _id ≠ new}, {active: False})
       │
       ▼
 Activate new document             → update_one({_id: new}, {active: True})
```

### 8.5 Load Flow

```
 Find active document              → {scheduler_type, fingerprint_hash, active: True}
       │
       ▼
 GridFS download                   → raw checkpoint bytes
       │
       ▼
 (SHA-256 verification available)  → compare with stored hash
       │
       ▼
 PPOState.from_checkpoint_bytes()  → reconstruct model, optimizer, normalizer
```

### 8.6 Version Activation (Rollback)

```python
store.activate_existing_version(scheduler_type, fingerprint_hash, version=N)
```

Deactivates all versions → activates version N. This supports instant rollback
to any previously saved model version.

### 8.7 Fingerprint Isolation

Different cluster topologies (different sets of workers) produce different
`fingerprint_hash` values. Each fingerprint gets its own model lineage — the
system automatically loads the correct model for the current cluster topology.
If no model exists for a fingerprint, a cold-start creates a fresh one.

### 8.8 Checkpoint Contents (`.pt` via `torch.save`)

```python
{
    "model_state_dict":     OrderedDict,    # all 18,690 parameters
    "optimizer_state_dict":  dict,           # Adam moments (first & second)
    "normalizer":            {               # RunningNormalizer state
        "size": 14,
        "count": 1234567,
        "mean": [0.1, 0.2, ...],            # 14 values
        "m2": [1.5, 2.3, ...]               # 14 values
    },
    "model_version":         "v42",
    "fingerprint_hash":      "a1b2c3d4...",
    "training_steps":        200,
    "lineage_metadata":      {
        "model_source": "offline-trace-replay",
        "training_corpus": "alibaba-v2018",
        "trace_window": "full",
        ...
    },
    "saved_at_unix":         1714070400.0
}
```

---

## 9. gRPC Server & Protocol

**Source:** `server.py`, `proto/ppo_scheduler_pb2.py`

### 9.1 RPC Methods

| RPC | Request | Response | Purpose |
|---|---|---|---|
| `Ping` | `PingRequest` | `PingResponse` | Health check |
| `LoadModelForFingerprint` | `LoadModelForFingerprintRequest` | `LoadModelForFingerprintResponse` | Load/create model for cluster |
| `SelectWorker` | `SelectWorkerRequest` | `SelectWorkerResponse` | Make scheduling decision |
| `ReportOutcome` | `ReportOutcomeRequest` | `ReportOutcomeResponse` | Report task completion |

### 9.2 Message Schemas

**SelectWorkerRequest:**

| Field | Type | Description |
|---|---|---|
| `task` | Task msg | Task requirements (req_cpu, req_memory, etc.) |
| `workers` | repeated CandidateWorker | Available workers |
| `cluster_fingerprint_hash` | string | Cluster topology hash |
| `cluster_fingerprint_payload` | string | Full topology description |
| `fallback_scheduler` | string | Fallback policy name |

**CandidateWorker:**

| Field | Type | Description |
|---|---|---|
| `worker_id` | string | Unique worker identifier |
| `worker_ip` | string | Network address |
| `is_active` | bool | Whether worker is accepting tasks |
| `available_cpu` | float | Currently available CPU cores |
| `available_memory` | float | Currently available memory GB |
| `available_storage` | float | Currently available storage GB |
| `total_cpu` | float | Total CPU capacity |
| `total_memory` | float | Total memory capacity |
| `total_storage` | float | Total storage capacity |
| `current_cpu_usage` | float | Current CPU utilization |
| `current_memory_usage` | float | Current memory utilization |

**SelectWorkerResponse:**

| Field | Type | Description |
|---|---|---|
| `worker_id` | string | Selected worker (empty if fallback) |
| `used_fallback_policy` | bool | True if PPO couldn't decide |
| `reason` | string | Decision explanation |
| `model_version` | string | Model version used |

**ReportOutcomeRequest:**

| Field | Type | Description |
|---|---|---|
| `task_id` | string | Task that completed |
| `worker_id` | string | Worker that ran the task |
| `status` | string | Outcome (success, failed, etc.) |
| `reward` | float | Explicit reward (0.0 = auto-derive) |
| `runtime_seconds` | float | Actual execution time |
| `sla_success` | bool | Whether SLA was met |
| `fingerprint_hash` | string | Cluster fingerprint for validation |
| `model_version` | string | Model version that made the decision |
| `task` | Task msg | Task details (for reward context) |

### 9.3 Server Configuration

| Setting | Value |
|---|---|
| Thread pool workers | 16 |
| Max workers per request | 512 |
| Graceful shutdown timeout | 5 seconds |
| Default listen address | `127.0.0.1:50061` |

### 9.4 Input Sanitization

All string inputs are truncated (`_sanitize_string`, max 256 chars for IDs, 4096
for payloads). Float inputs are clamped to `[-10, 10]` via `_clamp_float`, which
also maps `NaN` and `±Inf` to `0.0`. This prevents malformed gRPC requests from
corrupting the model or causing numeric instability.

---

## 10. Reward Curve Interpretation

After a training run, you can generate a reward curve by running:
```bash
./venv/bin/python agentic_scheduler/scripts/plot_training_curve.py \
    agentic_scheduler/logs/training_output.log \
    agentic_scheduler/results/training_reward_curve.png
```

This produces a chart like the one below:

![PPO Training Reward Curve](results/training_reward_curve.png)

### How to read the chart

The **X-axis** is the PPO update number (each update = one full rollout + one
GPU optimization pass). The **Y-axis** is the **average reward** over the most
recent 5,000 scheduling decisions.

**The cyan line** shows the raw `avg_reward` value logged at each update.
**The pink line** is a smoothed moving average that reveals the true trend.

### What does the reward value mean?

The reward is a composite score from `TraceReplayEnv` that combines:
- **Positive contributions:** Successfully placing a task on a machine with
  enough resources, tight resource packing, and meeting SLA deadlines.
- **Negative contributions:** Overloading a machine, violating resource
  constraints, queueing delays, and unbalanced cluster utilization.

A reward of **0.0** means breaking even — acceptable but not optimal. The
dashed gray line on the chart marks this baseline.

### Is a negative reward bad?

**Not necessarily.** A deeply negative reward at the start (e.g., `-1.34`) is
expected — the agent begins with randomly initialized weights and is essentially
flipping coins.

What matters is the **trend over time**:

| Curve Shape | What It Means | Good or Bad? |
|---|---|---|
| **Steadily rising toward 0** | Fewer bad placements. Policy converging. | ✅ Good |
| **Dips then recovers** | Tried aggressive packing, got penalized, adapted. | ✅ Normal |
| **Flat / oscillating** | Stuck in local optimum or LR too low. | ⚠️ Stalled — try adjusting `--entropy-coeff` or `--updates` |
| **Steadily falling** | Catastrophic forgetting or broken reward signal. | ❌ Bad — stop and investigate |
| **Crosses above 0** | Consistently better than neutral baseline. | 🎯 Excellent — production-ready |

### Interpreting a typical curve

- **Update 1 → 10**: Reward around `-1.34`. Random exploration phase.
- **Update 10 → 20**: Dip to `-1.61`. Agent experiments with tighter packing,
  overloads machines. Classic "exploration dip."
- **Update 20 → 50**: Recovery toward `-1.40`. Agent learns from mistakes.
  Smoothed trend confirms upward trajectory.
- **Update 50+**: Continued improvement. With 200+ updates configured, the
  model has room to climb toward zero and into positive territory.

---

## 11. Hyperparameter Reference

### 11.1 Model Architecture

| Parameter | Value | Effect |
|---|---|---|
| `hidden_dim` | 128 | Width of encoder layers. Larger = more capacity but slower inference |
| `input_dim` | 14 | TASK_FEATURE_DIM + WORKER_FEATURE_DIM |
| Encoder depth | 2 layers | Deeper = more complex patterns but harder to train |
| Activation | ReLU | Standard; simple, fast, gradient-friendly |
| Total parameters | 18,690 | Intentionally tiny for real-time inference |

### 11.2 Offline Training

| Parameter | CLI Flag | Default | Valid Range | Effect |
|---|---|---|---|---|
| Rollout steps | `--rollout-steps` | 1024 | 1 – 65,536 | More data per update; longer CPU phase |
| Minibatch size | `--minibatch-size` | 256 | 1 – rollout_steps | GPU utilization; larger = more stable gradients |
| PPO epochs | `--ppo-epochs` | 6 | 1 – 30 | Gradient steps per rollout; more = better data reuse |
| Total updates | `--updates` | 200 | 1 – 100,000 | Training duration |
| Discount (γ) | `--gamma` | 0.99 | 0.9 – 0.999 | Future reward weighting. Higher = longer horizon |
| GAE lambda (λ) | `--gae-lambda` | 0.95 | 0.0 – 1.0 | Bias-variance trade-off. Higher = lower bias |
| Learning rate | `--learning-rate` | 3e-4 | 1e-5 – 1e-2 | Step size. Too high = instability |
| Clip ratio (ε) | `--clip-ratio` | 0.2 | 0.05 – 0.3 | Policy change limit per update |
| Entropy coeff | `--entropy-coeff` | 0.01 | 0.0 – 0.1 | Exploration bonus. Higher = more exploration |
| Value coeff | `--value-coeff` | 0.5 | 0.1 – 1.0 | Critic loss weight relative to actor |
| Value clip range | `--value-clip-range` | 0.2 | 0.1 – 0.5 | Critic update stability |
| LR annealing | `--lr-anneal` | True | — | Linear decay to 1e-6 over training |
| Grad clip norm | (hardcoded) | 1.0 | — | Maximum gradient norm |

### 11.3 Online Learning

| Parameter | Default | Effect |
|---|---|---|
| `update_batch_size` | 32 | Min buffer size to trigger update |
| `online_gamma` | 0.97 | Shorter horizon for faster adaptation |
| `online_gae_lambda` | 0.92 | More biased GAE for small batches |
| `clip_ratio` | 0.2 | Same as offline |
| `entropy_coeff` | 0.01 | Same as offline |
| `value_coeff` | 0.5 | Same as offline |
| PPO epochs | 4 | Fewer than offline (smaller data) |
| Advantage clamp | [-8, 8] | Prevents outlier-driven gradient explosions |
| Max replay buffer | 4,096 | FIFO eviction beyond this |
| Max pending decisions | 8,192 | FIFO eviction beyond this |
| `deterministic_bias` | 0.25 | Headroom heuristic blending weight |

### 11.4 Environment Parameters

| Parameter | SchedulingEnv (Synthetic) | TraceReplayEnv (Trace) |
|---|---|---|
| Episode length | 96 steps | Full trace (loopable) |
| Load decay | CPU/mem ×0.85, storage ×0.90 per step | e^(-Δt/30) per inter-arrival gap |
| Feasible reward | 1.2 - load_penalty | Multi-component (see §5b) |
| Infeasible reward | -1.4 | -1.8 |
| Reward range | [-1.4, 1.2] | ~[-2.3, 1.65] |

---

## 12. End-to-End Worked Example

Walk through a concrete scheduling decision from raw inputs to weight update.

### 12.1 Setup

```
Task:     req_cpu=2.0, req_memory=4.0, req_storage=1.0, sla_mult=2.0, type=mixed
Worker 0: total_cpu=16, total_mem=48, total_stor=500, used_cpu=8, used_mem=20, used_stor=100
Worker 1: total_cpu=8,  total_mem=32, total_stor=200, used_cpu=2, used_mem=8,  used_stor=50
Worker 2: total_cpu=24, total_mem=96, total_stor=1000,used_cpu=22,used_mem=90, used_stor=900
```

### 12.2 Feature Extraction

**Task features [5]:**
```
[2.0, 4.0, 1.0, 2.0, 1.0]
   │    │    │    │    └── task_type_scalar = 3/3 = 1.0 (mixed)
   │    │    │    └── sla_multiplier
   │    │    └── req_storage
   │    └── req_memory
   └── req_cpu
```

**Worker features [3, 9]:**
```
Worker 0: [0.50, 0.583, 0.80, 16.0, 48.0, 500.0, 0.50, 0.417, 0.20]
Worker 1: [0.75, 0.750, 0.75,  8.0, 32.0, 200.0, 0.25, 0.250, 0.25]
Worker 2: [0.083,0.063, 0.10, 24.0, 96.0,1000.0, 0.917,0.938, 0.90]
```

### 12.3 Action Mask

```
Worker 0: avail_cpu=8 ≥ 2 ✓, avail_mem=28 ≥ 4 ✓, avail_stor=400 ≥ 1 ✓  → True
Worker 1: avail_cpu=6 ≥ 2 ✓, avail_mem=24 ≥ 4 ✓, avail_stor=150 ≥ 1 ✓  → True
Worker 2: avail_cpu=2 ≥ 2 ✓, avail_mem=6  ≥ 4 ✓, avail_stor=100 ≥ 1 ✓  → True

mask = [True, True, True]
```

### 12.4 Pairwise Concatenation

```
pairwise_rows [3, 14]:
  Worker 0: [2.0, 4.0, 1.0, 2.0, 1.0, 0.50, 0.583, 0.80, 16.0, 48.0, 500.0, 0.50, 0.417, 0.20]
  Worker 1: [2.0, 4.0, 1.0, 2.0, 1.0, 0.75, 0.750, 0.75,  8.0, 32.0, 200.0, 0.25, 0.250, 0.25]
  Worker 2: [2.0, 4.0, 1.0, 2.0, 1.0, 0.083,0.063, 0.10, 24.0, 96.0,1000.0, 0.917,0.938, 0.90]
```

### 12.5 Normalization

```
RunningNormalizer.update(pairwise_rows)  → updates streaming mean/M₂
RunningNormalizer.normalize(pairwise_rows) → normalized_rows [3, 14]

Split:
  normalized_task   = normalized_rows[0, :5]     → [5]
  normalized_workers = normalized_rows[:, 5:]    → [3, 9]
```

### 12.6 Forward Pass (Tensor Shapes)

```
task_tensor:   [1, 5]              # unsqueeze batch dim
worker_tensor: [1, 3, 9]           # unsqueeze batch dim
mask_tensor:   [1, 3]              # boolean

# Inside PPOActorCritic.forward():
expanded_task: [1, 3, 5]           # expand task to all workers
pairwise:      [1, 3, 14]          # concat along last dim
hidden:        [1, 3, 128]         # encoder output
logits:        [1, 3]              # policy_head + squeeze
pooled:        [1, 128]            # mean over workers dim
values:        [1]                 # value_head + squeeze

# After masking (all feasible, so no change):
logits:        [1, 3]  e.g. [0.42, 0.88, -0.31]
```

### 12.7 Masked Softmax → Action Probabilities

```
softmax([0.42, 0.88, -0.31]) = [0.295, 0.467, 0.142]   (sums to ~1.0)
                                         ↑
                              Worker 1 has highest probability
```

### 12.8 Action Selection with Headroom Bias (Production)

```
Top-3 policy candidates: [Worker 1, Worker 0, Worker 2]

headroom_scores:
  Worker 0: moderate headroom, moderate load  → score ≈ 0.3
  Worker 1: good headroom, low load           → score ≈ 0.6
  Worker 2: minimal headroom, high load       → score ≈ -0.5

reranked_logits = policy_logits + 0.25 × headroom_scores:
  Worker 0: 0.42 + 0.25×0.3  = 0.495
  Worker 1: 0.88 + 0.25×0.6  = 1.03    ← winner
  Worker 2: -0.31 + 0.25×(-0.5) = -0.435

Final selection: Worker 1
```

### 12.9 DecisionRecord Stored

```python
pending_decisions["task-123"] = DecisionRecord(
    task_features   = normalized_task,       # [5]
    worker_features = normalized_workers,    # [3, 9]
    action_mask     = [True, True, True],    # [3]
    action_index    = 1,                     # Worker 1
    old_log_prob    = log(0.467) ≈ -0.761,
    old_value       = 0.34,                  # Critic's estimate
    worker_id       = "worker-1",
)
```

### 12.10 Task Completes → Reward Derived

```
ReportOutcome: task_id="task-123", status="success", sla_success=True,
               runtime_seconds=45.0

_derive_reward:
    base:      +1.0  (success)
    sla:       +0.5  (sla met)
    duration:  -min(45/600, 0.5) = -0.075

    total reward = 1.0 + 0.5 - 0.075 = 1.425
```

### 12.11 Replay Buffer → PPO Mini-Update

```
replay_buffer grows to 32 entries (update_batch_size)
    ↓
_train_from_replay_locked():
    1. Stack 32 transitions into tensors
    2. Compute GAE (γ=0.97, λ=0.92)
    3. Clamp advantages to [-8, 8], normalize
    4. Run ppo_update (clip=0.2, 4 epochs)
    5. Clear buffer
    6. Persist checkpoint to Mongo
```

---

## 13. Thread Safety & Concurrency

**Source:** `service.py` — `PPOServiceCore`

### 13.1 Locking Strategy

All public methods acquire `self.lock` (a `threading.RLock`) before accessing
any shared state:

```python
def select_worker(self, task, workers, fallback_scheduler):
    with self.lock:
        # ... encode, choose_action, store pending decision
```

This serializes all scheduling decisions, outcome reports, and training updates.
The RLock (reentrant lock) allows nested calls within the same thread without
deadlocking.

### 13.2 Protected Resources

| Resource | Type | Guarded By |
|---|---|---|
| `self.state` (PPOState) | Model + optimizer + normalizer | `self.lock` |
| `self.pending_decisions` | Dict[str, DecisionRecord] | `self.lock` |
| `self.replay_buffer` | List[Dict] | `self.lock` |
| `self.current_fingerprint_hash` | str | `self.lock` |

### 13.3 Training Under Lock

The online PPO update (`_train_from_replay_locked`) executes entirely within
the lock. This means:

1. **Scheduling is briefly blocked** during a training step (~10-50ms for a
   32-entry batch with 4 epochs). This is acceptable because the update is tiny.
2. **No concurrent access** to the model during weight updates — no risk of
   reading stale or partially-updated weights.
3. If latency becomes an issue at scale, the architecture supports extracting
   training to a background thread with double-buffered model swaps.

### 13.4 Shutdown Flush

```python
def close(self):
    with self.lock:
        if self.online_updates_enabled and self.replay_buffer:
            self._train_from_replay_locked()   # flush remaining data
        elif self.current_fingerprint_hash:
            self._persist_current_state_locked("shutdown")  # save state
        self.store.close()
```

On graceful shutdown (SIGINT/SIGTERM), the service:
1. Acquires the lock (blocks new requests)
2. Flushes any remaining replay buffer data through a final PPO update
3. Persists the final checkpoint
4. Closes the MongoDB connection

---

## 14. Frequently Asked Questions

### Q: "What does the data look like?"

**Task data** is a 5-dimensional feature vector: `[req_cpu, req_memory, req_storage, sla_multiplier, task_type_scalar]`. Worker data is 9-dimensional (see §3.2). Together they form a 14-dimensional pairwise input per worker.

**Trace data** comes from three sources (Alibaba CSV, Google JSON, CloudAI MongoDB/files). All are normalized into the `TraceTask` dataclass (§4.1) which captures arrival time, resource requests, runtime, SLA parameters, queue metrics, and outcome data.

### Q: "How do we use the data?"

Raw trace data → `load_trace()` → `TraceCluster` → `TraceReplayEnv` for training. Each trace task becomes a scheduling step where the agent observes the task features and worker states, selects a worker, and receives a reward. The trace defines the workload distribution, inter-arrival times, and reward context (SLA targets, queue waits).

### Q: "What is the model and philosophy behind it?"

An **Actor-Critic PPO** architecture with 18,690 parameters. The Actor outputs a probability distribution over workers (who should run this task?), the Critic estimates the expected future reward (how healthy is the cluster right now?). Both share a common encoder that learns general "cluster understanding."

The philosophy is **sim-to-real transfer**: train offline on historical traces where mistakes are free, then deploy to production with pre-learned expertise, and continue adapting online as real outcomes flow in.

### Q: "Why and how do we use this model?"

**Why:** Rule-based schedulers (round-robin, least-loaded) can't handle multi-objective trade-offs (utilization vs. latency vs. SLA compliance vs. load balance) in non-linear, combinatorial state spaces. The neural network discovers these trade-offs automatically from reward signals.

**How:** The live gRPC service receives `SelectWorkerRequest` messages, runs the forward pass (~0.1ms), applies headroom bias for safety, returns the selected worker, stores a `DecisionRecord`, and later uses the outcome to improve the policy online.

### Q: "How do we use Alibaba data and our own scheduler data?"

**Alibaba data** is used for **offline bootstrap training**. The `load_alibaba_trace()` function reads `batch_task.csv` and `machine_meta.csv`, normalizes CPU (centi-cores → cores) and memory (fraction → GB), maps 12 task types to 4 canonical types, and produces a `TraceCluster` that `TraceReplayEnv` replays.

**Our own CloudAI data** serves dual purpose: (1) offline training via `load_cloudai_trace()` reading from MongoDB `TASKS` + `RESULTS` collections, and (2) online learning where live task outcomes continuously update the model through the replay buffer.

### Q: "How do we perform online and offline training?"

**Offline** (`train_ppo.py`): Load traces → create TraceReplayEnv → run the two-phase loop (rollout + PPO update) for N updates → save checkpoint. This produces a pre-trained model.

**Online** (`service.py`): Deploy checkpoint to gRPC service → make real scheduling decisions → collect outcomes via `ReportOutcome` → accumulate in replay buffer → trigger mini PPO updates (32+ transitions, 4 epochs) → persist updated model to MongoDB. The model adapts continuously to production workload drift.

### Q: "What are traces and replays?"

**Traces** are historical records of cluster workloads — when tasks arrived, what resources they needed, how long they ran, whether they met SLA targets. We support three trace formats: Alibaba cluster-trace-v2018 (8 days, 4000+ machines, CSV), Google ClusterData2019 (JSON), and our own CloudAI history (MongoDB/files).

**Replays** are the process of feeding these traces into `TraceReplayEnv`, which presents them to the RL agent in chronological order as if they were happening in real time. The agent makes scheduling decisions, the environment simulates resource consumption and decay, and the reward signal teaches the agent optimal placement strategies.

### Q: "Why is GPU utilization so low during training?"

Because of the two-phase nature: Phase 1 (rollout) is CPU-bound — the GPU processes batch size 1 while the CPU simulates the environment. Phase 2 (PPO update) is GPU-bound but finishes quickly because the model is tiny (18,690 params). To maximize GPU utilization, increase `--rollout-steps` (more data per update), `--minibatch-size` (larger GPU batches), and `--ppo-epochs` (more passes over the data).

### Q: "How does the model handle variable cluster sizes?"

The pairwise architecture naturally handles any number of workers. Each (task, worker) pair is independently encoded to a hidden representation, the policy head scores each worker individually, and the value head mean-pools all workers. No fixed-size assumption is baked into the architecture — a model trained on 4 workers can inference on 100 workers without modification.

### Q: "What happens if all workers are infeasible?"

The action mask will be all-False. In `choose_action()`, if no worker features exist (`worker_features.size == 0`), it returns `None`. In the gRPC service, this triggers a fallback response (`used_fallback_policy=True`). The master node then falls back to its configured fallback scheduler (e.g., round-robin).

### Q: "How do we prevent catastrophic forgetting during online learning?"

Three mechanisms: (1) PPO clipping limits policy changes to ±20% per update. (2) Value clipping limits Critic changes to ±0.2 per update. (3) Advantage clamping to [-8, 8] in online mode prevents outlier rewards from causing gradient explosions. The small online batch size (32) and fewer epochs (4 vs 6 offline) further limit the magnitude of each update.
