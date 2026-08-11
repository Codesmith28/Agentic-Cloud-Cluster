package storage

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pb "master/proto"
)

// FileStorageService handles file uploads and storage organization
type FileStorageService struct {
	baseDir       string // Base directory for all file storage (e.g., /var/cloudai/files)
	accessControl *AccessControl
	mu            sync.RWMutex
}

// FileInfo represents individual file information
type FileInfo struct {
	Path string // Relative path from task directory
	Size int64  // File size in bytes
}

// FileMetadata represents metadata for stored files
type FileMetadata struct {
	UserID      string
	TaskID      string
	TaskName    string
	Timestamp   time.Time
	FilePaths   []string   // Relative paths from task directory (deprecated, use Files)
	Files       []FileInfo // Detailed file information with sizes
	StoragePath string     // Absolute path to task directory
	TotalSize   int64      // Total size of all files in bytes
}

// NewFileStorageService creates a new file storage service
func NewFileStorageService(baseDir string) (*FileStorageService, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	fs := &FileStorageService{
		baseDir: baseDir,
	}

	// Initialize access control
	fs.accessControl = NewAccessControl(fs)

	log.Printf("✓ FileStorageService initialized with access control")

	return fs, nil
}

// GetAccessControl returns the access control instance
func (s *FileStorageService) GetAccessControl() *AccessControl {
	return s.accessControl
}

// GetTaskStoragePath returns the directory path for a specific task
// Path format: <baseDir>/<user_id>/<task_name>/<timestamp>/<task_id>/
// Creates directories with secure permissions (0700 - owner only)
func (s *FileStorageService) GetTaskStoragePath(userID, taskName string, timestamp int64, taskID string) string {
	timestampStr := time.Unix(timestamp, 0).Format("2006-01-02_15-04-05")

	// Create user directory with strict permissions (drwx------)
	userDir := filepath.Join(s.baseDir, userID)
	os.MkdirAll(userDir, 0700) // Only owner can read/write/execute

	// Create full path
	return filepath.Join(userDir, taskName, timestampStr, taskID)
}

func validateUploadFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("invalid file path: empty")
	}
	if filepath.IsAbs(filePath) {
		return fmt.Errorf("invalid file path: absolute path not allowed")
	}
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("invalid file path: path traversal detected")
	}
	cleaned := filepath.Clean(filePath)
	if cleaned == "." || cleaned == "" || cleaned != filePath {
		return fmt.Errorf("invalid file path: must be a clean relative path")
	}
	return nil
}

func ensurePathInsideBase(basePath, targetPath string) error {
	baseClean := filepath.Clean(basePath)
	targetClean := filepath.Clean(targetPath)

	if targetClean == baseClean {
		return nil
	}

	prefix := baseClean + string(filepath.Separator)
	if !strings.HasPrefix(targetClean, prefix) {
		return fmt.Errorf("invalid file path: escaped task storage directory")
	}
	return nil
}

