# PPO Performance Optimization (2026-04-11)

For full end-to-end chronology (all code changes, corresponding improvements, and complete result comparison tables), see:
- `docs/PPO_OPTIMIZATION_CONSOLIDATED_REPORT_2026-04-11.md`

## Objective and metrics hierarchy
Primary objective: improve PPO deployment quality against RTS on repeated cluster runs.

Primary ranking hierarchy (from repeated comparison reports):
1. `success_rate` (maximize)
2. `mean_turnaround_seconds` (minimize)
3. `mean_queue_wait_seconds` (minimize)
4. `duration_seconds` (minimize)

Secondary (offline sweep gate on Alibaba trace replay):
- `mean_reward` (maximize)
- `feasible_action_rate` (maximize)
- `evaluated_steps` parity check (all reported runs use `49909` steps)

## Evidence artifacts (exact paths)
- Sweep A: `results/ppo-opt-sweep-a-20260411T161235Z.json`
- Sweep B: `results/ppo-opt-sweep-b-20260411T161342Z.json`
- Repeated cluster compare (pre-opt): `results/testbench/ppo-vs-rts-cluster-repeated-20260411T164201Z.json`
- Repeated cluster compare (optimized): `results/testbench/ppo-vs-rts-cluster-repeated-optimized-20260411T174602Z.json`

## Code changes and why

### 1) Deterministic bias is now configurable in service startup/runtime
- `agentic_scheduler/server.py` exposes `--deterministic-bias` with env default `PPO_DETERMINISTIC_BIAS` (default `0.25`).
- `agentic_scheduler/service.py` stores `self.deterministic_bias` and passes it into `choose_action(..., headroom_bias=self.deterministic_bias)`.

Why: deterministic reranking strength can be calibrated per environment without code edits. The optimized repeated comparison used `ppo_deterministic_bias: "0.20"`.

### 2) Local checkpoint reuse in service fingerprint loading
- `PPOServiceCore` preloads `PPO_MODEL_PATH` at startup when present.
- On first `ensure_fingerprint_loaded(...)`, `_can_reuse_preloaded_local_checkpoint_locked(...)` allows reusing that already-loaded local checkpoint instead of re-reading it from disk.
- If `create_if_missing=true`, the reused local model is still persisted/activated for the fingerprint.
- Default `PPO_MODEL_PATH` is now `latest`, which resolves the newest checkpoint (`.pt`/`.pkl`) from `agentic_scheduler/models/` at startup.

Why: removes redundant local reload on first fingerprint bind and keeps model bootstrap behavior deterministic and lower-overhead.

### 3) Shutdown persistence for deployment continuity
- On graceful service stop, PPO now flushes buffered online replay updates (if present) and persists the active fingerprint model snapshot to MongoDB.

Why: prevents losing late-session adaptation and guarantees the end-of-run state is available for the next startup.

## Sweep outcomes (offline replay)

| Report | Best model | Headroom bias | mean_reward | feasible_action_rate | mean_reward_delta vs max_available | mean_reward_delta vs round_robin |
|---|---|---:|---:|---:|---:|---:|
| `results/ppo-opt-sweep-a-20260411T161235Z.json` | `agentic_scheduler/models/ppo_optA_u140_r2048_e8_mb512_ent0010_seed84.pt` | `0.2` | `1.4130804633183078` | `0.9484060991003627` | `-0.0007310627015653104` | `0.015465093366420213` |
| `results/ppo-opt-sweep-b-20260411T161342Z.json` | `agentic_scheduler/models/ppo_optB_u220_r1024_e8_mb256_ent010_gae093_seed42.pt` | `0.25` | `1.4031290538710595` | `0.9487467190286321` | `-0.010682472148813682` | `0.005513683919171841` |

Selection rationale: Sweep A delivered materially higher replay `mean_reward` than Sweep B, and was used as the deployment model family.

## Cluster benchmark protocol (fairness/repetition/order/workload)

