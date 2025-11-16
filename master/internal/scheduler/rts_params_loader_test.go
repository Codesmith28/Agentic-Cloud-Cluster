package scheduler

import (
	"os"
	"path/filepath"
	"testing"
)

// Test GetDefaultGAParams returns valid defaults
func TestGetDefaultGAParams(t *testing.T) {
	params := GetDefaultGAParams()

	// Verify Theta values
	if params.Theta.Theta1 != 0.1 {
		t.Errorf("Expected Theta1=0.1, got %.2f", params.Theta.Theta1)
	}
	if params.Theta.Theta2 != 0.1 {
		t.Errorf("Expected Theta2=0.1, got %.2f", params.Theta.Theta2)
	}
	if params.Theta.Theta3 != 0.3 {
		t.Errorf("Expected Theta3=0.3, got %.2f", params.Theta.Theta3)
	}
	if params.Theta.Theta4 != 0.2 {
		t.Errorf("Expected Theta4=0.2, got %.2f", params.Theta.Theta4)
	}

	// Verify Risk values
	if params.Risk.Alpha != 10.0 {
		t.Errorf("Expected Alpha=10.0, got %.2f", params.Risk.Alpha)
	}
	if params.Risk.Beta != 1.0 {
		t.Errorf("Expected Beta=1.0, got %.2f", params.Risk.Beta)
	}

	// Verify Affinity weights
	if params.AffinityW.A1 != 1.0 {
		t.Errorf("Expected A1=1.0, got %.2f", params.AffinityW.A1)
	}
	if params.AffinityW.A2 != 2.0 {
		t.Errorf("Expected A2=2.0, got %.2f", params.AffinityW.A2)
	}
	if params.AffinityW.A3 != 0.5 {
		t.Errorf("Expected A3=0.5, got %.2f", params.AffinityW.A3)
	}

	// Verify Penalty weights
	if params.PenaltyW.G1 != 2.0 {
		t.Errorf("Expected G1=2.0, got %.2f", params.PenaltyW.G1)
	}
	if params.PenaltyW.G2 != 1.0 {
		t.Errorf("Expected G2=1.0, got %.2f", params.PenaltyW.G2)
	}
	if params.PenaltyW.G3 != 0.5 {
		t.Errorf("Expected G3=0.5, got %.2f", params.PenaltyW.G3)
	}

	// Verify maps are initialized
	if params.AffinityMatrix == nil {
		t.Error("AffinityMatrix should be initialized")
	}
	if params.PenaltyVector == nil {
		t.Error("PenaltyVector should be initialized")
	}
}

