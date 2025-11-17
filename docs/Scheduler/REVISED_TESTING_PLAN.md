# **REVISED TESTING PLAN - CLI-Based Approach**

## **Overview**

Based on the production master node logs, we've revised the testing strategy from programmatic Go tests to **CLI-based manual testing** with automation scripts. This approach better aligns with how users will actually interact with the system.

---

## **Why CLI-Based Testing?**

### **Available Master CLI Commands** (from production):
```bash
# Task Management
task <docker_img> [-cpu_cores <num>] [-mem <gb>] [-gpu_cores <num>] [-k <1.5-2.5>] [-type <task_type>]
dispatch <worker_id> <docker_img> [options]
list-tasks [status]
monitor <task_id>
cancel <task_id>
queue

# Worker Management
workers
stats <worker_id>
internal-state
fix-resources
register <id> <ip:port>
unregister <id>

# System Status
status
help
```

### **Advantages**:
1. ✅ Tests actual production workflow
2. ✅ No complex gRPC client code needed
3. ✅ Easier debugging and visualization
4. ✅ Scripts can automate CLI commands
5. ✅ Validates end-to-end behavior
6. ✅ Users will use these exact commands

---

## **Milestone 5: Testing & Validation (4 days)**

### **Task 5.1: CLI Testing Procedures** (Day 1)
**Goal**: Create comprehensive testing guides and automation scripts

**Deliverables**:
1. **CLI Testing Guide** (`docs/Scheduler/TASK_5_1_CLI_TESTING_GUIDE.md`):
   - Step-by-step manual test procedures
   - Test scenarios for 6 task types
   - Validation criteria
   - Expected outputs

2. **Workload Scripts** (`test/workloads/`):
   ```bash
   submit_cpu_light.sh       # 10 cpu-light tasks
   submit_cpu_heavy.sh       # 10 cpu-heavy tasks
   submit_memory_heavy.sh    # 10 memory-heavy tasks
   submit_gpu_inference.sh   # 10 gpu-inference tasks
   submit_gpu_training.sh    # 10 gpu-training tasks
   submit_mixed.sh           # 10 mixed tasks
   submit_full_workload.sh   # 60 tasks total
   ```

3. **Verification Scripts** (`test/verify/`):
   ```bash
   check_task_distribution.sh  # Worker load balance
   check_sla_violations.sh     # MongoDB SLA queries
   check_tau_updates.sh        # Tau value updates
   check_ga_output.sh          # GA parameter structure
   check_rts_fallback.sh       # Fallback behavior
   ```

**Example Test**:
```bash
# Submit CPU-light task with explicit type
master> task moinvinchhi/cloudai-cpu-intensive:1 -cpu_cores 1 -mem 2 -type cpu-light

# Monitor execution
master> list-tasks running
master> monitor task-123

# Verify completion
master> list-tasks completed

# Check MongoDB for SLA success
mongo cloudai --eval "db.results.find({task_id: 'task-123'}, {sla_success:1})"
```

---

### **Task 5.2: Scheduler Comparison** (Day 2)
**Goal**: Compare RTS vs Round-Robin performance

**Test Procedure**:
1. **Baseline (Round-Robin)**:
   - Disable RTS temporarily (code change)
   - Submit 60 mixed tasks via CLI
   - Record: SLA violations, completion times, worker utilization

2. **Comparison (RTS)**:
   - Re-enable RTS scheduler
   - Submit same 60 tasks
   - Record same metrics

3. **Analysis**:
   - MongoDB queries for metrics
   - Generate comparison report
   - Script: `test/compare_schedulers.sh`

**Metrics**:
- SLA success rate per task type
- Average task completion time
- Worker utilization balance
- Task distribution patterns

---

### **Task 5.3: GA Convergence Verification** (Day 3, morning)
**Goal**: Verify GA learns from execution history

**Test Procedure**:
1. **Initial State**:
   ```bash
   cat master/config/ga_output.json
   # Empty AffinityMatrix, PenaltyVector
   ```

2. **Generate Training Data** (60+ tasks across 6 types):
   ```bash
   ./test/workloads/submit_full_workload.sh
   ```

3. **Wait for GA Epoch** (60 seconds):
   - Watch logs: `🧬 Starting AOD/GA epoch...`
   - Should retrieve 60+ task history records

4. **Verify GA Output**:
   ```bash
   cat master/config/ga_output.json | jq .
   # AffinityMatrix should have 6 task type keys
   # PenaltyVector should have worker IDs
   ```

5. **Verify Improvements**:
   - Submit 60 more tasks
   - Compare SLA success rates (should improve)

**Script**: `test/verify_ga_convergence.sh`

---

### **Task 5.4: End-to-End Integration** (Day 3, afternoon)
**Goal**: Validate complete RTS+GA workflow

**Test Scenarios**:
1. **Multi-Worker Setup**:
   ```bash
   # Terminal 1: Master
   ./runMaster.sh
   
   # Terminal 2-4: Workers
   ./runWorker.sh
   ```

2. **Task Type Inference** (without `-type` flag):
   ```bash
   master> task moinvinchhi/cloudai-cpu-intensive:1 -cpu_cores 1 -mem 2
   # Should infer: cpu-light
   ```

3. **Explicit Task Type** (with `-type` flag):
   ```bash
   master> task myapp:latest -cpu_cores 4 -mem 8 -type cpu-heavy
   # Should preserve: cpu-heavy
   ```

4. **SLA Tracking**:
   ```bash
   # Tight deadline (k=1.5)
   master> task sample:v1 -cpu_cores 2 -mem 4 -k 1.5 -type cpu-light
   
   # Loose deadline (k=2.5)
   master> task sample:v1 -cpu_cores 2 -mem 4 -k 2.5 -type cpu-light
   
   # Query MongoDB for results
   mongo cloudai --eval "db.results.find({}, {sla_success:1})"
   ```

