# Task 5.1: CLI-Based Testing Guide

## Overview

This guide provides step-by-step instructions for testing the CloudAI scheduler using the CLI-driven test suite. The test infrastructure submits 30 Docker images across 6 workload types and monitors execution, SLA compliance, and GA optimizer behavior.

## Test Infrastructure

### Components

1. **Workload Submission Scripts** (`test/workloads/`)
   - Individual scripts per workload type (CPU-light, CPU-heavy, Memory-heavy, GPU-inference, GPU-training, Mixed)
   - Automatic retry logic with exponential backoff (up to 5 attempts)
   - HTTP POST to Master API `/api/tasks`

2. **Full Test Wrapper** (`test/submit_all.sh`)
   - Submits all 30 images sequentially
   - Real-time progress monitoring with status bar
   - Automatic completion detection
   - Summary report generation

3. **Verification Scripts** (`test/verify/`)
   - Task distribution checker (per-worker assignment counts)
   - SLA violation analyzer (success rates by task type)
   - GA output validator (AffinityMatrix and parameters)

4. **Worker Registration** (`test/register_workers.sh`)
   - Registers 4 workers: Shehzada, kiwi, NullPointer, Tessa
   - Fallback between HTTP API and CLI socket

## Prerequisites

### Required Services

1. **Master Node** (running on port 8080 HTTP, 50051 gRPC)
   ```bash
   ./runMaster.sh
   ```

2. **MongoDB** (for persistent storage and verification)
   ```bash
   cd database && docker-compose up -d
   ```

3. **Worker Nodes** (4 workers on different machines)
   ```bash
   # On each worker machine
   ./runWorker.sh
   ```

### Required Tools

- `curl` - HTTP client for API calls
- `jq` - JSON processor for parsing responses
- `mongo` CLI - MongoDB shell (optional but recommended for detailed verification)
- `docker` - Docker engine (on workers for task execution)

### Installation (if missing)

```bash
# Ubuntu/Debian
sudo apt-get install curl jq mongodb-clients

# macOS
brew install curl jq mongodb-community-shell

# Check installations
curl --version
jq --version
mongo --version
```

## Quick Start (Smoke Test)

Before running the full suite, verify basic functionality:

```bash
# 1. Ensure master is running
curl http://localhost:8080/health || curl http://localhost:8080/

# 2. Register workers
bash test/register_workers.sh

# 3. Run smoke test (2 tasks only)
bash test/smoke_test.sh
```

Expected output:
```
╔════════════════════════════════════════════════╗
║      CloudAI Quick Smoke Test (2 tasks)       ║
╚════════════════════════════════════════════════╝

🔍 Checking master API...
✓ Master API is healthy

📦 Test 1: Submitting CPU-light task...
✓ CPU-light task submitted: task_abc123

📦 Test 2: Submitting CPU-heavy task...
✓ CPU-heavy task submitted: task_def456

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✓ Smoke test passed!
```

## Full Test Suite Execution

### Step 1: Register Workers

```bash
bash test/register_workers.sh
```

Verify registration via master CLI:
```bash
# In master terminal, type:
workers
```

Should show 4 workers with status "idle" or "active".

### Step 2: Run Full Workload (30 Tasks)

```bash
bash test/submit_all.sh
```

This script will:
1. ✅ Check prerequisites (API health, MongoDB, scripts)
2. 📦 Submit all 30 Docker images (5 per type × 6 types)
3. 📊 Monitor completion with real-time progress bar
4. 💾 Save logs to `test/logs/submission_YYYYMMDD_HHMMSS.log`
5. 📈 Generate JSON report to `test/logs/results_YYYYMMDD_HHMMSS.json`

**Expected Duration:** 10-30 minutes depending on:
- Worker performance
- Network speed (Docker image pulls)
- Task complexity (GPU training takes longest)

### Step 3: Monitor Progress

The script provides real-time updates:

```
[10:18:45] Starting full workload submission (30 tasks)
[10:18:45] ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[10:18:46] 📦 Submitting CPU-Light workload...
[10:18:48] CPU-Light: 5 tasks submitted
[10:18:49] 📦 Submitting CPU-Heavy workload...
[10:18:55] CPU-Heavy: 5 tasks submitted
...

[10:19:30] Monitoring task completion (polling every 10s)
[10:19:40] [##########                                        ] 20% | Completed: 6 | Running: 4 | Pending: 20 | Failed: 0
[10:19:50] [####################                              ] 40% | Completed: 12 | Running: 6 | Pending: 12 | Failed: 0
...
[10:25:30] [##################################################] 100% | Completed: 30 | Running: 0 | Pending: 0 | Failed: 0
[10:25:30] All tasks finished!
```

