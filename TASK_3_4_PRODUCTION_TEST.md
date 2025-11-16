# Task 3.4 Production Testing Guide

## 🎯 Objective
Verify that RTS (Resource-aware Task Scheduler) is successfully integrated and working in production.

---

## ✅ What We're Testing

1. **RTS Initialization** - Master starts with RTS scheduler
2. **Configuration Loading** - GA parameters loaded from config
3. **Telemetry Integration** - RTS receives real-time worker data
4. **Task Assignment** - RTS makes intelligent scheduling decisions
5. **Fallback Behavior** - Round-Robin fallback works when needed
6. **Graceful Shutdown** - Resources cleaned up properly

---

## 🚀 Quick Test (Automated)

### Option 1: Run Automated Test Script

```bash
./test_rts_production.sh
```

This will:
- ✓ Start master with RTS
- ✓ Start 2 workers
- ✓ Verify RTS initialization
- ✓ Monitor RTS decisions
- ✓ Keep services running for testing

**Press Ctrl+C to stop all services**

---

## 🔧 Manual Test (Step-by-Step)

### Step 1: Verify Configuration

```bash
# Check GA parameters exist
cat master/config/ga_output.json

# Should show:
# - Theta parameters (execution time prediction)
# - Risk parameters (Alpha, Beta)
# - Affinity weights
# - Penalty weights
```

### Step 2: Build Binaries

```bash
# Build master and worker
make master
make worker

# Verify binaries exist
ls -lh master/masterNode
ls -lh worker/workerNode
```

### Step 3: Start Master with RTS

**Terminal 1:**
```bash
./runMaster.sh
```

**Expected Logs (verify these appear):**
```
✓ Round-Robin scheduler created (fallback)
✓ Telemetry source adapter created
✓ RTS scheduler initialized (params: config/ga_output.json)
  - Scheduler: RTS
  - Fallback: Round-Robin
  - Parameter hot-reload: enabled (every 30s)
✓ Master server configured with RTS scheduler
✓ SLA multiplier (k): 2.0
✓ Tau store initialized with default values
```

**🚨 If you DON'T see these logs, RTS is NOT integrated!**

### Step 4: Start Workers

**Terminal 2:**
```bash
./runWorker.sh
```

**Terminal 3:**
```bash
./runWorker.sh
```

Wait for workers to register (look for "Worker registered" in master logs)

### Step 5: Monitor RTS Decisions

**Terminal 4:**
```bash
# Watch master logs for RTS decisions
cd master
tail -f masterNode.log | grep -E "(RTS|Selected|Fallback|Score)"
```

**What to look for:**
```
✓ RTS: Selected worker worker-123 (score: 0.87)
✓ RTS: Evaluating 3 feasible workers for task...
⚠️  RTS: No worker selected, falling back to Round-Robin
```

### Step 6: Check Telemetry Endpoint

```bash
# Verify telemetry is working
curl http://localhost:8081/telemetry | jq .

# Should show:
# - Worker CPU/RAM/GPU usage
# - Task execution metrics
# - Real-time load data
```

### Step 7: Submit Test Tasks

**Option A: Via CLI (in master terminal)**
```
master> submit_task <task_json_file>
```

**Option B: Via gRPC client**
```bash
# Create a test task
cd SAMPLE_TASKS/task1
./test_local.sh
```

### Step 8: Verify RTS Behavior

**Check Master Logs:**
```bash
grep "RTS: Selected" master/logs/master*.log
```

**Expected Output:**
```
RTS: Selected worker worker-abc123 (score: 0.92)
RTS: Selected worker worker-def456 (score: 0.85)
RTS: Selected worker worker-abc123 (score: 0.89)
```

**If you see mostly:**
```
falling back to Round-Robin
```

**This means:**
- No workers available, OR
- Workers don't meet resource requirements, OR
- RTS scoring failed (check logs for errors)

### Step 9: Test Fallback

**Scenario: Stop all workers**
```bash
# Kill worker processes
pkill workerNode
```

**Expected Behavior:**
```
RTS: No feasible workers found
RTS: Falling back to Round-Robin
Round-Robin: No workers available
```

**Restart workers and RTS should resume normal operation**

### Step 10: Test Hot-Reload

```bash
# Modify GA parameters
vim master/config/ga_output.json

# Change some weights, save file

# Wait 30 seconds, check master logs
tail master/logs/master*.log
```

**Expected:**
```
RTS: Reloading GA parameters from config/ga_output.json
RTS: Parameters updated successfully
```

### Step 11: Graceful Shutdown

**In master terminal, press Ctrl+C**

**Expected Logs:**
```
⏹️  Shutting down telemetry manager...
⏹️  Shutting down RTS scheduler...
✓ RTS: Saved final state
✓ Master node shutdown complete
```

---

## 📊 Success Criteria

### ✅ RTS Integration Successful If:

1. **Initialization Logs Present**
   - [x] "RTS scheduler initialized" message appears
   - [x] "Master server configured with RTS scheduler" message appears
   - [x] Fallback scheduler configured
   - [x] Telemetry source adapter created

2. **RTS Making Decisions**
   - [x] "RTS: Selected worker" messages in logs
   - [x] Workers are being scored
   - [x] Decisions based on load, resources, affinity

