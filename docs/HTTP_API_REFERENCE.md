# CloudAI HTTP API Reference

## Overview

CloudAI provides a REST API for programmatic access to cluster management and task operations. The API runs on the master node and provides endpoints for task submission, monitoring, and real-time telemetry via WebSocket.

## Base URL
```
http://master-node:8080
```

## Authentication

Currently, the API does not require authentication. All endpoints are publicly accessible.

## Endpoints

### Task Management

#### Submit Task
Submit a new task for execution in the cluster.

**Endpoint:** `POST /api/tasks`

**Request Body:**
```json
{
  "user_id": "user-123",
  "docker_image": "docker.io/library/ubuntu:latest",
  "command": "echo 'Hello World'",
  "req_cpu": 1.0,
  "req_memory": 2.0,
  "req_storage": 5.0,
  "req_gpu": 0.0
}
```

**Response:**
```json
{
  "task_id": "task-1731753000123456789",
  "status": "queued",
  "message": "Task submitted successfully"
}
```

**Status Codes:**
- `200` - Task submitted successfully
- `400` - Invalid request parameters
- `500` - Internal server error

#### Get Task Status
Retrieve the current status of a specific task.

**Endpoint:** `GET /api/tasks/{task_id}`

**Response:**
```json
{
  "task_id": "task-1731753000123456789",
  "user_id": "user-123",
  "docker_image": "docker.io/library/ubuntu:latest",
  "command": "echo 'Hello World'",
  "req_cpu": 1.0,
  "req_memory": 2.0,
  "req_storage": 5.0,
  "req_gpu": 0.0,
  "status": "running",
  "created_at": "2025-11-16T10:30:00Z",
  "started_at": "2025-11-16T10:30:05Z",
  "worker_id": "worker-1"
}
```

**Status Codes:**
- `200` - Task found
- `404` - Task not found
- `500` - Internal server error

#### List Tasks
Retrieve a list of tasks with optional filtering.

**Endpoint:** `GET /api/tasks`

**Query Parameters:**
- `status` - Filter by task status (pending, queued, running, completed, failed, cancelled)
- `user_id` - Filter by user ID
- `limit` - Maximum number of tasks to return (default: 50)
- `offset` - Number of tasks to skip (default: 0)

