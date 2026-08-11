package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"master/internal/telemetry"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for simplicity (restrict in production)
	},
}

// WSClient represents a WebSocket client connection
type WSClient struct {
	conn     *websocket.Conn
	workerID string // empty string means all workers
	send     chan []byte
}

// TelemetryServer provides WebSocket endpoints to stream worker telemetry data
type TelemetryServer struct {
	telemetryManager *telemetry.TelemetryManager
	server           *http.Server
	mux              *http.ServeMux
	clients          map[*WSClient]bool
	clientsMu        sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	quietMode        bool
}

// NewTelemetryServer creates a new HTTP server with WebSocket endpoints for telemetry streaming
func NewTelemetryServer(port int, telemetryMgr *telemetry.TelemetryManager) *TelemetryServer {
	ctx, cancel := context.WithCancel(context.Background())

	mux := http.NewServeMux()

	ts := &TelemetryServer{
		telemetryManager: telemetryMgr,
		mux:              mux,
		clients:          make(map[*WSClient]bool),
		ctx:              ctx,
		cancel:           cancel,
		quietMode:        true, // Enable quiet mode by default
	}

	// WebSocket endpoints
	mux.HandleFunc("/ws/telemetry", ts.handleAllWorkersWS)
	mux.HandleFunc("/ws/telemetry/", ts.handleWorkerTelemetryWS)

	// REST endpoints
	mux.HandleFunc("/health", ts.handleHealth)
	mux.HandleFunc("/telemetry", ts.handleTelemetryREST)
	mux.HandleFunc("/telemetry/", ts.handleWorkerTelemetryREST)
	mux.HandleFunc("/workers", ts.handleWorkersREST)

	ts.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsMiddleware(mux),
	}

	// Set callback on telemetry manager to broadcast updates
	telemetryMgr.SetUpdateCallback(ts.onTelemetryUpdate)

	return ts
}

// corsMiddleware adds CORS headers to all responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SetQuietMode enables or disables verbose logging
func (ts *TelemetryServer) SetQuietMode(quiet bool) {
	ts.quietMode = quiet
}

// Start starts the HTTP server with WebSocket support
func (ts *TelemetryServer) Start() error {
	log.Printf("Starting WebSocket telemetry server on %s", ts.server.Addr)
	log.Printf("WebSocket endpoints:")
	log.Printf("  - ws://localhost%s/ws/telemetry (all workers)", ts.server.Addr)
	log.Printf("  - ws://localhost%s/ws/telemetry/{worker_id} (specific worker)", ts.server.Addr)
	return ts.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (ts *TelemetryServer) Shutdown() error {
	log.Println("Shutting down WebSocket telemetry server...")
	ts.cancel()

	// Close all WebSocket connections
	ts.clientsMu.Lock()
	for client := range ts.clients {
		client.conn.Close()
		close(client.send)
	}
	ts.clients = make(map[*WSClient]bool)
	ts.clientsMu.Unlock()

	return ts.server.Close()
}

// handleHealth returns a simple health check
func (ts *TelemetryServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"status":         "healthy",
		"time":           time.Now().Unix(),
		"active_clients": ts.getClientCount(),
		"workers":        ts.telemetryManager.GetWorkerCount(),
		"active_workers": ts.telemetryManager.GetActiveWorkerCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTelemetryREST returns telemetry for all workers (REST endpoint)
func (ts *TelemetryServer) handleTelemetryREST(w http.ResponseWriter, r *http.Request) {
	// Only handle exact /telemetry path, not /telemetry/something
	if r.URL.Path != "/telemetry" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allTelemetry := ts.telemetryManager.GetAllWorkerTelemetry()
	jsonData := make(map[string]interface{})
	for workerID, data := range allTelemetry {
		jsonData[workerID] = convertTelemetryToJSON(data)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonData)
}

// handleWorkerTelemetryREST returns telemetry for a specific worker (REST endpoint)
func (ts *TelemetryServer) handleWorkerTelemetryREST(w http.ResponseWriter, r *http.Request) {
	// Extract worker ID from path
	workerID := strings.TrimPrefix(r.URL.Path, "/telemetry/")
	if workerID == "" || workerID == "telemetry" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workerTelemetry, exists := ts.telemetryManager.GetWorkerTelemetry(workerID)
	if !exists {
		http.Error(w, fmt.Sprintf("Worker %s not found", workerID), http.StatusNotFound)
		return
	}

	jsonData := convertTelemetryToJSON(workerTelemetry)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonData)
}

// handleWorkersREST returns basic info for all workers (REST endpoint)
func (ts *TelemetryServer) handleWorkersREST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allTelemetry := ts.telemetryManager.GetAllWorkerTelemetry()
	workersInfo := make(map[string]interface{})

	for workerID, data := range allTelemetry {
		workersInfo[workerID] = map[string]interface{}{
			"worker_id":           data.WorkerID,
			"is_active":           data.IsActive,
			"running_tasks_count": len(data.RunningTasks),
			"last_update":         data.LastUpdate,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workersInfo)
}

// handleAllWorkersWS handles WebSocket connections for streaming all workers' telemetry
func (ts *TelemetryServer) handleAllWorkersWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &WSClient{
		conn:     conn,
		workerID: "", // empty means all workers
		send:     make(chan []byte, 256),
	}

	ts.registerClient(client)
	defer ts.unregisterClient(client)

	if !ts.quietMode {
		log.Printf("WebSocket client connected (all workers)")
	}

	// Send initial telemetry data
	allTelemetry := ts.telemetryManager.GetAllWorkerTelemetry()
	jsonData := make(map[string]interface{})
	for workerID, data := range allTelemetry {
		jsonData[workerID] = convertTelemetryToJSON(data)
	}
	ts.sendTelemetryToClient(client, jsonData)

	// Start goroutine to handle writes
	go ts.writePump(client)

	// Handle reads (for ping/pong)
	ts.readPump(client)
}

