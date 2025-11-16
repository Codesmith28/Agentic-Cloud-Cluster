package aod

import (
	"fmt"
	"math"
	"testing"
	"time"

	"master/internal/db"
	"master/internal/scheduler"
)

// TestTrainThetaWithInsufficientData tests that default theta is returned with <10 records
func TestTrainThetaWithInsufficientData(t *testing.T) {
	history := []db.TaskHistory{
		{
			TaskID:        "task-1",
			WorkerID:      "worker-1",
			Tau:           10.0,
			ActualRuntime: 12.0,
			CPUUsed:       2.0,
			LoadAtStart:   0.5,
		},
	}

	theta := TrainTheta(history)

	// Should return defaults
	expected := getDefaultTheta()
	if theta.Theta1 != expected.Theta1 || theta.Theta2 != expected.Theta2 ||
		theta.Theta3 != expected.Theta3 || theta.Theta4 != expected.Theta4 {
		t.Errorf("Expected default theta, got %+v", theta)
	}
}

// TestTrainThetaWithValidData tests theta training with sufficient valid data
func TestTrainThetaWithValidData(t *testing.T) {
	// Create synthetic data where runtime increases with load
	// Note: Due to the heuristic in buildRegressionMatrix, all features correlate with load
	// So the learned theta values will be distributed across multiple coefficients
	history := generateSyntheticHistory(50, 0.0, 0.0, 0.0, 0.2)

	theta := TrainTheta(history)

	// Due to multicollinearity (all features correlate with load),
	// the sum of coefficients should approximate the target
	// Allow for distributed weight across correlated features
	totalCoeff := theta.Theta1 + theta.Theta2 + theta.Theta3 + theta.Theta4

	// Total should be close to 0.2, but distributed
	if math.Abs(totalCoeff-0.2) > 0.15 {
		t.Errorf("Expected total coefficients ≈ 0.2, got %.3f (θ1=%.3f, θ2=%.3f, θ3=%.3f, θ4=%.3f)",
			totalCoeff, theta.Theta1, theta.Theta2, theta.Theta3, theta.Theta4)
	}
}

// TestTrainThetaWithCPUPattern tests learning CPU impact
func TestTrainThetaWithCPUPattern(t *testing.T) {
	// Pattern: runtime = tau * (1 + 0.3 * cpuRatio)
	// Due to heuristic where cpuRatio=load, this creates a load pattern
	history := generateSyntheticHistory(50, 0.3, 0.0, 0.0, 0.0)

	theta := TrainTheta(history)

	// Due to multicollinearity, the total coefficient should approximate 0.3
	totalCoeff := theta.Theta1 + theta.Theta2 + theta.Theta3 + theta.Theta4

	if math.Abs(totalCoeff-0.3) > 0.2 {
		t.Errorf("Expected total coefficients ≈ 0.3, got %.3f (θ1=%.3f, θ2=%.3f, θ3=%.3f, θ4=%.3f)",
			totalCoeff, theta.Theta1, theta.Theta2, theta.Theta3, theta.Theta4)
	}
}

// TestBuildRegressionMatrix tests the regression matrix construction
func TestBuildRegressionMatrix(t *testing.T) {
	history := []db.TaskHistory{
		{
			TaskID:        "task-1",
			Tau:           10.0,
			ActualRuntime: 12.0,
			CPUUsed:       2.0,
			MemUsed:       4.0,
			GPUUsed:       0.0,
			LoadAtStart:   0.5,
		},
		{
			TaskID:        "task-2",
			Tau:           20.0,
			ActualRuntime: 25.0,
			CPUUsed:       4.0,
			MemUsed:       8.0,
			GPUUsed:       1.0,
			LoadAtStart:   0.7,
		},
	}

	X, y, err := buildRegressionMatrix(history)

	if err != nil {
		t.Fatalf("buildRegressionMatrix failed: %v", err)
	}

	if len(X) != 2 {
		t.Errorf("Expected 2 data points, got %d", len(X))
	}

	if len(y) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(y))
	}

	// Check first feature vector has 4 dimensions
	if len(X[0]) != 4 {
		t.Errorf("Expected 4 features, got %d", len(X[0]))
	}

	// Check target calculation: (runtime / tau) - 1
	expectedY1 := (12.0 / 10.0) - 1.0 // = 0.2
	if math.Abs(y[0]-expectedY1) > 0.001 {
		t.Errorf("Expected y[0] = %.3f, got %.3f", expectedY1, y[0])
	}
}

