# CloudAI Frontend Implementation Guide - Part 3 (Final)

**Continuation of:** FRONTEND_IMPLEMENTATION_GUIDE_PART2.md

---

## Phase 9: Worker Details & Charts

### Step 9.1: Create Worker Details Page (`src/components/workers/WorkerDetails.jsx`)
```javascript
import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Box,
  Paper,
  Typography,
  Button,
  Grid,
  Card,
  CardContent,
  Divider,
  Chip,
  List,
  ListItem,
  ListItemText,
} from '@mui/material';
import { ArrowBack as BackIcon } from '@mui/icons-material';
import { useWorker } from '../../hooks/useWorkers';
import ResourceChart from './ResourceChart';
import { formatGB, formatCPU, formatRelativeTime } from '../../utils/formatters';

const WorkerDetails = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const { worker, loading, error } = useWorker(id, true);

  if (loading) return <Typography>Loading worker details...</Typography>;
  if (error) return <Typography color="error">Error: {error}</Typography>;
  if (!worker) return <Typography>Worker not found</Typography>;

  return (
    <Box>
      {/* Header */}
      <Box sx={{ mb: 3, display: 'flex', alignItems: 'center', gap: 2 }}>
        <Button startIcon={<BackIcon />} onClick={() => navigate('/workers')}>
          Back
        </Button>
        <Typography variant="h4">{worker.worker_id}</Typography>
        <Chip
          label={worker.is_active ? 'Active' : 'Offline'}
          color={worker.is_active ? 'success' : 'error'}
        />
      </Box>

      <Grid container spacing={3}>
        {/* Worker Information */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Worker Information
              </Typography>
              <Divider sx={{ mb: 2 }} />
              
              {worker.worker_info && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  <Typography><strong>Worker ID:</strong> {worker.worker_info.worker_id}</Typography>
                  <Typography><strong>IP Address:</strong> {worker.worker_info.worker_ip}</Typography>
                  <Typography><strong>Registered:</strong> {formatRelativeTime(worker.worker_info.registered_at)}</Typography>
                  <Typography><strong>Last Heartbeat:</strong> {formatRelativeTime(worker.worker_info.last_heartbeat)}</Typography>
                </Box>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* Resource Capacity */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Resource Capacity
              </Typography>
              <Divider sx={{ mb: 2 }} />
              
              {worker.worker_info && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  <Typography><strong>CPU:</strong> {formatCPU(worker.worker_info.total_cpu)}</Typography>
                  <Typography><strong>Memory:</strong> {formatGB(worker.worker_info.total_memory)}</Typography>
                  <Typography><strong>Storage:</strong> {formatGB(worker.worker_info.total_storage)}</Typography>
                  {worker.worker_info.total_gpu > 0 && (
                    <Typography><strong>GPU:</strong> {worker.worker_info.total_gpu} cores</Typography>
                  )}
                </Box>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* Resource Usage Charts */}
        <Grid item xs={12}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Real-time Resource Usage
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <ResourceChart worker={worker} />
            </CardContent>
          </Card>
        </Grid>

        {/* Running Tasks */}
        <Grid item xs={12}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Running Tasks ({worker.running_tasks?.length || 0})
              </Typography>
              <Divider sx={{ mb: 2 }} />
              
              {worker.running_tasks && worker.running_tasks.length > 0 ? (
                <List>
                  {worker.running_tasks.map((task) => (
                    <ListItem
                      key={task.task_id}
                      sx={{
                        cursor: 'pointer',
                        '&:hover': { bgcolor: 'action.hover' },
                      }}
                      onClick={() => navigate(`/tasks/${task.task_id}`)}
                    >
                      <ListItemText
                        primary={task.task_id}
                        secondary={`CPU: ${task.cpu_allocated} | Memory: ${task.memory_allocated}GB | GPU: ${task.gpu_allocated}`}
                      />
                      <Chip label={task.status} size="small" color="primary" />
                    </ListItem>
                  ))}
                </List>
              ) : (
                <Typography color="text.secondary">No running tasks</Typography>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </Box>
  );
};

export default WorkerDetails;
```

