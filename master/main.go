// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"master/internal/db"
	httpserver "master/internal/http"
	"master/internal/scheduler"
	"master/internal/server"
	"master/internal/storage"
	"master/internal/system"
	"master/internal/telemetry"
	pb "master/proto"

	"go.mongodb.org/mongo-driver/mongo"
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
	masterAddress := sysInfo.GetMasterAddress() + cfg.GRPCPort
	masterServer.SetMasterInfo(masterID, masterAddress)

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
	masterServer.BroadcastMasterRegistration(masterID, masterAddress)

	// Start CLI interface
	log.Println("\n✓ Master node started successfully")
	log.Printf("✓ Starting gRPC server on %s\n", masterAddress)

	if cfg.Headless {
		log.Println("✓ Headless mode enabled (CLOUDAI_HEADLESS=true), CLI disabled")
		select {}
	}

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
		modelPath = "agentic_scheduler/models/ppo_latest.pt"
	}

	cmd := exec.Command(
		"python3",
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
