# Alibaba Test Benchmark — Comprehensive Scheduler Comparison

## Overview

`execute-alibaba-test` is an end-to-end benchmark script that:

- **Generates a contiguous Alibaba test split** from your local trace data (no overlap with train/test)
- **Creates realistic workload variants** (CPU-heavy, memory-heavy, mixed, bursty)
- **Trains PPO models** in both offline and online modes
- **Compares all 4 schedulers**: RR, RTS, PPO (offline), PPO (online)
- **Generates detailed comparison reports** with success rates, turnaround times, and wait times

## Quick Start

### Quick Start

#### Run both offline and online benchmarks (~3 hours):
```bash
./execute-alibaba-test
```

#### Run only offline PPO (~1.5 hours):
```bash
./execute-alibaba-test --offline
```

#### Run only online adaptive PPO (~1.5 hours):
```bash
./execute-alibaba-test --online
```

#### Tune online PPO behavior (less aggressive learning):
```bash
./execute-alibaba-test --online-lr-scale 0.05 --online-replay-batch 50
```

#### Custom test split parameters:
```bash
./execute-alibaba-test \
  --tasks 150000 \                    # Larger test split
  --start-row 1000000 \               # Custom start position
  --tasks-per-workload 40 \           # More tasks per workload
  --offline-updates 75                # More training iterations
```

## What It Does

### Phase 1: Preparation
1. **Validates dependencies** (Docker, Go, Python venv)
2. **Builds Go binaries** (master and worker nodes)
3. **Creates Alibaba test split** with contiguity validation and overlap checking
4. **Generates 4 test workloads**:
   - `alibaba-test-cpu`: CPU-heavy tasks
   - `alibaba-test-memory`: Memory-intensive tasks
   - `alibaba-test-mixed`: Mixed resource requirements
   - `alibaba-test-bursty`: Bursty arrival patterns

### Phase 2: Model Training (~45 minutes)
**Single training run**, then both modes reuse it:

**Offline PPO:**
- Trains 50 updates (optimized for speed)
- Learns a frozen policy from historical traces (baseline comparison)
- Output: `agentic_scheduler/results/ppo_offline_alibaba.pt`

**Online PPO:**
- Starts from the offline PPO model (no separate training!)
- Fine-tunes during benchmark execution on replay batches
- Uses tunable parameters:
  - `--replay-batch-size`: How many tasks to replay for online learning (default: 100)
  - `--online-lr-scale`: Learning rate scale [0-1] (default: 0.1)
- Demonstrates online learning capability under different workloads

### Phase 3: Deployment
1. **Starts Docker stack** (MongoDB, workers, observability)
2. **Starts local master node** (HOST-MASTER topology)
3. **Registers workers** with master
4. **Prepares workflow images**

### Phase 4: Benchmarking
Runs all combinations:
- **Schedulers**: RR, RTS, PPO (offline), PPO (online)
- **Workloads**: cpu, memory, mixed, bursty
- **Scenarios**: baseline, burst, overload (3 scenarios per workload)
- **Total runs**: 4 × 4 × 3 = 48 benchmark runs

### Phase 5: Analysis
Generates `comparison.md` with:
- Summary statistics (success rates, turnaround times)
- Detailed results by workload and scenario
- Scheduler rankings by metric
- Insights on when each scheduler excels

## Command-Line Options

```
--offline                   Run offline PPO only (default: both)
--online                    Run online PPO only (default: both)
--skip-build                Skip Go build step (use existing binaries)
--teardown                  Cleanup only (don't run tests)
--tasks N                   Task count for test split (default: 100,000)
--start-row N               Start row for contiguous split (default: auto-tail to avoid overlap)
--tasks-per-workload N      Tasks per workload JSON (default: 30)
--offline-updates N         PPO offline training updates (default: 50 for 3-hour turnaround)
--online-replay-batch N     Online PPO replay batch size (default: 100)
--online-lr-scale F         Online PPO learning rate scale [0-1] (default: 0.1)
```

## Output Structure

Results are saved to `results/alibaba-test-YYYYMMDD-HHMMSS/`:

```
results/alibaba-test-20260428-071500/
├── RR-alibaba-test-cpu-baseline.json      # Raw campaign results
├── RR-alibaba-test-cpu-burst.json
├── RTS-alibaba-test-cpu-baseline.json
├── PPO-alibaba-test-cpu-baseline.json
├── PPO_ONLINE-alibaba-test-cpu-baseline.json
├── ...
├── campaign-report.json                    # Aggregated metrics
└── comparison.md                           # Human-readable report
```

## Key Features

### 1. **Data Split Validation**
- Contiguous window extraction from source Alibaba CSV
- Automatic overlap checking against reference test split
- Prevents data leakage

### 2. **Workload Derivation**
- Buckets tasks by resource profile (CPU-heavy, memory-heavy, etc.)
- Normalizes Alibaba raw values to scheduler units
- Creates realistic, diverse task distributions

### 3. **Online vs Offline Modes**
- **Offline**: Fixed policy learned from historical traces (baseline)
- **Online**: Starts with offline model, fine-tunes during benchmark with replay batches
- **Tunable**: Control replay batch size and learning rate scale for online adaptation aggressiveness

