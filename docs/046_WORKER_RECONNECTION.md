# Worker Reconnection Feature

## Problem

Previously, when a worker was registered on the master but was offline at the time of registration, the master would attempt to connect once and then give up. If the worker came back online later, the master would never reconnect to it because:

1. The initial connection attempt failed
2. There was no retry mechanism
3. The worker didn't know the master's address to initiate connection
4. Workers remained in an "inactive" state indefinitely

## Solution

Implemented a **periodic worker reconnection monitor** that:
- Runs in the background on the master node
- Checks for inactive workers every 30 seconds
- Attempts to reconnect to offline workers
- Automatically registers workers when they come back online

## How It Works

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│              Master Node                                │
│                                                         │
│  ┌──────────────────────────────────────────┐          │
│  │  Worker Reconnection Monitor             │          │
│  │  (runs every 30 seconds)                 │          │
│  │                                           │          │
│  │  1. Check for inactive workers           │          │
│  │  2. Get worker IPs from registry         │          │
│  │  3. Attempt MasterRegister RPC           │          │
│  │  4. Worker responds and connects         │          │
│  └──────────────────────────────────────────┘          │
│                                                         │
│  Workers Map:                                           │
│  ┌────────────────────────────────────────┐            │
│  │ worker-1: IsActive=true  ✓             │            │
│  │ worker-2: IsActive=false ⏳ (retrying)  │            │
│  │ worker-3: IsActive=true  ✓             │            │
│  └────────────────────────────────────────┘            │
└─────────────────────────────────────────────────────────┘
                      │
                      │ MasterRegister RPC
                      ↓
         ┌────────────────────────┐
         │   Worker Node          │
         │   (comes back online)  │
         │                        │
         │   1. Receives          │
         │      MasterRegister    │
         │   2. Stores master     │
         │      address           │
         │   3. Calls             │
         │      RegisterWorker    │
         │   4. Starts heartbeats │
         └────────────────────────┘
```

### Connection Flow

#### Scenario: Worker is Offline During Registration

**Step 1: Admin registers worker (worker is offline)**
```bash
master> register worker-1 192.168.1.100:50052
✅ Worker worker-1 registered with address 192.168.1.100:50052
Failed to connect to worker worker-1 (192.168.1.100:50052) for MasterRegister: context deadline exceeded
```

**Step 2: Master continues to run**
- Worker remains in registry with `IsActive=false`
- Reconnection monitor checks every 30 seconds

**Step 3: Worker comes back online**
```bash
# Worker starts up
./workerNode
Worker listening on :50052...
```

**Step 4: Reconnection monitor detects and connects**
```
[Master logs]
🔄 Attempting to reconnect to 1 inactive worker(s)...
✓ Successfully reconnected to worker worker-1 (192.168.1.100:50052)
✓ Worker worker-1 registered - CPU=8.0, Memory=16.0GB, Storage=500.0GB, GPU=0.0

[Worker logs]
Master registration request from: master-1 (localhost:50051)
Registering worker worker-1 with master at localhost:50051
✓ Worker registered: Worker worker-1 registered successfully
```

**Step 5: Worker is now active and receiving tasks**
```bash
master> workers
╔═══ Registered Workers 
║ worker-1
║   Status: 🟢 Active
║   IP: 192.168.1.100:50052
║   Resources: CPU=8.0, Memory=16.0GB, GPU=0.0
║   Running Tasks: 0
╚═══════════════════════
```

### Implementation Details

#### New Methods

1. **`StartWorkerReconnectionMonitor()`**
   - Starts background goroutine with ticker (30-second interval)
   - Continuously monitors for inactive workers
   - Non-blocking and graceful shutdown support

2. **`StopWorkerReconnectionMonitor()`**
   - Gracefully stops the reconnection monitor
   - Called during master shutdown

3. **`attemptWorkerReconnections()`**
   - Identifies all inactive workers
   - Spawns goroutines to reconnect in parallel
   - Only logs successful reconnections (avoids log spam)

4. **`attemptSingleWorkerReconnection(workerID, workerIP, masterID, masterAddress)`**
   - Attempts connection to single worker
   - 3-second timeout per attempt
   - Silently fails if worker still offline
   - Logs success when worker connects

#### Configuration

- **Check Interval**: 30 seconds (configurable via `time.NewTicker`)
- **Connection Timeout**: 3 seconds per worker
- **Parallel Reconnections**: Yes (each worker in separate goroutine)
- **Retry Forever**: Yes, until worker connects or is unregistered

## Usage Examples

### Example 1: Pre-register Workers Before They Start

```bash
# Start master
./masterNode

# Register workers that will start later
master> register worker-1 node1.local:50052
master> register worker-2 node2.local:50052
master> register worker-3 node3.local:50052

