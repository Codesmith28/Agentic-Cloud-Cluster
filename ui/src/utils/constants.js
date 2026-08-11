// Task statuses
export const TASK_STATUS = {
  PENDING: 'pending',
  QUEUED: 'queued',
  RUNNING: 'running',
  COMPLETED: 'completed',
  FAILED: 'failed',
  CANCELLED: 'cancelled',
};

// Task tags - NEW FEATURE
export const TASK_TAGS = {
  CPU_LIGHT: 'cpu-light',
  CPU_HEAVY: 'cpu-heavy',
  MEMORY_LIGHT: 'memory-light',
  MEMORY_HEAVY: 'memory-heavy',
  GPU_TRAINING: 'gpu-training',
  MIXED: 'mixed',
};

// Task tag labels for display
export const TASK_TAG_LABELS = {
  'cpu-light': 'CPU Light',
  'cpu-heavy': 'CPU Heavy',
  'memory-light': 'Memory Light',
  'memory-heavy': 'Memory Heavy',
  'gpu-training': 'GPU Training',
  'mixed': 'Mixed',
};

// K-value range - NEW FEATURE
export const K_VALUE = {
  MIN: 1.5,
  MAX: 2.5,
  STEP: 0.1,
  DEFAULT: 2.0,
};

// Generate K-value options (1.5, 1.6, 1.7, ..., 2.4, 2.5)
export const K_VALUE_OPTIONS = [];
for (let i = K_VALUE.MIN; i <= K_VALUE.MAX; i = Math.round((i + K_VALUE.STEP) * 10) / 10) {
  K_VALUE_OPTIONS.push(i);
}

// Refresh intervals (ms)
export const REFRESH_INTERVALS = {
  FAST: 2000,      // 2 seconds
  MEDIUM: 5000,    // 5 seconds
  SLOW: 10000,     // 10 seconds
};

// Resource limits
export const RESOURCE_LIMITS = {
  MIN_CPU: 0.1,
  MAX_CPU: 64,
  MIN_MEMORY: 0.5,
  MAX_MEMORY: 256,
  MIN_STORAGE: 1,
  MAX_STORAGE: 1000,
  MIN_GPU: 0,
  MAX_GPU: 8,
};

// Chart colors
export const CHART_COLORS = {
  CPU: '#3b82f6',      // blue
  MEMORY: '#10b981',   // green
  GPU: '#f59e0b',      // amber
  STORAGE: '#8b5cf6',  // purple
};

// Status icons (Material-UI)
export const STATUS_ICONS = {
  pending: 'HourglassEmpty',
  queued: 'Queue',
  running: 'PlayArrow',
  completed: 'CheckCircle',
  failed: 'Error',
  cancelled: 'Cancel',
};