// ReceiveFileStreamTrusted handles streaming file uploads from workers using trusted task metadata.
// Ownership and task metadata are always derived from master-trusted values, not stream payload fields.
func (s *FileStorageService) ReceiveFileStreamTrusted(
	stream pb.MasterWorker_UploadTaskFilesServer,
	firstChunk *pb.FileChunk,
	trustedUserID, trustedTaskName string,
	trustedTimestamp int64,
) (*FileMetadata, error) {
	if firstChunk == nil {
		return nil, fmt.Errorf("missing first file chunk")
	}
	if firstChunk.TaskId == "" {
		return nil, fmt.Errorf("missing task_id in upload stream")
	}
	if trustedUserID == "" {
		return nil, fmt.Errorf("trusted user ID is required")
	}
	if trustedTaskName == "" {
		trustedTaskName = firstChunk.TaskId
	}
	if trustedTimestamp <= 0 {
		trustedTimestamp = time.Now().Unix()
	}

	metadata := FileMetadata{
		UserID:      trustedUserID,
		TaskID:      firstChunk.TaskId,
		TaskName:    trustedTaskName,
		Timestamp:   time.Unix(trustedTimestamp, 0),
		FilePaths:   []string{},
		StoragePath: s.GetTaskStoragePath(trustedUserID, trustedTaskName, trustedTimestamp, firstChunk.TaskId),
	}

	var currentFile *os.File
	var currentFilePath string
	filesReceived := 0

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create storage directory with secure permissions (drwx------).
	if err := os.MkdirAll(metadata.StoragePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	log.Printf("[FileStorage] 🔒 Receiving files for task %s (trusted user: %s, secure storage)", metadata.TaskID, metadata.UserID)

	processChunk := func(chunk *pb.FileChunk) (bool, error) {
		if chunk.TaskId != metadata.TaskID {
			return false, fmt.Errorf("invalid upload stream: multiple task IDs detected")
		}
		if err := validateUploadFilePath(chunk.FilePath); err != nil {
			return false, err
		}

		if currentFilePath != chunk.FilePath {
			if currentFile != nil {
				_ = currentFile.Close()
				filesReceived++
			}

			currentFilePath = chunk.FilePath
			fullPath := filepath.Join(metadata.StoragePath, chunk.FilePath)
			if err := ensurePathInsideBase(metadata.StoragePath, fullPath); err != nil {
				return false, err
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
				return false, fmt.Errorf("failed to create directory for %s: %w", chunk.FilePath, err)
			}

			var err error
			currentFile, err = os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return false, fmt.Errorf("failed to create file %s: %w", fullPath, err)
			}

			metadata.FilePaths = append(metadata.FilePaths, chunk.FilePath)
			log.Printf("[FileStorage] 📄 Receiving file: %s (secure)", chunk.FilePath)
		}

		if _, err := currentFile.Write(chunk.Data); err != nil {
			_ = currentFile.Close()
			return false, fmt.Errorf("failed to write to file %s: %w", currentFilePath, err)
		}

		if chunk.IsLastChunk {
			_ = currentFile.Close()
			filesReceived++
			log.Printf("[FileStorage] ✓ File complete: %s", chunk.FilePath)
			currentFile = nil
			currentFilePath = ""
		}

		if chunk.IsLastFile {
			log.Printf("[FileStorage] ✓ All files received (%d files) for task %s", filesReceived, chunk.TaskId)
			return true, nil
		}
		return false, nil
	}

	done, err := processChunk(firstChunk)
	if err != nil {
		if currentFile != nil {
			_ = currentFile.Close()
		}
		return nil, err
	}
	if done {
		return &metadata, nil
	}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if currentFile != nil {
				_ = currentFile.Close()
			}
			return nil, fmt.Errorf("error receiving file chunk: %w", err)
		}

		done, err := processChunk(chunk)
		if err != nil {
			if currentFile != nil {
				_ = currentFile.Close()
			}
			return nil, err
		}
		if done {
			break
		}
	}

	return &metadata, nil
}

// ListUserFiles returns all files for a specific user
func (s *FileStorageService) ListUserFiles(userID string) ([]FileMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userDir := filepath.Join(s.baseDir, userID)
	var metadataList []FileMetadata

	// Check if user directory exists
	if _, err := os.Stat(userDir); os.IsNotExist(err) {
		return metadataList, nil // Return empty list if no files
	}

	// Walk through user's directory structure
	err := filepath.Walk(userDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the user directory itself
		if path == userDir {
			return nil
		}

		// We're looking for task directories (deepest level)
		// Structure: <userDir>/<taskName>/<timestamp>/<taskID>/
		rel, _ := filepath.Rel(userDir, path)
		parts := strings.Split(rel, string(filepath.Separator))

		// Check if this is a task directory (3 levels deep relative to userDir)
		if len(parts) == 3 && info.IsDir() {
			taskName := parts[0]
			timestampStr := parts[1]
			taskID := parts[2]

			// Parse timestamp
			timestamp, err := time.Parse("2006-01-02_15-04-05", timestampStr)
			if err != nil {
				log.Printf("Warning: failed to parse timestamp %s: %v", timestampStr, err)
				return nil
			}

			// List files in this task directory
			var filePaths []string
			var files []FileInfo
			var totalSize int64
			taskDir := path
			filepath.Walk(taskDir, func(filePath string, fileInfo os.FileInfo, err error) error {
				if err != nil || fileInfo.IsDir() {
					return nil
				}
				relPath, _ := filepath.Rel(taskDir, filePath)
				filePaths = append(filePaths, relPath)
				files = append(files, FileInfo{
					Path: relPath,
					Size: fileInfo.Size(),
				})
				totalSize += fileInfo.Size()
				return nil
			})

			metadataList = append(metadataList, FileMetadata{
				UserID:      userID,
				TaskID:      taskID,
				TaskName:    taskName,
				Timestamp:   timestamp,
				FilePaths:   filePaths,
				Files:       files,
				StoragePath: taskDir,
				TotalSize:   totalSize,
			})
		}

		return nil
	})

	return metadataList, err
}

// GetTaskFiles returns files for a specific task
func (s *FileStorageService) GetTaskFiles(userID, taskID string) (*FileMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Search for task in user's directory
	userFiles, err := s.ListUserFiles(userID)
	if err != nil {
		return nil, err
	}

	for _, metadata := range userFiles {
		if metadata.TaskID == taskID {
			return &metadata, nil
		}
	}

	return nil, fmt.Errorf("task %s not found for user %s", taskID, userID)
}

// DeleteTaskFiles removes all files for a specific task
func (s *FileStorageService) DeleteTaskFiles(userID, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, err := s.GetTaskFiles(userID, taskID)
	if err != nil {
		return err
	}

	return os.RemoveAll(metadata.StoragePath)
}

