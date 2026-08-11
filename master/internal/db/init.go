package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"master/internal/config"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collections = []string{
	"USERS",
	"WORKER_REGISTRY",
	"TASKS",
	"ASSIGNMENTS",
	"RESULTS",
}

// EnsureCollections connects to the MongoDB instance and makes sure the
// collections required by the masternode exist. Idempotent-safe so repeat calls
// are inexpensive and harmless.
func EnsureCollections(ctx context.Context, cfg *config.Config) error {
	loadDotEnv()

	if cfg.MongoDBUsername == "" || cfg.MongoDBPassword == "" {
		return errors.New("missing MongoDB credentials in environment")
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDBURI).SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect mongo: %w", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("ping mongo: %w", err)
	}

	database := client.Database(cfg.MongoDBDatabase)

	existing, err := database.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		existingSet[name] = struct{}{}
	}

	for _, name := range collections {
		if _, ok := existingSet[name]; ok {
			continue
		}
		if err := database.CreateCollection(ctx, name); err != nil {
			return fmt.Errorf("create collection %s: %w", name, err)
		}
	}

	return nil
}

func loadDotEnv() {
	paths := []string{".env", "../.env", "../../.env"}
	for _, path := range paths {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
}
