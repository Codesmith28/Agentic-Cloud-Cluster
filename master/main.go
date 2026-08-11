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
	"master/internal/system"
	pb "master/proto"

	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Collect system information
	sysInfo, err := system.CollectSystemInfo()
	if err != nil {
		log.Fatalf("Failed to collect system information: %v", err)
	}
	sysInfo.SetMasterPort(cfg.GRPCPort)
	sysInfo.LogSystemInfo()

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
	masterAddress := sysInfo.GetMasterAddress() + cfg.GRPCPort
	go startGRPCServer(masterServer, masterAddress)

	// Wait briefly to ensure server is listening before contacting workers
	time.Sleep(500 * time.Millisecond)

	// Broadcast master registration to known workers so they can connect back
	masterServer.BroadcastMasterRegistration("master-1", masterAddress)

	// Start CLI interface
	log.Println("\n✓ Master node started successfully")
	log.Printf("✓ Starting gRPC server on %s\n", masterAddress)

	cliInterface := cli.NewCLI(masterServer)
	cliInterface.Run()
}

func startGRPCServer(masterServer *server.MasterServer, address string) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", address, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMasterWorkerServer(grpcServer, masterServer)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
