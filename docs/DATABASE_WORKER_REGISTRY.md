# Database Schema: WORKER_REGISTRY Collection

## Overview

The `WORKER_REGISTRY` collection in MongoDB stores all registered workers with their specifications and status.

## Collection Details

- **Database**: `cluster_db`
- **Collection**: `WORKER_REGISTRY` (existing collection, created by `db.EnsureCollections()`)
- **Purpose**: Persistent storage of authorized workers

## Document Schema

```javascript
{
  worker_id: "worker-1",              // String - Unique worker identifier
  worker_ip: "192.168.1.10",          // String - Worker IP address
  total_cpu: 8.0,                     // Float64 - Total CPU cores
  total_memory: 16.0,                 // Float64 - Total memory in GB
  total_storage: 500.0,               // Float64 - Total storage in GB
  total_gpu: 2.0,                     // Float64 - Total GPU count
  is_active: true,                    // Boolean - Currently connected?
  last_heartbeat: 1697371234,         // Int64 - Unix timestamp
  registered_at: ISODate("2025-10-15T10:30:00Z"),  // Date - Registration time
  updated_at: ISODate("2025-10-15T10:35:00Z")      // Date - Last update time
}
```

## Field Descriptions

| Field            | Type    | Description                      | Initial Value            |
| ---------------- | ------- | -------------------------------- | ------------------------ |
| `worker_id`      | String  | Unique identifier for the worker | Set by admin             |
| `worker_ip`      | String  | IP address or hostname           | Set by admin             |
| `total_cpu`      | Float64 | Total CPU cores available        | 0.0 (updated on connect) |
| `total_memory`   | Float64 | Total RAM in gigabytes           | 0.0 (updated on connect) |
| `total_storage`  | Float64 | Total storage in gigabytes       | 0.0 (updated on connect) |
| `total_gpu`      | Float64 | Number of GPU devices            | 0.0 (updated on connect) |
| `is_active`      | Boolean | Worker currently connected?      | false (true on connect)  |
| `last_heartbeat` | Int64   | Unix timestamp of last heartbeat | 0 (updated on connect)   |
| `registered_at`  | Date    | When admin registered the worker | Set on registration      |
| `updated_at`     | Date    | Last modification timestamp      | Updated on any change    |

## Lifecycle States

### 1. Manual Registration (Admin)

```javascript
{
  worker_id: "worker-2",
  worker_ip: "192.168.1.100",
  total_cpu: 0.0,        // Not yet known
  total_memory: 0.0,     // Not yet known
  total_storage: 0.0,    // Not yet known
  total_gpu: 0.0,        // Not yet known
  is_active: false,      // Not connected
  last_heartbeat: 0,
  registered_at: ISODate("2025-10-15T10:30:00Z"),
  updated_at: ISODate("2025-10-15T10:30:00Z")
}
```

### 2. Worker Connection (Auto-Update)

```javascript
{
  worker_id: "worker-2",
  worker_ip: "192.168.1.100",
  total_cpu: 8.0,        // ✓ Updated from worker
  total_memory: 16.0,    // ✓ Updated from worker
  total_storage: 500.0,  // ✓ Updated from worker
  total_gpu: 1.0,        // ✓ Updated from worker
  is_active: true,       // ✓ Now active
  last_heartbeat: 1697371300,  // ✓ Current timestamp
  registered_at: ISODate("2025-10-15T10:30:00Z"),
  updated_at: ISODate("2025-10-15T10:32:15Z")  // ✓ Updated
}
```

### 3. Heartbeat Updates (Every 5 seconds)

```javascript
{
  // ... other fields unchanged ...
  is_active: true,
  last_heartbeat: 1697371305,  // ✓ Updated every heartbeat
  updated_at: ISODate("2025-10-15T10:32:20Z")  // ✓ Updated
}
```

### 4. Worker Disconnection (Timeout)

```javascript
{
  // ... other fields unchanged ...
  is_active: false,      // ✓ Marked inactive (after timeout)
  last_heartbeat: 1697371305,  // Last known heartbeat
  updated_at: ISODate("2025-10-15T10:42:20Z")
}
```

### 5. Unregistration (Admin)

```javascript
// Document is deleted from collection
```

## Database Operations

### Create (Registration)

```go
db.RegisterWorker(ctx, "worker-2", "192.168.1.100")
```

**MongoDB Operation:**

```javascript
db.WORKER_REGISTRY.insertOne({
  worker_id: "worker-2",
  worker_ip: "192.168.1.100",
  total_cpu: 0.0,
  total_memory: 0.0,
  total_storage: 0.0,
  total_gpu: 0.0,
  is_active: false,
  last_heartbeat: 0,
  registered_at: new Date(),
  updated_at: new Date(),
});
```

