# Test Scripts for CloudAI

This folder contains CLI/HTTP-driven test helpers used for Milestone 5 (CLI-based testing).

## Prerequisites

- Master service running and reachable at http://localhost:8080 (or set $API)
- jq installed for JSON pretty printing
- mongo shell installed for detailed monitoring (optional)
- 4 workers registered with the master

## Quick Start

### 1. Start Master and Workers
```bash
# Terminal 1: Start master
./runMaster.sh

# Terminal 2-5: Start workers on different machines
./runWorker.sh
```

### 2. Register Workers
```bash
bash test/register_workers.sh
```

### 3. Run Full Test Suite (All 30 Images)
```bash
# Submit all workloads and monitor completion
bash test/submit_all.sh

# Or with custom settings
API=http://localhost:8080 TIMEOUT=3600 bash test/submit_all.sh
```

## Individual Scripts

### Workload Submission
All submission scripts support automatic retry with exponential backoff (up to 5 attempts).

- `workloads/submit_cpu_light.sh` — 5 CPU-light tasks (Health checks, parsers)
- `workloads/submit_cpu_heavy.sh` — 5 CPU-heavy tasks (Monte Carlo, primes, matrix ops)
- `workloads/submit_memory_heavy.sh` — 5 Memory-heavy tasks (Dataset merge, aggregation)
- `workloads/submit_gpu_inference.sh` — 5 GPU-inference tasks (Text embedding, classification)
- `workloads/submit_gpu_training.sh` — 5 GPU-training tasks (ResNet, LLM, GAN training)
- `workloads/submit_mixed.sh` — 5 Mixed workloads (ETL, preprocessing, validation)

Usage:
```bash
# Submit a specific workload type
API=http://localhost:8080 bash test/workloads/submit_cpu_light.sh

# Submit all types sequentially
for script in cpu_light cpu_heavy memory_heavy gpu_inference gpu_training mixed; do
  bash test/workloads/submit_${script}.sh
done
```

### Worker Registration
- `register_workers.sh` — Register 4 workers (Shehzada, kiwi, NullPointer, Tessa)

### Verification Scripts
- `verify/check_task_distribution.sh` — Query MongoDB to see task assignments per worker
- `verify/check_sla_violations.sh` — Compute SLA success rates by task type
- `verify/check_ga_output.sh` — Pretty-print master/config/ga_output.json

Usage:
```bash
bash test/verify/check_task_distribution.sh
bash test/verify/check_sla_violations.sh
bash test/verify/check_ga_output.sh
```

## Full Test Suite (`submit_all.sh`)

The comprehensive wrapper script that:
- ✅ Checks master API health and prerequisites
- 📦 Submits all 30 Docker images (6 workload types)
- 📊 Monitors task completion with progress bar
- ⏱️ Tracks execution time and detects stalls
- 📈 Generates detailed summary report with metrics
- 💾 Saves logs to `test/logs/`

Features:
- **Automatic retries**: Each task submission retries up to 5 times with exponential backoff
- **Progress monitoring**: Real-time progress bar showing completed/running/pending/failed tasks
- **Stall detection**: Alerts if no progress for 2 minutes
- **Detailed reporting**: JSON summary with task type breakdown, worker distribution, SLA rates
- **Timeout protection**: Configurable timeout (default 30 minutes)

Environment variables:
```bash
API=http://localhost:8080      # Master API endpoint
MONGO=mongodb://localhost:27017 # MongoDB connection string
DB=cloudai                      # Database name
POLL_INTERVAL=10                # Status check interval (seconds)
TIMEOUT=1800                    # Maximum wait time (seconds)
```

Example output:
```
╔════════════════════════════════════════════════════╗
║   CloudAI Full Workload Test - 30 Docker Images   ║
╚════════════════════════════════════════════════════╝

[10:15:30] Starting test run at Mon Nov 17 10:15:30 UTC 2025
[10:15:30] Checking prerequisites...
[10:15:31] Master API is reachable
[10:15:31] MongoDB is accessible
[10:15:31] All workload scripts found

[10:15:32] ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[10:15:32] Starting full workload submission (30 tasks)
[10:15:32] ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[10:15:33] 📦 Submitting CPU-Light workload...
[10:15:35] CPU-Light: 5 tasks submitted
...
[10:18:45] Submission complete in 193s
[10:18:45]   ✓ Submitted: 30 tasks

[10:18:45] Monitoring task completion (polling every 10s)
[10:18:55] [####################                              ] 40% | Completed: 12 | Running: 8 | Pending: 10 | Failed: 0
...
[10:25:30] [##################################################] 100% | Completed: 30 | Running: 0 | Pending: 0 | Failed: 0
[10:25:30] All tasks finished!

[10:25:31] 📊 Key Metrics:
[10:25:31]   Total Tasks: 30
[10:25:31]   Status Breakdown:
[10:25:31]     - completed: 30
```

## Output Files

Test runs generate logs in `test/logs/`:
- `submission_YYYYMMDD_HHMMSS.log` — Full execution log
- `results_YYYYMMDD_HHMMSS.json` — JSON summary with metrics

## Troubleshooting

**API unreachable:**
```bash
# Check master is running
curl http://localhost:8080/health

# Or try root endpoint
curl http://localhost:8080/
```

**MongoDB not accessible:**
```bash
# Start MongoDB
cd database && docker-compose up -d

# Test connection
mongo mongodb://localhost:27017/cloudai --eval "db.runCommand({ping:1})"
```

**Tasks stuck in pending:**
- Ensure workers are registered: check master CLI with `workers` command
- Verify workers are running and can pull Docker images
- Check worker resource availability

**Submission failures:**
- Review logs in `test/logs/submission_*.log`
- Check master logs for API errors
- Verify Docker images are accessible on Docker Hub