### Step 4: Verify Results

After completion, run verification scripts:

#### 4a. Check Task Distribution
```bash
bash test/verify/check_task_distribution.sh
```

Example output:
```json
[
  { "_id": "Shehzada", "count": 8 },
  { "_id": "kiwi", "count": 7 },
  { "_id": "NullPointer", "count": 8 },
  { "_id": "Tessa", "count": 7 }
]
```

✅ **Validation:** Tasks should be relatively balanced across workers (±2 tasks variance is acceptable).

#### 4b. Check SLA Compliance
```bash
bash test/verify/check_sla_violations.sh
```

Example output:
```json
[
  { "task_type": "cpu-light", "total": 5, "sla_success": 5, "sla_rate": 1.0 },
  { "task_type": "cpu-heavy", "total": 5, "sla_success": 4, "sla_rate": 0.8 },
  { "task_type": "gpu-training", "total": 5, "sla_success": 3, "sla_rate": 0.6 }
]
```

✅ **Validation:**
- SLA rate > 0.8 for light workloads is good
- SLA rate 0.6-0.8 for heavy workloads is acceptable (training takes time)
- SLA rate < 0.5 indicates potential scheduling issues

#### 4c. Check GA Output
```bash
bash test/verify/check_ga_output.sh
```

Example output:
```json
{
  "Theta": { "Theta1": 0.15, "Theta2": 0.12, "Theta3": 0.25, "Theta4": 0.18 },
  "Risk": { "Alpha": 9.5, "Beta": 1.2 },
  "AffinityMatrix": {
    "cpu-light": [0.8, 0.9, 0.7, 0.85],
    "cpu-heavy": [0.6, 0.7, 0.65, 0.68],
    ...
  },
  "PenaltyVector": {
    "cpu-light": 0.15,
    "cpu-heavy": 0.22,
    ...
  }
}
```

✅ **Validation:**
- `AffinityMatrix` should have 6 rows (one per task type)
- Affinity values range 0.0-1.0 (higher = better worker-task match)
- `PenaltyVector` should have 6 entries (penalty per task type)
- Values should differ from defaults after GA training

## Advanced Usage

### Custom Test Scenarios

#### Test Individual Workload Type
```bash
API=http://localhost:8080 bash test/workloads/submit_cpu_light.sh
```

#### Test with Custom Timeout
```bash
# 1 hour timeout instead of default 30 minutes
TIMEOUT=3600 bash test/submit_all.sh
```

#### Test Against Remote Master
```bash
API=http://10.1.133.148:8080 bash test/submit_all.sh
```

#### Custom MongoDB Connection
```bash
MONGO=mongodb://remote-host:27017 DB=cloudai_test bash test/submit_all.sh
```

### Retry Configuration

Edit `test/workloads/common.sh` to adjust retry behavior:

```bash
# Default: 5 retries with exponential backoff
max_retries=5
backoff=1  # Initial: 1s, then 2s, 4s, 8s, 16s
```

## Troubleshooting

### Issue: API Connection Refused

**Symptoms:**
```
❌ Master API unreachable at http://localhost:8080
```

**Solutions:**
1. Check master is running: `ps aux | grep masterNode`
2. Verify port: `netstat -tuln | grep 8080`
3. Check master logs for startup errors
4. Try health endpoint: `curl -v http://localhost:8080/health`

### Issue: Tasks Stuck in Pending

**Symptoms:**
```
[##########                                        ] 20% | Completed: 6 | Running: 0 | Pending: 24 | Failed: 0
⚠️  No progress for 2 minutes - tasks may be stalled
```

**Solutions:**
1. Check worker registration: In master CLI, type `workers`
2. Verify workers can reach Docker Hub: `docker pull moinvinchhi/cloudai-cpu-light:1`
3. Check worker resource availability: Workers may lack CPU/Memory/GPU
4. Review master scheduler logs for assignment errors

### Issue: High SLA Violation Rate

**Symptoms:**
```
{ "task_type": "cpu-heavy", "sla_success": 1, "sla_rate": 0.2 }
```

