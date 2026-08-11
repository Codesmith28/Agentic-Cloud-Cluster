package scheduler

import (
	"reflect"
	"time"

	pb "master/proto"
)

// Task type constants
const (
	TaskTypeCPULight     = "cpu-light"
	TaskTypeCPUHeavy     = "cpu-heavy"
	TaskTypeMemoryHeavy  = "memory-heavy"
	TaskTypeGPUInference = "gpu-inference"
	TaskTypeGPUTraining  = "gpu-training"
	TaskTypeMixed        = "mixed"
)

// TaskView represents a scheduler-level view of a task with computed properties
type TaskView struct {
	ID          string
	Type        string    // Task type classification
	CPU         float64   // Required CPU
	Mem         float64   // Required memory
	GPU         float64   // Required GPU
	Storage     float64   // Required storage
	ArrivalTime time.Time // When task was submitted
	Tau         float64   // Base runtime estimate (seconds)
	Deadline    time.Time // Computed deadline: ArrivalTime + k * Tau
	UserID      string    // User who submitted the task
}

// WorkerView represents a scheduler-level view of a worker's current state
type WorkerView struct {
	ID           string  // Worker identifier
	CPUAvail     float64 // Available CPU cores
	MemAvail     float64 // Available memory (GB)
	GPUAvail     float64 // Available GPU units
	StorageAvail float64 // Available storage (GB)
	Load         float64 // Normalized load (may exceed 1.0 due to oversubscription)
}

// Theta contains the predictor weights for execution time estimation
type Theta struct {
	Theta1 float64 // Weight for CPU ratio impact
	Theta2 float64 // Weight for memory ratio impact
	Theta3 float64 // Weight for GPU ratio impact
	Theta4 float64 // Weight for worker load impact
}

// Risk contains the risk model weights for RTS scheduling
type Risk struct {
	Alpha float64 // Weight for deadline violation risk (Δᵢⱼ)
	Beta  float64 // Weight for worker load (Lⱼ)
}

// GAParams contains all parameters computed by the AOD module
// Simplified version - NO weights, only direct affinity/penalty computation
type GAParams struct {
	Theta          Theta                         // Execution time predictor weights
	Risk           Risk                          // Risk model weights
	AffinityMatrix map[string]map[string]float64 // [taskType][workerID] -> affinity score
	PenaltyVector  map[string]float64            // [workerID] -> penalty score
}

// NewTaskViewFromProto constructs a TaskView from a protobuf Task message
// Parameters:
//   - pbTask: The protobuf task message
//   - now: Current time (used as arrival time)
//   - tau: Base runtime estimate in seconds
//   - k: SLA multiplier (default: 2.0, valid range: 1.5-2.5) - if 0, uses pbTask.SlaMultiplier or default 2.0
func NewTaskViewFromProto(pbTask *pb.Task, now time.Time, tau float64, k float64) TaskView {
	// Determine task type: use explicit type if provided and valid, otherwise infer
	taskType := ""

	// Try to get task type from proto using reflection (for forward compatibility)
	method := reflect.ValueOf(pbTask).MethodByName("GetTaskType")
	if method.IsValid() {
		results := method.Call(nil)
		if len(results) == 1 {
			if typeStr, ok := results[0].Interface().(string); ok && typeStr != "" {
				taskType = typeStr
			}
		}
	}

	// Validate the task type if provided
	if taskType != "" && !ValidateTaskType(taskType) {
		taskType = "" // Invalid type, will infer
	}

	// If no valid type provided, infer from resources
	if taskType == "" {
		taskType = InferTaskType(pbTask)
	}

	// Use k from task if provided and valid, otherwise use parameter k, otherwise default to 2.0
	slaMultiplier := k
	if slaMultiplier == 0 {
		slaMultiplier = GetSLAMultiplier(pbTask)
	}

	// Validate k is in acceptable range
	if slaMultiplier < 1.5 || slaMultiplier > 2.5 {
		slaMultiplier = 2.0
	}

	// Compute deadline: D_i = ArrivalTime + k * tau
	deadline := now.Add(time.Duration(slaMultiplier * tau * float64(time.Second)))

	return TaskView{
		ID:          pbTask.TaskId,
		Type:        taskType,
		CPU:         pbTask.ReqCpu,
		Mem:         pbTask.ReqMemory,
		GPU:         pbTask.ReqGpu,
		Storage:     pbTask.ReqStorage,
		ArrivalTime: now,
		Tau:         tau,
		Deadline:    deadline,
		UserID:      pbTask.UserId,
	}
}

// InferTaskType infers the task type from resource requirements
// This is used ONLY when the user does not specify a task type or provides an invalid one
// Inference rules:
//   - GPU > 2.0 AND CPU > 4.0 → gpu-training
//   - GPU > 0 → gpu-inference
//   - Memory > 8.0 → memory-heavy
//   - CPU > 4.0 → cpu-heavy
//   - CPU > 0 → cpu-light
//   - Otherwise → mixed
func InferTaskType(pbTask *pb.Task) string {
	// Check for GPU training (high GPU + high CPU)
	if pbTask.ReqGpu > 2.0 && pbTask.ReqCpu > 4.0 {
		return TaskTypeGPUTraining
	}

	// Check for GPU inference (any GPU usage)
	if pbTask.ReqGpu > 0 {
		return TaskTypeGPUInference
	}

	// Check for memory-heavy tasks
	if pbTask.ReqMemory > 8.0 {
		return TaskTypeMemoryHeavy
	}

	// Check for CPU-heavy tasks
	if pbTask.ReqCpu > 4.0 {
		return TaskTypeCPUHeavy
	}

	// Check for CPU-light tasks
	if pbTask.ReqCpu > 0 {
		return TaskTypeCPULight
	}

	// Default case
	return TaskTypeMixed
}

// ValidateTaskType checks if a task type string is one of the valid task types
// Returns true if the taskType is valid, false otherwise
func ValidateTaskType(taskType string) bool {
	switch taskType {
	case TaskTypeCPULight,
		TaskTypeCPUHeavy,
		TaskTypeMemoryHeavy,
		TaskTypeGPUInference,
		TaskTypeGPUTraining,
		TaskTypeMixed:
		return true
	default:
		return false
	}
}

// GetSLAMultiplier extracts and validates the SLA multiplier from a task
// Returns the k value in range [1.5, 2.5], defaulting to 2.0 if invalid
func GetSLAMultiplier(pbTask *pb.Task) float64 {
	// If task is nil, return default
	if pbTask == nil {
		return 2.0
	}

	// Prefer using a generated getter if present (e.g., GetSlaMultiplier).
	// Use reflection so this code compiles even if the generated getter/field is absent.
	method := reflect.ValueOf(pbTask).MethodByName("GetSlaMultiplier")
	if method.IsValid() {
		results := method.Call(nil)
		if len(results) == 1 {
			switch v := results[0].Interface().(type) {
			case float64:
				if v >= 1.5 && v <= 2.5 {
					return v
				}
			case float32:
				fv := float64(v)
				if fv >= 1.5 && fv <= 2.5 {
					return fv
				}
			case int32:
				fv := float64(v)
				if fv >= 1.5 && fv <= 2.5 {
					return fv
				}
			case int64:
				fv := float64(v)
				if fv >= 1.5 && fv <= 2.5 {
					return fv
				}
			}
		}
	}

	// Fallback to default when no getter/field is available or value out of range
	return 2.0
}
