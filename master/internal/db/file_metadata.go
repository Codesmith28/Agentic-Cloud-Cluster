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

// FileMetadata represents file metadata stored in MongoDB
type FileMetadata struct {
	UserID      string    `bson:"user_id"`
	TaskID      string    `bson:"task_id"`
	TaskName    string    `bson:"task_name"`
	Timestamp   time.Time `bson:"timestamp"`
	FilePaths   []string  `bson:"file_paths"`
	StoragePath string    `bson:"storage_path"`
	UploadedAt  time.Time `bson:"uploaded_at"`
}

// FileMetadataDB handles file metadata operations
type FileMetadataDB struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// NewFileMetadataDB creates a new FileMetadataDB instance
func NewFileMetadataDB(ctx context.Context, cfg *config.Config) (*FileMetadataDB, error) {
	clientOptions := options.Client().ApplyURI(cfg.MongoDBURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the database
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	collection := client.Database(cfg.MongoDBDatabase).Collection("FILE_METADATA")

	// Create indexes
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "task_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "task_name", Value: 1}},
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
		},
	}

	_, err = collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes: %v", err)
	}

	return &FileMetadataDB{
		client:     client,
		collection: collection,
	}, nil
}

// Close closes the database connection
func (db *FileMetadataDB) Close(ctx context.Context) error {
	return db.client.Disconnect(ctx)
}

// CreateFileMetadata stores file metadata
func (db *FileMetadataDB) CreateFileMetadata(ctx context.Context, metadata *FileMetadata) error {
	metadata.UploadedAt = time.Now()

	_, err := db.collection.InsertOne(ctx, metadata)
	if err != nil {
		log.Printf("Error creating file metadata: %v", err)
		return err
	}

	return nil
}

// GetFileMetadataByTask retrieves file metadata by task ID
func (db *FileMetadataDB) GetFileMetadataByTask(ctx context.Context, taskID string) (*FileMetadata, error) {
	var metadata FileMetadata
	err := db.collection.FindOne(ctx, bson.M{"task_id": taskID}).Decode(&metadata)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		log.Printf("Error getting file metadata: %v", err)
		return nil, err
	}

	return &metadata, nil
}

// GetFileMetadataByUser retrieves all file metadata for a user
func (db *FileMetadataDB) GetFileMetadataByUser(ctx context.Context, userID string) ([]*FileMetadata, error) {
	cursor, err := db.collection.Find(ctx, bson.M{"user_id": userID},
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}))
	if err != nil {
		log.Printf("Error finding file metadata: %v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*FileMetadata
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("Error decoding file metadata: %v", err)
		return nil, err
	}

	return results, nil
}

// GetFileMetadataByUserAndTaskName retrieves file metadata by user and task name
func (db *FileMetadataDB) GetFileMetadataByUserAndTaskName(ctx context.Context, userID, taskName string) ([]*FileMetadata, error) {
	cursor, err := db.collection.Find(ctx, bson.M{
		"user_id":   userID,
		"task_name": taskName,
	}, options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}))
	if err != nil {
		log.Printf("Error finding file metadata: %v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*FileMetadata
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("Error decoding file metadata: %v", err)
		return nil, err
	}

	return results, nil
}

// DeleteFileMetadata removes file metadata
func (db *FileMetadataDB) DeleteFileMetadata(ctx context.Context, taskID string) error {
	_, err := db.collection.DeleteOne(ctx, bson.M{"task_id": taskID})
	if err != nil {
		log.Printf("Error deleting file metadata: %v", err)
		return err
	}

	return nil
}

// UpdateFileMetadata updates file metadata
func (db *FileMetadataDB) UpdateFileMetadata(ctx context.Context, taskID string, update bson.M) error {
	_, err := db.collection.UpdateOne(ctx, bson.M{"task_id": taskID}, bson.M{"$set": update})
	if err != nil {
		log.Printf("Error updating file metadata: %v", err)
		return err
	}

	return nil
}