### 4. **Scenario Variations**
- **Baseline**: Normal load distribution
- **Burst**: Sudden spikes in task arrivals
- **Overload**: Sustained high load (tests queue management)

## Example Output

```
Success Rate Comparison:
| Scheduler | Success Rate | Avg Turnaround (s) | Avg Wait (s) |
|-----------|--------------|--------------------|--------------| 
| RR        | 94.2%        | 12.45              | 3.21         |
| RTS       | 96.1%        | 11.02              | 2.15         |
| PPO       | 97.8%        | 10.15              | 1.93         |
| PPO_ONLINE| 98.3%        | 9.82               | 1.45         |

Best performers:
- PPO excels in CPU-heavy workloads (98.1% success)
- PPO_ONLINE adapts fastest to bursty patterns (99.2% success in burst scenarios)
- RTS maintains consistent baseline across all workloads
```

## 3-Hour Optimization Strategy

### Why Fast Iteration Matters
- Allows tweaking models if first results are suboptimal
- Quick feedback loop for tuning online adaptation parameters
- Cost-effective for exploratory benchmarking

### How We Achieve 3-Hour Target
| Factor | Original | Optimized | Impact |
|--------|----------|-----------|--------|
| Test split size | 300K tasks | 100K tasks | -66% split creation |
| Training updates | 150 offline + 75 online | 50 offline only | -66% training time |
| Tasks per workload | 40 | 30 | -25% per run |
| Online training | Separate 75 updates | None (inherits offline) | -100% online training |
| **Total runs** | 48 | 48 | Same coverage |
| **Execution time** | ~8 hours | ~3 hours | -62% |

### Breakdown (~3 hours)
- **Build** (10 min) — Go binaries
- **Prep** (15 min) — Split creation + workload generation
- **Training** (30 min) — Offline PPO (50 updates)
- **Deploy** (10 min) — Docker + workers
- **Benchmark** (100 min) — 48 runs × ~2.1 min each
- **Analysis** (5 min) — Generate reports
- **Total** — ~170 minutes (~3 hours)

### Online PPO Optimization Detail
**Old approach**: Train separate base model for 75 updates  
**New approach**: Start from frozen offline model, tune during benchmark with:
- **Replay batch size** (default 100): How many tasks to use for online learning per workload
- **Learning rate scale** (default 0.1): How aggressively to adapt (0.0 = frozen, 1.0 = aggressive)

This eliminates 45 minutes of training while maintaining online adaptation capability!

## Troubleshooting

### Docker services not starting
```bash
docker compose -f testbench/docker-compose.host-master.yml down -v
./execute-alibaba-test  # Restart fresh
```

### Worker registration fails
Check that Docker services are healthy:
```bash
docker compose -f testbench/docker-compose.host-master.yml ps
```

### Benchmark hangs or times out
Cancel and run with smaller dataset:
```bash
Ctrl+C
./execute-alibaba-test --tasks 100000 --tasks-per-workload 20
```

## Performance Tuning

### Faster execution:
```bash
./execute-alibaba-test \
  --tasks 100000 \                    # Smaller split
  --tasks-per-workload 20 \           # Fewer tasks per workload
  --skip-build                        # Use existing binaries
```

### More comprehensive benchmark:
```bash
./execute-alibaba-test \
  --tasks 500000 \                    # Larger split
  --tasks-per-workload 100            # More diverse workloads
```

### Iterate on offline only:
```bash
./execute-alibaba-test --offline      # 48 runs of offline PPO vs RR/RTS, ~2 hours
```

## Integration with Existing Infrastructure

This script works with the existing:
- `execute-tests.sh` (alternate entry point for different campaigns)
- `run_campaign.py` (underlying campaign execution engine)
- Docker Compose stack (host-master topology)
- PPO training infrastructure

To run the original Alibaba test via `execute-tests.sh`:
```bash
./execute-tests.sh --alibaba-test  # Uses internal flags instead
```

## Architecture Decisions

1. **Why separate script?**
   - Clearer semantics for "compare all schedulers on Alibaba workloads"
   - Easier to document and teach
   - Better support for online/offline modes

2. **Why contiguous split?**
   - Avoids data leakage (no overlap with existing train/test)
   - More realistic distribution than random sampling
   - Simpler validation and reproducibility

3. **Why offline + online?**
   - Offline shows PPO's learned policy quality
   - Online shows PPO's online adaptation capability under distribution shifts
   - Together they demonstrate PPO's full potential

## Next Steps

1. **Run the benchmark:**
   ```bash
   ./execute-alibaba-test
   ```

2. **Analyze results:**
   ```bash
   cat results/alibaba-test-*/comparison.md
   ```

3. **Iterate on scenarios:**
   - Adjust `--tasks`, `--start-row`, `--tasks-per-workload` as needed
   - Create custom workloads in `testbench/workloads/`

4. **Compare with Google ClusterData2019:**
   - Use similar pattern with real-world Google traces
   - Validates findings across different cluster profiles