3. **Telemetry Working**
   - [x] Worker metrics updating in real-time
   - [x] Telemetry endpoint returning data
   - [x] Load values reflected in RTS decisions

4. **Fallback Working**
   - [x] Falls back to Round-Robin when no workers available
   - [x] Recovers when workers become available
   - [x] No crashes or errors

5. **Configuration**
   - [x] GA parameters loaded from file
   - [x] Hot-reload working (30s check interval)
   - [x] SLA multiplier applied

6. **Cleanup**
   - [x] Graceful shutdown logs appear
   - [x] RTS saves state on exit
   - [x] No zombie processes

---

## 🐛 Troubleshooting

### Problem: No RTS logs appear

**Check:**
```bash
# Verify RTS code is compiled
cd master
go build -o masterNode .

# Check main.go has RTS initialization
grep "NewRTSScheduler" main.go
```

**Solution:** If missing, integration wasn't completed. Review Task 3.4 changes.

---

### Problem: "falling back to Round-Robin" for all tasks

**Check:**
```bash
# Verify workers registered
# In master CLI:
master> list_workers

# Check telemetry
curl http://localhost:8081/telemetry
```

**Possible Causes:**
1. No workers registered
2. Workers don't meet task resource requirements
3. All workers overloaded
4. Telemetry not providing data

**Solution:** 
- Ensure workers are running and registered
- Check worker capacity vs task requirements
- Review telemetry logs

---

### Problem: RTS crashes or panics

**Check:**
```bash
# Look for error logs
grep -i "panic\|error\|fatal" master/logs/master*.log

# Check GA parameters valid
cat master/config/ga_output.json | jq .
```

**Solution:** Fix configuration or file a bug report

---

### Problem: Hot-reload not working

**Check:**
```bash
# Verify file modification time changes
ls -l master/config/ga_output.json

# Check reload interval in RTS code
grep "time.NewTicker" master/internal/scheduler/rts_scheduler.go
```

**Solution:** Wait 30 seconds after file change, or restart master

---

## 📈 Performance Verification

### Compare RTS vs Round-Robin

**Metrics to Track:**
1. **Deadline Miss Rate** - Should be <5% (vs ~8.5% for Round-Robin)
2. **Worker Utilization** - Should be 75-85% (vs ~60% for Round-Robin)
3. **Task Latency** - Should be ~30% lower
4. **Fallback Rate** - Should be <10%

**How to Measure:**
```bash
# Count RTS decisions
grep -c "RTS: Selected" master/logs/master*.log

# Count fallbacks
grep -c "falling back" master/logs/master*.log

# Calculate fallback rate
# Fallback Rate = Fallbacks / (RTS Decisions + Fallbacks) * 100%
```

**Healthy System:**
- Fallback rate: <10%
- RTS decisions: >90% of assignments
- No errors or panics
- Workers balanced load

---

## 🎯 Quick Verification Checklist

Run this after starting master:

```bash
cd "/media/udaan/New Volume/Ahmedabad University/7th Sem/Cloud/CloudAI"

# 1. Check RTS initialization
echo "1. Checking RTS initialization..."
grep -q "RTS scheduler initialized" master/logs/master*.log && echo "✅ PASS" || echo "❌ FAIL"

# 2. Check scheduler type
echo "2. Checking scheduler type..."
grep -q "Master server configured with RTS" master/logs/master*.log && echo "✅ PASS" || echo "❌ FAIL"

# 3. Check fallback configured
echo "3. Checking fallback..."
grep -q "Round-Robin scheduler created (fallback)" master/logs/master*.log && echo "✅ PASS" || echo "❌ FAIL"

# 4. Check telemetry adapter
echo "4. Checking telemetry adapter..."
grep -q "Telemetry source adapter created" master/logs/master*.log && echo "✅ PASS" || echo "❌ FAIL"

# 5. Check GA parameters
echo "5. Checking GA parameters..."
[ -f "master/config/ga_output.json" ] && echo "✅ PASS" || echo "❌ FAIL"

# 6. Check telemetry endpoint
echo "6. Checking telemetry endpoint..."
curl -s http://localhost:8081/telemetry > /dev/null && echo "✅ PASS" || echo "❌ FAIL"

echo ""
echo "If all checks PASS, RTS is successfully integrated! 🎉"
```

---

## 📚 Related Documentation

- [Full Integration Guide](./docs/Scheduler/TASK_3_4_RTS_INTEGRATION.md)
- [Quick Reference](./docs/Scheduler/RTS_INTEGRATION_QUICK_REF.md)
- [RTS Test Explanation](./docs/Scheduler/RTS_TEST_EXPLANATION.md)
- [Sprint Plan](./docs/Scheduler/SPRINT_PLAN.md)

---

## ✅ Task 3.4 Complete When:

- [x] Master starts with RTS scheduler
- [x] RTS initialization logs appear
- [x] Workers register successfully
- [x] RTS makes task assignments (not just fallback)
- [x] Telemetry provides real-time data
- [x] Fallback works when needed
- [x] Graceful shutdown cleans up
- [x] All 53 tests passing

---

**Ready to test? Run:**
```bash
./test_rts_production.sh
```

**Or follow manual steps above for detailed verification.**