// handleWorkerTelemetryWS handles WebSocket connections for streaming a specific worker's telemetry
func (ts *TelemetryServer) handleWorkerTelemetryWS(w http.ResponseWriter, r *http.Request) {
	// Extract worker ID from path
	workerID := strings.TrimPrefix(r.URL.Path, "/ws/telemetry/")
	if workerID == "" {
		http.Error(w, "Worker ID required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &WSClient{
		conn:     conn,
		workerID: workerID,
		send:     make(chan []byte, 256),
	}

	ts.registerClient(client)
	defer ts.unregisterClient(client)

	if !ts.quietMode {
		log.Printf("WebSocket client connected (worker: %s)", workerID)
	}

	// Send initial telemetry data for this worker
	if workerTelemetry, exists := ts.telemetryManager.GetWorkerTelemetry(workerID); exists {
		jsonData := map[string]interface{}{
			workerID: convertTelemetryToJSON(workerTelemetry),
		}
		ts.sendTelemetryToClient(client, jsonData)
	}

	// Start goroutine to handle writes	// Start goroutine to handle writes
	go ts.writePump(client)

	// Handle reads (for ping/pong)
	ts.readPump(client)
}

// onTelemetryUpdate is called when telemetry is updated for a worker
func (ts *TelemetryServer) onTelemetryUpdate(workerID string, data *telemetry.WorkerTelemetryData) {
	ts.clientsMu.RLock()
	defer ts.clientsMu.RUnlock()

	// Convert to JSON-friendly format
	telemetryData := map[string]interface{}{
		workerID: convertTelemetryToJSON(data),
	}

	for client := range ts.clients {
		// Send to all-workers clients or clients watching this specific worker
		if client.workerID == "" || client.workerID == workerID {
			ts.sendTelemetryToClient(client, telemetryData)
		}
	}
}

// convertTelemetryToJSON converts WorkerTelemetryData to JSON-friendly map
func convertTelemetryToJSON(data *telemetry.WorkerTelemetryData) map[string]interface{} {
	tasks := make([]map[string]interface{}, 0, len(data.RunningTasks))
	for _, task := range data.RunningTasks {
		tasks = append(tasks, map[string]interface{}{
			"task_id":          task.TaskId,
			"cpu_allocated":    task.CpuAllocated,
			"memory_allocated": task.MemoryAllocated,
			"gpu_allocated":    task.GpuAllocated,
			"status":           task.Status,
		})
	}

	return map[string]interface{}{
		"worker_id":     data.WorkerID,
		"cpu_usage":     data.CpuUsage,
		"memory_usage":  data.MemoryUsage,
		"gpu_usage":     data.GpuUsage,
		"running_tasks": tasks,
		"last_update":   data.LastUpdate,
		"is_active":     data.IsActive,
	}
}

// sendTelemetryToClient sends telemetry data to a WebSocket client
func (ts *TelemetryServer) sendTelemetryToClient(client *WSClient, data map[string]interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling telemetry: %v", err)
		return
	}

	select {
	case client.send <- jsonData:
	default:
		// Channel full, skip this update
	}
}

