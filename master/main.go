package main

import (
	"context"
	"log"
	"time"

	"master/internal/db"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.EnsureCollections(ctx); err != nil {
		log.Fatalf("mongo initialization failed: %v", err)
	}

	log.Println("MongoDB collections ensured")
}
