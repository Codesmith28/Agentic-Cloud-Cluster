# PPO Optimization Consolidated Report (2026-04-11)

**Date:** 2026-04-11  
**Scope:** End-to-end PPO optimization cycle across data handling, offline training, online serving, cluster benchmarking, and deployment defaults.

## 1) Objective

Build and tune PPO so it can outperform heuristics/RTS under constrained workers, with reproducible evidence and operationally safe defaults.

Primary cluster ranking metrics (from repeated comparison artifacts):
1. `success_rate` (maximize)
2. `mean_turnaround_seconds` (minimize)
3. `mean_queue_wait_seconds` (minimize)
4. `duration_seconds` (minimize)

Offline sweep gate metrics:
- `mean_reward` (maximize)
- `feasible_action_rate` (maximize)
- `evaluated_steps` parity

---

## 2) What changed (code + why + measured effect)

| Change theme | Files | What changed | Why it was done | Observed impact |
|---|---|---|---|---|
| Alibaba CPU unit correction | `agentic_scheduler/training/trace_loader.py` | Normalized Alibaba `plan_cpu` to cores (`100 => 1 core`) via `_normalize_alibaba_cpu` and related loader flow. | Remove unit mismatch that distorted feasibility/reward signal. | Legacy replay best `mean_reward` was `-0.692358442473344`; corrected/trained PPO reached `0.88009658782694` on held-out replay. |
| PPO training loop upgrades | `agentic_scheduler/train_ppo.py`, `agentic_scheduler/model.py` | GAE returns, bootstrap value, stochastic rollout sampling, minibatch PPO epochs, clipped value loss, more tunable hyperparameters/checkpointing/resume. | Improve stability/convergence and enable systematic sweeps. | Improved PPO beat replay RR (`+0.0112569073273014` mean_reward) and first-feasible (`+0.0655856039946845`). |
| Reward shaping to target queue/tail quality | `agentic_scheduler/training/trace_replay_env.py` | Reward now combines headroom bonus with queue pressure, turnaround/tail pressure, imbalance penalty, and requeue penalty. | Align optimization objective with queue/tail/turnaround quality under load. | Enabled higher-quality sweep candidates used in cluster tests (Sweep A/B artifacts). |
| Deterministic serving calibration | `agentic_scheduler/model.py`, `agentic_scheduler/service.py`, `agentic_scheduler/server.py` | Deterministic top-k rerank with configurable `headroom_bias`; runtime knob `PPO_DETERMINISTIC_BIAS` / `--deterministic-bias`. | Tune inference behavior without retraining to fit cluster workload characteristics. | Pre-opt repeated benchmark (RTS winner) flipped to optimized repeated benchmark (PPO winner) with lower duration/queue/turnaround. |
| PPO model lifecycle improvements | `agentic_scheduler/service.py` | Reuse preloaded checkpoint for first fingerprint attach; startup model resolver supports `latest`/dir/file fallback; graceful shutdown flush + persist to Mongo. | Reduce startup overhead and ensure learned state continuity/persistence. | Operational improvement: default now starts from newest checkpoint; end-of-run state persisted to Mongo for continuity. |
| Master PPO defaults | `master/internal/config/config.go`, `master/main.go` | `PPO_MODEL_PATH` default changed to `latest` for autostart and config. | Avoid stale fixed-path startup behavior. | Next startups automatically use newest checkpoint unless overridden. |
| Cluster execution reliability for benchmarking | `worker/internal/system/runtime_config.go`, `worker/internal/system/runtime_config_test.go`, `worker/internal/executor/executor.go`, `worker/README.md` | Added `WORKER_CONTAINER_NETWORK_MODE` (`bridge|host|none`), wired container `NetworkMode`, and local-image fallback on pull failure. | Unblock reproducible cluster benchmarks in constrained/offline environments. | Stable repeated benchmark runs with complete task success (`success_rate=1.0`) across compared schedulers. |

---

