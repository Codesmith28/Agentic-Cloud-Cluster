package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"master/internal/db"
	"master/internal/storage"
)

// FileAPIHandler handles HTTP REST API requests for file management.
type FileAPIHandler struct {
	fileStorage *storage.FileStorageService
	taskDB      *db.TaskDB
	quietMode   bool
}

// NewFileAPIHandler creates a new file API handler.
func NewFileAPIHandler(fileStorage *storage.FileStorageService, taskDB *db.TaskDB) *FileAPIHandler {
	return &FileAPIHandler{
		fileStorage: fileStorage,
		taskDB:      taskDB,
		quietMode:   true,
	}
}

type FileListResponse struct {
	UserID string         `json:"user_id"`
	Tasks  []TaskFileInfo `json:"tasks"`
	Count  int            `json:"count"`
}

type FileInfoJSON struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type TaskFileInfo struct {
	TaskID    string         `json:"task_id"`
	TaskName  string         `json:"task_name"`
	Timestamp string         `json:"timestamp"`
	Files     []FileInfoJSON `json:"files"`
	TotalSize int64          `json:"total_size"`
}

type FileDetailResponse struct {
	TaskID    string         `json:"task_id"`
	TaskName  string         `json:"task_name"`
	Timestamp string         `json:"timestamp"`
	Files     []FileInfoJSON `json:"files"`
	TotalSize int64          `json:"total_size"`
}

func rejectLegacyIdentityQuery(r *http.Request) error {
	q := r.URL.Query()
	if q.Has("user_id") || q.Has("requesting_user") {
		return fmt.Errorf("legacy identity query parameters are not supported")
	}
	return nil
}

func (h *FileAPIHandler) requirePrincipal(w http.ResponseWriter, r *http.Request) (*AuthPrincipal, bool) {
	principal, ok := getAuthPrincipal(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	return principal, true
}

func (h *FileAPIHandler) taskOwnerForTaskID(ctx context.Context, taskID string) (string, error) {
	if h.taskDB == nil {
		return "", fmt.Errorf("task database not available")
	}
	task, err := h.taskDB.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return "", fmt.Errorf("task %s not found", taskID)
	}
	return task.UserID, nil
}

func (h *FileAPIHandler) extractTaskID(path, prefix string) (string, error) {
	if !strings.HasPrefix(path, prefix) {
		return "", fmt.Errorf("invalid URL format")
	}
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return "", fmt.Errorf("task ID required")
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", fmt.Errorf("task ID required")
	}
	return parts[0], nil
}

func (h *FileAPIHandler) authorizeRead(w http.ResponseWriter, r *http.Request, principal *AuthPrincipal, owner, resource string) bool {
	if err := authorizeTaskResultRead(principal, owner, resource, r.Header.Get(breakglassReasonHeader)); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return false
	}
	return true
}

func toJSONFileList(fileMetadataList []*storage.FileMetadata) []TaskFileInfo {
	tasks := make([]TaskFileInfo, 0, len(fileMetadataList))
	for _, metadata := range fileMetadataList {
		if metadata == nil {
			continue
		}
		files := make([]FileInfoJSON, 0, len(metadata.Files))
		for _, f := range metadata.Files {
			files = append(files, FileInfoJSON{Path: f.Path, Size: f.Size})
		}
		tasks = append(tasks, TaskFileInfo{
			TaskID:    metadata.TaskID,
			TaskName:  metadata.TaskName,
			Timestamp: metadata.Timestamp.Format("2006-01-02 15:04:05"),
			Files:     files,
			TotalSize: metadata.TotalSize,
		})
	}
	return tasks
}

func toJSONFileDetail(metadata *storage.FileMetadata) FileDetailResponse {
	files := make([]FileInfoJSON, 0, len(metadata.Files))
	for _, f := range metadata.Files {
		files = append(files, FileInfoJSON{Path: f.Path, Size: f.Size})
	}
	return FileDetailResponse{
		TaskID:    metadata.TaskID,
		TaskName:  metadata.TaskName,
		Timestamp: metadata.Timestamp.Format("2006-01-02 15:04:05"),
		Files:     files,
		TotalSize: metadata.TotalSize,
	}
}

// HandleListFiles handles GET /api/files.
// Returns only authenticated user's files.
func (h *FileAPIHandler) HandleListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := rejectLegacyIdentityQuery(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	if h.fileStorage == nil {
		http.Error(w, "File storage not available", http.StatusServiceUnavailable)
		return
	}

	fileMetadataList, err := h.fileStorage.ListUserFilesWithAccess(principal.Email, principal.Email)
	if err != nil {
		log.Printf("Error listing files for user %s: %v", principal.Email, err)
		http.Error(w, fmt.Sprintf("Failed to list files: %v", err), http.StatusInternalServerError)
		return
	}

	tasks := toJSONFileList(fileMetadataList)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(FileListResponse{
		UserID: principal.Email,
		Tasks:  tasks,
		Count:  len(tasks),
	})
}

