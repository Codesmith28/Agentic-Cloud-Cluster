package storage

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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

// FileMetadata represents metadata for stored files
type FileMetadata struct {
	UserID      string
	TaskID      string
	TaskName    string
	Timestamp   time.Time
	FilePaths   []string // Relative paths from task directory
	StoragePath string   // Absolute path to task directory
}

// NewFileStorageService creates a new file storage service
func NewFileStorageService(baseDir string) (*FileStorageService, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
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

// ReceiveFileStream handles streaming file uploads from workers
func (s *FileStorageService) ReceiveFileStream(stream pb.MasterWorker_UploadTaskFilesServer) (*FileMetadata, error) {
	var metadata FileMetadata
	var currentFile *os.File
	var currentFilePath string
	filesReceived := 0

	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			// Close last file if open
			if currentFile != nil {
				currentFile.Close()
			}
			break
		}
		if err != nil {
			// Clean up on error
			if currentFile != nil {
				currentFile.Close()
			}
			return nil, fmt.Errorf("error receiving file chunk: %w", err)
		}

		// First chunk initializes metadata
		if metadata.TaskID == "" {
			metadata.UserID = chunk.UserId
			metadata.TaskID = chunk.TaskId
			metadata.TaskName = chunk.TaskName
			metadata.Timestamp = time.Unix(chunk.Timestamp, 0)
			metadata.StoragePath = s.GetTaskStoragePath(chunk.UserId, chunk.TaskName, chunk.Timestamp, chunk.TaskId)
			metadata.FilePaths = []string{}

			// Create storage directory with secure permissions (drwx------)
			if err := os.MkdirAll(metadata.StoragePath, 0700); err != nil {
				return nil, fmt.Errorf("failed to create storage directory: %w", err)
			}

			log.Printf("[FileStorage] 🔒 Receiving files for task %s (user: %s, secure storage)",
				chunk.TaskId, chunk.UserId)
		}

		// New file in the stream
		if currentFilePath != chunk.FilePath {
			// Close previous file
			if currentFile != nil {
				currentFile.Close()
				filesReceived++
			}

			// Open new file
			currentFilePath = chunk.FilePath
			fullPath := filepath.Join(metadata.StoragePath, chunk.FilePath)

			// Create parent directories with secure permissions
			if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
				return nil, fmt.Errorf("failed to create directory for %s: %w", chunk.FilePath, err)
			}

			// Create file with secure permissions (rw-------)
			currentFile, err = os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s: %w", fullPath, err)
			}

			metadata.FilePaths = append(metadata.FilePaths, chunk.FilePath)
			log.Printf("[FileStorage] 📄 Receiving file: %s (secure)", chunk.FilePath)
		}

		// Write chunk data
		if _, err := currentFile.Write(chunk.Data); err != nil {
			currentFile.Close()
			return nil, fmt.Errorf("failed to write to file %s: %w", currentFilePath, err)
		}

		// Close file if this is the last chunk
		if chunk.IsLastChunk {
			currentFile.Close()
			filesReceived++
			log.Printf("[FileStorage] ✓ File complete: %s", chunk.FilePath)
			currentFile = nil
			currentFilePath = ""
		}

		// All files received
		if chunk.IsLastFile {
			log.Printf("[FileStorage] ✓ All files received (%d files) for task %s", filesReceived, chunk.TaskId)
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
		parts := filepath.SplitList(rel)

		// Check if this is a task directory (4 levels deep)
		if len(parts) == 4 && info.IsDir() {
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
			taskDir := path
			filepath.Walk(taskDir, func(filePath string, fileInfo os.FileInfo, err error) error {
				if err != nil || fileInfo.IsDir() {
					return nil
				}
				relPath, _ := filepath.Rel(taskDir, filePath)
				filePaths = append(filePaths, relPath)
				return nil
			})

			metadataList = append(metadataList, FileMetadata{
				UserID:      userID,
				TaskID:      taskID,
				TaskName:    taskName,
				Timestamp:   timestamp,
				FilePaths:   filePaths,
				StoragePath: taskDir,
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
