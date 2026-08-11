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

package benchmark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunSuiteAndWriteArtifacts(t *testing.T) {
	suite, err := RunSuite(ProfileSteady, 42)
	if err != nil {
		t.Fatalf("RunSuite failed: %v", err)
	}
	if suite == nil {
		t.Fatal("suite is nil")
	}
	if len(suite.Profiles) != 1 {
		t.Fatalf("expected 1 profile result, got %d", len(suite.Profiles))
	}

	outputDir, err := WriteArtifacts(suite, t.TempDir())
	if err != nil {
		t.Fatalf("WriteArtifacts failed: %v", err)
	}

	requiredFiles := []string{
		"summary.json",
		"metrics.csv",
		"task_runs.csv",
		"report.html",
		"README.md",
	}
	for _, file := range requiredFiles {
		path := filepath.Join(outputDir, file)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}
}

func TestAvailableProfilesIncludesCoreProfiles(t *testing.T) {
	profiles := AvailableProfiles()
	if len(profiles) == 0 {
		t.Fatal("expected non-empty profile list")
	}

	required := map[string]bool{
		ProfileShowcase: false,
		ProfileSteady:   false,
		ProfileGPUSpike: false,
	}
	for _, profile := range profiles {
		if _, ok := required[profile]; ok {
			required[profile] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Fatalf("expected profile %s in AvailableProfiles", name)
		}
	}
}
