package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
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
	attemptDB    *db.AttemptDB
	resultDB     *db.ResultDB
	quietMode    bool
}

// NewTaskAPIHandler creates a new task API handler
func NewTaskAPIHandler(ms *server.MasterServer, taskDB *db.TaskDB, assignmentDB *db.AssignmentDB, attemptDB *db.AttemptDB, resultDB *db.ResultDB) *TaskAPIHandler {
	return &TaskAPIHandler{
		masterServer: ms,
		taskDB:       taskDB,
		assignmentDB: assignmentDB,
		attemptDB:    attemptDB,
		resultDB:     resultDB,
		quietMode:    true,
	}
}

// TaskRequest represents the JSON body for task submission
// Uses json.Number to accept both strings and numbers
type TaskRequest struct {
	DockerImage     string      `json:"docker_image"`
	Command         string      `json:"command,omitempty"`
	CPURequired     json.Number `json:"cpu_required"`
	MemoryRequired  json.Number `json:"memory_required"`
	StorageRequired json.Number `json:"storage_required,omitempty"`
	UserID          string      `json:"user_id,omitempty"`
	// New fields
	Tag    string      `json:"tag,omitempty"`
	KValue json.Number `json:"k_value,omitempty"`
}

// parseFloat64 safely parses a json.Number to float64
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

	// Parse request body with size limit (1MB)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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

	// Parse numeric fields
	cpuRequired := parseFloat64(taskReq.CPURequired, 0)
	memoryRequired := parseFloat64(taskReq.MemoryRequired, 0)
	storageRequired := parseFloat64(taskReq.StorageRequired, 1024) // Default 1GB
	kValue := parseFloat64(taskReq.KValue, 0)

	// Validate required fields
	if taskReq.DockerImage == "" {
		http.Error(w, "Missing required field: docker_image", http.StatusBadRequest)
		return
	}
	if cpuRequired <= 0 || memoryRequired <= 0 {
		http.Error(w, "Invalid resource requirements: cpu_required and memory_required must be greater than 0", http.StatusBadRequest)
		return
	}

	// Validate K-value if provided (allowed range 1.5 to 2.5)
	if taskReq.KValue != "" {
		if kValue < 1.5 || kValue > 2.5 {
			http.Error(w, "k_value must be between 1.5 and 2.5", http.StatusBadRequest)
			return
		}
	} else {
		kValue = 2.0 // Default SLA multiplier
	}

	// Create task protobuf with task_type and sla_multiplier
	task := &pb.Task{
		TaskId:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		DockerImage:   taskReq.DockerImage,
		Command:       taskReq.Command,
		ReqCpu:        cpuRequired,
		ReqMemory:     memoryRequired,
		ReqStorage:    storageRequired,
		UserId:        taskReq.UserID,
		TaskType:      taskReq.Tag,         // Set task_type from tag field
		SlaMultiplier: kValue,              // Set SLA multiplier
		TaskName:      taskReq.DockerImage, // Default task name
		SubmittedAt:   time.Now().Unix(),
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

	// Also persist tag and k_value fields for backward compatibility with GUI
	if h.taskDB != nil && (taskReq.Tag != "" || taskReq.KValue != "") {
		if err := h.taskDB.UpdateTaskMetadata(ctx, task.TaskId, taskReq.Tag, kValue); err != nil {
			// If update fails, log warning but don't fail the request
			fmt.Printf("Warning: failed to update task metadata for %s: %v\n", task.TaskId, err)
		}
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
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to retrieve tasks: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		// Get all tasks
		tasks, err = h.taskDB.GetAllTasks(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to retrieve tasks: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Convert to response format - initialize as empty array to avoid null in JSON
	taskList := make([]map[string]interface{}, 0)
	for _, task := range tasks {
		taskList = append(taskList, map[string]interface{}{
			"task_id":             task.TaskID,
			"docker_image":        task.DockerImage,
			"command":             task.Command,
			"status":              task.Status,
			"user_id":             task.UserID,
			"cpu_required":        task.ReqCPU,
			"memory_required":     task.ReqMemory,
			"storage_required":    task.ReqStorage,
			"tag":                 task.Tag,
			"k_value":             task.KValue,
			"current_attempt_id":  task.CurrentAttemptID,
			"current_attempt_no":  task.CurrentAttemptNo,
			"recovery_count":      task.RecoveryCount,
			"last_failure_reason": task.LastFailureReason,
			"created_at":          task.CreatedAt.Unix(),
		})
	}

	// Wrap in response object
	response := map[string]interface{}{
		"tasks": taskList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetTask handles GET /api/tasks/:id
func (h *TaskAPIHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from path and allow subroutes.
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/tasks/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" || pathParts[0] == "api" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}
	taskID := pathParts[0]

	if len(pathParts) > 1 && pathParts[1] == "attempts" {
		h.HandleGetTaskAttempts(w, r, taskID)
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
				"attempt_id":  assignment.AttemptID,
				"attempt_no":  assignment.AttemptNo,
				"assigned_at": assignment.AssignedAt.Unix(),
			}
		}
	}

	var attemptsInfo []map[string]interface{}
	if h.attemptDB != nil {
		if attempts, err := h.attemptDB.GetAttemptsByTask(ctx, taskID); err == nil {
			attemptsInfo = make([]map[string]interface{}, 0, len(attempts))
			for _, attempt := range attempts {
				if attempt == nil {
					continue
				}
				entry := map[string]interface{}{
					"attempt_id":      attempt.AttemptID,
					"attempt_no":      attempt.AttemptNo,
					"worker_id":       attempt.WorkerID,
					"status":          attempt.Status,
					"failure_reason":  attempt.FailureReason,
					"assigned_at":     attempt.AssignedAt.Unix(),
					"last_heartbeat":  attempt.LastHeartbeat,
					"load_at_start":   attempt.LoadAtStart,
					"result_status":   attempt.ResultStatus,
					"result_location": attempt.ResultLocation,
					"output_files":    attempt.OutputFiles,
				}
				if !attempt.CompletedAt.IsZero() {
					entry["completed_at"] = attempt.CompletedAt.Unix()
				}
				attemptsInfo = append(attemptsInfo, entry)
			}
		}
	}

	// Get result info if available
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
		"task_id":             task.TaskID,
		"docker_image":        task.DockerImage,
		"command":             task.Command,
		"status":              task.Status,
		"user_id":             task.UserID,
		"cpu_required":        task.ReqCPU,
		"memory_required":     task.ReqMemory,
		"storage_required":    task.ReqStorage,
		"tag":                 task.Tag,
		"k_value":             task.KValue,
		"current_attempt_id":  task.CurrentAttemptID,
		"current_attempt_no":  task.CurrentAttemptNo,
		"recovery_count":      task.RecoveryCount,
		"last_failure_reason": task.LastFailureReason,
		"last_worker_id":      task.LastWorkerID,
		"created_at":          task.CreatedAt.Unix(),
		"assignment":          assignmentInfo,
		"attempts":            attemptsInfo,
		"result":              resultInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetTaskAttempts handles GET /api/tasks/:id/attempts
func (h *TaskAPIHandler) HandleGetTaskAttempts(w http.ResponseWriter, r *http.Request, taskID string) {
	if h.attemptDB == nil {
		http.Error(w, "Attempt database not available", http.StatusServiceUnavailable)
		return
	}

	attempts, err := h.attemptDB.GetAttemptsByTask(context.Background(), taskID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve attempts: %v", err), http.StatusInternalServerError)
		return
	}

	response := make([]map[string]interface{}, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt == nil {
			continue
		}
		entry := map[string]interface{}{
			"attempt_id":      attempt.AttemptID,
			"attempt_no":      attempt.AttemptNo,
			"worker_id":       attempt.WorkerID,
			"status":          attempt.Status,
			"failure_reason":  attempt.FailureReason,
			"assigned_at":     attempt.AssignedAt.Unix(),
			"last_heartbeat":  attempt.LastHeartbeat,
			"load_at_start":   attempt.LoadAtStart,
			"result_status":   attempt.ResultStatus,
			"result_location": attempt.ResultLocation,
			"output_files":    attempt.OutputFiles,
		}
		if !attempt.CompletedAt.IsZero() {
			entry["completed_at"] = attempt.CompletedAt.Unix()
		}
		response = append(response, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id":  taskID,
		"attempts": response,
	})
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

// HandleTaskLogsStream handles WebSocket connections for streaming live task logs
// WebSocket endpoint: /ws/tasks/:id/logs
func (h *TaskAPIHandler) HandleTaskLogsStream(w http.ResponseWriter, r *http.Request) {
	// Extract task ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/ws/tasks/")
	taskID := strings.TrimSuffix(path, "/logs")

	if taskID == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	// Upgrade to WebSocket (reuse the package-level upgrader with origin check)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed for task %s: %v", taskID, err)
		http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Get task info to get userID
	ctx := context.Background()
	userID, err := h.masterServer.GetUserIDForTask(ctx, taskID)
	if err != nil {
		log.Printf("Failed to get task info for task %s: %v", taskID, err)
		conn.WriteJSON(map[string]interface{}{
			"error": "Failed to get task information",
		})
		return
	}

	// Send initial message
	conn.WriteJSON(map[string]interface{}{
		"type":    "connected",
		"task_id": taskID,
		"user_id": userID,
		"message": "Connected to task log stream",
	})

	// Create context for streaming
	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()

	// Stream logs using the master server's streaming function
	err = h.masterServer.StreamTaskLogsUnified(streamCtx, taskID, userID, func(logLine string, isComplete bool, status string) error {
		if logLine != "" {
			// Send log line
			if err := conn.WriteJSON(map[string]interface{}{
				"type":    "log",
				"line":    logLine,
				"task_id": taskID,
			}); err != nil {
				return err
			}
		}

		if isComplete {
			// Send completion message
			conn.WriteJSON(map[string]interface{}{
				"type":    "complete",
				"task_id": taskID,
				"status":  status,
				"message": fmt.Sprintf("Task completed with status: %s", status),
			})
		}

		return nil
	})

	if err != nil {
		conn.WriteJSON(map[string]interface{}{
			"type":    "error",
			"task_id": taskID,
			"error":   err.Error(),
		})
	}
}