# Workers will be inactive
master> workers
║ worker-1: 🔴 Inactive
║ worker-2: 🔴 Inactive
║ worker-3: 🔴 Inactive

# Start workers on their respective machines
# They will automatically connect within 30 seconds
```

### Example 2: Worker Goes Offline and Comes Back

```bash
# Worker is active and running tasks
master> workers
║ worker-1: 🟢 Active

# Worker crashes or network interruption occurs
# Worker status becomes inactive

master> workers
║ worker-1: 🔴 Inactive

# Worker restarts
./workerNode

# Within 30 seconds, reconnection monitor detects and reconnects
# [Master logs]
✓ Successfully reconnected to worker worker-1

master> workers
║ worker-1: 🟢 Active
```

### Example 3: Multiple Workers Coming Online

```bash
# Multiple workers registered but offline
master> workers
║ worker-1: 🔴 Inactive
║ worker-2: 🔴 Inactive
║ worker-3: 🔴 Inactive

# Start all workers simultaneously
# node1> ./workerNode
# node2> ./workerNode
# node3> ./workerNode

# Master reconnects to all within 30 seconds
[Master logs]
🔄 Attempting to reconnect to 3 inactive worker(s)...
✓ Successfully reconnected to worker worker-1 (node1.local:50052)
✓ Successfully reconnected to worker worker-2 (node2.local:50052)
✓ Successfully reconnected to worker worker-3 (node3.local:50052)
```

## Benefits

1. **Resilience**: System automatically recovers from worker failures
2. **Flexibility**: Workers can be registered before they're online
3. **Zero Manual Intervention**: No need to re-register workers after failures
4. **Clean Logs**: Only logs successful reconnections, not every attempt
5. **Resource Efficient**: Uses short timeouts and parallel connections
6. **Graceful**: Properly stops during master shutdown

## Technical Notes

### Thread Safety
- All worker state access is protected by `s.mu` (RWMutex)
- Read lock for identifying inactive workers
- Write operations happen in RPC handlers (already protected)

### Performance
- Minimal overhead: 30-second intervals
- 3-second timeout per connection attempt
- Parallel goroutines for multiple workers
- Non-blocking operation (doesn't affect main thread)

### Edge Cases Handled
1. **Worker IP not set**: Skipped (no connection possible)
2. **Worker still offline**: Silently skipped, will retry in 30 seconds
3. **Worker accepts connection but fails RPC**: Logged and retried later
4. **Master shutdown**: Monitor stops gracefully
5. **Multiple reconnection attempts**: Each attempt is independent

## Files Modified

### `master/internal/server/master_server.go`

**Added Fields:**
```go
type MasterServer struct {
    // ... existing fields ...
    
    // Worker reconnection
    reconnectTicker *time.Ticker
    reconnectStop   chan bool
}
```

**Added Methods:**
- `StartWorkerReconnectionMonitor()`
- `StopWorkerReconnectionMonitor()`
- `attemptWorkerReconnections()`
- `attemptSingleWorkerReconnection(workerID, workerIP, masterID, masterAddress)`

### `master/main.go`

**Added:**
```go
// After LoadWorkersFromDB
masterServer.StartWorkerReconnectionMonitor()
log.Println("✓ Worker reconnection monitor started")

// In shutdown handler
masterServer.StopWorkerReconnectionMonitor()
```

## Testing

### Test 1: Register Offline Worker

1. Start master
2. Register worker: `master> register worker-1 localhost:50052`
3. Verify worker is inactive: `master> workers`
4. Start worker: `./workerNode`
5. Wait up to 30 seconds
6. Verify worker becomes active: `master> workers`

**Expected Result**: Worker automatically connects and becomes active

### Test 2: Worker Restart

1. Start master with registered worker
2. Worker is active and connected
3. Kill worker: `Ctrl+C` on worker terminal
4. Verify worker shows inactive: `master> workers`
5. Restart worker: `./workerNode`
6. Wait up to 30 seconds
7. Verify worker reconnects

**Expected Result**: Worker reconnects automatically without manual intervention

### Test 3: Multiple Workers

1. Register 3 workers while all offline
2. Start all workers simultaneously
3. Observe master logs
4. Check all workers become active

**Expected Result**: All workers connect within 30 seconds

## Future Improvements

Possible enhancements:
1. Configurable reconnection interval (environment variable)
2. Exponential backoff for failed connections
3. Metrics tracking (reconnection attempts, success rate)
4. Admin command to trigger immediate reconnection check
5. Different intervals for different worker priorities

## Compatibility

- ✅ Backward compatible (no breaking changes)
- ✅ Works with existing worker registration flow
- ✅ Compatible with database persistence
- ✅ Works with manual and automatic registration
- ✅ Graceful shutdown support
