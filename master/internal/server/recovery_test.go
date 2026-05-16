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
