# Telemetry System - WebSocket Streaming

## Overview

The CloudAI telemetry system now uses **WebSocket connections** for real-time streaming of worker telemetry data. This is much more efficient than polling REST APIs, as telemetry updates are pushed to clients as soon as they're received from workers.

## Architecture

### Worker Side
- Each worker runs a dedicated goroutine for telemetry collection and transmission
- Telemetry is collected every 5 seconds (configurable)
- Data includes: CPU, Memory, GPU usage, and running tasks
- Sent via gRPC `SendHeartbeat` RPC to master

### Master Side
- **TelemetryManager**: Manages per-worker telemetry reception threads
  - Each worker gets its own dedicated goroutine
  - Non-blocking heartbeat processing
  - Thread-safe data access via channels and mutexes
  
- **WebSocket Server**: Streams telemetry to connected clients
  - Real-time push updates (no polling needed)
  - Supports filtering by worker ID
  - Automatic reconnection handling

## WebSocket Endpoints

### 1. Stream All Workers
```
ws://localhost:8080/ws/telemetry
```

Streams real-time telemetry for all registered workers. You'll receive updates whenever any worker sends a heartbeat.

**Example Response:**
```json
{
  "worker-1": {
    "worker_id": "worker-1",
    "cpu_usage": 45.2,
    "memory_usage": 62.1,
    "gpu_usage": 78.3,
    "running_tasks": [
      {
        "task_id": "task-123",
        "cpu_allocated": 2.0,
        "memory_allocated": 4096.0,
        "gpu_allocated": 1.0,
        "status": "running"
      }
    ],
    "last_update": 1699296000,
    "is_active": true
  }
}
```

### 2. Stream Specific Worker
```
ws://localhost:8080/ws/telemetry/{worker_id}
```

Streams real-time telemetry for a single worker. Only updates for the specified worker are sent.

**Example:**
```
ws://localhost:8080/ws/telemetry/worker-1
```

### 3. Health Check (HTTP)
```
GET http://localhost:8080/health
```

Returns server health and statistics.

**Example Response:**
```json
{
  "status": "healthy",
  "time": 1699296000,
  "active_clients": 2,
  "workers": 5,
  "active_workers": 4
}
```

## Usage Examples

### JavaScript/Browser
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');

ws.onopen = () => {
    console.log('Connected to telemetry stream');
};

ws.onmessage = (event) => {
    const telemetry = JSON.parse(event.data);
    console.log('Telemetry update:', telemetry);
    
    // Update your UI with the telemetry data
    updateDashboard(telemetry);
};

ws.onerror = (error) => {
    console.error('WebSocket error:', error);
};

ws.onclose = () => {
    console.log('Disconnected');
    // Implement reconnection logic here
};
```

### Python (asyncio)
```python
import asyncio
import websockets
import json

async def stream_telemetry():
    uri = "ws://localhost:8080/ws/telemetry"
    
    async with websockets.connect(uri) as websocket:
        print("Connected to telemetry stream")
        
        while True:
            message = await websocket.recv()
            telemetry = json.loads(message)
            
            for worker_id, data in telemetry.items():
                print(f"{worker_id}: CPU={data['cpu_usage']:.1f}% "
                      f"Mem={data['memory_usage']:.1f}% "
                      f"GPU={data['gpu_usage']:.1f}%")

asyncio.run(stream_telemetry())
```

### Node.js (ws library)
```javascript
const WebSocket = require('ws');

const ws = new WebSocket('ws://localhost:8080/ws/telemetry');

ws.on('open', () => {
    console.log('Connected to telemetry stream');
});

ws.on('message', (data) => {
    const telemetry = JSON.parse(data);
    console.log('Telemetry:', telemetry);
});

ws.on('close', () => {
    console.log('Disconnected');
});
```

### CLI (wscat)
```bash
# Install wscat
npm install -g wscat

# Connect to all workers
wscat -c ws://localhost:8080/ws/telemetry

# Connect to specific worker
wscat -c ws://localhost:8080/ws/telemetry/worker-1
```

### cURL (Health Check)
```bash
curl http://localhost:8080/health | jq
```

## Configuration

The WebSocket server is **enabled by default** on port **8080**. No configuration needed!

```bash
# Default: runs on port 8080
./masterNode

# Change port (optional)
HTTP_PORT=:9090 ./masterNode

# Disable WebSocket server (optional)
HTTP_PORT="" ./masterNode
```

You can also set it in the `.env` file:
```bash
# master/.env
HTTP_PORT=:8080  # This is already the default
```

The default is defined in `master/internal/config/config.go` (line 29).

## Testing

Use the provided Python test client:

```bash
# Install websockets if needed
pip3 install websockets

# Run the test client
python3 test_telemetry_websocket.py

# Stream specific worker
python3 test_telemetry_websocket.py <worker_id>

# Show examples
python3 test_telemetry_websocket.py --examples
```

This script will:
1. Test the health endpoint
2. Show usage examples for various languages
3. Stream live telemetry data with colored output

This script:
1. Tests the health endpoint
2. Shows examples for various clients
3. Includes a Python WebSocket client that displays live telemetry

## Benefits of WebSocket over REST

1. **Real-time**: Updates pushed immediately, no polling delay
2. **Efficient**: Single persistent connection, no repeated HTTP handshakes
3. **Low Latency**: Minimal overhead for each update
4. **Scalable**: Server can push to multiple clients without per-client requests
5. **Bidirectional**: Clients can send commands (future enhancement)

## Implementation Details

### Concurrency Model
- Main gRPC server handles `SendHeartbeat` RPCs
- Heartbeat data forwarded to `TelemetryManager` via channels
- Each worker has dedicated goroutine processing its telemetry
- WebSocket server runs in separate goroutine
- Each WebSocket client has dedicated read/write goroutines

### Thread Safety
- All data access protected by `sync.RWMutex`
- Channel-based communication for non-blocking updates
- Graceful shutdown ensures all goroutines are properly closed

### Performance
- Buffered channels prevent blocking
- Copy-on-read ensures no data races
- Minimal lock contention with RLock for reads
- WebSocket broadcasts use goroutines to avoid blocking

## Future Enhancements

1. **Authentication**: Add token-based auth for WebSocket connections
2. **Compression**: Enable WebSocket compression for large payloads
3. **Filtering**: Allow clients to subscribe to specific metrics only
4. **Aggregation**: Real-time aggregation and statistics
5. **Alerts**: Server-side alerting for threshold breaches
6. **Historical Data**: Time-series storage and playback

## Troubleshooting

### Connection Refused
- Ensure master is running with HTTP_PORT configured
- Check firewall settings

### No Updates Received
- Verify workers are sending heartbeats
- Check worker registration status
- Look for errors in master logs

### High Memory Usage
- Limit number of WebSocket clients
- Implement client disconnection timeouts
- Monitor goroutine count

## Related Files

- `master/internal/telemetry/telemetry_manager.go` - Telemetry management
- `master/internal/http/telemetry_server.go` - WebSocket server
- `worker/internal/telemetry/telemetry.go` - Worker telemetry collection
- `test_telemetry_websocket.sh` - Test script and examples
