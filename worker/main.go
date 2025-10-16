package main

import (
	"context"
	"flag"
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

// TODO: Fetch these automatically from the system
// take the worker name from what the master assigns it
var (
	workerID = flag.String("id", "worker-1", "Worker ID")
	workerIP = flag.String("ip", "localhost", "Worker IP address")
	grpcPort = flag.String("port", ":50052", "gRPC server port")
)

func main() {
	flag.Parse()

	// Collect system information
	sysInfo, err := system.CollectSystemInfo()
	if err != nil {
		log.Fatalf("Failed to collect system information: %v", err)
	}
	sysInfo.LogSystemInfo()

	// Use detected IP if not specified via flag
	workerIP := *workerIP
	if workerIP == "localhost" {
		workerIP = sysInfo.GetWorkerAddress()
	}

	log.Printf("Starting Worker Node: %s", *workerID)
	log.Printf("Worker IP: %s", workerIP)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create telemetry monitor (master address will be set later when master registers)
	monitor := telemetry.NewMonitor(*workerID, 5*time.Second)

	// Start telemetry monitoring (will start sending heartbeats once master is known)
	go monitor.Start(ctx)

	// Create worker server
	workerServer, err := server.NewWorkerServer(*workerID, monitor)
	if err != nil {
		log.Fatalf("Failed to create worker server: %v", err)
	}
	defer workerServer.Close()

	// Start gRPC server
	workerAddress := workerIP + *grpcPort
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
		log.Println("\nShutting down worker...")
		monitor.Stop()
		grpcServer.GracefulStop()
		cancel()
	}()

	log.Printf("✓ Worker %s started successfully", *workerID)
	log.Printf("✓ gRPC server listening on %s", workerAddress)
	log.Println("✓ Ready to receive tasks...")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
