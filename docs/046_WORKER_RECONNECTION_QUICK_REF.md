# Worker Reconnection - Quick Reference

## What Was Fixed

**Problem**: Workers registered while offline never reconnected when they came back online.

**Solution**: Automatic reconnection monitor that checks every 30 seconds and reconnects inactive workers.

## Quick Start

### No Changes Required!

The reconnection feature is **automatic**. Just:

1. Start master
2. Register workers (even if offline)
3. Workers automatically connect when they come online

## Command Reference

### Register Worker (Even If Offline)

```bash
master> register <worker_id> <ip:port>
```

**Example:**
```bash
master> register worker-1 192.168.1.100:50052
✅ Worker worker-1 registered with address 192.168.1.100:50052
Failed to connect to worker worker-1... 
# ↑ This is OK! Worker will connect automatically when online
```

### Check Worker Status

```bash
master> workers
```

**Output:**
```
╔═══ Registered Workers 
║ worker-1
║   Status: 🔴 Inactive  ← Worker offline
║   ...
```

### Wait for Reconnection

- Maximum wait: **30 seconds**
- Check again: `master> workers`
- Worker will show 🟢 Active when connected

## Typical Workflows

### Workflow 1: Pre-register Workers

```bash
# 1. Start master
./masterNode

# 2. Register workers (they're offline, that's OK!)
master> register worker-1 node1:50052
master> register worker-2 node2:50052

# 3. Start workers (on their machines)
node1> ./workerNode
node2> ./workerNode

# 4. Workers connect automatically (within 30 seconds)
# Done! No manual steps needed.
```

### Workflow 2: Worker Crash Recovery

```bash
# Worker crashes or network fails
# Worker shows: 🔴 Inactive

# Restart worker
./workerNode

# Wait up to 30 seconds
# Worker automatically reconnects: 🟢 Active

# Resume work - no manual registration needed!
```

## Monitoring

### Master Logs

**Successful Reconnection:**
```
✓ Successfully reconnected to worker worker-1 (192.168.1.100:50052)
```

**Multiple Workers:**
```
🔄 Attempting to reconnect to 3 inactive worker(s)...
✓ Successfully reconnected to worker worker-1
✓ Successfully reconnected to worker worker-2
✓ Successfully reconnected to worker worker-3
```

### Worker Logs

**Worker Receives Connection:**
```
Master registration request from: master-1 (localhost:50051)
Registering worker worker-1 with master at localhost:50051
✓ Worker registered: Worker worker-1 registered successfully
```

## Key Points

| Feature | Details |
|---------|---------|
| **Check Interval** | Every 30 seconds |
| **Connection Timeout** | 3 seconds per worker |
| **Retry Behavior** | Infinite (until connected or unregistered) |
| **Manual Intervention** | None required |
| **Log Verbosity** | Only logs successful reconnections |
| **Performance Impact** | Minimal (background task) |

## Troubleshooting

### Worker Still Inactive After 2 Minutes

**Check:**
1. Worker is actually running: `ps aux | grep workerNode`
2. Worker IP is correct in registration
3. Network connectivity: `ping <worker_ip>`
4. Port is accessible: `nc -zv <worker_ip> <port>`
5. Firewall rules allow connection

### Worker Keeps Disconnecting

**Possible Causes:**
- Network instability
- Worker crashes (check worker logs)
- Resource constraints on worker machine

**Solution:**
- Worker will automatically reconnect each time
- Fix underlying issue for stability

### Master Logs Show Connection Errors

**If you see:**
```
Failed to connect to worker worker-1...
```

**This is normal if:**
- Worker is offline/starting up
- Network temporarily unavailable
- Monitor will retry automatically

**This is a problem if:**
- Persists for >5 minutes
- Worker claims to be running
- Check network/firewall

## Configuration

### Change Reconnection Interval

Edit `master/internal/server/master_server.go`:

```go
func (s *MasterServer) StartWorkerReconnectionMonitor() {
    // Change 30 to desired seconds
    s.reconnectTicker = time.NewTicker(30 * time.Second)
    // ...
}
```

### Change Connection Timeout

Edit `attemptSingleWorkerReconnection()`:

```go
// Change 3 to desired seconds
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
```

## Advanced Usage

### Force Immediate Reconnection Check

Currently not supported via CLI. 

**Workaround:**
1. Unregister worker: `master> unregister worker-1`
2. Re-register: `master> register worker-1 <ip:port>`
3. This triggers immediate connection attempt

### Disable Reconnection (if needed)

Edit `master/main.go`, comment out:

```go
// masterServer.StartWorkerReconnectionMonitor()
// log.Println("✓ Worker reconnection monitor started")
```

**Note:** Not recommended. Workers won't reconnect after failures.

## Related Commands

- `master> workers` - View all workers and their status
- `master> register <id> <ip>` - Register new worker
- `master> unregister <id>` - Remove worker
- `master> status` - System overview

## See Also

- [Full Documentation](046_WORKER_RECONNECTION.md)
- [Worker Registration Guide](037_WORKER_REGISTRATION.md)
- [Manual Registration](015_MANUAL_REGISTRATION_SUMMARY.md)