// HandleGetTaskFiles handles GET /api/files/{task_id}.
func (h *FileAPIHandler) HandleGetTaskFiles(w http.ResponseWriter, r *http.Request) {
	h.handleGetTaskFilesWithPrefix(w, r, "/api/files/", false)
}

// HandleAdminGetTaskFiles handles GET /api/admin/files/{task_id}.
func (h *FileAPIHandler) HandleAdminGetTaskFiles(w http.ResponseWriter, r *http.Request) {
	h.handleGetTaskFilesWithPrefix(w, r, "/api/admin/files/", true)
}

func (h *FileAPIHandler) handleGetTaskFilesWithPrefix(w http.ResponseWriter, r *http.Request, prefix string, adminEndpoint bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := rejectLegacyIdentityQuery(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	if adminEndpoint && !principal.IsAdmin() {
		http.Error(w, "Forbidden: admin role required", http.StatusForbidden)
		return
	}
	if h.fileStorage == nil {
		http.Error(w, "File storage not available", http.StatusServiceUnavailable)
		return
	}

	taskID, err := h.extractTaskID(r.URL.Path, prefix)
	if err != nil {
		http.Error(w, "Invalid URL format. Expected /api/files/{task_id}", http.StatusBadRequest)
		return
	}

	owner, err := h.taskOwnerForTaskID(context.Background(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if !h.authorizeRead(w, r, principal, owner, fmt.Sprintf("task_files:%s", taskID)) {
		return
	}

	metadata, err := h.fileStorage.GetTaskFiles(owner, taskID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to get task files: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toJSONFileDetail(metadata))
}

// HandleDownloadFile handles GET /api/files/{task_id}/download/{file_path}.
func (h *FileAPIHandler) HandleDownloadFile(w http.ResponseWriter, r *http.Request) {
	h.handleDownloadFileWithPrefix(w, r, "/api/files/", false)
}

// HandleAdminDownloadFile handles GET /api/admin/files/{task_id}/download/{file_path}.
func (h *FileAPIHandler) HandleAdminDownloadFile(w http.ResponseWriter, r *http.Request) {
	h.handleDownloadFileWithPrefix(w, r, "/api/admin/files/", true)
}

func (h *FileAPIHandler) handleDownloadFileWithPrefix(w http.ResponseWriter, r *http.Request, prefix string, adminEndpoint bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := rejectLegacyIdentityQuery(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	if adminEndpoint && !principal.IsAdmin() {
		http.Error(w, "Forbidden: admin role required", http.StatusForbidden)
		return
	}
	if h.fileStorage == nil {
		http.Error(w, "File storage not available", http.StatusServiceUnavailable)
		return
	}

	pathRemainder := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(pathRemainder, "/")
	if len(parts) < 3 || parts[1] != "download" {
		http.Error(w, "Invalid URL format. Expected /api/files/{task_id}/download/{file_path}", http.StatusBadRequest)
		return
	}
	taskID := parts[0]
	filePath := strings.Join(parts[2:], "/")

	owner, err := h.taskOwnerForTaskID(context.Background(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if !h.authorizeRead(w, r, principal, owner, fmt.Sprintf("file_download:%s/%s", taskID, filePath)) {
		return
	}

	if err := h.fileStorage.GetAccessControl().ValidateFilePath(filePath); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fullPath, err := h.fileStorage.GetFilePath(owner, taskID, filePath)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to locate file: %v", err), http.StatusInternalServerError)
		return
	}

	fileData, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	filename := filepath.Base(filePath)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileData)))
	if _, err := w.Write(fileData); err != nil {
		log.Printf("Error writing file data: %v", err)
	}
}

// HandleDeleteTaskFiles handles DELETE /api/files/{task_id}.
func (h *FileAPIHandler) HandleDeleteTaskFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := rejectLegacyIdentityQuery(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	if h.fileStorage == nil {
		http.Error(w, "File storage not available", http.StatusServiceUnavailable)
		return
	}

	taskID, err := h.extractTaskID(r.URL.Path, "/api/files/")
	if err != nil {
		http.Error(w, "Invalid URL format. Expected /api/files/{task_id}", http.StatusBadRequest)
		return
	}
	owner, err := h.taskOwnerForTaskID(context.Background(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if !canOperateTask(principal, owner) {
		http.Error(w, "Forbidden: cannot operate on another user's files", http.StatusForbidden)
		return
	}

	if err := h.fileStorage.DeleteTaskFiles(owner, taskID); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to delete files: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Deleted files for task %s", taskID),
		"task_id": taskID,
	})
}
