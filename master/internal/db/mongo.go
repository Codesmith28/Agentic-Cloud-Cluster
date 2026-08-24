package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoStore manages a unified MongoDB client and connection pool for all database repositories.
type MongoStore struct {
	client   *mongo.Client
	database *mongo.Database
}

// NewMongoStore establishes a single pooled connection to MongoDB.
func NewMongoStore(ctx context.Context, uri, dbName string) (*MongoStore, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(50).
		SetMinPoolSize(5).
		SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongo: %w", err)
	}

	return &MongoStore{
		client:   client,
		database: client.Database(dbName),
	}, nil
}

// Collection returns a handle for the requested collection.
func (s *MongoStore) Collection(name string) *mongo.Collection {
	return s.database.Collection(name)
}

// Database returns the underlying Mongo database handle.
func (s *MongoStore) Database() *mongo.Database {
	return s.database
}

// Client returns the underlying Mongo client handle.
func (s *MongoStore) Client() *mongo.Client {
	return s.client
}

// Close gracefully closes the client connection.
func (s *MongoStore) Close(ctx context.Context) error {
	if s.client == nil {
		return nil
	}
	return s.client.Disconnect(ctx)
}
