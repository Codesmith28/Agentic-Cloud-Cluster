package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"master/internal/aod"
	"master/internal/cli"
	"master/internal/config"
	"master/internal/controlplane"
	"master/internal/db"
	httpserver "master/internal/http"
	"master/internal/scheduler"
	"master/internal/server"
	"master/internal/storage"
	"master/internal/system"
	"master/internal/telemetry"
	"master/internal/testworkflow"
	"master/internal/tui"
	pb "master/proto"

	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Handle "test" subcommand before flag parsing
	if len(os.Args) > 1 && strings.EqualFold(os.Args[1], "test") {
		os.Exit(runNonInteractiveTestCommand(cfg, os.Args[2:]))
	}

	// Parse --mode flag for UI mode selection
	var modeFlag string
	for i, arg := range os.Args[1:] {
		if arg == "--mode" && i+2 < len(os.Args) {
			modeFlag = os.Args[i+2]
		} else if strings.HasPrefix(arg, "--mode=") {
			modeFlag = strings.TrimPrefix(arg, "--mode=")
		}
	}
	uiMode := cfg.ResolveUIMode(modeFlag)

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
	var attemptDB *db.AttemptDB
	var resultDB *db.ResultDB
	var fileMetadataDB *db.FileMetadataDB
	var schedulerModelDB *db.SchedulerModelDB
	var userDB *db.UserDB
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

		// Create attempt database handler
		attemptDB, err = db.NewAttemptDB(ctx, cfg)
		if err != nil {
			log.Printf("Warning: Failed to create AttemptDB: %v", err)
			attemptDB = nil
		} else {
			log.Println("✓ AttemptDB initialized")
			defer attemptDB.Close(context.Background())
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

		// Create user database handler
		userDB, err = db.NewUserDB(ctx, cfg)
		if err != nil {
			log.Printf("Warning: Failed to create UserDB: %v", err)
			userDB = nil
		} else {
			log.Println("✓ UserDB initialized")
			defer userDB.Close(context.Background())
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

		// Create scheduler model persistence handler (PPO + RTS checkpoints)
		schedulerModelDB, err = db.NewSchedulerModelDB(ctx, cfg)
		if err != nil {
			log.Printf("Warning: Failed to create SchedulerModelDB: %v", err)
			schedulerModelDB = nil
		} else {
			log.Println("✓ SchedulerModelDB initialized")
			defer schedulerModelDB.Close(context.Background())
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

	paramsPath := resolveGAParamsPath(cfg.GAParamsPath)
	rtsFingerprintHash, rtsFingerprintPayload := deriveRTSFingerprintFromWorkerDB(context.Background(), workerDB)
	if schedulerModelDB != nil {
		if err := hydrateRTSParamsFromMongo(context.Background(), schedulerModelDB, rtsFingerprintHash, rtsFingerprintPayload, paramsPath); err != nil {
			log.Printf("⚠️  RTS Mongo hydration failed; using cache/default params: %v", err)
		}
	}

	// Create scheduler stack: RR -> RTS -> PPO (optional)
	rrScheduler := scheduler.NewRoundRobinScheduler()
	log.Println("✓ Round-Robin scheduler created")

	telemetrySource := scheduler.NewMasterTelemetrySource(telemetryMgr, workerDB)
	log.Println("✓ Telemetry source adapter created")

	// Initialize optional MongoDB store for RTS learned parameters.
	var rtsParamsStore scheduler.GAParamsStore
	if cfg.MongoDBURI != "" {
		mongoParamsStore, storeErr := scheduler.NewMongoGAParamsStore(ctx, cfg.MongoDBURI, cfg.MongoDBDatabase)
		if storeErr != nil {
			log.Printf("Warning: Failed to initialize RTS params MongoDB store: %v", storeErr)
			log.Println("RTS will fall back to JSON/default params only")
		} else {
			rtsParamsStore = mongoParamsStore
			log.Println("✓ RTS params store initialized (MongoDB: RTS_WEIGHTS)")
			defer mongoParamsStore.Close(context.Background())
		}
	}

	// Create RTS scheduler with Round-Robin fallback
	rtsScheduler := scheduler.NewRTSScheduler(rrScheduler, tauStore, telemetrySource, paramsPath, rtsParamsStore, slaMultiplier)
	log.Printf("✓ RTS scheduler initialized (params: %s)", paramsPath)
	log.Printf("  - Fallback: %s", rrScheduler.GetName())
	log.Printf("  - Parameter hot-reload: enabled (every 30s from local cache)")

	selectedAlgo := chooseSchedulerAlgorithm(cfg.SchedulerAlgo)
	ppoDeploymentMode := scheduler.NormalizePPODeploymentMode(cfg.PPODeploymentMode)
	activeScheduler := scheduler.Scheduler(rtsScheduler)
	var ppoScheduler *scheduler.PPOScheduler
	var ppoServiceCmd *exec.Cmd

	switch selectedAlgo {
	case "RR":
		activeScheduler = rrScheduler
		log.Printf("✓ Selected scheduler: %s", rrScheduler.GetName())
	case "PPO":
		log.Printf("✓ PPO deployment mode: %s (online updates: %t)", ppoDeploymentMode, cfg.PPOOnlineUpdates)
		if cfg.PPOAutostart && ppoDeploymentMode != scheduler.PPOModeFallback {
			cmd, err := startPPOServiceIfNeeded(cfg)
			if err != nil {
				log.Printf("⚠️  Failed to auto-start PPO service: %v", err)
			} else {
				ppoServiceCmd = cmd
				log.Printf("✓ PPO Python service auto-started (pid=%d)", cmd.Process.Pid)
				// Wait for the Python process to finish importing and open the gRPC port
				// before we attempt to dial. Cold Python+torch startup takes ~3-5s.
				log.Printf("  Waiting for PPO service port to open...")
				if waitErr := waitForTCPPort(cfg.PPOGRPCAddr, 15*time.Second); waitErr != nil {
					log.Printf("⚠️  PPO service port did not open in time: %v", waitErr)
				}
			}
		} else if cfg.PPOAutostart && ppoDeploymentMode == scheduler.PPOModeFallback {
			log.Println("ℹ️  PPO autostart skipped in fallback deployment mode")
		}

		ppoSchedulerCandidate, err := scheduler.NewPPOScheduler(
			cfg.PPOGRPCAddr,
			time.Duration(cfg.PPORequestTimeoutMS)*time.Millisecond,
			rtsScheduler,
			cfg.PPOModelPath,
			ppoDeploymentMode,
			cfg.PPOOnlineUpdates,
		)
		if err != nil {
			log.Printf("⚠️  Failed to initialize PPO scheduler: %v", err)
			log.Printf("  Falling back to %s scheduler", rtsScheduler.GetName())
			if ppoServiceCmd != nil {
				stopExternalProcess(ppoServiceCmd, 3*time.Second)
				ppoServiceCmd = nil
			}
			activeScheduler = rtsScheduler
		} else {
			ppoScheduler = ppoSchedulerCandidate
			if ppoScheduler.DeploymentMode() != scheduler.PPOModeFallback {
				if err := waitForPPOHealth(ppoScheduler, 10*time.Second); err != nil {
					log.Printf("⚠️  PPO health check failed: %v", err)
					log.Printf("  Falling back to %s scheduler", rtsScheduler.GetName())
					_ = ppoScheduler.Close()
					ppoScheduler = nil
					if ppoServiceCmd != nil {
						stopExternalProcess(ppoServiceCmd, 3*time.Second)
						ppoServiceCmd = nil
					}
					activeScheduler = rtsScheduler
				} else {
					activeScheduler = ppoScheduler
					log.Printf(
						"✓ Selected scheduler: %s (mode=%s fallback=%s)",
						ppoScheduler.GetName(),
						ppoScheduler.DeploymentMode(),
						rtsScheduler.GetName(),
					)
				}
			} else {
				activeScheduler = ppoScheduler
				log.Printf(
					"✓ Selected scheduler: %s (mode=%s fallback=%s)",
					ppoScheduler.GetName(),
					ppoScheduler.DeploymentMode(),
					rtsScheduler.GetName(),
				)
			}
		}
	default:
		activeScheduler = rtsScheduler
		log.Printf("✓ Selected scheduler: %s", rtsScheduler.GetName())
	}

	masterServer := server.NewMasterServer(workerDB, taskDB, assignmentDB, attemptDB, resultDB, fileMetadataDB, fileStorage, telemetryMgr)
	masterServer.SetScheduler(activeScheduler)
	log.Printf("✓ Master server configured with %s scheduler", activeScheduler.GetName())

	// Set master info
	masterID := "master-1"
	grpcBindAddress, advertisedMasterAddress := resolveMasterAddresses(sysInfo, cfg)
	masterServer.SetMasterInfo(masterID, advertisedMasterAddress)

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

	// Start AOD training ticker for parameter optimization
	persistRTSParams := func(ctx context.Context, params scheduler.GAParams) error {
		if schedulerModelDB == nil {
			return nil
		}

		encoded, err := json.MarshalIndent(params, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal RTS params: %w", err)
		}

		hash, payload := deriveRTSFingerprintFromWorkerDB(ctx, workerDB)
		_, err = schedulerModelDB.SaveAndActivateModel(
			ctx,
			"RTS",
			hash,
			payload,
			encoded,
			"go-aod",
			map[string]any{
				"source":      "aod-training",
				"params_path": paramsPath,
			},
		)
		if err != nil {
			return fmt.Errorf("save RTS params to Mongo: %w", err)
		}
		return nil
	}

	if historyDB != nil {
		// Start AOD training ticker (runs every 60 seconds)
		aodTrainingInterval := 60 * time.Second
		go func() {
			ticker := time.NewTicker(aodTrainingInterval)
			defer ticker.Stop()

			log.Printf("✓ AOD training ticker started (interval: %s)", aodTrainingInterval)
			log.Printf("  - Training method: Linear regression (Theta) + Direct computation (Affinity/Penalty)")
			log.Printf("  - Training data window: 24 hours")
			log.Printf("  - Output: %s", paramsPath)
			if rtsParamsStore != nil {
				log.Printf("  - Persistent store: MongoDB collection RTS_WEIGHTS")
			}
			log.Printf("  - RTS hot-reload: every 30s")

			for range ticker.C {
				log.Println("🧬 Starting AOD training cycle...")
				if err := aod.RunTraining(context.Background(), historyDB, paramsPath, rtsParamsStore, persistRTSParams); err != nil {
					log.Printf("❌ AOD training error: %v", err)
				} else {
					log.Println("✅ AOD training cycle completed successfully")
				}
			}
		}()
	} else {
		log.Println("⚠️  AOD training disabled (HistoryDB not available)")
		log.Printf("  - RTS will use default parameters from %s", paramsPath)
	}

	// Load workers from database
	if workerDB != nil {
		if err := masterServer.LoadWorkersFromDB(ctx); err != nil {
			log.Printf("Warning: Failed to load workers from DB: %v", err)
		}
	}

	// Restore queued tasks from database before starting the processor.
	if err := masterServer.RestoreQueuedTasks(ctx); err != nil {
		log.Printf("Warning: Failed to restore queued tasks: %v", err)
	}

	// Start task queue processor
	masterServer.StartQueueProcessor()
	log.Println("✓ Task queue processor started")

	// Start worker reconnection monitor
	masterServer.StartWorkerReconnectionMonitor()
	log.Println("✓ Worker reconnection monitor started")

	// Start gRPC server in background
	grpcServer := grpc.NewServer()
	pb.RegisterMasterWorkerServer(grpcServer, masterServer)
	go startGRPCServer(grpcServer, grpcBindAddress)

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
		taskHandler := httpserver.NewTaskAPIHandler(masterServer, taskDB, assignmentDB, attemptDB, resultDB)
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

		// Register auth handlers if user database is available
		if userDB != nil {
			authHandler := httpserver.NewAuthHandler(userDB)
			httpTelemetryServer.RegisterAuthHandlers(authHandler)
			log.Println("✓ Auth API handlers registered")
		}

		// Register scheduler switch API (POST/GET /api/config/scheduler)
		schedulerEntries := map[string]scheduler.Scheduler{
			"RR":  rrScheduler,
			"RTS": rtsScheduler,
		}
		if ppoScheduler != nil {
			schedulerEntries["PPO"] = ppoScheduler
		}
		schedulerRegistry := httpserver.NewSchedulerRegistry(schedulerEntries)
		schedulerHandler := httpserver.NewSchedulerSwitchHandler(masterServer, schedulerRegistry)
		httpTelemetryServer.RegisterSchedulerHandler(schedulerHandler)
		log.Printf("✓ Scheduler switch API registered (available: %v)", schedulerRegistry.Available())

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
		log.Printf("  - Config: POST/GET /api/config/scheduler")
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
		if ppoScheduler != nil {
			log.Println("⏹️  Shutting down PPO scheduler client...")
			if err := ppoScheduler.Close(); err != nil {
				log.Printf("Warning: failed to close PPO scheduler client: %v", err)
			}
		}
		if ppoServiceCmd != nil {
			stopExternalProcess(ppoServiceCmd, 5*time.Second)
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
	masterServer.BroadcastMasterRegistration(masterID, advertisedMasterAddress)

	// Start CLI interface
	log.Println("\n✓ Master node started successfully")
	log.Printf("✓ Starting gRPC server on %s (advertise: %s)\n", grpcBindAddress, advertisedMasterAddress)

	if cfg.Headless {
		log.Println("✓ Headless mode enabled (CLOUDAI_HEADLESS=true), CLI disabled")
		select {}
	}

	// Create host resource sampler and shared executor
	hostSampler := system.NewHostResourceSampler(2 * time.Second)
	defer hostSampler.Stop()
	exec := controlplane.NewExecutor(masterServer, fileStorage, hostSampler)

	switch uiMode {
	case "tui":
		log.Println("✓ Starting TUI mode")
		m := tui.New(exec)
		p := tui.NewProgram(m)
		if _, err := p.Run(); err != nil {
			log.Fatalf("TUI error: %v", err)
		}
	default:
		cliInterface := cli.NewCLI(masterServer, fileStorage, exec)
		cliInterface.Run()
	}
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

func runNonInteractiveTestCommand(cfg *config.Config, args []string) int {
	engine := testworkflow.NewEngine()

	if len(args) == 0 {
		printTestUsage()
		return 2
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "list":
		fmt.Println("Available test suites:")
		for _, suite := range engine.ListSuites() {
			fmt.Printf("  - %s: %s\n", suite.Name, suite.Description)
		}
		return 0
	case "cleanup":
		projectRoot := detectProjectRoot()
		if projectRoot == "" {
			log.Printf("Unable to locate project root for test workflow command")
			return 2
		}
		cleanupOpts := testworkflow.CleanupOptions{
			RepoRoot:    projectRoot,
			ComposeFile: resolveTestComposeFile(),
			Profile:     "hetero-small",
			Stdout:      os.Stdout,
			Stderr:      os.Stderr,
		}
		if err := engine.Cleanup(context.Background(), cleanupOpts); err != nil {
			log.Printf("Test cleanup failed: %v", err)
			return 1
		}
		fmt.Println("Test workflow environment cleaned up")
		return 0
	case "run":
		if len(args) < 2 {
			printTestUsage()
			return 2
		}
		suite := strings.ToLower(strings.TrimSpace(args[1]))
		if !isSupportedTestSuite(suite) {
			log.Printf("Unsupported test suite: %q", args[1])
			printTestUsage()
			return 2
		}
		projectRoot := detectProjectRoot()
		if projectRoot == "" {
			log.Printf("Unable to locate project root for test workflow command")
			return 2
		}
		runOpts, err := parseNonInteractiveTestRunOptions(args[2:])
		if err != nil {
			log.Printf("Invalid test options: %v", err)
			return 2
		}
		runOpts.RepoRoot = projectRoot
		runOpts.MasterURL = resolveMasterHTTPURL(cfg.HTTPPort)
		runOpts.PrometheusURL = strings.TrimSpace(os.Getenv("PROMETHEUS_URL"))
		runOpts.ComposeFile = resolveTestComposeFile()
		runOpts.Stdout = os.Stdout
		runOpts.Stderr = os.Stderr
		runOpts.ExtraEnv = defaultTestExtraEnv(runOpts.ComposeFile)
		runOpts.Mode = testworkflow.ModeNonInteractive

		requestedScheduler := strings.ToLower(strings.TrimSpace(runOpts.Scheduler))
		if strings.EqualFold(suite, "evidence") && (requestedScheduler == "" || requestedScheduler == "current") {
			return runNonInteractiveEvidenceMatrix(engine, cfg, runOpts)
		}
		runOpts.Scheduler = resolveRunScheduler(runOpts.Scheduler, normalizeSchedulerAlgorithm(cfg.SchedulerAlgo))

		stop, err := startHeadlessTestMaster(projectRoot, runOpts.Scheduler, cfg)
		if err != nil {
			log.Printf("Failed to start headless master for test run: %v", err)
			return 1
		}
		if !runOpts.KeepEnvironment {
			defer func() {
				if stopErr := stop(); stopErr != nil {
					log.Printf("Warning: failed to stop headless master: %v", stopErr)
				}
			}()
		}

		result, err := engine.RunSuite(context.Background(), suite, runOpts)
		if result != nil {
			fmt.Printf("Suite: %s\n", result.Suite)
			fmt.Printf("Artifacts: %s\n", result.OutputDir)
		}
		if err != nil {
			log.Printf("Test suite failed: %v", err)
			return 1
		}
		fmt.Printf("Suite %s completed successfully\n", suite)
		return 0
	default:
		printTestUsage()
		return 2
	}
}

func runNonInteractiveEvidenceMatrix(engine *testworkflow.Engine, cfg *config.Config, baseOpts testworkflow.RunOptions) int {
	baseOut := strings.TrimSpace(baseOpts.OutputDir)
	if baseOut == "" {
		ts := time.Now().UTC().Format("20060102-150405")
		baseOut = filepath.Join(baseOpts.RepoRoot, "results", "testbench", ts+"-evidence")
	}
	if !filepath.IsAbs(baseOut) {
		baseOut = filepath.Join(baseOpts.RepoRoot, baseOut)
	}
	if err := os.MkdirAll(baseOut, 0o755); err != nil {
		log.Printf("Failed to create evidence output root: %v", err)
		return 1
	}

	type matrixResult struct {
		Scheduler string `json:"scheduler"`
		OutputDir string `json:"output_dir"`
		Success   bool   `json:"success"`
		Error     string `json:"error,omitempty"`
	}
	results := make([]matrixResult, 0, 2)
	for _, scheduler := range []string{"rr", "rts"} {
		runOpts := baseOpts
		runOpts.Scheduler = scheduler
		runOpts.OutputDir = filepath.Join(baseOut, strings.ToUpper(scheduler))

		stop, err := startHeadlessTestMaster(baseOpts.RepoRoot, scheduler, cfg)
		if err != nil {
			log.Printf("Failed to start headless master (%s): %v", strings.ToUpper(scheduler), err)
			results = append(results, matrixResult{
				Scheduler: strings.ToUpper(scheduler),
				OutputDir: runOpts.OutputDir,
				Success:   false,
				Error:     err.Error(),
			})
			break
		}

		runResult, runErr := engine.RunSuite(context.Background(), "evidence", runOpts)
		stopErr := stop()
		if stopErr != nil {
			log.Printf("Warning: failed to stop headless master (%s): %v", strings.ToUpper(scheduler), stopErr)
		}

		row := matrixResult{
			Scheduler: strings.ToUpper(scheduler),
			OutputDir: runOpts.OutputDir,
			Success:   runErr == nil,
		}
		if runErr != nil {
			row.Error = runErr.Error()
		}
		if runResult != nil {
			row.OutputDir = runResult.OutputDir
		}
		results = append(results, row)
		if runErr != nil {
			break
		}
	}

	summaryPath := filepath.Join(baseOut, "summary.json")
	summaryPayload := map[string]any{
		"suite":   "evidence",
		"success": true,
		"runs":    results,
	}
	for _, row := range results {
		if !row.Success {
			summaryPayload["success"] = false
			break
		}
	}
	if payload, err := json.MarshalIndent(summaryPayload, "", "  "); err == nil {
		_ = os.WriteFile(summaryPath, payload, 0o644)
	}

	fmt.Printf("Evidence artifacts: %s\n", baseOut)
	if ok, _ := summaryPayload["success"].(bool); ok {
		return 0
	}
	return 1
}

func parseNonInteractiveTestRunOptions(args []string) (testworkflow.RunOptions, error) {
	opts := testworkflow.RunOptions{
		Profile:   "hetero-small",
		Scheduler: "current",
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-profile":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("-profile requires a value")
			}
			opts.Profile = args[i+1]
			i++
		case "-out":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("-out requires a value")
			}
			opts.OutputDir = args[i+1]
			i++
		case "-keep-env":
			opts.KeepEnvironment = true
		case "-ui-smoke":
			opts.EnableUISmoke = true
		case "-scheduler":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("-scheduler requires current|RR|RTS")
			}
			switch strings.ToLower(strings.TrimSpace(args[i+1])) {
			case "current", "rr", "rts":
				opts.Scheduler = strings.ToLower(strings.TrimSpace(args[i+1]))
			default:
				return opts, fmt.Errorf("-scheduler requires current|RR|RTS")
			}
			i++
		default:
			return opts, fmt.Errorf("unknown option %s", args[i])
		}
	}
	return opts, nil
}

