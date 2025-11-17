# CloudAI Frontend Implementation Guide - Part 2

**Continuation of:** FRONTEND_IMPLEMENTATION_GUIDE.md

---

## Phase 7: Task Components

### Step 7.1: Create Tasks List (`src/components/tasks/TasksList.jsx`)
```javascript
import React, { useState } from 'react';
import {
  Box,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TablePagination,
  Chip,
  IconButton,
  TextField,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
} from '@mui/material';
import {
  Visibility as ViewIcon,
  Delete as DeleteIcon,
  Refresh as RefreshIcon,
} from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import { useTasks } from '../../hooks/useTasks';
import { getStatusColor, formatRelativeTime } from '../../utils/formatters';
import { TASK_STATUS } from '../../utils/constants';

const TasksList = () => {
  const navigate = useNavigate();
  const { tasks, loading, error, refetch } = useTasks(true, 5000);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const [filterStatus, setFilterStatus] = useState('all');
  const [searchTerm, setSearchTerm] = useState('');

  const filteredTasks = tasks.filter((task) => {
    const matchesStatus = filterStatus === 'all' || task.status === filterStatus;
    const matchesSearch = task.task_id.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         task.docker_image.toLowerCase().includes(searchTerm.toLowerCase());
    return matchesStatus && matchesSearch;
  });

  const paginatedTasks = filteredTasks.slice(
    page * rowsPerPage,
    page * rowsPerPage + rowsPerPage
  );

  return (
    <Box>
      {/* Filters */}
      <Box sx={{ mb: 2, display: 'flex', gap: 2, alignItems: 'center' }}>
        <TextField
          label="Search"
          variant="outlined"
          size="small"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          sx={{ flexGrow: 1 }}
        />
        <FormControl size="small" sx={{ minWidth: 150 }}>
          <InputLabel>Status</InputLabel>
          <Select
            value={filterStatus}
            label="Status"
            onChange={(e) => setFilterStatus(e.target.value)}
          >
            <MenuItem value="all">All</MenuItem>
            {Object.values(TASK_STATUS).map((status) => (
              <MenuItem key={status} value={status}>{status}</MenuItem>
            ))}
          </Select>
        </FormControl>
        <IconButton onClick={refetch}>
          <RefreshIcon />
        </IconButton>
      </Box>

      {/* Table */}
      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Task ID</TableCell>
              <TableCell>Docker Image</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Resources</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {paginatedTasks.map((task) => (
              <TableRow key={task.task_id} hover>
                <TableCell>{task.task_id}</TableCell>
                <TableCell>{task.docker_image}</TableCell>
                <TableCell>
                  <Chip
                    label={task.status}
                    color={getStatusColor(task.status)}
                    size="small"
                  />
                </TableCell>
                <TableCell>
                  {task.cpu_required}C / {task.memory_required}GB
                  {task.gpu_required > 0 && ` / ${task.gpu_required}GPU`}
                </TableCell>
                <TableCell>{formatRelativeTime(task.created_at)}</TableCell>
                <TableCell>
                  <IconButton
                    size="small"
                    onClick={() => navigate(`/tasks/${task.task_id}`)}
                  >
                    <ViewIcon />
                  </IconButton>
                  {task.status === 'running' && (
                    <IconButton size="small" color="error">
                      <DeleteIcon />
                    </IconButton>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <TablePagination
          component="div"
          count={filteredTasks.length}
          page={page}
          onPageChange={(e, newPage) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => setRowsPerPage(parseInt(e.target.value, 10))}
        />
      </TableContainer>
    </Box>
  );
};

export default TasksList;
```

