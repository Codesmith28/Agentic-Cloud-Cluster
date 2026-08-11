// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	if override := strings.TrimSpace(os.Getenv(workerStateDirEnvVar)); override != "" {
		return override, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".cloudai", "worker"), nil
}

func generateWorkerID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "worker-" + hex.EncodeToString(buf), nil
}
