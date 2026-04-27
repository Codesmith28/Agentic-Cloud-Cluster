# PPO Trace Replay Guide

This document describes the trace replay CLI options and PPO training with real cluster traces.

## Overview

The CloudAI system includes a trace replay environment for training PPO scheduling agents on realistic cluster data. The `TraceReplayEnv` replays recorded task arrivals and resource requirements in chronological order, allowing the PPO agent to learn from real workload patterns.

## Trace Replay Environment

### Window Selection (Trace Window Options)

The trace replay environment supports window selection for controlled training on specific time ranges:

```python
from agentic_scheduler.training.trace_replay_env import TraceReplayEnv
from agentic_scheduler.training.trace_loader import load_trace

# Load trace data
trace = load_trace(
    trace_path="path/to/cloudai/replay",
    source="cloudai",
    max_tasks=5000,
    trace_window="imported",
    trace_window_start="2024-01-01T00:00:00Z",
    trace_window_end="2024-01-01T04:00:00Z",
)

# Create replay environment
env = TraceReplayEnv(
    trace=trace,
    num_workers=4,
    loop=True
)

# Train PPO agent
obs, info = env.reset()
for step in range(10000):
    action = ppo_agent.predict(obs)
    obs, reward, terminated, truncated, info = env.step(action)
    if terminated or truncated:
        obs, info = env.reset()
```

### Trace Window Parameters

When loading traces for replay:

```python
# No window filtering (default)
trace_window = ""  # or omitted

# Filter by time bounds (CloudAI trace source)
# Accepted formats: Unix epoch or ISO-8601
trace_window_start = "2024-01-01T00:00:00Z"
trace_window_end = "2024-01-01T06:00:00Z"

# Filter by trace label (CloudAI trace source)
# Exact match against record.trace_window when labels are present
trace_window = "imported"
```

Behavior details from `trace_loader.py`:

- `--trace-window`, `--trace-window-start`, and `--trace-window-end` are applied for `--trace-source cloudai`.
- `--trace-window` is an exact label match when CloudAI records carry a `trace_window` field.
- `--trace-window-start` / `--trace-window-end` filter by record arrival/submission timestamps.
- For `alibaba` and `google` sources, these window flags are currently ignored.

### Training Script Usage

```bash
python3 -m agentic_scheduler.train_ppo \
  --trace-source cloudai \
  --mongo-uri "mongodb://localhost:27017" \
  --mongo-db cluster_db \
  --num-workers 4 \
  --updates 100 \
  --rollout-steps 1024 \
  --trace-window "imported" \
  --trace-window-start "2024-01-01T00:00:00Z" \
  --trace-window-end "2024-01-01T06:00:00Z" \
  --checkpoint-dir agentic_scheduler/models/checkpoints \
  --checkpoint-every 10 \
  --output agentic_scheduler/models/ppo_trained.pkl
```

### Bootstrap Alibaba train/test (explicit)

#### Dataset download source URLs used

- `http://aliopentrace.oss-cn-beijing.aliyuncs.com/v2018Traces/machine_meta.tar.gz`
- `http://aliopentrace.oss-cn-beijing.aliyuncs.com/v2018Traces/batch_task.tar.gz`
- Upstream fetch script reference: `https://github.com/alibaba/clusterdata/blob/master/cluster-trace-v2018/fetchData.sh`

#### Required local directories and canonical filenames

- Bootstrap source directory (canonical raw CSV names):
  - `agentic_scheduler/data/alibaba_v2018/bootstrap/machine_meta.csv`
  - `agentic_scheduler/data/alibaba_v2018/bootstrap/batch_task.csv`
- Explicit split directories used for training/evaluation:
  - `agentic_scheduler/data/alibaba_v2018/train/machine_meta.csv`
  - `agentic_scheduler/data/alibaba_v2018/train/batch_task.csv`
  - `agentic_scheduler/data/alibaba_v2018/test/machine_meta.csv`
  - `agentic_scheduler/data/alibaba_v2018/test/batch_task.csv`

> CPU normalization note: Alibaba `plan_cpu` is normalized to cores (`100 => 1 core`) by the trace loader.

#### Deterministic train/test slicing (200k train + 50k test rows)

