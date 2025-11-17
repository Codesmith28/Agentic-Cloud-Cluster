# 🚀 Advanced Testing & Visualization Guide

## Overview
This guide covers batch task submission, real-time monitoring, and comprehensive visualization for CloudAI scheduler testing.

---

## 🔄 Testing Workflow Overview

```
┌─────────────────────────────────────────────────────────────┐
│  STEP 0: Start Services (One Time Setup)                   │
├─────────────────────────────────────────────────────────────┤
│  1. Start MongoDB (if not already running)                  │
│  2. Start Master Node (Terminal 1)                          │
│  3. Start Worker Nodes (Multiple machines/terminals)        │
│  4. Verify connections: curl http://localhost:8080/api/workers│
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  STEP 1: Submit Tasks (Terminal 2)                          │
├─────────────────────────────────────────────────────────────┤
│  bash test/batch_submit.sh                                  │
│  → Submits 40 tasks via HTTP API                            │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  STEP 2: Monitor Metrics (Terminal 3 - Parallel)            │
├─────────────────────────────────────────────────────────────┤
│  bash test/monitor_metrics.sh                               │
│  → Collects real-time metrics every 5 seconds               │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  STEP 3: Generate Visualizations (After completion)         │
├─────────────────────────────────────────────────────────────┤
│  bash test/visualize_results.sh                             │
│  → Creates 5 PNG plots + summary JSON                       │
└─────────────────────────────────────────────────────────────┘
```

---

## ⚙️ Prerequisites (Start Services First!)

**IMPORTANT:** Before running any tests, ensure Master and Workers are running.

### **Start Master Node** (Run on master machine)
```bash
cd /media/udaan/New\ Volume/Ahmedabad\ University/7th\ Sem/Cloud/CloudAI/master

# Option 1: Using shell script
bash ../runMaster.sh

# Option 2: Direct execution
./master
```

**Verify Master is Running:**
```bash
# Check gRPC server
curl http://localhost:8080/api/health

# Check master CLI prompt appears
# You should see: master>
```

---

### **Start Worker Nodes** (Run on each worker machine)

**On Worker Machine 1 (e.g., kiwi):**
```bash
cd /media/udaan/New\ Volume/Ahmedabad\ University/7th\ Sem/Cloud/CloudAI/worker

# Option 1: Using shell script
bash ../runWorker.sh

# Option 2: Direct execution
./worker
```

**On Worker Machine 2 (e.g., NullPointer):**
```bash
cd /media/udaan/New\ Volume/Ahmedabad\ University/7th\ Sem/Cloud/CloudAI/worker
bash ../runWorker.sh
```

**Verify Workers are Connected:**
```bash
# On master machine
curl http://localhost:8080/api/workers

# Or in master CLI
master> workers
```

**Expected Output:**
- 2-4 active workers with `is_active: true`
- Each worker showing available resources (CPU, Memory, GPU)

---

### **Ensure MongoDB is Running**
```bash
# Check MongoDB status
systemctl status mongodb
# OR
mongo cloudai --eval "db.stats()"

# If not running, start it:
sudo systemctl start mongodb
```

---

## 📦 Quick Start (Complete Workflow)

> **⚠️ IMPORTANT:** Before proceeding, ensure you have completed the Prerequisites section above:
> - ✅ Master node is running (port 50051)
> - ✅ 2-4 Worker nodes are connected
> - ✅ MongoDB is running
> 
> Verify with: `curl http://localhost:8080/api/workers | jq`

---

### 1. **Batch Submit 40 Tasks**
```bash
# Submit all 40 tasks concurrently (10 at a time)
bash test/batch_submit.sh

# Custom concurrency (e.g., 20 parallel submissions)
CONCURRENT=20 bash test/batch_submit.sh
```

**Output:**
- Real-time submission progress
- Success/failure count
- Results saved to `test/results/batch_submission_TIMESTAMP.json`

