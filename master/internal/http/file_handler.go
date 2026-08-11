package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"master/internal/storage"
)

// FileAPIHandler handles HTTP REST API requests for file management
type FileAPIHandler struct {
	fileStorage *storage.FileStorageService
	quietMode   bool
}

// NewFileAPIHandler creates a new file API handler
func NewFileAPIHandler(fileStorage *storage.FileStorageService) *FileAPIHandler {
	return &FileAPIHandler{
		fileStorage: fileStorage,
		quietMode:   true,
	}
}

// FileListResponse represents the JSON response for file listing
type FileListResponse struct {
	UserID string         `json:"user_id"`
	Tasks  []TaskFileInfo `json:"tasks"`
	Count  int            `json:"count"`
}

// FileInfo represents individual file information
type FileInfoJSON struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// TaskFileInfo represents file information for a single task
type TaskFileInfo struct {
	TaskID    string         `json:"task_id"`
	TaskName  string         `json:"task_name"`
	Timestamp string         `json:"timestamp"`
	Files     []FileInfoJSON `json:"files"`
	TotalSize int64          `json:"total_size"`
}

// FileDetailResponse represents the JSON response for task file details
type FileDetailResponse struct {
	TaskID    string         `json:"task_id"`
	TaskName  string         `json:"task_name"`
	Timestamp string         `json:"timestamp"`
	Files     []FileInfoJSON `json:"files"`
	TotalSize int64          `json:"total_size"`
}