```bash
mkdir -p agentic_scheduler/data/alibaba_v2018/train agentic_scheduler/data/alibaba_v2018/test

MACHINE_HEADER='machine_id,time_stamp,failure_domain_1,failure_domain_2,cpu_num,mem_size,status'
TASK_HEADER='task_name,instance_num,job_name,task_type,status,start_time,end_time,plan_cpu,plan_mem'

{ printf '%s\n' "$MACHINE_HEADER"; cat agentic_scheduler/data/alibaba_v2018/bootstrap/machine_meta.csv; } \
  > agentic_scheduler/data/alibaba_v2018/train/machine_meta.csv
cp agentic_scheduler/data/alibaba_v2018/train/machine_meta.csv \
  agentic_scheduler/data/alibaba_v2018/test/machine_meta.csv

{ printf '%s\n' "$TASK_HEADER"; sed -n '1,200000p' agentic_scheduler/data/alibaba_v2018/bootstrap/batch_task.csv; } \
  > agentic_scheduler/data/alibaba_v2018/train/batch_task.csv
{ printf '%s\n' "$TASK_HEADER"; sed -n '200001,250000p' agentic_scheduler/data/alibaba_v2018/bootstrap/batch_task.csv; } \
  > agentic_scheduler/data/alibaba_v2018/test/batch_task.csv
```

#### Training command (current `train_ppo.py` flags)

```bash
python3 -m agentic_scheduler.train_ppo \
  --trace-source alibaba \
  --trace-path agentic_scheduler/data/alibaba_v2018/train \
  --max-trace-tasks 50000 \
  --num-workers 4 \
  --updates 120 \
  --rollout-steps 1024 \
  --gae-lambda 0.95 \
  --ppo-epochs 10 \
  --minibatch-size 256 \
  --entropy-coeff 0.02 \
  --lr-anneal \
  --seed 84 \
  --output agentic_scheduler/models/ppo_lw4_improved_seed84.pt
```

#### Evaluation command (explicit replay with `load_trace` + `TraceReplayEnv`)

```bash
python3 - <<'PY'
import json
from datetime import datetime, timezone
from pathlib import Path

import numpy as np
import torch

from agentic_scheduler.model import PPOState, choose_action
from agentic_scheduler.training.trace_loader import load_trace
from agentic_scheduler.training.trace_replay_env import TraceReplayEnv

model_path = Path("agentic_scheduler/models/ppo_lw4_improved_seed84.pt")
trace = load_trace(
    trace_path="agentic_scheduler/data/alibaba_v2018/test",
    source="alibaba",
    max_tasks=50000,
)
env = TraceReplayEnv(trace, num_workers=4, loop=False)

device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
state = PPOState.from_checkpoint_bytes(
    model_path.read_bytes(),
    learning_rate=3e-4,
    device=device,
)

obs, _ = env.reset(seed=42)
target_steps = 50000
rewards = []
feasible = 0

for _ in range(target_steps):
    action_mask = obs["action_mask"].astype(bool)
    if action_mask.any():
        action_info = choose_action(
            state,
            task_features=obs["task"],
            worker_features=obs["workers"],
            action_mask=action_mask,
            device=device,
            deterministic=True,
            headroom_bias=0.15,
        )
        if action_info is None:
            action = int(np.where(action_mask)[0][0])
        else:
            action = int(action_info["action_index"])
    else:
        action = 0

    obs, reward, terminated, truncated, info = env.step(action)
    rewards.append(float(reward))
    feasible += int(bool(info.get("feasible", False)))
    if terminated or truncated:
        break

report = {
    "timestamp_utc": datetime.now(timezone.utc).isoformat(),
    "model_path": str(model_path),
    "trace_source": "alibaba",
    "trace_path": "agentic_scheduler/data/alibaba_v2018/test",
    "target_steps": target_steps,
    "evaluated_steps": len(rewards),
    "mean_reward": float(np.mean(rewards)) if rewards else 0.0,
    "feasible_action_rate": (feasible / len(rewards)) if rewards else 0.0,
}

report_path = Path("results") / f"ppo-bootstrap-eval-{datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')}.json"
report_path.write_text(json.dumps(report, indent=2), encoding="utf-8")
print(report_path)
PY
```

#### Output artifacts

- Model checkpoint: `agentic_scheduler/models/ppo_lw4_improved_seed84.pt`
- Comparison report: `results/ppo-lw4-final-comparison-20260411T122233Z.json`
- Single-model eval report pattern: `results/ppo-bootstrap-eval-<timestamp>.json`

### Checkpointing and Resume

