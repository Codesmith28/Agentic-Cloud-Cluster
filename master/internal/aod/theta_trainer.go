package aod

import (
	"fmt"
	"log"
	"math"

	"master/internal/db"
	"master/internal/scheduler"

	"gonum.org/v1/gonum/mat"
)

// TrainTheta trains the Theta parameters using linear regression on historical task data.
// It learns how resource ratios (CPU, Memory, GPU) and worker load affect execution time.
//
// The model: E_hat = tau * (1 + θ1*(C/Cavail) + θ2*(M/Mavail) + θ3*(G/Gavail) + θ4*Load)
// Rearranged: (E_hat/tau - 1) = θ1*(C/Cavail) + θ2*(M/Mavail) + θ3*(G/Gavail) + θ4*Load
//
// Parameters:
//   - history: Historical task execution records
//
// Returns:
//   - Trained Theta parameters (θ1, θ2, θ3, θ4)
//   - Default values if insufficient data
func TrainTheta(history []db.TaskHistory) scheduler.Theta {
	// Need at least 10 data points for meaningful regression
	if len(history) < 10 {
		log.Printf("⚠️  Insufficient data for Theta training (%d records), using defaults", len(history))
		return getDefaultTheta()
	}

	// Build regression matrix (X) and target vector (y)
	X, y, err := buildRegressionMatrix(history)
	if err != nil {
		log.Printf("⚠️  Failed to build regression matrix: %v, using defaults", err)
		return getDefaultTheta()
	}

	if len(X) < 10 {
		log.Printf("⚠️  Insufficient valid data points (%d), using defaults", len(X))
		return getDefaultTheta()
	}

	// Solve linear regression: θ = (X^T X)^(-1) X^T y
	thetaVec, err := solveLinearRegression(X, y)
	if err != nil {
		log.Printf("⚠️  Linear regression failed: %v, using defaults", err)
		return getDefaultTheta()
	}

	// Validate and bound theta values
	theta := scheduler.Theta{
		Theta1: boundTheta(thetaVec[0], "θ1"),
		Theta2: boundTheta(thetaVec[1], "θ2"),
		Theta3: boundTheta(thetaVec[2], "θ3"),
		Theta4: boundTheta(thetaVec[3], "θ4"),
	}

	log.Printf("✓ Theta trained successfully: θ1=%.3f, θ2=%.3f, θ3=%.3f, θ4=%.3f",
		theta.Theta1, theta.Theta2, theta.Theta3, theta.Theta4)

	return theta
}

// buildRegressionMatrix constructs the feature matrix X and target vector y from task history.
//
// For each task:
//   - Features X[i] = [CPU_ratio, Mem_ratio, GPU_ratio, Load_at_start]
//   - Target y[i] = (ActualRuntime / Tau) - 1.0
//
// This formulation learns how resource pressure and load affect runtime relative to baseline.
func buildRegressionMatrix(history []db.TaskHistory) ([][]float64, []float64, error) {
	var X [][]float64
	var y []float64

	for _, record := range history {
		// Skip invalid records
		if record.Tau <= 0 || record.ActualRuntime <= 0 {
			continue
		}

		// Skip records with invalid resource usage (need to know worker capacity)
		if record.CPUUsed <= 0 && record.MemUsed <= 0 && record.GPUUsed <= 0 {
			continue
		}

		// Compute resource ratios
		// Note: We need available capacity from worker state at task start
		// For now, we estimate from used resources and load
		// Better approach would be to store worker capacity in TaskHistory

		// Estimate available resources from used and load
		// If load was L and we used R, total capacity ≈ R / L (rough approximation)
		// For more accurate results, need to store worker capacity in history

		cpuRatio := 0.0
		memRatio := 0.0
		gpuRatio := 0.0

		// Simple heuristic: if resource was used, assume it contributed to load
		// Use load as proxy for resource pressure
		if record.CPUUsed > 0 {
			cpuRatio = record.LoadAtStart // Simplified: load reflects CPU pressure
		}
		if record.MemUsed > 0 {
			memRatio = record.LoadAtStart * (record.MemUsed / 8.0) // Normalize by typical 8GB
		}
		if record.GPUUsed > 0 {
			gpuRatio = record.LoadAtStart * (record.GPUUsed / 1.0) // Normalize by 1 GPU
		}

		// Load at task start
		load := record.LoadAtStart

		// Target: (actual_runtime / tau) - 1
		// This is the deviation from baseline
		target := (record.ActualRuntime / record.Tau) - 1.0

		// Skip extreme outliers (likely data errors)
		if target < -0.9 || target > 5.0 {
			continue
		}

		// Add to dataset
		features := []float64{cpuRatio, memRatio, gpuRatio, load}
		X = append(X, features)
		y = append(y, target)
	}

	if len(X) == 0 {
		return nil, nil, fmt.Errorf("no valid data points after filtering")
	}

	return X, y, nil
}

