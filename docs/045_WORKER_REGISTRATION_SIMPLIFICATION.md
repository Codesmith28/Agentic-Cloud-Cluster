# Worker Registration Simplification

## Overview
Simplified worker registration to only require Worker ID and Port. Resources are now auto-detected when the worker connects, eliminating manual entry and avoiding the need to restart the master server.

## Changes Made

### 1. Frontend Changes

#### WorkerRegistrationDialog.jsx
**Location:** `ui/src/components/WorkerRegistrationDialog.jsx`

**Before:** 6 input fields
- worker_id (required)
- worker_ip (required)
- total_cpu (required)
- total_memory (required)
- total_storage (required)
- total_gpu (optional)

**After:** 2 input fields
- worker_id (required)
- worker_port (required)

**Changes:**
- Removed 4 resource input fields (CPU, Memory, Storage, GPU)
- Changed `worker_ip` to `worker_port` (just the port number)
- Updated helper text to indicate resources will be auto-detected
- Simplified form submission to only send ID and port

### 2. Backend Changes

#### worker_handler.go
**Location:** `master/internal/http/worker_handler.go`

**Function:** `HandleRegisterWorker`

**Before:**
- Accepted 6 fields: worker_id, worker_ip, total_cpu, total_memory, total_storage, total_gpu
- Created full WorkerDocument with all resource specs
- Only inserted into database (didn't register with running master)
- Required manual restart to detect new workers

**After:**
- Accepts 2 fields: worker_id, worker_port
- Automatically prepends "localhost:" if port doesn't include hostname
- Calls `masterServer.ManualRegisterWorker()` directly
- Registers worker with both in-memory map AND database
- No restart needed - worker is immediately available

**Key Improvement:**
```go
// Old approach: Only DB insertion
if err := h.workerDB.RegisterWorkerWithSpecs(ctx, worker); err != nil {
    // Worker created in DB but not in master's memory
}

// New approach: Immediate registration
if err := h.masterServer.ManualRegisterWorker(ctx, req.WorkerID, workerIP); err != nil {
    // Worker registered in BOTH memory and DB
}
```

## How It Works

### Registration Flow

1. **User submits form** with worker_id and worker_port
   ```json
   {
     "worker_id": "worker-1",
     "worker_port": "50052"
   }
   ```

2. **Backend processes request:**
   - Converts port to "localhost:port" format if needed
   - Calls `ManualRegisterWorker()` on master server
   - Creates worker entry with minimal info (ID and address)
   - Resources initialized to 0 (will be filled when worker connects)

3. **Worker appears in system immediately:**
   - Shows as inactive (not connected yet)
   - Available resources show as 0
   - No restart required

4. **When worker connects via gRPC:**
   - Worker sends full resource info
   - Master updates resources automatically
   - Worker becomes active
   - Resources show actual values

### Resource Auto-Detection

Workers auto-detect resources through the gRPC `RegisterWorker` call:
- CPU cores
- Memory (GB)
- Storage (GB)
- GPU cores

The manual registration just creates a "placeholder" worker that gets populated when the actual worker connects.

## API Changes

### POST /api/workers

**Old Request:**
```json
{
  "worker_id": "worker-1",
  "worker_ip": "localhost:50052",
  "total_cpu": 4.0,
  "total_memory": 8.0,
  "total_storage": 100.0,
  "total_gpu": 0.0
}
```

**New Request:**
```json
{
  "worker_id": "worker-1",
  "worker_port": "50052"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Worker registered successfully. Resources will be auto-detected when the worker connects.",
  "worker_id": "worker-1",
  "worker": {
    "worker_id": "worker-1",
    "worker_ip": "localhost:50052",
    "is_active": false
  }
}
```

## Benefits

### 1. Better UX
- **Simplified form:** Only 2 fields instead of 6
- **No technical knowledge needed:** Users don't need to know their CPU/memory specs
- **Faster registration:** Less typing, fewer errors

### 2. More Accurate
- **No manual errors:** Resources are detected accurately by the worker
- **Always up-to-date:** Resources reflect actual hardware, not outdated manual entry

### 3. No Restart Required
- **Immediate availability:** Worker appears in UI instantly
- **Real-time updates:** WebSocket shows when worker becomes active
- **Better reliability:** No need to coordinate restart timing

### 4. Cleaner Architecture
- **Single source of truth:** Resources come from actual worker, not manual input
- **Proper separation:** Registration just creates identity, worker provides capabilities
- **Consistent with gRPC flow:** Manual registration now mirrors automatic worker connection

## Testing

### Test Manual Registration

1. **Start master server:**
   ```bash
   cd master
   go run main.go
   ```

2. **Open UI:**
   ```bash
   cd ui
   npm run dev
   # Open http://localhost:3000
   ```

3. **Register a worker:**
   - Click "Register Worker" button
   - Enter Worker ID: `worker-1`
   - Enter Port: `50052`
   - Click "Register Worker"

4. **Verify:**
   - Worker appears in list immediately (no refresh needed)
   - Shows as inactive (red status)
   - Resources show 0

5. **Start the actual worker:**
   ```bash
   cd worker
   go run main.go
   ```

6. **Verify auto-detection:**
   - Worker becomes active (green status)
   - Resources populate automatically
   - CPU, Memory, Storage show actual values

## Migration Notes

### For Existing Installations

If you have existing workers registered with the old API:
- They will continue to work
- Resource values are preserved in database
- New registrations use simplified flow

### For API Clients

If you have scripts calling the registration API:
- **Update your JSON payload** to use new format
- Remove: `total_cpu`, `total_memory`, `total_storage`, `total_gpu`
- Change: `worker_ip` → `worker_port`
- Port can be just the number (e.g., `"50052"`) or full address (e.g., `"localhost:50052"`)

**Old:**
```bash
curl -X POST http://localhost:8080/api/workers \
  -H "Content-Type: application/json" \
  -d '{
    "worker_id": "worker-1",
    "worker_ip": "localhost:50052",
    "total_cpu": 4,
    "total_memory": 8,
    "total_storage": 100,
    "total_gpu": 0
  }'
```

**New:**
```bash
curl -X POST http://localhost:8080/api/workers \
  -H "Content-Type: application/json" \
  -d '{
    "worker_id": "worker-1",
    "worker_port": "50052"
  }'
```

## Technical Details

### ManualRegisterWorker Function

**Location:** `master/internal/server/master_server.go`

**What it does:**
1. Checks if worker already exists (prevents duplicates)
2. Adds to database with minimal info
3. Adds to in-memory workers map
4. Sets IsActive = false (will be true when worker connects)
5. Initializes all resources to 0

**Code:**
```go
func (s *MasterServer) ManualRegisterWorker(ctx context.Context, workerID, workerIP string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Check if already exists
    if _, exists := s.workers[workerID]; exists {
        return fmt.Errorf("worker %s already registered", workerID)
    }

    // Add to database
    if s.workerDB != nil {
        // ... database operations ...
    }

    // Add to memory with minimal info
    s.workers[workerID] = &WorkerState{
        Info: &pb.WorkerInfo{
            WorkerId: workerID,
            WorkerIp: workerIP,
        },
        IsActive:     false, // Not active until worker connects
        RunningTasks: make(map[string]bool),
        // Resources initialized to 0
    }

    return nil
}
```

### Worker Connection Flow

1. **Manual registration** creates placeholder
2. **Worker starts** and calls gRPC `RegisterWorker`
3. **Master receives** worker info with full resource specs
4. **Master updates** the existing worker entry
5. **Telemetry system** starts tracking the worker
6. **UI updates** via WebSocket (worker becomes active)

## Files Modified

### Frontend
- `ui/src/components/WorkerRegistrationDialog.jsx`

### Backend
- `master/internal/http/worker_handler.go`

## Related Documentation
- [Worker Registration API](043_WORKER_REGISTRATION_API.md)
- [Real-time Updates Implementation](044_REALTIME_UPDATES_IMPLEMENTATION.md)
- [WebSocket Telemetry](009_WEBSOCKET_TELEMETRY_README.md)
