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

// Format K-value display
export const formatKValue = (kValue) => {
  return `K=${kValue.toFixed(1)}`;
};

// Get task tag color
export const getTaskTagColor = (tag) => {
  const colors = {
    'cpu-light': '#4caf50',      // green
    'cpu-heavy': '#ff9800',      // orange
    'memory-light': '#2196f3',   // blue
    'memory-heavy': '#f44336',   // red
    'gpu-training': '#9c27b0',   // purple
    'mixed': '#607d8b',          // blue-grey
  };
  return colors[tag] || '#9e9e9e';
};
