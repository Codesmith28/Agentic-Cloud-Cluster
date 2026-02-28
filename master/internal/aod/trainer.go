package aod

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"master/internal/db"
	"master/internal/scheduler"
)

// RunTraining executes one complete AOD training cycle.
//
// This function:
// 1. Fetches historical task and worker data from the database
// 2. Trains Theta parameters using linear regression
// 3. Builds affinity matrix using direct computation (SpeedAdvantage + SLAReliability)
// 4. Builds penalty vector using direct computation
// 5. Saves the optimized parameters to persistent storage (MongoDB and/or JSON file) for RTS to load
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - historyDB: Database connection for fetching historical data
//   - paramsOutputPath: File path to save the optimized GAParams JSON
//   - paramsStore: Optional persistent RTS params store (MongoDB)
//
// Returns: error if any step fails
func RunTraining(ctx context.Context, historyDB *db.HistoryDB, paramsOutputPath string, paramsStore scheduler.GAParamsStore) error {
	log.Println("🧬 Starting AOD training cycle...")
	startTime := time.Now()

	// Step 1: Fetch historical data (last 24 hours)
	until := time.Now()
	since := until.Add(-24 * time.Hour)

	log.Printf("📊 Fetching task history from %s to %s", since.Format(time.RFC3339), until.Format(time.RFC3339))
	history, err := historyDB.GetTaskHistory(ctx, since, until)
	if err != nil {
		return fmt.Errorf("fetch task history: %w", err)
	}

	log.Printf("📊 Fetching worker stats from %s to %s", since.Format(time.RFC3339), until.Format(time.RFC3339))
	workerStats, err := historyDB.GetWorkerStats(ctx, since, until)
	if err != nil {
		return fmt.Errorf("fetch worker stats: %w", err)
	}

	log.Printf("✓ Retrieved %d task history records and %d worker stats", len(history), len(workerStats))

	// Step 2: Check if we have sufficient data
	minDataPoints := 2 // Minimum tasks required for meaningful training
	if len(history) < minDataPoints {
		log.Printf("⚠️  Insufficient data (%d tasks < %d required), using default parameters", len(history), minDataPoints)
		return saveDefaultParams(ctx, paramsOutputPath, paramsStore)
	}

	// Step 3: Train Theta using linear regression
	log.Println("🔧 Training Theta parameters using linear regression...")
	theta := TrainTheta(history)
	log.Printf("✓ Theta trained: θ₁=%.4f, θ₂=%.4f, θ₃=%.4f, θ₄=%.4f",
		theta.Theta1, theta.Theta2, theta.Theta3, theta.Theta4)

	// Step 4: Build affinity matrix using direct computation (NO GA evolution, NO weights)
	log.Println("🔧 Building affinity matrix using direct computation...")
	affinityMatrix := BuildAffinityMatrix(history)
	log.Printf("✓ Affinity matrix built with %d task types", len(affinityMatrix))

	// Step 5: Build penalty vector using direct computation
	log.Println("🔧 Building penalty vector...")
	penaltyVector := BuildPenaltyVector(workerStats)
	log.Printf("✓ Penalty vector built for %d workers", len(penaltyVector))

	// Step 6: Create GAParams structure (simplified - no weights)
	params := scheduler.GAParams{
		Theta:          theta,
		Risk:           defaultRisk(), // Use default risk weights (alpha, beta)
		AffinityMatrix: affinityMatrix,
		PenaltyVector:  penaltyVector,
	}

	// Step 6: Persist parameters (MongoDB + JSON fallback path)
	if err := saveParams(ctx, params, paramsOutputPath, paramsStore); err != nil {
		return fmt.Errorf("save params: %w", err)
	}

	elapsed := time.Since(startTime)
	log.Printf("✅ AOD training completed in %s, parameters saved to %s", elapsed, paramsOutputPath)

	return nil
}

// saveParams writes GAParams to a JSON file
func saveParams(ctx context.Context, params scheduler.GAParams, filePath string, paramsStore scheduler.GAParamsStore) error {
	if filePath == "" && paramsStore == nil {
		return fmt.Errorf("no params persistence destination configured")
	}

	var fileErr error
	var storeErr error

	if filePath != "" {
		data, err := json.MarshalIndent(params, "", "  ")
		if err != nil {
			fileErr = fmt.Errorf("marshal json: %w", err)
		} else {
			dir := filepath.Dir(filePath)
			if dir != "." && dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					fileErr = fmt.Errorf("create params directory: %w", err)
				}
			}

			if fileErr == nil {
				if err := os.WriteFile(filePath, data, 0644); err != nil {
					fileErr = fmt.Errorf("write file: %w", err)
				} else {
					log.Printf("✓ AOD parameters saved to %s", filePath)
				}
			}
		}
	}

	if paramsStore != nil {
		if err := paramsStore.SaveGAParams(ctx, &params); err != nil {
			storeErr = fmt.Errorf("save to mongodb: %w", err)
		} else {
			log.Printf("✓ AOD parameters saved to MongoDB collection RTS_WEIGHTS")
		}
	}

	// If at least one destination succeeded, continue and only warn for the other.
	fileSucceeded := filePath == "" || fileErr == nil
	storeSucceeded := paramsStore == nil || storeErr == nil
	if fileSucceeded && storeSucceeded {
		return nil
	}

	if fileErr != nil && paramsStore != nil && storeErr == nil {
		log.Printf("⚠️  AOD: JSON persistence failed, MongoDB save succeeded (%v)", fileErr)
		return nil
	}
	if storeErr != nil && filePath != "" && fileErr == nil {
		log.Printf("⚠️  AOD: MongoDB persistence failed, JSON save succeeded (%v)", storeErr)
		return nil
	}

	if fileErr != nil && storeErr != nil {
		return fmt.Errorf("persist params failed (file: %v; mongo: %v)", fileErr, storeErr)
	}
	if fileErr != nil {
		return fileErr
	}
	return storeErr
}

// saveDefaultParams writes default GAParams to JSON file
func saveDefaultParams(ctx context.Context, filePath string, paramsStore scheduler.GAParamsStore) error {
	params := scheduler.GAParams{
		Theta:          defaultTheta(),
		Risk:           defaultRisk(),
		AffinityMatrix: make(map[string]map[string]float64),
		PenaltyVector:  make(map[string]float64),
	}
	return saveParams(ctx, params, filePath, paramsStore)
}

// Helper functions for default values

func defaultTheta() scheduler.Theta {
	return scheduler.Theta{
		Theta1: 0.1,
		Theta2: 0.1,
		Theta3: 0.3,
		Theta4: 0.2,
	}
}

func defaultRisk() scheduler.Risk {
	return scheduler.Risk{
		Alpha: 10.0,
		Beta:  1.0,
	}
}