### Step 7.2: Create Submit Task Form (`src/components/tasks/SubmitTask.jsx`)
```javascript
import React, { useState } from 'react';
import {
  Box,
  Paper,
  Typography,
  TextField,
  Button,
  Grid,
  Alert,
  Slider,
} from '@mui/material';
import { Send as SendIcon } from '@mui/icons-material';
import { tasksAPI } from '../../api/tasks';
import { RESOURCE_LIMITS } from '../../utils/constants';
import { useNavigate } from 'react-router-dom';

const SubmitTask = () => {
  const navigate = useNavigate();
  const [formData, setFormData] = useState({
    docker_image: '',
    command: '',
    cpu_required: 1.0,
    memory_required: 2.0,
    storage_required: 5.0,
    gpu_required: 0.0,
    user_id: 'user-001',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);

  const handleChange = (field, value) => {
    setFormData({ ...formData, [field]: value });
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setSuccess(null);

    try {
      const response = await tasksAPI.submitTask(formData);
      setSuccess(`Task submitted successfully! Task ID: ${response.data.task_id}`);
      setTimeout(() => navigate(`/tasks/${response.data.task_id}`), 2000);
    } catch (err) {
      setError(err.response?.data?.message || err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Submit New Task
      </Typography>

      <Paper sx={{ p: 3, maxWidth: 800 }}>
        <form onSubmit={handleSubmit}>
          <Grid container spacing={3}>
            {/* Docker Image */}
            <Grid item xs={12}>
              <TextField
                label="Docker Image"
                fullWidth
                required
                value={formData.docker_image}
                onChange={(e) => handleChange('docker_image', e.target.value)}
                placeholder="e.g., ubuntu:latest, nginx:alpine"
              />
            </Grid>

            {/* Command */}
            <Grid item xs={12}>
              <TextField
                label="Command"
                fullWidth
                multiline
                rows={3}
                value={formData.command}
                onChange={(e) => handleChange('command', e.target.value)}
                placeholder="e.g., echo 'Hello World'"
              />
            </Grid>

            {/* CPU */}
            <Grid item xs={12} md={6}>
              <Typography gutterBottom>
                CPU Cores: {formData.cpu_required}
              </Typography>
              <Slider
                value={formData.cpu_required}
                onChange={(e, val) => handleChange('cpu_required', val)}
                min={RESOURCE_LIMITS.MIN_CPU}
                max={RESOURCE_LIMITS.MAX_CPU}
                step={0.5}
                marks
                valueLabelDisplay="auto"
              />
            </Grid>

            {/* Memory */}
            <Grid item xs={12} md={6}>
              <Typography gutterBottom>
                Memory: {formData.memory_required} GB
              </Typography>
              <Slider
                value={formData.memory_required}
                onChange={(e, val) => handleChange('memory_required', val)}
                min={RESOURCE_LIMITS.MIN_MEMORY}
                max={RESOURCE_LIMITS.MAX_MEMORY}
                step={0.5}
                marks
                valueLabelDisplay="auto"
              />
            </Grid>

            {/* Storage */}
            <Grid item xs={12} md={6}>
              <Typography gutterBottom>
                Storage: {formData.storage_required} GB
              </Typography>
              <Slider
                value={formData.storage_required}
                onChange={(e, val) => handleChange('storage_required', val)}
                min={RESOURCE_LIMITS.MIN_STORAGE}
                max={RESOURCE_LIMITS.MAX_STORAGE}
                step={1}
                marks
                valueLabelDisplay="auto"
              />
            </Grid>

            {/* GPU */}
            <Grid item xs={12} md={6}>
              <Typography gutterBottom>
                GPU Cores: {formData.gpu_required}
              </Typography>
              <Slider
                value={formData.gpu_required}
                onChange={(e, val) => handleChange('gpu_required', val)}
                min={RESOURCE_LIMITS.MIN_GPU}
                max={RESOURCE_LIMITS.MAX_GPU}
                step={0.5}
                marks
                valueLabelDisplay="auto"
              />
            </Grid>

            {/* User ID */}
            <Grid item xs={12}>
              <TextField
                label="User ID"
                fullWidth
                value={formData.user_id}
                onChange={(e) => handleChange('user_id', e.target.value)}
              />
            </Grid>

            {/* Alerts */}
            {error && (
              <Grid item xs={12}>
                <Alert severity="error">{error}</Alert>
              </Grid>
            )}
            {success && (
              <Grid item xs={12}>
                <Alert severity="success">{success}</Alert>
              </Grid>
            )}

            {/* Submit Button */}
            <Grid item xs={12}>
              <Button
                type="submit"
                variant="contained"
                size="large"
                fullWidth
                disabled={loading}
                startIcon={<SendIcon />}
              >
                {loading ? 'Submitting...' : 'Submit Task'}
              </Button>
            </Grid>
          </Grid>
        </form>
      </Paper>
    </Box>
  );
};

export default SubmitTask;
```

