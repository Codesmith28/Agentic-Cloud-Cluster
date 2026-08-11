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

package cli

import "testing"

func TestParseTestRunOptionsAcceptsSchedulerVariants(t *testing.T) {
	opts, err := parseTestRunOptions([]string{"-scheduler", "RR", "-keep-env", "-ui-smoke"})
	if err != nil {
		t.Fatalf("parseTestRunOptions returned error: %v", err)
	}
	if opts.Scheduler != "rr" {
		t.Fatalf("expected scheduler rr, got %q", opts.Scheduler)
	}
	if !opts.KeepEnvironment {
		t.Fatalf("expected KeepEnvironment=true")
	}
	if !opts.EnableUISmoke {
		t.Fatalf("expected EnableUISmoke=true")
	}
}

func TestParseTestRunOptionsRejectsInvalidScheduler(t *testing.T) {
	if _, err := parseTestRunOptions([]string{"-scheduler", "PPO"}); err == nil {
		t.Fatal("expected error for unsupported scheduler")
	}
}

func TestIsSupportedTestSuite(t *testing.T) {
	if !isSupportedTestSuite("UI-SMOKE") {
		t.Fatal("expected ui-smoke suite to be supported")
	}
	if isSupportedTestSuite("unknown") {
		t.Fatal("expected unknown suite to be unsupported")
	}
}
