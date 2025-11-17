# CloudAI Frontend - Quick Reference Card

**Keep this handy while coding! 📌**

---

## 🚀 Quick Start (Copy-Paste Commands)

```bash
# Setup
cd web_interface/frontend
npm install

# Development
npm run dev              # Start dev server on port 3000

# Build
npm run build           # Production build
npm run preview         # Preview production build

# Backend (separate terminals)
cd database && docker-compose up          # Terminal 1
cd master && go run main.go               # Terminal 2
cd worker && go run main.go               # Terminal 3
```

---

## 📡 API Endpoints (Port 8080)

### Tasks
```
POST   /api/tasks           → Submit task
GET    /api/tasks           → List all (filter: ?status=running)
GET    /api/tasks/:id       → Task details
DELETE /api/tasks/:id       → Cancel task
GET    /api/tasks/:id/logs  → Get logs
```

### Workers
```
GET    /api/workers          → List all
GET    /api/workers/:id      → Worker details
GET    /api/workers/:id/tasks → Worker tasks
```

### WebSocket
```
WS     /ws/telemetry         → All workers (real-time)
WS     /ws/telemetry/:id     → Specific worker
```

---

## 🎯 Core Files & Their Purpose

```
src/
├── api/
│   ├── client.js         → Axios setup + interceptors
│   ├── tasks.js          → Task CRUD operations
│   ├── workers.js        → Worker operations
│   └── websocket.js      → WebSocket manager
│
├── hooks/
│   ├── useWebSocket.js   → Real-time data hook
│   ├── useTasks.js       → Tasks data + operations
│   └── useWorkers.js     → Workers data + operations
│
├── components/
│   ├── layout/           → Navbar, Sidebar, Layout
│   ├── dashboard/        → Main dashboard components
│   ├── tasks/            → Task management UI
│   ├── workers/          → Worker monitoring UI
│   └── common/           → Reusable components
│
└── utils/
    ├── formatters.js     → Data formatting functions
    └── constants.js      → App constants
```

---

## 🔧 Common Code Patterns

### 1. Fetch Data with Loading/Error States
```javascript
const [data, setData] = useState([]);
const [loading, setLoading] = useState(true);
const [error, setError] = useState(null);

useEffect(() => {
  const fetchData = async () => {
    try {
      const response = await tasksAPI.getAllTasks();
      setData(response.data);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };
  fetchData();
}, []);
```

### 2. WebSocket Real-time Updates
```javascript
const { data, isConnected } = useWebSocket('/ws/telemetry');

useEffect(() => {
  if (data) {
    // data = { "worker-1": {...}, "worker-2": {...} }
    setWorkers(Object.values(data));
  }
}, [data]);
```

### 3. Form Submission
```javascript
const handleSubmit = async (e) => {
  e.preventDefault();
  setLoading(true);
  try {
    const response = await tasksAPI.submitTask(formData);
    navigate(`/tasks/${response.data.task_id}`);
  } catch (err) {
    setError(err.message);
  } finally {
    setLoading(false);
  }
};
```

### 4. Navigate Between Pages
```javascript
import { useNavigate } from 'react-router-dom';

const navigate = useNavigate();

// Navigate to task details
navigate(`/tasks/${taskId}`);

// Go back
navigate(-1);

// Navigate with state
navigate('/tasks', { state: { filter: 'running' } });
```

---

## 🎨 Material-UI Quick Reference

### Common Components
```javascript
import {
  Box, Card, CardContent,      // Layout
  Typography, Button, Chip,     // Basic
  TextField, Select, Slider,    // Form
  Grid, Paper, Divider,         // Structure
  Table, TableBody, TableCell,  // Tables
  Alert, CircularProgress,      // Feedback
  IconButton, Badge,            // Actions
} from '@mui/material';

import {
  Dashboard, Computer,          // Icons
  Assignment, CheckCircle,
  Error, PlayArrow,
} from '@mui/icons-material';
```

### Styling Pattern
```javascript
<Box sx={{ 
  display: 'flex',
  gap: 2,
  p: 3,                        // padding: 24px
  mt: 2,                       // marginTop: 16px
  bgcolor: 'primary.light',
  borderRadius: 2,
}}>
```

---

## 📊 Data Formats

### Task Object
```javascript
{
  task_id: "task-1731753000123456789",
  docker_image: "ubuntu:latest",
  command: "echo hello",
  status: "running",  // pending|queued|running|completed|failed
  cpu_required: 1.0,
  memory_required: 2.0,
  gpu_required: 0.0,
  storage_required: 5.0,
  user_id: "user-123",
  created_at: 1731753000,
  assignment: {
    worker_id: "worker-1",
    assigned_at: 1731753005
  },
  result: {
    status: "success",
    logs: "...",
    completed_at: 1731753120
  }
}
```

### Worker Object
```javascript
{
  worker_id: "worker-1",
  is_active: true,
  cpu_usage: 45.2,        // percentage
  memory_usage: 60.1,
  gpu_usage: 25.5,
  running_tasks_count: 2,
  last_update: 1731753000,
  worker_info: {
    worker_ip: "192.168.1.100:50052",
    total_cpu: 8.0,
    total_memory: 16.0,
    total_storage: 100.0,
    total_gpu: 2.0
  },
  running_tasks: [
    {
      task_id: "task-123",
      cpu_allocated: 1.0,
      memory_allocated: 2.0,
      status: "running"
    }
  ]
}
```

### WebSocket Message
```javascript
{
  "worker-1": {
    worker_id: "worker-1",
    cpu_usage: 45.2,
    memory_usage: 60.1,
    gpu_usage: 25.5,
    running_tasks: [...],
    last_update: 1731753000,
    is_active: true
  },
  "worker-2": { ... }
}
```