// GetFilePath returns the absolute path to a specific file
func (s *FileStorageService) GetFilePath(userID, taskID, relativeFilePath string) (string, error) {
	metadata, err := s.GetTaskFiles(userID, taskID)
	if err != nil {
		return "", err
	}

	// Check if file exists in metadata
	found := false
	for _, filePath := range metadata.FilePaths {
		if filePath == relativeFilePath {
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("file %s not found in task %s", relativeFilePath, taskID)
	}

	return filepath.Join(metadata.StoragePath, relativeFilePath), nil
}

// Close cleans up resources (currently no-op, but useful for future extensions)
func (s *FileStorageService) Close() error {
	return nil
}

// ListUserFilesWithAccess lists files for a user with access control
// requestingUserID: the user making the request
// targetUserID: the user whose files to list
func (s *FileStorageService) ListUserFilesWithAccess(requestingUserID, targetUserID string) ([]*FileMetadata, error) {
	// Check access permission
	if err := s.accessControl.CanAccessFiles(requestingUserID, targetUserID); err != nil {
		s.accessControl.AuditFileAccess(requestingUserID, "list_files", targetUserID, false)
		return nil, err
	}

	// Access granted - get files
	files, err := s.ListUserFiles(targetUserID)
	if err != nil {
		s.accessControl.AuditFileAccess(requestingUserID, "list_files", targetUserID, false)
		return nil, err
	}

	s.accessControl.AuditFileAccess(requestingUserID, "list_files", targetUserID, true)

	log.Printf("🔐 [Access Control] User %s accessed file list for user %s (%d files)",
		requestingUserID, targetUserID, len(files))

	// Convert []FileMetadata to []*FileMetadata
	result := make([]*FileMetadata, len(files))
	for i := range files {
		result[i] = &files[i]
	}

	return result, nil
}

// GetTaskFilesWithAccess gets task files with access control
func (s *FileStorageService) GetTaskFilesWithAccess(requestingUserID, targetUserID, taskID string) (*FileMetadata, error) {
	// Check access permission
	if err := s.accessControl.CanAccessFiles(requestingUserID, targetUserID); err != nil {
		s.accessControl.AuditFileAccess(requestingUserID, "get_task", taskID, false)
		return nil, err
	}

	// Access granted - get task files
	metadata, err := s.GetTaskFiles(targetUserID, taskID)
	s.accessControl.AuditFileAccess(requestingUserID, "get_task", taskID, err == nil)

	if err == nil {
		log.Printf("🔐 [Access Control] User %s accessed task %s files (user: %s)",
			requestingUserID, taskID, targetUserID)
	}

	return metadata, err
}

// ReadFileWithAccess reads a file with access control
func (s *FileStorageService) ReadFileWithAccess(requestingUserID, targetUserID, taskID, filePath string) ([]byte, error) {
	// Check access permission
	if err := s.accessControl.CanAccessFiles(requestingUserID, targetUserID); err != nil {
		s.accessControl.AuditFileAccess(requestingUserID, "read_file", filePath, false)
		return nil, err
	}

	// Sanitize file path to prevent directory traversal
	if err := s.accessControl.ValidateFilePath(filePath); err != nil {
		s.accessControl.AuditFileAccess(requestingUserID, "read_file", filePath, false)
		return nil, err
	}

	// Get full file path
	fullPath, err := s.GetFilePath(targetUserID, taskID, filePath)
	if err != nil {
		s.accessControl.AuditFileAccess(requestingUserID, "read_file", filePath, false)
		return nil, err
	}

	// Read file
	data, err := os.ReadFile(fullPath)
	s.accessControl.AuditFileAccess(requestingUserID, "read_file", filePath, err == nil)

	if err == nil {
		log.Printf("🔐 [Access Control] User %s read file %s (task: %s, user: %s, size: %d bytes)",
			requestingUserID, filePath, taskID, targetUserID, len(data))
	}

	return data, err
}

// DeleteTaskFilesWithAccess deletes task files with access control
func (s *FileStorageService) DeleteTaskFilesWithAccess(requestingUserID, targetUserID, taskID string) error {
	// Check access permission (only owner or admin can delete)
	if requestingUserID != targetUserID && !s.accessControl.isAdmin(requestingUserID) {
		s.accessControl.AuditFileAccess(requestingUserID, "delete_task", taskID, false)
		return fmt.Errorf("access denied: only file owner or admin can delete files")
	}

	// Delete files
	err := s.DeleteTaskFiles(targetUserID, taskID)
	s.accessControl.AuditFileAccess(requestingUserID, "delete_task", taskID, err == nil)

	if err == nil {
		log.Printf("🔐 [Access Control] User %s deleted task %s files (user: %s)",
			requestingUserID, taskID, targetUserID)
	}

	return err
}