// TestBuildRegressionMatrixWithInvalidData tests filtering of invalid records
func TestBuildRegressionMatrixWithInvalidData(t *testing.T) {
	history := []db.TaskHistory{
		{
			TaskID:        "task-1",
			Tau:           0.0, // Invalid: zero tau
			ActualRuntime: 12.0,
			CPUUsed:       2.0,
			LoadAtStart:   0.5,
		},
		{
			TaskID:        "task-2",
			Tau:           10.0,
			ActualRuntime: 0.0, // Invalid: zero runtime
			CPUUsed:       2.0,
			LoadAtStart:   0.5,
		},
		{
			TaskID:        "task-3",
			Tau:           10.0,
			ActualRuntime: 12.0,
			CPUUsed:       0.0,
			MemUsed:       0.0,
			GPUUsed:       0.0, // Invalid: no resources used
			LoadAtStart:   0.5,
		},
		{
			TaskID:        "task-4",
			Tau:           10.0,
			ActualRuntime: 120.0, // Target = 11.0 (too large, will be filtered)
			CPUUsed:       2.0,
			LoadAtStart:   0.5,
		},
		{
			TaskID:        "task-5",
			Tau:           10.0,
			ActualRuntime: 12.0, // Valid
			CPUUsed:       2.0,
			LoadAtStart:   0.5,
		},
	}

	X, _, err := buildRegressionMatrix(history)

	if err != nil {
		t.Fatalf("buildRegressionMatrix failed: %v", err)
	}

	// Only task-5 should remain
	if len(X) != 1 {
		t.Errorf("Expected 1 valid data point after filtering, got %d", len(X))
	}
}

// TestSolveLinearRegression tests the linear regression solver
func TestSolveLinearRegression(t *testing.T) {
	// Perfect linear relationship: y = 2*x1 + 3*x2 + 1*x3 + 0.5*x4
	X := [][]float64{
		{1.0, 0.0, 0.0, 0.0},
		{0.0, 1.0, 0.0, 0.0},
		{0.0, 0.0, 1.0, 0.0},
		{0.0, 0.0, 0.0, 1.0},
		{1.0, 1.0, 0.0, 0.0},
		{1.0, 0.0, 1.0, 0.0},
		{0.0, 1.0, 1.0, 0.0},
		{1.0, 1.0, 1.0, 1.0},
	}

	y := []float64{
		2.0, // 2*1
		3.0, // 3*1
		1.0, // 1*1
		0.5, // 0.5*1
		5.0, // 2*1 + 3*1
		3.0, // 2*1 + 1*1
		4.0, // 3*1 + 1*1
		6.5, // 2*1 + 3*1 + 1*1 + 0.5*1
	}

	theta, err := solveLinearRegression(X, y)

	if err != nil {
		t.Fatalf("solveLinearRegression failed: %v", err)
	}

	// Check if theta is close to [2.0, 3.0, 1.0, 0.5]
	expected := []float64{2.0, 3.0, 1.0, 0.5}
	for i := 0; i < 4; i++ {
		if math.Abs(theta[i]-expected[i]) > 0.1 {
			t.Errorf("Theta[%d]: expected %.3f, got %.3f", i, expected[i], theta[i])
		}
	}
}

// TestSolveLinearRegressionWithEmptyData tests error handling
func TestSolveLinearRegressionWithEmptyData(t *testing.T) {
	X := [][]float64{}
	y := []float64{}

	_, err := solveLinearRegression(X, y)

	if err == nil {
		t.Error("Expected error for empty dataset, got nil")
	}
}

// TestBoundTheta tests theta bounding
func TestBoundTheta(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected float64
	}{
		{"Valid value", 0.5, 0.5},
		{"Zero", 0.0, 0.0},
		{"Max valid", 2.0, 2.0},
		{"Negative", -0.5, 0.0},
		{"Too large", 3.0, 2.0},
		{"NaN", math.NaN(), 0.1},
		{"Inf", math.Inf(1), 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boundTheta(tt.value, "test")
			if math.IsNaN(tt.expected) {
				if !math.IsNaN(got) {
					t.Errorf("Expected NaN, got %.3f", got)
				}
			} else if math.Abs(got-tt.expected) > 0.001 {
				t.Errorf("boundTheta(%.3f) = %.3f, want %.3f", tt.value, got, tt.expected)
			}
		})
	}
}

// TestGetDefaultTheta tests default theta values
func TestGetDefaultTheta(t *testing.T) {
	theta := getDefaultTheta()

	// Check values are within valid range
	if theta.Theta1 < 0 || theta.Theta1 > 2.0 {
		t.Errorf("Default Theta1 out of range: %.3f", theta.Theta1)
	}
	if theta.Theta2 < 0 || theta.Theta2 > 2.0 {
		t.Errorf("Default Theta2 out of range: %.3f", theta.Theta2)
	}
	if theta.Theta3 < 0 || theta.Theta3 > 2.0 {
		t.Errorf("Default Theta3 out of range: %.3f", theta.Theta3)
	}
	if theta.Theta4 < 0 || theta.Theta4 > 2.0 {
		t.Errorf("Default Theta4 out of range: %.3f", theta.Theta4)
	}

	// GPU should have higher impact than CPU/Mem (per EDD paper)
	if theta.Theta3 <= theta.Theta1 || theta.Theta3 <= theta.Theta2 {
		t.Errorf("Expected Theta3 (GPU) > Theta1,2 (CPU,Mem), got θ1=%.3f, θ2=%.3f, θ3=%.3f",
			theta.Theta1, theta.Theta2, theta.Theta3)
	}
}