### Pre-opt repeated compare
Source: `results/testbench/ppo-vs-rts-cluster-repeated-20260411T164201Z.json`

- Workload: `testbench/workloads/heterogeneous-smoke.json`
- Schedulers: `rts`, `ppo_sweep_a_best`, `ppo_sweep_b_best`
- Sequence (9 runs):
  - `["rts", "ppo_sweep_a_best", "ppo_sweep_b_best", "rts", "ppo_sweep_a_best", "ppo_sweep_b_best", "rts", "ppo_sweep_a_best", "ppo_sweep_b_best"]`
- Repetitions: `3` runs per scheduler (equal counts)
- Per-run workload size: `10` submitted tasks
- Aggregated totals: `30` submitted / `30` completed for each scheduler

### Optimized repeated compare
Source: `results/testbench/ppo-vs-rts-cluster-repeated-optimized-20260411T174602Z.json`

- Workload: `testbench/workloads/heterogeneous-smoke.json`
- Schedulers: `rts`, `ppo_optimized`
- Sequence (8 runs):
  - `["rts", "ppo_optimized", "ppo_optimized", "rts", "rts", "ppo_optimized", "ppo_optimized", "rts"]`
- Repetitions: `4` runs per scheduler (equal counts)
- Per-run workload size: `10` submitted tasks
- Aggregated totals: `40` submitted / `40` completed for each scheduler
- PPO runtime config recorded in report:
  - `model_path`: `/home/codesmith28/Projects/ACC/BTEP/agentic_scheduler/models/ppo_optA_u140_r2048_e8_mb512_ent0010_seed84.pt`
  - `ppo_deterministic_bias`: `"0.20"`
  - `ppo_online_updates_enabled`: `"false"`

## Before vs after (RTS vs PPO)

Pre-opt PPO column uses `ppo_sweep_b_best` (ranked best PPO in the pre-opt report).

| Metric (mean) | Pre-opt RTS | Pre-opt PPO (`ppo_sweep_b_best`) | Optimized RTS | Optimized PPO (`ppo_optimized`) |
|---|---:|---:|---:|---:|
| success_rate | `1.0` | `1.0` | `1.0` | `1.0` |
| duration_seconds | `8.073` | `9.404333333333334` | `16.57275` | `13.57375` |
| mean_queue_wait_seconds | `4.0` | `4.666666666666667` | `5.25` | `4.125` |
| p95_queue_wait_seconds | `4.0` | `4.666666666666667` | `12.75` | `9.5` |
| mean_turnaround_seconds | `7.366666666666667` | `7.7` | `7.8` | `7.025` |
| assignment_max_share | `1.0` | `1.0` | `0.42500000000000004` | `0.42500000000000004` |

PPO delta vs RTS by phase (from `comparisons_vs_rts`):

| Phase | duration_seconds delta_pct | mean_queue_wait_seconds delta_pct | p95_queue_wait_seconds delta_pct | mean_turnaround_seconds delta_pct |
|---|---:|---:|---:|---:|
| Pre-opt (`ppo_sweep_b_best`) | `16.491184607126634` | `16.666666666666675` | `16.666666666666675` | `4.52488687782805` |
| Optimized (`ppo_optimized`) | `-18.095970795432255` | `-21.428571428571427` | `-25.49019607843137` | `-9.935897435897429` |

## Final recommendation and deployment confidence
- For the tested workload and topology, deployment confidence is **high** for PPO with sweep A model + calibrated deterministic bias (`0.20`) and online updates disabled for stable A/B comparison.
- Evidence basis: equal `success_rate` (`1.0`) with consistent improvements in optimized runs (`duration`, `mean_queue_wait_seconds`, `p95_queue_wait_seconds`, `mean_turnaround_seconds`) vs RTS.
- Recommended rollout: keep RTS fallback enabled while extending repeated comparisons to additional workloads/topologies before broad default promotion.
