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

package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"master/internal/config"

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
	client     *mongo.Client
	collection *mongo.Collection
}

// NewResultDB creates a new ResultDB instance
func NewResultDB(ctx context.Context, cfg *config.Config) (*ResultDB, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDBURI))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	collection := client.Database(cfg.MongoDBDatabase).Collection("RESULTS")

	return &ResultDB{
		client:     client,
		collection: collection,
	}, nil
}

// Close closes the database connection
func (rdb *ResultDB) Close(ctx context.Context) error {
	if rdb.client != nil {
		return rdb.client.Disconnect(ctx)
	}
	return nil
}

// CreateResult stores a task result with logs
func (rdb *ResultDB) CreateResult(ctx context.Context, result *TaskResult) error {
	result.CompletedAt = time.Now()

	_, err := rdb.collection.InsertOne(ctx, result)
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
