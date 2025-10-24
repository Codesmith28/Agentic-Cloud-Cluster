# Worker Registration Guide

## Overview

The CloudAI master node uses **authorization-based registration**:

- **✅ Manual Registration Required**: Administrators must pre-register workers via CLI
- **❌ Automatic Registration Disabled**: Workers cannot self-register without admin approval

This ensures only authorized workers can join your cluster.

## Registration Security Model

### Authorization Flow

1. **Admin pre-registers** worker with ID and IP via CLI
2. **Worker attempts connection** with that ID
3. **Master validates** worker is pre-registered
4. **Connection accepted** only if pre-registered
5. **Worker specs updated** with actual system capabilities

### Why This Design?

- **Security**: Prevents unauthorized workers from joining
- **Control**: Admin explicitly approves each worker
- **Auditability**: All workers are known and tracked
- **Persistence**: Registration survives master restarts

## Manual Registration

### Purpose

Manual registration allows you to:

- Pre-register workers before they come online
- Control which workers can join the cluster
- Maintain a persistent registry across master restarts

### Usage

#### Register a Worker

```bash
master> register <worker_id> <worker_address>
```

**Format:** `worker_address` must include port (e.g., `192.168.1.100:50052`)

**Example:**

```bash
master> register worker-2 192.168.1.100:50052:50052
✅ Worker worker-2 registered with address 192.168.1.100:50052
   Note: Worker will send full specs when it connects
```

**What happens:**

1. Worker is added to the in-memory registry with address `ip:port`
2. Worker is persisted to MongoDB (if connected)
3. Worker status is marked as `inactive`
4. Resource specs (CPU, Memory, GPU) are initialized to 0

#### When the Worker Connects

When a manually registered worker starts up and connects:

1. Master recognizes it's already registered
2. Worker sends its full system specifications (CPU, Memory, GPU, Storage)
3. Master updates the worker's info in memory and database
4. Worker status changes to `active`
5. Worker begins sending heartbeats

### Unregistering Workers

#### Remove a Worker

```bash
master> unregister <worker_id>
```

**Example:**

```bash
master> unregister worker-2
✅ Worker worker-2 has been unregistered
```

**What happens:**

1. Worker is removed from in-memory registry
2. Worker is deleted from MongoDB (if connected)
3. Any active connections from that worker will be rejected

### Viewing Workers

Use the `workers` command to see all registered workers:

```bash
master> workers

╔═══ Registered Workers 
║ worker-1
║   Status: 🟢 Active
║   IP: localhost
║   Resources: CPU=4.0, Memory=8.0GB, GPU=0.0
║   Running Tasks: 0
║
║ worker-2
║   Status: 🔴 Inactive
║   IP: 192.168.1.100
║   Resources: CPU=0.0, Memory=0.0GB, GPU=0.0
║   Running Tasks: 0
║
╚═══════════════════════
```

**Status Indicators:**

- 🟢 **Active**: Worker is connected and sending heartbeats
- 🔴 **Inactive**: Worker is registered but not connected

## Authorization and Security

### Unauthorized Worker Rejection

If a worker tries to connect without being pre-registered:

**Worker Output:**

```
2025/10/15 10:30:00 Starting Worker Node: worker-unauthorized
2025/10/15 10:30:00 Master Address: localhost:50051
2025/10/15 10:30:00 ❌ Registration failed: Worker worker-unauthorized is not authorized.
                     Admin must register it first using: register worker-unauthorized <ip>
```

**Master Output:**

```
2025/10/15 10:30:00 ❌ Rejected unauthorized worker registration attempt:
                     worker-unauthorized (IP: 192.168.1.50)
```

**Solution:**

```bash
master> register worker-unauthorized 192.168.1.50
✅ Worker worker-unauthorized registered with IP 192.168.1.50

# Now the worker can connect successfully
```

### Required Workflow

1. **Admin registers first**:

   ```bash
   master> register worker-1 192.168.1.10:50052
   ```

2. **Then worker can start**:

   ```bash
   ./workerNode  # worker_id must match "worker-1"
   ```

3. **Worker connects and updates specs**:
   ```
   ✓ Pre-registered worker connecting: worker-1 (CPU: 8.0, Memory: 16.0GB)
   ```

## Persistence

### Database Storage

Workers are stored in the `WORKER_REGISTRY` collection in MongoDB with:

- `worker_id`: Unique worker identifier
- `worker_ip`: IP address
- `total_cpu`, `total_memory`, `total_storage`, `total_gpu`: Resource specs
- `is_active`: Current connection status
- `last_heartbeat`: Unix timestamp of last heartbeat
- `registered_at`: Registration timestamp
- `updated_at`: Last update timestamp

### Startup Behavior

When the master node starts:

1. Connects to MongoDB
2. Loads all registered workers from database
3. Initializes them in memory with their last known state
4. Waits for workers to connect and update their specs

