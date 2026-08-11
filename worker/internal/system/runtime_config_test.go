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

import "testing"

func TestResolveWorkerPortUsesEnvOverride(t *testing.T) {
	t.Setenv(workerPortEnv, "51000")

	port, err := ResolveWorkerPort(50052)
	if err != nil {
		t.Fatalf("ResolveWorkerPort returned error: %v", err)
	}
	if port != 51000 {
		t.Fatalf("expected override port 51000, got %d", port)
	}
}

func TestResolveWorkerPortRejectsInvalidOverride(t *testing.T) {
	t.Setenv(workerPortEnv, "not-a-port")

	if _, err := ResolveWorkerPort(50052); err == nil {
		t.Fatalf("expected ResolveWorkerPort to reject invalid override")
	}
}

func TestApplyResourceOverrides(t *testing.T) {
	t.Setenv(workerTotalCPUEnv, "2.5")
	t.Setenv(workerTotalMemoryGBEnv, "6")
	t.Setenv(workerTotalStorageGBEnv, "40")
	t.Setenv(workerTotalGPUCoresEnv, "1")

	resources := &ResourceInfo{
		TotalCPU:     1.0,
		TotalMemory:  2.0,
		TotalStorage: 10.0,
		TotalGPU:     0.0,
	}

	applyResourceOverrides(resources)

	if resources.TotalCPU != 2.5 {
		t.Fatalf("cpu override mismatch: got %.2f", resources.TotalCPU)
	}
	if resources.TotalMemory != 6 {
		t.Fatalf("memory override mismatch: got %.2f", resources.TotalMemory)
	}
	if resources.TotalStorage != 40 {
		t.Fatalf("storage override mismatch: got %.2f", resources.TotalStorage)
	}
	if resources.TotalGPU != 1 {
		t.Fatalf("gpu override mismatch: got %.2f", resources.TotalGPU)
	}
}
