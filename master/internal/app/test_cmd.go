package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"master/internal/config"
	"master/internal/testworkflow"
)

// RunTestCommand executes non-interactive CLI test suites.
func RunTestCommand(cfg *config.Config, args []string) int {
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
	cmd.Env = append(cmd.Env, "AGENTIC_HEADLESS=true", "CLOUDAI_HEADLESS=true")

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
		resp, err := client.Get(url)
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