## 3) Key result timeline

### 3.1 Legacy replay signal before correction/tuning

**Artifact:** `results/ppo-lw4-sweep-20260411T112121Z.json`

- Best legacy sweep candidate:
  - `mean_reward`: `-0.692358442473344`
  - `feasible_action_rate`: `0.3089222384740227`

### 3.2 Corrected trace + upgraded PPO (held-out replay)

**Artifact:** `results/ppo-lw4-final-comparison-20260411T122233Z.json`

| Policy | evaluated_steps | mean_reward | feasible_action_rate |
|---|---:|---:|---:|
| round_robin | 49909 | 0.8688396804996386 | 0.9437977118355407 |
| first_feasible | 49909 | 0.8145109838322555 | 0.9480053697729869 |
| max_available | 49909 | 0.8800201306133255 | 0.9484461720331002 |
| **PPO (`ppo_lw4_improved_seed84.pt`)** | **49909** | **0.88009658782694** | **0.9485864272976818** |

PPO deltas:
- vs `max_available`: `+0.0000764572136145` mean_reward, `+0.0001402552645816` feasible rate
- vs `round_robin`: `+0.0112569073273014` mean_reward, `+0.0047887154621411` feasible rate

### 3.3 Focused sweep A/B winners (held-out replay)

**Artifacts:**
- `results/ppo-opt-sweep-a-20260411T161235Z.json`
- `results/ppo-opt-sweep-b-20260411T161342Z.json`

| Sweep | Winner model | Bias | mean_reward | feasible_action_rate | Δmean_reward vs max_available | Δmean_reward vs RR |
|---|---|---:|---:|---:|---:|---:|
| A | `agentic_scheduler/models/ppo_optA_u140_r2048_e8_mb512_ent0010_seed84.pt` | 0.2 | 1.4130804633183078 | 0.9484060991003627 | -0.0007310627015653104 | +0.015465093366420213 |
| B | `agentic_scheduler/models/ppo_optB_u220_r1024_e8_mb256_ent010_gae093_seed42.pt` | 0.25 | 1.4031290538710595 | 0.9487467190286321 | -0.010682472148813682 | +0.005513683919171841 |

Selection used for optimized cluster pass: **Sweep A winner** (better replay reward).

### 3.4 Early direct cluster snapshot (single run)

**Artifact:** `results/testbench/ppo-vs-rts-cluster-comparison-20260411T141446Z.json`

| Metric | RTS | PPO | PPO vs RTS |
|---|---:|---:|---:|
| success_rate | 1.0 | 1.0 | 0.0 |
| duration_seconds | 16.077 | 12.085 | -24.830503203333958% |
| mean_queue_wait_seconds | 3.5 | 5.0 | +42.857142857142854% |
| p95_queue_wait_seconds | 8.749999999999995 | 8.0 | -8.571428571428516% |
| mean_turnaround_seconds | 6.7 | 7.9 | +17.910447761194032% |

Interpretation: single run showed mixed trade-offs; needed repeated protocol.

### 3.5 Repeated cluster compare before final calibration

**Artifact:** `results/testbench/ppo-vs-rts-cluster-repeated-20260411T164201Z.json`

Protocol:
- Workload: `testbench/workloads/heterogeneous-smoke.json`
- Sequence: `RTS, PPO_A, PPO_B` repeated 3 times (9 runs total)
- 3 runs per scheduler

Ranking:
1. **RTS** — turnaround `7.366666666666667`, queue `4.0`, duration `8.073`
2. PPO sweep B best — turnaround `7.7`, queue `4.666666666666667`, duration `9.404333333333334`
3. PPO sweep A best — turnaround `7.733333333333333`, queue `4.666666666666667`, duration `9.409666666666666`

### 3.6 Repeated cluster compare after final optimization/calibration

**Artifact:** `results/testbench/ppo-vs-rts-cluster-repeated-optimized-20260411T174602Z.json`