// solveLinearRegression solves the linear regression problem using the normal equation.
//
// Given:
//   - X: n x 4 feature matrix
//   - y: n x 1 target vector
//
// Computes: θ = (X^T X)^(-1) X^T y
//
// Returns the 4 theta coefficients [θ1, θ2, θ3, θ4]
func solveLinearRegression(X [][]float64, y []float64) ([]float64, error) {
	n := len(X)
	if n == 0 {
		return nil, fmt.Errorf("empty dataset")
	}

	// Convert to gonum matrices
	xMat := mat.NewDense(n, 4, nil)
	for i := 0; i < n; i++ {
		xMat.SetRow(i, X[i])
	}

	yVec := mat.NewVecDense(n, y)

	// Compute X^T X
	var xtx mat.Dense
	xtx.Mul(xMat.T(), xMat)

	// Check if matrix is singular (determinant near zero)
	det := mat.Det(&xtx)
	if math.Abs(det) < 1e-10 {
		log.Printf("⚠️  X^T X is near-singular (det=%.2e), adding regularization", det)
		// Add small regularization to diagonal (Ridge regression)
		for i := 0; i < 4; i++ {
			xtx.Set(i, i, xtx.At(i, i)+0.01)
		}
	}

	// Compute (X^T X)^(-1)
	var xtxInv mat.Dense
	err := xtxInv.Inverse(&xtx)
	if err != nil {
		return nil, fmt.Errorf("failed to invert X^T X: %v", err)
	}

	// Compute X^T y
	var xty mat.VecDense
	xty.MulVec(xMat.T(), yVec)

	// Compute θ = (X^T X)^(-1) X^T y
	var theta mat.VecDense
	theta.MulVec(&xtxInv, &xty)

	// Extract theta values
	result := make([]float64, 4)
	for i := 0; i < 4; i++ {
		result[i] = theta.AtVec(i)
	}

	return result, nil
}

// boundTheta ensures theta values are within reasonable bounds.
// Theta values outside [0, 2.0] are clipped and logged as warnings.
func boundTheta(value float64, name string) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		log.Printf("⚠️  %s is invalid (%.3f), using 0.1", name, value)
		return 0.1
	}

	if value < 0 {
		log.Printf("⚠️  %s is negative (%.3f), clipping to 0.0", name, value)
		return 0.0
	}

	if value > 2.0 {
		log.Printf("⚠️  %s is too large (%.3f), clipping to 2.0", name, value)
		return 2.0
	}

	return value
}

// getDefaultTheta returns sensible default Theta values based on EDD paper recommendations.
func getDefaultTheta() scheduler.Theta {
	return scheduler.Theta{
		Theta1: 0.1, // CPU impact: 10% per unit ratio
		Theta2: 0.1, // Memory impact: 10% per unit ratio
		Theta3: 0.3, // GPU impact: 30% per unit ratio (higher due to GPU importance)
		Theta4: 0.2, // Load impact: 20% per unit load
	}
}

// ComputeRSquared computes the R² (coefficient of determination) for model evaluation.
// R² ∈ [0, 1] where 1 means perfect fit, 0 means model is no better than mean.
//
// Used for validating theta quality after training.
func ComputeRSquared(history []db.TaskHistory, theta scheduler.Theta) float64 {
	if len(history) == 0 {
		return 0.0
	}

	// Build predictions and actuals
	var predictions []float64
	var actuals []float64

	for _, record := range history {
		if record.Tau <= 0 || record.ActualRuntime <= 0 {
			continue
		}

		// Compute predicted deviation
		cpuRatio := record.LoadAtStart
		memRatio := record.LoadAtStart * (record.MemUsed / 8.0)
		gpuRatio := record.LoadAtStart * (record.GPUUsed / 1.0)
		load := record.LoadAtStart

		predicted := theta.Theta1*cpuRatio + theta.Theta2*memRatio +
			theta.Theta3*gpuRatio + theta.Theta4*load

		actual := (record.ActualRuntime / record.Tau) - 1.0

		predictions = append(predictions, predicted)
		actuals = append(actuals, actual)
	}

	if len(actuals) == 0 {
		return 0.0
	}

	// Compute mean of actuals
	mean := 0.0
	for _, a := range actuals {
		mean += a
	}
	mean /= float64(len(actuals))

	// Compute SS_tot (total sum of squares)
	ssTot := 0.0
	for _, a := range actuals {
		diff := a - mean
		ssTot += diff * diff
	}

	// Compute SS_res (residual sum of squares)
	ssRes := 0.0
	for i := range actuals {
		diff := actuals[i] - predictions[i]
		ssRes += diff * diff
	}

	// R² = 1 - (SS_res / SS_tot)
	if ssTot == 0 {
		return 0.0
	}

	r2 := 1.0 - (ssRes / ssTot)
	return r2
}
