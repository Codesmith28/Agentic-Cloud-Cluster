// master/internal/storage/file_storage_secure.go
package storage

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// SetFileOwnership changes ownership of files to match the CloudAI user
// This requires the master process to run as root or with CAP_CHOWN capability
func (s *FileStorageService) SetFileOwnership(path string, cloudaiUserID string) error {
	// Map CloudAI user_id to OS user
	osUsername := fmt.Sprintf("cloudai-%s", cloudaiUserID)

	// Look up OS user
	osUser, err := user.Lookup(osUsername)
	if err != nil {
		// OS user doesn't exist - skip ownership change
		// Files will remain owned by process user
		return fmt.Errorf("OS user %s not found: %w", osUsername, err)
	}

	uid, _ := strconv.Atoi(osUser.Uid)
	gid, _ := strconv.Atoi(osUser.Gid)

	// Change ownership recursively
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(filePath, uid, gid)
	})
}

// CreateUserDirectory creates a user directory with proper ownership
func (s *FileStorageService) CreateUserDirectory(userID string) error {
	userDir := filepath.Join(s.baseDir, userID)

	// Create directory with secure permissions
	if err := os.MkdirAll(userDir, 0700); err != nil {
		return fmt.Errorf("failed to create user directory: %w", err)
	}

	// Try to set ownership (requires root or CAP_CHOWN)
	if err := s.SetFileOwnership(userDir, userID); err != nil {
		// Log warning but don't fail - we'll rely on application-level security
		fmt.Printf("Warning: Could not set file ownership for user %s: %v\n", userID, err)
	}

	return nil
}
