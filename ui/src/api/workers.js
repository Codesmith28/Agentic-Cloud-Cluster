import apiClient from './client';

export const workersAPI = {
  // Get all workers
  getAllWorkers: () => {
    return apiClient.get('/api/workers');
  },

  // Register a new worker manually
  registerWorker: (workerData) => {
    return apiClient.post('/api/workers', workerData);
  },

  // Get single worker
  getWorker: (workerId) => {
    return apiClient.get(`/api/workers/${workerId}`);
  },

  // Get worker tasks
  getWorkerTasks: (workerId) => {
    return apiClient.get(`/api/workers/${workerId}/tasks`);
  },

  // Get worker metrics
  getWorkerMetrics: (workerId) => {
    return apiClient.get(`/api/workers/${workerId}/metrics`);
  },
};
