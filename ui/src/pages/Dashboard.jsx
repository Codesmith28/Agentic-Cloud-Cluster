import React, { useState, useEffect } from 'react';
import {
  Container,
  Grid,
  Paper,
  Typography,
  Box,
  CircularProgress,
  Chip,
  Tooltip,
} from '@mui/material';
import WifiIcon from '@mui/icons-material/Wifi';
import WifiOffIcon from '@mui/icons-material/WifiOff';
import { useRealTimeTasks } from '../hooks/useRealTimeTasks';
import { useTelemetry } from '../hooks/useTelemetry';
import { useAuth } from '../context/AuthContext';
import { workersAPI } from '../api/workers';

const Dashboard = () => {
  const [workers, setWorkers] = useState([]);
  const [loading, setLoading] = useState(true);
  const { user } = useAuth();
  
  // Real-time tasks
  const { tasks } = useRealTimeTasks(3000);
  
  // Real-time telemetry
  const { telemetryData, isConnected: wsConnected } = useTelemetry();

  useEffect(() => {
    fetchWorkers();
  }, []);

  // Update workers when telemetry arrives
  useEffect(() => {
    if (telemetryData.workers && Object.keys(telemetryData.workers).length > 0) {
      updateWorkersFromTelemetry();
    }
  }, [telemetryData]);

  const fetchWorkers = async () => {
    try {
      const response = await workersAPI.getAllWorkers();
      setWorkers(response.data.workers || []);
      setLoading(false);
    } catch (error) {
      console.error('Failed to fetch workers:', error);
      setLoading(false);
    }
  };

  const updateWorkersFromTelemetry = () => {
    setWorkers((prevWorkers) => {
      return prevWorkers.map((worker) => {
        const telemetry = telemetryData.workers[worker.worker_id];
        if (telemetry) {
          return {
            ...worker,
            is_active: telemetry.is_active,
            cpu_usage: telemetry.cpu_usage,
            memory_usage: telemetry.memory_usage,
            gpu_usage: telemetry.gpu_usage,
            // Update resource allocations from telemetry
            total_resources: {
              cpu: telemetry.total_resources?.cpu || worker.total_resources?.cpu || 0,
              memory: telemetry.total_resources?.memory || worker.total_resources?.memory || 0,
              storage: telemetry.total_resources?.storage || worker.total_resources?.storage || 0,
              gpu: telemetry.total_resources?.gpu || worker.total_resources?.gpu || 0,
            },
            allocated_resources: {
              cpu: telemetry.allocated_resources?.cpu || worker.allocated_resources?.cpu || 0,
              memory: telemetry.allocated_resources?.memory || worker.allocated_resources?.memory || 0,
              storage: telemetry.allocated_resources?.storage || worker.allocated_resources?.storage || 0,
              gpu: telemetry.allocated_resources?.gpu || worker.allocated_resources?.gpu || 0,
            },
            available_resources: {
              cpu: telemetry.available_resources?.cpu || worker.available_resources?.cpu || 0,
              memory: telemetry.available_resources?.memory || worker.available_resources?.memory || 0,
              storage: telemetry.available_resources?.storage || worker.available_resources?.storage || 0,
              gpu: telemetry.available_resources?.gpu || worker.available_resources?.gpu || 0,
            },
          };
        }
        return worker;
      });
    });
  };

  // Calculate stats from real-time data
  const stats = {
    totalTasks: tasks.length,
    runningTasks: tasks.filter((t) => t.status === 'running').length,
    completedTasks: tasks.filter((t) => t.status === 'completed').length,
    failedTasks: tasks.filter((t) => t.status === 'failed').length,
    totalWorkers: workers.length,
    activeWorkers: workers.filter((w) => w.is_active).length,
  };

  // Calculate total resources from active workers only
  const activeWorkers = workers.filter((w) => w.is_active);
  const totalResources = activeWorkers.reduce(
    (acc, worker) => ({
      cpu: acc.cpu + (worker.total_resources?.cpu || 0),
      memory: acc.memory + (worker.total_resources?.memory || 0),
      storage: acc.storage + (worker.total_resources?.storage || 0),
      gpu: acc.gpu + (worker.total_resources?.gpu || 0),
    }),
    { cpu: 0, memory: 0, storage: 0, gpu: 0 }
  );

  // Calculate allocated resources from active workers
  const allocatedResources = activeWorkers.reduce(
    (acc, worker) => ({
      cpu: acc.cpu + (worker.allocated_resources?.cpu || 0),
      memory: acc.memory + (worker.allocated_resources?.memory || 0),
      storage: acc.storage + (worker.allocated_resources?.storage || 0),
      gpu: acc.gpu + (worker.allocated_resources?.gpu || 0),
    }),
    { cpu: 0, memory: 0, storage: 0, gpu: 0 }
  );

  // Calculate available resources
  const availableResources = {
    cpu: totalResources.cpu - allocatedResources.cpu,
    memory: totalResources.memory - allocatedResources.memory,
    storage: totalResources.storage - allocatedResources.storage,
    gpu: totalResources.gpu - allocatedResources.gpu,
  };

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="60vh">
        <CircularProgress />
      </Box>
    );
  }

  const StatCard = ({ title, value, color }) => (
    <Paper elevation={3} sx={{ p: 3, textAlign: 'center' }}>
      <Typography variant="h6" color="text.secondary" gutterBottom>
        {title}
      </Typography>
      <Typography variant="h3" sx={{ color, fontWeight: 'bold' }}>
        {value}
      </Typography>
    </Paper>
  );

  const ResourceCard = ({ title, total, allocated, available, unit, color }) => {
    const usagePercent = total > 0 ? ((allocated / total) * 100).toFixed(1) : 0;
    
    return (
      <Paper elevation={3} sx={{ p: 3 }}>
        <Typography variant="h6" color="text.secondary" gutterBottom>
          {title}
        </Typography>
        <Box sx={{ mt: 2 }}>
          <Box display="flex" justifyContent="space-between" mb={1}>
            <Typography variant="body2" color="text.secondary">Total Capacity</Typography>
            <Typography variant="h6" sx={{ color, fontWeight: 'bold' }}>
              {total.toFixed(1)} {unit}
            </Typography>
          </Box>
          <Box display="flex" justifyContent="space-between" mb={1}>
            <Typography variant="body2" color="text.secondary">Allocated</Typography>
            <Typography variant="body2">
              {allocated.toFixed(1)} {unit} ({usagePercent}%)
            </Typography>
          </Box>
          <Box display="flex" justifyContent="space-between" mb={1}>
            <Typography variant="body2" color="text.secondary">Available</Typography>
            <Typography variant="body2" sx={{ color: 'success.main', fontWeight: 'bold' }}>
              {available.toFixed(1)} {unit}
            </Typography>
          </Box>
          {/* Progress bar */}
          <Box sx={{ width: '100%', height: 8, bgcolor: 'grey.200', borderRadius: 1, mt: 2 }}>
            <Box
              sx={{
                width: `${Math.min(usagePercent, 100)}%`,
                height: '100%',
                bgcolor: usagePercent > 90 ? 'error.main' : usagePercent > 70 ? 'warning.main' : color,
                borderRadius: 1,
                transition: 'width 0.3s ease',
              }}
            />
          </Box>
        </Box>
      </Paper>
    );
  };

  return (
    <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
      {/* Welcome Message */}
      <Box mb={4}>
        <Typography variant="h4" gutterBottom>
          Welcome back, {user?.name || 'User'}!
        </Typography>
      </Box>

      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h5">Cluster Overview</Typography>
        <Tooltip title={wsConnected ? 'Real-time updates active' : 'Connecting...'}>
          <Chip
            icon={wsConnected ? <WifiIcon /> : <WifiOffIcon />}
            label={wsConnected ? 'Live Updates' : 'Connecting'}
            color={wsConnected ? 'success' : 'default'}
            size="small"
            variant="outlined"
          />
        </Tooltip>
      </Box>

      {/* Cluster Resources Summary */}
      <Paper elevation={3} sx={{ p: 3, mb: 3, bgcolor: 'primary.50' }}>
        <Typography variant="h5" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          🖥️ Cluster Resources ({stats.activeWorkers} Active Workers)
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
          Total capacity from all active workers - Maximum resources available for task execution
        </Typography>
        <Grid container spacing={3}>
          <Grid item xs={12} sm={6} md={3}>
            <ResourceCard
              title="CPU Cores"
              total={totalResources.cpu}
              allocated={allocatedResources.cpu}
              available={availableResources.cpu}
              unit="cores"
              color="#1976d2"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <ResourceCard
              title="Memory (RAM)"
              total={totalResources.memory}
              allocated={allocatedResources.memory}
              available={availableResources.memory}
              unit="GB"
              color="#2e7d32"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <ResourceCard
              title="Storage"
              total={totalResources.storage}
              allocated={allocatedResources.storage}
              available={availableResources.storage}
              unit="GB"
              color="#ed6c02"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <ResourceCard
              title="GPU Cores"
              total={totalResources.gpu}
              allocated={allocatedResources.gpu}
              available={availableResources.gpu}
              unit="cores"
              color="#9c27b0"
            />
          </Grid>
        </Grid>
      </Paper>

      {/* Task Statistics */}
      <Typography variant="h5" gutterBottom sx={{ mt: 4, mb: 2 }}>
        📊 Task Statistics
      </Typography>
      <Grid container spacing={3} sx={{ mt: 2 }}>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard title="Total Tasks" value={stats.totalTasks} color="#1976d2" />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard title="Running" value={stats.runningTasks} color="#2e7d32" />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard title="Completed" value={stats.completedTasks} color="#ed6c02" />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard title="Failed" value={stats.failedTasks} color="#d32f2f" />
        </Grid>
        <Grid item xs={12} sm={6}>
          <StatCard title="Total Workers" value={stats.totalWorkers} color="#9c27b0" />
        </Grid>
        <Grid item xs={12} sm={6}>
          <StatCard title="Active Workers" value={stats.activeWorkers} color="#0288d1" />
        </Grid>
      </Grid>
    </Container>
  );
};

export default Dashboard;
