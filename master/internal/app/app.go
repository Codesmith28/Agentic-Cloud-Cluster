package app

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
	"sync"
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
	"master/internal/tui"
	pb "master/proto"

	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
)

// App manages the master node server lifecycle and component wiring.
type App struct {
	cfg *config.Config
}

// New creates a new master App instance with loaded configuration.
func New(cfg *config.Config) *App {
	if cfg == nil {
		cfg = config.LoadConfig()
	}
	return &App{cfg: cfg}
}

// Run boots up all master node subsystems and runs until shutdown.
func (a *App) Run() {
	cfg := a.cfg

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
	fileStorageBaseDir := "/var/agentic-cloud-cluster/files"
	if err := os.MkdirAll(fileStorageBaseDir, 0700); err != nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get home directory: %v", err)
		}
		fileStorageBaseDir = filepath.Join(homeDir, ".agentic-cloud-cluster", "files")
		if err := os.MkdirAll(fileStorageBaseDir, 0700); err != nil {
			log.Fatalf("Failed to create fallback directory %s: %v", fileStorageBaseDir, err)
		}
		log.Printf("✓ Using fallback file storage directory: %s", fileStorageBaseDir)
	} else {
		log.Printf("✓ File storage directory ready (secure): %s", fileStorageBaseDir)
	}

	os.Setenv("AGENTIC_FILES_DIR", fileStorageBaseDir)

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

	var mongoStore *db.MongoStore
	var workerDB *db.WorkerDB
	var taskDB *db.TaskDB
	var assignmentDB *db.AssignmentDB
	var attemptDB *db.AttemptDB
	var resultDB *db.ResultDB
	var fileMetadataDB *db.FileMetadataDB
	var schedulerModelDB *db.SchedulerModelDB
	var userDB *db.UserDB
	var fileStorage *storage.FileStorageService

	if cfg.MongoDBURI != "" {
		var err error
		mongoStore, err = db.NewMongoStore(ctx, cfg.MongoDBURI, cfg.MongoDBDatabase)
		if err != nil {
			log.Printf("Warning: MongoDB connection failed: %v", err)
			log.Println("Continuing without database persistence...")
		} else {
			log.Println("✓ MongoDB connected (single connection pool)")
			defer mongoStore.Close(context.Background())

			if err := db.EnsureCollections(ctx, mongoStore); err != nil {
				log.Printf("Warning: Failed to ensure MongoDB collections: %v", err)
			} else {
				log.Println("✓ MongoDB collections ensured")
			}

			workerDB = db.NewWorkerDB(mongoStore)
			log.Println("✓ WorkerDB initialized")

			taskDB = db.NewTaskDB(mongoStore)
			log.Println("✓ TaskDB initialized")

			assignmentDB = db.NewAssignmentDB(mongoStore)
			log.Println("✓ AssignmentDB initialized")

			attemptDB = db.NewAttemptDB(mongoStore)
			log.Println("✓ AttemptDB initialized")

			resultDB = db.NewResultDB(mongoStore)
			log.Println("✓ ResultDB initialized")

			userDB = db.NewUserDB(mongoStore)
			log.Println("✓ UserDB initialized")
			if err := bootstrapDefaultWebUIAdmin(userDB); err != nil {
				log.Printf("Warning: Failed to bootstrap Web UI admin user: %v", err)
			}

			fileMetadataDB = db.NewFileMetadataDB(mongoStore)
			log.Println("✓ FileMetadataDB initialized")

			schedulerModelDB, err = db.NewSchedulerModelDB(mongoStore)
			if err != nil {
				log.Printf("Warning: Failed to create SchedulerModelDB: %v", err)
			} else {
				log.Println("✓ SchedulerModelDB initialized")
			}
		}
	}

	fileStorage, err = storage.NewFileStorageService(fileStorageBaseDir)
	if err != nil {
		log.Printf("Warning: Failed to create FileStorageService: %v", err)
		log.Println("Continuing without file storage...")
		fileStorage = nil
	} else {
		log.Printf("✓ FileStorageService initialized (base: %s)", fileStorageBaseDir)
		defer fileStorage.Close()
	}

	telemetryMgr := telemetry.NewTelemetryManager(30 * time.Second)
	telemetryMgr.Start()
	log.Println("✓ Telemetry manager started")

	tauStore := telemetry.NewInMemoryTauStore()
	log.Println("✓ Tau store initialized with default values:")
	for taskType, tau := range tauStore.GetAllTau() {
		log.Printf("  - %s: %.1fs", taskType, tau)
	}

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

	rrScheduler := scheduler.NewRoundRobinScheduler()
	log.Println("✓ Round-Robin scheduler created")

	var workerDBIface scheduler.WorkerDBInterface
	if workerDB != nil {
		workerDBIface = workerDB
	}
	telemetrySource := scheduler.NewMasterTelemetrySource(telemetryMgr, workerDBIface)
	log.Println("✓ Telemetry source adapter created")

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

	masterID := "master-1"
	grpcBindAddress, advertisedMasterAddress := resolveMasterAddresses(sysInfo, cfg)
	masterServer.SetMasterInfo(masterID, advertisedMasterAddress)

	var historyDB *db.HistoryDB
	if mongoStore != nil {
		historyDB = db.NewHistoryDB(mongoStore)
		log.Println("✓ HistoryDB initialized for AOD/GA training")
	} else {
		log.Println("AOD/GA training will be disabled (no MongoDB store)")
	}

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

	if workerDB != nil {
		if err := masterServer.LoadWorkersFromDB(ctx); err != nil {
			log.Printf("Warning: Failed to load workers from DB: %v", err)
		}
	}

	if err := masterServer.RestoreQueuedTasks(ctx); err != nil {
		log.Printf("Warning: Failed to restore queued tasks: %v", err)
	}

	masterServer.StartQueueProcessor()
	log.Println("✓ Task queue processor started")

	masterServer.StartWorkerReconnectionMonitor()
	log.Println("✓ Worker reconnection monitor started")

	grpcServer := grpc.NewServer()
	pb.RegisterMasterWorkerServer(grpcServer, masterServer)
	go startGRPCServer(grpcServer, grpcBindAddress)

	var httpTelemetryServer *httpserver.TelemetryServer
	if cfg.HTTPPort != "" {
		port := 8080
		if len(cfg.HTTPPort) > 1 && cfg.HTTPPort[0] == ':' {
			fmt.Sscanf(cfg.HTTPPort, ":%d", &port)
		}

		httpTelemetryServer = httpserver.NewTelemetryServer(port, telemetryMgr)

		taskHandler := httpserver.NewTaskAPIHandler(masterServer, taskDB, assignmentDB, attemptDB, resultDB)
		workerHandler := httpserver.NewWorkerAPIHandler(masterServer, workerDB, assignmentDB, telemetryMgr)

		httpTelemetryServer.RegisterTaskHandlers(taskHandler)
		httpTelemetryServer.RegisterWorkerHandlers(workerHandler)

		if fileStorage != nil {
			fileHandler := httpserver.NewFileAPIHandler(fileStorage)
			httpTelemetryServer.RegisterFileHandlers(fileHandler)
			log.Println("✓ File API handlers registered")
		}

		if userDB != nil {
			authHandler := httpserver.NewAuthHandler(userDB)
			httpTelemetryServer.RegisterAuthHandlers(authHandler)
			log.Println("✓ Auth API handlers registered")
		}

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
	}

	shutdownOnce := &sync.Once{}
	shutdownMaster := func(trigger string) {
		shutdownOnce.Do(func() {
			log.Printf("\n\nShutting down master node (%s)...", trigger)

			masterServer.StopQueueProcessor()
			masterServer.StopWorkerReconnectionMonitor()

			if httpTelemetryServer != nil {
				httpTelemetryServer.Shutdown()
			}

			telemetryMgr.Shutdown()

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

			grpcServer.GracefulStop()

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
		})
	}
	defer shutdownMaster("normal exit")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		shutdownMaster(fmt.Sprintf("signal: %s", sig))
		os.Exit(0)
	}()

	time.Sleep(500 * time.Millisecond)

	masterServer.BroadcastMasterRegistration(masterID, advertisedMasterAddress)

	log.Println("\n✓ Master node started successfully")
	log.Printf("✓ Starting gRPC server on %s (advertise: %s)\n", grpcBindAddress, advertisedMasterAddress)

	if cfg.Headless {
		log.Println("✓ Headless mode enabled, CLI disabled")
		select {}
	}

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
	if isTCPAddressReachable(cfg.PPOGRPCAddr, 500*time.Millisecond) {
		log.Printf("ℹ️  PPO service already reachable at %s, reusing existing process", cfg.PPOGRPCAddr)
		return nil, nil
	}

	projectRoot := detectProjectRoot()
	if projectRoot == "" {
		return nil, fmt.Errorf("unable to locate project root for PPO autostart")
	}

	modelPath := cfg.PPOModelPath
	if modelPath == "" {
		modelPath = "latest"
	}

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

func isTCPAddressReachable(addr string, timeout time.Duration) bool {
	if strings.TrimSpace(addr) == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
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
