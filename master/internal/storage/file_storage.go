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
	baseDir string // Base directory for all file storage (e.g., /var/cloudai/files)
	mu      sync.RWMutex
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

	return &FileStorageService{
		baseDir: baseDir,
	}, nil
}

// GetTaskStoragePath returns the directory path for a specific task
// Path format: <baseDir>/<user_id>/<task_name>/<timestamp>/<task_id>/
func (s *FileStorageService) GetTaskStoragePath(userID, taskName string, timestamp int64, taskID string) string {
	timestampStr := time.Unix(timestamp, 0).Format("2006-01-02_15-04-05")
	return filepath.Join(s.baseDir, userID, taskName, timestampStr, taskID)
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

			// Create storage directory
			if err := os.MkdirAll(metadata.StoragePath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create storage directory: %w", err)
			}

			log.Printf("[FileStorage] Receiving files for task %s (user: %s, task_name: %s)",
				chunk.TaskId, chunk.UserId, chunk.TaskName)
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

			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory for %s: %w", chunk.FilePath, err)
			}

			currentFile, err = os.Create(fullPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s: %w", fullPath, err)
			}

			metadata.FilePaths = append(metadata.FilePaths, chunk.FilePath)
			log.Printf("[FileStorage] Receiving file: %s", chunk.FilePath)
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
