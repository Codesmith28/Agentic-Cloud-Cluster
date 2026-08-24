package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/constants"
	"worker/internal/config"
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

	// Load typed configuration
	cfg := config.LoadConfig()

	// Collect system information
	sysInfo, err := system.CollectSystemInfo()
	if err != nil {
		log.Fatalf("Failed to collect system information: %v", err)
	}

	availablePort, err := system.ResolveWorkerPort(cfg.DefaultPort)
	if err != nil {
		log.Fatalf("Failed to resolve worker port: %v", err)
	}
	sysInfo.SetWorkerPort(availablePort)

	workerIP := sysInfo.GetWorkerAddress()
	if cfg.WorkerIP != "" {
		workerIP = cfg.WorkerIP
	}
	workerBindIP := system.ResolveWorkerBindIP(workerIP)
	workerPort := sysInfo.GetWorkerPort()
	metricsPort, err := system.ResolveWorkerMetricsPort(cfg.DefaultMetricsPort)
	if err != nil {
		log.Fatalf("Failed to resolve worker metrics port: %v", err)
	}

	var workerID string
	if cfg.WorkerID != "" {
		workerID = cfg.WorkerID
	} else {
		workerID, err = system.ResolveWorkerID(sysInfo.Hostname)
		if err != nil {
			log.Printf("⚠️  Failed to resolve persistent worker ID: %v", err)
			workerID = sysInfo.Hostname
			if workerID == "" {
				workerID = "worker-unknown"
			}
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

	monitor := telemetry.NewMonitor(workerID, constants.DefaultHeartbeatInterval)
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

	grpcServer := grpc.NewServer()
	workerServer, err := server.NewWorkerServer(workerID, monitor)
	if err != nil {
		log.Fatalf("Failed to create worker server: %v", err)
	}
	pb.RegisterMasterWorkerServer(grpcServer, workerServer)

	listener, err := net.Listen("tcp", workerBindIP+workerPort)
	if err != nil {
		log.Fatalf("Failed to listen on %s%s: %v", workerBindIP, workerPort, err)
	}

	go func() {
		log.Printf("✓ Worker gRPC server listening on %s%s", workerBindIP, workerPort)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutdown signal received, gracefully terminating...")
	grpcServer.GracefulStop()
	cancel()
	log.Println("✓ Worker shutdown complete.")
}
