package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"master/internal/db"
	"master/internal/server"
	pb "master/proto"
)

// TaskAPIHandler handles HTTP REST API requests for task management
type TaskAPIHandler struct {
	masterServer *server.MasterServer
	taskDB       *db.TaskDB
	assignmentDB *db.AssignmentDB
	resultDB     *db.ResultDB
	quietMode    bool
}

// NewTaskAPIHandler creates a new task API handler
func NewTaskAPIHandler(ms *server.MasterServer, taskDB *db.TaskDB, assignmentDB *db.AssignmentDB, resultDB *db.ResultDB) *TaskAPIHandler {
	return &TaskAPIHandler{
		masterServer: ms,
		taskDB:       taskDB,
		assignmentDB: assignmentDB,
		resultDB:     resultDB,
		quietMode:    true,
	}
}

// TaskRequest represents the JSON body for task submission
type TaskRequest struct {
	DockerImage     string  `json:"docker_image"`
	Command         string  `json:"command,omitempty"`
	CPURequired     float64 `json:"cpu_required"`
	MemoryRequired  float64 `json:"memory_required"`
	GPURequired     float64 `json:"gpu_required,omitempty"`
	StorageRequired float64 `json:"storage_required,omitempty"`
	UserID          string  `json:"user_id,omitempty"`
}

// TaskResponse represents the JSON response for task operations
type TaskResponse struct {
	TaskID    string                 `json:"task_id"`
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	CreatedAt int64                  `json:"created_at,omitempty"`
	UpdatedAt int64                  `json:"updated_at,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HandleCreateTask handles POST /api/tasks
func (h *TaskAPIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var taskReq TaskRequest
	if err := json.Unmarshal(body, &taskReq); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if taskReq.DockerImage == "" {
		http.Error(w, "Missing required field: docker_image", http.StatusBadRequest)
		return
	}
	if taskReq.CPURequired <= 0 || taskReq.MemoryRequired <= 0 {
		http.Error(w, "Invalid resource requirements", http.StatusBadRequest)
		return
	}

	// Create task protobuf
	task := &pb.Task{
		TaskId:      fmt.Sprintf("task-%d", time.Now().UnixNano()),
		DockerImage: taskReq.DockerImage,
		Command:     taskReq.Command,
		ReqCpu:      taskReq.CPURequired,
		ReqMemory:   taskReq.MemoryRequired,
		ReqStorage:  taskReq.StorageRequired,
		ReqGpu:      taskReq.GPURequired,
		UserId:      taskReq.UserID,
	}

	// Submit task to master server
	ctx := context.Background()
	ack, err := h.masterServer.SubmitTask(ctx, task)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit task: %v", err), http.StatusInternalServerError)
		return
	}

	response := TaskResponse{
		TaskID:  task.TaskId,
		Status:  "queued",
		Message: ack.Message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// HandleListTasks handles GET /api/tasks
func (h *TaskAPIHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.taskDB == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()

	// Get query parameters for filtering
	status := r.URL.Query().Get("status")

	var tasks []*db.Task
	var err error

	// Filter by status if provided
	if status != "" {
		tasks, err = h.taskDB.GetTasksByStatus(ctx, status)
	} else {
		// Get all tasks (we need to implement this method or get by multiple statuses)
		tasks, err = h.taskDB.GetTasksByStatus(ctx, "")
		if err != nil {
			// Try to get tasks of different statuses and combine
			pendingTasks, _ := h.taskDB.GetTasksByStatus(ctx, "pending")
			queuedTasks, _ := h.taskDB.GetTasksByStatus(ctx, "queued")
			runningTasks, _ := h.taskDB.GetTasksByStatus(ctx, "running")
			completedTasks, _ := h.taskDB.GetTasksByStatus(ctx, "completed")
			failedTasks, _ := h.taskDB.GetTasksByStatus(ctx, "failed")

			tasks = append(tasks, pendingTasks...)
			tasks = append(tasks, queuedTasks...)
			tasks = append(tasks, runningTasks...)
			tasks = append(tasks, completedTasks...)
			tasks = append(tasks, failedTasks...)
			err = nil
		}
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve tasks: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var responses []map[string]interface{}
	for _, task := range tasks {
		responses = append(responses, map[string]interface{}{
			"task_id":          task.TaskID,
			"docker_image":     task.DockerImage,
			"command":          task.Command,
			"status":           task.Status,
			"user_id":          task.UserID,
			"cpu_required":     task.ReqCPU,
			"memory_required":  task.ReqMemory,
			"gpu_required":     task.ReqGPU,
			"storage_required": task.ReqStorage,
			"created_at":       task.CreatedAt.Unix(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// HandleGetTask handles GET /api/tasks/:id
func (h *TaskAPIHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from path
	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if taskID == "" || taskID == "api/tasks" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	if h.taskDB == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()

	// Get task from database
	task, err := h.taskDB.GetTask(ctx, taskID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Task not found: %v", err), http.StatusNotFound)
		return
	}

	// Get assignment info if available
	var assignmentInfo map[string]interface{}
	if h.assignmentDB != nil {
		if assignment, err := h.assignmentDB.GetAssignmentByTaskID(ctx, taskID); err == nil {
			assignmentInfo = map[string]interface{}{
				"worker_id":   assignment.WorkerID,
				"assigned_at": assignment.AssignedAt.Unix(),
			}
		}
	}

	// Get result info if available
	var resultInfo map[string]interface{}
	if h.resultDB != nil {
		if result, err := h.resultDB.GetResult(ctx, taskID); err == nil {
			resultInfo = map[string]interface{}{
				"status":       result.Status,
				"completed_at": result.CompletedAt.Unix(),
				"logs":         result.Logs,
			}
		}
	}

	response := map[string]interface{}{
		"task_id":          task.TaskID,
		"docker_image":     task.DockerImage,
		"command":          task.Command,
		"status":           task.Status,
		"user_id":          task.UserID,
		"cpu_required":     task.ReqCPU,
		"memory_required":  task.ReqMemory,
		"gpu_required":     task.ReqGPU,
		"storage_required": task.ReqStorage,
		"created_at":       task.CreatedAt.Unix(),
		"assignment":       assignmentInfo,
		"result":           resultInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleDeleteTask handles DELETE /api/tasks/:id (cancel task)
func (h *TaskAPIHandler) HandleDeleteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from path
	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if taskID == "" || taskID == "api/tasks" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Cancel task
	_, err := h.masterServer.CancelTask(ctx, &pb.TaskID{TaskId: taskID})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to cancel task: %v", err), http.StatusInternalServerError)
		return
	}

	response := TaskResponse{
		TaskID:  taskID,
		Status:  "cancelled",
		Message: "Task cancellation requested",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetTaskLogs handles GET /api/tasks/:id/logs
func (h *TaskAPIHandler) HandleGetTaskLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from path - format is /api/tasks/{id}/logs
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/tasks/"), "/")
	if len(pathParts) < 2 {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}
	taskID := pathParts[0]

	if h.resultDB == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()

	// Get result which contains logs
	result, err := h.resultDB.GetResult(ctx, taskID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Logs not found: %v", err), http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"task_id":      taskID,
		"logs":         result.Logs,
		"status":       result.Status,
		"completed_at": result.CompletedAt.Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
