package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadGAParams loads GAParams from a JSON file
// Returns error if file doesn't exist or contains invalid data
func LoadGAParams(filePath string) (*GAParams, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read GA params file: %w", err)
	}

	// Unmarshal JSON
	var params GAParams
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, fmt.Errorf("failed to parse GA params JSON: %w", err)
	}

	// Validate parameters
	if err := validateGAParams(&params); err != nil {
		return nil, fmt.Errorf("invalid GA params: %w", err)
	}

	return &params, nil
}

// SaveToFile saves GAParams to a JSON file with pretty formatting
func (p *GAParams) SaveToFile(filePath string) error {
	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal GA params: %w", err)
	}

	// Write to file with appropriate permissions
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write GA params file: %w", err)
	}

	return nil
}

// GetDefaultGAParams returns sensible default parameters based on EDD §6
// These defaults are used when no trained parameters are available
func GetDefaultGAParams() *GAParams {
	return &GAParams{
		// Execution time predictor weights (EDD §3.5)
		Theta: Theta{
			Theta1: 0.1, // CPU ratio impact
			Theta2: 0.1, // Memory ratio impact
			Theta3: 0.3, // GPU ratio impact (higher weight)
			Theta4: 0.2, // Worker load impact
		},

		// Risk model weights (EDD §3.7)
		Risk: Risk{
			Alpha: 10.0, // Deadline violation penalty (high priority)
			Beta:  1.0,  // Worker load consideration
		},

		// Initialize empty maps (will be populated by AOD training)
		// Affinity matrix structure: map[taskType]map[workerID]affinity
		// Should have 6 task types: cpu-light, cpu-heavy, memory-heavy,
		// gpu-heavy, gpu-training, mixed
		AffinityMatrix: make(map[string]map[string]float64),

		// Penalty vector structure: map[workerID]penalty
		PenaltyVector: make(map[string]float64),
	}
}

// validateGAParams checks if GAParams contains reasonable values
func validateGAParams(params *GAParams) error {
	// Validate Theta parameters (should be reasonable multipliers)
	if params.Theta.Theta1 < 0 || params.Theta.Theta1 > 10 {
		return fmt.Errorf("Theta1 out of range [0, 10]: %.2f", params.Theta.Theta1)
	}
	if params.Theta.Theta2 < 0 || params.Theta.Theta2 > 10 {
		return fmt.Errorf("Theta2 out of range [0, 10]: %.2f", params.Theta.Theta2)
	}
	if params.Theta.Theta3 < 0 || params.Theta.Theta3 > 10 {
		return fmt.Errorf("Theta3 out of range [0, 10]: %.2f", params.Theta.Theta3)
	}
	if params.Theta.Theta4 < 0 || params.Theta.Theta4 > 10 {
		return fmt.Errorf("Theta4 out of range [0, 10]: %.2f", params.Theta.Theta4)
	}

	// Validate Risk parameters (should be positive)
	if params.Risk.Alpha < 0 || params.Risk.Alpha > 1000 {
		return fmt.Errorf("Alpha out of range [0, 1000]: %.2f", params.Risk.Alpha)
	}
	if params.Risk.Beta < 0 || params.Risk.Beta > 100 {
		return fmt.Errorf("Beta out of range [0, 100]: %.2f", params.Risk.Beta)
	}

	// Validate Affinity matrix structure
	if params.AffinityMatrix != nil {
		// Check for valid task types in affinity matrix
		validTaskTypes := map[string]bool{
			TaskTypeCPULight:     true,
			TaskTypeCPUHeavy:     true,
			TaskTypeMemoryHeavy:  true,
			TaskTypeGPUInference: true,
			TaskTypeGPUTraining:  true,
			TaskTypeMixed:        true,
		}

		for taskType := range params.AffinityMatrix {
			if !validTaskTypes[taskType] {
				return fmt.Errorf("invalid task type in affinity matrix: %s", taskType)
			}

			// Check affinity values are in reasonable range
			for workerID, affinity := range params.AffinityMatrix[taskType] {
				if affinity < -10 || affinity > 10 {
					return fmt.Errorf("affinity out of range [-10, 10] for task %s, worker %s: %.2f",
						taskType, workerID, affinity)
				}
			}
		}
	}

	// Validate Penalty vector
	if params.PenaltyVector != nil {
		for workerID, penalty := range params.PenaltyVector {
			if penalty < 0 || penalty > 100 {
				return fmt.Errorf("penalty out of range [0, 100] for worker %s: %.2f",
					workerID, penalty)
			}
		}
	}

	return nil
}

// LoadGAParamsOrDefault attempts to load GAParams from file,
// returns defaults if file doesn't exist or is invalid
func LoadGAParamsOrDefault(filePath string) *GAParams {
	params, err := LoadGAParams(filePath)
	if err != nil {
		// Log the error but don't fail - use defaults
		// This allows the system to start even without trained parameters
		return GetDefaultGAParams()
	}
	return params
}
