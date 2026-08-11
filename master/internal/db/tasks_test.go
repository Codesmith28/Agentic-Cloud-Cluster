package db

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// TestUpdateTaskMetadata tests the UpdateTaskMetadata function
func TestUpdateTaskMetadata(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		taskDB := &TaskDB{
			client:     mt.Client,
			collection: mt.Coll,
		}

		// Mock successful update
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		err := taskDB.UpdateTaskMetadata(context.Background(), "task-123", "cpu-heavy", 2.0)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	mt.Run("task not found", func(mt *mtest.T) {
		taskDB := &TaskDB{
			client:     mt.Client,
			collection: mt.Coll,
		}

		// Mock no documents matched
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 0},
			bson.E{Key: "nModified", Value: 0},
		))

		err := taskDB.UpdateTaskMetadata(context.Background(), "nonexistent", "cpu-light", 1.5)
		if err == nil {
			t.Error("Expected error for task not found")
		}
	})
}

// TestTaskStructWithNewFields tests that Task struct has new fields
func TestTaskStructWithNewFields(t *testing.T) {
	task := Task{
		TaskID:      "task-123",
		UserID:      "user-001",
		DockerImage: "ubuntu:latest",
		Command:     "echo test",
		ReqCPU:      2.0,
		ReqMemory:   4.0,
		ReqStorage:  10.0,
		ReqGPU:      0.0,
		Tag:         "cpu-heavy",
		KValue:      2.0,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	// Verify Tag field exists and is set
	if task.Tag != "cpu-heavy" {
		t.Errorf("Expected Tag to be 'cpu-heavy', got '%s'", task.Tag)
	}

	// Verify KValue field exists and is set
	if task.KValue != 2.0 {
		t.Errorf("Expected KValue to be 2.0, got %f", task.KValue)
	}
}

// TestKValueRange validates that k_value is within acceptable range
func TestKValueRange(t *testing.T) {
	tests := []struct {
		name    string
		kValue  float64
		isValid bool
	}{
		{"minimum valid", 1.5, true},
		{"mid range", 2.0, true},
		{"maximum valid", 2.5, true},
		{"below minimum", 1.4, false},
		{"above maximum", 2.6, false},
		{"step 0.1", 1.8, true},
		{"step 0.1", 2.3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate range
			valid := tt.kValue >= 1.5 && tt.kValue <= 2.5
			if valid != tt.isValid {
				t.Errorf("Expected kValue %f validity to be %v, got %v", tt.kValue, tt.isValid, valid)
			}
		})
	}
}

// TestTagValues validates acceptable tag values
func TestTagValues(t *testing.T) {
	validTags := []string{
		"cpu-light",
		"cpu-heavy",
		"memory-light",
		"memory-heavy",
		"gpu-training",
		"mixed",
	}

	for _, tag := range validTags {
		t.Run(tag, func(t *testing.T) {
			task := Task{
				TaskID: "test-task",
				Tag:    tag,
			}

			if task.Tag != tag {
				t.Errorf("Expected tag to be '%s', got '%s'", tag, task.Tag)
			}
		})
	}
}