### Step 9.2: Create Resource Chart (`src/components/workers/ResourceChart.jsx`)
```javascript
import React, { useState, useEffect } from 'react';
import { Box } from '@mui/material';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import { CHART_COLORS } from '../../utils/constants';

const ResourceChart = ({ worker }) => {
  const [data, setData] = useState([]);
  const maxDataPoints = 20;

  useEffect(() => {
    if (!worker) return;

    const timestamp = new Date().toLocaleTimeString();
    const newDataPoint = {
      time: timestamp,
      cpu: worker.cpu_usage || 0,
      memory: worker.memory_usage || 0,
      gpu: worker.gpu_usage || 0,
    };

    setData((prevData) => {
      const updated = [...prevData, newDataPoint];
      return updated.slice(-maxDataPoints);
    });
  }, [worker?.cpu_usage, worker?.memory_usage, worker?.gpu_usage]);

  return (
    <Box sx={{ width: '100%', height: 300 }}>
      <ResponsiveContainer>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="time" />
          <YAxis domain={[0, 100]} label={{ value: '%', angle: -90, position: 'insideLeft' }} />
          <Tooltip />
          <Legend />
          <Line
            type="monotone"
            dataKey="cpu"
            stroke={CHART_COLORS.CPU}
            name="CPU %"
            strokeWidth={2}
            dot={false}
          />
          <Line
            type="monotone"
            dataKey="memory"
            stroke={CHART_COLORS.MEMORY}
            name="Memory %"
            strokeWidth={2}
            dot={false}
          />
          {worker?.gpu_usage > 0 && (
            <Line
              type="monotone"
              dataKey="gpu"
              stroke={CHART_COLORS.GPU}
              name="GPU %"
              strokeWidth={2}
              dot={false}
            />
          )}
        </LineChart>
      </ResponsiveContainer>
    </Box>
  );
};

export default ResourceChart;
```

---

## Phase 10: Common Components

### Step 10.1: Create Status Badge (`src/components/common/StatusBadge.jsx`)
```javascript
import React from 'react';
import { Chip } from '@mui/material';
import {
  HourglassEmpty as PendingIcon,
  Queue as QueueIcon,
  PlayArrow as RunningIcon,
  CheckCircle as CompletedIcon,
  Error as ErrorIcon,
  Cancel as CancelledIcon,
} from '@mui/icons-material';

const STATUS_CONFIG = {
  pending: { icon: <PendingIcon />, color: 'warning' },
  queued: { icon: <QueueIcon />, color: 'info' },
  running: { icon: <RunningIcon />, color: 'primary' },
  completed: { icon: <CompletedIcon />, color: 'success' },
  failed: { icon: <ErrorIcon />, color: 'error' },
  cancelled: { icon: <CancelledIcon />, color: 'default' },
};

const StatusBadge = ({ status }) => {
  const config = STATUS_CONFIG[status] || STATUS_CONFIG.pending;
  
  return (
    <Chip
      icon={config.icon}
      label={status}
      color={config.color}
      size="small"
    />
  );
};

export default StatusBadge;
```

### Step 10.2: Create Loading Spinner (`src/components/common/LoadingSpinner.jsx`)
```javascript
import React from 'react';
import { Box, CircularProgress, Typography } from '@mui/material';

const LoadingSpinner = ({ message = 'Loading...' }) => {
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '200px',
        gap: 2,
      }}
    >
      <CircularProgress />
      <Typography color="text.secondary">{message}</Typography>
    </Box>
  );
};

export default LoadingSpinner;
```

### Step 10.3: Create Error Alert (`src/components/common/ErrorAlert.jsx`)
```javascript
import React from 'react';
import { Alert, AlertTitle, Button } from '@mui/material';
import { Refresh as RefreshIcon } from '@mui/icons-material';

const ErrorAlert = ({ error, onRetry }) => {
  return (
    <Alert
      severity="error"
      action={
        onRetry && (
          <Button
            color="inherit"
            size="small"
            startIcon={<RefreshIcon />}
            onClick={onRetry}
          >
            Retry
          </Button>
        )
      }
    >
      <AlertTitle>Error</AlertTitle>
      {error}
    </Alert>
  );
};

export default ErrorAlert;
```

---

## Phase 11: Main App & Routing

