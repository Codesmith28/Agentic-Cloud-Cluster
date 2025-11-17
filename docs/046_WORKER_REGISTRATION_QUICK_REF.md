# Worker Registration - Quick Reference

## Quick Start

### Register a Worker (UI)
1. Click "Register Worker" button
2. Enter Worker ID (e.g., `worker-1`)
3. Enter Port (e.g., `50052`)
4. Click "Register Worker"
5. Worker appears immediately - no restart needed!

### Register a Worker (API)
```bash
curl -X POST http://localhost:8080/api/workers \
  -H "Content-Type: application/json" \
  -d '{"worker_id": "worker-1", "worker_port": "50052"}'
```

## Key Points

✅ **Only 2 fields needed:** worker_id and worker_port  
✅ **Resources auto-detected** when worker connects  
✅ **No restart required** - worker appears immediately  
✅ **Shows inactive** until actual worker connects  
✅ **Real-time updates** via WebSocket when worker becomes active

## Field Reference

| Field | Required | Format | Example |
|-------|----------|--------|---------|
| worker_id | Yes | string | `"worker-1"` |
| worker_port | Yes | string | `"50052"` or `"localhost:50052"` |

## Common Issues

### Worker shows as inactive
- Worker is registered but hasn't connected yet
- Start the actual worker process: `cd worker && go run main.go`

### Worker resources show 0
- Normal before worker connects
- Resources populate when worker connects via gRPC

### "Worker already registered" error
- Worker ID already exists
- Choose a different worker_id or unregister the existing worker

## Migration from Old API

**Old format (6 fields):**
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

**New format (2 fields):**
```json
{
  "worker_id": "worker-1",
  "worker_port": "50052"
}
```

## Response Format

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

## Testing Checklist

- [ ] Register worker via UI - appears immediately
- [ ] Check worker list - shows as inactive with 0 resources
- [ ] Start actual worker process
- [ ] Worker becomes active (green badge)
- [ ] Resources populate automatically
- [ ] No master restart needed at any step

## Related Commands

**Start Master:**
```bash
cd master && go run main.go
```

**Start Worker:**
```bash
cd worker && go run main.go
```

**Start UI:**
```bash
cd ui && npm run dev
```

**Check Workers (API):**
```bash
curl http://localhost:8080/api/workers
```

## See Also
- [Full Documentation](045_WORKER_REGISTRATION_SIMPLIFICATION.md)
- [Worker Registration API](043_WORKER_REGISTRATION_API.md)
