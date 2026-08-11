package telemetry

import (
	"sync"
)

// TauStore is the interface for managing task type-specific runtime estimations (tau values)
// Tau represents the expected runtime for each task type and is updated using EMA (Exponential Moving Average)
type TauStore interface {
	// GetTau retrieves the current tau value for a given task type
	// Returns the task-type-specific default if not found
	GetTau(taskType string) float64

	// UpdateTau updates the tau value for a task type using EMA based on actual runtime
	// Formula: tau_new = lambda * actualRuntime + (1-lambda) * tau_old
	UpdateTau(taskType string, actualRuntime float64)

	// SetTau explicitly sets the tau value for a task type
	// Useful for initialization or manual overrides
	SetTau(taskType string, tau float64)
}

// InMemoryTauStore implements TauStore using an in-memory map with thread-safe operations
type InMemoryTauStore struct {
	tauMap map[string]float64 // Maps task type to tau value
	mu     sync.RWMutex       // Protects concurrent access to tauMap
	lambda float64            // EMA weight for new observations (default 0.2)
}

// Task type constants (must match scheduler.TaskType* constants)
const (
	TaskTypeCPULight     = "cpu-light"
	TaskTypeCPUHeavy     = "cpu-heavy"
	TaskTypeMemoryHeavy  = "memory-heavy"
	TaskTypeGPUInference = "gpu-inference"
	TaskTypeGPUTraining  = "gpu-training"
	TaskTypeMixed        = "mixed"
)

// Default tau values (in seconds) for each task type
var defaultTauValues = map[string]float64{
	TaskTypeCPULight:     5.0,  // Light CPU tasks typically finish quickly
	TaskTypeCPUHeavy:     15.0, // Heavy CPU tasks take longer
	TaskTypeMemoryHeavy:  20.0, // Memory-intensive tasks need more time
	TaskTypeGPUInference: 10.0, // GPU inference is relatively fast
	TaskTypeGPUTraining:  60.0, // GPU training can take much longer
	TaskTypeMixed:        10.0, // Mixed workloads use a moderate default
}

// NewInMemoryTauStore creates a new InMemoryTauStore with default tau values
func NewInMemoryTauStore() *InMemoryTauStore {
	store := &InMemoryTauStore{
		tauMap: make(map[string]float64),
		lambda: 0.2, // Default EMA weight (20% new, 80% old)
	}

	// Initialize with default tau values for all 6 task types
	for taskType, tau := range defaultTauValues {
		store.tauMap[taskType] = tau
	}

	return store
}

// NewInMemoryTauStoreWithLambda creates a new InMemoryTauStore with a custom lambda value
func NewInMemoryTauStoreWithLambda(lambda float64) *InMemoryTauStore {
	store := NewInMemoryTauStore()
	store.lambda = lambda
	return store
}

// GetTau retrieves the current tau value for a given task type
// Returns the task-type-specific default if the task type is not found or invalid
func (s *InMemoryTauStore) GetTau(taskType string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if tau exists in map
	if tau, exists := s.tauMap[taskType]; exists {
		return tau
	}

	// Return task-type-specific default if not found
	if defaultTau, exists := defaultTauValues[taskType]; exists {
		return defaultTau
	}

	// Fallback to mixed type default for unknown task types
	return defaultTauValues[TaskTypeMixed]
}

// UpdateTau updates the tau value for a task type using EMA based on actual runtime
// Formula: tau_new = lambda * actualRuntime + (1-lambda) * tau_old
// Only updates if taskType is one of the 6 valid types
func (s *InMemoryTauStore) UpdateTau(taskType string, actualRuntime float64) {
	// Validate task type before updating
	if !isValidTaskType(taskType) {
		return
	}

	// Ignore invalid runtime values
	if actualRuntime <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current tau value (or default if not set)
	tauOld := s.tauMap[taskType]
	if tauOld == 0 {
		// If no previous value, use default
		if defaultTau, exists := defaultTauValues[taskType]; exists {
			tauOld = defaultTau
		} else {
			tauOld = defaultTauValues[TaskTypeMixed]
		}
	}

	// Apply EMA formula: tau_new = lambda * actualRuntime + (1-lambda) * tau_old
	tauNew := s.lambda*actualRuntime + (1-s.lambda)*tauOld

	// Update the map with new tau value
	s.tauMap[taskType] = tauNew
}

// SetTau explicitly sets the tau value for a task type
// Useful for initialization, manual overrides, or loading from persistent storage
func (s *InMemoryTauStore) SetTau(taskType string, tau float64) {
	// Validate task type
	if !isValidTaskType(taskType) {
		return
	}

	// Ignore invalid tau values
	if tau <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.tauMap[taskType] = tau
}

// GetAllTau returns a copy of all tau values (useful for debugging/monitoring)
func (s *InMemoryTauStore) GetAllTau() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid external modifications
	result := make(map[string]float64, len(s.tauMap))
	for k, v := range s.tauMap {
		result[k] = v
	}
	return result
}

// GetLambda returns the current EMA lambda value
func (s *InMemoryTauStore) GetLambda() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lambda
}

// SetLambda updates the EMA lambda value
// Lambda should be in range [0, 1] where:
// - 0 means only use historical data (no learning)
// - 1 means only use new observations (no memory)
// - 0.2 (default) gives 20% weight to new data, 80% to historical
func (s *InMemoryTauStore) SetLambda(lambda float64) {
	if lambda < 0 || lambda > 1 {
		return // Ignore invalid lambda values
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.lambda = lambda
}

// isValidTaskType checks if the given task type is one of the 6 valid types
func isValidTaskType(taskType string) bool {
	validTypes := []string{
		TaskTypeCPULight,
		TaskTypeCPUHeavy,
		TaskTypeMemoryHeavy,
		TaskTypeGPUInference,
		TaskTypeGPUTraining,
		TaskTypeMixed,
	}

	for _, valid := range validTypes {
		if taskType == valid {
			return true
		}
	}
	return false
}

// ResetToDefaults resets all tau values back to their defaults
// Useful for testing or recovering from poor learning
func (s *InMemoryTauStore) ResetToDefaults() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for taskType, tau := range defaultTauValues {
		s.tauMap[taskType] = tau
	}
}