`train_ppo.py` writes PyTorch checkpoint payload bytes. Periodic checkpoints are `.pkl`; final output can use `.pkl` or `.pt` (extension is path naming only).

- Periodic local checkpoints are enabled by default (`--checkpoint-every 10`) and written under `--checkpoint-dir` using `--checkpoint-prefix`.
- Periodic filename pattern: `<checkpoint-prefix>_u<update>_s<training_steps>.pkl`.
- Each periodic save also updates `<checkpoint-prefix>_latest.pkl` for quick resume.
- The final checkpoint is always written to `--output` (default: `agentic_scheduler/models/ppo_trained.pkl`).

Resume precedence is local first:
- `--resume-from <path>` resumes from an explicit local checkpoint.
- If `--resume-from` does not exist and `--resume-latest` is set, training falls back to latest local `.pkl`.
- `--resume-latest` scans only `.pkl` files in `--checkpoint-dir`.
- `--resume-mongo` is only used when local resume is not selected/found.

```bash
python3 -m agentic_scheduler.train_ppo \
  --resume-latest \
  --checkpoint-dir agentic_scheduler/models/checkpoints
```

### Reproducibility knobs for optimization evidence (2026-04-11)

For the repeated optimized cluster comparison (`results/testbench/ppo-vs-rts-cluster-repeated-optimized-20260411T174602Z.json`), the recorded PPO runtime settings were:

- `PPO_MODEL_PATH=/home/codesmith28/Projects/ACC/BTEP/agentic_scheduler/models/ppo_optA_u140_r2048_e8_mb512_ent0010_seed84.pt`
- `PPO_DETERMINISTIC_BIAS=0.20`
- `PPO_ONLINE_UPDATES_ENABLED=false`

Service behavior note for reproducibility:
- when `PPO_MODEL_PATH` exists, the service preloads that local checkpoint at startup and reuses the preloaded state for the first fingerprint load (instead of re-reading the same file), then optionally persists it for that fingerprint.
- when `PPO_MODEL_PATH=latest` or `PPO_MODEL_PATH=auto`, the service resolves the newest checkpoint (`.pt`/`.pkl`) under `agentic_scheduler/models/` at startup.
- when `PPO_MODEL_PATH` points to a missing file, the service falls back to the newest checkpoint in the file's parent directory (then `agentic_scheduler/models/`).
- on graceful shutdown, the service flushes buffered online-update samples; if a fingerprint is active, it persists the current snapshot for that fingerprint.

Related selection artifacts:
- `results/ppo-opt-sweep-a-20260411T161235Z.json`
- `results/ppo-opt-sweep-b-20260411T161342Z.json`
- `results/testbench/ppo-vs-rts-cluster-repeated-20260411T164201Z.json`

### Optional Mongo Checkpoint Persistence/Resume

Mongo checkpointing is optional and requires `--mongo-uri` plus `--fingerprint-hash`.

- `--mongo-checkpoint-every N`: persist active checkpoint to Mongo every N updates.
- `--mongo-save-final` (default) / `--no-mongo-save-final`: control final Mongo persistence.
- `--resume-mongo`: resume from the active Mongo checkpoint for the fingerprint (when local resume is not used).
- `--fingerprint-payload` is optional metadata stored with the checkpoint.

```bash
python3 -m agentic_scheduler.train_ppo \
  --trace-source cloudai \
  --mongo-uri "mongodb://localhost:27017" \
  --mongo-db cluster_db \
  --fingerprint-hash "cluster-fp-123" \
  --mongo-checkpoint-every 5 \
  --resume-mongo
```

### GPU Preference Behavior

- Offline training prefers CUDA automatically (`torch.cuda.is_available()`), otherwise CPU.
- Service runtime defaults to GPU preference and falls back to CPU if CUDA is unavailable.
- Configure service GPU preference with:
  - `--prefer-gpu true|false`
  - `PPO_PREFER_GPU=true|false`

## PPO Deployment Modes

`PPO_DEPLOYMENT_MODE` applies when `SCHED_ALGO=PPO`.

### 1. Shadow Mode (`shadow`)

PPO is queried, but the fallback scheduler's decision is used for dispatch.

```bash
export PPO_DEPLOYMENT_MODE=shadow
export SCHED_ALGO=PPO
./runMaster.sh
```

In shadow mode:
- fallback scheduler (RTS, then RR fallback inside RTS) still decides assignments
- PPO runs in parallel and logs divergence when PPO and fallback disagree

