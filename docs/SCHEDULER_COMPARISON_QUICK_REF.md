# Scheduler Analysis - Quick Reference

## ✅ What Was Fixed

### 1. batch_submit.sh Script
- **K-values**: Now in range 1.5-2.5 (cpu-heavy=2.5, gpu-heavy=2.4, memory-heavy=2.3, mixed=2.0, cpu-light=1.5)
- **Total tasks**: 35 tasks (includes 7 GPU tasks)
- **Task breakdown**: 5 cpu-intensive, 5 cpu-light, 5 cpu-heavy, 8 memory-heavy, 5 mixed, 7 gpu-heavy
- **Better output**: Shows resource details for each task
- **Proper tags**: Tags will display correctly in UI

### 2. Created Analysis Tools
- `docs/SCHEDULER_ANALYSIS_GUIDE.md` - Complete analysis methodology
- `test/analyze_scheduler.sh` - Automated analysis script

---

## 🎯 How to Compare RTS vs Round-Robin

### Step 1: Test Round-Robin Scheduler

```bash
# 1. Start master and workers
./runMaster.sh
./runWorker.sh  # On each worker machine

# 2. In master CLI, set scheduler
master> scheduler set round-robin

# 3. Clear previous data (in MongoDB)
mongosh cloudai
> db.TASK_ASSIGNMENTS.deleteMany({})
> db.TASK_RESULTS.deleteMany({})
> db.TASK_HISTORY.deleteMany({})
> exit

# 4. Submit tasks
bash test/batch_submit.sh

# 5. Wait for all tasks to complete (~20-30 minutes)

# 6. Run analysis
bash test/analyze_scheduler.sh > results_roundrobin.txt

# 7. Export data
mongoexport --db=cloudai --collection=TASK_RESULTS --out=results_rr.json
mongoexport --db=cloudai --collection=TASK_HISTORY --out=history_rr.json
```

### Step 2: Test RTS Scheduler

```bash
# 1. Train RTS with AOD
master> aod train

# 2. Set scheduler to RTS
master> scheduler set rts

# 3. Clear previous data
mongosh cloudai
> db.TASK_ASSIGNMENTS.deleteMany({})
> db.TASK_RESULTS.deleteMany({})
> db.TASK_HISTORY.deleteMany({})
> exit

# 4. Submit same tasks
bash test/batch_submit.sh

# 5. Wait for all tasks to complete

# 6. Run analysis
bash test/analyze_scheduler.sh > results_rts.txt

# 7. Export data
mongoexport --db=cloudai --collection=TASK_RESULTS --out=results_rts.json
mongoexport --db=cloudai --collection=TASK_HISTORY --out=history_rts.json
```

### Step 3: Compare Results

```bash
# Side-by-side comparison
diff results_roundrobin.txt results_rts.txt

# Or manually compare:
cat results_roundrobin.txt
cat results_rts.txt
```

---

## 📊 Key Metrics to Compare

| Metric | What to Look For | Better Value |
|--------|------------------|--------------|
| **Task Distribution** | Even spread across workers | More even = better |
| **SLA Success Rate** | % of tasks meeting SLA | Higher = better |
| **Avg Execution Time** | Time to complete tasks | Lower = better |
| **Queue Wait Time** | Time spent waiting | Lower = better |
| **Task Affinity** | Right tasks on right workers | RTS should excel |
| **Failure Rate** | % of failed tasks | Lower = better |

---

## 🔍 What Data to Collect

### Required MongoDB Collections:

1. **TASK_ASSIGNMENTS** - Shows which worker got which task
2. **TASK_RESULTS** - Shows success/failure and execution times
3. **TASK_HISTORY** - Shows complete task lifecycle
4. **WORKER_REGISTRY** - Shows worker capacities

### Required Master Logs:

```bash
# Save master logs
./runMaster.sh 2>&1 | tee master_roundrobin.log
# Then later:
./runMaster.sh 2>&1 | tee master_rts.log
```

**Look for:**
- `"RTS: Selected worker"` or `"Round-robin selected"`
- `"No suitable worker available"`
- `"Task.*assigned to"`
- `"Worker.*marked as inactive"`

---

## 🚨 Known Issues to Check

### 1. "NullPointer" Worker

**Check:**
```bash
mongosh cloudai --eval 'db.TASK_ASSIGNMENTS.countDocuments({worker_id: "NullPointer"})'
```

**If count > 0:** Scheduler is returning null/empty worker ID

**Fix:** Add validation in `selectWorkerForTask()` to never return empty string

### 2. Docker Image Pull Failures

**Check:**
```bash
mongosh cloudai --eval 'db.TASK_RESULTS.find({logs: /pull access denied/}).count()'
```

**If count > 0:** Docker images don't exist or are private

**Verify all images exist:**
```bash
# CPU tasks
docker pull moinvinchhi/cloudai-cpu-light:1
docker pull moinvinchhi/cloudai-cpu-heavy:1
docker pull moinvinchhi/cloudai-cpu-intensive:1

# Memory tasks
docker pull moinvinchhi/cloudai-memory-heavy:1
docker pull moinvinchhi/cloudai-io-intensive:1

# Mixed tasks
docker pull moinvinchhi/cloudai-mixed:1

# GPU tasks (NEW)
docker pull moinvinchhi/cloudai-gpu-intensive:1
docker pull moinvinchhi/cloudai-gpu-heavy:1
```

