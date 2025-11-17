# Worker Registration API and UI Implementation

## Summary
Fixed the CORS error and implemented manual worker registration functionality.

## Issues Fixed

### 1. CORS Error - "response.data is null"
**Problem**: The backend API returned workers as an array `[...]`, but frontend expected `{ workers: [...] }`

**Solution**: Updated `HandleListWorkers` in `master/internal/http/worker_handler.go` to wrap the response:
```go
response := map[string]interface{}{
    "workers": workers,
}
```

## New Features

### 2. Manual Worker Registration API

**Backend Changes:**

1. **New Database Function** (`master/internal/db/workers.go`):
   - Added `RegisterWorkerWithSpecs()` function
   - Accepts full worker specifications (CPU, Memory, Storage, GPU)
   - Validates worker doesn't already exist
   - Sets timestamps automatically

2. **New HTTP Handler** (`master/internal/http/worker_handler.go`):
   - Added `HandleRegisterWorker()` function
   - Endpoint: `POST /api/workers`
   - Request body:
     ```json
     {
       "worker_id": "worker-01",
       "worker_ip": "192.168.1.100:50052",
       "total_cpu": 8.0,
       "total_memory": 16.0,
       "total_storage": 500.0,
       "total_gpu": 1.0
     }
     ```
   - Response (201 Created):
     ```json
     {
       "success": true,
       "message": "Worker registered successfully",
       "worker_id": "worker-01",
       "worker": { ... }
     }
     ```

3. **Updated Route Handler** (`master/internal/http/telemetry_server.go`):
   - Modified `/api/workers` to handle both GET and POST methods
   - GET: List all workers
   - POST: Register new worker

**Frontend Changes:**

1. **New API Method** (`ui/src/api/workers.js`):
   ```javascript
   registerWorker: (workerData) => {
     return apiClient.post('/api/workers', workerData);
   }
   ```

2. **New Component** (`ui/src/components/RegisterWorkerDialog.jsx`):
   - Material-UI Dialog for worker registration
   - Form fields:
     - Worker ID (required)
     - Worker IP (required, format: ip:port)
     - Total CPU (cores)
     - Total Memory (GB)
     - Total Storage (GB)
     - Total GPU (count)
   - Validation and error handling
   - Success callback

3. **Updated WorkersPage** (`ui/src/pages/WorkersPage.jsx`):
   - Added "Register Worker" button
   - Integrated RegisterWorkerDialog
   - Success message display
   - Auto-refresh after registration

## API Reference

### POST /api/workers - Register Worker

**Request:**
```bash
curl -X POST http://localhost:8080/api/workers \
  -H "Content-Type: application/json" \
  -d '{
    "worker_id": "worker-01",
    "worker_ip": "192.168.1.100:50052",
    "total_cpu": 8.0,
    "total_memory": 16.0,
    "total_storage": 500.0,
    "total_gpu": 1.0
  }'
```

**Response (201 Created):**
```json
{
  "success": true,
  "message": "Worker registered successfully",
  "worker_id": "worker-01",
  "worker": {
    "worker_id": "worker-01",
    "worker_ip": "192.168.1.100:50052",
    "total_cpu": 8.0,
    "total_memory": 16.0,
    "total_storage": 500.0,
    "total_gpu": 1.0,
    "is_active": true,
    "registered_at": 1700123456
  }
}
```

**Error Responses:**
- `400 Bad Request`: Missing required fields or invalid data
- `500 Internal Server Error`: Database error or worker already exists

### GET /api/workers - List Workers

**Response:**
```json
{
  "workers": [
    {
      "worker_id": "worker-01",
      "is_active": true,
      "cpu_usage": 45.2,
      "memory_usage": 8.5,
      "gpu_usage": 0.0,
      "running_tasks_count": 2,
      "last_update": 1700123456
    }
  ]
}
```

## Testing

### Backend
1. Restart the master node:
   ```bash
   cd /home/vishv/websites/cloned/CloudAI
   ./runMaster.sh
   ```

2. Test the API:
   ```bash
   curl -X POST http://localhost:8080/api/workers \
     -H "Content-Type: application/json" \
     -d '{
       "worker_id": "test-worker",
       "worker_ip": "127.0.0.1:50052",
       "total_cpu": 4,
       "total_memory": 8,
       "total_storage": 100,
       "total_gpu": 0
     }'
   ```

### Frontend
1. Start the UI:
   ```bash
   cd /home/vishv/websites/cloned/CloudAI/ui
   npm run dev
   ```

2. Navigate to Workers page
3. Click "Register Worker" button
4. Fill in the form and submit
5. Verify worker appears in the list

## Files Modified

### Backend
- `master/internal/http/worker_handler.go` - Added time import, HandleRegisterWorker function
- `master/internal/http/telemetry_server.go` - Updated route handler for POST
- `master/internal/db/workers.go` - Added RegisterWorkerWithSpecs function

### Frontend
- `ui/src/api/workers.js` - Added registerWorker method
- `ui/src/pages/WorkersPage.jsx` - Added register button and dialog integration
- `ui/src/components/RegisterWorkerDialog.jsx` - New component (created)

## Notes

- Workers registered manually will appear in the workers list immediately
- The worker must actually connect via gRPC to start receiving tasks
- Manual registration is useful for pre-configuring workers before they come online
- The `is_active` status will be updated when the worker sends its first heartbeat
- All resource fields (CPU, Memory, etc.) are optional and default to 0 if not provided
