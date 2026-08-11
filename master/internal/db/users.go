package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"master/internal/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user document in MongoDB
type User struct {
	Email        string    `bson:"email" json:"email"`
	Name         string    `bson:"name" json:"name"`
	PasswordHash string    `bson:"password_hash" json:"-"` // Never return in JSON
	VisitCount   int       `bson:"visit_count" json:"visit_count"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
}

// UserDB handles user database operations
type UserDB struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// NewUserDB creates a new UserDB instance
func NewUserDB(ctx context.Context, cfg *config.Config) (*UserDB, error) {
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
	collection := database.Collection("USERS")

	return &UserDB{
		client:     client,
		collection: collection,
	}, nil
}

// Close closes the database connection
func (db *UserDB) Close(ctx context.Context) error {
	if db.client != nil {
		return db.client.Disconnect(ctx)
	}
	return nil
}

// CreateUser creates a new user with hashed password
func (db *UserDB) CreateUser(name, email, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if user already exists
	var existingUser User
	err := db.collection.FindOne(ctx, bson.M{"email": email}).Decode(&existingUser)
	if err == nil {
		return errors.New("user with this email already exists")
	}
	if err != mongo.ErrNoDocuments {
		return err
	}

	// Hash password with bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := User{
		Email:        email,
		Name:         name,
		PasswordHash: string(hashedPassword),
		VisitCount:   0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = db.collection.InsertOne(ctx, user)
	return err
}

// GetUserByEmail retrieves a user by email
func (db *UserDB) GetUserByEmail(email string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user User
	err := db.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

// ValidateCredentials checks if email and password are correct
func (db *UserDB) ValidateCredentials(email, password string) (*User, error) {
	user, err := db.GetUserByEmail(email)
	if err != nil {
		return nil, err
	}

	// Compare password with hashed password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

// IncrementVisitCount increments the visit count for a user
func (db *UserDB) IncrementVisitCount(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$inc": bson.M{"visit_count": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}

	_, err := db.collection.UpdateOne(ctx, bson.M{"email": email}, update)
	return err
}

// UpdateUser updates user information
func (db *UserDB) UpdateUser(email string, updates bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates["updated_at"] = time.Now()

	_, err := db.collection.UpdateOne(ctx, bson.M{"email": email}, bson.M{"$set": updates})
	return err
}
