package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"master/internal/db"
	"master/internal/server"
	pb "master/proto"

	"github.com/gorilla/websocket"
)

// TaskAPIHandler handles HTTP REST API requests for task management.
type TaskAPIHandler struct {
	masterServer *server.MasterServer
	taskDB       *db.TaskDB
	assignmentDB *db.AssignmentDB
	resultDB     *db.ResultDB
	quietMode    bool
}

// NewTaskAPIHandler creates a new task API handler.
func NewTaskAPIHandler(ms *server.MasterServer, taskDB *db.TaskDB, assignmentDB *db.AssignmentDB, resultDB *db.ResultDB) *TaskAPIHandler {
	return &TaskAPIHandler{
		masterServer: ms,
		taskDB:       taskDB,
		assignmentDB: assignmentDB,
		resultDB:     resultDB,
		quietMode:    true,
	}
}

// TaskRequest represents the JSON body for task submission.
// Uses json.Number to accept both strings and numbers.
type TaskRequest struct {
	DockerImage     string      `json:"docker_image"`
	Command         string      `json:"command,omitempty"`
	CPURequired     json.Number `json:"cpu_required"`
	MemoryRequired  json.Number `json:"memory_required"`
	GPURequired     json.Number `json:"gpu_required,omitempty"`
	StorageRequired json.Number `json:"storage_required,omitempty"`
	Tag             string      `json:"tag,omitempty"`
	KValue          json.Number `json:"k_value,omitempty"`
}

