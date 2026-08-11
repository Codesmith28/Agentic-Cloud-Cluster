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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"master/internal/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	schedulerModelsCollection = "SCHEDULER_MODELS"
	schedulerModelsBucketName = "scheduler_models"
)

type SchedulerModelMetadata struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty"`
	SchedulerType      string             `bson:"scheduler_type"`
	FingerprintHash    string             `bson:"fingerprint_hash"`
	FingerprintPayload string             `bson:"fingerprint_payload,omitempty"`
	Version            int64              `bson:"version"`
	Active             bool               `bson:"active"`
	FileID             primitive.ObjectID `bson:"file_id"`
	FileSize           int64              `bson:"file_size"`
	FileSHA256         string             `bson:"file_sha256"`
	Framework          string             `bson:"framework"`
	Metadata           map[string]any     `bson:"metadata,omitempty"`
	CreatedAt          time.Time          `bson:"created_at"`
	UpdatedAt          time.Time          `bson:"updated_at"`
	ActivatedAt        *time.Time         `bson:"activated_at,omitempty"`
}

type SchedulerModelDB struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
	bucket     *gridfs.Bucket
}

func NewSchedulerModelDB(ctx context.Context, cfg *config.Config) (*SchedulerModelDB, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDBURI).SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongo: %w", err)
	}

	database := client.Database(cfg.MongoDBDatabase)
	bucket, err := gridfs.NewBucket(database, options.GridFSBucket().SetName(schedulerModelsBucketName))
	if err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("create scheduler models bucket: %w", err)
	}

	db := &SchedulerModelDB{
		client:     client,
		database:   database,
		collection: database.Collection(schedulerModelsCollection),
		bucket:     bucket,
	}
	if err := db.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}
	return db, nil
}

func (db *SchedulerModelDB) Close(ctx context.Context) error {
	if db.client != nil {
		return db.client.Disconnect(ctx)
	}
	return nil
}

func (db *SchedulerModelDB) ensureIndexes(ctx context.Context) error {
	if db.collection == nil {
		return fmt.Errorf("scheduler models collection is nil")
	}
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "scheduler_type", Value: 1},
				{Key: "fingerprint_hash", Value: 1},
				{Key: "version", Value: -1},
			},
			Options: options.Index().SetName("scheduler_lookup_idx"),
		},
		{
			Keys: bson.D{
				{Key: "scheduler_type", Value: 1},
				{Key: "fingerprint_hash", Value: 1},
				{Key: "active", Value: 1},
			},
			Options: options.Index().
				SetName("scheduler_active_unique_idx").
				SetUnique(true).
				SetPartialFilterExpression(bson.D{{Key: "active", Value: true}}),
		},
	}
	_, err := db.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("create scheduler model indexes: %w", err)
	}
	return nil
}

func (db *SchedulerModelDB) nextVersion(ctx context.Context, schedulerType, fingerprintHash string) (int64, error) {
	var latest SchedulerModelMetadata
	err := db.collection.FindOne(
		ctx,
		bson.M{
			"scheduler_type":   schedulerType,
			"fingerprint_hash": fingerprintHash,
		},
		options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}}),
	).Decode(&latest)
	if err == mongo.ErrNoDocuments {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return latest.Version + 1, nil
}

func (db *SchedulerModelDB) SaveAndActivateModel(
	ctx context.Context,
	schedulerType string,
	fingerprintHash string,
	fingerprintPayload string,
	payload []byte,
	framework string,
	metadata map[string]any,
) (*SchedulerModelMetadata, error) {
	if schedulerType == "" || fingerprintHash == "" {
		return nil, fmt.Errorf("schedulerType and fingerprintHash are required")
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty model payload")
	}

	version, err := db.nextVersion(ctx, schedulerType, fingerprintHash)
	if err != nil {
		return nil, fmt.Errorf("next scheduler model version: %w", err)
	}

	now := time.Now().UTC()
	sum := sha256.Sum256(payload)
	filename := fmt.Sprintf("%s_%s_v%d.ckpt", schedulerType, fingerprintHash, version)
	fileID, err := db.bucket.UploadFromStream(filename, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("upload checkpoint to GridFS: %w", err)
	}

	doc := &SchedulerModelMetadata{
		SchedulerType:      schedulerType,
		FingerprintHash:    fingerprintHash,
		FingerprintPayload: fingerprintPayload,
		Version:            version,
		Active:             false,
		FileID:             fileID,
		FileSize:           int64(len(payload)),
		FileSHA256:         hex.EncodeToString(sum[:]),
		Framework:          framework,
		Metadata:           metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	insertRes, err := db.collection.InsertOne(ctx, doc)
	if err != nil {
		_ = db.bucket.Delete(fileID)
		return nil, fmt.Errorf("insert scheduler model metadata: %w", err)
	}
	insertedID, _ := insertRes.InsertedID.(primitive.ObjectID)

	_, _ = db.collection.UpdateMany(
		ctx,
		bson.M{
			"scheduler_type":   schedulerType,
			"fingerprint_hash": fingerprintHash,
			"active":           true,
			"_id":              bson.M{"$ne": insertedID},
		},
		bson.M{"$set": bson.M{"active": false, "updated_at": now}},
	)

	activatedAt := now
	_, err = db.collection.UpdateOne(
		ctx,
		bson.M{"_id": insertedID},
		bson.M{"$set": bson.M{
			"active":       true,
			"updated_at":   now,
			"activated_at": activatedAt,
		}},
	)
	if err != nil {
		return nil, fmt.Errorf("activate scheduler model version: %w", err)
	}

	doc.ID = insertedID
	doc.Active = true
	doc.ActivatedAt = &activatedAt
	return doc, nil
}

func (db *SchedulerModelDB) LoadActiveModel(
	ctx context.Context,
	schedulerType string,
	fingerprintHash string,
) ([]byte, *SchedulerModelMetadata, error) {
	filter := bson.M{
		"scheduler_type":   schedulerType,
		"fingerprint_hash": fingerprintHash,
		"active":           true,
	}

	var metadata SchedulerModelMetadata
	err := db.collection.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})).Decode(&metadata)
	if err == mongo.ErrNoDocuments {
		// Fallback to latest version if no active version marker exists.
		err = db.collection.FindOne(
			ctx,
			bson.M{"scheduler_type": schedulerType, "fingerprint_hash": fingerprintHash},
			options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}}),
		).Decode(&metadata)
	}
	if err != nil {
		return nil, nil, err
	}

	stream, err := db.bucket.OpenDownloadStream(metadata.FileID)
	if err != nil {
		return nil, nil, fmt.Errorf("open GridFS stream: %w", err)
	}
	defer stream.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := buf.ReadFrom(stream); err != nil {
		return nil, nil, fmt.Errorf("read GridFS checkpoint: %w", err)
	}
	return buf.Bytes(), &metadata, nil
}
