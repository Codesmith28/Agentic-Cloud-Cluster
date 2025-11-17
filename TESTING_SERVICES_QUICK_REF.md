# 🚀 Testing Quick Reference Card

## 📍 When to Start Master & Workers

### **BEFORE ANY TESTING**

**1. Start Master (Terminal 1):**
```bash
cd /media/udaan/New\ Volume/Ahmedabad\ University/7th\ Sem/Cloud/CloudAI/master
./master
```
✅ Wait for: `Master node started on :50051`

**2. Start Workers (Separate machines/terminals):**
```bash
# On kiwi:
cd /path/to/CloudAI/worker && ./worker

# On NullPointer:
cd /path/to/CloudAI/worker && ./worker
```
✅ Wait for: `Worker registered with master`

**3. Verify:**
```bash
curl http://localhost:8080/api/workers | jq
```
✅ Should show 2-4 workers with `is_active: true`

---

## 🧪 Testing Commands (Run AFTER services are up)

### Quick Test (2 tasks)
```bash
bash test/smoke_test.sh
```

### Full Test (40 tasks)
```bash
# Terminal 2: Submit
bash test/batch_submit.sh

# Terminal 3: Monitor (parallel)
bash test/monitor_metrics.sh

# After completion: Visualize
bash test/visualize_results.sh
```

### Compare Schedulers
```bash
bash test/compare_schedulers.sh
# Follow prompts to switch schedulers
```

---

## 🔍 Health Checks

```bash
# Master running?
curl http://localhost:8080/api/health

# Workers active?
curl http://localhost:8080/api/workers | jq '. | length'

# MongoDB running?
mongo cloudai --eval "db.stats()"

# Task stats?
mongo cloudai --eval "db.tasks.aggregate([{\$group: {_id: '\$status', count: {\$sum: 1}}}]).pretty()"
```

---

## 📊 Results Location

```
test/results/
├── batch_submission_*.json      # Submission results
├── metrics/
│   ├── metrics_*.jsonl         # Time-series data
│   └── plots/                  # Visualizations
│       ├── tau_evolution.png
│       ├── throughput.png
│       ├── sla_success.png
│       ├── worker_distribution.png
│       └── task_states.png
└── comparison_*/               # Scheduler comparison
    ├── round_robin_results.json
    ├── rts_results.json
    └── comparison.json
```

---

## 🚨 Common Issues

| Issue | Solution |
|-------|----------|
| "Connection refused" | Start master: `./master` |
| "No workers found" | Start workers on separate machines |
| "Tasks stuck pending" | Check `master> workers` shows active workers |
| "Import matplotlib error" | `pip3 install --user matplotlib numpy` |
| "Database error" | Start MongoDB: `sudo systemctl start mongodb` |

---

## ⚡ Environment Variables

```bash
# Batch submission
API=http://localhost:8080
CONCURRENT=10

# Monitoring
DURATION=300    # seconds
INTERVAL=5      # poll interval

# Example
CONCURRENT=20 bash test/batch_submit.sh
DURATION=600 bash test/monitor_metrics.sh
```

---

## 📝 Typical Session Flow

```
1. Start MongoDB           (systemctl start mongodb)
2. Start Master            (./master in Terminal 1)
3. Start Workers           (./worker on each machine)
4. Verify connections      (curl api/workers)
5. Submit tasks            (bash test/batch_submit.sh)
6. Monitor metrics         (bash test/monitor_metrics.sh)
7. Generate visualizations (bash test/visualize_results.sh)
8. Review plots            (test/results/metrics/plots/)
```

---

## 🎯 Master CLI Commands (Use in master terminal)

```bash
master> workers              # List active workers
master> queue                # Show task queue
master> list-tasks           # List all tasks
master> stats                # System statistics
master> set-scheduler rts    # Switch to RTS
master> set-scheduler round-robin  # Switch to Round-Robin
```

---

**Remember:** Always start Master + Workers BEFORE running any test scripts! 🔥
