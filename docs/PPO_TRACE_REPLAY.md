# PPO Trace Replay Guide

This document describes the trace replay CLI options and PPO training with real cluster traces.

## Overview

The CloudAI system includes a trace replay environment for training PPO scheduling agents on realistic cluster data. The `TraceReplayEnv` replays recorded task arrivals and resource requirements in chronological order, allowing the PPO agent to learn from real workload patterns.

## Trace Replay Environment

### Window Selection (Trace Window Options)

The trace replay environment supports window selection for controlled training on specific time ranges:

```python
from agentic_scheduler.training.trace_replay_env import TraceReplayEnv
from agentic_scheduler.training.trace_loader import load_cluster_trace

# Load trace data
trace = load_cluster_trace(
    csv_path="path/to/trace.csv",
    trace_window="imported",
    trace_window_start="2024-01-01T00:00:00Z",
    trace_window_end="2024-01-01T04:00:00Z"
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

When loading traces for replay, specify the time window:

```python
# Option 1: No window filtering (use entire trace)
trace_window = ""  # or omitted

# Option 2: Specify custom window by ISO 8601 timestamps
trace_window_start = "2024-01-01T00:00:00Z"
trace_window_end = "2024-01-01T06:00:00Z"

# Option 3: Preset window labels
trace_window = "imported"  # Use preset labeled window
trace_window = "live"      # Live cluster data window
trace_window = "none"      # No filtering
```

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

### Checkpointing and Resume

`train_ppo.py` saves checkpoints as `.pkl` payloads.

- Periodic local checkpoints are enabled by default (`--checkpoint-every 10`) and written under `--checkpoint-dir` using `--checkpoint-prefix`.
- Each periodic save also updates `<checkpoint-prefix>_latest.pkl` for quick resume.
- The final checkpoint is always written to `--output` (default: `agentic_scheduler/models/ppo_trained.pkl`).

Resume precedence is local first:
- `--resume-from <path>` resumes from an explicit local checkpoint.
- `--resume-latest` resumes from the newest `.pkl` in `--checkpoint-dir`.
- `--resume-mongo` is only used when local resume is not selected/found.

```bash
python3 -m agentic_scheduler.train_ppo \
  --resume-latest \
  --checkpoint-dir agentic_scheduler/models/checkpoints
```

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

The master node supports three PPO deployment modes controlled via `PPO_DEPLOYMENT_MODE` environment variable:

### 1. Shadow Mode (`shadow`)

PPO runs alongside the active scheduler but does not influence decisions. Used for offline evaluation and convergence tracking.

```bash
export PPO_DEPLOYMENT_MODE=shadow
export SCHED_ALGO=RTS  # Primary scheduler remains active
./runMaster.sh
```

In shadow mode:
- RTS (or other primary scheduler) makes all dispatch decisions
- PPO runs in parallel and reports metrics to `/metrics`
- No latency impact on task assignment

### 2. Active Mode (`active`)

PPO makes scheduling decisions. If PPO becomes unavailable, falls back to RR.

```bash
export PPO_DEPLOYMENT_MODE=active
export SCHED_ALGO=PPO
./runMaster.sh
```

In active mode:
- PPO decides worker assignment for every task
- If PPO service is unavailable, Round-Robin fallback handles dispatch
- Full latency budget applies (see `PPO_REQUEST_TIMEOUT_MS`)

### 3. Fallback Mode (`fallback`)

PPO-assisted mode: primary scheduler makes decisions, PPO validates/ranks candidates (future feature).

```bash
export PPO_DEPLOYMENT_MODE=fallback
export SCHED_ALGO=RTS
./runMaster.sh
```

## Online Update Gate (Adaptive Training)

When `PPO_ONLINE_UPDATES_ENABLED=true`, the PPO service can accept task outcomes and update the model online. This requires the PPO gRPC service to be configured and running.

```bash
export PPO_ONLINE_UPDATES_ENABLED=true
export PPO_GRPC_ADDR=127.0.0.1:50061
export PPO_REQUEST_TIMEOUT_MS=1500
./runMaster.sh
```

Configuration:
- `PPO_ONLINE_UPDATES_ENABLED`: Enable/disable online updates (default: true)
- `PPO_GRPC_ADDR`: PPO service gRPC endpoint
- `PPO_REQUEST_TIMEOUT_MS`: Timeout for PPO decisions (default: 1500ms)
- `PPO_AUTOSTART`: Auto-start PPO service if available (default: true)

With online updates enabled:
- Task completion outcomes are sent to PPO service
- PPO can incrementally refine the model during deployment
- Scheduler performance improves over time in production

## Configuration Reference

### Master Node Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PPO_DEPLOYMENT_MODE` | `active` | Deployment mode: `shadow`, `active`, or `fallback` |
| `PPO_ONLINE_UPDATES_ENABLED` | `true` | Enable online model updates |
| `PPO_PREFER_GPU` | `true` | Prefer CUDA for PPO service; fallback to CPU if unavailable |
| `PPO_GRPC_ADDR` | `127.0.0.1:50061` | PPO gRPC service endpoint |
| `PPO_REQUEST_TIMEOUT_MS` | `1500` | Timeout for PPO decisions (ms) |
| `PPO_AUTOSTART` | `true` | Auto-start PPO service |
| `PPO_MODEL_PATH` | `agentic_scheduler/models/ppo_latest.pt` | Path to PPO model file |
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
| `--learning-rate` | `3e-4` | PPO learning rate |
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
| `--trace-window` | `` | Trace window label or empty for all |
| `--trace-window-start` | `` | Window start (ISO 8601) |
| `--trace-window-end` | `` | Window end (ISO 8601) |

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

The evidence benchmark campaign framework includes PPO variants:

```bash
make campaign  # Tests RR, RTS, PPO-pretrained, PPO-adapted, recovery variants
```

PPO variants in campaign:
- `PPO-pretrained`: Uses pre-trained model (offline training)
- `PPO-adapted`: Adapted on live cluster traces (online learning)
- `PPO+recovery`: PPO with recovery-aware task prioritization

Results are aggregated in `results/campaign/<timestamp>-report.html`.

## Troubleshooting

**PPO service not responding:**

```bash
# Check if PPO gRPC service is running
grpcurl -plaintext localhost:50061 list

# Check master logs for PPO startup/connection issues
docker logs master | grep -i ppo
```

**Model file not found:**

```bash
# Verify model exists and is readable
ls -la agentic_scheduler/models/ppo_latest.pt

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