func isSupportedTestSuite(raw string) bool {
	suite := strings.ToLower(strings.TrimSpace(raw))
	for _, candidate := range testworkflow.ListSuiteNames() {
		if suite == strings.ToLower(candidate) {
			return true
		}
	}
	return false
}

func printTestUsage() {
	fmt.Println("Usage:")
	fmt.Println("  ./masterNode test list")
	fmt.Println("  ./masterNode test run <smoke|reliability|ui-smoke|evidence|full> [-profile <hetero-small|recovery-lab>] [-out <dir>] [-keep-env] [-ui-smoke] [-scheduler <current|RR|RTS>]")
	fmt.Println("  ./masterNode test cleanup")
}

func resolveMasterHTTPURL(httpPort string) string {
	port := strings.TrimSpace(httpPort)
	if port == "" {
		port = ":8080"
	}
	if strings.HasPrefix(port, "http://") || strings.HasPrefix(port, "https://") {
		return strings.TrimRight(port, "/")
	}
	if strings.HasPrefix(port, ":") {
		return "http://localhost" + port
	}
	if strings.Contains(port, ":") {
		return "http://" + port
	}
	return "http://localhost:" + port
}

func resolveRunScheduler(rawFlag string, activeScheduler string) string {
	flag := strings.ToLower(strings.TrimSpace(rawFlag))
	if flag != "" && flag != "current" {
		return flag
	}

	active := strings.ToLower(strings.TrimSpace(activeScheduler))
	switch {
	case strings.Contains(active, "rr"):
		return "rr"
	case strings.Contains(active, "rts"):
		return "rts"
	default:
		return "current"
	}
}

