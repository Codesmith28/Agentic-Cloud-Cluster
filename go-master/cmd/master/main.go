package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
	"github.com/Codesmith28/CloudAI/pkg/master"
	"github.com/Codesmith28/CloudAI/pkg/taskqueue"
	"github.com/Codesmith28/CloudAI/pkg/workerregistry"
	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting CloudAI Master Node...")

	// Initialize core components
	registry := workerregistry.NewRegistry()
	taskQueue := taskqueue.NewTaskQueue()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	masterServer := master.NewMasterServer(taskQueue, registry)
	pb.RegisterSchedulerServiceServer(grpcServer, masterServer)

	// Start listener
	port := getEnvOrDefault("MASTER_PORT", "50051")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Master node listening on port %s", port)

	// Start background cleanup routines
	go cleanupLoop(registry)

	// Handle graceful shutdown
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")
	grpcServer.GracefulStop()
	log.Println("Master node stopped")
}

func cleanupLoop(registry *workerregistry.Registry) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		registry.CleanupStaleWorkers(30 * time.Second)
		registry.CleanupExpiredReservations()
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
