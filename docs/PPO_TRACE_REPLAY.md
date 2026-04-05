# PPO Trace Replay and Deployment Modes

This document covers the branch-5 PPO workflow:

- offline pretraining on public traces
- offline domain adaptation on CloudAI history
- live deployment modes for PPO inference
- model lineage metadata stored with PPO checkpoints

The live PPO gRPC service remains the production inference path. Public traces are an additional offline training and evaluation source, not a replacement for real-world adaptation.

## Supported Replay Sources

### CloudAI history

CloudAI history replay reads directly from MongoDB collections already used by the system:

- `WORKER_REGISTRY`
- `TASKS`
- `RESULTS`

The loader normalizes these into replayable tasks with:

- arrival time
- requested CPU, memory, storage
- task type
- runtime
- queue wait
- SLA outcome when available
- recovery count and last failure reason when present

### Public traces

The PPO training entrypoint also supports:

- Alibaba Cluster Trace Program `cluster-trace-v2018`
- Google `ClusterData2019`

Recommended usage:

1. Pretrain on Alibaba traces.
2. Evaluate on Google holdout.
3. Adapt on CloudAI history.
4. Roll out in `shadow` mode before switching to `active`.

## Training Commands

### Pretrain on Alibaba

```bash
python3 -m agentic_scheduler.train_ppo \
  --trace-source alibaba \
  --trace-path /path/to/cluster-trace-v2018 \
  --num-workers 4 \
  --max-trace-tasks 5000 \
  --updates 200 \
  --output agentic_scheduler/models/ppo_alibaba.pt
```

Expected files in the trace directory:

- `machine_meta.csv`
- `batch_task.csv`

### Evaluate / train on Google holdout

```bash
python3 -m agentic_scheduler.train_ppo \
  --trace-source google \
  --trace-path /path/to/google-clusterdata-2019 \
  --num-workers 4 \
  --max-trace-tasks 5000 \
  --updates 100 \
  --output agentic_scheduler/models/ppo_google_holdout.pt
```

Accepted formats:

- `instance_events.json` + `machine_events.json`
- or CSV equivalents

### Domain-adapt on CloudAI history

```bash
python3 -m agentic_scheduler.train_ppo \
  --trace-source cloudai \
  --mongo-uri mongodb://localhost:27017 \
  --mongo-db cluster_db \
  --trace-start 2026-03-01T00:00:00Z \
  --trace-end 2026-03-15T00:00:00Z \
  --num-workers 4 \
  --max-trace-tasks 5000 \
  --updates 100 \
  --output agentic_scheduler/models/ppo_cloudai_adapted.pt
```

If `--trace-start` and `--trace-end` are omitted, the loader replays the earliest matching completed tasks.

## Model Lineage Metadata

Offline checkpoints persisted to MongoDB now include lineage fields in `SCHEDULER_MODELS.metadata`:

- `model_source`
- `training_corpus`
- `trace_window`
- `cluster_fingerprint`
- `train_timestamp`
- `updates`

Runtime checkpoints created by the live PPO service also record:

- `reason`
- `training_steps`
- `online_updates_enabled`

This makes it possible to distinguish:

- public-trace pretraining
- CloudAI domain adaptation
- live online adaptation

## Live PPO Modes

Set `PPO_MODE` on the master before startup:

- `active`: PPO decides worker placement. RTS remains the fallback on RPC errors or invalid PPO choices.
- `shadow`: PPO is queried and logged, but RTS still decides the actual worker placement.
- `fallback`: PPO is bypassed and RTS is used directly.

Examples:

```bash
export SCHED_ALGO=PPO
export PPO_MODE=shadow
./runMaster.sh
```

```bash
export SCHED_ALGO=PPO
export PPO_MODE=active
./runMaster.sh
```

```bash
export SCHED_ALGO=PPO
export PPO_MODE=fallback
./runMaster.sh
```

## Live Online Adaptation Switch

Set `PPO_ONLINE_UPDATES_ENABLED` to control whether the Python PPO service fine-tunes on live CloudAI outcomes:

```bash
export PPO_ONLINE_UPDATES_ENABLED=false
./runMaster.sh
```

When auto-start is enabled, the master forwards this setting to the Python PPO service.

## Suggested Rollout Path

1. Pretrain on Alibaba traces.
2. Evaluate on Google holdout.
3. Adapt on CloudAI history using a recent replay window.
4. Run production in `shadow` mode with `PPO_ONLINE_UPDATES_ENABLED=false`.
5. Review cluster outcomes and model behavior.
6. Switch to `active` and re-enable online adaptation if the shadow results hold.