func resolveTestComposeFile() string {
	value := strings.TrimSpace(os.Getenv("TESTBENCH_COMPOSE_FILE"))
	if value != "" {
		return value
	}
	return "testbench/docker-compose.host-master.yml"
}

func defaultTestExtraEnv(composeFile string) map[string]string {
	if !strings.Contains(strings.ToLower(composeFile), "host-master") {
		return nil
	}
	return map[string]string{
		"WORKER_SPECS": "worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054",
	}
}

func startHeadlessTestMaster(projectRoot string, scheduler string, cfg *config.Config) (func() error, error) {
	binaryPath := filepath.Join(projectRoot, "master", "masterNode")
	if _, err := os.Stat(binaryPath); err != nil {
		return nil, fmt.Errorf("master binary missing at %s (run make master): %w", binaryPath, err)
	}

	cmd := exec.Command(binaryPath)
	cmd.Dir = filepath.Join(projectRoot, "master")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env, "CLOUDAI_HEADLESS=true")

	if scheduler != "" && scheduler != "current" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SCHED_ALGO=%s", strings.ToUpper(scheduler)))
	}

	bindAddr := strings.TrimSpace(cfg.MasterBindAddr)
	if bindAddr == "" {
		bindAddr = ":50051"
	}
	advAddr := strings.TrimSpace(cfg.MasterAdvAddr)
	if advAddr == "" {
		advAddr = "localhost:50051"
	}
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("MASTER_BIND_ADDR=%s", bindAddr),
		fmt.Sprintf("MASTER_ADVERTISE_ADDR=%s", advAddr),
	)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	masterURL := resolveMasterHTTPURL(cfg.HTTPPort)
	if err := waitForHTTPHealth(masterURL+"/health", 45*time.Second); err != nil {
		_ = stopExternalProcessSync(cmd, 3*time.Second)
		return nil, err
	}

	return func() error {
		return stopExternalProcessSync(cmd, 5*time.Second)
	}, nil
}

