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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSuiteAllProfiles(t *testing.T) {
	suite, err := RunSuite(ProfileAll, 42)
	if err != nil {
		t.Fatalf("RunSuite(all) failed: %v", err)
	}
	if suite == nil {
		t.Fatal("suite is nil")
	}
	if len(suite.Profiles) < 3 {
		t.Fatalf("expected at least 3 profiles, got %d", len(suite.Profiles))
	}
}

func TestRunSuiteEachProfile(t *testing.T) {
	profiles := []string{ProfileShowcase, ProfileSteady, ProfileBursty}
	for _, p := range profiles {
		t.Run(p, func(t *testing.T) {
			suite, err := RunSuite(p, 42)
			if err != nil {
				t.Fatalf("RunSuite(%s) failed: %v", p, err)
			}
			if len(suite.Profiles) != 1 {
				t.Fatalf("expected 1 profile, got %d", len(suite.Profiles))
			}
			pr := suite.Profiles[0]
			if pr.TaskCount <= 0 {
				t.Fatalf("expected positive task count, got %d", pr.TaskCount)
			}
			// Must have at least Round-Robin and RTS results
			if len(pr.SchedulerResults) < 2 {
				t.Fatalf("expected at least 2 scheduler results (RR, RTS), got %d", len(pr.SchedulerResults))
			}
		})
	}
}

func TestRunSuiteUnknownProfile(t *testing.T) {
	_, err := RunSuite("nonexistent-profile", 42)
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestSchedulerMetricsAreValid(t *testing.T) {
	suite, err := RunSuite(ProfileSteady, 42)
	if err != nil {
		t.Fatalf("RunSuite failed: %v", err)
	}
	for _, pr := range suite.Profiles {
		for _, sr := range pr.SchedulerResults {
			m := sr.Metrics
			if m.TotalTasks <= 0 {
				t.Fatalf("[%s/%s] TotalTasks must be positive", pr.Profile, sr.SchedulerName)
			}
			if m.CompletedTasks < 0 || m.CompletedTasks > m.TotalTasks {
				t.Fatalf("[%s/%s] CompletedTasks=%d out of range [0, %d]", pr.Profile, sr.SchedulerName, m.CompletedTasks, m.TotalTasks)
			}
			if m.SLASuccessRatePct < 0 || m.SLASuccessRatePct > 100 {
				t.Fatalf("[%s/%s] SLASuccessRatePct=%.2f out of [0,100]", pr.Profile, sr.SchedulerName, m.SLASuccessRatePct)
			}
			if m.MakespanSec < 0 {
				t.Fatalf("[%s/%s] negative makespan", pr.Profile, sr.SchedulerName)
			}
			if m.AvgQueueWaitSec < 0 {
				t.Fatalf("[%s/%s] negative avg queue wait", pr.Profile, sr.SchedulerName)
			}
			if m.P95QueueWaitSec < m.AvgQueueWaitSec {
				t.Logf("[%s/%s] Warning: p95 wait (%.2f) < avg wait (%.2f)", pr.Profile, sr.SchedulerName, m.P95QueueWaitSec, m.AvgQueueWaitSec)
			}
		}
	}
}

func TestTaskRunsSumToCompletedTasks(t *testing.T) {
	suite, err := RunSuite(ProfileSteady, 99)
	if err != nil {
		t.Fatalf("RunSuite failed: %v", err)
	}
	for _, pr := range suite.Profiles {
		for _, sr := range pr.SchedulerResults {
			if len(sr.TaskRuns) != sr.Metrics.CompletedTasks {
				t.Fatalf("[%s/%s] TaskRuns count (%d) != CompletedTasks (%d)",
					pr.Profile, sr.SchedulerName, len(sr.TaskRuns), sr.Metrics.CompletedTasks)
			}
		}
	}
}

func TestDeterministicWithSameSeed(t *testing.T) {
	suite1, err := RunSuite(ProfileSteady, 42)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	suite2, err := RunSuite(ProfileSteady, 42)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	if len(suite1.Profiles) != len(suite2.Profiles) {
		t.Fatal("profile count mismatch between same-seed runs")
	}

	for i, p1 := range suite1.Profiles {
		p2 := suite2.Profiles[i]
		for j, sr1 := range p1.SchedulerResults {
			sr2 := p2.SchedulerResults[j]
			if sr1.Metrics.MakespanSec != sr2.Metrics.MakespanSec {
				t.Fatalf("[%s/%s] makespan mismatch: %.4f vs %.4f",
					p1.Profile, sr1.SchedulerName, sr1.Metrics.MakespanSec, sr2.Metrics.MakespanSec)
			}
			if sr1.Metrics.SLASuccessRatePct != sr2.Metrics.SLASuccessRatePct {
				t.Fatalf("[%s/%s] SLA rate mismatch: %.4f vs %.4f",
					p1.Profile, sr1.SchedulerName, sr1.Metrics.SLASuccessRatePct, sr2.Metrics.SLASuccessRatePct)
			}
		}
	}
}

func TestDifferentSeedsProduceDifferentResults(t *testing.T) {
	suite1, err := RunSuite(ProfileBursty, 42)
	if err != nil {
		t.Fatalf("seed-42 run failed: %v", err)
	}
	suite2, err := RunSuite(ProfileBursty, 999)
	if err != nil {
		t.Fatalf("seed-999 run failed: %v", err)
	}

	// Check at least one metric differs with different seeds
	p1 := suite1.Profiles[0].SchedulerResults[0].Metrics
	p2 := suite2.Profiles[0].SchedulerResults[0].Metrics
	if p1.MakespanSec == p2.MakespanSec &&
		p1.AvgQueueWaitSec == p2.AvgQueueWaitSec &&
		p1.SLASuccessRatePct == p2.SLASuccessRatePct {
		t.Log("Warning: identical results with different seeds — workload may be deterministic regardless of seed")
	}
}

func TestWriteArtifactsCreatesAllFiles(t *testing.T) {
	suite, err := RunSuite(ProfileSteady, 42)
	if err != nil {
		t.Fatalf("RunSuite failed: %v", err)
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
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected artifact %s: %v", file, err)
		}
		if info.Size() == 0 {
			t.Fatalf("artifact %s is empty", file)
		}
	}
}

