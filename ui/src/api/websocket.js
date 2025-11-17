const WS_BASE_URL = import.meta.env.VITE_WS_BASE_URL || 'ws://localhost:8080';

class WebSocketManager {
  constructor() {
    this.connections = new Map();
    this.reconnectAttempts = new Map();
    this.maxReconnectAttempts = 5;
    this.reconnectDelay = 3000;
  }

  connect(endpoint, onMessage, onError = null) {
    const url = `${WS_BASE_URL}${endpoint}`;
    
    if (this.connections.has(endpoint)) {
      console.warn(`WebSocket already connected to ${endpoint}`);
      return this.connections.get(endpoint);
    }

    const ws = new WebSocket(url);

    ws.onopen = () => {
      console.log(`WebSocket connected: ${endpoint}`);
      this.reconnectAttempts.set(endpoint, 0);
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        onMessage(data);
      } catch (error) {
        console.error('WebSocket message parse error:', error);
      }
    };

    ws.onerror = (error) => {
      console.error(`WebSocket error on ${endpoint}:`, error);
      if (onError) onError(error);
    };

    ws.onclose = () => {
      console.log(`WebSocket closed: ${endpoint}`);
      this.connections.delete(endpoint);
      this.handleReconnect(endpoint, onMessage, onError);
    };

    this.connections.set(endpoint, ws);
    return ws;
  }

  handleReconnect(endpoint, onMessage, onError) {
    const attempts = this.reconnectAttempts.get(endpoint) || 0;
    
    if (attempts < this.maxReconnectAttempts) {
      setTimeout(() => {
        console.log(`Reconnecting to ${endpoint} (attempt ${attempts + 1})`);
        this.reconnectAttempts.set(endpoint, attempts + 1);
        this.connect(endpoint, onMessage, onError);
      }, this.reconnectDelay);
    } else {
      console.error(`Max reconnection attempts reached for ${endpoint}`);
    }
  }

  disconnect(endpoint) {
    const ws = this.connections.get(endpoint);
    if (ws) {
      ws.close();
      this.connections.delete(endpoint);
      this.reconnectAttempts.delete(endpoint);
    }
  }

  disconnectAll() {
    this.connections.forEach((ws, endpoint) => {
      ws.close();
    });
    this.connections.clear();
    this.reconnectAttempts.clear();
  }
}

export const wsManager = new WebSocketManager();

// Helper functions
export const connectToAllWorkers = (onMessage, onError) => {
  return wsManager.connect('/ws/telemetry', onMessage, onError);
};

export const connectToWorker = (workerId, onMessage, onError) => {
  return wsManager.connect(`/ws/telemetry/${workerId}`, onMessage, onError);
};