**Solutions:**
1. Check worker performance: Tasks may be taking longer than expected
2. Review tau values: In master CLI, type `internal-state` to see tau per task type
3. Verify network latency: Slow Docker image pulls increase execution time
4. Check GA parameters: AffinityMatrix may need more training data

### Issue: MongoDB Connection Failed

**Symptoms:**
```
⚠️  MongoDB not accessible - monitoring will be limited
```

**Solutions:**
1. Start MongoDB: `cd database && docker-compose up -d`
2. Check connection: `mongo mongodb://localhost:27017/cloudai --eval "db.runCommand({ping:1})"`
3. Verify master can write to MongoDB: Check master logs for telemetry save errors
4. Alternative: Use HTTP API only (monitoring still works but less detailed)

### Issue: Worker Registration Fails

**Symptoms:**
```
No HTTP register endpoint detected, try CLI registration
```

**Solutions:**
1. **Option A:** Use master CLI directly
   ```bash
   # In master terminal
   master> register Shehzada 10.1.133.148:50052
   master> register kiwi 10.1.174.169:50052
   master> register NullPointer 10.1.186.172:50052
   master> register Tessa 10.1.129.143:50052
   ```

2. **Option B:** Update registration script to match master's CLI socket
   ```bash
   # Edit test/register_workers.sh
   # Change: nc localhost 7000
   # To: nc localhost <actual_cli_port>
   ```

## Expected Test Results

### Baseline Metrics (First Run)

| Metric | Expected Value | Notes |
|--------|---------------|-------|
| Total Tasks | 30 | 5 per workload type |
| Submission Success Rate | 95-100% | Retries handle transient failures |
| Completion Rate | 95-100% | Some tasks may fail due to worker issues |
| Average Duration | 10-30 min | Varies by worker performance |
| SLA Success Rate (Light) | 80-100% | Fast tasks should meet SLA easily |
| SLA Success Rate (Heavy) | 60-80% | Training tasks take longer |
| Task Distribution Variance | ±2 tasks | Round-robin should balance load |

### After GA Training (Multiple Runs)

After running the test suite 3-5 times:

| Metric | Expected Improvement | Validation |
|--------|---------------------|------------|
| SLA Success Rate | +5-15% | GA learns better worker-task matches |
| Task Distribution | More uneven | GA assigns tasks to best-fit workers |
| Affinity Values | Converge | Worker affinities stabilize |
| Tau Updates | Decrease frequency | Estimates become more accurate |

## Interpreting Results

### Good Results ✅

- All 30 tasks submitted successfully
- Completion rate > 95%
- SLA success rate > 70% average
- AffinityMatrix populated with 6 task types
- Task distribution balanced (first run) or optimized (after GA training)

### Needs Investigation ⚠️

- Submission failures > 2 tasks (API issues or master overload)
- Completion rate < 90% (worker stability issues)
- SLA success rate < 60% (tau estimates inaccurate or workers underperforming)
- AffinityMatrix still empty after completion (GA not training)

### Critical Issues ❌

- Cannot reach master API (master not running or network issue)
- Zero tasks completed (workers not registered or not reachable)
- All tasks failed (Docker image pull failures or execution errors)
- Master crashes during test (check logs for panic/OOM)

## Next Steps

After successful test execution:

1. **Repeat Test Runs:** Run `submit_all.sh` multiple times to train GA
2. **Analyze Trends:** Compare `results_*.json` files across runs
3. **Tune Parameters:** Adjust GA parameters in `master/config/ga_output.json`
4. **Scale Testing:** Add more workers and increase workload size
5. **Performance Profiling:** Use `pprof` endpoints to analyze master performance
6. **Stress Testing:** Submit tasks continuously to test under load

## Appendix: Manual Testing Commands

If you prefer manual testing via master CLI:

```bash
# In master terminal:

# Submit a task
master> task moinvinchhi/cloudai-cpu-light:1 1.0 2.0 0.0 -type cpu-light

# Check queue
master> queue

# Monitor specific task
master> monitor <task_id>

# View all tasks
master> list-tasks

# Check internal state
master> internal-state

# View statistics
master> stats
```

## References

- [Master CLI Quick Reference](./CLI_QUICK_REFERENCE.md)
- [Sprint Plan (Milestones 5-7)](./SPRINT_PLAN.md)
- [Revised Testing Plan](./REVISED_TESTING_PLAN.md)
- [Test README](../../test/README.md)