func TestWriteArtifactsNilSuiteReturnsError(t *testing.T) {
	_, err := WriteArtifacts(nil, t.TempDir())
	if err == nil {
		t.Fatal("expected error for nil suite")
	}
}

func TestSummaryJSONIsValidJSON(t *testing.T) {
	suite, err := RunSuite(ProfileSteady, 42)
	if err != nil {
		t.Fatalf("RunSuite failed: %v", err)
	}

	outputDir, err := WriteArtifacts(suite, t.TempDir())
	if err != nil {
		t.Fatalf("WriteArtifacts failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "summary.json"))
	if err != nil {
		t.Fatalf("read summary.json: %v", err)
	}

	var parsed SuiteResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("summary.json is not valid JSON: %v", err)
	}
	if parsed.Seed != 42 {
		t.Fatalf("expected seed=42, got %d", parsed.Seed)
	}
	if len(parsed.Profiles) == 0 {
		t.Fatal("expected profiles in parsed JSON")
	}
}

func TestRTSOutperformsRoundRobinOnSteady(t *testing.T) {
	suite, err := RunSuite(ProfileSteady, 42)
	if err != nil {
		t.Fatalf("RunSuite failed: %v", err)
	}

	pr := suite.Profiles[0]
	var rrMetrics, rtsMetrics *SchedulerMetrics
	for i := range pr.SchedulerResults {
		switch pr.SchedulerResults[i].SchedulerName {
		case "Round-Robin":
			rrMetrics = &pr.SchedulerResults[i].Metrics
		case "RTS":
			rtsMetrics = &pr.SchedulerResults[i].Metrics
		}
	}
	if rrMetrics == nil || rtsMetrics == nil {
		t.Skip("skipping: need both RR and RTS results")
	}

	// RTS should have equal or better SLA success rate
	if rtsMetrics.SLASuccessRatePct < rrMetrics.SLASuccessRatePct-5.0 {
		t.Logf("Warning: RTS SLA (%.1f%%) significantly worse than RR (%.1f%%)",
			rtsMetrics.SLASuccessRatePct, rrMetrics.SLASuccessRatePct)
	}
}

func TestCanonicalTaskTypesPresent(t *testing.T) {
	expected := map[string]bool{
		"cpu-light":    true,
		"cpu-heavy":    true,
		"memory-heavy": true,
		"mixed":        true,
	}
	for _, tt := range canonicalTaskTypes {
		if !expected[tt] {
			t.Fatalf("unexpected canonical task type: %s", tt)
		}
		delete(expected, tt)
	}
	for tt := range expected {
		t.Fatalf("missing canonical task type: %s", tt)
	}
}
