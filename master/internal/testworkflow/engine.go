package testworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type commandRequest struct {
	Name       string
	Executable string
	Args       []string
	WorkingDir string
	Env        map[string]string
	LogDir     string
	Stdout     io.Writer
	Stderr     io.Writer
}

type commandRunner interface {
	Run(ctx context.Context, req commandRequest) (StepResult, error)
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, req commandRequest) (StepResult, error) {
	startedAt := time.Now()
	step := StepResult{
		Name:       req.Name,
		Command:    strings.TrimSpace(strings.Join(append([]string{req.Executable}, req.Args...), " ")),
		WorkingDir: req.WorkingDir,
		StartedAt:  startedAt,
		ExitCode:   -1,
	}

	cmd := exec.CommandContext(ctx, req.Executable, req.Args...)
	cmd.Dir = req.WorkingDir
	cmd.Env = mergeEnvironment(req.Env)

	stdout := req.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := req.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	var closeFns []func() error
	if req.LogDir != "" {
		if err := os.MkdirAll(req.LogDir, 0o755); err != nil {
			return step, fmt.Errorf("create log dir: %w", err)
		}

		safeName := sanitizeStepName(req.Name)
		stdoutPath := filepath.Join(req.LogDir, safeName+".stdout.log")
		stderrPath := filepath.Join(req.LogDir, safeName+".stderr.log")

		stdoutFile, err := os.Create(stdoutPath)
		if err != nil {
			return step, fmt.Errorf("create stdout log file: %w", err)
		}
		stderrFile, err := os.Create(stderrPath)
		if err != nil {
			_ = stdoutFile.Close()
			return step, fmt.Errorf("create stderr log file: %w", err)
		}

		closeFns = append(closeFns, stdoutFile.Close, stderrFile.Close)
		stdout = io.MultiWriter(stdout, stdoutFile)
		stderr = io.MultiWriter(stderr, stderrFile)
		step.StdoutLog = stdoutPath
		step.StderrLog = stderrPath
	}
	for i := len(closeFns) - 1; i >= 0; i-- {
		defer closeFns[i]()
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()

	step.FinishedAt = time.Now()
	step.Duration = step.FinishedAt.Sub(step.StartedAt)
	if cmd.ProcessState != nil {
		step.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return step, fmt.Errorf("step %q failed: %w", req.Name, err)
	}
	return step, nil
}

// ListSuites returns the supported suites in deterministic order.
func (e *Engine) ListSuites() []SuiteInfo {
	return ListSuites()
}

// Run executes one selected suite.
func (e *Engine) Run(ctx context.Context, suiteName string, opts RunOptions) (*RunResult, error) {
	return e.RunSuite(ctx, suiteName, opts)
}

// RunSuite executes one test workflow suite and writes run metadata to artifacts.
func (e *Engine) RunSuite(ctx context.Context, suiteName string, opts RunOptions) (*RunResult, error) {
	if strings.TrimSpace(opts.RepoRoot) == "" {
		opts.RepoRoot = e.projectRoot
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	normalized, err := normalizeRunOptions(suiteName, opts, e.now())
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(normalized.outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	result := &RunResult{
		Suite:          normalized.suite,
		OutputDir:      normalized.outputDir,
		SummaryPath:    filepath.Join(normalized.outputDir, "run-result.json"),
		AssertionsPath: filepath.Join(normalized.outputDir, "assertion_results.json"),
		StartedAt:      e.now(),
	}

	runErr := e.runNormalizedSuite(ctx, normalized, result)
	result.FinishedAt = e.now()
	result.Duration = result.FinishedAt.Sub(result.StartedAt)
	if runErr != nil {
		result.Error = runErr.Error()
	}
	result.Success = runErr == nil

	if persistErr := persistRunResult(result); persistErr != nil {
		if runErr == nil {
			runErr = persistErr
			result.Error = persistErr.Error()
			result.Success = false
		} else if result.Error == "" {
			result.Error = runErr.Error()
		}
	}

	if runErr != nil {
		return result, runErr
	}
	return result, nil
}

// Cleanup tears down the compose environment.
func (e *Engine) Cleanup(ctx context.Context, opts CleanupOptions) error {
	if strings.TrimSpace(opts.RepoRoot) == "" {
		opts.RepoRoot = e.projectRoot
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	normalized, err := normalizeCleanupOptions(opts)
	if err != nil {
		return err
	}
	_, err = e.runner.Run(ctx, commandRequest{
		Name:       "cleanup-environment",
		Executable: "docker",
		Args:       []string{"compose", "-f", normalized.composeFile, "down", "--remove-orphans"},
		WorkingDir: normalized.repoRoot,
		Env:        composeCommandEnv(normalized.extraEnv, normalized.repoRoot),
		Stdout:     normalized.stdout,
		Stderr:     normalized.stderr,
	})
	return err
}

func (e *Engine) runNormalizedSuite(ctx context.Context, opts normalizedRunOptions, result *RunResult) error {
	switch opts.suite {
	case SuiteSmoke:
		return e.runSmokeSuite(ctx, opts, result)
	case SuiteReliability:
		return e.runReliabilitySuite(ctx, opts, result)
	case SuiteUISmoke:
		return e.runUISmokeSuite(ctx, opts, result)
	case SuiteEvidence:
		return e.runEvidenceSuite(ctx, opts, result)
	case SuiteFull:
		return e.runFullSuite(ctx, opts, result)
	default:
		return fmt.Errorf("unsupported suite %q", opts.suite)
	}
}

func (e *Engine) runSmokeSuite(ctx context.Context, opts normalizedRunOptions, result *RunResult) (err error) {
	if !opts.keepEnvironment {
		defer func() {
			cleanupErr := e.cleanupSuiteEnvironment(opts, result)
			if cleanupErr != nil {
				if err == nil {
					err = cleanupErr
				} else {
					result.CleanupError = cleanupErr.Error()
				}
			}
		}()
	}

	if err = e.prepareEnvironment(ctx, opts, result); err != nil {
		return err
	}

	summaryPath := filepath.Join(opts.outputDir, "summary.json")
	result.SummaryPath = summaryPath
	if err = e.runWorkload(ctx, opts, result, summaryPath); err != nil {
		return err
	}
	if err = e.exportAttemptSnapshots(ctx, opts, result, summaryPath, filepath.Join(opts.outputDir, "attempt_snapshots")); err != nil {
		return err
	}
	if err = e.exportObservability(ctx, opts, result, summaryPath, filepath.Join(opts.outputDir, "observability")); err != nil {
		return err
	}

	if opts.enableUISmoke {
		if err = e.runInlineStep(result, "ui-api-verification", func() error {
			return verifyUISmokeEndpoints(ctx, opts.masterURL)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) runReliabilitySuite(ctx context.Context, opts normalizedRunOptions, result *RunResult) (err error) {
	if !opts.keepEnvironment {
		defer func() {
			cleanupErr := e.cleanupSuiteEnvironment(opts, result)
			if cleanupErr != nil {
				if err == nil {
					err = cleanupErr
				} else {
					result.CleanupError = cleanupErr.Error()
				}
			}
		}()
	}

	if err = e.prepareEnvironment(ctx, opts, result); err != nil {
		return err
	}

	campaignDir := filepath.Join(opts.outputDir, "campaign")
	args := []string{
		scriptPath(opts.repoRoot, "run_campaign.py"),
		"--master-url", opts.masterURL,
		"--prometheus-url", opts.prometheusURL,
		"--output-dir", campaignDir,
		"--scenarios", "overload",
		"--workloads", opts.reliabilityWorkloads,
		"--schedulers", reliabilitySchedulers(opts),
		"--timeout", strconv.Itoa(int(opts.timeout.Seconds())),
	}
	result.SummaryPath = filepath.Join(campaignDir, "run-result.json")
	return e.runStep(ctx, opts, result, "run-reliability-campaign", "python3", args, nil)
}

func (e *Engine) runUISmokeSuite(ctx context.Context, opts normalizedRunOptions, result *RunResult) (err error) {
	if !opts.keepEnvironment {
		defer func() {
			cleanupErr := e.cleanupSuiteEnvironment(opts, result)
			if cleanupErr != nil {
				if err == nil {
					err = cleanupErr
				} else {
					result.CleanupError = cleanupErr.Error()
				}
			}
		}()
	}

	if err = e.prepareEnvironment(ctx, opts, result); err != nil {
		return err
	}

	summaryPath := filepath.Join(opts.outputDir, "summary.json")
	result.SummaryPath = summaryPath
	if err = e.runWorkload(ctx, opts, result, summaryPath); err != nil {
		return err
	}
	if err = e.exportAttemptSnapshots(ctx, opts, result, summaryPath, filepath.Join(opts.outputDir, "attempt_snapshots")); err != nil {
		return err
	}
	if err = e.exportObservability(ctx, opts, result, summaryPath, filepath.Join(opts.outputDir, "observability")); err != nil {
		return err
	}
	return e.runInlineStep(result, "ui-api-verification", func() error {
		return verifyUISmokeEndpoints(ctx, opts.masterURL)
	})
}

func (e *Engine) runEvidenceSuite(ctx context.Context, opts normalizedRunOptions, result *RunResult) (err error) {
	if !opts.keepEnvironment {
		defer func() {
			cleanupErr := e.cleanupSuiteEnvironment(opts, result)
			if cleanupErr != nil {
				if err == nil {
					err = cleanupErr
				} else {
					result.CleanupError = cleanupErr.Error()
				}
			}
		}()
	}

	if err = e.prepareEnvironment(ctx, opts, result); err != nil {
		return err
	}

	campaignDir := filepath.Join(opts.outputDir, "campaign")
	args := []string{
		scriptPath(opts.repoRoot, "run_campaign.py"),
		"--master-url", opts.masterURL,
		"--prometheus-url", opts.prometheusURL,
		"--compose-file", opts.composeFile,
		"--output-dir", campaignDir,
		"--scenarios", "all",
		"--workloads", opts.evidenceWorkloads,
		"--schedulers", evidenceSchedulers(opts),
		"--timeout", strconv.Itoa(int(opts.timeout.Seconds())),
	}
	result.SummaryPath = filepath.Join(campaignDir, "run-result.json")
	return e.runStep(ctx, opts, result, "run-evidence-campaign", "python3", args, nil)
}

func (e *Engine) runFullSuite(ctx context.Context, opts normalizedRunOptions, result *RunResult) error {
	if err := e.runStep(ctx, opts, result, "go-test-preflight-master", "go", []string{"test", "./...", "-count=1"}, nil, filepath.Join(opts.repoRoot, "master")); err != nil {
		return err
	}
	if err := e.runStep(ctx, opts, result, "go-test-preflight-worker", "go", []string{"test", "./...", "-count=1"}, nil, filepath.Join(opts.repoRoot, "worker")); err != nil {
		return err
	}

	suiteOrder := []Suite{SuiteSmoke, SuiteReliability, SuiteUISmoke, SuiteEvidence}
	for _, suite := range suiteOrder {
		childOpts := opts
		childOpts.suite = suite
		childOpts.outputDir = filepath.Join(opts.outputDir, string(suite))

		if err := os.MkdirAll(childOpts.outputDir, 0o755); err != nil {
			return fmt.Errorf("create child output dir: %w", err)
		}

		childResult := RunResult{
			Suite:          suite,
			OutputDir:      childOpts.outputDir,
			SummaryPath:    filepath.Join(childOpts.outputDir, "run-result.json"),
			AssertionsPath: filepath.Join(childOpts.outputDir, "assertion_results.json"),
			StartedAt:      e.now(),
		}
		runErr := e.runNormalizedSuite(ctx, childOpts, &childResult)
		childResult.FinishedAt = e.now()
		childResult.Duration = childResult.FinishedAt.Sub(childResult.StartedAt)
		if runErr != nil {
			childResult.Error = runErr.Error()
		}
		childResult.Success = runErr == nil
		if persistErr := persistRunResult(&childResult); persistErr != nil {
			if runErr == nil {
				runErr = persistErr
				childResult.Error = persistErr.Error()
				childResult.Success = false
			}
		}
		result.SubRuns = append(result.SubRuns, childResult)
		if runErr != nil {
			return fmt.Errorf("full suite failed at %s: %w", suite, runErr)
		}
	}

	return nil
}

func (e *Engine) prepareEnvironment(ctx context.Context, opts normalizedRunOptions, result *RunResult) error {
	if err := e.ensureDockerReady(ctx, opts, result); err != nil {
		return err
	}

	composeEnv := composeCommandEnv(opts.extraEnv, opts.repoRoot)
	if err := e.runStep(ctx, opts, result, "compose-up", "docker", []string{"compose", "-f", opts.composeFile, "up", "-d", "--build"}, composeEnv); err != nil {
		return err
	}
	if err := e.runStep(ctx, opts, result, "prepare-workflow-images", "bash",
		[]string{scriptPath(opts.repoRoot, "prepare_workflow_images.sh")},
		map[string]string{"COMPOSE_FILE": opts.composeFile}); err != nil {
		return err
	}
	registerEnv := map[string]string{
		"MASTER_URL": opts.masterURL,
	}
	if opts.extraEnv == nil || opts.extraEnv["WORKER_SPECS"] == "" {
		if specs := defaultWorkerSpecsForComposeFile(opts.composeFile); specs != "" {
			registerEnv["WORKER_SPECS"] = specs
		}
	}
	return e.runStep(ctx, opts, result, "register-workers", "bash",
		[]string{scriptPath(opts.repoRoot, "register_workers.sh")},
		registerEnv)
}

func (e *Engine) runWorkload(ctx context.Context, opts normalizedRunOptions, result *RunResult, summaryPath string) error {
	args := []string{
		scriptPath(opts.repoRoot, "run_workload.py"),
		"--master-url", opts.masterURL,
		"--workload", opts.smokeWorkloadPath,
		"--output", summaryPath,
		"--fail-on-task-failure",
	}
	return e.runStep(ctx, opts, result, "run-workload", "python3", args, nil)
}

func (e *Engine) exportObservability(ctx context.Context, opts normalizedRunOptions, result *RunResult, summaryPath, outputDir string) error {
	args := []string{
		scriptPath(opts.repoRoot, "export_metrics.py"),
		"--prometheus-url", opts.prometheusURL,
		"--master-url", opts.masterURL,
		"--summary", summaryPath,
		"--output-dir", outputDir,
	}
	return e.runStep(ctx, opts, result, "export-observability", "python3", args, nil)
}

func (e *Engine) exportAttemptSnapshots(ctx context.Context, opts normalizedRunOptions, result *RunResult, summaryPath, outputDir string) error {
	args := []string{
		scriptPath(opts.repoRoot, "export_attempt_snapshots.py"),
		"--master-url", opts.masterURL,
		"--summary", summaryPath,
		"--output-dir", outputDir,
	}
	return e.runStep(ctx, opts, result, "export-attempt-snapshots", "python3", args, nil)
}

func (e *Engine) cleanupSuiteEnvironment(opts normalizedRunOptions, result *RunResult) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return e.runStep(
		cleanupCtx,
		opts,
		result,
		"cleanup-environment",
		"docker",
		[]string{"compose", "-f", opts.composeFile, "down", "--remove-orphans"},
		composeCommandEnv(opts.extraEnv, opts.repoRoot),
	)
}

func (e *Engine) ensureDockerReady(ctx context.Context, opts normalizedRunOptions, result *RunResult) error {
	if err := e.runStep(ctx, opts, result, "docker-daemon-check", "docker", []string{"info"}, nil); err != nil {
		return fmt.Errorf("docker daemon is unavailable; start Docker and retry: %w", err)
	}
	if err := e.runStep(ctx, opts, result, "docker-compose-check", "docker", []string{"compose", "version"}, nil); err != nil {
		return fmt.Errorf("docker compose is unavailable; ensure Docker Compose is installed: %w", err)
	}
	return nil
}

func (e *Engine) runStep(ctx context.Context, opts normalizedRunOptions, result *RunResult, name, executable string, args []string, stepEnv map[string]string, workingDir ...string) error {
	stepCtx := ctx
	var cancel context.CancelFunc
	if opts.timeout > 0 {
		stepCtx, cancel = context.WithTimeout(ctx, opts.timeout)
		defer cancel()
	}

	cwd := opts.repoRoot
	if len(workingDir) > 0 && strings.TrimSpace(workingDir[0]) != "" {
		cwd = workingDir[0]
	}

	mergedEnv := cloneStringMap(opts.extraEnv)
	for k, v := range stepEnv {
		if mergedEnv == nil {
			mergedEnv = map[string]string{}
		}
		mergedEnv[k] = v
	}

	step, err := e.runner.Run(stepCtx, commandRequest{
		Name:       name,
		Executable: executable,
		Args:       args,
		WorkingDir: cwd,
		Env:        mergedEnv,
		LogDir:     filepath.Join(opts.outputDir, "logs"),
		Stdout:     opts.stdout,
		Stderr:     opts.stderr,
	})
	if err != nil {
		step.Error = err.Error()
	}
	result.Steps = append(result.Steps, step)
	return err
}

func (e *Engine) runInlineStep(result *RunResult, name string, fn func() error) error {
	startedAt := e.now()
	err := fn()
	finishedAt := e.now()
	step := StepResult{
		Name:       name,
		Command:    "inline",
		WorkingDir: "",
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Duration:   finishedAt.Sub(startedAt),
		ExitCode:   0,
	}
	if err != nil {
		step.ExitCode = 1
		step.Error = err.Error()
	}
	result.Steps = append(result.Steps, step)
	return err
}

func verifyUISmokeEndpoints(ctx context.Context, masterURL string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := strings.TrimRight(masterURL, "/")

	var health map[string]any
	if err := requestJSON(ctx, client, http.MethodGet, baseURL+"/health", nil, &health); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	if status, _ := health["status"].(string); strings.ToLower(status) != "healthy" {
		return fmt.Errorf("unexpected health status %q", status)
	}

	email := fmt.Sprintf("ui-smoke-%d@example.com", time.Now().UnixNano())
	password := "ui-smoke-pass-1234"
	registerPayload := map[string]string{
		"name":     "UI Smoke",
		"email":    email,
		"password": password,
	}

	var registerResp struct {
		Success bool `json:"success"`
	}
	if err := requestJSON(ctx, client, http.MethodPost, baseURL+"/api/auth/register", registerPayload, &registerResp); err != nil {
		return fmt.Errorf("register request failed: %w", err)
	}
	if !registerResp.Success {
		return fmt.Errorf("register response reported failure")
	}

	loginPayload := map[string]string{
		"email":    email,
		"password": password,
	}
	var loginResp struct {
		Success bool `json:"success"`
	}
	if err := requestJSON(ctx, client, http.MethodPost, baseURL+"/api/auth/login", loginPayload, &loginResp); err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	if !loginResp.Success {
		return fmt.Errorf("login response reported failure")
	}

	var workersResp struct {
		Workers []map[string]any `json:"workers"`
	}
	if err := requestJSON(ctx, client, http.MethodGet, baseURL+"/api/workers", nil, &workersResp); err != nil {
		return fmt.Errorf("workers request failed: %w", err)
	}
	if len(workersResp.Workers) == 0 {
		return fmt.Errorf("workers list is empty")
	}

	var tasksResp struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := requestJSON(ctx, client, http.MethodGet, baseURL+"/api/tasks", nil, &tasksResp); err != nil {
		return fmt.Errorf("tasks request failed: %w", err)
	}
	if len(tasksResp.Tasks) == 0 {
		return fmt.Errorf("tasks list is empty")
	}

	return nil
}

func requestJSON(ctx context.Context, client *http.Client, method string, url string, payload any, dest any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if dest == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func persistRunResult(result *RunResult) error {
	if result == nil || result.OutputDir == "" {
		return nil
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run result: %w", err)
	}
	path := filepath.Join(result.OutputDir, "run-result.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write run result: %w", err)
	}
	return nil
}

func scriptPath(repoRoot string, scriptName string) string {
	return filepath.Join(repoRoot, "testbench", "scripts", scriptName)
}

func evidenceSchedulers(opts normalizedRunOptions) string {
	switch opts.scheduler {
	case "rr":
		return "RR"
	case "rts":
		return "RTS"
	default:
		switch opts.activeScheduler {
		case "rr":
			return "RR"
		case "rts":
			return "RTS"
		}
		return "RR,RTS"
	}
}

func reliabilitySchedulers(opts normalizedRunOptions) string {
	switch opts.scheduler {
	case "rr":
		return "RR"
	case "rts":
		return "RTS"
	case "ppo":
		return "PPO"
	default:
		switch opts.activeScheduler {
		case "rr":
			return "RR"
		case "rts":
			return "RTS"
		case "ppo":
			return "PPO"
		}
		return "RR,RTS,PPO"
	}
}

func sanitizeStepName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "step"
	}
	parts := strings.Fields(strings.ToLower(name))
	cleaned := strings.Join(parts, "-")
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", ".", "-", ",", "-", "=", "-")
	cleaned = replacer.Replace(cleaned)
	runes := make([]rune, 0, len(cleaned))
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			runes = append(runes, r)
		}
	}
	if len(runes) == 0 {
		return "step"
	}
	return string(runes)
}

func mergeEnvironment(overrides map[string]string) []string {
	base := map[string]string{}
	for _, kv := range os.Environ() {
		if key, val, ok := strings.Cut(kv, "="); ok {
			base[key] = val
		}
	}
	keys := make([]string, 0, len(base))
	for k := range base {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for k, v := range overrides {
		base[k] = v
		if !slices.Contains(keys, k) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+base[key])
	}
	return env
}

func defaultWorkerSpecsForComposeFile(composeFile string) string {
	if strings.Contains(strings.ToLower(composeFile), "host-master") {
		return "worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054"
	}
	return ""
}

func composeCommandEnv(base map[string]string, repoRoot string) map[string]string {
	env := cloneStringMap(base)
	if env == nil {
		env = map[string]string{}
	}

	dotenv := loadRepoDotEnv(repoRoot)

	if strings.TrimSpace(env["GF_ADMIN_USER"]) == "" && strings.TrimSpace(os.Getenv("GF_ADMIN_USER")) == "" {
		if value := strings.TrimSpace(dotenv["GF_ADMIN_USER"]); value != "" {
			env["GF_ADMIN_USER"] = value
		} else {
			env["GF_ADMIN_USER"] = "admin"
		}
	}
	if strings.TrimSpace(env["GF_ADMIN_PASSWORD"]) == "" && strings.TrimSpace(os.Getenv("GF_ADMIN_PASSWORD")) == "" {
		if value := strings.TrimSpace(dotenv["GF_ADMIN_PASSWORD"]); value != "" {
			env["GF_ADMIN_PASSWORD"] = value
		} else {
			env["GF_ADMIN_PASSWORD"] = "password"
		}
	}
	return env
}

func loadRepoDotEnv(repoRoot string) map[string]string {
	if strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	values, err := godotenv.Read(filepath.Join(repoRoot, ".env"))
	if err != nil {
		return nil
	}
	return values
}