### Step 11.1: Update App.jsx
```javascript
import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import Layout from './components/layout/Layout';
import Dashboard from './components/dashboard/Dashboard';
import TasksList from './components/tasks/TasksList';
import TaskDetails from './components/tasks/TaskDetails';
import SubmitTask from './components/tasks/SubmitTask';
import WorkersList from './components/workers/WorkersList';
import WorkerDetails from './components/workers/WorkerDetails';

// Create theme
const theme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: '#1976d2',
    },
    secondary: {
      main: '#dc004e',
    },
  },
});

function App() {
  return (
    <ThemeProvider theme={theme}>
      <BrowserRouter>
        <Layout>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/tasks" element={<TasksList />} />
            <Route path="/tasks/submit" element={<SubmitTask />} />
            <Route path="/tasks/:id" element={<TaskDetails />} />
            <Route path="/workers" element={<WorkersList />} />
            <Route path="/workers/:id" element={<WorkerDetails />} />
            <Route path="*" element={<Navigate to="/" />} />
          </Routes>
        </Layout>
      </BrowserRouter>
    </ThemeProvider>
  );
}

export default App;
```

### Step 11.2: Update main.jsx
```javascript
import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './styles/global.css';

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

### Step 11.3: Create Global Styles (`src/styles/global.css`)
```css
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: 'Roboto', 'Helvetica', 'Arial', sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

code {
  font-family: source-code-pro, Menlo, Monaco, Consolas, 'Courier New', monospace;
}

#root {
  min-height: 100vh;
}

/* Custom scrollbar */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

::-webkit-scrollbar-track {
  background: #f1f1f1;
}

::-webkit-scrollbar-thumb {
  background: #888;
  border-radius: 4px;
}

::-webkit-scrollbar-thumb:hover {
  background: #555;
}
```

---

## Phase 12: Testing & Optimization

### Step 12.1: Test WebSocket Connection
```javascript
// Test in browser console
const ws = new WebSocket('ws://localhost:8080/ws/telemetry');

ws.onopen = () => console.log('Connected');
ws.onmessage = (e) => console.log('Message:', JSON.parse(e.data));
ws.onerror = (e) => console.error('Error:', e);
```

### Step 12.2: Test REST APIs
```bash
# Test health endpoint
curl http://localhost:8080/health

# Test workers list
curl http://localhost:8080/api/workers

# Test tasks list
curl http://localhost:8080/api/tasks

# Submit a task
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "docker_image": "ubuntu:latest",
    "command": "echo Hello World",
    "cpu_required": 1.0,
    "memory_required": 2.0,
    "storage_required": 5.0,
    "gpu_required": 0.0
  }'
```

### Step 12.3: Performance Optimizations
- Use `React.memo()` for expensive components
- Implement virtual scrolling for large lists
- Debounce search inputs
- Lazy load route components
- Optimize WebSocket reconnection logic

---

## Phase 13: Advanced Features (Optional)

### Step 13.1: Dark Mode Toggle
```javascript
// Add to theme.js
import { createTheme } from '@mui/material/styles';

export const getTheme = (mode) => createTheme({
  palette: {
    mode,
    ...(mode === 'light'
      ? {
          // Light mode colors
          primary: { main: '#1976d2' },
          background: { default: '#f5f5f5', paper: '#fff' },
        }
      : {
          // Dark mode colors
          primary: { main: '#90caf9' },
          background: { default: '#121212', paper: '#1e1e1e' },
        }),
  },
});
```

### Step 13.2: Notifications System
```javascript
// Create notification context
import { createContext, useContext, useState } from 'react';
import { Snackbar, Alert } from '@mui/material';

const NotificationContext = createContext();

export const NotificationProvider = ({ children }) => {
  const [notification, setNotification] = useState(null);

  const showNotification = (message, severity = 'info') => {
    setNotification({ message, severity });
  };

  return (
    <NotificationContext.Provider value={{ showNotification }}>
      {children}
      <Snackbar
        open={!!notification}
        autoHideDuration={6000}
        onClose={() => setNotification(null)}
      >
        <Alert severity={notification?.severity}>
          {notification?.message}
        </Alert>
      </Snackbar>
    </NotificationContext.Provider>
  );
};