### Step 7.3: Create Task Details (`src/components/tasks/TaskDetails.jsx`)
```javascript
import React, { useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Box,
  Paper,
  Typography,
  Grid,
  Chip,
  Button,
  Divider,
  Card,
  CardContent,
} from '@mui/material';
import {
  ArrowBack as BackIcon,
  Delete as CancelIcon,
  Refresh as RefreshIcon,
} from '@mui/icons-material';
import { useTask } from '../../hooks/useTasks';
import { getStatusColor, formatRelativeTime, formatGB, formatCPU } from '../../utils/formatters';
import TaskLogs from './TaskLogs';

const TaskDetails = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const { task, loading, error, refetch } = useTask(id);

  if (loading) return <Typography>Loading task details...</Typography>;
  if (error) return <Typography color="error">Error: {error}</Typography>;
  if (!task) return <Typography>Task not found</Typography>;

  return (
    <Box>
      {/* Header */}
      <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Button startIcon={<BackIcon />} onClick={() => navigate('/tasks')}>
            Back
          </Button>
          <Typography variant="h4">Task Details</Typography>
          <Chip label={task.status} color={getStatusColor(task.status)} />
        </Box>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button startIcon={<RefreshIcon />} onClick={refetch}>
            Refresh
          </Button>
          {task.status === 'running' && (
            <Button
              variant="outlined"
              color="error"
              startIcon={<CancelIcon />}
            >
              Cancel Task
            </Button>
          )}
        </Box>
      </Box>

      <Grid container spacing={3}>
        {/* Task Info */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Task Information
              </Typography>
              <Divider sx={{ mb: 2 }} />
              
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                <Typography><strong>Task ID:</strong> {task.task_id}</Typography>
                <Typography><strong>User ID:</strong> {task.user_id}</Typography>
                <Typography><strong>Docker Image:</strong> {task.docker_image}</Typography>
                {task.command && (
                  <Typography><strong>Command:</strong> {task.command}</Typography>
                )}
                <Typography><strong>Created:</strong> {formatRelativeTime(task.created_at)}</Typography>
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* Resource Requirements */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Resource Requirements
              </Typography>
              <Divider sx={{ mb: 2 }} />
              
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                <Typography><strong>CPU:</strong> {formatCPU(task.cpu_required)}</Typography>
                <Typography><strong>Memory:</strong> {formatGB(task.memory_required)}</Typography>
                <Typography><strong>Storage:</strong> {formatGB(task.storage_required)}</Typography>
                {task.gpu_required > 0 && (
                  <Typography><strong>GPU:</strong> {task.gpu_required} cores</Typography>
                )}
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* Assignment Info */}
        {task.assignment && (
          <Grid item xs={12}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>
                  Assignment
                </Typography>
                <Divider sx={{ mb: 2 }} />
                <Typography>
                  <strong>Worker:</strong> {task.assignment.worker_id}
                </Typography>
                <Typography>
                  <strong>Assigned:</strong> {formatRelativeTime(task.assignment.assigned_at)}
                </Typography>
              </CardContent>
            </Card>
          </Grid>
        )}

        {/* Logs */}
        <Grid item xs={12}>
          <TaskLogs taskId={id} status={task.status} />
        </Grid>
      </Grid>
    </Box>
  );
};

export default TaskDetails;
```

### Step 7.4: Create Task Logs Viewer (`src/components/tasks/TaskLogs.jsx`)
```javascript
import React, { useEffect, useState } from 'react';
import {
  Card,
  CardContent,
  Typography,
  Box,
  CircularProgress,
  Paper,
} from '@mui/material';
import { tasksAPI } from '../../api/tasks';

const TaskLogs = ({ taskId, status }) => {
  const [logs, setLogs] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchLogs = async () => {
      if (status === 'pending' || status === 'queued') {
        setLogs('Task not started yet. No logs available.');
        setLoading(false);
        return;
      }

      try {
        setLoading(true);
        const response = await tasksAPI.getTaskLogs(taskId);
        setLogs(response.data.logs || 'No logs available');
      } catch (err) {
        setError(err.message);
        setLogs('Error fetching logs');
      } finally {
        setLoading(false);
      }
    };

    fetchLogs();

    // Auto-refresh logs for running tasks
    if (status === 'running') {
      const interval = setInterval(fetchLogs, 3000);
      return () => clearInterval(interval);
    }
  }, [taskId, status]);

  return (
    <Card>
      <CardContent>
        <Typography variant="h6" gutterBottom>
          Task Logs
        </Typography>
        
        {loading && (
          <Box sx={{ display: 'flex', justifyContent: 'center', p: 3 }}>
            <CircularProgress />
          </Box>
        )}

        {!loading && (
          <Paper
            sx={{
              p: 2,
              bgcolor: '#1e1e1e',
              color: '#d4d4d4',
              fontFamily: 'monospace',
              fontSize: '0.875rem',
              maxHeight: '400px',
              overflow: 'auto',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}
          >
            {logs}
          </Paper>
        )}
      </CardContent>
    </Card>
  );
};

export default TaskLogs;
```