**Fix:** 
- Ensure images are public on Docker Hub
- Or add Docker authentication to workers
- **For GPU tasks:** Ensure workers have GPU capability (check with `nvidia-smi`)

### 3. GPU Worker Availability

**Check if workers have GPU:**
```bash
# On each worker machine:
nvidia-smi
# Should show GPU info

# OR check in master CLI:
master> workers
# Look for gpu_available > 0
```

**If NO GPU workers available:**
- GPU tasks will remain queued indefinitely
- Or will fail with "no suitable worker" error

**Options:**
1. **Skip GPU tasks** - Remove GPU tasks from batch_submit.sh
2. **Register GPU-enabled worker** - Add a worker with NVIDIA GPU
3. **Test GPU tasks separately** - Run GPU tasks when GPU worker is available

### 4. All Tasks to One Worker

**Check:**
```bash
bash test/analyze_scheduler.sh
# Look at "Task Distribution" section
```

**If one worker has >80%:** Load balancing is broken

**Possible causes:**
- Other workers are inactive
- Scheduler logic issue
- Resource tracking not updating

### 5. Wrong Task Types on Wrong Workers

**Check:**
```bash
mongosh cloudai --eval '
  db.TASK_HISTORY.aggregate([
    { $match: { task_type: "cpu-heavy" } },
    { $group: { _id: "$worker_id", count: { $sum: 1 } } }
  ])
'

# Check GPU task distribution
mongosh cloudai --eval '
  db.TASK_HISTORY.aggregate([
    { $match: { tag: "gpu-heavy" } },
    { $group: { _id: "$worker_id", count: { $sum: 1 } } }
  ])
'
```

**Expected:** 
- CPU-heavy tasks should go to workers with more CPU
- GPU-heavy tasks should ONLY go to workers with GPU available

**If not:** RTS affinity not working correctly

---

## 📈 Expected Results

### Good Scheduler Behavior:

✅ **Task Distribution:** 40-60% per worker (for 2 workers)  
✅ **SLA Success:** >90%  
✅ **Failure Rate:** <10%  
✅ **Avg Wait Time:** <10 seconds  
✅ **Task Affinity:** 
   - CPU-heavy → high-CPU workers
   - GPU-heavy → GPU-enabled workers only
   - Memory-heavy → high-memory workers  
✅ **No NullPointer:** 0 assignments

### Task Type Distribution (35 tasks total):
- 5 cpu-intensive (cpu-heavy tag)
- 5 cpu-light (cpu-light tag)
- 5 cpu-heavy (cpu-heavy tag)
- 8 memory-heavy (memory-heavy tag)
- 5 mixed (mixed tag)
- 7 gpu-heavy (gpu-heavy tag) - **Requires GPU worker!**

### RTS Should Excel At:

1. **Task Affinity** - Better matching of task types to workers
2. **Load Prediction** - Using historical data to estimate execution time
3. **Risk Minimization** - Avoiding overload situations

### Round-Robin Should Excel At:

1. **Simplicity** - Predictable behavior
2. **Even Distribution** - Fair task allocation
3. **Low Overhead** - Fast scheduling decisions

---

## 🎯 Decision Criteria

**Use Round-Robin if:**
- All workers are identical
- Tasks are similar in resource requirements
- Predictability is more important than optimization

**Use RTS if:**
- Workers have different capacities
- Tasks vary significantly in resource needs
- You have enough historical data (>10 completed tasks)
- Optimal resource utilization is critical

---

## 📝 Action Items

### Immediate (Before Testing):

1. ✅ Fix batch_submit.sh (DONE - includes GPU tasks)
2. ⚠️ Fix "NullPointer" issue in scheduler
3. ⚠️ Verify Docker images exist and are public (including GPU images)
4. ⚠️ Ensure workers are registered and active
5. ⚠️ **CRITICAL:** Verify at least ONE worker has GPU capability (run `nvidia-smi`)
   - If NO GPU workers: GPU tasks will fail or remain queued
   - Consider removing GPU tasks from batch_submit.sh if no GPU available

### For Testing:

1. Run Round-Robin test (save results)
2. Run RTS test (save results)
3. Compare results using analyze_scheduler.sh
4. Document findings

### For Improvement:

1. Add better logging in scheduler
2. Add validation to prevent null worker assignments
3. Improve resource tracking accuracy
4. Add real-time monitoring dashboard

---

## 🔧 Quick Commands

```bash
# Check current scheduler
master> scheduler info

# Set scheduler
master> scheduler set round-robin
master> scheduler set rts

# Train RTS
master> aod train

# Check workers
master> workers

# Check queue
master> queue

# Run analysis
bash test/analyze_scheduler.sh

# Submit tasks
bash test/batch_submit.sh

# Clear MongoDB data
mongosh cloudai
> db.TASK_ASSIGNMENTS.deleteMany({})
> db.TASK_RESULTS.deleteMany({})
> db.TASK_HISTORY.deleteMany({})
```

---

## 📞 Next Steps

1. **Run the tests** with both schedulers
2. **Collect the data** from MongoDB and logs
3. **Run analysis script** to get metrics
4. **Compare results** side-by-side
5. **Document findings** with evidence
6. **Make decision** based on metrics

Good luck! 🚀
