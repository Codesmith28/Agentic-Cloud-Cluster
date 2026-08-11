import { useState, useCallback } from 'react';
import { useWebSocket } from './useWebSocket';

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
    if (data.type === 'telemetry' && data.workers) {
      setTelemetryData({
        workers: data.workers,
        lastUpdate: Date.now(),
      });
    }
  }, []);

  const wsUrl = `ws://${window.location.hostname}:8080/ws/telemetry`;

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
