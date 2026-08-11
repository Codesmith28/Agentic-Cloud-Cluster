import { useEffect, useRef, useState, useCallback } from 'react';

/**
 * Custom hook for WebSocket connections with auto-reconnect
 * @param {string} url - WebSocket URL
 * @param {function} onMessage - Callback for handling messages
 * @param {object} options - Configuration options
 */
export const useWebSocket = (url, onMessage, options = {}) => {
  const {
    reconnectInterval = 3000,
    reconnectAttempts = 10,
    enabled = true,
  } = options;

  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState(null);
  const wsRef = useRef(null);
  const reconnectTimeoutRef = useRef(null);
  const attemptCountRef = useRef(0);
  const shouldConnectRef = useRef(enabled);

  const connect = useCallback(() => {
    if (!shouldConnectRef.current || !url) return;

    try {
      // Close existing connection if any
      if (wsRef.current) {
        wsRef.current.close();
      }

      const ws = new WebSocket(url);

      ws.onopen = () => {
        console.log(`WebSocket connected: ${url}`);
        setIsConnected(true);
        setError(null);
        attemptCountRef.current = 0;
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          onMessage(data);
        } catch (err) {
          console.error('Failed to parse WebSocket message:', err);
        }
      };

      ws.onerror = (event) => {
        console.error('WebSocket error:', event);
        setError('WebSocket connection error');
      };

      ws.onclose = (event) => {
        console.log(`WebSocket closed: ${url}`, event.code, event.reason);
        setIsConnected(false);
        wsRef.current = null;

        // Attempt reconnection
        if (
          shouldConnectRef.current &&
          attemptCountRef.current < reconnectAttempts
        ) {
          attemptCountRef.current += 1;
          console.log(
            `Reconnecting... Attempt ${attemptCountRef.current}/${reconnectAttempts}`
          );
          reconnectTimeoutRef.current = setTimeout(() => {
            connect();
          }, reconnectInterval);
        } else if (attemptCountRef.current >= reconnectAttempts) {
          setError('Max reconnection attempts reached');
        }
      };

      wsRef.current = ws;
    } catch (err) {
      console.error('Failed to create WebSocket:', err);
      setError(err.message);
    }
  }, [url, onMessage, reconnectInterval, reconnectAttempts]);

  const disconnect = useCallback(() => {
    shouldConnectRef.current = false;
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setIsConnected(false);
  }, []);

  const sendMessage = useCallback((data) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data));
      return true;
    }
    console.warn('WebSocket is not connected');
    return false;
  }, []);

  useEffect(() => {
    shouldConnectRef.current = enabled;
    if (enabled) {
      connect();
    } else {
      disconnect();
    }

    return () => {
      disconnect();
    };
  }, [enabled, connect, disconnect]);

  return {
    isConnected,
    error,
    sendMessage,
    reconnect: connect,
  };
};

export default useWebSocket;
