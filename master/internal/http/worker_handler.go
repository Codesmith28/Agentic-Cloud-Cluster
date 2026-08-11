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

	// Get telemetry data for all workers (contains most up-to-date info)
	allTelemetry := h.telemetryManager.GetAllWorkerTelemetry()

	var workers []map[string]interface{}
	for workerID, telemetry := range allTelemetry {
		workers = append(workers, map[string]interface{}{
			"worker_id":           workerID,
			"is_active":           telemetry.IsActive,
			"cpu_usage":           telemetry.CpuUsage,
			"memory_usage":        telemetry.MemoryUsage,
			"gpu_usage":           telemetry.GpuUsage,
			"running_tasks_count": len(telemetry.RunningTasks),
			"last_update":         telemetry.LastUpdate,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workers)
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
