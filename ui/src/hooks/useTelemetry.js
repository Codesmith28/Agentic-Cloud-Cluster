import { useState, useCallback } from 'react';
import { useWebSocket } from './useWebSocket';

const parseTelemetryWorkers = (payload) => {
  if (!payload || typeof payload !== 'object') {
    return null;
  }

  // Preferred format: { type: 'telemetry', workers: { workerId: {...} } }
  if (payload.type === 'telemetry' && payload.workers && typeof payload.workers === 'object') {
    return payload.workers;
  }

  // Alternate wrapped format: { workers: { workerId: {...} } }
  if (payload.workers && typeof payload.workers === 'object') {
    return payload.workers;
  }

  // Single worker payload: { worker_id, cpu_usage, ... }
  if (payload.worker_id && (payload.cpu_usage !== undefined || payload.memory_usage !== undefined)) {
    return {
      [payload.worker_id]: payload,
    };
  }

  // Current master payload: { workerId: { ...telemetry } }
  return payload;
};

/**
 * Custom hook for real-time telemetry updates via WebSocket
 * Connects to the master server's telemetry WebSocket endpoint
 */
export const useTelemetry = () => {
  const [telemetryData, setTelemetryData] = useState({
    workers: {},
    lastUpdate: null,
  });

  const handleTelemetryMessage = useCallback((data) => {
    const workersUpdate = parseTelemetryWorkers(data);
    if (workersUpdate && Object.keys(workersUpdate).length > 0) {
      setTelemetryData((prev) => ({
        workers: {
          ...prev.workers,
          ...workersUpdate,
        },
        lastUpdate: Date.now(),
      }));
    }
  }, []);

  const wsBase =
    import.meta.env.VITE_WS_BASE_URL ||
    `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.hostname}:8080`;
  const wsUrl = `${wsBase.replace(/\/$/, '')}/ws/telemetry`;

  const { isConnected, error } = useWebSocket(
    wsUrl,
    handleTelemetryMessage,
    {
      reconnectInterval: 3000,
      reconnectAttempts: 10,
      enabled: true,
    }
  );

  return {
    telemetryData,
    isConnected,
    error,
  };
};

export default useTelemetry;
