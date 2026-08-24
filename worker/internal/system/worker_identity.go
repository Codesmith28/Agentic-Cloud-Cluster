package system

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	workerIDEnvVar       = "WORKER_ID"
	workerStateDirEnvVar = "CLOUDAI_WORKER_STATE_DIR"
	workerIDFileName     = "worker_id"
)

// ResolveWorkerID returns a stable worker identity that survives restarts and endpoint changes.
//
// Resolution order:
// 1. WORKER_ID env override
// 2. Persisted worker ID from state file
// 3. Current hostname (persisted for future runs)
// 4. Randomly generated fallback ID (persisted)
func ResolveWorkerID(hostname string) (string, error) {
	if override := strings.TrimSpace(os.Getenv(workerIDEnvVar)); override != "" {
		return override, nil
	}

	stateDir, err := resolveWorkerStateDir()
	if err != nil {
		return "", fmt.Errorf("resolve state directory: %w", err)
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return "", fmt.Errorf("create state directory %s: %w", stateDir, err)
	}

	workerIDPath := filepath.Join(stateDir, workerIDFileName)
	if data, err := os.ReadFile(workerIDPath); err == nil {
		existing := strings.TrimSpace(string(data))
		if existing != "" {
			return existing, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read worker id file: %w", err)
	}

	candidate := strings.TrimSpace(hostname)
	if candidate == "" {
		candidate, err = generateWorkerID()
		if err != nil {
			return "", fmt.Errorf("generate worker id: %w", err)
		}
	}

	if err := os.WriteFile(workerIDPath, []byte(candidate+"\n"), 0600); err != nil {
		return "", fmt.Errorf("persist worker id to %s: %w", workerIDPath, err)
	}

	return candidate, nil
}

func resolveWorkerStateDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("AGENTIC_WORKER_STATE_DIR")); override != "" {
		return override, nil
	}
	if override := strings.TrimSpace(os.Getenv(workerStateDirEnvVar)); override != "" {
		return override, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Check if legacy dir exists
	legacyDir := filepath.Join(homeDir, ".cloudai", "worker")
	if _, err := os.Stat(legacyDir); err == nil {
		return legacyDir, nil
	}

	return filepath.Join(homeDir, ".agentic-cloud-cluster", "worker"), nil
}

func generateWorkerID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "worker-" + hex.EncodeToString(buf), nil
}