func waitForHTTPHealth(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url) // #nosec G107: URL controlled by local config.
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("master health check did not become ready at %s", url)
}

func stopExternalProcessSync(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return nil
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func resolveMasterAddresses(sysInfo *system.SystemInfo, cfg *config.Config) (string, string) {
	defaultAddress := sysInfo.GetMasterAddress() + cfg.GRPCPort
	bindAddress := defaultAddress
	if cfg.MasterBindAddr != "" {
		bindAddress = strings.TrimSpace(cfg.MasterBindAddr)
	}
	advertiseAddress := defaultAddress
	if cfg.MasterAdvAddr != "" {
		advertiseAddress = strings.TrimSpace(cfg.MasterAdvAddr)
	}
	return bindAddress, advertiseAddress
}

func resolveGAParamsPath(configPath string) string {
	if configPath == "" {
		configPath = "config/ga_output.json"
	}
	if filepath.IsAbs(configPath) {
		return configPath
	}

	exePath, err := os.Executable()
	if err == nil {
		return filepath.Clean(filepath.Join(filepath.Dir(exePath), configPath))
	}

	absPath, err := filepath.Abs(configPath)
	if err == nil {
		return absPath
	}

	return configPath
}

func chooseSchedulerAlgorithm(configValue string) string {
	defaultAlgo := normalizeSchedulerAlgorithm(configValue)

	// Honor explicit environment override in non-interactive environments.
	if envValue := strings.TrimSpace(os.Getenv("SCHED_ALGO")); envValue != "" {
		return normalizeSchedulerAlgorithm(envValue)
	}
	if !isInteractiveTerminal() {
		return defaultAlgo
	}

	fmt.Println("\nScheduler Selection")
	fmt.Println("  1. Round-Robin (RR)")
	fmt.Println("  2. RTS (risk-aware)")
	fmt.Println("  3. PPO (Python gRPC)")
	fmt.Printf("Choose scheduler [default: %s]: ", defaultAlgo)

	reader := bufio.NewReader(os.Stdin)
	choice, err := reader.ReadString('\n')
	if err != nil {
		return defaultAlgo
	}
	choice = strings.TrimSpace(strings.ToUpper(choice))

	switch choice {
	case "1", "RR", "ROUND-ROBIN", "ROUNDROBIN":
		return "RR"
	case "2", "RTS":
		return "RTS"
	case "3", "PPO":
		return "PPO"
	default:
		return defaultAlgo
	}
}

func normalizeSchedulerAlgorithm(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "RR", "ROUND-ROBIN", "ROUNDROBIN":
		return "RR"
	case "PPO":
		return "PPO"
	default:
		return "RTS"
	}
}

func isInteractiveTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// waitForTCPPort polls addr (host:port) until the port accepts connections or timeout.
func waitForTCPPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("port %s not open after %s", addr, timeout)
}

func waitForPPOHealth(ppoScheduler *scheduler.PPOScheduler, timeout time.Duration) error {
	if ppoScheduler == nil {
		return fmt.Errorf("nil PPO scheduler")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)

	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
		resp, err := ppoScheduler.Ping(ctx)
		cancel()
		if err == nil && resp != nil && resp.Healthy {
			log.Printf("✓ PPO service healthy (model=%s, fingerprint=%s)", resp.ModelVersion, resp.FingerprintHash)
			return nil
		}
		if err != nil {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout waiting for healthy PPO service")
	}
	return lastErr
}

func startPPOServiceIfNeeded(cfg *config.Config) (*exec.Cmd, error) {
	projectRoot := detectProjectRoot()
	if projectRoot == "" {
		return nil, fmt.Errorf("unable to locate project root for PPO autostart")
	}

	modelPath := cfg.PPOModelPath
	if modelPath == "" {
		modelPath = "latest"
	}

	// Prefer the project venv Python (has grpc, torch, etc.)
	pythonBin := "python3"
	venvPython := filepath.Join(projectRoot, "venv", "bin", "python3")
	if _, err := os.Stat(venvPython); err == nil {
		pythonBin = venvPython
		log.Printf("Using venv Python for PPO service: %s", venvPython)
	}

	cmd := exec.Command(
		pythonBin,
		"-m",
		"agentic_scheduler.server",
		"--grpc-addr", cfg.PPOGRPCAddr,
		"--mongo-uri", cfg.MongoDBURI,
		"--mongo-db", cfg.MongoDBDatabase,
		"--model-path", modelPath,
		"--online-updates", fmt.Sprintf("%t", cfg.PPOOnlineUpdates),
	)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env := os.Environ()
	pythonPath := mergePythonPath(os.Getenv("PYTHONPATH"), projectRoot)
	env = append(env, fmt.Sprintf("PYTHONPATH=%s", pythonPath))
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start PPO python service: %w", err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("⚠️  PPO python service exited: %v", err)
		} else {
			log.Printf("ℹ️  PPO python service exited")
		}
	}()
	return cmd, nil
}