// HandleListFiles handles GET /api/files?user_id=<user>
// Lists all files for a user with access control
func (h *FileAPIHandler) HandleListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get requesting user from query parameters or auth header
	// For now, using query parameter. In production, use proper authentication.
	requestingUserID := r.URL.Query().Get("requesting_user")
	targetUserID := r.URL.Query().Get("user_id")

	if requestingUserID == "" {
		http.Error(w, "Missing requesting_user parameter", http.StatusBadRequest)
		return
	}

	if targetUserID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	if h.fileStorage == nil {
		http.Error(w, "File storage not available", http.StatusServiceUnavailable)
		return
	}

	// Use access-controlled method to list files
	fileMetadataList, err := h.fileStorage.ListUserFilesWithAccess(requestingUserID, targetUserID)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		log.Printf("Error listing files for user %s (requested by %s): %v", targetUserID, requestingUserID, err)
		http.Error(w, fmt.Sprintf("Failed to list files: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	tasks := make([]TaskFileInfo, 0, len(fileMetadataList))
	for _, metadata := range fileMetadataList {
		// Convert []storage.FileInfo to []FileInfoJSON
		files := make([]FileInfoJSON, 0, len(metadata.Files))
		for _, f := range metadata.Files {
			files = append(files, FileInfoJSON{
				Path: f.Path,
				Size: f.Size,
			})
		}

		tasks = append(tasks, TaskFileInfo{
			TaskID:    metadata.TaskID,
			TaskName:  metadata.TaskName,
			Timestamp: metadata.Timestamp.Format("2006-01-02 15:04:05"),
			Files:     files,
			TotalSize: metadata.TotalSize,
		})
	}

	response := FileListResponse{
		UserID: targetUserID,
		Tasks:  tasks,
		Count:  len(tasks),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	if !h.quietMode {
		log.Printf("✓ Listed %d task(s) for user %s (requested by %s)", len(tasks), targetUserID, requestingUserID)
	}
}

// HandleGetTaskFiles handles GET /api/files/{task_id}?user_id=<user>&requesting_user=<user>
// Gets file details for a specific task with access control
func (h *FileAPIHandler) HandleGetTaskFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from URL path
	// Expected format: /api/files/{task_id}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL format. Expected: /api/files/{task_id}", http.StatusBadRequest)
		return
	}
	taskID := parts[2]

	requestingUserID := r.URL.Query().Get("requesting_user")
	targetUserID := r.URL.Query().Get("user_id")

	if requestingUserID == "" {
		http.Error(w, "Missing requesting_user parameter", http.StatusBadRequest)
		return
	}

	if targetUserID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	if h.fileStorage == nil {
		http.Error(w, "File storage not available", http.StatusServiceUnavailable)
		return
	}

	// Use access-controlled method to get task files
	metadata, err := h.fileStorage.GetTaskFilesWithAccess(requestingUserID, targetUserID, taskID)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Printf("Error getting task files %s for user %s: %v", taskID, targetUserID, err)
		http.Error(w, fmt.Sprintf("Failed to get task files: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert []storage.FileInfo to []FileInfoJSON
	files := make([]FileInfoJSON, 0, len(metadata.Files))
	for _, f := range metadata.Files {
		files = append(files, FileInfoJSON{
			Path: f.Path,
			Size: f.Size,
		})
	}

	response := FileDetailResponse{
		TaskID:    metadata.TaskID,
		TaskName:  metadata.TaskName,
		Timestamp: metadata.Timestamp.Format("2006-01-02 15:04:05"),
		Files:     files,
		TotalSize: metadata.TotalSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	if !h.quietMode {
		log.Printf("✓ Retrieved task %s files for user %s (requested by %s)", taskID, targetUserID, requestingUserID)
	}
}

// HandleDownloadFile handles GET /api/files/{task_id}/download/{file_path}?user_id=<user>&requesting_user=<user>
// Downloads a specific file with access control
func (h *FileAPIHandler) HandleDownloadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID and file path from URL
	// Expected format: /api/files/{task_id}/download/{file_path}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[3] != "download" {
		http.Error(w, "Invalid URL format. Expected: /api/files/{task_id}/download/{file_path}", http.StatusBadRequest)
		return
	}

	taskID := parts[2]
	// Join remaining parts as file path (handles paths with slashes)
	filePath := strings.Join(parts[4:], "/")

	requestingUserID := r.URL.Query().Get("requesting_user")
	targetUserID := r.URL.Query().Get("user_id")

	if requestingUserID == "" {
		http.Error(w, "Missing requesting_user parameter", http.StatusBadRequest)
		return
	}

	if targetUserID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	if h.fileStorage == nil {
		http.Error(w, "File storage not available", http.StatusServiceUnavailable)
		return
	}

	// Use access-controlled method to read file
	fileData, err := h.fileStorage.ReadFileWithAccess(requestingUserID, targetUserID, taskID, filePath)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Printf("Error reading file %s for task %s: %v", filePath, taskID, err)
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	// Set headers for file download
	filename := filepath.Base(filePath)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileData)))

	// Write file data
	_, err = w.Write(fileData)
	if err != nil {
		log.Printf("Error writing file data: %v", err)
		return
	}

	if !h.quietMode {
		log.Printf("✓ Downloaded file %s for task %s (user: %s, requested by: %s)", filePath, taskID, targetUserID, requestingUserID)
	}
}

// HandleDeleteTaskFiles handles DELETE /api/files/{task_id}?user_id=<user>&requesting_user=<user>
// Deletes all files for a specific task with access control
func (h *FileAPIHandler) HandleDeleteTaskFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from URL path
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL format. Expected: /api/files/{task_id}", http.StatusBadRequest)
		return
	}
	taskID := parts[2]

	requestingUserID := r.URL.Query().Get("requesting_user")
	targetUserID := r.URL.Query().Get("user_id")

	if requestingUserID == "" {
		http.Error(w, "Missing requesting_user parameter", http.StatusBadRequest)
		return
	}

	if targetUserID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	if h.fileStorage == nil {
		http.Error(w, "File storage not available", http.StatusServiceUnavailable)
		return
	}

	// Use access-controlled method to delete files
	err := h.fileStorage.DeleteTaskFilesWithAccess(requestingUserID, targetUserID, taskID)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Printf("Error deleting files for task %s: %v", taskID, err)
		http.Error(w, fmt.Sprintf("Failed to delete files: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Deleted files for task %s", taskID),
		"task_id": taskID,
	})

	if !h.quietMode {
		log.Printf("✓ Deleted files for task %s (user: %s, requested by: %s)", taskID, targetUserID, requestingUserID)
	}
}