// TestComputeRSquared tests R² computation
func TestComputeRSquared(t *testing.T) {
	// Perfect fit: runtime = tau * (1 + 0.2 * load)
	history := generateSyntheticHistory(50, 0.0, 0.0, 0.0, 0.2)
	theta := scheduler.Theta{
		Theta1: 0.0,
		Theta2: 0.0,
		Theta3: 0.0,
		Theta4: 0.2,
	}

	r2 := ComputeRSquared(history, theta)

	// R² should be close to 1.0 for perfect fit
	if r2 < 0.9 {
		t.Errorf("Expected R² > 0.9 for perfect fit, got %.3f", r2)
	}
}

// TestComputeRSquaredWithPoorFit tests R² with bad model
func TestComputeRSquaredWithPoorFit(t *testing.T) {
	// Data has positive correlation with load (theta4=0.5)
	history := generateSyntheticHistory(50, 0.0, 0.0, 0.0, 0.5)

	// Use all zeros - predicting no deviation when there actually is deviation
	theta := scheduler.Theta{
		Theta1: 0.0,
		Theta2: 0.0,
		Theta3: 0.0,
		Theta4: 0.0, // Should be 0.5, but we predict 0
	}

	r2 := ComputeRSquared(history, theta)

	// R² should be 0 or negative for constant prediction when variance exists
	if r2 > 0.1 {
		t.Errorf("Expected R² ≈ 0 for zero model with non-zero data, got %.3f", r2)
	}
}

// TestComputeRSquaredWithEmptyHistory tests R² with no data
func TestComputeRSquaredWithEmptyHistory(t *testing.T) {
	history := []db.TaskHistory{}
	theta := getDefaultTheta()

	r2 := ComputeRSquared(history, theta)

	if r2 != 0.0 {
		t.Errorf("Expected R² = 0.0 for empty history, got %.3f", r2)
	}
}

// Helper function to generate synthetic task history with known patterns
func generateSyntheticHistory(count int, theta1, theta2, theta3, theta4 float64) []db.TaskHistory {
	history := make([]db.TaskHistory, count)
	now := time.Now()

	for i := 0; i < count; i++ {
		tau := 10.0 + float64(i%5) // Vary tau: 10, 11, 12, 13, 14

		// Vary load from 0.1 to 0.9
		load := 0.1 + float64(i%9)*0.1

		// For the simplified heuristic in buildRegressionMatrix:
		// - cpuRatio = load (if CPUUsed > 0)
		// - memRatio = load * (MemUsed / 8.0)
		// - gpuRatio = load * (GPUUsed / 1.0)

		// So to test specific theta values, we need to work backwards:
		// If we want pure load pattern (theta4), set CPU/Mem/GPU proportionally
		// If we want CPU pattern (theta1), rely on load since cpuRatio = load

		cpuUsed := 2.0
		memUsed := 4.0 // memRatio will be load * 0.5
		gpuUsed := 0.5 // gpuRatio will be load * 0.5

		// Compute effective ratios based on the heuristic
		cpuRatio := load
		memRatio := load * (memUsed / 8.0) // 4.0/8.0 = 0.5, so memRatio = load * 0.5
		gpuRatio := load * (gpuUsed / 1.0) // 0.5/1.0 = 0.5, so gpuRatio = load * 0.5

		// Compute actual runtime based on formula
		// runtime = tau * (1 + theta1*cpuRatio + theta2*memRatio + theta3*gpuRatio + theta4*load)
		deviation := theta1*cpuRatio + theta2*memRatio + theta3*gpuRatio + theta4*load
		actualRuntime := tau * (1.0 + deviation)

		history[i] = db.TaskHistory{
			TaskID:        fmt.Sprintf("task-%d", i),
			WorkerID:      fmt.Sprintf("worker-%d", i%3),
			Type:          "cpu-light",
			ArrivalTime:   now.Add(-time.Duration(count-i) * time.Minute),
			Tau:           tau,
			ActualRuntime: actualRuntime,
			CPUUsed:       cpuUsed,
			MemUsed:       memUsed,
			GPUUsed:       gpuUsed,
			LoadAtStart:   load,
			SLASuccess:    actualRuntime <= tau*2.0,
		}
	}

	return history
}