---

### 2. **Monitor Metrics in Real-time**
```bash
# Monitor for 5 minutes (default)
bash test/monitor_metrics.sh

# Monitor for 10 minutes with 3-second intervals
DURATION=600 INTERVAL=3 bash test/monitor_metrics.sh
```

**Metrics Collected:**
- Task states (pending, running, completed, failed)
- SLA success rate
- Throughput (tasks/min)
- Tau evolution by task type
- Worker distribution

**Output:**
- Real-time console updates
- Metrics saved to `test/results/metrics/metrics_TIMESTAMP.jsonl`

---

### 3. **Generate Visualizations**
```bash
# Auto-detect latest metrics file
bash test/visualize_results.sh

# Specify metrics file
bash test/visualize_results.sh test/results/metrics/metrics_20250117_143022.jsonl
```

**Generated Plots:**
1. `tau_evolution.png` - Tau values over time by task type
2. `throughput.png` - Task completion rate
3. `sla_success.png` - SLA compliance over time
4. `worker_distribution.png` - Task distribution across workers
5. `task_states.png` - Task state transitions
6. `summary_stats.json` - Key performance metrics

**Output Location:** `test/results/metrics/plots/`

---

### 4. **Compare RTS vs Round-Robin**
```bash
# Run full comparison (requires manual scheduler switching)
bash test/compare_schedulers.sh
```

**Process:**
1. Script prompts to switch to Round-Robin scheduler
2. Submits 40 tasks, waits for completion
3. Collects metrics
4. Prompts to switch to RTS scheduler
5. Repeats submission and collection
6. Generates comparison report

**Output:**
- `test/results/comparison_TIMESTAMP/round_robin_results.json`
- `test/results/comparison_TIMESTAMP/rts_results.json`
- `test/results/comparison_TIMESTAMP/comparison.json`
- Console table showing winner

---

## 📊 Visualization Examples

### Tau Evolution
Shows how task execution time (tau) changes over time for each task type. Helps identify:
- Learning curve of GA optimizer
- Task type performance characteristics
- Anomalies in execution time

### SLA Success Rate
Tracks SLA compliance percentage over time:
- Target: 95% (red dashed line)
- Shows optimizer effectiveness
- Identifies SLA violation patterns

### Worker Distribution
Bar and pie charts showing:
- Task count per worker
- Load balancing effectiveness
- Worker utilization

### Throughput
Task completion rate (tasks/min):
- Shows system capacity
- Identifies bottlenecks
- Tracks performance trends

---

## 🎯 Complete Test Scenarios

### Scenario 1: Single Run with Visualization
```bash
# Terminal 1: Start monitoring
bash test/monitor_metrics.sh

# Terminal 2: Submit tasks
bash test/batch_submit.sh

# After completion: Generate plots
bash test/visualize_results.sh
```

---

### Scenario 2: RTS vs Round-Robin Comparison
```bash
# Run automated comparison
bash test/compare_schedulers.sh

# Follow prompts to switch schedulers
# Results automatically compared
```

---

### Scenario 3: Multi-Run GA Training
```bash
# Run 3 cycles to collect GA training data
for i in {1..3}; do
  echo "=== Run $i ==="
  bash test/batch_submit.sh
  sleep 60  # Wait for tasks to complete
  
  # Check GA output
  bash test/verify/check_ga_output.sh
done

# Verify AffinityMatrix populated
cat config/ga_output.json | jq '.AffinityMatrix'
```

---

## 📈 Metrics Explained

### Task States
- **Pending**: Queued, waiting for assignment
- **Assigned**: Assigned to worker, not yet started
- **Running**: Currently executing on worker
- **Completed**: Finished successfully
- **Failed**: Execution failed

### SLA Metrics
- **SLA Met**: Task completed before deadline (arrival_time + k × tau)
- **SLA Violated**: Task completed after deadline
- **Success Rate**: (Met / Total) × 100%