### 2. Active Mode (`active`)

PPO makes scheduling decisions. RPC/validation failures fall back to RTS (which itself falls back to RR).

```bash
export PPO_DEPLOYMENT_MODE=active
export SCHED_ALGO=PPO
./runMaster.sh
```

In active mode:
- PPO decides worker assignment for every task
- If PPO service is unavailable or returns invalid output, fallback scheduler handles dispatch
- Full latency budget applies (see `PPO_REQUEST_TIMEOUT_MS`)

### 3. Fallback Mode (`fallback`)

PPO gRPC is bypassed; fallback scheduler is always used.

```bash
export PPO_DEPLOYMENT_MODE=fallback
export SCHED_ALGO=PPO
./runMaster.sh
```

In fallback mode:
- PPO autostart is skipped
- no PPO RPC calls are required
- dispatch is handled by fallback scheduler (RTS, then RR fallback inside RTS)

## Online Update Gate (Adaptive Training)

When `PPO_ONLINE_UPDATES_ENABLED=true`, PPO can accept task outcomes and run online updates in **active mode**. This requires the PPO gRPC service to be configured and reachable.

```bash
export PPO_ONLINE_UPDATES_ENABLED=true
export PPO_GRPC_ADDR=127.0.0.1:50050
export PPO_REQUEST_TIMEOUT_MS=1500
./runMaster.sh
```

Configuration:
- `PPO_ONLINE_UPDATES_ENABLED`: Enable/disable online updates (default: true; active mode only)
- `PPO_DETERMINISTIC_BIAS`: Headroom reranking bias for deterministic PPO inference (default: 0.25)
- `PPO_GRPC_ADDR`: PPO service gRPC endpoint
- `PPO_REQUEST_TIMEOUT_MS`: Timeout for PPO decisions (default: 1500ms)
- `PPO_AUTOSTART`: Auto-start PPO service if available (default: true)
- `PPO_UPDATE_BATCH_SIZE`: PPO service replay batch size for online updates (default: 32)

With online updates enabled:
- Task completion outcomes are sent to PPO service
- outcomes are buffered and PPO updates run when replay buffer size reaches `PPO_UPDATE_BATCH_SIZE`
- service shutdown flushes buffered outcomes before close

## Configuration Reference

### Master Node Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PPO_DEPLOYMENT_MODE` | `active` | PPO mode (`shadow`, `active`, `fallback`) when `SCHED_ALGO=PPO` |
| `PPO_ONLINE_UPDATES_ENABLED` | `true` | Enable online model updates in active mode |
| `PPO_DETERMINISTIC_BIAS` | `0.25` | Deterministic inference reranking strength (higher favors capacity headroom prior) |
| `PPO_PREFER_GPU` | `true` | Prefer CUDA for PPO service; fallback to CPU if unavailable |
| `PPO_GRPC_ADDR` | `127.0.0.1:50050` | PPO gRPC service endpoint |
| `PPO_REQUEST_TIMEOUT_MS` | `1500` | Timeout for PPO decisions (ms) |
| `PPO_AUTOSTART` | `true` | Auto-start PPO service |
| `PPO_MODEL_PATH` | `latest` | Startup model selector (`latest`/`auto`, checkpoint directory, or file path; missing path falls back to latest checkpoint) |
| `SCHED_ALGO` | `RTS` | Scheduler algorithm: `RR`, `RTS`, or `PPO` |

### Training Script CLI Options

```bash
python3 -m agentic_scheduler.train_ppo --help
```

