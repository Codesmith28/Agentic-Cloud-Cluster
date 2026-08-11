import apiClient from './client';

export const tasksAPI = {
  // Get all tasks
  getAllTasks: (status = null) => {
    const params = status ? { status } : {};
    return apiClient.get('/api/tasks', { params });
  },

  // Get single task
  getTask: (taskId) => {
    return apiClient.get(`/api/tasks/${taskId}`);
  },

  // Submit new task with NEW FIELDS: tag and k_value
  submitTask: (taskData) => {
    // Ensure tag and k_value are included
    const payload = {
      docker_image: taskData.docker_image,
      command: taskData.command || '',
      cpu_required: taskData.cpu_required,
      memory_required: taskData.memory_required,
      storage_required: taskData.storage_required || 5.0,
      gpu_required: taskData.gpu_required || 0.0,
      user_id: taskData.user_id || 'user-001',
      tag: taskData.tag,           // NEW FIELD
      k_value: taskData.k_value,   // NEW FIELD
    };
    return apiClient.post('/api/tasks', payload);
  },

  // Cancel task
  cancelTask: (taskId) => {
    return apiClient.delete(`/api/tasks/${taskId}`);
  },

  // Get task logs
  getTaskLogs: (taskId) => {
    return apiClient.get(`/api/tasks/${taskId}/logs`);
  },
};