### Tau (τ)
- Average execution time for each task type
- Updated by GA optimizer based on actual execution times
- Used for deadline calculation and risk scoring

### Throughput
- Tasks completed per minute
- Calculated over rolling 60-second window
- Indicates system capacity

---

## 🔧 Configuration

### Batch Submit (`batch_submit.sh`)
```bash
API=http://localhost:8080      # Master API endpoint
CONCURRENT=10                  # Parallel submissions
```

### Monitor Metrics (`monitor_metrics.sh`)
```bash
MONGO_URI=mongodb://localhost:27017  # MongoDB connection
DB_NAME=cloudai                      # Database name
API=http://localhost:8080            # Master API
INTERVAL=5                           # Poll interval (seconds)
DURATION=300                         # Total duration (seconds)
```

### Visualization (`visualize_results.py`)
```bash
# Requires: python3, matplotlib, numpy
pip3 install --user matplotlib numpy
```

---

## 🐛 Troubleshooting

### Issue: "No metrics files found"
**Solution:**
```bash
# Ensure monitoring ran successfully
ls -lh test/results/metrics/

# Manually specify file
bash test/visualize_results.sh test/results/metrics/metrics_TIMESTAMP.jsonl
```

---

### Issue: "Import matplotlib could not be resolved"
**Solution:**
```bash
# Install Python dependencies
pip3 install --user matplotlib numpy

# Verify installation
python3 -c "import matplotlib, numpy; print('OK')"
```

---

### Issue: Tasks stuck in "pending"
**Solution:**
```bash
# Check workers are active
mongo cloudai --eval "db.workers.find({is_active: true}).pretty()"

# Check master logs for errors
tail -f master/logs/master.log

# Verify queue processor running
curl http://localhost:8080/api/internal/state | jq '.queue'
```

---

### Issue: SLA success rate very low
**Solution:**
- Check if GA has sufficient training data (10+ completed tasks)
- Verify tau values are reasonable: `bash test/verify/check_ga_output.sh`
- Increase k_value (SLA multiplier) from 2.0 to 3.0
- Check worker resources match task requirements

---

## 📁 File Structure

```
test/
├── batch_submit.sh              # Batch task submission
├── monitor_metrics.sh           # Real-time monitoring
├── visualize_results.sh         # Wrapper for visualization
├── visualize_results.py         # Python visualization script
├── compare_schedulers.sh        # RTS vs Round-Robin comparison
├── results/
│   ├── batch_submission_*.json  # Submission results
│   ├── metrics/
│   │   ├── metrics_*.jsonl      # Time-series metrics
│   │   └── plots/               # Generated visualizations
│   └── comparison_*/            # Scheduler comparison results
└── verify/
    ├── check_task_distribution.sh
    ├── check_sla_violations.sh
    └── check_ga_output.sh
```

---

## 🎓 Best Practices

1. **Always monitor during testing**
   ```bash
   bash test/monitor_metrics.sh &
   bash test/batch_submit.sh
   ```

2. **Collect multiple runs for GA training**
   - GA needs 10+ completed tasks per task type
   - Run at least 3 full batches

3. **Compare schedulers with equal workloads**
   - Use `compare_schedulers.sh` for fair comparison
   - Clears database between runs

4. **Visualize after every test run**
   - Helps identify issues early
   - Tracks performance trends

5. **Check worker distribution**
   - Verify load balancing effectiveness
   - Identify overloaded workers

---

## 📊 Expected Results

### Healthy System
- **SLA Success Rate**: 95%+
- **Throughput**: Steady increase until saturation
- **Worker Distribution**: Balanced (within 20% variance)
- **Task States**: Pending → Running → Completed (minimal failures)
- **Tau Evolution**: Decreasing or stabilizing over time

### Issues to Watch For
- **SLA Rate < 80%**: GA not trained or workers overloaded
- **Uneven Distribution**: Scheduler not load balancing
- **High Failure Rate**: Resource constraints or Docker issues
- **Stagnant Throughput**: Worker connectivity issues

