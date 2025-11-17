package db

import (
	"context"
	"fmt"
	"time"

	"master/internal/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TaskHistory represents enriched historical task execution data
// This combines data from TASKS, ASSIGNMENTS, and RESULTS collections
// Used for GA training and telemetry analysis
type TaskHistory struct {
	TaskID        string    `bson:"task_id"`
	WorkerID      string    `bson:"worker_id"`
	Type          string    `bson:"type"`           // Must be one of: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed
	ArrivalTime   time.Time `bson:"arrival_time"`   // When task was submitted
	Deadline      time.Time `bson:"deadline"`       // Computed SLA deadline
	ActualStart   time.Time `bson:"actual_start"`   // When task started executing
	ActualFinish  time.Time `bson:"actual_finish"`  // When task completed
	ActualRuntime float64   `bson:"actual_runtime"` // Duration in seconds
	SLASuccess    bool      `bson:"sla_success"`    // Whether task met SLA deadline
	CPUUsed       float64   `bson:"cpu_used"`       // CPU cores allocated
	MemUsed       float64   `bson:"mem_used"`       // Memory GB allocated
	GPUUsed       float64   `bson:"gpu_used"`       // GPU cores allocated
	StorageUsed   float64   `bson:"storage_used"`   // Storage GB allocated
	LoadAtStart   float64   `bson:"load_at_start"`  // Worker load when task started
	Tau           float64   `bson:"tau"`            // Expected runtime (baseline)
	SLAMultiplier float64   `bson:"sla_multiplier"` // k value used for deadline
}

// WorkerStats represents aggregated statistics for a worker over a time period
// Used for penalty vector computation and worker performance analysis
type WorkerStats struct {
	WorkerID      string    `bson:"worker_id"`
	TasksRun      int       `bson:"tasks_run"`      // Total tasks executed
	SLAViolations int       `bson:"sla_violations"` // Number of missed deadlines
	TotalRuntime  float64   `bson:"total_runtime"`  // Sum of all task runtimes (seconds)
	CPUUsedTotal  float64   `bson:"cpu_used_total"` // Sum of CPU-seconds
	MemUsedTotal  float64   `bson:"mem_used_total"` // Sum of Memory-GB-seconds
	GPUUsedTotal  float64   `bson:"gpu_used_total"` // Sum of GPU-seconds
	OverloadTime  float64   `bson:"overload_time"`  // Time spent in high-load state (seconds)
	TotalTime     float64   `bson:"total_time"`     // Total observation period (seconds)
	AvgLoad       float64   `bson:"avg_load"`       // Average normalized load
	PeriodStart   time.Time `bson:"period_start"`   // Start of observation period
	PeriodEnd     time.Time `bson:"period_end"`     // End of observation period
}

// HistoryDB handles historical telemetry data operations
type HistoryDB struct {
	client            *mongo.Client
	tasksCollection   *mongo.Collection
	assignCollection  *mongo.Collection
	resultsCollection *mongo.Collection
}

// NewHistoryDB creates a new HistoryDB instance
func NewHistoryDB(ctx context.Context, cfg *config.Config) (*HistoryDB, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDBURI))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	database := client.Database(cfg.MongoDBDatabase)

	return &HistoryDB{
		client:            client,
		tasksCollection:   database.Collection("TASKS"),
		assignCollection:  database.Collection("ASSIGNMENTS"),
		resultsCollection: database.Collection("RESULTS"),
	}, nil
}

// Close closes the database connection
func (db *HistoryDB) Close(ctx context.Context) error {
	if db.client != nil {
		return db.client.Disconnect(ctx)
	}
	return nil
}

