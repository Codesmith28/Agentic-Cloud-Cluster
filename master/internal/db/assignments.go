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

// Assignment represents a task-worker assignment in the database
type Assignment struct {
	AssignmentID string    `bson:"ass_id"`
	TaskID       string    `bson:"task_id"`
	WorkerID     string    `bson:"worker_id"`
	AssignedAt   time.Time `bson:"assigned_at"`
	LoadAtStart  float64   `bson:"load_at_start,omitempty"` // Worker load (0-1) when task was assigned
}

// AssignmentDB handles assignment-related database operations
type AssignmentDB struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// NewAssignmentDB creates a new AssignmentDB instance
func NewAssignmentDB(ctx context.Context, cfg *config.Config) (*AssignmentDB, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDBURI))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	collection := client.Database(cfg.MongoDBDatabase).Collection("ASSIGNMENTS")

	return &AssignmentDB{
		client:     client,
		collection: collection,
	}, nil
}

// CreateAssignment inserts a new assignment into the database
func (db *AssignmentDB) CreateAssignment(ctx context.Context, assignment *Assignment) error {
	assignment.AssignedAt = time.Now()

	_, err := db.collection.InsertOne(ctx, assignment)
	if err != nil {
		return fmt.Errorf("insert assignment: %w", err)
	}

	return nil
}

// GetAssignmentByTaskID retrieves an assignment by task_id
func (db *AssignmentDB) GetAssignmentByTaskID(ctx context.Context, taskID string) (*Assignment, error) {
	var assignment Assignment
	err := db.collection.FindOne(ctx, bson.M{"task_id": taskID}).Decode(&assignment)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("assignment not found for task: %s", taskID)
		}
		return nil, fmt.Errorf("find assignment: %w", err)
	}

	return &assignment, nil
}

// GetAssignmentsByWorker retrieves all assignments for a specific worker
func (db *AssignmentDB) GetAssignmentsByWorker(ctx context.Context, workerID string) ([]*Assignment, error) {
	cursor, err := db.collection.Find(ctx, bson.M{"worker_id": workerID})
	if err != nil {
		return nil, fmt.Errorf("find assignments: %w", err)
	}
	defer cursor.Close(ctx)

	var assignments []*Assignment
	if err := cursor.All(ctx, &assignments); err != nil {
		return nil, fmt.Errorf("decode assignments: %w", err)
	}

	return assignments, nil
}

// GetWorkerForTask retrieves the worker ID assigned to a task
func (db *AssignmentDB) GetWorkerForTask(ctx context.Context, taskID string) (string, error) {
	assignment, err := db.GetAssignmentByTaskID(ctx, taskID)
	if err != nil {
		return "", err
	}

	return assignment.WorkerID, nil
}

// DeleteAssignment removes an assignment from the database
func (db *AssignmentDB) DeleteAssignment(ctx context.Context, taskID string) error {
	result, err := db.collection.DeleteOne(ctx, bson.M{"task_id": taskID})
	if err != nil {
		return fmt.Errorf("delete assignment: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("assignment not found for task: %s", taskID)
	}

	return nil
}

// ListAllAssignments retrieves all assignments (for admin purposes)
func (db *AssignmentDB) ListAllAssignments(ctx context.Context) ([]*Assignment, error) {
	cursor, err := db.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("find all assignments: %w", err)
	}
	defer cursor.Close(ctx)

	var assignments []*Assignment
	if err := cursor.All(ctx, &assignments); err != nil {
		return nil, fmt.Errorf("decode assignments: %w", err)
	}

	return assignments, nil
}

// Close closes the database connection
func (db *AssignmentDB) Close(ctx context.Context) error {
	if db.client != nil {
		return db.client.Disconnect(ctx)
	}
	return nil
}
