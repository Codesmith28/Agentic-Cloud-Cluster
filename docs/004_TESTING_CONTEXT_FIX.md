# Quick Testing Guide - Context and Persistence Fixes

## Setup

### 1. Start MongoDB (if not running)
```bash
cd /home/codesmith28/Projects/CloudAI/database
docker-compose up -d
```

### 2. Rebuild and Start Master
```bash
cd /home/codesmith28/Projects/CloudAI/master
go build -o masterNode .
./masterNode
```

### 3. Rebuild and Start Worker (in another terminal)
```bash
cd /home/codesmith28/Projects/CloudAI/worker
go build -o workerNode .
./workerNode
```

## Test Sequence

### Test 1: Worker Registration
**In Master CLI:**
```
> workers
Should show: No workers registered

> register worker-1 localhost:50052
Should show: ✓ Worker worker-1 registered

> workers
Should show: worker-1 listed with status
```

### Test 2: Task Execution (Context Fix)
**In Master CLI:**
```
> task Tessa hello-world:latest
Note the task ID (e.g., task-1762629077)
```

**Expected Worker Logs:**
```
Received task: task-1762629077
Pulling image: hello-world:latest
Starting container with resource limits...
Container output:
Hello from Docker!
This message shows that your installation appears to be working correctly...
Container completed successfully
```

**What to verify:**
- ✅ NO "context canceled" errors
- ✅ Full container output visible
- ✅ Task completes successfully
- ✅ Worker shows "Completed successfully"

**What should NOT happen:**
- ❌ Task completing immediately
- ❌ "Failed to report task result: context canceled"
- ❌ Incomplete execution

### Test 3: Live Monitoring
**While task is running (immediately after assignment):**
```
> monitor task-1762629077 Tessa
```

**Expected:**
```
═══════════════════════════════════════════════════════
Task ID: task-1762629077
User ID: Tessa
───────────────────────────────────────────────────────
Press any key to exit

[real-time container output appears here]

═══════════════════════════════════════════════════════
  Task Completed
═══════════════════════════════════════════════════════
```

### Test 4: Historical Log Retrieval (Persistence Fix)
**After task completes, monitor again:**
```
> monitor task-1762629077 Tessa
```

**Expected:**
```
═══════════════════════════════════════════════════════
Task ID: task-1762629077
User ID: Tessa
───────────────────────────────────────────────────────
Press any key to exit

[complete stored logs appear immediately]

═══════════════════════════════════════════════════════
  Task Completed
═══════════════════════════════════════════════════════
```

**What to verify:**
- ✅ Logs retrieved instantly (not streaming)
- ✅ Complete output shown
- ✅ No "Task not found" error

### Test 5: Database Verification
**Check MongoDB for stored results:**
```bash
mongosh
use cloudai
db.RESULTS.find().pretty()
```

**Expected output:**
```javascript
{
  _id: ObjectId("..."),
  task_id: "task-1762629077",
  worker_id: "worker-1",
  status: "success",
  logs: "Hello from Docker!\nThis message shows...",
  completed_at: ISODate("2024-...")
}
```

**Verify:**
- ✅ Result exists with correct task_id
- ✅ Logs field contains full output
- ✅ Status is "success"
- ✅ completed_at timestamp present

### Test 6: Multiple Tasks
**Assign several tasks to verify consistent behavior:**
```
> task Tessa alpine:latest
> task Tessa nginx:alpine
> task Tessa ubuntu:latest
```

**Monitor each:**
```
> monitor task-XXXXX Tessa
```

**Verify:**
- ✅ All tasks execute completely
- ✅ All logs stored in database
- ✅ Can monitor any task after completion

## Troubleshooting

### Issue: "Worker not registered"
**Solution:**
```
> register worker-1 localhost:50052
```

### Issue: "Task not found"
**Possible causes:**
1. Task ID incorrect - check `tasks` command
2. Database not running - check `docker ps`
3. ResultDB not initialized - check master logs for "✓ ResultDB initialized"

### Issue: Still seeing "context canceled"
**Solution:**
1. Verify worker was rebuilt: `cd worker && go build -o workerNode .`
2. Restart worker process
3. Check worker logs for executeTask signature (should NOT have ctx parameter)

### Issue: Logs not persisting
**Checks:**
1. Master logs show "✓ ResultDB initialized"
2. MongoDB is running: `docker ps | grep mongo`
3. ReportTaskCompletion is being called (check master logs)
4. Check for database errors in master logs

## Success Criteria

All tests pass if:
1. ✅ Tasks execute to completion without "context canceled" errors
2. ✅ Container output visible in worker logs
3. ✅ Live monitoring shows real-time logs
4. ✅ Historical monitoring retrieves stored logs
5. ✅ Database contains task results with logs
6. ✅ No "Task not found" errors for completed tasks

## Performance Notes

- **Live streaming**: Logs appear as container generates them
- **Historical retrieval**: Logs appear instantly from database
- **Database query**: < 10ms for result retrieval
- **Worker reporting**: 10s timeout for result submission

## Cleanup

```bash
# Stop master (Ctrl+C in master terminal)
# Stop worker (Ctrl+C in worker terminal)

# Clear test data (optional)
mongosh
use cloudai
db.RESULTS.deleteMany({})
db.TASKS.deleteMany({})
db.ASSIGNMENTS.deleteMany({})
```
