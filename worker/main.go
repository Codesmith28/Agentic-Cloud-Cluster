package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"worker/internal/server"
	"worker/internal/system"
	"worker/internal/telemetry"
	pb "worker/proto"

	"google.golang.org/grpc"
)

func main() {
	log.Println("═══════════════════════════════════════════════════════")
	log.Println("  CloudAI Worker Node - Starting...")
	log.Println("═══════════════════════════════════════════════════════")

	// Collect system information
	sysInfo, err := system.CollectSystemInfo()
	if err != nil {
		log.Fatalf("Failed to collect system information: %v", err)
	}

	// Find available port starting from default
	defaultPort := 50052
	availablePort, err := system.FindAvailablePort(defaultPort)
	if err != nil {
		log.Fatalf("Failed to find available port: %v", err)
	}
	sysInfo.SetWorkerPort(availablePort)

	// Auto-detect IP address and port
	workerIP := sysInfo.GetWorkerAddress()
	workerPort := sysInfo.GetWorkerPort()
	workerID := sysInfo.Hostname // Use hostname as worker ID

	log.Println("")
	log.Println("═══════════════════════════════════════════════════════")
	log.Println("  Worker Details (use these to register with master):")
	log.Println("═══════════════════════════════════════════════════════")
	log.Printf("  Worker ID:      %s", workerID)
	log.Printf("  Worker Address: %s%s", workerIP, workerPort)
	log.Println("═══════════════════════════════════════════════════════")
	log.Println("")
	log.Printf("To register this worker, run in master CLI:")
	log.Printf("  master> register %s %s%s", workerID, workerIP, workerPort)
	log.Println("")
	log.Println("═══════════════════════════════════════════════════════")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create telemetry monitor (master address will be set later when master registers)
	monitor := telemetry.NewMonitor(workerID, 5*time.Second)

	// Start telemetry monitoring (will start sending heartbeats once master is known)
	go monitor.Start(ctx)

	// Create worker server
	workerServer, err := server.NewWorkerServer(workerID, monitor)
	if err != nil {
		log.Fatalf("Failed to create worker server: %v", err)
	}
	defer workerServer.Close()

	// Start gRPC server
	workerAddress := workerIP + workerPort
	lis, err := net.Listen("tcp", workerAddress)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", workerAddress, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMasterWorkerServer(grpcServer, workerServer)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n╔═══════════════════════════════════════════════════════")
		fmt.Println("║  Shutdown signal received - gracefully shutting down...")
		fmt.Println("╚═══════════════════════════════════════════════════════")

		// Report running tasks as failed before shutting down
		workerServer.Shutdown()

		monitor.Stop()
		grpcServer.GracefulStop()
		cancel()
	}()

	log.Printf("✓ Worker %s started successfully", workerID)
	log.Printf("✓ gRPC server listening on %s", workerAddress)
	log.Println("✓ Ready to receive master registration...")
	log.Println("✓ Waiting for tasks...")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