### Read (Get Worker)

```go
worker, err := db.GetWorker(ctx, "worker-2")
```

**MongoDB Operation:**

```javascript
db.WORKER_REGISTRY.findOne({ worker_id: "worker-2" });
```

### Read (Get All Workers)

```go
workers, err := db.GetAllWorkers(ctx)
```

**MongoDB Operation:**

```javascript
db.WORKER_REGISTRY.find({});
```

### Update (Worker Info)

```go
db.UpdateWorkerInfo(ctx, workerInfo)
```

**MongoDB Operation:**

```javascript
db.WORKER_REGISTRY.updateOne(
  { worker_id: "worker-2" },
  {
    $set: {
      worker_ip: "192.168.1.100",
      total_cpu: 8.0,
      total_memory: 16.0,
      total_storage: 500.0,
      total_gpu: 1.0,
      is_active: true,
      last_heartbeat: 1697371300,
      updated_at: new Date(),
    },
  }
);
```

### Update (Heartbeat)

```go
db.UpdateHeartbeat(ctx, "worker-2", timestamp)
```

**MongoDB Operation:**

```javascript
db.WORKER_REGISTRY.updateOne(
  { worker_id: "worker-2" },
  {
    $set: {
      last_heartbeat: 1697371305,
      is_active: true,
      updated_at: new Date(),
    },
  }
);
```

### Delete (Unregister)

```go
db.UnregisterWorker(ctx, "worker-2")
```

**MongoDB Operation:**

```javascript
db.WORKER_REGISTRY.deleteOne({ worker_id: "worker-2" });
```

### Check Existence

```go
exists, err := db.WorkerExists(ctx, "worker-2")
```

**MongoDB Operation:**

```javascript
db.WORKER_REGISTRY.countDocuments({ worker_id: "worker-2" });
```

## Indexes

### Recommended Indexes

```javascript
// Primary key (unique)
db.WORKER_REGISTRY.createIndex({ worker_id: 1 }, { unique: true });

// Query by IP
db.WORKER_REGISTRY.createIndex({ worker_ip: 1 });

// Query active workers
db.WORKER_REGISTRY.createIndex({ is_active: 1 });

// Query by last heartbeat (for cleanup)
db.WORKER_REGISTRY.createIndex({ last_heartbeat: 1 });
```

## Example Queries

### Get all active workers

```javascript
db.WORKER_REGISTRY.find({ is_active: true });
```

### Get workers by IP

```javascript
db.WORKER_REGISTRY.find({ worker_ip: "192.168.1.100" });
```

### Get workers with no recent heartbeat (stale)

```javascript
const fiveMinutesAgo = Math.floor(Date.now() / 1000) - 300;
db.WORKER_REGISTRY.find({
  last_heartbeat: { $lt: fiveMinutesAgo },
  is_active: true,
});
```

### Get workers by resource capacity

```javascript
// Workers with > 4 CPUs and > 8GB RAM
db.WORKER_REGISTRY.find({
  total_cpu: { $gt: 4.0 },
  total_memory: { $gt: 8.0 },
});
```

### Get GPU workers

```javascript
db.WORKER_REGISTRY.find({ total_gpu: { $gt: 0 } });
```

## Access from Code

### Go (master/internal/db/workers.go)

```go
import "master/internal/db"

// Initialize
workerDB, err := db.NewWorkerDB(ctx)
defer workerDB.Close(ctx)

// Register
err = workerDB.RegisterWorker(ctx, "worker-1", "192.168.1.10")

// Get all
workers, err := workerDB.GetAllWorkers(ctx)

// Update
err = workerDB.UpdateWorkerInfo(ctx, workerInfo)

// Delete
err = workerDB.UnregisterWorker(ctx, "worker-1")
```

### MongoDB Shell

```bash
# Connect
mongosh -u cloudai -p secret123 cluster_db

# View all workers
db.WORKER_REGISTRY.find().pretty()

# Count workers
db.WORKER_REGISTRY.countDocuments()

# View active workers
db.WORKER_REGISTRY.find({ is_active: true }).pretty()
```

## Integration with Existing Collections

The `WORKER_REGISTRY` collection is created by the existing `db.EnsureCollections()` function alongside:

- `USERS` - User management
- `TASKS` - Task definitions
- `ASSIGNMENTS` - Task-to-worker assignments
- `RESULTS` - Task execution results

All collections are in the `cluster_db` database and are created on master startup if they don't exist.

## Connection Configuration

### Environment Variables

```bash
MONGODB_USERNAME=cloudai
MONGODB_PASSWORD=secret123
```

### Connection String

```
mongodb://cloudai:secret123@localhost:27017
```

### Database

```
cluster_db
```

See `database/docker-compose.yml` for MongoDB configuration.
