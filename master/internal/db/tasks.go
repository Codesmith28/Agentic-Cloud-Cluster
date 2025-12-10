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

// Task represents a task in the database
type Task struct {
	TaskID      string  `bson:"task_id"`
	UserID      string  `bson:"user_id"`
	TaskName    string  `bson:"task_name"`    // User-friendly task name
	SubmittedAt int64   `bson:"submitted_at"` // Unix timestamp when task was submitted
	DockerImage string  `bson:"docker_image"`
	Command     string  `bson:"command"`
	ReqCPU      float64 `bson:"req_cpu"`
	ReqMemory   float64 `bson:"req_memory"`
	ReqStorage  float64 `bson:"req_storage"`
	ReqGPU      float64 `bson:"req_gpu"`
	
	// GUI fields: generic tagging
	Tag    string  `bson:"tag,omitempty"`    // Generic tag field from GUI
	KValue float64 `bson:"k_value,omitempty"` // K-value from GUI (same as SLAMultiplier)
	
	// Scheduler fields: SLA and task classification
	TaskType      string    `bson:"task_type,omitempty"`    // Task type: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed
	SLAMultiplier float64   `bson:"sla_multiplier"`         // k value: 1.5-2.5, default: 2.0 (prioritized over KValue if both set)
	Deadline      time.Time `bson:"deadline,omitempty"`     // SLA deadline: arrival_time + k * tau
	Tau           float64   `bson:"tau,omitempty"`          // Expected runtime baseline (seconds)
	
	Status      string    `bson:"status"` // pending, running, completed, failed
	CreatedAt   time.Time `bson:"created_at"`
	StartedAt   time.Time `bson:"started_at,omitempty"`
	CompletedAt time.Time `bson:"completed_at,omitempty"`
}

// TaskDB handles task-related database operations
type TaskDB struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// NewTaskDB creates a new TaskDB instance
func NewTaskDB(ctx context.Context, cfg *config.Config) (*TaskDB, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDBURI))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	collection := client.Database(cfg.MongoDBDatabase).Collection("TASKS")

	return &TaskDB{
		client:     client,
		collection: collection,
	}, nil
}

// CreateTask inserts a new task into the database
func (db *TaskDB) CreateTask(ctx context.Context, task *Task) error {
	task.CreatedAt = time.Now()
	task.Status = "pending"

	_, err := db.collection.InsertOne(ctx, task)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	return nil
}

// GetTask retrieves a task by task_id
func (db *TaskDB) GetTask(ctx context.Context, taskID string) (*Task, error) {
	var task Task
	err := db.collection.FindOne(ctx, bson.M{"task_id": taskID}).Decode(&task)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, fmt.Errorf("find task: %w", err)
	}

	return &task, nil
}

// GetTasksByUser retrieves all tasks for a specific user
func (db *TaskDB) GetTasksByUser(ctx context.Context, userID string) ([]*Task, error) {
	cursor, err := db.collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, fmt.Errorf("find tasks: %w", err)
	}
	defer cursor.Close(ctx)

	var tasks []*Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}

	return tasks, nil
}

// GetAllTasks retrieves all tasks from the database
func (db *TaskDB) GetAllTasks(ctx context.Context) ([]*Task, error) {
	cursor, err := db.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("find all tasks: %w", err)
	}
	defer cursor.Close(ctx)

	var tasks []*Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}

	return tasks, nil
}

// GetTasksByStatus retrieves all tasks with a specific status
func (db *TaskDB) GetTasksByStatus(ctx context.Context, status string) ([]*Task, error) {
	cursor, err := db.collection.Find(ctx, bson.M{"status": status})
	if err != nil {
		return nil, fmt.Errorf("find tasks by status: %w", err)
	}
	defer cursor.Close(ctx)

	var tasks []*Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}

	return tasks, nil
}

// UpdateTaskStatus updates the status of a task
func (db *TaskDB) UpdateTaskStatus(ctx context.Context, taskID string, status string) error {
	update := bson.M{
		"$set": bson.M{
			"status": status,
		},
	}

	// Add timestamp fields based on status
	if status == "running" {
		update["$set"].(bson.M)["started_at"] = time.Now()
	} else if status == "completed" || status == "failed" {
		update["$set"].(bson.M)["completed_at"] = time.Now()
	}

	result, err := db.collection.UpdateOne(
		ctx,
		bson.M{"task_id": taskID},
		update,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}

// UpdateTaskMetadata updates task metadata fields such as tag and k_value
func (db *TaskDB) UpdateTaskMetadata(ctx context.Context, taskID string, tag string, kValue float64) error {
	update := bson.M{
		"$set": bson.M{
			"tag":        tag,
			"k_value":    kValue,
			"updated_at": time.Now(),
		},
	}

	result, err := db.collection.UpdateOne(ctx, bson.M{"task_id": taskID}, update)
	if err != nil {
		return fmt.Errorf("update task metadata: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}
	return nil
}

// DeleteTask removes a task from the database
func (db *TaskDB) DeleteTask(ctx context.Context, taskID string) error {
	result, err := db.collection.DeleteOne(ctx, bson.M{"task_id": taskID})
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}

// ListAllTasks retrieves all tasks (for admin purposes)
func (db *TaskDB) ListAllTasks(ctx context.Context) ([]*Task, error) {
	cursor, err := db.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("find all tasks: %w", err)
	}
	defer cursor.Close(ctx)

	var tasks []*Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}

	return tasks, nil
}

// UpdateTaskWithSLA updates a task's SLA-related fields (deadline, tau, task type)
// This method is called after task submission to enrich it with scheduling parameters
func (db *TaskDB) UpdateTaskWithSLA(ctx context.Context, taskID string, deadline time.Time, tau float64, taskType string) error {
	// Validate task type
	validTypes := map[string]bool{
		"cpu-light": true, "cpu-heavy": true, "memory-heavy": true,
		"gpu-inference": true, "gpu-training": true, "mixed": true,
	}

	if taskType != "" && !validTypes[taskType] {
		return fmt.Errorf("invalid task type: %s (must be one of: cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed)", taskType)
	}

	// Build update document
	update := bson.M{
		"$set": bson.M{
			"deadline": deadline,
			"tau":      tau,
		},
	}

	// Only update task_type if it's provided and valid
	if taskType != "" {
		update["$set"].(bson.M)["task_type"] = taskType
	}

	result, err := db.collection.UpdateOne(
		ctx,
		bson.M{"task_id": taskID},
		update,
	)

	if err != nil {
		return fmt.Errorf("update task with SLA: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}

// Close closes the database connection
func (db *TaskDB) Close(ctx context.Context) error {
	if db.client != nil {
		return db.client.Disconnect(ctx)
	}
	return nil
}
