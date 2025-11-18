import React, { useState, useEffect } from 'react';
import {
  Container,
  Typography,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  Box,
  CircularProgress,
  Button,
  LinearProgress,
  Alert,
  Tooltip,
} from '@mui/material';
import RefreshIcon from '@mui/icons-material/Refresh';
import AddIcon from '@mui/icons-material/Add';
import WifiIcon from '@mui/icons-material/Wifi';
import WifiOffIcon from '@mui/icons-material/WifiOff';
import { workersAPI } from '../api/workers';
import { formatCPU, formatGB } from '../utils/formatters';
import WorkerRegistrationDialog from '../components/WorkerRegistrationDialog';
import { useTelemetry } from '../hooks/useTelemetry';

const WorkersPage = () => {
  const [workers, setWorkers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [successMessage, setSuccessMessage] = useState('');
  
  // Real-time telemetry via WebSocket
  const { telemetryData, isConnected: wsConnected } = useTelemetry();

  useEffect(() => {
    fetchWorkers();
  }, []);

  // Update workers when telemetry data arrives
  useEffect(() => {
    if (telemetryData.workers && Object.keys(telemetryData.workers).length > 0) {
      // Merge telemetry data with existing workers data
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
      // Create a map of existing workers
      const workerMap = new Map();
      prevWorkers.forEach((w) => workerMap.set(w.worker_id, w));

      // Update with telemetry data
      Object.entries(telemetryWorkers).forEach(([workerId, telemetry]) => {
        const existingWorker = workerMap.get(workerId);
        if (existingWorker) {
          // Update real-time data including resources
          workerMap.set(workerId, {
            ...existingWorker,
            is_active: telemetry.is_active,
            cpu_usage: telemetry.cpu_usage,
            memory_usage: telemetry.memory_usage,
            gpu_usage: telemetry.gpu_usage,
            running_tasks_count: telemetry.running_tasks?.length || 0,
            last_update: telemetry.last_update,
            // Update resource allocations from telemetry
            total_resources: {
              cpu: telemetry.total_resources?.cpu || existingWorker.total_resources?.cpu || 0,
              memory: telemetry.total_resources?.memory || existingWorker.total_resources?.memory || 0,
              storage: telemetry.total_resources?.storage || existingWorker.total_resources?.storage || 0,
              gpu: telemetry.total_resources?.gpu || existingWorker.total_resources?.gpu || 0,
            },
            allocated_resources: {
              cpu: telemetry.allocated_resources?.cpu || existingWorker.allocated_resources?.cpu || 0,
              memory: telemetry.allocated_resources?.memory || existingWorker.allocated_resources?.memory || 0,
              storage: telemetry.allocated_resources?.storage || existingWorker.allocated_resources?.storage || 0,
              gpu: telemetry.allocated_resources?.gpu || existingWorker.allocated_resources?.gpu || 0,
            },
            available_resources: {
              cpu: telemetry.available_resources?.cpu || existingWorker.available_resources?.cpu || 0,
              memory: telemetry.available_resources?.memory || existingWorker.available_resources?.memory || 0,
              storage: telemetry.available_resources?.storage || existingWorker.available_resources?.storage || 0,
              gpu: telemetry.available_resources?.gpu || existingWorker.available_resources?.gpu || 0,
            },
          });
        } else {
          // New worker from telemetry
          workerMap.set(workerId, {
            worker_id: workerId,
            address: telemetry.worker_ip || 'Unknown',
            is_active: telemetry.is_active,
            cpu_usage: telemetry.cpu_usage,
            memory_usage: telemetry.memory_usage,
            gpu_usage: telemetry.gpu_usage,
            running_tasks_count: telemetry.running_tasks?.length || 0,
            total_resources: {
              cpu: telemetry.total_resources?.cpu || 0,
              memory: telemetry.total_resources?.memory || 0,
              storage: telemetry.total_resources?.storage || 0,
              gpu: telemetry.total_resources?.gpu || 0,
            },
            allocated_resources: {
              cpu: telemetry.allocated_resources?.cpu || 0,
              memory: telemetry.allocated_resources?.memory || 0,
              storage: telemetry.allocated_resources?.storage || 0,
              gpu: telemetry.allocated_resources?.gpu || 0,
            },
            available_resources: {
              cpu: telemetry.available_resources?.cpu || 0,
              memory: telemetry.available_resources?.memory || 0,
              storage: telemetry.available_resources?.storage || 0,
              gpu: telemetry.available_resources?.gpu || 0,
            },
          });
        }
      });

      return Array.from(workerMap.values());
    });
  };

  const handleRegisterSuccess = () => {
    setSuccessMessage('Worker registered successfully!');
    fetchWorkers();
    setTimeout(() => setSuccessMessage(''), 5000);
  };

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="60vh">
        <CircularProgress />
      </Box>
    );
  }

  const calculateUsage = (used, total) => {
    if (!total || total === 0) return 0;
    return (used / total) * 100;
  };

  return (
    <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Box display="flex" alignItems="center" gap={2}>
          <Typography variant="h4">Workers</Typography>
          <Tooltip title={wsConnected ? 'Real-time updates active' : 'Connecting to real-time updates...'}>
            <Chip
              icon={wsConnected ? <WifiIcon /> : <WifiOffIcon />}
              label={wsConnected ? 'Live' : 'Offline'}
              color={wsConnected ? 'success' : 'default'}
              size="small"
              variant="outlined"
            />
          </Tooltip>
        </Box>
        <Box>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={() => setDialogOpen(true)}
            sx={{ mr: 2 }}
          >
            Register Worker
          </Button>
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={fetchWorkers}
          >
            Refresh
          </Button>
        </Box>
      </Box>

      {successMessage && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccessMessage('')}>
          {successMessage}
        </Alert>
      )}

      <WorkerRegistrationDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        onSuccess={handleRegisterSuccess}
      />

      <TableContainer component={Paper} elevation={3}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Worker ID</TableCell>
              <TableCell>Address</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>CPU</TableCell>
              <TableCell>Memory</TableCell>
              <TableCell>GPU</TableCell>
              <TableCell>Storage</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {workers.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} align="center">
                  No workers found
                </TableCell>
              </TableRow>
            ) : (
              workers.map((worker) => {
                const cpuUsage = calculateUsage(
                  worker.allocated_resources?.cpu || 0,
                  worker.total_resources?.cpu || 1
                );
                const memUsage = calculateUsage(
                  worker.allocated_resources?.memory || 0,
                  worker.total_resources?.memory || 1
                );

                return (
                  <TableRow key={worker.worker_id}>
                    <TableCell>
                      <Typography variant="body2" fontFamily="monospace">
                        {worker.worker_id?.substring(0, 8)}...
                      </Typography>
                    </TableCell>
                    <TableCell>{worker.address}</TableCell>
                    <TableCell>
                      <Chip
                        label={worker.is_active ? 'Active' : 'Inactive'}
                        size="small"
                        color={worker.is_active ? 'success' : 'error'}
                      />
                    </TableCell>
                    <TableCell>
                      <Box>
                        <Typography variant="caption">
                          {formatCPU(worker.allocated_resources?.cpu || 0)} / {formatCPU(worker.total_resources?.cpu || 0)}
                        </Typography>
                        <LinearProgress
                          variant="determinate"
                          value={cpuUsage}
                          color={cpuUsage > 80 ? 'error' : 'primary'}
                          sx={{ mt: 0.5 }}
                        />
                      </Box>
                    </TableCell>
                    <TableCell>
                      <Box>
                        <Typography variant="caption">
                          {formatGB(worker.allocated_resources?.memory || 0)} / {formatGB(worker.total_resources?.memory || 0)}
                        </Typography>
                        <LinearProgress
                          variant="determinate"
                          value={memUsage}
                          color={memUsage > 80 ? 'error' : 'primary'}
                          sx={{ mt: 0.5 }}
                        />
                      </Box>
                    </TableCell>
                    <TableCell>
                      {worker.total_resources?.gpu > 0 ? (
                        `${worker.allocated_resources?.gpu || 0} / ${worker.total_resources?.gpu}`
                      ) : (
                        'N/A'
                      )}
                    </TableCell>
                    <TableCell>
                      {formatGB(worker.total_resources?.storage || 0)}
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Container>
  );
};

export default WorkersPage;
