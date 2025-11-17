// master/internal/storage/access_control_test.go
package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAccessControl(t *testing.T) {
	// Create temp directory for testing
	tmpDir, err := os.MkdirTemp("", "cloudai-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create file storage service
	fs, err := NewFileStorageService(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	// Create test files for different users
	testFiles := map[string]string{
		"alice/project1/task-1/data.txt":  "Alice's secret data",
		"bob/project2/task-2/results.csv": "Bob's results",
		"admin/logs/task-3/log.txt":       "Admin logs",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0700)
		os.WriteFile(fullPath, []byte(content), 0600)
	}

	ac := fs.GetAccessControl()

	t.Run("User can access own files", func(t *testing.T) {
		err := ac.CanAccessFiles("alice", "alice")
		if err != nil {
			t.Errorf("Alice should be able to access her own files: %v", err)
		}
	})

	t.Run("User cannot access other user's files", func(t *testing.T) {
		err := ac.CanAccessFiles("alice", "bob")
		if err == nil {
			t.Error("Alice should NOT be able to access Bob's files")
		}
	})

	t.Run("Admin can access any files", func(t *testing.T) {
		err := ac.CanAccessFiles("admin", "alice")
		if err != nil {
			t.Errorf("Admin should be able to access Alice's files: %v", err)
		}

		err = ac.CanAccessFiles("admin", "bob")
		if err != nil {
			t.Errorf("Admin should be able to access Bob's files: %v", err)
		}
	})

	t.Run("Path traversal is blocked", func(t *testing.T) {
		err := ac.ValidateFilePath("../../../etc/passwd")
		if err == nil {
			t.Error("Path traversal should be blocked")
		}

		err = ac.ValidateFilePath("../../sensitive")
		if err == nil {
			t.Error("Path traversal with .. should be blocked")
		}
	})

	t.Run("Absolute paths are blocked", func(t *testing.T) {
		err := ac.ValidateFilePath("/etc/passwd")
		if err == nil {
			t.Error("Absolute paths should be blocked")
		}
	})

	t.Run("Valid relative paths are allowed", func(t *testing.T) {
		err := ac.ValidateFilePath("subdir/file.txt")
		if err != nil {
			t.Errorf("Valid relative path should be allowed: %v", err)
		}

		err = ac.ValidateFilePath("file.txt")
		if err != nil {
			t.Errorf("Simple filename should be allowed: %v", err)
		}
	})
}

func TestAccessControlWithActualStorage(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cloudai-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create file storage
	fs, err := NewFileStorageService(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	// Create a test file for alice
	aliceDir := filepath.Join(tmpDir, "alice", "project", "2025-11-17_14-00-00", "task-123")
	os.MkdirAll(aliceDir, 0700)
	testFile := filepath.Join(aliceDir, "secret.txt")
	os.WriteFile(testFile, []byte("Alice's secret"), 0600)

	t.Run("Alice can read her own file", func(t *testing.T) {
		data, err := fs.ReadFileWithAccess("alice", "alice", "task-123", "secret.txt")
		if err != nil {
			t.Errorf("Alice should be able to read her own file: %v", err)
		}
		if string(data) != "Alice's secret" {
			t.Errorf("Expected 'Alice's secret', got '%s'", string(data))
		}
	})

	t.Run("Bob cannot read Alice's file", func(t *testing.T) {
		_, err := fs.ReadFileWithAccess("bob", "alice", "task-123", "secret.txt")
		if err == nil {
			t.Error("Bob should NOT be able to read Alice's file")
		}
	})

	t.Run("Admin can read Alice's file", func(t *testing.T) {
		data, err := fs.ReadFileWithAccess("admin", "alice", "task-123", "secret.txt")
		if err != nil {
			t.Errorf("Admin should be able to read Alice's file: %v", err)
		}
		if string(data) != "Alice's secret" {
			t.Errorf("Expected 'Alice's secret', got '%s'", string(data))
		}
	})
}
