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

package testworkflow

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeSuite(t *testing.T) {
	suite, err := normalizeSuite("  UI-Smoke ")
	if err != nil {
		t.Fatalf("normalizeSuite returned error: %v", err)
	}
	if suite != SuiteUISmoke {
		t.Fatalf("expected %q, got %q", SuiteUISmoke, suite)
	}
}

func TestNormalizeRunOptionsDefaults(t *testing.T) {
	repoRoot := createFixtureRepo(t)
	fixedNow := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)

	opts, err := normalizeRunOptions("smoke", RunOptions{RepoRoot: repoRoot}, fixedNow)
	if err != nil {
		t.Fatalf("normalizeRunOptions returned error: %v", err)
	}

	expectedOut := filepath.Join(repoRoot, "results", "testbench", "20250203-040506-smoke")
	if opts.outputDir != expectedOut {
		t.Fatalf("unexpected outputDir: got %q want %q", opts.outputDir, expectedOut)
	}
	if opts.composeFile != filepath.Join(repoRoot, "testbench", "docker-compose.host-master.yml") {
		t.Fatalf("unexpected compose file %q", opts.composeFile)
	}
	if opts.smokeWorkloadPath != filepath.Join(repoRoot, "testbench", "workloads", "heterogeneous-smoke.json") {
		t.Fatalf("unexpected smoke workload path %q", opts.smokeWorkloadPath)
	}
	if opts.masterURL != defaultMasterURL {
		t.Fatalf("unexpected master URL %q", opts.masterURL)
	}
	if opts.prometheusURL != defaultPrometheusURL {
		t.Fatalf("unexpected prometheus URL %q", opts.prometheusURL)
	}
	if opts.profile != defaultProfile {
		t.Fatalf("unexpected profile %q", opts.profile)
	}
	if opts.scheduler != defaultScheduler {
		t.Fatalf("unexpected scheduler %q", opts.scheduler)
	}
}

func TestNormalizeRunOptionsRelativeOutputDir(t *testing.T) {
	repoRoot := createFixtureRepo(t)
	fixedNow := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)

	opts, err := normalizeRunOptions("evidence", RunOptions{
		RepoRoot:  repoRoot,
		OutputDir: "custom/artifacts",
	}, fixedNow)
	if err != nil {
		t.Fatalf("normalizeRunOptions returned error: %v", err)
	}

	expected := filepath.Join(repoRoot, "custom", "artifacts")
	if opts.outputDir != expected {
		t.Fatalf("unexpected output dir: got %q want %q", opts.outputDir, expected)
	}
}

func TestNormalizeRunOptionsInvalidScheduler(t *testing.T) {
	repoRoot := createFixtureRepo(t)
	fixedNow := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)

	_, err := normalizeRunOptions("smoke", RunOptions{
		RepoRoot:  repoRoot,
		Scheduler: "PPO",
	}, fixedNow)
	if err == nil {
		t.Fatal("expected error for unsupported scheduler")
	}
}

func TestNormalizeCampaignWorkloads_PathInput(t *testing.T) {
	repoRoot := createFixtureRepo(t)
	workloadPath := filepath.Join("testbench", "workloads", "heterogeneous-smoke.json")

	workloads, err := normalizeCampaignWorkloads(repoRoot, workloadPath, "heterogeneous-smoke")
	if err != nil {
		t.Fatalf("normalizeCampaignWorkloads returned error: %v", err)
	}
	if workloads != "heterogeneous-smoke" {
		t.Fatalf("unexpected campaign workloads: got %q", workloads)
	}
}

func TestFindRepoRootFromMasterDir(t *testing.T) {
	repoRoot := createFixtureRepo(t)
	start := filepath.Join(repoRoot, "master")

	found, err := findRepoRoot(start)
	if err != nil {
		t.Fatalf("findRepoRoot returned error: %v", err)
	}
	if found != repoRoot {
		t.Fatalf("unexpected repo root: got %q want %q", found, repoRoot)
	}
}

func TestNormalizeCleanupOptionsDefaults(t *testing.T) {
	repoRoot := createFixtureRepo(t)
	opts, err := normalizeCleanupOptions(CleanupOptions{RepoRoot: repoRoot})
	if err != nil {
		t.Fatalf("normalizeCleanupOptions returned error: %v", err)
	}
	expectedCompose := filepath.Join(repoRoot, "testbench", "docker-compose.host-master.yml")
	if opts.composeFile != expectedCompose {
		t.Fatalf("unexpected compose file: got %q want %q", opts.composeFile, expectedCompose)
	}
}

func createFixtureRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "Makefile"), "all:\n\t@echo ok\n")
	mustWriteFile(t, filepath.Join(root, "master", "go.mod"), "module master\n")
	mustWriteFile(t, filepath.Join(root, "testbench", "docker-compose.yml"), "services: {}\n")
	mustWriteFile(t, filepath.Join(root, "testbench", "docker-compose.host-master.yml"), "services: {}\n")
	mustWriteFile(t, filepath.Join(root, "testbench", "scripts", "run_workload.py"), "#!/usr/bin/env python3\n")
	mustWriteFile(t, filepath.Join(root, "testbench", "workloads", "heterogeneous-smoke.json"), "{}\n")
	mustWriteFile(t, filepath.Join(root, "testbench", "workloads", "deterministic-full.json"), "{}\n")
	mustWriteFile(t, filepath.Join(root, "testbench", "workloads", "failure-stressed.json"), "{}\n")
	return root
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
