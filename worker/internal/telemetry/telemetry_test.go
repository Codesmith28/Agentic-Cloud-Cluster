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

package telemetry

import (
	"testing"
	"time"
)

func TestAddTaskCarriesAttemptMetadata(t *testing.T) {
	monitor := NewMonitor("worker-1", time.Second)
	monitor.AddTask("task-1", "att-task-1-2", 2, 1.5, 3.0)

	runningTasks := monitor.GetRunningTasks()
	if len(runningTasks) != 1 {
		t.Fatalf("expected 1 running task, got %d", len(runningTasks))
	}

	task := runningTasks[0]
	if task.TaskId != "task-1" {
		t.Fatalf("unexpected task id: %s", task.TaskId)
	}
	if task.AttemptId != "att-task-1-2" {
		t.Fatalf("unexpected attempt id: %s", task.AttemptId)
	}
	if task.AttemptNo != 2 {
		t.Fatalf("unexpected attempt no: %d", task.AttemptNo)
	}
}