func stopExternalProcess(cmd *exec.Cmd, timeout time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
}

func detectProjectRoot() string {
	candidates := make([]string, 0, 6)
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd, filepath.Dir(cwd))
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, exeDir, filepath.Dir(exeDir), filepath.Dir(filepath.Dir(exeDir)))
	}

	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if hasProjectLayout(candidate) {
			return candidate
		}
	}
	return ""
}

func hasProjectLayout(root string) bool {
	required := []string{
		filepath.Join(root, "master"),
		filepath.Join(root, "proto"),
		filepath.Join(root, "agentic_scheduler"),
	}
	for _, path := range required {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

func mergePythonPath(existing string, prepend string) string {
	if existing == "" {
		return prepend
	}
	parts := strings.Split(existing, string(os.PathListSeparator))
	for _, part := range parts {
		if part == prepend {
			return existing
		}
	}
	return prepend + string(os.PathListSeparator) + existing
}

func deriveRTSFingerprintFromWorkerDB(ctx context.Context, workerDB *db.WorkerDB) (string, string) {
	workers := make(map[string]*scheduler.WorkerInfo)
	if workerDB != nil {
		docs, err := workerDB.GetAllWorkers(ctx)
		if err != nil {
			log.Printf("⚠️  Failed to read workers for RTS fingerprint: %v", err)
		} else {
			for _, doc := range docs {
				workers[doc.WorkerID] = &scheduler.WorkerInfo{
					WorkerID:     doc.WorkerID,
					TotalCPU:     doc.TotalCPU,
					TotalMemory:  doc.TotalMemory,
					TotalStorage: doc.TotalStorage,
				}
			}
		}
	}
	return scheduler.BuildClusterFingerprint(workers)
}

func hydrateRTSParamsFromMongo(
	ctx context.Context,
	schedulerModelDB *db.SchedulerModelDB,
	fingerprintHash string,
	fingerprintPayload string,
	paramsPath string,
) error {
	if schedulerModelDB == nil {
		return nil
	}

	payload, metadata, err := schedulerModelDB.LoadActiveModel(ctx, "RTS", fingerprintHash)
	if err == nil && len(payload) > 0 {
		if err := writeFileAtomic(paramsPath, payload, 0644); err != nil {
			return fmt.Errorf("write RTS cache from Mongo: %w", err)
		}
		version := int64(0)
		if metadata != nil {
			version = metadata.Version
		}
		log.Printf("✓ RTS params loaded from Mongo (version v%d, fingerprint=%s)", version, fingerprintHash)
		return nil
	}

	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("load RTS params from Mongo: %w", err)
	}

	// No active Mongo checkpoint for this fingerprint. Migrate local cache if present.
	cachePayload, readErr := os.ReadFile(paramsPath)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read local RTS cache for migration: %w", readErr)
	}

	_, saveErr := schedulerModelDB.SaveAndActivateModel(
		ctx,
		"RTS",
		fingerprintHash,
		fingerprintPayload,
		cachePayload,
		"go-rts-cache",
		map[string]any{
			"source":      "startup-file-migration",
			"params_path": paramsPath,
		},
	)
	if saveErr != nil {
		return fmt.Errorf("migrate local RTS cache to Mongo: %w", saveErr)
	}

	log.Printf("✓ Migrated local RTS cache to Mongo for fingerprint=%s", fingerprintHash)
	return nil
}

func writeFileAtomic(path string, payload []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	tempFile, err := os.CreateTemp(dir, "scheduler_cache_*.tmp")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(payload); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}