---

## Phase 8: Worker Components

### Step 8.1: Create Workers List (`src/components/workers/WorkersList.jsx`)
```javascript
import React from 'react';
import { Grid, Box, Typography } from '@mui/material';
import { useWorkers } from '../../hooks/useWorkers';
import WorkerCard from './WorkerCard';

const WorkersList = () => {
  const { workers, loading, error } = useWorkers(true);

  if (loading) return <Typography>Loading workers...</Typography>;
  if (error) return <Typography color="error">Error: {error}</Typography>;
  if (!workers.length) return <Typography>No workers available</Typography>;

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Workers
      </Typography>
      
      <Grid container spacing={3}>
        {workers.map((worker) => (
          <Grid item xs={12} sm={6} md={4} key={worker.worker_id}>
            <WorkerCard worker={worker} />
          </Grid>
        ))}
      </Grid>
    </Box>
  );
};

export default WorkersList;
```

### Step 8.2: Create Worker Card (`src/components/workers/WorkerCard.jsx`)
```javascript
import React from 'react';
import {
  Card,
  CardContent,
  Typography,
  Box,
  Chip,
  LinearProgress,
  IconButton,
} from '@mui/material';
import { Visibility as ViewIcon, Computer as ComputerIcon } from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import { formatPercentage, formatRelativeTime, getUsageColor } from '../../utils/formatters';

const ResourceBar = ({ label, value, color }) => (
  <Box sx={{ mb: 1.5 }}>
    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
      <Typography variant="body2">{label}</Typography>
      <Typography variant="body2" fontWeight="bold">
        {formatPercentage(value)}
      </Typography>
    </Box>
    <LinearProgress
      variant="determinate"
      value={value}
      color={color}
      sx={{ height: 8, borderRadius: 1 }}
    />
  </Box>
);

const WorkerCard = ({ worker }) => {
  const navigate = useNavigate();

  return (
    <Card
      sx={{
        height: '100%',
        transition: 'all 0.3s',
        '&:hover': {
          boxShadow: 8,
          transform: 'translateY(-4px)',
        },
      }}
    >
      <CardContent>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <ComputerIcon color="primary" />
            <Typography variant="h6" noWrap>
              {worker.worker_id}
            </Typography>
          </Box>
          <Box>
            <Chip
              label={worker.is_active ? 'Active' : 'Offline'}
              color={worker.is_active ? 'success' : 'error'}
              size="small"
            />
          </Box>
        </Box>

        {/* Resources */}
        <ResourceBar
          label="CPU"
          value={worker.cpu_usage || 0}
          color={getUsageColor(worker.cpu_usage || 0)}
        />
        <ResourceBar
          label="Memory"
          value={worker.memory_usage || 0}
          color={getUsageColor(worker.memory_usage || 0)}
        />
        {worker.gpu_usage !== undefined && worker.gpu_usage > 0 && (
          <ResourceBar
            label="GPU"
            value={worker.gpu_usage}
            color={getUsageColor(worker.gpu_usage)}
          />
        )}

        {/* Footer */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mt: 2 }}>
          <Typography variant="body2" color="text.secondary">
            Tasks: {worker.running_tasks_count || 0}
          </Typography>
          <IconButton
            size="small"
            color="primary"
            onClick={() => navigate(`/workers/${worker.worker_id}`)}
          >
            <ViewIcon />
          </IconButton>
        </Box>

        <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 1 }}>
          Last seen: {formatRelativeTime(worker.last_update)}
        </Typography>
      </CardContent>
    </Card>
  );
};

export default WorkerCard;
```

---

