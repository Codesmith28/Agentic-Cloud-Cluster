# WebSocket Telemetry - Quick Start Guide

## Quick Setup (5 minutes)

### 1. Build the System
```bash
# Build master
cd master
go build -o masterNode

# Build worker
cd ../worker
go build -o workerNode
```

### 2. Start Master (WebSocket enabled by default!)
```bash
cd master
./masterNode
```

**Note**: The WebSocket server is **automatically enabled** on port 8080. You don't need to set `HTTP_PORT` unless you want to change the port!

You should see:
```
✓ Telemetry manager started
✓ gRPC server listening on :50051
✓ WebSocket telemetry server started on port 8080
```

### 3. Start Worker
```bash
# In a new terminal
cd worker
./workerNode
```

### 4. Connect with WebSocket Client

#### Option A: Python (Recommended)
```bash
### Option A: Python Test Client (Recommended)

```bash
# Install websockets if needed
pip3 install websockets

# Run the test client
python3 test_telemetry_websocket.py
```
```

#### Option B: Browser Console
Open your browser's developer console and paste:
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');
ws.onmessage = (e) => console.log(JSON.parse(e.data));
```

#### Option C: wscat (Node.js)
```bash
npm install -g wscat
wscat -c ws://localhost:8080/ws/telemetry
```

## WebSocket Endpoints

| Endpoint | Description |
|----------|-------------|
| `ws://localhost:8080/ws/telemetry` | Stream all workers' telemetry |
| `ws://localhost:8080/ws/telemetry/{id}` | Stream specific worker's telemetry |
| `http://localhost:8080/health` | Health check (HTTP GET) |

## Message Format

Messages are JSON objects with worker IDs as keys:

```json
{
  "worker-hostname": {
    "worker_id": "worker-hostname",
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

## Common Use Cases

### 1. Monitor All Workers
```python
import asyncio
import websockets
import json

async def monitor():
    uri = "ws://localhost:8080/ws/telemetry"
    async with websockets.connect(uri) as ws:
        async for message in ws:
            data = json.loads(message)
            for worker_id, telemetry in data.items():
                print(f"{worker_id}: CPU={telemetry['cpu_usage']:.1f}%")

asyncio.run(monitor())
```

### 2. Monitor Specific Worker
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/telemetry/worker-1');
ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    updateDashboard(data['worker-1']);
};
```

### 3. Build a Dashboard
```html
<!DOCTYPE html>
<html>
<body>
<div id="telemetry"></div>
<script>
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');
const div = document.getElementById('telemetry');

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    let html = '';
    
    for (const [workerId, telemetry] of Object.entries(data)) {
        html += `
            <div>
                <h3>${workerId}</h3>
                <p>CPU: ${telemetry.cpu_usage.toFixed(1)}%</p>
                <p>Memory: ${telemetry.memory_usage.toFixed(1)}%</p>
                <p>GPU: ${telemetry.gpu_usage.toFixed(1)}%</p>
                <p>Tasks: ${telemetry.running_tasks.length}</p>
            </div>
        `;
    }
    
    div.innerHTML = html;
};
</script>
</body>
</html>
```

## Troubleshooting

### Can't Connect to WebSocket
```bash
# Check if master is running with HTTP_PORT set
ps aux | grep masterNode

# Check if port is open
curl http://localhost:8080/health
```

### No Telemetry Updates
```bash
# Check if worker is connected
# In master CLI:
master> workers

# Check worker logs
cd worker
./workerNode
```

### High CPU Usage
- Reduce telemetry interval in `worker/internal/telemetry/telemetry.go`
- Limit number of WebSocket clients
- Use specific worker endpoints instead of all-workers endpoint

## Configuration

### Change WebSocket Port
```bash
# Option 1: Environment variable
HTTP_PORT=:9090 ./masterNode

# Option 2: .env file
echo "HTTP_PORT=:9090" >> master/.env
./masterNode
```

### Change Telemetry Interval
Edit `worker/internal/telemetry/telemetry.go`:
```go
// Change from 5 seconds to 10 seconds
monitor := telemetry.NewMonitor(workerID, 10*time.Second)
```

### Disable WebSocket Server
```bash
# Don't set HTTP_PORT
unset HTTP_PORT
./masterNode
```

## Next Steps

- Read full documentation: `docs/WEBSOCKET_TELEMETRY.md`
- See implementation details: `docs/TELEMETRY_REFACTORING_SUMMARY.md`
- Try the examples: `./test_telemetry_websocket.sh`
- Build a monitoring dashboard
- Integrate with your monitoring tools (Grafana, Prometheus, etc.)

## Support

For issues or questions, check:
1. Master logs for errors
2. Worker logs for connection issues
3. Documentation in `docs/` directory
4. Test script: `./test_telemetry_websocket.sh`
