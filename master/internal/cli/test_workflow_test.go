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
