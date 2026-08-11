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

package scheduler

import (
	"testing"

	pb "master/proto"
)

func makeWorkerMap(workers ...*WorkerInfo) map[string]*WorkerInfo {
	m := make(map[string]*WorkerInfo, len(workers))
	for _, w := range workers {
		m[w.WorkerID] = w
	}
	return m
}

func activeWorker(id string, cpu, mem, stor, totCPU, totMem, totStor float64) *WorkerInfo {
	return &WorkerInfo{
		WorkerID:         id,
		IsActive:         true,
		WorkerIP:         "127.0.0.1:50052",
		AvailableCPU:     cpu,
		AvailableMemory:  mem,
		AvailableStorage: stor,
		TotalCPU:         totCPU,
		TotalMemory:      totMem,
		TotalStorage:     totStor,
	}
}

func simpleTask(id string, cpu, mem, stor float64) *pb.Task {
	return &pb.Task{
		TaskId:     id,
		ReqCpu:     cpu,
		ReqMemory:  mem,
		ReqStorage: stor,
	}
}

func TestRoundRobinSelectsNextSuitableWorker(t *testing.T) {
	rr := NewRoundRobinScheduler()
	workers := makeWorkerMap(
		activeWorker("w-a", 4, 8, 40, 4, 8, 40),
		activeWorker("w-b", 2, 4, 20, 2, 4, 20),
	)
	task := simpleTask("t-1", 1, 2, 5)

	first := rr.SelectWorker(task, workers)
	if first == "" {
		t.Fatal("expected a worker, got empty")
	}

	second := rr.SelectWorker(task, workers)
	if second == first {
		t.Fatalf("expected round-robin to pick different worker, got %s again", first)
	}

	third := rr.SelectWorker(task, workers)
	if third != first {
		t.Fatalf("expected round-robin cycle back to %s, got %s", first, third)
	}
}

func TestRoundRobinSkipsInactiveWorkers(t *testing.T) {
	rr := NewRoundRobinScheduler()
	inactive := activeWorker("w-a", 4, 8, 40, 4, 8, 40)
	inactive.IsActive = false

	active := activeWorker("w-b", 4, 8, 40, 4, 8, 40)
	workers := makeWorkerMap(inactive, active)
	task := simpleTask("t-1", 1, 2, 5)

	selected := rr.SelectWorker(task, workers)
	if selected != "w-b" {
		t.Fatalf("expected active worker w-b, got %s", selected)
	}
}

func TestRoundRobinSkipsInsufficientResources(t *testing.T) {
	rr := NewRoundRobinScheduler()
	workers := makeWorkerMap(
		activeWorker("w-a", 0.5, 8, 40, 4, 8, 40),
		activeWorker("w-b", 4, 0.5, 40, 4, 8, 40),
		activeWorker("w-c", 4, 8, 40, 4, 8, 40),
	)
	task := simpleTask("t-1", 2, 2, 5)

	selected := rr.SelectWorker(task, workers)
	if selected != "w-c" {
		t.Fatalf("expected w-c (only suitable), got %s", selected)
	}
}

func TestRoundRobinReturnsEmptyWhenNoSuitable(t *testing.T) {
	rr := NewRoundRobinScheduler()
	workers := makeWorkerMap(
		activeWorker("w-a", 0.5, 1, 5, 4, 8, 40),
	)
	task := simpleTask("t-1", 2, 4, 10)

	selected := rr.SelectWorker(task, workers)
	if selected != "" {
		t.Fatalf("expected empty when no suitable worker, got %s", selected)
	}
}

func TestRoundRobinEmptyWorkerMap(t *testing.T) {
	rr := NewRoundRobinScheduler()
	selected := rr.SelectWorker(simpleTask("t-1", 1, 1, 1), map[string]*WorkerInfo{})
	if selected != "" {
		t.Fatalf("expected empty for no workers, got %s", selected)
	}
}

func TestRoundRobinResetRestartsRotation(t *testing.T) {
	rr := NewRoundRobinScheduler()
	workers := makeWorkerMap(
		activeWorker("w-a", 4, 8, 40, 4, 8, 40),
		activeWorker("w-b", 4, 8, 40, 4, 8, 40),
	)
	task := simpleTask("t-1", 1, 1, 1)

	first := rr.SelectWorker(task, workers)
	rr.Reset()
	afterReset := rr.SelectWorker(task, workers)
	if afterReset != first {
		t.Fatalf("expected reset to restart rotation, first=%s afterReset=%s", first, afterReset)
	}
}

func TestRoundRobinGetName(t *testing.T) {
	rr := NewRoundRobinScheduler()
	if rr.GetName() != "Round-Robin" {
		t.Fatalf("expected name Round-Robin, got %s", rr.GetName())
	}
}

func TestRoundRobinDeterministicOrdering(t *testing.T) {
	workers := makeWorkerMap(
		activeWorker("w-c", 4, 8, 40, 4, 8, 40),
		activeWorker("w-a", 4, 8, 40, 4, 8, 40),
		activeWorker("w-b", 4, 8, 40, 4, 8, 40),
	)
	task := simpleTask("t-1", 1, 1, 1)

	rr := NewRoundRobinScheduler()
	var cycle []string
	for i := 0; i < 3; i++ {
		cycle = append(cycle, rr.SelectWorker(task, workers))
	}

	rr2 := NewRoundRobinScheduler()
	for i := 0; i < 3; i++ {
		got := rr2.SelectWorker(task, workers)
		if got != cycle[i] {
			t.Fatalf("iteration %d: expected %s, got %s (non-deterministic)", i, cycle[i], got)
		}
	}
}

func TestIsWorkerSuitableChecksBothActiveAndResources(t *testing.T) {
	task := simpleTask("t-1", 2, 4, 10)
	rr := NewRoundRobinScheduler()

	tests := []struct {
		name   string
		worker *WorkerInfo
		want   bool
	}{
		{
			name:   "active with sufficient resources",
			worker: activeWorker("w-1", 4, 8, 20, 4, 8, 20),
			want:   true,
		},
		{
			name:   "active with exact resources",
			worker: activeWorker("w-1", 2, 4, 10, 4, 8, 20),
			want:   true,
		},
		{
			name: "inactive worker",
			worker: &WorkerInfo{
				WorkerID: "w-1", IsActive: false, WorkerIP: "1.2.3.4:50052",
				AvailableCPU: 10, AvailableMemory: 16, AvailableStorage: 100,
				TotalCPU: 10, TotalMemory: 16, TotalStorage: 100,
			},
			want: false,
		},
		{
			name:   "insufficient CPU",
			worker: activeWorker("w-1", 1, 8, 20, 4, 8, 20),
			want:   false,
		},
		{
			name:   "insufficient memory",
			worker: activeWorker("w-1", 4, 2, 20, 4, 8, 20),
			want:   false,
		},
		{
			name:   "insufficient storage",
			worker: activeWorker("w-1", 4, 8, 5, 4, 8, 20),
			want:   false,
		},
		{
			name: "no IP configured",
			worker: &WorkerInfo{
				WorkerID: "w-1", IsActive: true, WorkerIP: "",
				AvailableCPU: 10, AvailableMemory: 16, AvailableStorage: 100,
				TotalCPU: 10, TotalMemory: 16, TotalStorage: 100,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rr.isWorkerSuitable(tt.worker, task)
			if got != tt.want {
				t.Fatalf("isWorkerSuitable() = %v, want %v", got, tt.want)
			}
		})
	}
}