// GetTaskHistory retrieves enriched task history by joining TASKS, ASSIGNMENTS, and RESULTS
// Filters tasks completed between 'since' and 'until' timestamps
// Returns only tasks with valid task types (one of the 6 standardized types)
func (db *HistoryDB) GetTaskHistory(ctx context.Context, since time.Time, until time.Time) ([]TaskHistory, error) {
	// MongoDB aggregation pipeline to join collections
	pipeline := mongo.Pipeline{
		// Stage 1: Match tasks completed in the time range
		{
			{Key: "$match", Value: bson.D{
				{Key: "completed_at", Value: bson.D{
					{Key: "$gte", Value: since},
					{Key: "$lte", Value: until},
				}},
				{Key: "status", Value: bson.D{
					{Key: "$in", Value: bson.A{"completed", "failed"}},
				}},
			}},
		},

		// Stage 2: Join with ASSIGNMENTS to get worker_id and load_at_start
		{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "ASSIGNMENTS"},
				{Key: "localField", Value: "task_id"},
				{Key: "foreignField", Value: "task_id"},
				{Key: "as", Value: "assignment"},
			}},
		},
		{
			{Key: "$unwind", Value: bson.D{
				{Key: "path", Value: "$assignment"},
				{Key: "preserveNullAndEmptyArrays", Value: false}, // Only keep tasks with assignments
			}},
		},

		// Stage 3: Join with RESULTS to get actual completion status
		{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "RESULTS"},
				{Key: "localField", Value: "task_id"},
				{Key: "foreignField", Value: "task_id"},
				{Key: "as", Value: "result"},
			}},
		},
		{
			{Key: "$unwind", Value: bson.D{
				{Key: "path", Value: "$result"},
				{Key: "preserveNullAndEmptyArrays", Value: true}, // Keep tasks without results (failed to report)
			}},
		},

		// Stage 4: Project fields into TaskHistory structure
		{
			{Key: "$project", Value: bson.D{
				{Key: "task_id", Value: "$task_id"},
				{Key: "worker_id", Value: "$assignment.worker_id"},
				{Key: "type", Value: "$task_type"},
				{Key: "arrival_time", Value: "$created_at"},
				{Key: "deadline", Value: bson.D{
					{Key: "$add", Value: bson.A{
						"$created_at",
						bson.D{{Key: "$multiply", Value: bson.A{
							"$sla_multiplier",
							"$tau",
							1000, // Convert seconds to milliseconds
						}}},
					}},
				}},
				{Key: "actual_start", Value: "$started_at"},
				{Key: "actual_finish", Value: "$completed_at"},
				{Key: "actual_runtime", Value: bson.D{
					{Key: "$divide", Value: bson.A{
						bson.D{{Key: "$subtract", Value: bson.A{"$completed_at", "$started_at"}}},
						1000, // Convert milliseconds to seconds
					}},
				}},
				{Key: "sla_success", Value: bson.D{
					{Key: "$lte", Value: bson.A{
						"$completed_at",
						bson.D{{Key: "$add", Value: bson.A{
							"$created_at",
							bson.D{{Key: "$multiply", Value: bson.A{
								"$sla_multiplier",
								"$tau",
								1000,
							}}},
						}}},
					}},
				}},
				{Key: "cpu_used", Value: "$req_cpu"},
				{Key: "mem_used", Value: "$req_memory"},
				{Key: "gpu_used", Value: "$req_gpu"},
				{Key: "storage_used", Value: "$req_storage"},
				{Key: "load_at_start", Value: bson.D{
					{Key: "$ifNull", Value: bson.A{"$assignment.load_at_start", 0.0}},
				}},
				{Key: "tau", Value: bson.D{
					{Key: "$ifNull", Value: bson.A{"$tau", 10.0}}, // Default tau if not set
				}},
				{Key: "sla_multiplier", Value: bson.D{
					{Key: "$ifNull", Value: bson.A{"$sla_multiplier", 2.0}}, // Default k=2.0
				}},
			}},
		},

		// Stage 5: Filter only valid task types (exclude empty or invalid types)
		{
			{Key: "$match", Value: bson.D{
				{Key: "type", Value: bson.D{
					{Key: "$in", Value: bson.A{
						"cpu-light", "cpu-heavy", "memory-heavy",
						"gpu-inference", "gpu-training", "mixed",
					}},
				}},
			}},
		},

		// Stage 6: Sort by arrival time
		{
			{Key: "$sort", Value: bson.D{
				{Key: "arrival_time", Value: 1},
			}},
		},
	}

	cursor, err := db.tasksCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate task history: %w", err)
	}
	defer cursor.Close(ctx)

	var history []TaskHistory
	if err := cursor.All(ctx, &history); err != nil {
		return nil, fmt.Errorf("decode task history: %w", err)
	}

	return history, nil
}

