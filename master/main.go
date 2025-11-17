package main

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

	"master/internal/aod"
	"master/internal/cli"
	"master/internal/config"
	"master/internal/db"
	httpserver "master/internal/http"
	"master/internal/scheduler"
	"master/internal/server"
	"master/internal/storage"
	"master/internal/system"
	"master/internal/telemetry"
	pb "master/proto"

	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Determine file storage base directory with fallback
	fileStorageBaseDir := "/var/cloudai/files"
	if err := os.MkdirAll(fileStorageBaseDir, 0700); err != nil {
		// If /var/cloudai/files fails (permission denied), fallback to ~/.cloudai/files
		log.Printf("Warning: Cannot create %s: %v", fileStorageBaseDir, err)
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get home directory: %v", err)
		}
		fileStorageBaseDir = filepath.Join(homeDir, ".cloudai", "files")
		if err := os.MkdirAll(fileStorageBaseDir, 0700); err != nil {
			log.Fatalf("Failed to create fallback directory %s: %v", fileStorageBaseDir, err)
		}
		log.Printf("✓ Using fallback file storage directory: %s", fileStorageBaseDir)
	} else {
		log.Printf("✓ File storage directory ready (secure): %s", fileStorageBaseDir)
	}

	// Set environment variable for file storage components
	os.Setenv("CLOUDAI_FILES_DIR", fileStorageBaseDir)

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
	var taskDB *db.TaskDB
	var assignmentDB *db.AssignmentDB
	var resultDB *db.ResultDB
	var fileMetadataDB *db.FileMetadataDB
	var fileStorage *storage.FileStorageService

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

		// Create task database handler
		taskDB, err = db.NewTaskDB(ctx, cfg)
		if err != nil {
			log.Printf("Warning: Failed to create TaskDB: %v", err)
			taskDB = nil
		} else {
			log.Println("✓ TaskDB initialized")
			defer taskDB.Close(context.Background())
		}

		// Create assignment database handler
		assignmentDB, err = db.NewAssignmentDB(ctx, cfg)
		if err != nil {
			log.Printf("Warning: Failed to create AssignmentDB: %v", err)
			assignmentDB = nil
		} else {
			log.Println("✓ AssignmentDB initialized")
			defer assignmentDB.Close(context.Background())
		}

		// Create result database handler
		resultDB, err = db.NewResultDB(ctx, cfg)
		if err != nil {
			log.Printf("Warning: Failed to create ResultDB: %v", err)
			resultDB = nil
		} else {
			log.Println("✓ ResultDB initialized")
			defer resultDB.Close(context.Background())
		}

		// Create file metadata database handler
		fileMetadataDB, err = db.NewFileMetadataDB(ctx, cfg)
		if err != nil {
			log.Printf("Warning: Failed to create FileMetadataDB: %v", err)
			fileMetadataDB = nil
		} else {
			log.Println("✓ FileMetadataDB initialized")
			defer fileMetadataDB.Close(context.Background())
		}
	}

	// Initialize file storage service
	fileStorage, err = storage.NewFileStorageService(fileStorageBaseDir)
	if err != nil {
		log.Printf("Warning: Failed to create FileStorageService: %v", err)
		log.Println("Continuing without file storage...")
		fileStorage = nil
	} else {
		log.Printf("✓ FileStorageService initialized (base: %s)", fileStorageBaseDir)
		defer fileStorage.Close()
	}

	// Create master server
	// Initialize telemetry manager (30 second inactivity timeout)
	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telemetryMgr.Start()
	log.Println("✓ Telemetry manager started")

	// Initialize tau store for runtime learning
	tauStore := telemetry.NewInMemoryTauStore()
	log.Println("✓ Tau store initialized with default values:")
	for taskType, tau := range tauStore.GetAllTau() {
		log.Printf("  - %s: %.1fs", taskType, tau)
	}

	// Load SLA multiplier from environment or use default
	slaMultiplier := 2.0
	if cfg.SLAMultiplier > 0 {
		slaMultiplier = cfg.SLAMultiplier
	}
	log.Printf("✓ SLA multiplier (k): %.1f", slaMultiplier)

	// Create scheduler with RTS (Risk-aware Task Scheduling)
	// Create Round-Robin as fallback
	rrScheduler := scheduler.NewRoundRobinScheduler()
	log.Println("✓ Round-Robin scheduler created (fallback)")

	// Create telemetry source adapter for RTS
	telemetrySource := scheduler.NewMasterTelemetrySource(telemetryMgr, workerDB)
	log.Println("✓ Telemetry source adapter created")

	// Create RTS scheduler with Round-Robin fallback
	paramsPath := "config/ga_output.json"
	rtsScheduler := scheduler.NewRTSScheduler(rrScheduler, tauStore, telemetrySource, paramsPath, slaMultiplier)
	log.Printf("✓ RTS scheduler initialized (params: %s)", paramsPath)
	log.Printf("  - Scheduler: %s", rtsScheduler.GetName())
	log.Printf("  - Fallback: Round-Robin")
	log.Printf("  - Parameter hot-reload: enabled (every 30s)")

	masterServer := server.NewMasterServer(workerDB, taskDB, assignmentDB, resultDB, fileMetadataDB, fileStorage, telemetryMgr)
	masterServer.SetScheduler(rtsScheduler)
	log.Printf("✓ Master server configured with %s scheduler", rtsScheduler.GetName())

	// Set master info
	masterID := "master-1"
	masterAddress := sysInfo.GetMasterAddress() + cfg.GRPCPort
	masterServer.SetMasterInfo(masterID, masterAddress)

	// Start task queue processor
	masterServer.StartQueueProcessor()
	log.Println("✓ Task queue processor started")

	// Initialize HistoryDB for AOD/GA training
	var historyDB *db.HistoryDB
	if cfg.MongoDBURI != "" {
		historyDB, err = db.NewHistoryDB(ctx, cfg)
		if err != nil {
			log.Printf("Warning: Failed to create HistoryDB: %v", err)
			log.Println("AOD/GA training will be disabled")
			historyDB = nil
		} else {
			log.Println("✓ HistoryDB initialized for AOD/GA training")
			defer historyDB.Close(context.Background())
		}
	}

	// Start AOD/GA epoch ticker for parameter optimization
	if historyDB != nil {
		// Get GA configuration (can be overridden via env vars in future)
		gaConfig := aod.GetDefaultGAConfig()
		log.Printf("✓ GA configuration loaded:")
		log.Printf("  - Population size: %d", gaConfig.PopulationSize)
		log.Printf("  - Generations: %d", gaConfig.Generations)
		log.Printf("  - Mutation rate: %.2f", gaConfig.MutationRate)
		log.Printf("  - Crossover rate: %.2f", gaConfig.CrossoverRate)
		log.Printf("  - Elitism count: %d", gaConfig.ElitismCount)
		log.Printf("  - Tournament size: %d", gaConfig.TournamentSize)

		// Start GA epoch ticker (runs every 60 seconds)
		gaEpochInterval := 60 * time.Second
		go func() {
			ticker := time.NewTicker(gaEpochInterval)
			defer ticker.Stop()

			log.Printf("✓ AOD/GA epoch ticker started (interval: %s)", gaEpochInterval)
			log.Printf("  - Training data window: 24 hours")
			log.Printf("  - Output: %s", paramsPath)
			log.Printf("  - RTS hot-reload: every 30s")

			for range ticker.C {
				log.Println("🧬 Starting AOD/GA epoch...")
				if err := aod.RunGAEpoch(context.Background(), historyDB, gaConfig, paramsPath); err != nil {
					log.Printf("❌ AOD/GA epoch error: %v", err)
				} else {
					log.Println("✅ AOD/GA epoch completed successfully")
				}
			}
		}()
	} else {
		log.Println("⚠️  AOD/GA training disabled (HistoryDB not available)")
		log.Println("  - RTS will use default parameters from config/ga_output.json")
	}

	// Load workers from database
	if workerDB != nil {
		if err := masterServer.LoadWorkersFromDB(ctx); err != nil {
			log.Printf("Warning: Failed to load workers from DB: %v", err)
		}
	}

	// Start worker reconnection monitor
	masterServer.StartWorkerReconnectionMonitor()
	log.Println("✓ Worker reconnection monitor started")

	// Start gRPC server in background
	grpcServer := grpc.NewServer()
	pb.RegisterMasterWorkerServer(grpcServer, masterServer)
	go startGRPCServer(grpcServer, masterAddress)

	// Start HTTP telemetry server (optional, configurable via HTTP_PORT env var)
	var httpTelemetryServer *httpserver.TelemetryServer
	if cfg.HTTPPort != "" {
		// Parse port number from config (e.g., ":8080" -> 8080)
		port := 8080 // default
		if len(cfg.HTTPPort) > 1 && cfg.HTTPPort[0] == ':' {
			// Parse port from ":8080" format
			fmt.Sscanf(cfg.HTTPPort, ":%d", &port)
		}

		// Create telemetry server with WebSocket support
		httpTelemetryServer = httpserver.NewTelemetryServer(port, telemetryMgr)

		// Create task and worker API handlers
		taskHandler := httpserver.NewTaskAPIHandler(masterServer, taskDB, assignmentDB, resultDB)
		workerHandler := httpserver.NewWorkerAPIHandler(masterServer, workerDB, assignmentDB, telemetryMgr)

		// Add API routes
		httpTelemetryServer.RegisterTaskHandlers(taskHandler)
		httpTelemetryServer.RegisterWorkerHandlers(workerHandler)

		// Register file handlers if file storage is available
		if fileStorage != nil {
			fileHandler := httpserver.NewFileAPIHandler(fileStorage)
			httpTelemetryServer.RegisterFileHandlers(fileHandler)
			log.Println("✓ File API handlers registered")
		}

		go func() {
			if err := httpTelemetryServer.Start(); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTP API server error: %v", err)
			}
		}()
		log.Printf("✓ HTTP API server started on port %d", port)
		log.Printf("  - Telemetry: GET /health, /telemetry, /workers")
		log.Printf("  - WebSocket: WS /ws/telemetry, /ws/telemetry/{worker_id}")
		log.Printf("  - Tasks: POST/GET/DELETE /api/tasks, GET /api/tasks/{id}")
		log.Printf("  - Workers: GET /api/workers, /api/workers/{id}")
		if fileStorage != nil {
			log.Printf("  - Files: GET /api/files, /api/files/{task_id}")
			log.Printf("           GET /api/files/{task_id}/download/{file_path}")
			log.Printf("           DELETE /api/files/{task_id}")
		}
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle shutdown in background
	go func() {
		<-sigChan
		log.Println("\n\nShutting down master node...")

		// Stop queue processor
		masterServer.StopQueueProcessor()

		// Stop worker reconnection monitor
		masterServer.StopWorkerReconnectionMonitor()

		// Shutdown HTTP server
		if httpTelemetryServer != nil {
			httpTelemetryServer.Shutdown()
		}

		// Shutdown telemetry manager
		telemetryMgr.Shutdown()

		// Shutdown RTS scheduler
		if rtsScheduler != nil {
			log.Println("⏹️  Shutting down RTS scheduler...")
			rtsScheduler.Shutdown()
		}

		// Shutdown gRPC server
		grpcServer.GracefulStop()

		// Close database
		if workerDB != nil {
			workerDB.Close(context.Background())
		}
		if taskDB != nil {
			taskDB.Close(context.Background())
		}
		if assignmentDB != nil {
			assignmentDB.Close(context.Background())
		}
		if resultDB != nil {
			resultDB.Close(context.Background())
		}
		if historyDB != nil {
			historyDB.Close(context.Background())
		}

		log.Println("✓ Master node shutdown complete")
		os.Exit(0)
	}()

	// Wait briefly to ensure server is listening before contacting workers
	time.Sleep(500 * time.Millisecond)

	// Broadcast master registration to known workers so they can connect back
	masterServer.BroadcastMasterRegistration(masterID, masterAddress)

	// Start CLI interface
	log.Println("\n✓ Master node started successfully")
	log.Printf("✓ Starting gRPC server on %s\n", masterAddress)

	cliInterface := cli.NewCLI(masterServer, fileStorage)
	cliInterface.Run()
}

func startGRPCServer(grpcServer *grpc.Server, address string) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", address, err)
	}

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
