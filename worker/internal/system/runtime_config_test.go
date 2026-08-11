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

func TestResolveWorkerMetricsPortUsesEnvOverride(t *testing.T) {
	t.Setenv(workerMetricsPortEnv, "9105")

	port, err := ResolveWorkerMetricsPort(9101)
	if err != nil {
		t.Fatalf("ResolveWorkerMetricsPort returned error: %v", err)
	}
	if port != 9105 {
		t.Fatalf("expected override port 9105, got %d", port)
	}
}

func TestApplyResourceOverrides(t *testing.T) {
	t.Setenv(workerTotalCPUEnv, "2.5")
	t.Setenv(workerTotalMemoryGBEnv, "6")
	t.Setenv(workerTotalStorageGBEnv, "40")

	resources := &ResourceInfo{
		TotalCPU:     1.0,
		TotalMemory:  2.0,
		TotalStorage: 10.0,
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
}
