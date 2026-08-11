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

package server

import (
	"testing"

	"master/internal/db"
)

func TestShouldIgnoreAttemptResult(t *testing.T) {
	tests := []struct {
		name                   string
		currentAttemptID       string
		resultAttemptID        string
		persistedAttemptStatus string
		want                   bool
	}{
		{
			name:             "missing result attempt is not stale",
			currentAttemptID: "att-task-1-2",
			resultAttemptID:  "",
			want:             false,
		},
		{
			name:                   "lost attempt is always ignored",
			currentAttemptID:       "att-task-1-2",
			resultAttemptID:        "att-task-1-2",
			persistedAttemptStatus: db.AttemptStatusLost,
			want:                   true,
		},
		{
			name:                   "stale attempt is always ignored",
			currentAttemptID:       "att-task-1-2",
			resultAttemptID:        "att-task-1-1",
			persistedAttemptStatus: db.AttemptStatusStale,
			want:                   true,
		},
		{
			name:             "different active attempt is ignored",
			currentAttemptID: "att-task-1-2",
			resultAttemptID:  "att-task-1-1",
			want:             true,
		},
		{
			name:             "current attempt is accepted",
			currentAttemptID: "att-task-1-2",
			resultAttemptID:  "att-task-1-2",
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldIgnoreAttemptResult(tt.currentAttemptID, tt.resultAttemptID, tt.persistedAttemptStatus)
			if got != tt.want {
				t.Fatalf("shouldIgnoreAttemptResult(%q, %q, %q) = %v, want %v", tt.currentAttemptID, tt.resultAttemptID, tt.persistedAttemptStatus, got, tt.want)
			}
		})
	}
}

func TestNextAttemptID(t *testing.T) {
	if got, want := nextAttemptID("task-123", 4), "att-task-123-4"; got != want {
		t.Fatalf("nextAttemptID() = %q, want %q", got, want)
	}
}
