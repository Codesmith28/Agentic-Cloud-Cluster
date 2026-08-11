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
	"context"
	"time"

	pb "master/proto"
)

// Scheduler is the interface that all scheduling algorithms must implement
type Scheduler interface {
	// SelectWorker selects the best worker for the given task
	// Returns worker ID or empty string if no suitable worker found
	SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string

	// GetName returns the name of the scheduling algorithm
	GetName() string

	// Reset resets any internal state (useful for testing)
	Reset()
}

// TaskOutcome is fed back to schedulers that support online learning.
type TaskOutcome struct {
	TaskID           string
	WorkerID         string
	Status           string
	Reward           float64
	RuntimeSeconds   float64
	SLASuccess       bool
	Task             *pb.Task
	ClusterHash      string
	ModelVersionHint string
	CompletedAt      time.Time
}

// OutcomeReporter is implemented by schedulers that accept post-execution feedback.
type OutcomeReporter interface {
	ReportOutcome(ctx context.Context, outcome TaskOutcome) error
}
