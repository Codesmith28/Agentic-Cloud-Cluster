import React, { useState, useEffect, useMemo } from 'react';
import {
  Container,
  Grid,
  Paper,
  Typography,
  Box,
  CircularProgress,
  Chip,
  Tooltip,
  LinearProgress,
  Divider,
} from '@mui/material';
import WifiIcon from '@mui/icons-material/Wifi';
import WifiOffIcon from '@mui/icons-material/WifiOff';
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as RechartsTooltip,
  Legend,
  BarChart,
  Bar,
} from 'recharts';
import { useRealTimeTasks } from '../hooks/useRealTimeTasks';
import { useTelemetry } from '../hooks/useTelemetry';
import { useAuth } from '../context/AuthContext';
import { workersAPI } from '../api/workers';

const HISTORY_POINTS = 72; // ~6 minutes at 5s heartbeat cadence
const BENCHMARK_WINDOW_MS = 5 * 60 * 1000;

const safeNumber = (value, fallback = 0) =>
  typeof value === 'number' && Number.isFinite(value) ? value : fallback;

const normalizePercentage = (value) => {
  const numeric = safeNumber(value);
  if (numeric <= 1) {
    return numeric * 100;
  }
  return numeric;
};

const formatTimelineLabel = (timestamp) =>
  new Date(timestamp).toLocaleTimeString([], {
    hour12: false,
    minute: '2-digit',
    second: '2-digit',
  });