export const useNotification = () => useContext(NotificationContext);
```

### Step 13.3: Export/Download Features
```javascript
// Export tasks to CSV
const exportTasksToCSV = (tasks) => {
  const headers = ['Task ID', 'Status', 'Image', 'CPU', 'Memory', 'Created'];
  const rows = tasks.map(t => [
    t.task_id,
    t.status,
    t.docker_image,
    t.cpu_required,
    t.memory_required,
    new Date(t.created_at * 1000).toISOString(),
  ]);

  const csv = [headers, ...rows]
    .map(row => row.join(','))
    .join('\n');

  const blob = new Blob([csv], { type: 'text/csv' });
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'tasks.csv';
  a.click();
};
```

---

## 🎨 UI/UX Design Guidelines

### Color Scheme
- **Primary:** Blue (#1976d2) - Actions, links
- **Success:** Green (#2e7d32) - Completed tasks, active workers
- **Warning:** Orange (#ed6c02) - High resource usage, pending tasks
- **Error:** Red (#d32f2f) - Failed tasks, offline workers
- **Info:** Light Blue (#0288d1) - Queued tasks, information

### Typography
- **Headers:** Roboto Bold, 24-32px
- **Body:** Roboto Regular, 14-16px
- **Captions:** Roboto Light, 12px
- **Code:** Fira Code Mono, 14px

### Spacing
- **Component padding:** 16-24px
- **Grid gaps:** 16-24px
- **Card margins:** 8-16px

### Responsiveness
- **Mobile:** < 600px (single column)
- **Tablet:** 600-960px (2 columns)
- **Desktop:** > 960px (3-4 columns)

### Animations
- **Transitions:** 0.3s ease-in-out
- **Hover effects:** scale(1.02), shadow increase
- **Loading:** Skeleton screens + spinners

---

## 📝 Final Checklist

### Essential Features
- [ ] Dashboard with cluster overview
- [ ] Real-time worker monitoring (WebSocket)
- [ ] Task submission form with validation
- [ ] Task list with filtering and search
- [ ] Task details with logs viewer
- [ ] Worker list with real-time updates
- [ ] Worker details with resource charts
- [ ] Responsive design (mobile, tablet, desktop)
- [ ] Error handling and loading states
- [ ] API integration (REST + WebSocket)

### Nice-to-Have Features
- [ ] Dark mode toggle
- [ ] Notification system
- [ ] Export data (CSV, JSON)
- [ ] Advanced filtering and sorting
- [ ] Task history and analytics
- [ ] Worker health alerts
- [ ] Drag-and-drop task submission
- [ ] Multi-language support

### Testing
- [ ] Test all API endpoints
- [ ] Test WebSocket connections
- [ ] Test error scenarios
- [ ] Test on different browsers
- [ ] Test on mobile devices
- [ ] Test with slow network
- [ ] Test with many workers/tasks

### Deployment
- [ ] Build production bundle
- [ ] Configure environment variables
- [ ] Set up CORS properly
- [ ] Enable HTTPS/WSS
- [ ] Add authentication (if needed)
- [ ] Set up monitoring
- [ ] Document API endpoints

---

## 🚀 Quick Start Commands

```bash
# Development
cd web_interface/frontend
npm install
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Run with backend
# Terminal 1: Start MongoDB
cd database && docker-compose up

# Terminal 2: Start Master
cd master && go run main.go

# Terminal 3: Start Worker
cd worker && go run main.go

# Terminal 4: Start Frontend
cd web_interface/frontend && npm run dev
```

---

## 📚 Additional Resources

### Documentation
- React: https://react.dev
- Material-UI: https://mui.com
- Recharts: https://recharts.org
- Axios: https://axios-http.com
- WebSocket API: https://developer.mozilla.org/en-US/docs/Web/API/WebSocket

### Design Inspiration
- Vercel Dashboard: https://vercel.com
- Kubernetes Dashboard: https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/
- Grafana: https://grafana.com
- Docker Desktop: https://www.docker.com/products/docker-desktop/

---

**Implementation Complete! 🎉**

This guide provides a complete, production-ready frontend for CloudAI. Follow the phases sequentially, test thoroughly, and customize the UI to match your preferences. Good luck with your implementation!

