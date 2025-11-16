# Task Cancellation - Quick Reference

## Cancel a Task

```bash
master> cancel <task_id>
```

**Example:**
```bash
master> cancel task-1234567890
```

---

## What Happens When You Cancel

1. ✅ Master finds which worker has the task
2. ✅ Master sends cancellation request to worker
3. ✅ Worker stops Docker container (gracefully, then forcefully if needed)
4. ✅ Worker removes container
5. ✅ Task status updated to "cancelled" in database
6. ✅ Worker reports cancellation to master

---

## Common Commands

### Check if task is running
```bash
master> workers
# Look for task in "Running Tasks" section

master> stats <worker_id>
# Shows running tasks on specific worker
```

### Monitor task before cancelling
```bash
master> monitor <task_id>
# See live logs, press any key to exit
```

### Cancel task
```bash
master> cancel <task_id>
# Stops the task immediately
```

---

## Error Messages

| Message | Meaning | Solution |
|---------|---------|----------|
| "Task not found..." | Task doesn't exist or already completed | Verify task ID |
| "Failed to connect to worker" | Worker is offline | Check worker status with `workers` |
| "Worker not found" | Worker has been unregistered | Check available workers |

---

## Database

**Status Values:**
- `pending` - Not started yet
- `running` - Currently executing
- `completed` - Finished successfully
- `failed` - Execution failed
- `cancelled` - Stopped by user ✨

**Check in MongoDB:**
```javascript
db.TASKS.find({task_id: "task-1234567890"})
db.RESULTS.find({task_id: "task-1234567890"})
```

---

## Tips

💡 **Cancel soon after assignment:** If you realize a task was misconfigured, cancel immediately

💡 **Check worker status first:** Use `workers` command to verify worker is active

💡 **Monitor for cleanup:** Use `stats <worker_id>` to confirm task was removed

💡 **Check logs:** Cancelled tasks still save logs to database

---

## Full Documentation

See [TASK_CANCELLATION.md](TASK_CANCELLATION.md) for complete documentation.
