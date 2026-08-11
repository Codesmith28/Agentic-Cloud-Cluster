package main

import (
	"context"
	"log"

	"github.com/Codesmith28/CloudAI/pkg/persistence"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	log.Println("Initializing CouchDB...")

	// Initialize - this will create database if needed
	db, err := persistence.NewCouchDBClient()
	if err != nil {
		log.Fatalf("❌ Failed to initialize CouchDB: %v", err)
	}

	log.Println("✓ CouchDB initialized successfully")

	// Example: Save a task
	task := map[string]interface{}{
		"type":   "task",
		"id":     "task-123",
		"status": "pending",
	}

	if err := db.Put(context.Background(), "task:task-123", task); err != nil {
		log.Fatalf("❌ Error saving task: %v", err)
	}
	log.Println("✓ Task saved")

	// Example: Retrieve task
	var result map[string]interface{}
	if err := db.Get(context.Background(), "task:task-123", &result); err != nil {
		log.Fatalf("❌ Error retrieving task: %v", err)
	}
	log.Printf("✓ Retrieved task: %v", result)
}
