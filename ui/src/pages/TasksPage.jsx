import React, { useState } from 'react';
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
  IconButton,
  Box,
  CircularProgress,
  Button,
  Tooltip,
} from '@mui/material';
import RefreshIcon from '@mui/icons-material/Refresh';
import DeleteIcon from '@mui/icons-material/Delete';
import AddIcon from '@mui/icons-material/Add';
import UpdateIcon from '@mui/icons-material/Update';
import { useNavigate } from 'react-router-dom';
import { tasksAPI } from '../api/tasks';
import { getStatusColor } from '../utils/formatters';
import { formatRelativeTime } from '../utils/formatters';
import { TASK_TAG_LABELS } from '../utils/constants';
import { useRealTimeTasks } from '../hooks/useRealTimeTasks';

const TasksPage = () => {
  const navigate = useNavigate();
  const { 
    tasks, 
    loading, 
    lastUpdate, 
    fetchTasks, 
    updateTaskStatus, 
    removeTask 
  } = useRealTimeTasks(3000); // Poll every 3 seconds
  
  const [cancelingTask, setCancelingTask] = useState(null);

  const handleCancelTask = async (taskId) => {
    if (!confirm('Are you sure you want to cancel this task?')) return;
    
    setCancelingTask(taskId);
    // Optimistic update
    updateTaskStatus(taskId, 'cancelled');
    
    try {
      await tasksAPI.cancelTask(taskId);
      // Refetch to ensure consistency
      setTimeout(() => fetchTasks(), 500);
    } catch (error) {
      console.error('Failed to cancel task:', error);
      // Revert on error
      fetchTasks();
    } finally {
      setCancelingTask(null);
    }
  };

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="60vh">
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Box display="flex" alignItems="center" gap={2}>
          <Typography variant="h4">Tasks</Typography>
          {lastUpdate && (
            <Tooltip title={`Last updated: ${new Date(lastUpdate).toLocaleTimeString()}`}>
              <Chip
                icon={<UpdateIcon />}
                label="Auto-updating"
                color="primary"
                size="small"
                variant="outlined"
              />
            </Tooltip>
          )}
        </Box>
        <Box>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={() => navigate('/submit')}
            sx={{ mr: 2 }}
          >
            New Task
          </Button>
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={fetchTasks}
          >
            Refresh
          </Button>
        </Box>
      </Box>

      <TableContainer component={Paper} elevation={3}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Task ID</TableCell>
              <TableCell>Docker Image</TableCell>
              <TableCell>Tag</TableCell>
              <TableCell>K-Value</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Resources</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {tasks.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} align="center">
                  No tasks found
                </TableCell>
              </TableRow>
            ) : (
              tasks.map((task) => (
                <TableRow key={task.task_id}>
                  <TableCell>
                    <Typography variant="body2" fontFamily="monospace">
                      {task.task_id?.substring(0, 8)}...
                    </Typography>
                  </TableCell>
                  <TableCell>{task.docker_image}</TableCell>
                  <TableCell>
                    <Chip
                      label={TASK_TAG_LABELS[task.tag] || task.tag}
                      size="small"
                      color="primary"
                      variant="outlined"
                    />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" fontWeight="bold">
                      {task.k_value?.toFixed(1) || 'N/A'}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={task.status}
                      size="small"
                      color={getStatusColor(task.status)}
                    />
                  </TableCell>
                  <TableCell>
                    <Typography variant="caption" display="block">
                      CPU: {task.cpu_required} | RAM: {task.memory_required}GB
                    </Typography>
                    {task.gpu_required > 0 && (
                      <Typography variant="caption" display="block">
                        GPU: {task.gpu_required}
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell>
                    <Typography variant="caption">
                      {formatRelativeTime(task.created_at)}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <IconButton
                      size="small"
                      color="error"
                      onClick={() => handleCancelTask(task.task_id)}
                      disabled={
                        task.status === 'completed' || 
                        task.status === 'cancelled' || 
                        cancelingTask === task.task_id
                      }
                    >
                      <DeleteIcon />
                    </IconButton>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Container>
  );
};

export default TasksPage;
