package main

import (
	"context"
	"log"
	"net"
	"time"

	"master/internal/cli"
	"master/internal/config"
	"master/internal/db"
	"master/internal/server"
	pb "master/proto"

	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize database
	var workerDB *db.WorkerDB
	if err := db.EnsureCollections(ctx, cfg); err != nil {
		log.Printf("Warning: MongoDB initialization failed: %v", err)
		log.Println("Continuing without database persistence...")
	} else {
		log.Println("✓ MongoDB collections ensured")

		// Create worker database handler
		var err error
		workerDB, err = db.NewWorkerDB(ctx, cfg)
		if err != nil {
			log.Printf("Warning: Failed to create WorkerDB: %v", err)
			log.Println("Continuing without database persistence...")
			workerDB = nil
		} else {
			log.Println("✓ WorkerDB initialized")
			defer workerDB.Close(context.Background())
		}
	}

	// Create master server
	masterServer := server.NewMasterServer(workerDB)

	// Load workers from database
	if workerDB != nil {
		if err := masterServer.LoadWorkersFromDB(ctx); err != nil {
			log.Printf("Warning: Failed to load workers from DB: %v", err)
		}
	}

	// Start gRPC server in background
	go startGRPCServer(masterServer, cfg.GRPCPort)

	// Start CLI interface
	log.Println("\n✓ Master node started successfully")
	log.Printf("✓ gRPC server listening on %s\n", cfg.GRPCPort)

	cliInterface := cli.NewCLI(masterServer)
	cliInterface.Run()
}

func startGRPCServer(masterServer *server.MasterServer, grpcPort string) {
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcPort, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMasterWorkerServer(grpcServer, masterServer)

	log.Printf("Starting gRPC server on %s...", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
