package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/envutil"
	workermetrics "worker/internal/metrics"
	"worker/internal/server"
	"worker/internal/system"
	"worker/internal/telemetry"
	pb "worker/proto"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// App manages the worker node lifecycle and service components.
type App struct{}

// New creates a new worker App instance.
func New() *App {
	return &App{}
}

// Run executes the worker node application.
func (a *App) Run() {
	// Docker HEALTHCHECK: exit 0 immediately when called with --healthcheck
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		os.Exit(0)
	}

	log.Println("═══════════════════════════════════════════════════════")
	log.Println("  Agentic Cloud Cluster Worker Node - Starting...")
	log.Println("═══════════════════════════════════════════════════════")

	// Determine output base directory
	outputBaseDir := envutil.GetEnv("AGENTIC_OUTPUT_DIR", envutil.GetEnv("CLOUDAI_OUTPUT_DIR", "/var/agentic-cloud-cluster/outputs"))
	if err := os.MkdirAll(outputBaseDir, 0700); err != nil {
		homeDir, _ := os.UserHomeDir()
		outputBaseDir = filepath.Join(homeDir, ".agentic-cloud-cluster", "outputs")
		if err := os.MkdirAll(outputBaseDir, 0700); err != nil {
			log.Fatalf("Failed to create output directory %s: %v", outputBaseDir, err)
		}
		log.Printf("⚠️  Using user directory (no root access): %s", outputBaseDir)
	} else {
		log.Printf("✓ Output directory ready (secure): %s", outputBaseDir)
	}

	os.Setenv("CLOUDAI_OUTPUT_DIR", outputBaseDir)
	os.Setenv("AGENTIC_OUTPUT_DIR", outputBaseDir)

	// Collect system information
	sysInfo, err := system.CollectSystemInfo()
	if err != nil {
		log.Fatalf("Failed to collect system information: %v", err)
	}

	defaultPort := 50052
	availablePort, err := system.ResolveWorkerPort(defaultPort)
	if err != nil {
		log.Fatalf("Failed to resolve worker port: %v", err)
	}
	sysInfo.SetWorkerPort(availablePort)

	workerIP := sysInfo.GetWorkerAddress()
	workerBindIP := system.ResolveWorkerBindIP(workerIP)
	workerPort := sysInfo.GetWorkerPort()
	metricsPort, err := system.ResolveWorkerMetricsPort(9101)
	if err != nil {
		log.Fatalf("Failed to resolve worker metrics port: %v", err)
	}
	workerID, err := system.ResolveWorkerID(sysInfo.Hostname)
	if err != nil {
		log.Printf("⚠️  Failed to resolve persistent worker ID: %v", err)
		workerID = sysInfo.Hostname
		if workerID == "" {
			workerID = "worker-unknown"
		}
	}

	log.Println("")
	log.Println("═══════════════════════════════════════════════════════")
	log.Println("  Worker Details (use these to register with master):")
	log.Println("═══════════════════════════════════════════════════════")
	log.Printf("  Hostname:       %s", sysInfo.Hostname)
	log.Printf("  Worker ID:      %s", workerID)
	log.Printf("  Bind Address:   %s%s", workerBindIP, workerPort)
	log.Printf("  Reachable Addr: %s%s", workerIP, workerPort)
	log.Printf("  Metrics Port:   %d", metricsPort)
	log.Println("═══════════════════════════════════════════════════════")
	log.Println("")
	log.Printf("To register this worker, run in master CLI:")
	log.Printf("  master> register %s %s%s", workerID, workerIP, workerPort)
	log.Println("")
	log.Println("═══════════════════════════════════════════════════════")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor := telemetry.NewMonitor(workerID, 5*time.Second)
	go monitor.Start(ctx)
	workermetrics.Get()

	go func() {
		metricsAddress := net.JoinHostPort(workerBindIP, fmt.Sprintf("%d", metricsPort))
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Printf("✓ Metrics server listening on %s", metricsAddress)
		if err := http.ListenAndServe(metricsAddress, mux); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server stopped: %v", err)
		}
	}()

	workerServer, err := server.NewWorkerServer(workerID, monitor)
	if err != nil {
		log.Fatalf("Failed to create worker server: %v", err)
	}
	defer workerServer.Close()

	workerAddress := net.JoinHostPort(workerBindIP, fmt.Sprintf("%d", availablePort))
	lis, err := net.Listen("tcp", workerAddress)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", workerAddress, err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(4 * 1024 * 1024),
	)
	pb.RegisterMasterWorkerServer(grpcServer, workerServer)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n╔═══════════════════════════════════════════════════════")
		fmt.Println("║  Shutdown signal received - gracefully shutting down...")
		fmt.Println("╚═══════════════════════════════════════════════════════")

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
