# CouchDB Migration Guide

## Overview
The CloudAI project has been updated to use **CouchDB** as the primary persistence layer instead of BoltDB. This document outlines the changes and benefits.

---

## Why CouchDB?

### Advantages over BoltDB

1. **Distributed Architecture**
   - Built-in replication and clustering
   - Multi-master replication for high availability
   - Eventual consistency with conflict resolution

2. **Document-Oriented**
   - Perfect for storing complex JSON structures (tasks, workers, plans)
   - Schema-less design allows flexibility
   - No need for serialization/deserialization

3. **RESTful API**
   - HTTP-based access from any language
   - Easy integration with both Go and Python
   - Simple CURL-based debugging

4. **Real-time Changes Feed**
   - Monitor document changes in real-time
   - Perfect for event-driven architecture
   - Enables reactive planner updates

5. **MapReduce Views**
   - Complex queries without full table scans
   - Pre-computed indexes for fast queries
   - Aggregate functions for analytics

6. **Production-Ready**
   - ACID compliant
   - Battle-tested in production (used by npm, Apple, etc.)
   - Excellent documentation and community support

---

## Changes Made to Sprint.md

### 1. Technology Stack
- ✅ Updated: `BoltDB` → `CouchDB`
- ✅ Added: CouchDB client libraries for Go and Python

### 2. Docker Compose Configuration
- ✅ Added: CouchDB service with health checks
- ✅ Added: Environment variables for CouchDB connection
- ✅ Added: Persistent volumes for CouchDB data
- ✅ Updated: Master and Planner services to depend on CouchDB

### 3. Sprint 0 - New Task Added
- ✅ **Task 0.6: CouchDB Setup & Configuration**
  - Database schema design
  - Document structure examples
  - Design documents (views) creation
  - Go CouchDB client implementation
  - Python CouchDB client setup
  - Connection tests

### 4. Folder Structure
- ✅ Updated: `pkg/persistence/` comment from "BoltDB" to "CouchDB"

### 5. Persistence Strategy
- ✅ Changed: From embedded database to distributed database
- ✅ Added: Optional planner data persistence to CouchDB

---

## Database Schema

### Databases

1. **`cloudai`** - Master node data
   - Tasks
   - Workers
   - Plans
   - Assignments
   - Reservations

2. **`cloudai_planner`** - Planner service data
   - Training data for ML models
   - Historical plans for analysis

### Document Types

#### Task Document
```json
{
  "_id": "task:task-123",
  "type": "task",
  "cpu_req": 2.0,
  "mem_mb": 4096,
  "status": "pending",
  "priority": 5,
  "deadline_unix": 1696512000
}
```

#### Worker Document
```json
{
  "_id": "worker:worker-1",
  "type": "worker",
  "total_cpu": 8.0,
  "free_cpu": 4.0,
  "status": "active",
  "last_seen_unix": 1696512345
}
```

#### Assignment Document
```json
{
  "_id": "assignment:task-123",
  "type": "assignment",
  "task_id": "task-123",
  "worker_id": "worker-1",
  "status": "running"
}
```

### Views (MapReduce)

#### Tasks by Status
```javascript
function(doc) {
  if (doc.type === 'task') {
    emit(doc.status, doc);
  }
}
```

#### Active Workers
```javascript
function(doc) {
  if (doc.type === 'worker' && doc.status === 'active') {
    emit(doc.last_seen_unix, doc);
  }
}
```

---

## Implementation Guide

### 1. Environment Variables

Add to your `.env` file:
```bash
COUCHDB_URL=http://localhost:5984
COUCHDB_USER=admin
COUCHDB_PASSWORD=password
COUCHDB_DATABASE=cloudai
```

### 2. Docker Compose

Start CouchDB:
```bash
docker-compose up -d couchdb
```

Access CouchDB UI:
```
http://localhost:5984/_utils
```

### 3. Go Client Usage

```go
import "github.com/Codesmith28/CloudAI/go-master/pkg/persistence"

// Create client
client, err := persistence.NewCouchDBClient()
if err != nil {
    log.Fatal(err)
}

// Save a task
task := map[string]interface{}{
    "type": "task",
    "id": "task-123",
    "cpu_req": 2.0,
}
err = client.PutDocument(ctx, "task:task-123", task)

// Retrieve a task
var result map[string]interface{}
err = client.GetDocument(ctx, "task:task-123", &result)

// Query view
err = client.QueryView(ctx, "tasks", "by_status", 
    map[string]string{"key": `"pending"`}, &result)
```

### 4. Python Client Usage

```python
import couchdb

# Connect to CouchDB
couch = couchdb.Server('http://admin:password@localhost:5984/')
db = couch['cloudai']

# Save a document
doc = {
    'type': 'task',
    'id': 'task-123',
    'cpu_req': 2.0
}
db['task:task-123'] = doc

# Retrieve a document
doc = db['task:task-123']

# Query view
for row in db.view('tasks/by_status', key='pending'):
    print(row.value)
```

---

## Migration Steps (If Migrating from BoltDB)

### Step 1: Export BoltDB Data
```go
// Read all data from BoltDB
db.View(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("tasks"))
    b.ForEach(func(k, v []byte) error {
        // Export to JSON
        return nil
    })
    return nil
})
```

### Step 2: Import to CouchDB
```go
// Import JSON data to CouchDB
for _, doc := range exportedDocs {
    client.PutDocument(ctx, doc.ID, doc)
}
```

### Step 3: Update Application Code
- Replace BoltDB calls with CouchDB client calls
- Update queries to use CouchDB views
- Test thoroughly

---

## Testing CouchDB

### 1. Start CouchDB
```bash
docker-compose up -d couchdb
```

### 2. Verify CouchDB is Running
```bash
curl http://admin:password@localhost:5984/
```

Expected response:
```json
{
  "couchdb": "Welcome",
  "version": "3.3.x"
}
```

### 3. Run Tests
```bash
cd go-master
go test ./pkg/persistence/...
```

---

## Performance Considerations

### Indexing
- Create views for frequently queried fields
- Use compound keys for complex queries
- Monitor view performance with `_stats`

### Caching
- Implement in-memory caching for hot data
- Use CouchDB's `_changes` feed to invalidate cache
- Consider Redis for session data

### Replication
- Set up master-master replication for HA
- Use filtered replication for large datasets
- Monitor replication lag

---

## Troubleshooting

### Connection Issues
```bash
# Check if CouchDB is running
docker ps | grep couchdb

# Check CouchDB logs
docker logs cloudai-couchdb-1

# Test connection
curl http://localhost:5984/_up
```

### Authentication Issues
```bash
# Verify credentials
curl -u admin:password http://localhost:5984/_session
```

### View Issues
```bash
# Rebuild views
curl -X POST http://admin:password@localhost:5984/cloudai/_view_cleanup

# Check view errors
curl http://admin:password@localhost:5984/cloudai/_design/tasks/_view/by_status
```

---

## Next Steps

1. ✅ Complete Sprint 0 - CouchDB setup
2. ⏳ Implement persistence layer in Sprint 1
3. ⏳ Integrate with worker registry and task queue
4. ⏳ Add real-time change monitoring
5. ⏳ Set up replication for HA

---

## Resources

- [CouchDB Documentation](https://docs.couchdb.org/)
- [CouchDB Go Client](https://github.com/go-kivik/kivik)
- [CouchDB Python Client](https://github.com/djc/couchdb-python)
- [CouchDB Best Practices](https://docs.couchdb.org/en/stable/best-practices/index.html)
