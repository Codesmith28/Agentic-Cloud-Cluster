package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"master/internal/db"
	"master/internal/server"
	"master/internal/telemetry"
)

// WorkerAPIHandler handles HTTP REST API requests for worker management
type WorkerAPIHandler struct {
	masterServer     *server.MasterServer
	workerDB         *db.WorkerDB
	assignmentDB     *db.AssignmentDB
	telemetryManager *telemetry.TelemetryManager
	quietMode        bool
}

// NewWorkerAPIHandler creates a new worker API handler
func NewWorkerAPIHandler(ms *server.MasterServer, workerDB *db.WorkerDB, assignmentDB *db.AssignmentDB, telemetryMgr *telemetry.TelemetryManager) *WorkerAPIHandler {
	return &WorkerAPIHandler{
		masterServer:     ms,
		workerDB:         workerDB,
		assignmentDB:     assignmentDB,
		telemetryManager: telemetryMgr,
		quietMode:        true,
	}
}

// HandleListWorkers handles GET /api/workers
func (h *WorkerAPIHandler) HandleListWorkers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get telemetry data for all workers (contains most up-to-date info from connected workers)
	allTelemetry := h.telemetryManager.GetAllWorkerTelemetry()

	// Also get workers from database (includes manually registered workers)
	var dbWorkers []db.WorkerDocument
	if h.workerDB != nil {
		ctx := context.Background()
		var err error
		dbWorkers, err = h.workerDB.GetAllWorkers(ctx)
		if err != nil {
			// Log error but continue with telemetry data
			fmt.Printf("Warning: Failed to fetch workers from database: %v\n", err)
		}
	}

	// Create a map to merge database and telemetry data
	workerMap := make(map[string]map[string]interface{})

	// First, add all workers from database
	for _, dbWorker := range dbWorkers {
		workerMap[dbWorker.WorkerID] = map[string]interface{}{
			"worker_id": dbWorker.WorkerID,
			"address":   dbWorker.WorkerIP,
			"worker_ip": dbWorker.WorkerIP,
			"is_active": dbWorker.IsActive,
			"total_resources": map[string]interface{}{
				"cpu":     dbWorker.TotalCPU,
				"memory":  dbWorker.TotalMemory,
				"storage": dbWorker.TotalStorage,
				"gpu":     dbWorker.TotalGPU,
			},
			"allocated_resources": map[string]interface{}{
				"cpu":     dbWorker.AllocatedCPU,
				"memory":  dbWorker.AllocatedMemory,
				"storage": dbWorker.AllocatedStorage,
				"gpu":     dbWorker.AllocatedGPU,
			},
			"available_resources": map[string]interface{}{
				"cpu":     dbWorker.AvailableCPU,
				"memory":  dbWorker.AvailableMemory,
				"storage": dbWorker.AvailableStorage,
				"gpu":     dbWorker.AvailableGPU,
			},
			"last_heartbeat":      dbWorker.LastHeartbeat,
			"registered_at":       dbWorker.RegisteredAt.Unix(),
			"cpu_usage":           0.0,
			"memory_usage":        0.0,
			"gpu_usage":           0.0,
			"running_tasks_count": 0,
		}
	}

	// Then, overlay telemetry data (real-time data from connected workers)
	for workerID, telemetry := range allTelemetry {
		if existingWorker, exists := workerMap[workerID]; exists {
			// Update with live telemetry data
			existingWorker["is_active"] = telemetry.IsActive
			existingWorker["cpu_usage"] = telemetry.CpuUsage
			existingWorker["memory_usage"] = telemetry.MemoryUsage
			existingWorker["gpu_usage"] = telemetry.GpuUsage
			existingWorker["running_tasks_count"] = len(telemetry.RunningTasks)
			existingWorker["last_update"] = telemetry.LastUpdate
		} else {
			// Worker in telemetry but not in DB (shouldn't happen, but handle it)
			workerMap[workerID] = map[string]interface{}{
				"worker_id":           workerID,
				"is_active":           telemetry.IsActive,
				"cpu_usage":           telemetry.CpuUsage,
				"memory_usage":        telemetry.MemoryUsage,
				"gpu_usage":           telemetry.GpuUsage,
				"running_tasks_count": len(telemetry.RunningTasks),
				"last_update":         telemetry.LastUpdate,
				"total_resources": map[string]interface{}{
					"cpu":     0.0,
					"memory":  0.0,
					"storage": 0.0,
					"gpu":     0.0,
				},
				"allocated_resources": map[string]interface{}{
					"cpu":     0.0,
					"memory":  0.0,
					"storage": 0.0,
					"gpu":     0.0,
				},
			}
		}
	}

	// Convert map to array
	var workers []map[string]interface{}
	for _, worker := range workerMap {
		workers = append(workers, worker)
	}

	// Wrap in response object
	response := map[string]interface{}{
		"workers": workers,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetWorker handles GET /api/workers/:id
func (h *WorkerAPIHandler) HandleGetWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract worker ID from path
	workerID := strings.TrimPrefix(r.URL.Path, "/api/workers/")
	if workerID == "" || workerID == "api/workers" {
		http.Error(w, "Worker ID required", http.StatusBadRequest)
		return
	}

	// Check if we have additional path components (like /tasks)
	pathParts := strings.Split(workerID, "/")
	actualWorkerID := pathParts[0]

	// If this is a request for worker tasks
	if len(pathParts) > 1 && pathParts[1] == "tasks" {
		h.HandleGetWorkerTasks(w, r, actualWorkerID)
		return
	}

	// Get telemetry data
	telemetryData, exists := h.telemetryManager.GetWorkerTelemetry(actualWorkerID)
	if !exists {
		http.Error(w, fmt.Sprintf("Worker %s not found", actualWorkerID), http.StatusNotFound)
		return
	}

	// Get worker info from database if available
	var workerInfo map[string]interface{}
	if h.workerDB != nil {
		ctx := context.Background()
		if worker, err := h.workerDB.GetWorker(ctx, actualWorkerID); err == nil {
			workerInfo = map[string]interface{}{
				"worker_id":      worker.WorkerID,
				"worker_ip":      worker.WorkerIP,
				"total_cpu":      worker.TotalCPU,
				"total_memory":   worker.TotalMemory,
				"total_storage":  worker.TotalStorage,
				"total_gpu":      worker.TotalGPU,
				"registered_at":  worker.RegisteredAt.Unix(),
				"last_heartbeat": worker.LastHeartbeat,
			}
		}
	}

	// Build running tasks info
	var runningTasks []map[string]interface{}
	for _, task := range telemetryData.RunningTasks {
		runningTasks = append(runningTasks, map[string]interface{}{
			"task_id":          task.TaskId,
			"cpu_allocated":    task.CpuAllocated,
			"memory_allocated": task.MemoryAllocated,
			"gpu_allocated":    task.GpuAllocated,
			"status":           task.Status,
		})
	}

	response := map[string]interface{}{
		"worker_id":     actualWorkerID,
		"is_active":     telemetryData.IsActive,
		"cpu_usage":     telemetryData.CpuUsage,
		"memory_usage":  telemetryData.MemoryUsage,
		"gpu_usage":     telemetryData.GpuUsage,
		"running_tasks": runningTasks,
		"last_update":   telemetryData.LastUpdate,
		"worker_info":   workerInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetWorkerTasks handles GET /api/workers/:id/tasks
func (h *WorkerAPIHandler) HandleGetWorkerTasks(w http.ResponseWriter, r *http.Request, workerID string) {
	if h.assignmentDB == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()

	// Get all assignments for this worker
	assignments, err := h.assignmentDB.GetAssignmentsByWorker(ctx, workerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get worker tasks: %v", err), http.StatusInternalServerError)
		return
	}

	var tasks []map[string]interface{}
	for _, assignment := range assignments {
		tasks = append(tasks, map[string]interface{}{
			"task_id":     assignment.TaskID,
			"assigned_at": assignment.AssignedAt.Unix(),
		})
	}

	response := map[string]interface{}{
		"worker_id": workerID,
		"tasks":     tasks,
		"count":     len(tasks),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetWorkerMetrics handles GET /api/workers/:id/metrics
func (h *WorkerAPIHandler) HandleGetWorkerMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract worker ID from path - format is /api/workers/{id}/metrics
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/workers/"), "/")
	if len(pathParts) < 2 {
		http.Error(w, "Worker ID required", http.StatusBadRequest)
		return
	}
	workerID := pathParts[0]

	// Get telemetry data
	telemetryData, exists := h.telemetryManager.GetWorkerTelemetry(workerID)
	if !exists {
		http.Error(w, fmt.Sprintf("Worker %s not found", workerID), http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"worker_id":    workerID,
		"cpu_usage":    telemetryData.CpuUsage,
		"memory_usage": telemetryData.MemoryUsage,
		"gpu_usage":    telemetryData.GpuUsage,
		"is_active":    telemetryData.IsActive,
		"last_update":  telemetryData.LastUpdate,
		"timestamp":    telemetryData.LastUpdate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleRegisterWorker handles POST /api/workers - Manual worker registration
func (h *WorkerAPIHandler) HandleRegisterWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if workerDB is available
	if h.workerDB == nil {
		http.Error(w, "Worker registration is not available (database not connected)", http.StatusServiceUnavailable)
		return
	}

	// Parse request body
	var req struct {
		WorkerID string `json:"worker_id"`
		WorkerIP string `json:"worker_ip"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.WorkerID == "" {
		http.Error(w, "worker_id is required", http.StatusBadRequest)
		return
	}
	if req.WorkerIP == "" {
		http.Error(w, "worker_ip is required", http.StatusBadRequest)
		return
	}

	// Get master info to send to worker
	masterID, masterAddress := h.masterServer.GetMasterInfo()
	if masterID == "" || masterAddress == "" {
		http.Error(w, "Master info not set. Cannot register worker.", http.StatusInternalServerError)
		return
	}

	// Register worker and notify it - this will trigger the worker to connect back with its resources
	ctx := context.Background()
	if err := h.masterServer.ManualRegisterAndNotify(ctx, req.WorkerID, req.WorkerIP, masterID, masterAddress); err != nil {
		http.Error(w, fmt.Sprintf("Failed to register worker: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	response := map[string]interface{}{
		"success":   true,
		"message":   "Worker registered successfully. Master is notifying worker to connect and send resource information.",
		"worker_id": req.WorkerID,
		"worker": map[string]interface{}{
			"worker_id": req.WorkerID,
			"worker_ip": req.WorkerIP,
			"is_active": false, // Will become active when worker connects
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
