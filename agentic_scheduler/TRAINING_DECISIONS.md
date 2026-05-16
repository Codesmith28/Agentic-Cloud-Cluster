# Training Decisions & Rationale

This document records every deliberate change made to the PPO training
pipeline together with the data-driven reasoning behind each decision.

---

## 1. Fix: Alibaba Memory Normalisation (`trace_loader.py`)

**Problem.**  `_normalize_alibaba_memory()` multiplied `plan_mem ≤ 1.0` by 64,
interpreting the value as a fraction of 64 GB.  Machine `mem_size` was kept
as-is (100 — Alibaba's normalised capacity units).  This mixed scales:

| Item | Old value | Machine capacity | Utilisation |
|------|-----------|-----------------|-------------|
| Median task | 0.3 × 64 = **19.2** | 100 | **19.2 %** |
| After 5 tasks | 96.0 | 100 | **96 %** → infeasible |

Workers saturated after ~5 placements, producing −1.8 infeasible penalties for
the remaining 98 % of each rollout.  Average reward across on-disk training runs
was **−1.3 to −1.6** — entirely explained by this bug.

**Fix.**  Remove the ×64 multiplier.  Both `plan_mem` (task) and `mem_size`
(machine) are in Alibaba's native normalised scale where 100 = full machine.

| Item | New value | Machine capacity | Utilisation |
|------|-----------|-----------------|-------------|
| Median task | **0.3** | 100 | **0.3 %** |
| After 100 tasks | 30.0 | 100 | 30 % |

**Verified:** post-fix, 100 % of placements are feasible in test runs and
average reward rises to **+1.64**.

---

## 2. Fix: Machine Deduplication (`trace_loader.py`)

**Problem.**  `machine_meta.csv` contains 17 593 rows but only 4 034 unique
machines — each machine appears at multiple timestamps recording status
changes.  The loader was treating every row as a distinct machine.

**Fix.**  Deduplicate by `machine_id`, keeping the first occurrence.  All
machines in the curated dataset have identical specs (CPU = 96, Mem = 100),
so which timestamp entry is kept does not matter.

---

## 3. Feature: Lifecycle-Based Resource Tracking (`trace_replay_env.py`)

**Problem.**  The old exponential-decay model (`e^(−dt/30)`) was a poor proxy
for task completion:

- **65 % of arrivals are simultaneous** (`dt = 0`) → no decay fires.
- Resources from simultaneous tasks accumulated permanently until the next
  non-zero gap.
- The 30 s time constant was not calibrated to actual task runtimes
  (median 10 s, mean 88 s).

**Fix.**  Replace decay with explicit task lifecycle tracking:

```
On placement → record (worker_idx, req_cpu, req_mem, req_storage, end_time)
On each step → complete_tasks(current_time):
                 release resources for all tasks where end_time ≤ now
```

This correctly models concurrent resource occupation.  Measured concurrency in
the training slice (200 K tasks):

| Metric | Value |
|--------|-------|
| Mean concurrent tasks | 34.5 |
| Median | 28 |
| P95 | 82 |
| Max | 229 |

Edge cases handled:
- On trace loop: `_active_tasks.clear()` + worker `used_*` reset.
- Simultaneous arrivals: `_complete_tasks(t)` runs once per arrival at time `t`;
  tasks placed at time `t` have `end_time = t + runtime > t` so they are not
  prematurely released.

---

## 4. Improvement: Delta-Imbalance Reward Term (`trace_replay_env.py`)

**Problem.**  The original `imbalance_penalty = max(projected_load −
cluster_load, 0)` only penalises placing on the single most-loaded worker.
It gives zero signal when all workers are equally loaded (even if the action
makes them unequal).

**Fix.**  Add a `delta_imbalance` term:

```
loads_before_std = std(worker_loads)  [before placement]
loads_after_std  = std(worker_loads)  [after placement]
delta_imbalance  = loads_after_std − loads_before_std
```

This correctly penalises actions that *worsen* cluster balance (positive delta)
and rewards actions that *improve* it (negative delta).  The existing
`imbalance_penalty` is kept to ensure the overloaded-worker signal remains.

**Updated reward:**

```
reward = 1.4
       + 0.25 × headroom_bonus
       − 0.35 × queue_pressure
       − 0.55 × tail_pressure
       − 0.20 × imbalance_penalty     [original: penalise heaviest worker]
       − 0.40 × delta_imbalance       [new: penalise worsening balance]
       − requeue_penalty
```

---

## 5. Configuration: Workers 64 → 8 (`run_offline_training.sh`)

**Reasoning.**  With 200 K tasks and mean concurrency 34.5:

| Workers | Concurrent tasks / worker | CPU load / worker |
|---------|---------------------------|-------------------|
| 64 | 0.5 | 0.7 % |
| 16 | 2.2 | 2.8 % |
| **8** | **4.3** | **5.7 %** |

64 workers spread load so thin that all workers are nearly identical — the
model receives no meaningful scheduling signal.  8 workers gives ~4 concurrent
tasks per worker on average, with bursts (P95 = 82 / 8 ≈ 10 tasks) creating
genuine capacity pressure.

All machines are identical (CPU = 96, Mem = 100), so reducing from 64 to 8
does not reduce problem diversity — the only axis of differentiation is load
level.

---

## 6. Configuration: Log Frequency 1 → 10 (`run_offline_training.sh`)

**Reasoning.**  `--log-every 1` with 1000 updates produces 1000 log lines and
adds ~2 % overhead from per-step numpy aggregation.  Every-10 still gives
100 data points for monitoring training progress while reducing noise.

---

## 7. Fix: Report Parser (`scripts/generate_report.py`)

**Problem.**  The report generator parsed the old log format
(`records_processed=X/Y (Z%)`), which was replaced with `steps=S epoch=E.EE`
in the progress-logging fix.

**Fix.**  Updated regex to parse both the new format and the legacy format
for backward compatibility with old log files.  Added best/worst reward
summary statistics.

---

## Prior Changes (already committed)

### PPO Epochs 15 → 4

Standard PPO range is 3–10 (original paper uses 3–4 for Atari, 10 for
MuJoCo).  15 was excessive and caused ~3.7× slower training with marginal
return given small network size (229 KB).

### AMP Mixed Precision

CUDA `autocast` + `GradScaler` for the PPO update phase.  ~1.5× speedup on
modern GPUs.  Rollout inference runs on CPU (see below), so AMP is only active
during backpropagation.

### CPU Rollout Inference

The rollout phase calls `choose_action` with batch_size = 1, 16 384 times per
update.  Profiling showed GPU transfer overhead exceeded compute time at this
batch size.  A CPU model copy is maintained and synced after each PPO update;
rollout runs entirely on CPU while the GPU handles minibatch training.

### Vectorised GAE

Replaced Python-loop GAE computation with a single NumPy reverse scan,
eliminating ~16 K Python-level iterations per update.

### Pre-Cached Task Features

`_compute_task_features()` is called once at environment construction and
cached in a numpy array.  `_task_features()` becomes a single array index
instead of a per-step computation.

### Vectorised `RunningNormalizer.update()`

Replaced row-by-row Python loop with batch Welford algorithm operating on
full NumPy arrays.  Numerically verified identical to the sequential version.

### Consistent Feature Normalisation

Both task and worker features are now normalised in `choose_action()` and
stored consistently in rollout transitions regardless of the fallback path.