const Dashboard = () => {
  const [workers, setWorkers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [history, setHistory] = useState([]);
  const { user } = useAuth();

  const { tasks } = useRealTimeTasks(3000);
  const { telemetryData, isConnected: wsConnected } = useTelemetry();

  useEffect(() => {
    fetchWorkers();

    // Keep resource capacities aligned with backend state.
    const refreshInterval = setInterval(fetchWorkers, 15000);

    return () => clearInterval(refreshInterval);
  }, []);

  useEffect(() => {
    if (telemetryData.workers && Object.keys(telemetryData.workers).length > 0) {
      updateWorkersFromTelemetry(telemetryData.workers);
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

  const updateWorkersFromTelemetry = (telemetryWorkers) => {
    setWorkers((prevWorkers) => {
      const workerMap = new Map();
      prevWorkers.forEach((worker) => workerMap.set(worker.worker_id, worker));

      Object.entries(telemetryWorkers).forEach(([workerId, telemetry]) => {
        const existingWorker = workerMap.get(workerId) || {};
        const runningTasks = Array.isArray(telemetry.running_tasks) ? telemetry.running_tasks : [];

        const inferredAllocated = runningTasks.reduce(
          (acc, task) => ({
            cpu: acc.cpu + safeNumber(task.cpu_allocated),
            memory: acc.memory + safeNumber(task.memory_allocated),
            gpu: acc.gpu + safeNumber(task.gpu_allocated),
          }),
          { cpu: 0, memory: 0, gpu: 0 }
        );

        const totalResources = {
          cpu: safeNumber(telemetry.total_resources?.cpu, safeNumber(existingWorker.total_resources?.cpu)),
          memory: safeNumber(
            telemetry.total_resources?.memory,
            safeNumber(existingWorker.total_resources?.memory)
          ),
          storage: safeNumber(
            telemetry.total_resources?.storage,
            safeNumber(existingWorker.total_resources?.storage)
          ),
          gpu: safeNumber(telemetry.total_resources?.gpu, safeNumber(existingWorker.total_resources?.gpu)),
        };

        const allocatedResources = {
          cpu: safeNumber(
            telemetry.allocated_resources?.cpu,
            runningTasks.length > 0
              ? inferredAllocated.cpu
              : safeNumber(existingWorker.allocated_resources?.cpu)
          ),
          memory: safeNumber(
            telemetry.allocated_resources?.memory,
            runningTasks.length > 0
              ? inferredAllocated.memory
              : safeNumber(existingWorker.allocated_resources?.memory)
          ),
          storage: safeNumber(
            telemetry.allocated_resources?.storage,
            safeNumber(existingWorker.allocated_resources?.storage)
          ),
          gpu: safeNumber(
            telemetry.allocated_resources?.gpu,
            runningTasks.length > 0
              ? inferredAllocated.gpu
              : safeNumber(existingWorker.allocated_resources?.gpu)
          ),
        };

        const availableResources = {
          cpu: Math.max(0, totalResources.cpu - allocatedResources.cpu),
          memory: Math.max(0, totalResources.memory - allocatedResources.memory),
          storage: Math.max(0, totalResources.storage - allocatedResources.storage),
          gpu: Math.max(0, totalResources.gpu - allocatedResources.gpu),
        };

        workerMap.set(workerId, {
          ...existingWorker,
          worker_id: workerId,
          address: existingWorker.address || existingWorker.worker_ip || telemetry.worker_ip || 'Unknown',
          worker_ip: existingWorker.worker_ip || telemetry.worker_ip,
          is_active: Boolean(telemetry.is_active),
          cpu_usage: safeNumber(telemetry.cpu_usage),
          memory_usage: safeNumber(telemetry.memory_usage),
          gpu_usage: safeNumber(telemetry.gpu_usage),
          running_tasks_count: runningTasks.length,
          total_resources: totalResources,
          allocated_resources: allocatedResources,
          available_resources: availableResources,
          last_update: telemetry.last_update || existingWorker.last_update,
        });
      });

      return Array.from(workerMap.values());
    });
  };

  const taskStats = useMemo(() => {
    const runningTasks = tasks.filter((task) => task.status === 'running').length;
    const pendingTasks = tasks.filter((task) => task.status === 'pending').length;
    const queuedTasks = tasks.filter((task) => task.status === 'queued').length;
    const completedTasks = tasks.filter((task) => task.status === 'completed').length;
    const failedTasks = tasks.filter((task) => task.status === 'failed').length;

    return {
      totalTasks: tasks.length,
      runningTasks,
      pendingTasks,
      queuedTasks,
      pendingAndQueued: pendingTasks + queuedTasks,
      completedTasks,
      failedTasks,
    };
  }, [tasks]);

  const workerStats = useMemo(() => {
    const activeWorkers = workers.filter((worker) => worker.is_active);
    return {
      totalWorkers: workers.length,
      activeWorkers,
      activeCount: activeWorkers.length,
    };
  }, [workers]);

  const resourceStats = useMemo(() => {
    const totalResources = workerStats.activeWorkers.reduce(
      (acc, worker) => ({
        cpu: acc.cpu + safeNumber(worker.total_resources?.cpu),
        memory: acc.memory + safeNumber(worker.total_resources?.memory),
        storage: acc.storage + safeNumber(worker.total_resources?.storage),
        gpu: acc.gpu + safeNumber(worker.total_resources?.gpu),
      }),
      { cpu: 0, memory: 0, storage: 0, gpu: 0 }
    );

    const allocatedResources = workerStats.activeWorkers.reduce(
      (acc, worker) => ({
        cpu: acc.cpu + safeNumber(worker.allocated_resources?.cpu),
        memory: acc.memory + safeNumber(worker.allocated_resources?.memory),
        storage: acc.storage + safeNumber(worker.allocated_resources?.storage),
        gpu: acc.gpu + safeNumber(worker.allocated_resources?.gpu),
      }),
      { cpu: 0, memory: 0, storage: 0, gpu: 0 }
    );

    const availableResources = {
      cpu: Math.max(0, totalResources.cpu - allocatedResources.cpu),
      memory: Math.max(0, totalResources.memory - allocatedResources.memory),
      storage: Math.max(0, totalResources.storage - allocatedResources.storage),
      gpu: Math.max(0, totalResources.gpu - allocatedResources.gpu),
    };

    const usagePercents = [];
    if (totalResources.cpu > 0) {
      usagePercents.push((allocatedResources.cpu / totalResources.cpu) * 100);
    }
    if (totalResources.memory > 0) {
      usagePercents.push((allocatedResources.memory / totalResources.memory) * 100);
    }
    if (totalResources.gpu > 0) {
      usagePercents.push((allocatedResources.gpu / totalResources.gpu) * 100);
    }

    return {
      totalResources,
      allocatedResources,
      availableResources,
      saturationPct:
        usagePercents.length > 0
          ? usagePercents.reduce((sum, value) => sum + value, 0) / usagePercents.length
          : 0,
    };
  }, [workerStats]);

  const aggregateUtilization = useMemo(() => {
    if (workerStats.activeCount === 0) {
      return {
        cpuPct: 0,
        memoryPct: 0,
        gpuPct: 0,
      };
    }

    const totals = workerStats.activeWorkers.reduce(
      (acc, worker) => ({
        cpuPct: acc.cpuPct + normalizePercentage(worker.cpu_usage),
        memoryPct: acc.memoryPct + normalizePercentage(worker.memory_usage),
        gpuPct: acc.gpuPct + normalizePercentage(worker.gpu_usage),
      }),
      { cpuPct: 0, memoryPct: 0, gpuPct: 0 }
    );

    return {
      cpuPct: totals.cpuPct / workerStats.activeCount,
      memoryPct: totals.memoryPct / workerStats.activeCount,
      gpuPct: totals.gpuPct / workerStats.activeCount,
    };
  }, [workerStats]);

  const avgQueueAgeSeconds = useMemo(() => {
    const nowSec = Date.now() / 1000;
    const queued = tasks.filter((task) => task.status === 'pending' || task.status === 'queued');

    if (queued.length === 0) {
      return 0;
    }

    const totalAge = queued.reduce((acc, task) => {
      const createdAt = safeNumber(task.created_at, nowSec);
      return acc + Math.max(0, nowSec - createdAt);
    }, 0);

    return totalAge / queued.length;
  }, [tasks]);

  const snapshot = useMemo(
    () => ({
      runningTasks: taskStats.runningTasks,
      queuedTasks: taskStats.pendingAndQueued,
      completedTasks: taskStats.completedTasks,
      failedTasks: taskStats.failedTasks,
      cpuUtilizationPct: aggregateUtilization.cpuPct,
      memoryUtilizationPct: aggregateUtilization.memoryPct,
      gpuUtilizationPct: aggregateUtilization.gpuPct,
      resourceSaturationPct: resourceStats.saturationPct,
      activeWorkers: workerStats.activeCount,
      totalWorkers: workerStats.totalWorkers,
    }),
    [taskStats, aggregateUtilization, resourceStats, workerStats]
  );

  useEffect(() => {
    const timestamp = Date.now();

    setHistory((prev) => {
      const next = [
        ...prev,
        {
          timestamp,
          label: formatTimelineLabel(timestamp),
          ...snapshot,
        },
      ];

      if (next.length > HISTORY_POINTS) {
        return next.slice(next.length - HISTORY_POINTS);
      }
      return next;
    });
  }, [snapshot]);

  const benchmarkStats = useMemo(() => {
    if (history.length === 0) {
      return {
        throughputPerMinute: 0,
        failedPerMinute: 0,
        successRatePct: 100,
      };
    }

    const latest = history[history.length - 1];
    const windowStartTs = latest.timestamp - BENCHMARK_WINDOW_MS;
    const windowSeries = history.filter((point) => point.timestamp >= windowStartTs);
    const baseline = windowSeries.length > 0 ? windowSeries[0] : history[0];

    const elapsedMinutes = Math.max((latest.timestamp - baseline.timestamp) / 60000, 1 / 60);
    const completedDelta = Math.max(0, latest.completedTasks - baseline.completedTasks);
    const failedDelta = Math.max(0, latest.failedTasks - baseline.failedTasks);

    const terminalTasks = taskStats.completedTasks + taskStats.failedTasks;
    const successRatePct =
      terminalTasks > 0 ? (taskStats.completedTasks / terminalTasks) * 100 : 100;

    return {
      throughputPerMinute: completedDelta / elapsedMinutes,
      failedPerMinute: failedDelta / elapsedMinutes,
      successRatePct,
    };
  }, [history, taskStats]);

  const queuePressurePct =
    taskStats.totalTasks > 0 ? (taskStats.pendingAndQueued / taskStats.totalTasks) * 100 : 0;

  const queuedPerActiveWorker =
    workerStats.activeCount > 0 ? taskStats.pendingAndQueued / workerStats.activeCount : 0;

  const workerLoadData = useMemo(() => {
    const computeWorkerLoad = (worker) => {
      const loadSignals = [
        normalizePercentage(worker.cpu_usage),
        normalizePercentage(worker.memory_usage),
      ];

      if (safeNumber(worker.total_resources?.gpu) > 0) {
        loadSignals.push(normalizePercentage(worker.gpu_usage));
      }

      return loadSignals.reduce((sum, value) => sum + value, 0) / loadSignals.length;
    };

    return workers
      .map((worker) => ({
        workerId: worker.worker_id,
        label: worker.worker_id ? worker.worker_id.slice(0, 8) : 'unknown',
        loadPct: computeWorkerLoad(worker),
        isActive: Boolean(worker.is_active),
      }))
      .sort((a, b) => b.loadPct - a.loadPct)
      .slice(0, 8);
  }, [workers]);

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="60vh">
        <CircularProgress />
      </Box>
    );
  }

  const StatCard = ({ title, value, subtitle, color }) => (
    <Paper elevation={3} sx={{ p: 2.5, height: '100%' }}>
      <Typography variant="body2" color="text.secondary" gutterBottom>
        {title}
      </Typography>
      <Typography variant="h4" sx={{ color, fontWeight: 700, lineHeight: 1.2 }}>
        {value}
      </Typography>
      {subtitle ? (
        <Typography variant="caption" color="text.secondary">
          {subtitle}
        </Typography>
      ) : null}
    </Paper>
  );

  const ResourceCard = ({ title, total, allocated, available, unit, color }) => {
    const usagePercent = total > 0 ? (allocated / total) * 100 : 0;

    return (
      <Paper elevation={2} sx={{ p: 2, height: '100%' }}>
        <Typography variant="subtitle1" color="text.secondary" gutterBottom>
          {title}
        </Typography>
        <Box display="flex" justifyContent="space-between" mb={0.5}>
          <Typography variant="body2" color="text.secondary">
            Total
          </Typography>
          <Typography variant="body2" fontWeight={600}>
            {total.toFixed(1)} {unit}
          </Typography>
        </Box>
        <Box display="flex" justifyContent="space-between" mb={0.5}>
          <Typography variant="body2" color="text.secondary">
            Allocated
          </Typography>
          <Typography variant="body2">
            {allocated.toFixed(1)} {unit}
          </Typography>
        </Box>
        <Box display="flex" justifyContent="space-between" mb={1.5}>
          <Typography variant="body2" color="text.secondary">
            Available
          </Typography>
          <Typography variant="body2" color="success.main" fontWeight={600}>
            {available.toFixed(1)} {unit}
          </Typography>
        </Box>
        <LinearProgress
          variant="determinate"
          value={Math.min(100, usagePercent)}
          sx={{
            height: 8,
            borderRadius: 8,
            '& .MuiLinearProgress-bar': {
              backgroundColor: usagePercent > 85 ? '#d32f2f' : color,
            },
          }}
        />
        <Typography variant="caption" color="text.secondary">
          {usagePercent.toFixed(1)}% in use
        </Typography>
      </Paper>
    );
  };

  return (
    <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
      <Box mb={3}>
        <Typography variant="h4" gutterBottom>
          Welcome back, {user?.name || 'User'}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Master dashboard with near-realtime cluster telemetry and benchmark signals
        </Typography>
      </Box>

      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Typography variant="h5">Cluster Overview</Typography>
        <Tooltip title={wsConnected ? 'Realtime telemetry connected' : 'Waiting for telemetry stream'}>
          <Chip
            icon={wsConnected ? <WifiIcon /> : <WifiOffIcon />}
            label={wsConnected ? 'Live updates' : 'Connecting'}
            color={wsConnected ? 'success' : 'default'}
            size="small"
            variant="outlined"
          />
        </Tooltip>
      </Box>

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="Total Tasks"
            value={taskStats.totalTasks}
            subtitle={`${taskStats.pendingAndQueued} pending/queued`}
            color="#1976d2"
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="Running Tasks"
            value={taskStats.runningTasks}
            subtitle={`${taskStats.completedTasks} completed`}
            color="#2e7d32"
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="Active Workers"
            value={workerStats.activeCount}
            subtitle={`${workerStats.totalWorkers} total workers`}
            color="#0288d1"
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="Cluster Saturation"
            value={`${resourceStats.saturationPct.toFixed(1)}%`}
            subtitle="CPU, memory, GPU allocation blend"
            color="#ed6c02"
          />
        </Grid>
      </Grid>

      <Paper elevation={3} sx={{ p: 2.5, mb: 3 }}>
        <Typography variant="h6" gutterBottom>
          Resource Capacity (Active Workers)
        </Typography>
        <Grid container spacing={2}>
          <Grid item xs={12} sm={6} md={3}>
            <ResourceCard
              title="CPU"
              total={resourceStats.totalResources.cpu}
              allocated={resourceStats.allocatedResources.cpu}
              available={resourceStats.availableResources.cpu}
              unit="cores"
              color="#1976d2"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <ResourceCard
              title="Memory"
              total={resourceStats.totalResources.memory}
              allocated={resourceStats.allocatedResources.memory}
              available={resourceStats.availableResources.memory}
              unit="GB"
              color="#2e7d32"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <ResourceCard
              title="Storage"
              total={resourceStats.totalResources.storage}
              allocated={resourceStats.allocatedResources.storage}
              available={resourceStats.availableResources.storage}
              unit="GB"
              color="#ed6c02"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <ResourceCard
              title="GPU"
              total={resourceStats.totalResources.gpu}
              allocated={resourceStats.allocatedResources.gpu}
              available={resourceStats.availableResources.gpu}
              unit="cores"
              color="#9c27b0"
            />
          </Grid>
        </Grid>
      </Paper>

      <Paper elevation={3} sx={{ p: 2.5, mb: 3 }}>
        <Typography variant="h6" gutterBottom>
          Live Benchmarking (Rolling 5-Min Window)
        </Typography>
        <Grid container spacing={2} columns={{ xs: 12, md: 10 }}>
          <Grid item xs={12} sm={6} md={2}>
            <StatCard
              title="Throughput"
              value={`${benchmarkStats.throughputPerMinute.toFixed(2)}/min`}
              subtitle="Completed tasks"
              color="#2e7d32"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={2}>
            <StatCard
              title="Failure Rate"
              value={`${benchmarkStats.failedPerMinute.toFixed(2)}/min`}
              subtitle="Failed tasks"
              color="#d32f2f"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={2}>
            <StatCard
              title="Success Rate"
              value={`${benchmarkStats.successRatePct.toFixed(1)}%`}
              subtitle="Completed vs failed"
              color="#1976d2"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={2}>
            <StatCard
              title="Queue Pressure"
              value={`${queuePressurePct.toFixed(1)}%`}
              subtitle={`${queuedPerActiveWorker.toFixed(2)} queued per active worker`}
              color="#ed6c02"
            />
          </Grid>
          <Grid item xs={12} sm={6} md={2}>
            <StatCard
              title="Avg Queue Age"
              value={`${avgQueueAgeSeconds.toFixed(1)}s`}
              subtitle="Pending and queued tasks"
              color="#6a1b9a"
            />
          </Grid>
        </Grid>
      </Paper>

      <Grid container spacing={3}>
        <Grid item xs={12} lg={8}>
          <Paper elevation={3} sx={{ p: 2.5, height: 360 }}>
            <Typography variant="h6" gutterBottom>
              Resource Utilization Trend
            </Typography>
            <Divider sx={{ mb: 1.5 }} />
            <ResponsiveContainer width="100%" height="84%">
              <LineChart data={history} margin={{ top: 10, right: 20, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="label" minTickGap={28} />
                <YAxis domain={[0, 100]} unit="%" />
                <RechartsTooltip formatter={(value) => `${safeNumber(value).toFixed(1)}%`} />
                <Legend />
                <Line
                  type="monotone"
                  dataKey="cpuUtilizationPct"
                  name="CPU"
                  stroke="#1976d2"
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
                <Line
                  type="monotone"
                  dataKey="memoryUtilizationPct"
                  name="Memory"
                  stroke="#2e7d32"
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
                <Line
                  type="monotone"
                  dataKey="gpuUtilizationPct"
                  name="GPU"
                  stroke="#9c27b0"
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
                <Line
                  type="monotone"
                  dataKey="resourceSaturationPct"
                  name="Allocation Saturation"
                  stroke="#ed6c02"
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </Paper>
        </Grid>

        <Grid item xs={12} lg={4}>
          <Paper elevation={3} sx={{ p: 2.5, height: 360 }}>
            <Typography variant="h6" gutterBottom>
              Worker Load (Top 8)
            </Typography>
            <Divider sx={{ mb: 1.5 }} />
            <ResponsiveContainer width="100%" height="84%">
              <BarChart data={workerLoadData} margin={{ top: 10, right: 20, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="label" />
                <YAxis domain={[0, 100]} unit="%" />
                <RechartsTooltip formatter={(value) => `${safeNumber(value).toFixed(1)}%`} />
                <Legend />
                <Bar dataKey="loadPct" name="Composite Load" fill="#0288d1" isAnimationActive={false} />
              </BarChart>
            </ResponsiveContainer>
          </Paper>
        </Grid>

        <Grid item xs={12}>
          <Paper elevation={3} sx={{ p: 2.5, height: 340 }}>
            <Typography variant="h6" gutterBottom>
              Task State Trend
            </Typography>
            <Divider sx={{ mb: 1.5 }} />
            <ResponsiveContainer width="100%" height="82%">
              <LineChart data={history} margin={{ top: 10, right: 20, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="label" minTickGap={28} />
                <YAxis allowDecimals={false} />
                <RechartsTooltip formatter={(value) => safeNumber(value).toFixed(0)} />
                <Legend />
                <Line
                  type="monotone"
                  dataKey="runningTasks"
                  name="Running"
                  stroke="#2e7d32"
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
                <Line
                  type="monotone"
                  dataKey="queuedTasks"
                  name="Pending + Queued"
                  stroke="#ed6c02"
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
                <Line
                  type="monotone"
                  dataKey="completedTasks"
                  name="Completed"
                  stroke="#1976d2"
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
                <Line
                  type="monotone"
                  dataKey="failedTasks"
                  name="Failed"
                  stroke="#d32f2f"
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </Paper>
        </Grid>
      </Grid>

      <Box mt={2}>
        <Typography variant="caption" color="text.secondary">
          Metrics window: rolling 5 minutes. History points shown: last {Math.min(history.length, HISTORY_POINTS)}.
        </Typography>
      </Box>
    </Container>
  );
};

export default Dashboard;
