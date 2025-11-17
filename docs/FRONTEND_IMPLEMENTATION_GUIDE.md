# CloudAI Frontend Implementation Guide

**Last Updated:** November 16, 2025  
**Branch:** vrunda/frontend  
**Purpose:** Step-by-step guide to build a beautiful, modern frontend for CloudAI distributed scheduler

---

## 📋 Table of Contents

1. [System Overview](#system-overview)
2. [Available Backend APIs](#available-backend-apis)
3. [Frontend Architecture](#frontend-architecture)
4. [Implementation Steps](#implementation-steps)
5. [Technology Stack](#technology-stack)
6. [UI/UX Design Guidelines](#uiux-design-guidelines)

---

## 🎯 System Overview

CloudAI is a distributed Docker-based task execution platform with:
- **Master Node**: Orchestrates workers, assigns tasks, collects telemetry (Go)
- **Worker Nodes**: Execute Docker containers, report status (Go)
- **MongoDB**: Persistent storage for tasks, workers, assignments, results
- **Real-time Communication**: gRPC for master-worker, WebSocket for UI updates

Your goal is to create a beautiful, responsive frontend that provides:
1. **Real-time cluster monitoring** (workers, resources, tasks)
2. **Task management** (submit, monitor, cancel tasks)
3. **Worker management** (view workers, resource allocation)
4. **Live telemetry** (CPU, Memory, GPU usage via WebSocket)
5. **Log streaming** (view real-time task execution logs)

---

## 🔌 Available Backend APIs

### Base URL
```
Master Node: http://localhost:8080
WebSocket: ws://localhost:8080
```

### REST API Endpoints

#### 1. Task Management APIs

##### **POST /api/tasks** - Submit a new task
```javascript
// Request
{
  "docker_image": "ubuntu:latest",
  "command": "echo 'Hello World'",
  "cpu_required": 1.0,
  "memory_required": 2.0,
  "gpu_required": 0.0,
  "storage_required": 5.0,
  "user_id": "user-123"
}

// Response
{
  "task_id": "task-1731753000123456789",
  "status": "queued",
  "message": "Task submitted successfully",
  "created_at": 1731753000
}
```

##### **GET /api/tasks** - List all tasks
```javascript
// Query params: ?status=running (optional)
// Response
[
  {
    "task_id": "task-1731753000123456789",
    "docker_image": "ubuntu:latest",
    "command": "echo hello",
    "status": "running",
    "user_id": "user-123",
    "cpu_required": 1.0,
    "memory_required": 2.0,
    "gpu_required": 0.0,
    "storage_required": 5.0,
    "created_at": 1731753000
  }
]
```

##### **GET /api/tasks/:id** - Get task details
```javascript
// Response
{
  "task_id": "task-123",
  "docker_image": "ubuntu:latest",
  "command": "echo hello",
  "status": "completed",
  "user_id": "user-123",
  "cpu_required": 1.0,
  "memory_required": 2.0,
  "gpu_required": 0.0,
  "storage_required": 5.0,
  "created_at": 1731753000,
  "assignment": {
    "worker_id": "worker-1",
    "assigned_at": 1731753005
  },
  "result": {
    "status": "success",
    "completed_at": 1731753120,
    "logs": "Hello World\nTask completed"
  }
}
```

##### **DELETE /api/tasks/:id** - Cancel a task
```javascript
// Response
{
  "task_id": "task-123",
  "status": "cancelled",
  "message": "Task cancellation requested"
}
```

##### **GET /api/tasks/:id/logs** - Get task logs
```javascript
// Response
{
  "task_id": "task-123",
  "logs": "Starting container...\nHello World\nTask completed",
  "status": "success",
  "completed_at": 1731753120
}
```

#### 2. Worker Management APIs

##### **GET /api/workers** - List all workers
```javascript
// Response
[
  {
    "worker_id": "worker-1",
    "is_active": true,
    "cpu_usage": 45.2,
    "memory_usage": 60.1,
    "gpu_usage": 25.5,
    "running_tasks_count": 2,
    "last_update": 1731753000
  }
]
```

##### **GET /api/workers/:id** - Get worker details
```javascript
// Response
{
  "worker_id": "worker-1",
  "is_active": true,
  "cpu_usage": 45.2,
  "memory_usage": 60.1,
  "gpu_usage": 25.5,
  "running_tasks": [
    {
      "task_id": "task-123",
      "cpu_allocated": 1.0,
      "memory_allocated": 2.0,
      "gpu_allocated": 0.0,
      "status": "running"
    }
  ],
  "last_update": 1731753000,
  "worker_info": {
    "worker_id": "worker-1",
    "worker_ip": "192.168.1.100:50052",
    "total_cpu": 8.0,
    "total_memory": 16.0,
    "total_storage": 100.0,
    "total_gpu": 2.0,
    "registered_at": 1731750000,
    "last_heartbeat": 1731753000
  }
}
```

##### **GET /api/workers/:id/tasks** - Get tasks assigned to worker
```javascript
// Response
{
  "worker_id": "worker-1",
  "tasks": [
    {
      "task_id": "task-123",
      "assigned_at": 1731753005
    }
  ],
  "count": 1
}
```

##### **GET /api/workers/:id/metrics** - Get worker metrics
```javascript
// Response
{
  "worker_id": "worker-1",
  "cpu_usage": 45.2,
  "memory_usage": 60.1,
  "gpu_usage": 25.5,
  "is_active": true,
  "last_update": 1731753000
}
```

#### 3. System Health & Telemetry APIs

##### **GET /health** - Health check
```javascript
// Response
{
  "status": "healthy",
  "time": 1731753000,
  "active_clients": 3,
  "workers": 5,
  "active_workers": 4
}
```

##### **GET /telemetry** - All workers telemetry (REST)
```javascript
// Response
{
  "worker-1": {
    "worker_id": "worker-1",
    "cpu_usage": 45.2,
    "memory_usage": 60.1,
    "gpu_usage": 25.5,
    "running_tasks": [...],
    "last_update": 1731753000,
    "is_active": true
  }
}
```

##### **GET /workers** - Basic workers info (REST)
```javascript
// Response
{
  "worker-1": {
    "worker_id": "worker-1",
    "is_active": true,
    "running_tasks_count": 2,
    "last_update": 1731753000
  }
}
```

### WebSocket Endpoints

#### **WS /ws/telemetry** - Real-time telemetry for all workers
```javascript
// Connect
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');

// Receive messages (every ~5 seconds)
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  // data = { "worker-1": { cpu_usage: 45.2, ... }, "worker-2": {...} }
};
```

#### **WS /ws/telemetry/:worker_id** - Real-time telemetry for specific worker
```javascript
// Connect
const ws = new WebSocket('ws://localhost:8080/ws/telemetry/worker-1');

// Receive messages
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  // data = { "worker-1": { cpu_usage: 45.2, ... } }
};
```

---

## 🏗️ Frontend Architecture

### Recommended Structure
```
web_interface/frontend/
├── public/
│   └── index.html
├── src/
│   ├── main.jsx                 # Entry point
│   ├── App.jsx                  # Main app component
│   ├── api/
│   │   ├── client.js           # Axios client configuration
│   │   ├── tasks.js            # Task API calls
│   │   ├── workers.js          # Worker API calls
│   │   └── websocket.js        # WebSocket connection manager
│   ├── components/
│   │   ├── layout/
│   │   │   ├── Navbar.jsx      # Top navigation
│   │   │   ├── Sidebar.jsx     # Side navigation
│   │   │   └── Layout.jsx      # Main layout wrapper
│   │   ├── dashboard/
│   │   │   ├── Dashboard.jsx   # Main dashboard
│   │   │   ├── ClusterStats.jsx # Overall cluster stats
│   │   │   ├── WorkerGrid.jsx  # Grid of worker cards
│   │   │   └── TasksOverview.jsx # Recent tasks overview
│   │   ├── workers/
│   │   │   ├── WorkersList.jsx # List all workers
│   │   │   ├── WorkerCard.jsx  # Single worker card
│   │   │   ├── WorkerDetails.jsx # Detailed worker view
│   │   │   └── ResourceChart.jsx # Resource usage charts
│   │   ├── tasks/
│   │   │   ├── TasksList.jsx   # List all tasks
│   │   │   ├── TaskCard.jsx    # Single task card
│   │   │   ├── TaskDetails.jsx # Detailed task view
│   │   │   ├── SubmitTask.jsx  # Task submission form
│   │   │   └── TaskLogs.jsx    # Live task logs viewer
│   │   └── common/
│   │       ├── StatusBadge.jsx # Status indicator
│   │       ├── LoadingSpinner.jsx
│   │       ├── ErrorAlert.jsx
│   │       └── ProgressBar.jsx
│   ├── hooks/
│   │   ├── useWebSocket.js     # WebSocket hook
│   │   ├── useTasks.js         # Tasks data hook
│   │   └── useWorkers.js       # Workers data hook
│   ├── utils/
│   │   ├── formatters.js       # Data formatters
│   │   ├── validators.js       # Input validators
│   │   └── constants.js        # Constants
│   └── styles/
│       ├── global.css          # Global styles
│       └── theme.js            # Theme configuration
├── package.json
└── vite.config.js
```

---

## 🚀 Implementation Steps

### Phase 1: Project Setup & Configuration

#### Step 1.1: Initialize the Project
```bash
cd web_interface/frontend

# If starting fresh, initialize Vite + React
npm create vite@latest . -- --template react

# Install dependencies
npm install
```

#### Step 1.2: Install Required Packages
```bash
# Core dependencies
npm install react-router-dom axios

# UI Framework (Choose one)
# Option 1: Material-UI (Recommended for this project)
npm install @mui/material @mui/icons-material @emotion/react @emotion/styled

# Option 2: Tailwind CSS + Headless UI
npm install -D tailwindcss postcss autoprefixer
npm install @headlessui/react @heroicons/react

# Charts and Visualization
npm install recharts chart.js react-chartjs-2

# State Management (optional but recommended)
npm install zustand

# Utilities
npm install date-fns clsx
```

#### Step 1.3: Configure Environment Variables
Create `.env` file:
```env
VITE_API_BASE_URL=http://localhost:8080
VITE_WS_BASE_URL=ws://localhost:8080
```

#### Step 1.4: Configure Vite
Update `vite.config.js`:
```javascript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true
      }
    }
  }
})
```

---

### Phase 2: API Layer Setup

#### Step 2.1: Create API Client (`src/api/client.js`)
```javascript
import axios from 'axios';

const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

const apiClient = axios.create({
  baseURL: BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 10000,
});

// Request interceptor
apiClient.interceptors.request.use(
  (config) => {
    // Add auth token if available
    const token = localStorage.getItem('auth_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    console.error('API Error:', error.response?.data || error.message);
    return Promise.reject(error);
  }
);

export default apiClient;
```

#### Step 2.2: Create Tasks API (`src/api/tasks.js`)
```javascript
import apiClient from './client';

export const tasksAPI = {
  // Get all tasks
  getAllTasks: (status = null) => {
    const params = status ? { status } : {};
    return apiClient.get('/api/tasks', { params });
  },

  // Get single task
  getTask: (taskId) => {
    return apiClient.get(`/api/tasks/${taskId}`);
  },

  // Submit new task
  submitTask: (taskData) => {
    return apiClient.post('/api/tasks', taskData);
  },

  // Cancel task
  cancelTask: (taskId) => {
    return apiClient.delete(`/api/tasks/${taskId}`);
  },

  // Get task logs
  getTaskLogs: (taskId) => {
    return apiClient.get(`/api/tasks/${taskId}/logs`);
  },
};
```

#### Step 2.3: Create Workers API (`src/api/workers.js`)
```javascript
import apiClient from './client';

export const workersAPI = {
  // Get all workers
  getAllWorkers: () => {
    return apiClient.get('/api/workers');
  },

  // Get single worker
  getWorker: (workerId) => {
    return apiClient.get(`/api/workers/${workerId}`);
  },

  // Get worker tasks
  getWorkerTasks: (workerId) => {
    return apiClient.get(`/api/workers/${workerId}/tasks`);
  },

  // Get worker metrics
  getWorkerMetrics: (workerId) => {
    return apiClient.get(`/api/workers/${workerId}/metrics`);
  },
};
```

#### Step 2.4: Create WebSocket Manager (`src/api/websocket.js`)
```javascript
const WS_BASE_URL = import.meta.env.VITE_WS_BASE_URL || 'ws://localhost:8080';

class WebSocketManager {
  constructor() {
    this.connections = new Map();
    this.reconnectAttempts = new Map();
    this.maxReconnectAttempts = 5;
    this.reconnectDelay = 3000;
  }

  connect(endpoint, onMessage, onError = null) {
    const url = `${WS_BASE_URL}${endpoint}`;
    
    if (this.connections.has(endpoint)) {
      console.warn(`WebSocket already connected to ${endpoint}`);
      return this.connections.get(endpoint);
    }

    const ws = new WebSocket(url);

    ws.onopen = () => {
      console.log(`WebSocket connected: ${endpoint}`);
      this.reconnectAttempts.set(endpoint, 0);
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        onMessage(data);
      } catch (error) {
        console.error('WebSocket message parse error:', error);
      }
    };

    ws.onerror = (error) => {
      console.error(`WebSocket error on ${endpoint}:`, error);
      if (onError) onError(error);
    };

    ws.onclose = () => {
      console.log(`WebSocket closed: ${endpoint}`);
      this.connections.delete(endpoint);
      this.handleReconnect(endpoint, onMessage, onError);
    };

    this.connections.set(endpoint, ws);
    return ws;
  }

  handleReconnect(endpoint, onMessage, onError) {
    const attempts = this.reconnectAttempts.get(endpoint) || 0;
    
    if (attempts < this.maxReconnectAttempts) {
      setTimeout(() => {
        console.log(`Reconnecting to ${endpoint} (attempt ${attempts + 1})`);
        this.reconnectAttempts.set(endpoint, attempts + 1);
        this.connect(endpoint, onMessage, onError);
      }, this.reconnectDelay);
    } else {
      console.error(`Max reconnection attempts reached for ${endpoint}`);
    }
  }

  disconnect(endpoint) {
    const ws = this.connections.get(endpoint);
    if (ws) {
      ws.close();
      this.connections.delete(endpoint);
      this.reconnectAttempts.delete(endpoint);
    }
  }

  disconnectAll() {
    this.connections.forEach((ws, endpoint) => {
      ws.close();
    });
    this.connections.clear();
    this.reconnectAttempts.clear();
  }
}

export const wsManager = new WebSocketManager();

// Helper functions
export const connectToAllWorkers = (onMessage, onError) => {
  return wsManager.connect('/ws/telemetry', onMessage, onError);
};

export const connectToWorker = (workerId, onMessage, onError) => {
  return wsManager.connect(`/ws/telemetry/${workerId}`, onMessage, onError);
};
```

---

### Phase 3: Custom Hooks

#### Step 3.1: Create WebSocket Hook (`src/hooks/useWebSocket.js`)
```javascript
import { useEffect, useState } from 'react';
import { wsManager } from '../api/websocket';

export const useWebSocket = (endpoint, enabled = true) => {
  const [data, setData] = useState(null);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!enabled) return;

    const handleMessage = (message) => {
      setData(message);
      setIsConnected(true);
    };

    const handleError = (err) => {
      setError(err);
      setIsConnected(false);
    };

    wsManager.connect(endpoint, handleMessage, handleError);

    return () => {
      wsManager.disconnect(endpoint);
      setIsConnected(false);
    };
  }, [endpoint, enabled]);

  return { data, isConnected, error };
};
```

#### Step 3.2: Create Tasks Hook (`src/hooks/useTasks.js`)
```javascript
import { useState, useEffect } from 'react';
import { tasksAPI } from '../api/tasks';

export const useTasks = (autoRefresh = false, refreshInterval = 5000) => {
  const [tasks, setTasks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const fetchTasks = async (status = null) => {
    try {
      setLoading(true);
      const response = await tasksAPI.getAllTasks(status);
      setTasks(response.data);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTasks();

    if (autoRefresh) {
      const interval = setInterval(fetchTasks, refreshInterval);
      return () => clearInterval(interval);
    }
  }, [autoRefresh, refreshInterval]);

  return { tasks, loading, error, refetch: fetchTasks };
};

export const useTask = (taskId) => {
  const [task, setTask] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const fetchTask = async () => {
    if (!taskId) return;
    
    try {
      setLoading(true);
      const response = await tasksAPI.getTask(taskId);
      setTask(response.data);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTask();
  }, [taskId]);

  return { task, loading, error, refetch: fetchTask };
};
```

#### Step 3.3: Create Workers Hook (`src/hooks/useWorkers.js`)
```javascript
import { useState, useEffect } from 'react';
import { workersAPI } from '../api/workers';
import { useWebSocket } from './useWebSocket';

export const useWorkers = (enableRealtime = true) => {
  const [workers, setWorkers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // WebSocket for real-time updates
  const { data: telemetryData } = useWebSocket('/ws/telemetry', enableRealtime);

  // Initial fetch from REST API
  const fetchWorkers = async () => {
    try {
      setLoading(true);
      const response = await workersAPI.getAllWorkers();
      setWorkers(response.data);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchWorkers();
  }, []);

  // Update workers with telemetry data
  useEffect(() => {
    if (telemetryData) {
      setWorkers((prevWorkers) => {
        const updatedWorkers = [...prevWorkers];
        
        Object.entries(telemetryData).forEach(([workerId, telemetry]) => {
          const index = updatedWorkers.findIndex(w => w.worker_id === workerId);
          if (index !== -1) {
            updatedWorkers[index] = {
              ...updatedWorkers[index],
              ...telemetry,
            };
          }
        });
        
        return updatedWorkers;
      });
    }
  }, [telemetryData]);

  return { workers, loading, error, refetch: fetchWorkers };
};

export const useWorker = (workerId, enableRealtime = true) => {
  const [worker, setWorker] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // WebSocket for specific worker
  const { data: telemetryData } = useWebSocket(
    `/ws/telemetry/${workerId}`,
    enableRealtime && !!workerId
  );

  const fetchWorker = async () => {
    if (!workerId) return;

    try {
      setLoading(true);
      const response = await workersAPI.getWorker(workerId);
      setWorker(response.data);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchWorker();
  }, [workerId]);

  // Update worker with telemetry
  useEffect(() => {
    if (telemetryData && telemetryData[workerId]) {
      setWorker((prev) => ({
        ...prev,
        ...telemetryData[workerId],
      }));
    }
  }, [telemetryData, workerId]);

  return { worker, loading, error, refetch: fetchWorker };
};
```

---

### Phase 4: Utility Functions

#### Step 4.1: Create Formatters (`src/utils/formatters.js`)
```javascript
import { formatDistanceToNow } from 'date-fns';

// Format bytes to human-readable
export const formatBytes = (bytes, decimals = 2) => {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + ' ' + sizes[i];
};

// Format GB to readable
export const formatGB = (gb, decimals = 2) => {
  return `${gb.toFixed(decimals)} GB`;
};

// Format CPU cores
export const formatCPU = (cores, decimals = 2) => {
  return `${cores.toFixed(decimals)} cores`;
};

// Format percentage
export const formatPercentage = (value, decimals = 1) => {
  return `${value.toFixed(decimals)}%`;
};

// Format timestamp to relative time
export const formatRelativeTime = (timestamp) => {
  if (!timestamp) return 'N/A';
  const date = typeof timestamp === 'number' 
    ? new Date(timestamp * 1000) 
    : new Date(timestamp);
  return formatDistanceToNow(date, { addSuffix: true });
};

// Format duration in seconds
export const formatDuration = (seconds) => {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${hours}h ${minutes}m`;
};

// Format task status
export const getStatusColor = (status) => {
  const colors = {
    pending: 'warning',
    queued: 'info',
    running: 'primary',
    completed: 'success',
    failed: 'error',
    cancelled: 'default',
  };
  return colors[status] || 'default';
};

// Format worker status
export const getWorkerStatusColor = (isActive) => {
  return isActive ? 'success' : 'error';
};

// Format resource usage color (based on percentage)
export const getUsageColor = (percentage) => {
  if (percentage < 50) return 'success';
  if (percentage < 80) return 'warning';
  return 'error';
};
```

#### Step 4.2: Create Constants (`src/utils/constants.js`)
```javascript
// Task statuses
export const TASK_STATUS = {
  PENDING: 'pending',
  QUEUED: 'queued',
  RUNNING: 'running',
  COMPLETED: 'completed',
  FAILED: 'failed',
  CANCELLED: 'cancelled',
};

// Refresh intervals (ms)
export const REFRESH_INTERVALS = {
  FAST: 2000,      // 2 seconds
  MEDIUM: 5000,    // 5 seconds
  SLOW: 10000,     // 10 seconds
};

// Resource limits
export const RESOURCE_LIMITS = {
  MIN_CPU: 0.1,
  MAX_CPU: 64,
  MIN_MEMORY: 0.5,
  MAX_MEMORY: 256,
  MIN_STORAGE: 1,
  MAX_STORAGE: 1000,
  MIN_GPU: 0,
  MAX_GPU: 8,
};

// Chart colors
export const CHART_COLORS = {
  CPU: '#3b82f6',      // blue
  MEMORY: '#10b981',   // green
  GPU: '#f59e0b',      // amber
  STORAGE: '#8b5cf6',  // purple
};

// Status icons (Material-UI)
export const STATUS_ICONS = {
  pending: 'HourglassEmpty',
  queued: 'Queue',
  running: 'PlayArrow',
  completed: 'CheckCircle',
  failed: 'Error',
  cancelled: 'Cancel',
};
```

---

### Phase 5: Core Components - Layout

#### Step 5.1: Create Layout Component (`src/components/layout/Layout.jsx`)
```javascript
import React from 'react';
import { Box, CssBaseline } from '@mui/material';
import Navbar from './Navbar';
import Sidebar from './Sidebar';

const Layout = ({ children }) => {
  const [sidebarOpen, setSidebarOpen] = React.useState(true);

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <CssBaseline />
      <Navbar onMenuClick={() => setSidebarOpen(!sidebarOpen)} />
      <Sidebar open={sidebarOpen} />
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          p: 3,
          mt: 8,
          ml: sidebarOpen ? '240px' : 0,
          transition: 'margin 0.3s',
        }}
      >
        {children}
      </Box>
    </Box>
  );
};

export default Layout;
```

#### Step 5.2: Create Navbar Component (`src/components/layout/Navbar.jsx`)
```javascript
import React from 'react';
import {
  AppBar,
  Toolbar,
  Typography,
  IconButton,
  Badge,
  Box,
} from '@mui/material';
import {
  Menu as MenuIcon,
  Notifications as NotificationsIcon,
  Settings as SettingsIcon,
} from '@mui/icons-material';

const Navbar = ({ onMenuClick }) => {
  return (
    <AppBar position="fixed" sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}>
      <Toolbar>
        <IconButton
          color="inherit"
          edge="start"
          onClick={onMenuClick}
          sx={{ mr: 2 }}
        >
          <MenuIcon />
        </IconButton>
        
        <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
          CloudAI Scheduler
        </Typography>

        <Box sx={{ display: 'flex', gap: 1 }}>
          <IconButton color="inherit">
            <Badge badgeContent={4} color="error">
              <NotificationsIcon />
            </Badge>
          </IconButton>
          <IconButton color="inherit">
            <SettingsIcon />
          </IconButton>
        </Box>
      </Toolbar>
    </AppBar>
  );
};

export default Navbar;
```

#### Step 5.3: Create Sidebar Component (`src/components/layout/Sidebar.jsx`)
```javascript
import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import {
  Drawer,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Toolbar,
  Divider,
} from '@mui/material';
import {
  Dashboard as DashboardIcon,
  Assignment as TaskIcon,
  Computer as WorkerIcon,
  AddCircle as AddIcon,
  Storage as StorageIcon,
} from '@mui/icons-material';

const menuItems = [
  { text: 'Dashboard', icon: <DashboardIcon />, path: '/' },
  { text: 'Tasks', icon: <TaskIcon />, path: '/tasks' },
  { text: 'Submit Task', icon: <AddIcon />, path: '/tasks/submit' },
  { text: 'Workers', icon: <WorkerIcon />, path: '/workers' },
  { text: 'Storage', icon: <StorageIcon />, path: '/storage' },
];

const Sidebar = ({ open }) => {
  const navigate = useNavigate();
  const location = useLocation();

  return (
    <Drawer
      variant="persistent"
      anchor="left"
      open={open}
      sx={{
        width: 240,
        flexShrink: 0,
        '& .MuiDrawer-paper': {
          width: 240,
          boxSizing: 'border-box',
        },
      }}
    >
      <Toolbar />
      <Divider />
      <List>
        {menuItems.map((item) => (
          <ListItem key={item.text} disablePadding>
            <ListItemButton
              selected={location.pathname === item.path}
              onClick={() => navigate(item.path)}
            >
              <ListItemIcon>{item.icon}</ListItemIcon>
              <ListItemText primary={item.text} />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </Drawer>
  );
};

export default Sidebar;
```

---

### Phase 6: Dashboard Components

#### Step 6.1: Create Main Dashboard (`src/components/dashboard/Dashboard.jsx`)
```javascript
import React from 'react';
import { Grid, Paper, Typography, Box } from '@mui/material';
import ClusterStats from './ClusterStats';
import WorkerGrid from './WorkerGrid';
import TasksOverview from './TasksOverview';
import { useWorkers } from '../../hooks/useWorkers';
import { useTasks } from '../../hooks/useTasks';

const Dashboard = () => {
  const { workers, loading: workersLoading } = useWorkers(true);
  const { tasks, loading: tasksLoading } = useTasks(true, 5000);

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Cluster Dashboard
      </Typography>

      {/* Cluster Statistics */}
      <ClusterStats workers={workers} tasks={tasks} />

      {/* Workers Grid */}
      <Box sx={{ mt: 3 }}>
        <Typography variant="h5" gutterBottom>
          Workers
        </Typography>
        <WorkerGrid workers={workers} loading={workersLoading} />
      </Box>

      {/* Recent Tasks */}
      <Box sx={{ mt: 3 }}>
        <Typography variant="h5" gutterBottom>
          Recent Tasks
        </Typography>
        <TasksOverview tasks={tasks.slice(0, 5)} loading={tasksLoading} />
      </Box>
    </Box>
  );
};

export default Dashboard;
```

#### Step 6.2: Create Cluster Stats (`src/components/dashboard/ClusterStats.jsx`)
```javascript
import React from 'react';
import { Grid, Paper, Typography, Box } from '@mui/material';
import {
  Computer as ComputerIcon,
  Assignment as TaskIcon,
  CheckCircle as CompleteIcon,
  Error as ErrorIcon,
} from '@mui/icons-material';

const StatCard = ({ title, value, icon, color = 'primary' }) => (
  <Paper sx={{ p: 2, display: 'flex', alignItems: 'center' }}>
    <Box
      sx={{
        backgroundColor: `${color}.light`,
        borderRadius: 2,
        p: 1.5,
        mr: 2,
      }}
    >
      {React.cloneElement(icon, { sx: { color: `${color}.main`, fontSize: 32 } })}
    </Box>
    <Box>
      <Typography variant="h4">{value}</Typography>
      <Typography variant="body2" color="text.secondary">
        {title}
      </Typography>
    </Box>
  </Paper>
);

const ClusterStats = ({ workers, tasks }) => {
  const activeWorkers = workers.filter(w => w.is_active).length;
  const runningTasks = tasks.filter(t => t.status === 'running').length;
  const completedTasks = tasks.filter(t => t.status === 'completed').length;
  const failedTasks = tasks.filter(t => t.status === 'failed').length;

  return (
    <Grid container spacing={3}>
      <Grid item xs={12} sm={6} md={3}>
        <StatCard
          title="Active Workers"
          value={`${activeWorkers}/${workers.length}`}
          icon={<ComputerIcon />}
          color="primary"
        />
      </Grid>
      <Grid item xs={12} sm={6} md={3}>
        <StatCard
          title="Running Tasks"
          value={runningTasks}
          icon={<TaskIcon />}
          color="info"
        />
      </Grid>
      <Grid item xs={12} sm={6} md={3}>
        <StatCard
          title="Completed"
          value={completedTasks}
          icon={<CompleteIcon />}
          color="success"
        />
      </Grid>
      <Grid item xs={12} sm={6} md={3}>
        <StatCard
          title="Failed"
          value={failedTasks}
          icon={<ErrorIcon />}
          color="error"
        />
      </Grid>
    </Grid>
  );
};

export default ClusterStats;
```

---

