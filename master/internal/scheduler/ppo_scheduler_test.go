package scheduler

import (
	"context"
	"testing"

	pb "master/proto"
)

type stubScheduler struct {
	name     string
	selected string
	selects  int
	resets   int
}

func (s *stubScheduler) SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string {
	s.selects++
	return s.selected
}

func (s *stubScheduler) GetName() string {
	if s.name != "" {
		return s.name
	}
	return "stub"
}

func (s *stubScheduler) Reset() {
	s.resets++
}

func TestPPOSchedulerFallbackModeUsesFallbackScheduler(t *testing.T) {
	fallback := &stubScheduler{name: "RTS", selected: "worker-b"}
	sched := &PPOScheduler{
		fallback: fallback,
		mode:     PPOModeFallback,
	}

	selected := sched.SelectWorker(simpleTask("task-1", 1, 1, 1), makeWorkerMap(
		activeWorker("worker-a", 4, 8, 40, 4, 8, 40),
		activeWorker("worker-b", 4, 8, 40, 4, 8, 40),
	))
	if selected != "worker-b" {
		t.Fatalf("expected fallback worker worker-b, got %s", selected)
	}
	if fallback.selects != 1 {
		t.Fatalf("expected fallback scheduler to be called once, got %d", fallback.selects)
	}
}

func TestPPOSchedulerReportOutcomeIsNoopOutsideActiveMode(t *testing.T) {
	sched := &PPOScheduler{mode: PPOModeShadow}
	if err := sched.ReportOutcome(context.Background(), TaskOutcome{TaskID: "task-1"}); err != nil {
		t.Fatalf("expected nil error in shadow mode, got %v", err)
	}

	sched.mode = PPOModeFallback
	if err := sched.ReportOutcome(context.Background(), TaskOutcome{TaskID: "task-2"}); err != nil {
		t.Fatalf("expected nil error in fallback mode, got %v", err)
	}
}
