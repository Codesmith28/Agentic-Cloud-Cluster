# Task Assignment Troubleshooting Guide

## Common Error: "Failed to connect to worker: received empty target in Build()"

### Problem
When sending a task to a worker, you get:
```
❌ Failed to assign task: task assignment failed: Failed to connect to worker: 
failed to exit idle mode: passthrough: received empty target in Build()
```

### Root Cause
The worker's IP address is empty in the master's worker registry. This happens when:
1. Worker registration overwrites the manually configured IP address
2. Worker sends empty IP during auto-registration

### Solution (Fixed in latest code)
The master now preserves the manually configured IP address when workers auto-register.

### How to Verify

1. **Check worker details in master CLI:**
   ```
   master> workers
   ```
   
   Look for the worker's IP field:
   ```
   ║ Tessa
   ║   Status: 🟢 Active
   ║   IP: 192.168.1.100:50052    ← Should NOT be empty
   ║   Resources: CPU=4.0, Memory=8.0GB, GPU=0.0
   ║   Running Tasks: 0
   ```

2. **If IP is empty or incorrect:**
   ```
   master> unregister Tessa
   master> register Tessa 192.168.1.100:50052
   ```
   Wait a few seconds for worker to reconnect, then check again:
   ```
   master> workers
   ```

### Step-by-Step Fix

#### Step 1: Rebuild Master (if you just updated the code)
```bash
cd master
go build -o masterNode
```

#### Step 2: Restart Master
```bash
./runMaster.sh
```

#### Step 3: Restart Worker
```bash
./runWorker.sh
```

#### Step 4: Register Worker
In master CLI:
```
master> register <worker-id> <worker-ip:port>
```

Example:
```
master> register Tessa 192.168.1.100:50052
```

#### Step 5: Verify Registration
```
master> workers
```

Check that:
- ✅ Worker status is "🟢 Active"
- ✅ IP field shows the correct address (not empty)
- ✅ Resources are displayed

#### Step 6: Test Task Assignment
```
master> task Tessa docker.io/library/hello-world:latest
```

### Expected Success Output

**Master CLI:**
```
═══════════════════════════════════════════════════════
  📤 SENDING TASK TO WORKER
═══════════════════════════════════════════════════════
  Task ID:           task-1762403831
  Target Worker:     Tessa
  Docker Image:      docker.io/library/hello-world:latest
  Command:           docker run --rm --cpus=1.0 --memory=0.5g docker.io/library/hello-world:latest
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════

✅ Task task-1762403831 assigned successfully!
```

**Master Server Logs:**
```
2024/11/06 10:30:45 Connecting to worker Tessa at 192.168.1.100:50052

═══════════════════════════════════════════════════════
  📤 TASK ASSIGNED TO WORKER
═══════════════════════════════════════════════════════
  Task ID:           task-1762403831
  Target Worker:     Tessa
  Docker Image:      docker.io/library/hello-world:latest
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════
```

### Other Common Issues

#### "Worker not found"
**Problem:** Worker ID doesn't match
**Solution:** Check exact worker ID (case-sensitive):
```
master> workers  # List all workers with their IDs
```

#### "Worker is not active"
**Problem:** Worker hasn't sent heartbeats recently
**Solution:** 
- Check worker is running
- Wait 5-10 seconds after registration
- Restart worker if needed

#### "Worker has no IP address configured"
**Problem:** IP field is empty
**Solution:** Follow steps above to unregister and re-register

#### Connection timeout
**Problem:** Network connectivity issues
**Solution:**
- Verify worker is reachable: `ping <worker-ip>`
- Check firewall rules
- Ensure worker port is open
- Verify IP:port format is correct

### Debug Commands

Check worker details:
```
master> workers
```

Check worker stats (live view):
```
master> stats Tessa
```

View all commands:
```
master> help
```

### Prevention

1. **Always use the correct format** when manually registering:
   ```
   master> register <worker-id> <ip>:<port>
   ```

2. **Wait for confirmation** after registration:
   ```
   ✅ Worker Tessa registered with address 192.168.1.100:50052
   ```

3. **Verify before sending tasks:**
   ```
   master> workers  # Check IP is not empty
   ```

4. **Keep master and worker running** - don't restart one without the other unless necessary

### What the Fix Does

The updated code in `master/internal/server/master_server.go`:
```go
// Worker IS pre-registered - update with full specs but preserve the IP from manual registration
preservedIP := existingWorker.Info.WorkerIp
existingWorker.Info = info

// If worker didn't provide IP or provided empty IP, use the one from manual registration
if existingWorker.Info.WorkerIp == "" {
    existingWorker.Info.WorkerIp = preservedIP
    log.Printf("✓ Worker %s registered - using pre-configured address: %s", info.WorkerId, preservedIP)
}
```

This ensures the manually configured IP address is preserved even when the worker sends an empty IP during auto-registration.