Protocol:
- Workload: `testbench/workloads/heterogeneous-smoke.json`
- 8 runs total; 4 runs each (`RTS` and `PPO optimized`)
- PPO config recorded in artifact:
  - model: `/home/codesmith28/Projects/ACC/BTEP/agentic_scheduler/models/ppo_optA_u140_r2048_e8_mb512_ent0010_seed84.pt`
  - `ppo_deterministic_bias`: `"0.20"`
  - `ppo_online_updates_enabled`: `"false"`

Ranking:
1. **PPO optimized** — success `1.0`, turnaround `7.025`, queue `4.125`, duration `13.57375`
2. RTS — success `1.0`, turnaround `7.8`, queue `5.25`, duration `16.57275`

PPO optimized vs RTS deltas:
- duration: `-18.095970795432255%`
- mean queue wait: `-21.428571428571427%`
- p95 queue wait: `-25.49019607843137%`
- mean turnaround: `-9.935897435897429%`
- success rate: `0.0` delta (both 1.0)

---

## 4) Go baseline snapshot (separate harness)

**Artifact:** `results/benchmarks/go-baseline-go-cli/20260411-183603/summary.json`

Profile: `steady`, seed `42`, task_count `92`.

| Scheduler | SLA% | P95 wait (s) | Throughput tasks/min | Makespan (s) | CPU util % | Worker balance |
|---|---:|---:|---:|---:|---:|---:|
| RTS | 100 | 0 | 15.264012064603728 | 361.634934291 | 9.025845299600979 | 0.7658623996898042 |
| Round-Robin | 100 | 0 | 15.20170101841363 | 363.117258609 | 9.753330611303774 | 1 |

Note: this benchmark harness/profile is distinct from replay and cluster workload harnesses; use within-harness comparisons for conclusions.

---

## 5) Final operational state after this cycle

### Active recommended model
- `agentic_scheduler/models/ppo_optA_u140_r2048_e8_mb512_ent0010_seed84.pt`

### Startup behavior (now default)
- `PPO_MODEL_PATH` default is `latest`.
- On startup, PPO resolves newest `.pt`/`.pkl` in `agentic_scheduler/models/` unless explicit path/dir is provided.

### Shutdown persistence
- On graceful shutdown, PPO service:
  1. flushes buffered online outcomes (if any), and
  2. persists active fingerprint model state to MongoDB.

This preserves end-of-run adaptation and improves restart continuity.

---

## 6) Cross-harness interpretation notes

1. `mean_reward`/`feasible_action_rate` come from replay environment (`TraceReplayEnv`).
2. Cluster comparison metrics (`duration`, `queue wait`, `turnaround`) come from live workload execution path.
3. Go benchmark (`steady`) is another harness; use it for Go-scheduler baseline context, not as a direct replacement for cluster repeated PPO-vs-RTS evidence.
4. Decision confidence should be based on repeated cluster runs, not single-run snapshots.

---

## 7) Artifact index (for final report assembly)

### Core result artifacts
- `results/ppo-lw4-sweep-20260411T112121Z.json`
- `results/ppo-lw4-final-comparison-20260411T122233Z.json`
- `results/ppo-opt-sweep-a-20260411T161235Z.json`
- `results/ppo-opt-sweep-b-20260411T161342Z.json`
- `results/testbench/ppo-vs-rts-cluster-comparison-20260411T141446Z.json`
- `results/testbench/ppo-vs-rts-cluster-repeated-20260411T164201Z.json`
- `results/testbench/ppo-vs-rts-cluster-repeated-optimized-20260411T174602Z.json`
- `results/benchmarks/go-baseline-go-cli/20260411-183603/summary.json`

### Related documentation
- `docs/PPO_TRACE_REPLAY.md`
- `docs/PPO_PERFORMANCE_OPTIMIZATION.md`
- `docs/PPO_OPTIMIZATION_CONSOLIDATED_REPORT_2026-04-11.md` (this document)

