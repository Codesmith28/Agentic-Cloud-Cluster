package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	rtsWeightsCollectionName = "RTS_WEIGHTS"
	activeGAParamsDocID      = "active"
)

// ErrNoStoredGAParams indicates that no GA params document exists yet in the store.
var ErrNoStoredGAParams = errors.New("no stored ga params")

// GAParamsStore defines persistence operations for RTS GA parameters.
type GAParamsStore interface {
	LoadGAParams(ctx context.Context) (*GAParams, error)
	SaveGAParams(ctx context.Context, params *GAParams) error
}

// MongoGAParamsStore persists RTS GA params in MongoDB.
type MongoGAParamsStore struct {
	client     *mongo.Client
	collection *mongo.Collection
}

type gaParamsDocument struct {
	ID        string    `bson:"_id"`
	Params    GAParams  `bson:"params"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// NewMongoGAParamsStore creates a MongoDB-backed GA params store.
func NewMongoGAParamsStore(ctx context.Context, mongoURI, database string) (*MongoGAParamsStore, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	return &MongoGAParamsStore{
		client:     client,
		collection: client.Database(database).Collection(rtsWeightsCollectionName),
	}, nil
}

// Close closes the MongoDB connection.
func (s *MongoGAParamsStore) Close(ctx context.Context) error {
	if s.client != nil {
		return s.client.Disconnect(ctx)
	}
	return nil
}

// LoadGAParams returns the latest persisted RTS GA params.
func (s *MongoGAParamsStore) LoadGAParams(ctx context.Context) (*GAParams, error) {
	var doc gaParamsDocument
	err := s.collection.FindOne(ctx, bson.M{"_id": activeGAParamsDocID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNoStoredGAParams
		}
		return nil, fmt.Errorf("find ga params document: %w", err)
	}

	normalizeLegacyTaskTypeAliases(&doc.Params)
	if err := validateGAParams(&doc.Params); err != nil {
		return nil, fmt.Errorf("validate stored ga params: %w", err)
	}

	return &doc.Params, nil
}

// SaveGAParams persists RTS GA params as the active document.
func (s *MongoGAParamsStore) SaveGAParams(ctx context.Context, params *GAParams) error {
	if params == nil {
		return fmt.Errorf("ga params is nil")
	}

	// Copy to avoid mutating caller-owned state during normalization.
	snapshot := *params
	normalizeLegacyTaskTypeAliases(&snapshot)
	if err := validateGAParams(&snapshot); err != nil {
		return fmt.Errorf("validate ga params before save: %w", err)
	}

	update := bson.M{
		"$set": bson.M{
			"params":     snapshot,
			"updated_at": time.Now(),
		},
		"$setOnInsert": bson.M{
			"_id": activeGAParamsDocID,
		},
	}

	if _, err := s.collection.UpdateOne(
		ctx,
		bson.M{"_id": activeGAParamsDocID},
		update,
		options.Update().SetUpsert(true),
	); err != nil {
		return fmt.Errorf("upsert ga params document: %w", err)
	}

	return nil
}