---

## 🚀 Next Steps

1. **Run baseline test:**
   ```bash
   bash test/batch_submit.sh
   bash test/monitor_metrics.sh
   bash test/visualize_results.sh
   ```

2. **Compare schedulers:**
   ```bash
   bash test/compare_schedulers.sh
   ```

3. **Verify GA convergence:**
   ```bash
   # Run 3 cycles
   for i in {1..3}; do
     bash test/batch_submit.sh
     sleep 120
   done
   bash test/verify/check_ga_output.sh
   ```

4. **Analyze results:**
   - Review generated plots
   - Check `summary_stats.json`
   - Compare RTS vs Round-Robin metrics

---

## 📞 Quick Commands Reference

```bash
# Submit 40 tasks
bash test/batch_submit.sh

# Monitor for 5 minutes
bash test/monitor_metrics.sh

# Visualize results
bash test/visualize_results.sh

# Compare schedulers
bash test/compare_schedulers.sh

# Check task distribution
bash test/verify/check_task_distribution.sh

# Check SLA violations
bash test/verify/check_sla_violations.sh

# Check GA output
bash test/verify/check_ga_output.sh

# Quick stats
mongo cloudai --eval "db.tasks.aggregate([{$group: {_id: '\$status', count: {\$sum: 1}}}]).pretty()"
```

---

## 🎬 Complete Testing Session Example

Here's a real-world example of running a complete testing session:

```bash
# ===== TERMINAL 1: Master Node =====
cd /media/udaan/New\ Volume/Ahmedabad\ University/7th\ Sem/Cloud/CloudAI/master
./master
# Wait for: Master node started on :50051
# Leave this running

# ===== TERMINAL 2: Worker (kiwi) =====
ssh kiwi  # SSH to worker machine
cd /media/udaan/New\ Volume/Ahmedabad\ University/7th\ Sem/Cloud/CloudAI/worker
./worker
# Wait for: Worker registered with master
# Leave this running

# ===== TERMINAL 3: Worker (NullPointer) =====
ssh NullPointer  # SSH to another worker
cd /media/udaan/New\ Volume/Ahmedabad\ University/7th\ Sem/Cloud/CloudAI/worker
./worker
# Leave this running

# ===== TERMINAL 4: Verify Setup =====
curl http://localhost:8080/api/workers | jq
# Should show 2 active workers

# ===== TERMINAL 5: Submit Tasks =====
cd /media/udaan/New\ Volume/Ahmedabad\ University/7th\ Sem/Cloud/CloudAI
bash test/batch_submit.sh

# ===== TERMINAL 6: Monitor (run in parallel) =====
cd /media/udaan/New\ Volume/Ahmedabad\ University/7th\ Sem/Cloud/CloudAI
bash test/monitor_metrics.sh

# ===== Wait for completion (~5-10 minutes) =====

# ===== TERMINAL 7: Visualize Results =====
bash test/visualize_results.sh

# ===== View Plots =====
cd test/results/metrics/plots
ls -lh
# tau_evolution.png
# throughput.png
# sla_success.png
# worker_distribution.png
# task_states.png
```

---

## 📋 Pre-Flight Checklist

Before running tests, ensure:

- [ ] MongoDB is running: `systemctl status mongodb`
- [ ] Master node is running: `curl http://localhost:8080/api/health`
- [ ] At least 2 workers are active: `curl http://localhost:8080/api/workers | jq '. | length'`
- [ ] Workers show `is_active: true`: `curl http://localhost:8080/api/workers | jq '.[].is_active'`
- [ ] No stale workers in database: `mongo cloudai --eval "db.workers.find({is_active: true}).count()"`
- [ ] Python dependencies installed: `python3 -c "import matplotlib, numpy"`

---

**Happy Testing! 🎉**