---

## 🛠️ Utility Functions

### Formatters (import from `utils/formatters.js`)
```javascript
formatBytes(1024)              → "1.00 KB"
formatGB(2.5)                  → "2.50 GB"
formatCPU(1.5)                 → "1.50 cores"
formatPercentage(45.267)       → "45.3%"
formatRelativeTime(timestamp)  → "2 mins ago"
formatDuration(125)            → "2m 5s"
getStatusColor(status)         → "success" | "error" | "warning"
getUsageColor(percentage)      → "success" | "warning" | "error"
```

### Constants (import from `utils/constants.js`)
```javascript
TASK_STATUS.PENDING            → "pending"
TASK_STATUS.RUNNING            → "running"
TASK_STATUS.COMPLETED          → "completed"

REFRESH_INTERVALS.FAST         → 2000 (ms)
REFRESH_INTERVALS.MEDIUM       → 5000
REFRESH_INTERVALS.SLOW         → 10000

RESOURCE_LIMITS.MIN_CPU        → 0.1
RESOURCE_LIMITS.MAX_CPU        → 64
RESOURCE_LIMITS.MIN_MEMORY     → 0.5
RESOURCE_LIMITS.MAX_MEMORY     → 256

CHART_COLORS.CPU               → "#3b82f6"
CHART_COLORS.MEMORY            → "#10b981"
CHART_COLORS.GPU               → "#f59e0b"
```

---

## 🔍 Debugging Tips

### Check WebSocket Connection
```javascript
// In browser console
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');
ws.onopen = () => console.log('✅ Connected');
ws.onmessage = (e) => console.log('📩', JSON.parse(e.data));
ws.onerror = (e) => console.error('❌', e);
```

### Test API Endpoints
```bash
# Health check
curl http://localhost:8080/health

# Get workers
curl http://localhost:8080/api/workers | jq

# Submit task
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"docker_image":"ubuntu:latest","cpu_required":1,"memory_required":2}'
```

### React DevTools
```javascript
// Install React DevTools browser extension
// View component tree, props, state, hooks

// Add to component for debugging
useEffect(() => {
  console.log('Component mounted:', { workers, tasks });
}, [workers, tasks]);
```

### Network Tab
```
1. Open Chrome DevTools (F12)
2. Go to Network tab
3. Filter: WS (WebSocket) or XHR (API calls)
4. Click on request to see details
```

---

## ⚠️ Common Errors & Fixes

### Error: WebSocket connection failed
```
Cause: Master node not running or wrong port
Fix: Verify master is running on port 8080
  → curl http://localhost:8080/health
```

### Error: CORS policy blocking request
```
Cause: Backend not allowing origin
Fix: Backend already allows all origins
     If still failing, add proxy in vite.config.js
```

### Error: Cannot read property of undefined
```
Cause: Data not loaded yet
Fix: Add optional chaining
  → task?.assignment?.worker_id
  → {task && <TaskDetails task={task} />}
```

### Error: Too many re-renders
```
Cause: Setting state in render or wrong useEffect deps
Fix: 
  → Move state updates to event handlers
  → Add proper dependencies to useEffect
  → Use useCallback for event handlers
```

---

## 📦 Package Versions (Reference)

```json
{
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.14.0",
    "axios": "^1.5.0",
    "@mui/material": "^5.14.0",
    "@mui/icons-material": "^5.14.0",
    "@emotion/react": "^11.11.0",
    "@emotion/styled": "^11.11.0",
    "recharts": "^2.8.0",
    "date-fns": "^2.30.0",
    "clsx": "^2.0.0"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.0.0",
    "vite": "^5.0.0"
  }
}
```

---

## 🎯 Performance Checklist

- [ ] Use React.memo() for expensive components
- [ ] Implement proper useEffect dependencies
- [ ] Debounce search inputs (300ms)
- [ ] Lazy load route components
- [ ] Close WebSocket on unmount
- [ ] Avoid creating objects in render
- [ ] Use key prop in lists
- [ ] Optimize bundle size
- [ ] Enable gzip compression
- [ ] Cache API responses

---

## 🔐 Security Checklist

- [ ] Validate all user inputs
- [ ] Sanitize data before rendering
- [ ] Use HTTPS in production
- [ ] Implement authentication
- [ ] Add CSRF protection
- [ ] Restrict CORS origins
- [ ] Use WSS for WebSocket
- [ ] Don't expose sensitive data
- [ ] Add rate limiting
- [ ] Log security events

---

## 📱 Browser Support

```
✅ Chrome 90+
✅ Firefox 88+
✅ Safari 14+
✅ Edge 90+
❌ IE 11 (not supported)
```

---

## 🔗 Helpful Links

- **React Docs:** https://react.dev
- **MUI Docs:** https://mui.com
- **Vite Docs:** https://vitejs.dev
- **Recharts:** https://recharts.org
- **MDN WebSocket:** https://developer.mozilla.org/en-US/docs/Web/API/WebSocket

---

## 💡 Pro Tips

1. **Use React DevTools** - Essential for debugging
2. **Keep components small** - Max 200 lines per file
3. **Extract repeated logic** - Create custom hooks
4. **Test WebSocket early** - Don't wait until deployment
5. **Use TypeScript** - Consider for type safety (optional)
6. **Mobile-first design** - Start with mobile layout
7. **Git commit often** - Save your progress
8. **Read error messages** - They usually tell you the fix
9. **Use console.log liberally** - Debug as you go
10. **Ask for help** - Check Stack Overflow, Discord

---

**Happy Coding! 🎉**

Print this page and keep it next to your keyboard!

