// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
//   - postSave: Optional callback for additional persistence/versioning
//
// Returns: error if all persistence destinations fail.
func RunTraining(
	ctx context.Context,
	historyDB *db.HistoryDB,
	paramsOutputPath string,
	paramsStore scheduler.GAParamsStore,
	postSave func(context.Context, scheduler.GAParams) error,
) error {
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
		return saveDefaultParams(ctx, paramsOutputPath, paramsStore, postSave)
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

	// Step 7: Persist parameters
	if err := saveParams(ctx, params, paramsOutputPath, paramsStore, postSave); err != nil {
		return fmt.Errorf("save params: %w", err)
	}

	elapsed := time.Since(startTime)
	log.Printf("✅ AOD training completed in %s, parameters saved to %s", elapsed, paramsOutputPath)

	return nil
}

// saveParams writes GAParams to configured persistent destinations.
func saveParams(
	ctx context.Context,
	params scheduler.GAParams,
	filePath string,
	paramsStore scheduler.GAParamsStore,
	postSave func(context.Context, scheduler.GAParams) error,
) error {
	if filePath == "" && paramsStore == nil && postSave == nil {
		return fmt.Errorf("no params persistence destination configured")
	}

	attempted := 0
	succeeded := 0

	var fileErr error
	if filePath != "" {
		attempted++
		data, err := json.MarshalIndent(params, "", "  ")
		if err != nil {
			fileErr = fmt.Errorf("marshal json: %w", err)
		} else if err := writeFileAtomic(filePath, data, 0644); err != nil {
			fileErr = fmt.Errorf("write file: %w", err)
		} else {
			succeeded++
			log.Printf("✓ AOD parameters saved to %s", filePath)
		}
	}

	var storeErr error
	if paramsStore != nil {
		attempted++
		if err := paramsStore.SaveGAParams(ctx, &params); err != nil {
			storeErr = fmt.Errorf("save to mongodb: %w", err)
		} else {
			succeeded++
			log.Printf("✓ AOD parameters saved to MongoDB collection RTS_WEIGHTS")
		}
	}

	var callbackErr error
	if postSave != nil {
		attempted++
		if err := postSave(ctx, params); err != nil {
			callbackErr = fmt.Errorf("post-save hook failed: %w", err)
		} else {
			succeeded++
		}
	}

	if attempted == 0 {
		return fmt.Errorf("no params persistence destination configured")
	}
	if succeeded == attempted {
		return nil
	}
	if succeeded > 0 {
		if fileErr != nil {
			log.Printf("⚠️  AOD: JSON persistence failed (%v)", fileErr)
		}
		if storeErr != nil {
			log.Printf("⚠️  AOD: MongoDB persistence failed (%v)", storeErr)
		}
		if callbackErr != nil {
			log.Printf("⚠️  AOD: post-save callback failed (%v)", callbackErr)
		}
		return nil
	}

	return fmt.Errorf("persist params failed (file: %v; mongo: %v; callback: %v)", fileErr, storeErr, callbackErr)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create params directory: %w", err)
		}
	}

	tempFile, err := os.CreateTemp(dir, "ga_params_*.tmp")
	if err != nil {
		return fmt.Errorf("create temp params file: %w", err)
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// saveDefaultParams writes default GAParams to configured persistence backends.
func saveDefaultParams(
	ctx context.Context,
	filePath string,
	paramsStore scheduler.GAParamsStore,
	postSave func(context.Context, scheduler.GAParams) error,
) error {
	params := scheduler.GAParams{
		Theta:          defaultTheta(),
		Risk:           defaultRisk(),
		AffinityMatrix: make(map[string]map[string]float64),
		PenaltyVector:  make(map[string]float64),
	}
	return saveParams(ctx, params, filePath, paramsStore, postSave)
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