| Option | Default | Description |
|--------|---------|-------------|
| `--num-workers` | `4` | Number of workers in the training environment |
| `--episode-length` | `96` | Episode length for synthetic (non-trace) training |
| `--rollout-steps` | `1024` | Steps collected per PPO update |
| `--updates` | `200` | Number of PPO updates |
| `--gamma` | `0.99` | Discount factor |
| `--gae-lambda` | `0.95` | GAE lambda for bias/variance trade-off in advantages |
| `--learning-rate` | `3e-4` | PPO learning rate |
| `--clip-ratio` | `0.2` | PPO policy clip ratio |
| `--entropy-coeff` | `0.01` | Entropy regularization coefficient |
| `--value-coeff` | `0.5` | Value loss coefficient |
| `--value-clip-range` | `0.2` | Value function clipping range |
| `--ppo-epochs` | `6` | PPO optimization epochs per rollout batch |
| `--minibatch-size` | `256` | PPO minibatch size (`<=0` uses full batch) |
| `--lr-anneal` / `--no-lr-anneal` | `true` | Enable/disable linear LR annealing across updates |
| `--seed` | `42` | Random seed for NumPy/PyTorch/environment |
| `--output` | `agentic_scheduler/models/ppo_trained.pkl` | Final output checkpoint path |
| `--checkpoint-dir` | `agentic_scheduler/models/checkpoints` | Local periodic checkpoint directory |
| `--checkpoint-prefix` | `ppo_offline` | Prefix for periodic checkpoint filenames |
| `--checkpoint-every` | `10` | Save local checkpoint every N updates (`<=0` disables) |
| `--resume-from` | `""` | Resume from explicit local checkpoint path |
| `--resume-latest` | `false` | Resume from latest `.pkl` in `--checkpoint-dir` |
| `--log-every` | `10` | Log training progress every N updates |
| `--mongo-uri` | `""` | MongoDB URI for trace loading/checkpoint persistence |
| `--mongo-db` | `cluster_db` | MongoDB database name |
| `--fingerprint-hash` | `""` | Cluster fingerprint hash for Mongo checkpoint namespace |
| `--fingerprint-payload` | `""` | Optional fingerprint payload metadata |
| `--mongo-checkpoint-every` | `0` | Persist active Mongo checkpoint every N updates (`<=0` disables) |
| `--mongo-save-final` / `--no-mongo-save-final` | `true` | Enable/disable final Mongo checkpoint persistence |
| `--resume-mongo` | `false` | Resume from active Mongo checkpoint if no local resume is used |
| `--trace-source` | `""` | Trace source: `alibaba`, `google`, `cloudai`, or empty for synthetic |
| `--trace-path` | `""` | Path to trace data (optional for `cloudai` when `--mongo-uri` is set) |
| `--max-trace-tasks` | `5000` | Maximum tasks loaded from trace |
| `--trace-window` | `` | CloudAI trace-window label filter (exact match when labels exist) |
| `--trace-window-start` | `` | CloudAI window start (Unix epoch or ISO 8601) |
| `--trace-window-end` | `` | CloudAI window end (Unix epoch or ISO 8601) |

## Metrics and Monitoring

PPO performance is tracked via Prometheus metrics exported at `/metrics`:

```bash
curl http://localhost:8080/metrics | grep -i ppo
```

Key metrics:
- `scheduler_decisions_total{algorithm="ppo"}`: Total PPO decisions made
- `scheduler_latency_seconds{algorithm="ppo"}`: Decision latency (shadow/active modes)
- `ppo_online_updates_total`: Number of online updates processed
- `ppo_model_version`: Current model version in use

## Integration with Campaign Framework

The evidence benchmark campaign framework compares all schedulers:

```bash
make campaign  # Tests RR, RTS, PPO across multiple scenarios
```

Schedulers in campaign:
- `RR`: Round-Robin (baseline)
- `RTS`: Risk-aware Task Scheduling with GA-tuned parameters
- `PPO`: PPO agent trained offline on Alibaba traces, with online learning enabled

Results are aggregated in `results/campaign/<timestamp>-report.html`.

## Troubleshooting

**PPO service not responding:**

```bash
# Check if PPO gRPC service is running
grpcurl -plaintext localhost:50050 list

# Check master logs for PPO startup/connection issues
docker logs master | grep -i ppo
```

**Model file not found:**

```bash
# Inspect newest available checkpoints selected by PPO_MODEL_PATH=latest
ls -lt agentic_scheduler/models/*.{pt,pkl} 2>/dev/null | head

# Use PPO_MODEL_PATH to specify custom path
export PPO_MODEL_PATH=/custom/path/to/model.pt
```

**Online updates not working:**

```bash
# Ensure online updates are enabled
export PPO_ONLINE_UPDATES_ENABLED=true

# Check master logs for update errors
docker logs master | grep -i "online\|update"
```

## References

- **Trace Loader**: `agentic_scheduler/training/trace_loader.py` - Load cluster traces with window selection
- **Trace Replay Environment**: `agentic_scheduler/training/trace_replay_env.py` - RL environment for PPO training
- **PPO Training**: `agentic_scheduler/train_ppo.py` - Main training script
- **Campaign Framework**: `testbench/scripts/run_campaign.py` - Evidence benchmark orchestration
