package db

import (
	"context"
	"fmt"

	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// EnsureCollections ensures that all collections required by the master node exist.
func EnsureCollections(ctx context.Context, store *MongoStore) error {
	if store == nil || store.Database() == nil {
		return fmt.Errorf("mongo store is nil or uninitialized")
	}

	database := store.Database()

	existing, err := database.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		existingSet[name] = struct{}{}
	}

	for _, name := range constants.AllRequiredCollections {
		if _, ok := existingSet[name]; ok {
			continue
		}
		if err := database.CreateCollection(ctx, name); err != nil {
			return fmt.Errorf("create collection %s: %w", name, err)
		}
	}

	return nil
}
