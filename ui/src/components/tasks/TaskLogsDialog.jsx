import React, { useEffect, useState, useRef } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Box,
  Typography,
  Paper,
  Chip,
  CircularProgress,
  IconButton,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import TerminalIcon from '@mui/icons-material/Terminal';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';

const TaskLogsDialog = ({ open, onClose, taskId }) => {
  const [logs, setLogs] = useState([]);
  const [status, setStatus] = useState('connecting');
  const [error, setError] = useState('');
  const [isComplete, setIsComplete] = useState(false);
  const [taskStatus, setTaskStatus] = useState('');
  const wsRef = useRef(null);
  const logsEndRef = useRef(null);

  // Auto-scroll to bottom when new logs arrive
  const scrollToBottom = () => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [logs]);

  useEffect(() => {
    if (!open || !taskId) return;

    // Reset state
    setLogs([]);
    setStatus('connecting');
    setError('');
    setIsComplete(false);
    setTaskStatus('');

    // Connect to WebSocket
    const ws = new WebSocket(`ws://localhost:8080/ws/tasks/${taskId}/logs`);
    wsRef.current = ws;

    ws.onopen = () => {
      setStatus('connected');
      console.log('Connected to task log stream');
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);

        switch (data.type) {
          case 'connected':
            setLogs((prev) => [
              ...prev,
              {
                type: 'system',
                text: `Connected to task ${data.task_id} (User: ${data.user_id})`,
                timestamp: new Date().toISOString(),
              },
            ]);
            break;

          case 'log':
            setLogs((prev) => [
              ...prev,
              {
                type: 'log',
                text: data.line,
                timestamp: new Date().toISOString(),
              },
            ]);
            break;

          case 'complete':
            setIsComplete(true);
            setTaskStatus(data.status);
            setLogs((prev) => [
              ...prev,
              {
                type: 'system',
                text: `Task completed with status: ${data.status}`,
                timestamp: new Date().toISOString(),
              },
            ]);
            break;

          case 'error':
            setError(data.error);
            setStatus('error');
            setLogs((prev) => [
              ...prev,
              {
                type: 'error',
                text: `Error: ${data.error}`,
                timestamp: new Date().toISOString(),
              },
            ]);
            break;

          default:
            console.log('Unknown message type:', data.type);
        }
      } catch (err) {
        console.error('Failed to parse WebSocket message:', err);
      }
    };

    ws.onerror = (err) => {
      console.error('WebSocket error:', err);
      setStatus('error');
      setError('WebSocket connection error');
    };

    ws.onclose = () => {
      setStatus('disconnected');
      if (!isComplete) {
        setLogs((prev) => [
          ...prev,
          {
            type: 'system',
            text: 'Connection closed',
            timestamp: new Date().toISOString(),
          },
        ]);
      }
    };

    // Cleanup on unmount or when dialog closes
    return () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    };
  }, [open, taskId]);

  const handleClose = () => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.close();
    }
    onClose();
  };

  const getStatusColor = () => {
    switch (status) {
      case 'connected':
        return 'success';
      case 'connecting':
        return 'info';
      case 'error':
        return 'error';
      case 'disconnected':
        return 'default';
      default:
        return 'default';
    }
  };

  const getStatusIcon = () => {
    if (isComplete) {
      return taskStatus === 'COMPLETED' ? (
        <CheckCircleIcon sx={{ color: 'success.main' }} />
      ) : (
        <ErrorIcon sx={{ color: 'error.main' }} />
      );
    }
    if (status === 'connecting') {
      return <CircularProgress size={20} />;
    }
    return <TerminalIcon />;
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="lg" fullWidth>
      <DialogTitle>
        <Box display="flex" alignItems="center" justifyContent="space-between">
          <Box display="flex" alignItems="center" gap={1}>
            {getStatusIcon()}
            <Typography variant="h6">Task Logs</Typography>
            <Chip
              label={status}
              color={getStatusColor()}
              size="small"
              sx={{ ml: 1 }}
            />
          </Box>
          <IconButton onClick={handleClose} size="small">
            <CloseIcon />
          </IconButton>
        </Box>
        <Typography variant="caption" color="text.secondary">
          Task ID: {taskId}
        </Typography>
      </DialogTitle>

      <DialogContent dividers>
        <Paper
          sx={{
            backgroundColor: '#1e1e1e',
            color: '#d4d4d4',
            fontFamily: 'monospace',
            fontSize: '0.875rem',
            padding: 2,
            minHeight: '400px',
            maxHeight: '600px',
            overflow: 'auto',
          }}
        >
          {logs.length === 0 && status === 'connecting' && (
            <Box display="flex" alignItems="center" gap={1}>
              <CircularProgress size={16} />
              <Typography variant="body2">Connecting to log stream...</Typography>
            </Box>
          )}

          {logs.map((log, index) => (
            <Box
              key={index}
              sx={{
                mb: 0.5,
                color:
                  log.type === 'error'
                    ? '#f48771'
                    : log.type === 'system'
                    ? '#4ec9b0'
                    : '#d4d4d4',
              }}
            >
              <Typography
                component="pre"
                sx={{
                  margin: 0,
                  fontFamily: 'inherit',
                  fontSize: 'inherit',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                }}
              >
                {log.text}
              </Typography>
            </Box>
          ))}

          <div ref={logsEndRef} />

          {error && (
            <Box sx={{ mt: 2, color: '#f48771' }}>
              <Typography variant="body2">Error: {error}</Typography>
            </Box>
          )}
        </Paper>

        {isComplete && (
          <Box sx={{ mt: 2, textAlign: 'center' }}>
            <Chip
              icon={
                taskStatus === 'COMPLETED' ? (
                  <CheckCircleIcon />
                ) : (
                  <ErrorIcon />
                )
              }
              label={`Task ${taskStatus}`}
              color={taskStatus === 'COMPLETED' ? 'success' : 'error'}
            />
          </Box>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={handleClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
};

export default TaskLogsDialog;