// readPump handles reading from the WebSocket connection
func (ts *TelemetryServer) readPump(client *WSClient) {
	defer func() {
		ts.unregisterClient(client)
		client.conn.Close()
	}()

	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				if !ts.quietMode {
					log.Printf("WebSocket error: %v", err)
				}
			}
			break
		}
	}
}

// writePump handles writing to the WebSocket connection
func (ts *TelemetryServer) writePump(client *WSClient) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-ts.ctx.Done():
			return
		}
	}
}

// registerClient registers a new WebSocket client
func (ts *TelemetryServer) registerClient(client *WSClient) {
	ts.clientsMu.Lock()
	defer ts.clientsMu.Unlock()
	ts.clients[client] = true
}

// unregisterClient unregisters a WebSocket client
func (ts *TelemetryServer) unregisterClient(client *WSClient) {
	ts.clientsMu.Lock()
	defer ts.clientsMu.Unlock()
	if _, ok := ts.clients[client]; ok {
		delete(ts.clients, client)
		close(client.send)
		if !ts.quietMode {
			log.Printf("WebSocket client disconnected")
		}
	}
}

// getClientCount returns the number of active WebSocket clients
func (ts *TelemetryServer) getClientCount() int {
	ts.clientsMu.RLock()
	defer ts.clientsMu.RUnlock()
	return len(ts.clients)
}

// RegisterTaskHandlers registers task API handlers
func (ts *TelemetryServer) RegisterTaskHandlers(handler *TaskAPIHandler) {
	ts.mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handler.HandleCreateTask(w, r)
		case http.MethodGet:
			handler.HandleListTasks(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// WebSocket endpoint for live task logs
	ts.mux.HandleFunc("/ws/tasks/", handler.HandleTaskLogsStream)

	ts.mux.HandleFunc("/api/tasks/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a /logs or /retry request
		if strings.Contains(r.URL.Path, "/logs") {
			handler.HandleGetTaskLogs(w, r)
		} else {
			// Handle GET /api/tasks/{id} or DELETE /api/tasks/{id}
			switch r.Method {
			case http.MethodGet:
				handler.HandleGetTask(w, r)
			case http.MethodDelete:
				handler.HandleDeleteTask(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})
}

// RegisterWorkerHandlers registers worker API handlers
func (ts *TelemetryServer) RegisterWorkerHandlers(handler *WorkerAPIHandler) {
	ts.mux.HandleFunc("/api/workers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.HandleRegisterWorker(w, r)
		} else {
			handler.HandleListWorkers(w, r)
		}
	})
	ts.mux.HandleFunc("/api/workers/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a /metrics request
		if strings.Contains(r.URL.Path, "/metrics") {
			handler.HandleGetWorkerMetrics(w, r)
		} else {
			handler.HandleGetWorker(w, r)
		}
	})
}

// RegisterFileHandlers registers file API handlers
func (ts *TelemetryServer) RegisterFileHandlers(handler *FileAPIHandler) {
	// List all files for a user
	ts.mux.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler.HandleListFiles(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Handle specific file operations: get task files, download file, delete files
	ts.mux.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a download request: /api/files/{task_id}/download/{file_path}
		if strings.Contains(r.URL.Path, "/download") {
			handler.HandleDownloadFile(w, r)
		} else {
			// /api/files/{task_id} - get task files or delete task files
			switch r.Method {
			case http.MethodGet:
				handler.HandleGetTaskFiles(w, r)
			case http.MethodDelete:
				handler.HandleDeleteTaskFiles(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})
}
