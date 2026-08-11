// master/internal/storage/access_control.go
package storage

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// AccessControl enforces user-level file access policies
type AccessControl struct {
	storage *FileStorageService
}

func NewAccessControl(storage *FileStorageService) *AccessControl {
	return &AccessControl{storage: storage}
}

// CanAccessFiles checks if a requesting user can access target user's files
func (ac *AccessControl) CanAccessFiles(requestingUserID, targetUserID string) error {
	// Users can access their own files
	if requestingUserID == targetUserID {
		return nil
	}

	// Admin users can access any files
	if ac.isAdmin(requestingUserID) {
		return nil
	}

	// TODO: Check if files are shared with requesting user
	// if ac.isSharedFile(targetUserID, requestingUserID) {
	//     return nil
	// }

	return fmt.Errorf("access denied: user %s cannot access files of user %s",
		requestingUserID, targetUserID)
}

// ValidateFilePath checks if a file path is safe (prevents directory traversal)
func (ac *AccessControl) ValidateFilePath(filePath string) error {
	// Reject empty paths
	if filePath == "" {
		return fmt.Errorf("invalid file path: empty path")
	}

	// Reject absolute paths
	if filepath.IsAbs(filePath) {
		return fmt.Errorf("invalid file path: must be relative")
	}

	// Clean and validate - reject paths with traversal components
	cleaned := filepath.Clean(filePath)

	// After cleaning, reject if path escapes the base (starts with "..")
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid file path: path traversal detected")
	}

	// Reject null bytes which can truncate paths in C-based filesystem calls
	if strings.ContainsRune(filePath, 0) {
		return fmt.Errorf("invalid file path: contains null byte")
	}

	return nil
}

// CanAccessFile checks if a user can access a specific file (legacy method)
func (ac *AccessControl) CanAccessFile(requestingUserID, targetUserID, filePath string) error {
	// First check user-level access
	if err := ac.CanAccessFiles(requestingUserID, targetUserID); err != nil {
		return err
	}

	// Then validate file path
	return ac.ValidateFilePath(filePath)
}

// GetUserFiles returns files accessible by a user with access control
func (ac *AccessControl) GetUserFiles(requestingUserID, targetUserID string) ([]*FileMetadata, error) {
	// Check access permission
	if err := ac.CanAccessFiles(requestingUserID, targetUserID); err != nil {
		return nil, err
	}

	// Get files from storage (using the storage reference)
	userDir := filepath.Join(ac.storage.baseDir, targetUserID)
	var files []*FileMetadata

	err := filepath.Walk(userDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Create metadata for each file
		relPath, _ := filepath.Rel(userDir, path)
		metadata := &FileMetadata{
			UserID:      targetUserID,
			FilePaths:   []string{relPath},
			StoragePath: filepath.Dir(path),
		}
		files = append(files, metadata)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// ReadFile reads a file with access control
func (ac *AccessControl) ReadFile(requestingUserID, targetUserID, filePath string) ([]byte, error) {
	// Check access permission
	if err := ac.CanAccessFile(requestingUserID, targetUserID, filePath); err != nil {
		return nil, err
	}

	// Construct and validate the resolved path stays within the base directory
	fullPath := filepath.Join(ac.storage.baseDir, targetUserID, filePath)
	resolvedPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}
	baseResolved, err := filepath.Abs(filepath.Join(ac.storage.baseDir, targetUserID))
	if err != nil {
		return nil, fmt.Errorf("invalid base path: %w", err)
	}
	if !strings.HasPrefix(resolvedPath, baseResolved+string(filepath.Separator)) && resolvedPath != baseResolved {
		return nil, fmt.Errorf("access denied: path escapes user directory")
	}

	return os.ReadFile(resolvedPath)
}

// isAdmin checks if a user has admin privileges
func (ac *AccessControl) isAdmin(userID string) bool {
	// TODO: Query user roles from database
	// For now, only "admin" user is admin
	return userID == "admin"
}

// isSharedFile checks if a file is shared with the requesting user
func (ac *AccessControl) isSharedFile(targetUserID string, requestingUserID string) bool {
	// TODO: Query file sharing permissions from database
	return false
}

// AuditFileAccess logs file access attempts for security auditing
func (ac *AccessControl) AuditFileAccess(userID, action, resource string, success bool) {
	// TODO: Store in audit log database
	status := "✅ SUCCESS"
	if !success {
		status = "❌ DENIED"
	}
	log.Printf("[AUDIT] %s | User=%s | Action=%s | Resource=%s",
		status, userID, action, resource)
}