// Test SaveToFile and LoadGAParams round-trip
func TestSaveAndLoadGAParams(t *testing.T) {
	// Create temporary file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_params.json")

	// Create test params
	params := &GAParams{
		Theta: Theta{
			Theta1: 0.15,
			Theta2: 0.12,
			Theta3: 0.35,
			Theta4: 0.25,
		},
		Risk: Risk{
			Alpha: 12.5,
			Beta:  1.5,
		},
		AffinityW: AffinityWeights{
			A1: 1.2,
			A2: 2.5,
			A3: 0.8,
		},
		PenaltyW: PenaltyWeights{
			G1: 2.5,
			G2: 1.2,
			G3: 0.6,
		},
		AffinityMatrix: map[string]map[string]float64{
			TaskTypeCPULight: {
				"worker1": 0.5,
				"worker2": 0.3,
			},
			TaskTypeGPUTraining: {
				"worker1": -0.2,
				"worker2": 0.8,
			},
		},
		PenaltyVector: map[string]float64{
			"worker1": 1.5,
			"worker2": 0.5,
		},
	}

	// Save to file
	err := params.SaveToFile(filePath)
	if err != nil {
		t.Fatalf("Failed to save params: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("File was not created")
	}

	// Load from file
	loaded, err := LoadGAParams(filePath)
	if err != nil {
		t.Fatalf("Failed to load params: %v", err)
	}

	// Verify loaded params match original
	if loaded.Theta.Theta1 != params.Theta.Theta1 {
		t.Errorf("Theta1 mismatch: expected %.2f, got %.2f", params.Theta.Theta1, loaded.Theta.Theta1)
	}
	if loaded.Risk.Alpha != params.Risk.Alpha {
		t.Errorf("Alpha mismatch: expected %.2f, got %.2f", params.Risk.Alpha, loaded.Risk.Alpha)
	}
	if loaded.AffinityW.A2 != params.AffinityW.A2 {
		t.Errorf("A2 mismatch: expected %.2f, got %.2f", params.AffinityW.A2, loaded.AffinityW.A2)
	}
	if loaded.PenaltyW.G3 != params.PenaltyW.G3 {
		t.Errorf("G3 mismatch: expected %.2f, got %.2f", params.PenaltyW.G3, loaded.PenaltyW.G3)
	}

	// Verify affinity matrix
	if loaded.AffinityMatrix[TaskTypeCPULight]["worker1"] != 0.5 {
		t.Errorf("Affinity mismatch for cpu-light/worker1")
	}

	// Verify penalty vector
	if loaded.PenaltyVector["worker2"] != 0.5 {
		t.Errorf("Penalty mismatch for worker2")
	}
}

// Test LoadGAParams with non-existent file
func TestLoadGAParams_FileNotFound(t *testing.T) {
	_, err := LoadGAParams("/nonexistent/path/params.json")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

// Test LoadGAParams with invalid JSON
func TestLoadGAParams_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(filePath, []byte("{invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err = LoadGAParams(filePath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// Test validation with out-of-range Theta values
func TestValidateGAParams_InvalidTheta(t *testing.T) {
	tests := []struct {
		name   string
		params *GAParams
	}{
		{
			name: "Theta1 too high",
			params: &GAParams{
				Theta: Theta{Theta1: 15.0, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
				Risk:  Risk{Alpha: 10.0, Beta: 1.0},
			},
		},
		{
			name: "Theta2 negative",
			params: &GAParams{
				Theta: Theta{Theta1: 0.1, Theta2: -0.5, Theta3: 0.3, Theta4: 0.2},
				Risk:  Risk{Alpha: 10.0, Beta: 1.0},
			},
		},
		{
			name: "Theta3 too high",
			params: &GAParams{
				Theta: Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 12.0, Theta4: 0.2},
				Risk:  Risk{Alpha: 10.0, Beta: 1.0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGAParams(tt.params)
			if err == nil {
				t.Error("Expected validation error for invalid Theta")
			}
		})
	}
}

// Test validation with out-of-range Risk values
func TestValidateGAParams_InvalidRisk(t *testing.T) {
	tests := []struct {
		name   string
		params *GAParams
	}{
		{
			name: "Alpha too high",
			params: &GAParams{
				Theta: Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
				Risk:  Risk{Alpha: 2000.0, Beta: 1.0},
			},
		},
		{
			name: "Beta negative",
			params: &GAParams{
				Theta: Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
				Risk:  Risk{Alpha: 10.0, Beta: -5.0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGAParams(tt.params)
			if err == nil {
				t.Error("Expected validation error for invalid Risk")
			}
		})
	}
}

// Test validation with invalid affinity matrix task types
func TestValidateGAParams_InvalidTaskType(t *testing.T) {
	params := &GAParams{
		Theta: Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
		Risk:  Risk{Alpha: 10.0, Beta: 1.0},
		AffinityMatrix: map[string]map[string]float64{
			"invalid-task-type": {
				"worker1": 0.5,
			},
		},
	}

	err := validateGAParams(params)
	if err == nil {
		t.Error("Expected validation error for invalid task type")
	}
}

// Test validation with valid task types in affinity matrix
func TestValidateGAParams_ValidTaskTypes(t *testing.T) {
	params := &GAParams{
		Theta:     Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
		Risk:      Risk{Alpha: 10.0, Beta: 1.0},
		AffinityW: AffinityWeights{A1: 1.0, A2: 2.0, A3: 0.5},
		PenaltyW:  PenaltyWeights{G1: 2.0, G2: 1.0, G3: 0.5},
		AffinityMatrix: map[string]map[string]float64{
			TaskTypeCPULight:     {"worker1": 0.5},
			TaskTypeCPUHeavy:     {"worker1": 0.3},
			TaskTypeMemoryHeavy:  {"worker1": 0.2},
			TaskTypeGPUInference: {"worker1": 0.8},
			TaskTypeGPUTraining:  {"worker1": 0.9},
			TaskTypeMixed:        {"worker1": 0.4},
		},
		PenaltyVector: map[string]float64{
			"worker1": 1.0,
		},
	}

	err := validateGAParams(params)
	if err != nil {
		t.Errorf("Expected no validation error for valid task types, got: %v", err)
	}
}

// Test validation with out-of-range affinity values
func TestValidateGAParams_InvalidAffinityValue(t *testing.T) {
	params := &GAParams{
		Theta: Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
		Risk:  Risk{Alpha: 10.0, Beta: 1.0},
		AffinityMatrix: map[string]map[string]float64{
			TaskTypeCPULight: {
				"worker1": 15.0, // Out of range
			},
		},
	}

	err := validateGAParams(params)
	if err == nil {
		t.Error("Expected validation error for affinity out of range")
	}
}

// Test validation with out-of-range penalty values
func TestValidateGAParams_InvalidPenaltyValue(t *testing.T) {
	params := &GAParams{
		Theta: Theta{Theta1: 0.1, Theta2: 0.1, Theta3: 0.3, Theta4: 0.2},
		Risk:  Risk{Alpha: 10.0, Beta: 1.0},
		PenaltyVector: map[string]float64{
			"worker1": 150.0, // Out of range
		},
	}

	err := validateGAParams(params)
	if err == nil {
		t.Error("Expected validation error for penalty out of range")
	}
}

// Test LoadGAParamsOrDefault with valid file
func TestLoadGAParamsOrDefault_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "params.json")

	// Save default params
	defaults := GetDefaultGAParams()
	defaults.Theta.Theta1 = 0.25 // Modify to distinguish from defaults
	err := defaults.SaveToFile(filePath)
	if err != nil {
		t.Fatalf("Failed to save params: %v", err)
	}

	// Load with fallback
	loaded := LoadGAParamsOrDefault(filePath)

	if loaded.Theta.Theta1 != 0.25 {
		t.Errorf("Expected loaded Theta1=0.25, got %.2f", loaded.Theta.Theta1)
	}
}

// Test LoadGAParamsOrDefault with missing file returns defaults
func TestLoadGAParamsOrDefault_MissingFile(t *testing.T) {
	loaded := LoadGAParamsOrDefault("/nonexistent/params.json")

	// Should return defaults
	if loaded.Theta.Theta1 != 0.1 {
		t.Errorf("Expected default Theta1=0.1, got %.2f", loaded.Theta.Theta1)
	}
	if loaded.Risk.Alpha != 10.0 {
		t.Errorf("Expected default Alpha=10.0, got %.2f", loaded.Risk.Alpha)
	}
}

// Test LoadGAParamsOrDefault with invalid file returns defaults
func TestLoadGAParamsOrDefault_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(filePath, []byte("{invalid"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loaded := LoadGAParamsOrDefault(filePath)

	// Should return defaults
	if loaded.Theta.Theta1 != 0.1 {
		t.Errorf("Expected default Theta1=0.1, got %.2f", loaded.Theta.Theta1)
	}
}

// Test JSON format is readable and pretty-printed
func TestSaveToFile_PrettyPrint(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "params.json")

	params := GetDefaultGAParams()
	params.AffinityMatrix[TaskTypeCPULight] = map[string]float64{
		"worker1": 0.5,
	}

	err := params.SaveToFile(filePath)
	if err != nil {
		t.Fatalf("Failed to save params: %v", err)
	}

	// Read file and verify it contains newlines (pretty-printed)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	content := string(data)
	if len(content) < 100 {
		t.Error("JSON content seems too short")
	}

	// Check for pretty-printing indicators
	hasNewlines := false
	hasIndentation := false
	for i := 0; i < len(content)-1; i++ {
		if content[i] == '\n' {
			hasNewlines = true
		}
		if content[i] == '\n' && i+1 < len(content) && content[i+1] == ' ' {
			hasIndentation = true
		}
	}

	if !hasNewlines {
		t.Error("JSON should be pretty-printed with newlines")
	}
	if !hasIndentation {
		t.Error("JSON should be pretty-printed with indentation")
	}
}

// Test complete affinity matrix with all 6 task types
func TestValidateGAParams_CompleteAffinityMatrix(t *testing.T) {
	params := GetDefaultGAParams()

	// Populate affinity matrix with all 6 task types
	params.AffinityMatrix = map[string]map[string]float64{
		TaskTypeCPULight:     {"worker1": 0.5, "worker2": 0.3},
		TaskTypeCPUHeavy:     {"worker1": 0.3, "worker2": 0.6},
		TaskTypeMemoryHeavy:  {"worker1": 0.2, "worker2": 0.4},
		TaskTypeGPUInference: {"worker1": 0.8, "worker2": 0.1},
		TaskTypeGPUTraining:  {"worker1": 0.9, "worker2": 0.2},
		TaskTypeMixed:        {"worker1": 0.4, "worker2": 0.5},
	}

	err := validateGAParams(params)
	if err != nil {
		t.Errorf("Expected no error for complete affinity matrix, got: %v", err)
	}

	// Verify all 6 task types are present
	if len(params.AffinityMatrix) != 6 {
		t.Errorf("Expected 6 task types in affinity matrix, got %d", len(params.AffinityMatrix))
	}
}