// TaskResponse represents the JSON response for task operations.
type TaskResponse struct {
	TaskID    string                 `json:"task_id"`
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	CreatedAt int64                  `json:"created_at,omitempty"`
	UpdatedAt int64                  `json:"updated_at,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

func parseFloat64(num json.Number, defaultVal float64) float64 {
	if num == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(string(num), 64)
	if err != nil {
		return defaultVal
	}
	return val
}

func requirePrincipalForTask(w http.ResponseWriter, r *http.Request) (*AuthPrincipal, bool) {
	principal, ok := getAuthPrincipal(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	return principal, true
}

func ensureAdminEndpoint(principal *AuthPrincipal, w http.ResponseWriter) bool {
	if principal == nil || !principal.IsAdmin() {
		http.Error(w, "Forbidden: admin role required", http.StatusForbidden)
		return false
	}
	return true
}

func extractTaskID(path, prefix string) (string, error) {
	if !strings.HasPrefix(path, prefix) {
		return "", fmt.Errorf("invalid path")
	}
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return "", fmt.Errorf("task ID required")
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", fmt.Errorf("task ID required")
	}
	return parts[0], nil
}

func extractTaskIDFromLogsPath(path, prefix string) (string, error) {
	if !strings.HasPrefix(path, prefix) {
		return "", fmt.Errorf("invalid path")
	}
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "logs" {
		return "", fmt.Errorf("invalid logs path")
	}
	return parts[0], nil
}

func (h *TaskAPIHandler) loadTask(ctx context.Context, taskID string) (*db.Task, error) {
	if h.taskDB == nil {
		return nil, fmt.Errorf("database not available")
	}
	task, err := h.taskDB.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

func (h *TaskAPIHandler) authorizeResultReadOrReject(w http.ResponseWriter, r *http.Request, principal *AuthPrincipal, owner, resource string) bool {
	if err := authorizeTaskResultRead(principal, owner, resource, r.Header.Get(breakglassReasonHeader)); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return false
	}
	return true
}

// HandleCreateTask handles POST /api/tasks.
func (h *TaskAPIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	principal, ok := requirePrincipalForTask(w, r)
	if !ok {
		return
	}

	var taskReq TaskRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&taskReq); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			http.Error(w, "Invalid JSON: multiple JSON payloads", http.StatusBadRequest)
		} else {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		}
		return
	}

	cpuRequired := parseFloat64(taskReq.CPURequired, 0)
	memoryRequired := parseFloat64(taskReq.MemoryRequired, 0)
	gpuRequired := parseFloat64(taskReq.GPURequired, 0)
	storageRequired := parseFloat64(taskReq.StorageRequired, 1024)
	kValue := parseFloat64(taskReq.KValue, 0)

	if strings.TrimSpace(taskReq.DockerImage) == "" {
		http.Error(w, "Missing required field: docker_image", http.StatusBadRequest)
		return
	}
	if cpuRequired <= 0 || memoryRequired <= 0 {
		http.Error(w, "Invalid resource requirements: cpu_required and memory_required must be greater than 0", http.StatusBadRequest)
		return
	}

	if taskReq.KValue != "" {
		if kValue < 1.5 || kValue > 2.5 {
			http.Error(w, "k_value must be between 1.5 and 2.5", http.StatusBadRequest)
			return
		}
	} else {
		kValue = 2.0
	}

	task := &pb.Task{
		TaskId:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		DockerImage:   taskReq.DockerImage,
		Command:       taskReq.Command,
		ReqCpu:        cpuRequired,
		ReqMemory:     memoryRequired,
		ReqStorage:    storageRequired,
		ReqGpu:        gpuRequired,
		UserId:        principal.Email,
		TaskType:      taskReq.Tag,
		SlaMultiplier: kValue,
		TaskName:      taskReq.DockerImage,
		SubmittedAt:   time.Now().Unix(),
	}

	ack, err := h.masterServer.SubmitTask(context.Background(), task)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit task: %v", err), http.StatusInternalServerError)
		return
	}

	if h.taskDB != nil && (taskReq.Tag != "" || taskReq.KValue != "") {
		if err := h.taskDB.UpdateTaskMetadata(context.Background(), task.TaskId, taskReq.Tag, kValue); err != nil {
			fmt.Printf("Warning: failed to update task metadata for %s: %v\n", task.TaskId, err)
		}
	}

	response := TaskResponse{
		TaskID:  task.TaskId,
		Status:  "queued",
		Message: ack.Message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

// HandleListTasks handles GET /api/tasks.
func (h *TaskAPIHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	principal, ok := requirePrincipalForTask(w, r)
	if !ok {
		return
	}

	if h.taskDB == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	ctx := context.Background()

	var tasks []*db.Task
	var err error

	if principal.IsAdmin() {
		if statusFilter != "" {
			tasks, err = h.taskDB.GetTasksByStatus(ctx, statusFilter)
		} else {
			tasks, err = h.taskDB.GetAllTasks(ctx)
		}
	} else {
		tasks, err = h.taskDB.GetTasksByUser(ctx, principal.Email)
		if err == nil && statusFilter != "" {
			filtered := make([]*db.Task, 0, len(tasks))
			for _, task := range tasks {
				if task != nil && strings.EqualFold(task.Status, statusFilter) {
					filtered = append(filtered, task)
				}
			}
			tasks = filtered
		}
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve tasks: %v", err), http.StatusInternalServerError)
		return
	}

	taskList := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		taskList = append(taskList, map[string]interface{}{
			"task_id":          task.TaskID,
			"docker_image":     task.DockerImage,
			"command":          task.Command,
			"status":           task.Status,
			"user_id":          task.UserID,
			"cpu_required":     task.ReqCPU,
			"memory_required":  task.ReqMemory,
			"gpu_required":     task.ReqGPU,
			"storage_required": task.ReqStorage,
			"tag":              task.Tag,
			"k_value":          task.KValue,
			"created_at":       task.CreatedAt.Unix(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": taskList,
	})
}

// HandleGetTask handles GET /api/tasks/:id.
func (h *TaskAPIHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	h.handleGetTaskWithPrefix(w, r, "/api/tasks/", false)
}

// HandleAdminGetTask handles GET /api/admin/tasks/:id.
func (h *TaskAPIHandler) HandleAdminGetTask(w http.ResponseWriter, r *http.Request) {
	h.handleGetTaskWithPrefix(w, r, "/api/admin/tasks/", true)
}

func (h *TaskAPIHandler) handleGetTaskWithPrefix(w http.ResponseWriter, r *http.Request, prefix string, adminEndpoint bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	principal, ok := requirePrincipalForTask(w, r)
	if !ok {
		return
	}
	if adminEndpoint && !ensureAdminEndpoint(principal, w) {
		return
	}

	taskID, err := extractTaskID(r.URL.Path, prefix)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	task, err := h.loadTask(ctx, taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if !h.authorizeResultReadOrReject(w, r, principal, task.UserID, fmt.Sprintf("task:%s", taskID)) {
		return
	}

	var assignmentInfo map[string]interface{}
	if h.assignmentDB != nil {
		if assignment, err := h.assignmentDB.GetAssignmentByTaskID(ctx, taskID); err == nil && assignment != nil {
			assignmentInfo = map[string]interface{}{
				"worker_id":   assignment.WorkerID,
				"assigned_at": assignment.AssignedAt.Unix(),
			}
		}
	}

	var resultInfo map[string]interface{}
	if h.resultDB != nil {
		if result, err := h.resultDB.GetResult(ctx, taskID); err == nil && result != nil {
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
		"tag":              task.Tag,
		"k_value":          task.KValue,
		"created_at":       task.CreatedAt.Unix(),
		"assignment":       assignmentInfo,
		"result":           resultInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// HandleDeleteTask handles DELETE /api/tasks/:id (cancel task).
func (h *TaskAPIHandler) HandleDeleteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	principal, ok := requirePrincipalForTask(w, r)
	if !ok {
		return
	}

	taskID, err := extractTaskID(r.URL.Path, "/api/tasks/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task, err := h.loadTask(context.Background(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if !canOperateTask(principal, task.UserID) {
		http.Error(w, "Forbidden: cannot operate on another user's task", http.StatusForbidden)
		return
	}

	if _, err := h.masterServer.CancelTask(context.Background(), &pb.TaskID{TaskId: taskID}); err != nil {
		http.Error(w, fmt.Sprintf("Failed to cancel task: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(TaskResponse{
		TaskID:  taskID,
		Status:  "cancelled",
		Message: "Task cancellation requested",
	})
}

// HandleGetTaskLogs handles GET /api/tasks/:id/logs.
func (h *TaskAPIHandler) HandleGetTaskLogs(w http.ResponseWriter, r *http.Request) {
	h.handleGetTaskLogsWithPrefix(w, r, "/api/tasks/", false)
}

// HandleAdminGetTaskLogs handles GET /api/admin/tasks/:id/logs.
func (h *TaskAPIHandler) HandleAdminGetTaskLogs(w http.ResponseWriter, r *http.Request) {
	h.handleGetTaskLogsWithPrefix(w, r, "/api/admin/tasks/", true)
}

func (h *TaskAPIHandler) handleGetTaskLogsWithPrefix(w http.ResponseWriter, r *http.Request, prefix string, adminEndpoint bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	principal, ok := requirePrincipalForTask(w, r)
	if !ok {
		return
	}
	if adminEndpoint && !ensureAdminEndpoint(principal, w) {
		return
	}

	taskID, err := extractTaskIDFromLogsPath(r.URL.Path, prefix)
	if err != nil {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	if h.resultDB == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	task, err := h.loadTask(context.Background(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if !h.authorizeResultReadOrReject(w, r, principal, task.UserID, fmt.Sprintf("task_logs:%s", taskID)) {
		return
	}

	result, err := h.resultDB.GetResult(context.Background(), taskID)
	if err != nil || result == nil {
		http.Error(w, fmt.Sprintf("Logs not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id":      taskID,
		"logs":         result.Logs,
		"status":       result.Status,
		"completed_at": result.CompletedAt.Unix(),
	})
}

// HandleTaskLogsStream handles WebSocket connections for streaming live task logs.
// WebSocket endpoint: /ws/tasks/:id/logs.
func (h *TaskAPIHandler) HandleTaskLogsStream(w http.ResponseWriter, r *http.Request) {
	h.handleTaskLogsStreamWithPrefix(w, r, "/ws/tasks/", false)
}

// HandleAdminTaskLogsStream handles WebSocket connections on /ws/admin/tasks/:id/logs.
func (h *TaskAPIHandler) HandleAdminTaskLogsStream(w http.ResponseWriter, r *http.Request) {
	h.handleTaskLogsStreamWithPrefix(w, r, "/ws/admin/tasks/", true)
}

func (h *TaskAPIHandler) handleTaskLogsStreamWithPrefix(w http.ResponseWriter, r *http.Request, prefix string, adminEndpoint bool) {
	principal, ok := requirePrincipalForTask(w, r)
	if !ok {
		return
	}
	if adminEndpoint && !ensureAdminEndpoint(principal, w) {
		return
	}

	taskID, err := extractTaskIDFromLogsPath(r.URL.Path, prefix)
	if err != nil {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	task, err := h.loadTask(context.Background(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if !h.authorizeResultReadOrReject(w, r, principal, task.UserID, fmt.Sprintf("task_logs_ws:%s", taskID)) {
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("WebSocket upgrade failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	_ = conn.WriteJSON(map[string]interface{}{
		"type":    "connected",
		"task_id": taskID,
		"message": "Connected to task log stream",
	})

	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()

	err = h.masterServer.StreamTaskLogsUnified(streamCtx, taskID, task.UserID, func(logLine string, isComplete bool, status string) error {
		if logLine != "" {
			if err := conn.WriteJSON(map[string]interface{}{
				"type":    "log",
				"line":    logLine,
				"task_id": taskID,
			}); err != nil {
				return err
			}
		}

		if isComplete {
			_ = conn.WriteJSON(map[string]interface{}{
				"type":    "complete",
				"task_id": taskID,
				"status":  status,
				"message": fmt.Sprintf("Task completed with status: %s", status),
			})
		}
		return nil
	})

	if err != nil {
		_ = conn.WriteJSON(map[string]interface{}{
			"type":    "error",
			"task_id": taskID,
			"error":   err.Error(),
		})
	}
}