### Without Database

If MongoDB is not available:

- Manual registration still works (in-memory only)
- Registrations are lost on master restart
- Warning messages are logged but system continues

## Registration Flow Diagram

### Manual Registration Flow

```
CLI Command          Master Server         MongoDB
    |                      |                   |
    | register worker-2    |                   |
    |--------------------->|                   |
    |                      |                   |
    |                      | Insert worker     |
    |                      |------------------>|
    |                      |                   |
    |                      | Store in memory   |
    |                      | (inactive)        |
    |                      |                   |
    | ✅ Success           |                   |
    |<---------------------|                   |
```

### Worker Connection Flow

```
Worker Node          Master Server         MongoDB
    |                      |                   |
    | RegisterWorker()     |                   |
    | (with full specs)    |                   |
    |--------------------->|                   |
    |                      |                   |
    |                      | Check if exists   |
    |                      |                   |
    |                      | Update specs      |
    |                      |------------------>|
    |                      |                   |
    |                      | Mark active       |
    |                      |                   |
    | ✅ Ack               |                   |
    |<---------------------|                   |
    |                      |                   |
    | SendHeartbeat()      |                   |
    |--------------------->|                   |
    |      (every 5s)      |                   |
```

## Best Practices

1. **Pre-register known workers**: For production deployments, pre-register all expected workers
2. **Use descriptive IDs**: Use meaningful worker IDs like `worker-gpu-1`, `worker-cpu-5`
3. **Verify registration**: Use `workers` command to confirm worker is registered
4. **Clean up inactive workers**: Periodically unregister workers that won't reconnect
5. **Monitor database**: Ensure MongoDB is running for persistence across restarts

## Troubleshooting

### Worker Not Appearing After Registration

```bash
master> register worker-2 192.168.1.100:50052
✅ Worker worker-2 registered with IP 192.168.1.100

master> workers
No workers registered yet.
```

**Possible Causes:**

- Command succeeded but database write failed (check MongoDB connection)
- System bug (check logs)

**Solution:**

- Check master logs for database errors
- Verify MongoDB is running: `docker ps | grep mongo`

### Worker Connection Rejected

```
Worker log: rpc error: code = Unknown desc = worker worker-2 not authorized - must be pre-registered by admin
```

**Cause:** Worker ID hasn't been pre-registered by admin

**Solution:**

- Register the worker first: `master> register worker-2 192.168.1.100:50052`
- Then start the worker

### Duplicate Registration Error

```bash
master> register worker-1 localhost
❌ Failed to register worker: worker worker-1 already registered
```

**Cause:** Worker ID already exists

**Solution:**

- Use different worker ID
- Or unregister existing worker first: `master> unregister worker-1`

## API Reference

### Master Server Methods

#### `ManualRegisterWorker(ctx, workerID, workerIP) error`

Manually registers a worker with minimal info.

#### `UnregisterWorker(ctx, workerID) error`

Removes a worker from the system.

#### `LoadWorkersFromDB(ctx) error`

Loads all workers from database on startup.

#### `RegisterWorker(ctx, info) (*RegisterAck, error)`

Handles worker gRPC registration (called by workers).

### Database Methods

#### `db.RegisterWorker(ctx, workerID, workerIP) error`

Stores worker registration in MongoDB.

#### `db.UpdateWorkerInfo(ctx, info) error`

Updates worker specifications.

#### `db.UnregisterWorker(ctx, workerID) error`

Deletes worker from database.

#### `db.GetAllWorkers(ctx) ([]WorkerDocument, error)`

Retrieves all registered workers.

## Examples

### Scenario 1: Pre-registering Workers for a Cluster

```bash
# Start master
./masterNode

# Register three workers
master> register worker-1 192.168.1.10:50052
master> register worker-2 192.168.1.11
master> register worker-3 192.168.1.12

# Verify registration
master> workers
# Shows all 3 workers as inactive

# Start workers on respective machines
# They will automatically update their specs when connecting
```

### Scenario 2: Removing a Failed Worker

```bash
# Check current workers
master> workers
║ worker-3
║   Status: 🔴 Inactive
║   Last seen: 2 hours ago

# Remove the failed worker
master> unregister worker-3
✅ Worker worker-3 has been unregistered

# Verify removal
master> workers
# worker-3 no longer appears
```

### Scenario 3: Database Persistence Across Restarts

```bash
# Day 1: Register workers
master> register worker-1 192.168.1.10:50052
master> register worker-2 192.168.1.11
master> exit

# Day 2: Restart master
./masterNode
# Output: Loaded 2 workers from database

master> workers
# Both workers appear (inactive until they connect)
```

---

For more information, see:

- [SETUP.md](SETUP.md) - Initial setup guide
- [TESTING.md](TESTING.md) - Testing procedures
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues
