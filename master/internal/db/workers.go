package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"master/internal/config"
	pb "master/proto"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WorkerDB struct {
	client     *mongo.Client
	collection *mongo.Collection
}

type WorkerDocument struct {
	WorkerID     string  `bson:"worker_id"`
	WorkerIP     string  `bson:"worker_ip"` // Format: "ip:port" (e.g., "192.168.1.100:50052")
	TotalCPU     float64 `bson:"total_cpu"`
	TotalMemory  float64 `bson:"total_memory"`
	TotalStorage float64 `bson:"total_storage"`
	TotalGPU     float64 `bson:"total_gpu"`
	// Resource tracking
	AllocatedCPU     float64   `bson:"allocated_cpu"`
	AllocatedMemory  float64   `bson:"allocated_memory"`
	AllocatedStorage float64   `bson:"allocated_storage"`
	AllocatedGPU     float64   `bson:"allocated_gpu"`
	AvailableCPU     float64   `bson:"available_cpu"`
	AvailableMemory  float64   `bson:"available_memory"`
	AvailableStorage float64   `bson:"available_storage"`
	AvailableGPU     float64   `bson:"available_gpu"`
	IsActive         bool      `bson:"is_active"`
	LastHeartbeat    int64     `bson:"last_heartbeat"`
	RegisteredAt     time.Time `bson:"registered_at"`
	UpdatedAt        time.Time `bson:"updated_at"`
}

// NewWorkerDB creates a new WorkerDB instance
func NewWorkerDB(ctx context.Context, cfg *config.Config) (*WorkerDB, error) {
	loadDotEnv()

	user := os.Getenv("MONGODB_USERNAME")
	pass := os.Getenv("MONGODB_PASSWORD")
	if user == "" || pass == "" {
		return nil, errors.New("missing MongoDB credentials in environment")
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDBURI).SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongo: %w", err)
	}

	database := client.Database(cfg.MongoDBDatabase)
	collection := database.Collection("WORKER_REGISTRY")

	return &WorkerDB{
		client:     client,
		collection: collection,
	}, nil
}

// Close closes the database connection
func (db *WorkerDB) Close(ctx context.Context) error {
	if db.client != nil {
		return db.client.Disconnect(ctx)
	}
	return nil
}

// RegisterWorker registers a new worker (manual registration with just ID and address)
// workerIP should be in format "ip:port" (e.g., "192.168.1.100:50052")
func (db *WorkerDB) RegisterWorker(ctx context.Context, workerID, workerIP string) error {
	doc := WorkerDocument{
		WorkerID:     workerID,
		WorkerIP:     workerIP, // Format: "ip:port"
		TotalCPU:     0.0,      // Will be updated when worker connects
		TotalMemory:  0.0,
		TotalStorage: 0.0,
		TotalGPU:     0.0,
		// Initialize resource tracking
		AllocatedCPU:     0.0,
		AllocatedMemory:  0.0,
		AllocatedStorage: 0.0,
		AllocatedGPU:     0.0,
		AvailableCPU:     0.0,
		AvailableMemory:  0.0,
		AvailableStorage: 0.0,
		AvailableGPU:     0.0,
		IsActive:         false,
		RegisteredAt:     time.Now(),
		UpdatedAt:        time.Now(),
	}

	_, err := db.collection.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("insert worker: %w", err)
	}

	return nil
}

// UpdateWorkerInfo updates worker details (called when worker connects and sends full specs)
func (db *WorkerDB) UpdateWorkerInfo(ctx context.Context, info *pb.WorkerInfo) error {
	filter := bson.M{"worker_id": info.WorkerId}

	// Get current allocated resources to preserve them
	var currentWorker WorkerDocument
	err := db.collection.FindOne(ctx, filter).Decode(&currentWorker)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("worker %s not found", info.WorkerId)
		}
		return fmt.Errorf("find worker: %w", err)
	}

	// Calculate available resources: total - allocated
	availableCPU := info.TotalCpu - currentWorker.AllocatedCPU
	availableMemory := info.TotalMemory - currentWorker.AllocatedMemory
	availableStorage := info.TotalStorage - currentWorker.AllocatedStorage
	availableGPU := info.TotalGpu - currentWorker.AllocatedGPU

	update := bson.M{
		"$set": bson.M{
			"worker_ip":         info.WorkerIp,
			"total_cpu":         info.TotalCpu,
			"total_memory":      info.TotalMemory,
			"total_storage":     info.TotalStorage,
			"total_gpu":         info.TotalGpu,
			"available_cpu":     availableCPU,
			"available_memory":  availableMemory,
			"available_storage": availableStorage,
			"available_gpu":     availableGPU,
			"is_active":         true,
			"last_heartbeat":    time.Now().Unix(),
			"updated_at":        time.Now(),
		},
	}

	result, err := db.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("update worker info: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("worker %s not found", info.WorkerId)
	}

	return nil
}

// UpdateHeartbeat updates the last heartbeat timestamp
func (db *WorkerDB) UpdateHeartbeat(ctx context.Context, workerID string, timestamp int64) error {
	filter := bson.M{"worker_id": workerID}
	update := bson.M{
		"$set": bson.M{
			"last_heartbeat": timestamp,
			"is_active":      true,
			"updated_at":     time.Now(),
		},
	}

	_, err := db.collection.UpdateOne(ctx, filter, update)
	return err
}

// UnregisterWorker removes a worker from the registry
func (db *WorkerDB) UnregisterWorker(ctx context.Context, workerID string) error {
	filter := bson.M{"worker_id": workerID}
	result, err := db.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("delete worker: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("worker %s not found", workerID)
	}

	return nil
}

// GetWorker retrieves a worker by ID
func (db *WorkerDB) GetWorker(ctx context.Context, workerID string) (*WorkerDocument, error) {
	filter := bson.M{"worker_id": workerID}
	var doc WorkerDocument

	err := db.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("find worker: %w", err)
	}

	return &doc, nil
}

// GetAllWorkers retrieves all registered workers
func (db *WorkerDB) GetAllWorkers(ctx context.Context) ([]WorkerDocument, error) {
	cursor, err := db.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("find workers: %w", err)
	}
	defer cursor.Close(ctx)

	var workers []WorkerDocument
	if err := cursor.All(ctx, &workers); err != nil {
		return nil, fmt.Errorf("decode workers: %w", err)
	}

	return workers, nil
}

// WorkerExists checks if a worker is already registered
func (db *WorkerDB) WorkerExists(ctx context.Context, workerID string) (bool, error) {
	filter := bson.M{"worker_id": workerID}
	count, err := db.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("count workers: %w", err)
	}
	return count > 0, nil
}

// AllocateResources allocates resources to a worker when a task is assigned
func (db *WorkerDB) AllocateResources(ctx context.Context, workerID string, cpu, memory, storage, gpu float64) error {
	filter := bson.M{"worker_id": workerID}
	update := bson.M{
		"$inc": bson.M{
			"allocated_cpu":     cpu,
			"allocated_memory":  memory,
			"allocated_storage": storage,
			"allocated_gpu":     gpu,
			"available_cpu":     -cpu,
			"available_memory":  -memory,
			"available_storage": -storage,
			"available_gpu":     -gpu,
		},
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	result, err := db.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("allocate resources: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("worker %s not found", workerID)
	}

	return nil
}

// ReleaseResources releases resources from a worker when a task completes
func (db *WorkerDB) ReleaseResources(ctx context.Context, workerID string, cpu, memory, storage, gpu float64) error {
	filter := bson.M{"worker_id": workerID}
	update := bson.M{
		"$inc": bson.M{
			"allocated_cpu":     -cpu,
			"allocated_memory":  -memory,
			"allocated_storage": -storage,
			"allocated_gpu":     -gpu,
			"available_cpu":     cpu,
			"available_memory":  memory,
			"available_storage": storage,
			"available_gpu":     gpu,
		},
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	result, err := db.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("release resources: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("worker %s not found", workerID)
	}

	return nil
}

// SetWorkerResources sets the exact resource allocation values for a worker (used during reconciliation)
func (db *WorkerDB) SetWorkerResources(ctx context.Context, workerID string,
	allocatedCPU, allocatedMemory, allocatedStorage, allocatedGPU,
	availableCPU, availableMemory, availableStorage, availableGPU float64) error {

	filter := bson.M{"worker_id": workerID}
	update := bson.M{
		"$set": bson.M{
			"allocated_cpu":     allocatedCPU,
			"allocated_memory":  allocatedMemory,
			"allocated_storage": allocatedStorage,
			"allocated_gpu":     allocatedGPU,
			"available_cpu":     availableCPU,
			"available_memory":  availableMemory,
			"available_storage": availableStorage,
			"available_gpu":     availableGPU,
			"updated_at":        time.Now(),
		},
	}

	result, err := db.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("set worker resources: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("worker %s not found", workerID)
	}

	return nil
}
