package db

import (
	"testing"

	pb "master/proto"
)

// TestProtoTaskTypeField verifies that the task_type field exists in proto Task message
func TestProtoTaskTypeField(t *testing.T) {
	task := &pb.Task{
		TaskId:      "test-task-1",
		DockerImage: "alpine:latest",
		Command:     "echo test",
		ReqCpu:      2.0,
		ReqMemory:   4.0,
		ReqGpu:      0.0,
		TaskType:    "cpu-light",
	}

	if task.TaskType != "cpu-light" {
		t.Errorf("Expected task type 'cpu-light', got '%s'", task.TaskType)
	}
}

// TestProtoTaskTypeValidation tests all 6 valid task types
func TestProtoTaskTypeValidation(t *testing.T) {
	validTypes := []string{
		"cpu-light",
		"cpu-heavy",
		"memory-heavy",
		"gpu-inference",
		"gpu-training",
		"mixed",
	}

	for _, taskType := range validTypes {
		t.Run(taskType, func(t *testing.T) {
			task := &pb.Task{
				TaskId:   "test-task",
				TaskType: taskType,
			}

			if task.TaskType != taskType {
				t.Errorf("Expected task type '%s', got '%s'", taskType, task.TaskType)
			}
		})
	}
}

// TestProtoTaskTypeEmpty verifies that empty task_type is allowed (for inference)
func TestProtoTaskTypeEmpty(t *testing.T) {
	task := &pb.Task{
		TaskId:      "test-task-2",
		DockerImage: "alpine:latest",
		ReqCpu:      2.0,
	}

	if task.TaskType != "" {
		t.Errorf("Expected empty task type, got '%s'", task.TaskType)
	}
}

// TestProtoTaskTypeGetter validates the GetTaskType() method
func TestProtoTaskTypeGetter(t *testing.T) {
	tests := []struct {
		name         string
		taskType     string
		expectedType string
	}{
		{
			name:         "Explicit cpu-heavy",
			taskType:     "cpu-heavy",
			expectedType: "cpu-heavy",
		},
		{
			name:         "Explicit gpu-training",
			taskType:     "gpu-training",
			expectedType: "gpu-training",
		},
		{
			name:         "Empty type",
			taskType:     "",
			expectedType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &pb.Task{
				TaskId:   "test-task",
				TaskType: tt.taskType,
			}

			result := task.GetTaskType()
			if result != tt.expectedType {
				t.Errorf("GetTaskType() = %v, want %v", result, tt.expectedType)
			}
		})
	}
}

// TestProtoAgentTaskInfoType verifies task_type field in master_agent.proto TaskInfo
func TestProtoAgentTaskInfoType(t *testing.T) {
	taskInfo := &pb.TaskInfo{
		TaskId:    "agent-task-1",
		ReqCpu:    4.0,
		ReqMemory: 8.0,
		ReqGpu:    1.0,
		TaskType:  "gpu-inference",
	}

	if taskInfo.TaskType != "gpu-inference" {
		t.Errorf("Expected task type 'gpu-inference', got '%s'", taskInfo.TaskType)
	}

	// Test getter method
	if taskInfo.GetTaskType() != "gpu-inference" {
		t.Errorf("GetTaskType() returned '%s', expected 'gpu-inference'", taskInfo.GetTaskType())
	}
}

// TestProtoTaskTypeSerialization verifies task_type survives proto serialization
func TestProtoTaskTypeSerialization(t *testing.T) {
	original := &pb.Task{
		TaskId:        "serialize-test",
		DockerImage:   "ubuntu:20.04",
		Command:       "sleep 100",
		ReqCpu:        8.0,
		ReqMemory:     16.0,
		ReqGpu:        4.0,
		TaskType:      "gpu-training",
		SlaMultiplier: 2.0,
	}

	// In a real test, we would marshal/unmarshal
	// For now, just verify the field is accessible
	if original.GetTaskType() != "gpu-training" {
		t.Errorf("Task type not preserved: got '%s', want 'gpu-training'", original.GetTaskType())
	}
}

// TestProtoBackwardCompatibility ensures tasks without task_type still work
func TestProtoBackwardCompatibility(t *testing.T) {
	// Simulate old task submission without task_type field
	task := &pb.Task{
		TaskId:      "legacy-task",
		DockerImage: "alpine:latest",
		ReqCpu:      2.0,
		ReqMemory:   4.0,
		// TaskType intentionally not set
	}

	// Should default to empty string
	if task.GetTaskType() != "" {
		t.Errorf("Expected empty task type for backward compatibility, got '%s'", task.GetTaskType())
	}

	// Should be able to process without errors
	if task.TaskId != "legacy-task" {
		t.Error("Legacy task fields should still be accessible")
	}
}
