// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { useState, useEffect, useCallback } from 'react';
import { tasksAPI } from '../api/tasks';

/**
 * Custom hook for real-time task updates
 * Combines REST API polling with optimistic updates
 */
export const useRealTimeTasks = (pollInterval = 3000) => {
  const [tasks, setTasks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [lastUpdate, setLastUpdate] = useState(null);

  const fetchTasks = useCallback(async () => {
    try {
      const response = await tasksAPI.getAllTasks();
      const fetchedTasks = response.data.tasks || [];
      
      // Only update if there are actual changes
      setTasks((prevTasks) => {
        const hasChanges = JSON.stringify(prevTasks) !== JSON.stringify(fetchedTasks);
        return hasChanges ? fetchedTasks : prevTasks;
      });
      
      setLastUpdate(Date.now());
      setLoading(false);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch tasks:', err);
      setError(err.message);
      setLoading(false);
    }
  }, []);

  // Optimistic update for task status
  const updateTaskStatus = useCallback((taskId, newStatus) => {
    setTasks((prevTasks) =>
      prevTasks.map((task) =>
        task.task_id === taskId ? { ...task, status: newStatus } : task
      )
    );
  }, []);

  // Add new task optimistically
  const addTask = useCallback((newTask) => {
    setTasks((prevTasks) => [newTask, ...prevTasks]);
  }, []);

  // Remove task optimistically
  const removeTask = useCallback((taskId) => {
    setTasks((prevTasks) => prevTasks.filter((task) => task.task_id !== taskId));
  }, []);

  useEffect(() => {
    fetchTasks();
    const interval = setInterval(fetchTasks, pollInterval);
    return () => clearInterval(interval);
  }, [fetchTasks, pollInterval]);

  return {
    tasks,
    loading,
    error,
    lastUpdate,
    fetchTasks,
    updateTaskStatus,
    addTask,
    removeTask,
  };
};

export default useRealTimeTasks;
