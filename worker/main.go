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
	"worker/internal/telemetry"
	pb "worker/proto"

	"google.golang.org/grpc"
)

var (
	workerID   = flag.String("id", "worker-1", "Worker ID")
	workerIP   = flag.String("ip", "localhost", "Worker IP address")
	masterAddr = flag.String("master", "localhost:50051", "Master server address")
	grpcPort   = flag.String("port", ":50052", "gRPC server port")
)

func main() {
	flag.Parse()

	log.Printf("Starting Worker Node: %s", *workerID)
	log.Printf("Master Address: %s", *masterAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create telemetry monitor
	monitor := telemetry.NewMonitor(*workerID, *masterAddr, 5*time.Second)

	// Start telemetry monitoring
	go monitor.Start(ctx)

	// Create worker server
	workerServer, err := server.NewWorkerServer(*workerID, *masterAddr, monitor)
	if err != nil {
		log.Fatalf("Failed to create worker server: %v", err)
	}
	defer workerServer.Close()

	// Start gRPC server
	lis, err := net.Listen("tcp", *grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *grpcPort, err)
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
	log.Printf("✓ gRPC server listening on %s", *grpcPort)
	log.Println("✓ Ready to receive tasks...")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