**Response:**
```json
{
  "tasks": [
    {
      "task_id": "task-1731753000123456789",
      "user_id": "user-123",
      "status": "running",
      "created_at": "2025-11-16T10:30:00Z",
      "worker_id": "worker-1"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

**Status Codes:**
- `200` - Success
- `400` - Invalid query parameters
- `500` - Internal server error

#### Cancel Task
Cancel a running or queued task.

**Endpoint:** `POST /api/tasks/{task_id}/cancel`

**Response:**
```json
{
  "task_id": "task-1731753000123456789",
  "status": "cancelled",
  "message": "Task cancellation requested"
}
```

**Status Codes:**
- `200` - Cancellation requested
- `404` - Task not found
- `409` - Task cannot be cancelled (already completed/failed)
- `500` - Internal server error

### Worker Management

#### List Workers
Retrieve information about all registered workers.

**Endpoint:** `GET /api/workers`

**Response:**
```json
{
  "workers": [
    {
      "worker_id": "worker-1",
      "worker_ip": "192.168.1.100:50052",
      "total_cpu": 8.0,
      "total_memory": 16.0,
      "total_storage": 100.0,
      "total_gpu": 2.0,
      "allocated_cpu": 2.0,
      "allocated_memory": 4.0,
      "allocated_storage": 20.0,
      "allocated_gpu": 1.0,
      "available_cpu": 6.0,
      "available_memory": 12.0,
      "available_storage": 80.0,
      "available_gpu": 1.0,
      "is_active": true,
      "last_heartbeat": 1731753000,
      "registered_at": "2025-11-16T09:00:00Z"
    }
  ]
}
```

**Status Codes:**
- `200` - Success
- `500` - Internal server error

#### Get Worker Details
Retrieve detailed information about a specific worker.

**Endpoint:** `GET /api/workers/{worker_id}`

**Response:**
```json
{
  "worker_id": "worker-1",
  "worker_ip": "192.168.1.100:50052",
  "total_cpu": 8.0,
  "total_memory": 16.0,
  "total_storage": 100.0,
  "total_gpu": 2.0,
  "allocated_cpu": 2.0,
  "allocated_memory": 4.0,
  "allocated_storage": 20.0,
  "allocated_gpu": 1.0,
  "available_cpu": 6.0,
  "available_memory": 12.0,
  "available_storage": 80.0,
  "available_gpu": 1.0,
  "is_active": true,
  "last_heartbeat": 1731753000,
  "registered_at": "2025-11-16T09:00:00Z",
  "running_tasks": [
    {
      "task_id": "task-1731753000123456789",
      "user_id": "user-123",
      "started_at": "2025-11-16T10:30:05Z"
    }
  ]
}
```

**Status Codes:**
- `200` - Worker found
- `404` - Worker not found
- `500` - Internal server error

### Cluster Status

#### Get Cluster Overview
Retrieve high-level cluster statistics and status.

**Endpoint:** `GET /api/cluster/status`

**Response:**
```json
{
  "total_workers": 3,
  "active_workers": 3,
  "total_cpu": 24.0,
  "total_memory": 48.0,
  "total_storage": 300.0,
  "total_gpu": 6.0,
  "available_cpu": 18.0,
  "available_memory": 36.0,
  "available_storage": 240.0,
  "available_gpu": 4.0,
  "total_tasks": 15,
  "running_tasks": 3,
  "queued_tasks": 2,
  "completed_tasks": 10
}
```

**Status Codes:**
- `200` - Success
- `500` - Internal server error

## WebSocket Telemetry

### Real-time Telemetry Stream
Connect to the WebSocket endpoint for real-time cluster telemetry.

**Endpoint:** `ws://master-node:8080/ws/telemetry`

**Message Format:**
The server sends JSON messages containing real-time updates:

```json
{
  "type": "telemetry_update",
  "timestamp": "2025-11-16T10:30:00Z",
  "data": {
    "workers": [
      {
        "worker_id": "worker-1",
        "cpu_usage": 45.2,
        "memory_usage": 60.1,
        "active_tasks": 2,
        "last_heartbeat": 1731753000
      }
    ],
    "tasks": [
      {
        "task_id": "task-1731753000123456789",
        "status": "running",
        "worker_id": "worker-1",
        "progress": 75.0
      }
    ],
    "cluster": {
      "total_workers": 3,
      "active_workers": 3,
      "available_cpu": 18.0,
      "available_memory": 36.0
    }
  }
}
```

**Supported Message Types:**
- `telemetry_update` - Periodic telemetry data (every 5 seconds)
- `task_started` - A task has started execution
- `task_completed` - A task has completed
- `worker_registered` - A new worker has registered
- `worker_disconnected` - A worker has disconnected

## Error Handling

All API endpoints return errors in the following format:

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Invalid request parameters",
    "details": {
      "field": "req_cpu",
      "reason": "must be greater than 0"
    }
  }
}
```

## Rate Limiting

- Task submission: 10 requests per minute per IP
- Status queries: 60 requests per minute per IP
- WebSocket connections: 5 concurrent connections per IP

## Examples

### Submit a Task (curl)
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "docker_image": "ubuntu:latest",
    "command": "echo Hello World",
    "req_cpu": 1.0,
    "req_memory": 2.0,
    "req_storage": 5.0,
    "req_gpu": 0.0
  }'
```

### Monitor Tasks (curl)
```bash
curl http://localhost:8080/api/tasks?status=running
```

### WebSocket Connection (JavaScript)
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Telemetry:', data);
};
```

---

**API Version:** 1.0  
**Last Updated:** November 16, 2025