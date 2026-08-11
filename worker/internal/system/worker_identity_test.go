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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkerIDPersistsInitialIdentity(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(workerStateDirEnvVar, stateDir)
	t.Setenv(workerIDEnvVar, "")

	first, err := ResolveWorkerID("worker-host-a")
	if err != nil {
		t.Fatalf("ResolveWorkerID returned error: %v", err)
	}
	if first != "worker-host-a" {
		t.Fatalf("expected first worker ID to use hostname, got %q", first)
	}

	second, err := ResolveWorkerID("worker-host-b")
	if err != nil {
		t.Fatalf("ResolveWorkerID returned error on second call: %v", err)
	}
	if second != first {
		t.Fatalf("expected persisted worker ID %q, got %q", first, second)
	}

	idPath := filepath.Join(stateDir, workerIDFileName)
	data, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatalf("failed to read persisted worker ID: %v", err)
	}
	if strings.TrimSpace(string(data)) != first {
		t.Fatalf("persisted worker ID mismatch: want %q got %q", first, strings.TrimSpace(string(data)))
	}
}

func TestResolveWorkerIDUsesEnvOverride(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(workerStateDirEnvVar, stateDir)
	t.Setenv(workerIDEnvVar, "custom-worker-id")

	id, err := ResolveWorkerID("worker-host")
	if err != nil {
		t.Fatalf("ResolveWorkerID returned error: %v", err)
	}
	if id != "custom-worker-id" {
		t.Fatalf("expected env override worker ID, got %q", id)
	}
}
