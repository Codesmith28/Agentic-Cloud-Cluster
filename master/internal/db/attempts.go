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
	"time"

	"master/internal/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	AttemptStatusAssigned  = "assigned"
	AttemptStatusRunning   = "running"
	AttemptStatusCompleted = "completed"
	AttemptStatusFailed    = "failed"
	AttemptStatusCancelled = "cancelled"
	AttemptStatusLost      = "lost"
	AttemptStatusStale     = "stale"
)

// TaskAttempt stores the lifecycle of a single execution attempt for a logical task.
type TaskAttempt struct {
	AttemptID      string    `bson:"attempt_id"`
	TaskID         string    `bson:"task_id"`
	WorkerID       string    `bson:"worker_id"`
	AttemptNo      int32     `bson:"attempt_no"`
	Status         string    `bson:"status"`
	FailureReason  string    `bson:"failure_reason,omitempty"`
	LoadAtStart    float64   `bson:"load_at_start,omitempty"`
	AssignedAt     time.Time `bson:"assigned_at"`
	LastHeartbeat  int64     `bson:"last_heartbeat,omitempty"`
	CompletedAt    time.Time `bson:"completed_at,omitempty"`
	ResultStatus   string    `bson:"result_status,omitempty"`
	Logs           string    `bson:"logs,omitempty"`
	ResultLocation string    `bson:"result_location,omitempty"`
	OutputFiles    []string  `bson:"output_files,omitempty"`
}

// AttemptDB handles task attempt persistence.
type AttemptDB struct {
	client     *mongo.Client
	collection *mongo.Collection
}

func NewAttemptDB(ctx context.Context, cfg *config.Config) (*AttemptDB, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDBURI))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	collection := client.Database(cfg.MongoDBDatabase).Collection("ATTEMPTS")
	return &AttemptDB{
		client:     client,
		collection: collection,
	}, nil
}

func (db *AttemptDB) Close(ctx context.Context) error {
	if db.client != nil {
		return db.client.Disconnect(ctx)
	}
	return nil
}

func (db *AttemptDB) CreateAttempt(ctx context.Context, attempt *TaskAttempt) error {
	if attempt == nil {
		return fmt.Errorf("attempt is required")
	}
	if attempt.AttemptID == "" {
		return fmt.Errorf("attempt_id is required")
	}
	if attempt.AssignedAt.IsZero() {
		attempt.AssignedAt = time.Now()
	}
	if attempt.Status == "" {
		attempt.Status = AttemptStatusAssigned
	}

	_, err := db.collection.InsertOne(ctx, attempt)
	if err != nil {
		return fmt.Errorf("insert attempt: %w", err)
	}
	return nil
}

func (db *AttemptDB) GetAttempt(ctx context.Context, attemptID string) (*TaskAttempt, error) {
	var attempt TaskAttempt
	err := db.collection.FindOne(ctx, bson.M{"attempt_id": attemptID}).Decode(&attempt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("find attempt: %w", err)
	}
	return &attempt, nil
}

func (db *AttemptDB) GetAttemptsByTask(ctx context.Context, taskID string) ([]*TaskAttempt, error) {
	opts := options.Find().SetSort(bson.D{{Key: "attempt_no", Value: 1}})
	cursor, err := db.collection.Find(ctx, bson.M{"task_id": taskID}, opts)
	if err != nil {
		return nil, fmt.Errorf("find attempts by task: %w", err)
	}
	defer cursor.Close(ctx)

	var attempts []*TaskAttempt
	if err := cursor.All(ctx, &attempts); err != nil {
		return nil, fmt.Errorf("decode attempts by task: %w", err)
	}
	return attempts, nil
}

func (db *AttemptDB) GetActiveAttemptsByWorker(ctx context.Context, workerID string) ([]*TaskAttempt, error) {
	filter := bson.M{
		"worker_id": workerID,
		"status": bson.M{
			"$in": bson.A{AttemptStatusAssigned, AttemptStatusRunning},
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "assigned_at", Value: 1}})
	cursor, err := db.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("find active attempts by worker: %w", err)
	}
	defer cursor.Close(ctx)

	var attempts []*TaskAttempt
	if err := cursor.All(ctx, &attempts); err != nil {
		return nil, fmt.Errorf("decode active attempts by worker: %w", err)
	}
	return attempts, nil
}

func (db *AttemptDB) TouchHeartbeat(ctx context.Context, attemptID string, heartbeatTs int64) error {
	if attemptID == "" {
		return nil
	}
	_, err := db.collection.UpdateOne(ctx, bson.M{"attempt_id": attemptID}, bson.M{
		"$set": bson.M{
			"status":         AttemptStatusRunning,
			"last_heartbeat": heartbeatTs,
		},
	})
	if err != nil {
		return fmt.Errorf("touch attempt heartbeat: %w", err)
	}
	return nil
}

func (db *AttemptDB) MarkAttemptLost(ctx context.Context, attemptID, failureReason string) error {
	if attemptID == "" {
		return nil
	}
	_, err := db.collection.UpdateOne(ctx, bson.M{"attempt_id": attemptID}, bson.M{
		"$set": bson.M{
			"status":         AttemptStatusLost,
			"failure_reason": failureReason,
			"completed_at":   time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("mark attempt lost: %w", err)
	}
	return nil
}

func (db *AttemptDB) CompleteAttempt(ctx context.Context, attemptID, finalStatus, failureReason, resultStatus, logs, resultLocation string, outputFiles []string) error {
	if attemptID == "" {
		return nil
	}
	update := bson.M{
		"$set": bson.M{
			"status":          finalStatus,
			"failure_reason":  failureReason,
			"result_status":   resultStatus,
			"logs":            logs,
			"result_location": resultLocation,
			"output_files":    outputFiles,
			"completed_at":    time.Now(),
		},
	}
	_, err := db.collection.UpdateOne(ctx, bson.M{"attempt_id": attemptID}, update)
	if err != nil {
		return fmt.Errorf("complete attempt: %w", err)
	}
	return nil
}
