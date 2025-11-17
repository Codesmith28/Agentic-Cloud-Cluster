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
import { workersAPI } from '../api/workers';

const Dashboard = () => {
  const [workers, setWorkers] = useState([]);
  const [loading, setLoading] = useState(true);
  
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

  return (
    <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h4">Dashboard</Typography>
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