// GetWorkerStats computes aggregated statistics for each worker over a time period
// This data is used for penalty vector computation in the GA module
func (db *HistoryDB) GetWorkerStats(ctx context.Context, since time.Time, until time.Time) ([]WorkerStats, error) {
	// First, get task history
	history, err := db.GetTaskHistory(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get task history: %w", err)
	}

	// Aggregate statistics per worker
	statsMap := make(map[string]*WorkerStats)

	for _, task := range history {
		if _, exists := statsMap[task.WorkerID]; !exists {
			statsMap[task.WorkerID] = &WorkerStats{
				WorkerID:      task.WorkerID,
				TasksRun:      0,
				SLAViolations: 0,
				TotalRuntime:  0,
				CPUUsedTotal:  0,
				MemUsedTotal:  0,
				GPUUsedTotal:  0,
				OverloadTime:  0,
				TotalTime:     until.Sub(since).Seconds(),
				AvgLoad:       0,
				PeriodStart:   since,
				PeriodEnd:     until,
			}
		}

		stats := statsMap[task.WorkerID]
		stats.TasksRun++

		if !task.SLASuccess {
			stats.SLAViolations++
		}

		stats.TotalRuntime += task.ActualRuntime
		stats.CPUUsedTotal += task.CPUUsed * task.ActualRuntime
		stats.MemUsedTotal += task.MemUsed * task.ActualRuntime
		stats.GPUUsedTotal += task.GPUUsed * task.ActualRuntime

		// Count overload time (when load > 0.8)
		if task.LoadAtStart > 0.8 {
			stats.OverloadTime += task.ActualRuntime
		}

		// Running average of load
		stats.AvgLoad += task.LoadAtStart
	}

	// Compute final averages
	for _, stats := range statsMap {
		if stats.TasksRun > 0 {
			stats.AvgLoad /= float64(stats.TasksRun)
		}
	}

	// Convert map to slice
	result := make([]WorkerStats, 0, len(statsMap))
	for _, stats := range statsMap {
		result = append(result, *stats)
	}

	return result, nil
}

// GetTaskHistoryByType retrieves task history filtered by task type
// Useful for per-type analysis and tau computation
func (db *HistoryDB) GetTaskHistoryByType(ctx context.Context, taskType string, since time.Time, until time.Time) ([]TaskHistory, error) {
	// Validate task type
	validTypes := map[string]bool{
		"cpu-light": true, "cpu-heavy": true, "memory-heavy": true,
		"gpu-inference": true, "gpu-training": true, "mixed": true,
	}

	if !validTypes[taskType] {
		return nil, fmt.Errorf("invalid task type: %s (must be one of: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed)", taskType)
	}

	// Get all history first
	allHistory, err := db.GetTaskHistory(ctx, since, until)
	if err != nil {
		return nil, err
	}

	// Filter by type
	filtered := make([]TaskHistory, 0)
	for _, task := range allHistory {
		if task.Type == taskType {
			filtered = append(filtered, task)
		}
	}

	return filtered, nil
}

// GetWorkerStatsForWorker retrieves statistics for a specific worker
func (db *HistoryDB) GetWorkerStatsForWorker(ctx context.Context, workerID string, since time.Time, until time.Time) (*WorkerStats, error) {
	allStats, err := db.GetWorkerStats(ctx, since, until)
	if err != nil {
		return nil, err
	}

	for _, stats := range allStats {
		if stats.WorkerID == workerID {
			return &stats, nil
		}
	}

	// Worker has no tasks in this period
	return &WorkerStats{
		WorkerID:      workerID,
		TasksRun:      0,
		SLAViolations: 0,
		TotalRuntime:  0,
		CPUUsedTotal:  0,
		MemUsedTotal:  0,
		GPUUsedTotal:  0,
		OverloadTime:  0,
		TotalTime:     until.Sub(since).Seconds(),
		AvgLoad:       0,
		PeriodStart:   since,
		PeriodEnd:     until,
	}, nil
}

// GetSLASuccessRate computes the overall SLA success rate for a time period
func (db *HistoryDB) GetSLASuccessRate(ctx context.Context, since time.Time, until time.Time) (float64, error) {
	history, err := db.GetTaskHistory(ctx, since, until)
	if err != nil {
		return 0, err
	}

	if len(history) == 0 {
		return 1.0, nil // No tasks = 100% success (no violations)
	}

	successCount := 0
	for _, task := range history {
		if task.SLASuccess {
			successCount++
		}
	}

	return float64(successCount) / float64(len(history)), nil
}

// GetSLASuccessRateByType computes SLA success rate for a specific task type
func (db *HistoryDB) GetSLASuccessRateByType(ctx context.Context, taskType string, since time.Time, until time.Time) (float64, error) {
	history, err := db.GetTaskHistoryByType(ctx, taskType, since, until)
	if err != nil {
		return 0, err
	}

	if len(history) == 0 {
		return 1.0, nil
	}

	successCount := 0
	for _, task := range history {
		if task.SLASuccess {
			successCount++
		}
	}

	return float64(successCount) / float64(len(history)), nil
}
