package main

import (
	"context"
	"log"
	"net"
	"time"

	"master/internal/cli"
	"master/internal/db"
	"master/internal/server"
	pb "master/proto/pb"

	"google.golang.org/grpc"
)

const (
	grpcPort = ":50051"
)

func main() {
	// Initialize MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.EnsureCollections(ctx); err != nil {
		log.Printf("Warning: MongoDB initialization failed: %v", err)
		log.Println("Continuing without database...")
	} else {
		log.Println("✓ MongoDB collections ensured")
	}

	// Create master server
	masterServer := server.NewMasterServer()

	// Start gRPC server in background
	go startGRPCServer(masterServer)

	// Start CLI interface
	log.Println("\n✓ Master node started successfully")
	log.Printf("✓ gRPC server listening on %s\n", grpcPort)

	cliInterface := cli.NewCLI(masterServer)
	cliInterface.Run()
}

func startGRPCServer(masterServer *server.MasterServer) {
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