5. **Load Balancing**:
   ```bash
   # Submit 20 tasks rapidly
   for i in {1..20}; do
       echo "task moinvinchhi/cloudai-cpu-intensive:$i -type cpu-light"
   done
   
   master> internal-state
   # Verify distribution across workers
   ```

6. **Fallback to Round-Robin**:
   ```bash
   # Infeasible task (100 CPU cores)
   master> task heavy:latest -cpu_cores 100 -mem 200
   # Should log: "⚠️ RTS: No feasible workers, falling back to Round-Robin"
   ```

---

### **Task 5.5: Performance & Stress Testing** (Day 4)
**Goal**: Validate system under load

**Test Scenarios**:

1. **High-Volume Submission** (100 tasks/minute):
   ```bash
   # Script: test/stress_test.sh
   for i in {1..100}; do
       echo "task moinvinchhi/cloudai-cpu-intensive:$((i % 12 + 1)) -cpu_cores $((i % 8 + 1)) -mem $((i % 16 + 2)) -type cpu-light" | sleep 0.6
   done
   ```

2. **Concurrent Worker Load** (3+ workers, 200 tasks):
   - Start 3+ workers
   - Submit 200 tasks
   - Monitor: `status` every 10s

3. **GA Training Under Load**:
   - Submit 100 tasks
   - Wait for GA epoch during active task submission
   - Verify: No blocking, epoch completes < 5s

4. **Long-Running Stability** (8 hours):
   - Master + 2 workers
   - 500 tasks over 8 hours (1 task/minute)
   - Monitor: Memory leaks, disconnections, queue growth

**Performance Targets**:
- Task submission latency: < 100ms
- RTS scheduling decision: < 10ms
- GA epoch duration: < 5s
- No memory leaks over 8 hours
- Queue drains correctly

---

## **Milestone 6: Documentation (2 days, parallel with M5)**

### **Task 6.1: CLI User Guide** (Day 1, morning)
**File**: `docs/Scheduler/CLI_USER_GUIDE.md`

**Content**:
- Complete command reference with examples
- Task type guide (6 types explained)
- SLA configuration (`-k` parameter)
- Common workflows

---

### **Task 6.2: Operator Guide** (Day 1, afternoon)
**File**: `docs/Scheduler/OPERATOR_GUIDE.md`

**Content**:
- System monitoring (log interpretation)
- GA parameter tuning
- Troubleshooting guide:
  - High SLA violations
  - Underutilized workers
  - GA not training
  - Tasks stuck in queue
- Performance optimization

---

### **Task 6.3: Configuration Reference** (Day 2, morning)
**File**: `docs/Scheduler/CONFIGURATION_REFERENCE.md`

**Content**:
- Environment variables
- `ga_output.json` schema
- Task type definitions
- Default values

---

### **Task 6.4: Testing Documentation** (Day 2, afternoon)
**File**: `docs/Scheduler/TESTING_GUIDE.md`

**Content**:
- Links to all M5 testing docs
- Quick test checklist
- Test script locations

---

## **Milestone 7: Production Readiness (3 days)**

### **Task 7.1: Enhanced CLI Monitoring** (Day 1)
**New Commands**:
```bash
master> rts-stats           # RTS scheduling statistics
master> ga-stats            # GA training statistics
master> task-type-stats     # Task distribution by type
master> sla-report [hours]  # SLA violation report
```

---

### **Task 7.2: Automated Health Checks** (Day 1)
**New Command**:
```bash
master> health-check        # Comprehensive system health
```

**Output**:
- Master node status
- Worker health (3/3 online)
- Task queue status
- RTS scheduler health
- GA training status
- Database connectivity
- SLA performance

---

### **Task 7.3-7.4: Logging & Alerting** (Day 2)
**Improvements**:
- Structured logging with context
- Debug mode (`LOG_LEVEL=debug`)
- Performance logs
- Alert system for critical conditions:
  - SLA < 70% (CRITICAL)
  - All workers offline (CRITICAL)
  - GA training failures (WARNING)

---

### **Task 7.5-7.6: Optimization & Deployment** (Day 3)
**Optimizations**:
- MongoDB indexes for fast queries
- Query timeouts
- Aggregation pipelines

**Deployment Scripts**:
- `deploy_production.sh` - Automated deployment
- `check_production_health.sh` - Health validation

---

## **Timeline Summary**

| Milestone | Duration | Can Parallel? |
|-----------|----------|---------------|
| M5: Testing | 4 days | No (sequential) |
| M6: Documentation | 2 days | Yes (with M5) |
| M7: Production | 3 days | No (after M5) |
| **Total** | **9 days** | **~2 weeks** |

---

## **Next Immediate Steps**

1. **Delete old test directory**:
   ```bash
   cd /media/udaan/.../CloudAI
   rm -rf test/generate_workload.go test/README.md test/QUICK_START.md test/build.sh
   ```

2. **Create new test structure**:
   ```bash
   mkdir -p test/workloads test/verify
   ```

3. **Start Task 5.1**:
   - Create `docs/Scheduler/TASK_5_1_CLI_TESTING_GUIDE.md`
   - Write workload submission scripts
   - Write verification scripts

4. **Parallel: Start Task 6.1**:
   - Create `docs/Scheduler/CLI_USER_GUIDE.md`
   - Document all CLI commands from master logs

---

## **Success Criteria**

✅ **M5**: All test scenarios pass with documented results  
✅ **M6**: Complete documentation published (4 guides)  
✅ **M7**: Production features deployed and validated  

**Overall**: System ready for production use with comprehensive testing, documentation, and monitoring.
