package db

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TaskResult represents a task result with logs stored in MongoDB
type TaskResult struct {
	TaskID      string    `bson:"task_id"`
	WorkerID    string    `bson:"worker_id"`
	Status      string    `bson:"status"` // "success", "failed"
	Logs        string    `bson:"logs"`
	CompletedAt time.Time `bson:"completed_at"`
	SLASuccess  bool      `bson:"sla_success"` // Task 2.5: Whether task met its deadline
}

// ResultDB handles task results operations
type ResultDB struct {
	collection *mongo.Collection
}

// NewResultDB creates a new ResultDB instance from MongoStore
func NewResultDB(store *MongoStore) *ResultDB {
	return &ResultDB{
		collection: store.Collection("RESULTS"),
	}
}

// Close closes the database connection (no-op; managed by MongoStore)
func (rdb *ResultDB) Close(ctx context.Context) error {
	return nil
}

// CreateResult stores a task result with logs
func (rdb *ResultDB) CreateResult(ctx context.Context, result *TaskResult) error {
	result.CompletedAt = time.Now()

	_, err := rdb.collection.ReplaceOne(
		ctx,
		bson.M{"task_id": result.TaskID},
		result,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Error creating result: %v", err)
		return err
	}

	return nil
}

// GetResult retrieves a task result by task ID
func (rdb *ResultDB) GetResult(ctx context.Context, taskID string) (*TaskResult, error) {
	var result TaskResult
	err := rdb.collection.FindOne(ctx, bson.M{"task_id": taskID}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		log.Printf("Error getting result: %v", err)
		return nil, err
	}

	return &result, nil
}

// GetResultsByWorker retrieves all results for a specific worker
func (rdb *ResultDB) GetResultsByWorker(ctx context.Context, workerID string) ([]TaskResult, error) {
	cursor, err := rdb.collection.Find(ctx, bson.M{"worker_id": workerID})
	if err != nil {
		log.Printf("Error querying results by worker: %v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []TaskResult
	if err = cursor.All(ctx, &results); err != nil {
		log.Printf("Error decoding results: %v", err)
		return nil, err
	}

	return results, nil
}
