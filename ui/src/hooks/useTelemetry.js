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
