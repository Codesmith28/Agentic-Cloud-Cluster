# Agentic Scheduler: PPO-Based Task Scheduling

AI-driven task scheduling using Proximal Policy Optimization (PPO), a reinforcement learning algorithm trained on production workload traces.

**See Also:** [Project Structure Guide](../docs/PROJECT_STRUCTURE.md#agentic-scheduler-agentic_scheduler) | [Project Appendix](../docs/PROJECT_APPENDIX.md) | [ARCHITECTURE](../ARCHITECTURE.md)

---

## Overview

The agentic scheduler (PPO) is an **optional, pluggable component** that provides ML-based scheduling decisions. It can:

- Train offline on historical task traces (e.g., Alibaba cluster traces)
- Serve scheduling decisions via gRPC at inference time
- Adapt online as the master reports task completion outcomes
- Deploy in multiple modes: active, shadow, or fallback

Unlike fixed-logic schedulers (RTS, Round-Robin), PPO learns optimal policies from data and adapts to workload characteristics.

---

## Architecture

### High-Level Flow

```
Master Node
    ↓
    ├─→ [Scheduler] Pick next task
    │        ↓
    │   ┌────────────────────┐
    │   │ PPO Service        │
    │   │ (gRPC :50050)      │
    │   ├─────────────────── │
    │   │ ✓ Load checkpoint  │
    │   │ ✓ Encode features  │
    │   │ ✓ Infer action     │
    │   │ ✓ Return decision  │
    │   │ ✓ Update online    │
    │   └────────────────────┘
    │
    └─→ Assign task to selected worker
```

### Module Responsibilities

| Module | Purpose | Key Files |
|--------|---------|-----------|
| **Model** | Core PPO neural network architecture | `model.py` |
| **Features** | Encode task/worker state into input tensors | `features.py` |
| **Service** | PPO policy lifecycle (load, infer, update) | `service.py` |
| **Server** | gRPC server exposing scheduling API | `server.py` |
| **Persistence** | MongoDB checkpoint storage & retrieval | `persistence.py` |
| **Training** | Offline RL training pipeline | `train_ppo.py` |
| **Data Loading** | Trace ingestion & preprocessing | `training/` |

---

## Installation & Setup

### Prerequisites

- Python 3.8+
- PyTorch (CPU or GPU)
- Dependencies in `requirements.txt`

### Installation

```bash
# Create virtual environment
python3 -m venv venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt
```

### Running the PPO Service

```bash
# Start PPO gRPC server (listening on :50050)
python3 -m agentic_scheduler.server

# Or use Make
make ppo-server
```

### Configuration

| Environment Variable | Default | Purpose |
|----------------------|---------|---------|
| `PPO_MODEL_PATH` | `latest` | Model checkpoint path (auto-detect latest or specify exact path) |
| `PPO_AUTOSTART` | `true` | Auto-start Python gRPC service when master starts |
| `PPO_GRPC_ADDR` | `127.0.0.1:50050` | Address master uses to connect to PPO service |
| `PPO_DEPLOYMENT_MODE` | `active` | `active` (used for scheduling), `shadow` (test only), `fallback` (fallback if unavailable) |
| `PPO_ONLINE_UPDATES_ENABLED` | `true` | Enable online learning (PPO adapts as tasks complete) |

---

## Usage

### Starting Master with PPO

```bash
# Use Make
make run-master-ppo

# Or directly with environment variables
export SCHED_ALGO=PPO
export PPO_AUTOSTART=true
export PPO_MODEL_PATH=latest
./runMaster.sh
```

### Training (Offline)

```bash
# Prepare training data (Alibaba traces)
python3 agentic_scheduler/train_ppo.py \
  --trace-dir data/alibaba_traces/ \
  --output agentic_scheduler/results/ppo_trained_latest.pt \
  --updates 200

# Promote to active
./scripts/model_promote.sh agentic_scheduler/results/ppo_trained_latest.pt
```

### Benchmarking

```bash
# Run campaign with PPO scheduler
./execute-tests.sh

# Specific workload
./execute-tests.sh --full

# Isolated mode (reset model per workload)
./execute-tests.sh --isolated-workloads
```

---

## Key Concepts

### Policy Learning

PPO learns an optimal policy: `π(action | state)` that maximizes expected reward (task completion within SLA).

- **State:** Task features (CPU, memory, duration) + worker state (available resources, queue depth)
- **Action:** Which worker to assign the task to
- **Reward:** +1 if task completes within SLA, -1 otherwise (configurable)

### Fingerprinting

Each **workload fingerprint** is a unique combination of:
- Task size distribution
- Inter-arrival rate patterns
- Worker heterogeneity

PPO maintains separate policies per fingerprint, allowing specialization for different workload types.

### Online Adaptation

After training, PPO can adapt in real-time:
- Collect task completion outcomes
- Update policy parameters using gradient steps
- Improve over time without retraining

Controlled via `PPO_ONLINE_UPDATES_ENABLED` environment variable.

---

## Model Management

### Model Files

```
agentic_scheduler/models/
├── ppo_latest.pt              # Active model (used at inference)
├── PPO_frozen.pt              # Read-only baseline (chmod 444)
├── archive/
│   ├── v001_20260426-164212.pt
│   ├── v002_20260426-183335.pt
│   └── (previous versions)
└── checkpoints/               # Periodic training checkpoints
    ├── ppo_offline_u000010_s000010.pt
    └── (other checkpoints)
```

### Promotion Workflow

```bash
# 1. Train offline
python3 agentic_scheduler/train_ppo.py \
  --trace-dir data/alibaba_traces/ \
  --output agentic_scheduler/results/ppo_trained_final.pt

# 2. Promote to active (archives previous)
./scripts/model_promote.sh agentic_scheduler/results/ppo_trained_final.pt

# 3. Master auto-detects & loads latest
# (or specify PPO_MODEL_PATH=agentic_scheduler/models/ppo_latest.pt)
```

### Version History

```bash
# List archived versions
make model-archive-list

# Dry-run: preview promotion
./scripts/model_promote.sh --dry-run path/to/model.pt
```

---

## Troubleshooting

### PPO Service Not Starting

```bash
# Check if port is available
netstat -tuln | grep 50050

# Check for Python errors
python3 -m agentic_scheduler.server 2>&1 | head -50

# Verify PyTorch installation
python3 -c "import torch; print(torch.__version__)"
```

### Model Checkpoint Not Found

```bash
# Check available models
ls -la agentic_scheduler/models/

# Manually set model path
export PPO_MODEL_PATH=/path/to/model.pt
python3 -m agentic_scheduler.server
```

### Online Learning Not Working

```bash
# Verify online learning is enabled
export PPO_ONLINE_UPDATES_ENABLED=true

# Check MongoDB connectivity
docker exec cloudai-mongo mongosh --quiet --eval "db.SCHEDULER_MODELS.countDocuments()"

# Monitor PPO service logs
python3 -m agentic_scheduler.server 2>&1 | grep -i "online\|update"
```

---

## Documentation

### Core Architecture
- **[TRAINING_ARCHITECTURE.md](TRAINING_ARCHITECTURE.md)** — Detailed training system design
- **[TRAINING_DECISIONS.md](TRAINING_DECISIONS.md)** — Hyperparameter choices & justification

### Project Documentation
- **[docs/PROJECT_STRUCTURE.md](../docs/PROJECT_STRUCTURE.md)** — Codebase organization
- **[docs/PROJECT_APPENDIX.md](../docs/PROJECT_APPENDIX.md)** — Scripts & utilities
- **[ARCHITECTURE.md](../ARCHITECTURE.md)** — System-wide design

---

## Development

### Adding a New Feature

1. **Model Changes:**
   - Edit `model.py` (network architecture)
   - Update `features.py` if encoding changes
   - Retrain: `python3 agentic_scheduler/train_ppo.py`

2. **Training Changes:**
   - Edit `train_ppo.py` (algorithm parameters)
   - Add tests in `training/`
   - Regenerate checkpoints

3. **Inference Changes:**
   - Edit `service.py` or `server.py`
   - Test with `make ppo-server` + manual calls
   - Update `docs/` if behavior changes

### Running Tests

```bash
# Unit tests (if implemented)
python3 -m pytest agentic_scheduler/tests/ -v

# Integration test (full training + inference)
./execute-tests.sh --model agentic_scheduler/results/ppo_trained_latest.pt
```

---

## Performance Tuning

### Hyperparameters (in `train_ppo.py`)

| Parameter | Default | Effect |
|-----------|---------|--------|
| `--learning-rate` | 3e-4 | Policy gradient step size |
| `--batch-size` | 32 | Samples per update step |
| `--epochs` | 3 | Training epochs per rollout |
| `--updates` | 200 | Total training updates |
| `--rollout-steps` | 512 | Samples collected per update |

### Tuning Strategy

1. **Start with defaults** — Train baseline model
2. **Benchmark** — Measure performance (task success rate, SLA compliance)
3. **Ablate** — Change one hyperparameter at a time
4. **Compare** — Track metrics across versions using `make model-archive-list`
5. **Promote** — Keep best version as active

---

## Comparison with Other Schedulers

| Scheduler | Type | Learning | Deployment | Latency |
|-----------|------|----------|------------|---------|
| **RR (Round-Robin)** | Fixed | None | Direct in master | < 1ms |
| **RTS** | Fixed + tuned | Offline GA | Direct in master | < 1ms |
| **PPO** | ML-based | Offline + online | Separate gRPC service | ~50-100ms |

**Choose PPO when:**
- Workload characteristics are complex/time-varying
- You have good training data (historical traces)
- Slight latency increase is acceptable
- Online learning potential is valuable

---

**Last Updated:** 2026-05-13  
**Version:** 2.0 (Post-Cleanup Reorganization)
