package testworkflow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultMasterURL     = "http://localhost:8080"
	defaultPrometheusURL = "http://localhost:9090"
	defaultProfile       = "hetero-small"
	defaultScheduler     = "current"
	defaultRunTimeout    = 60 * time.Minute
)

type profileDefaults struct {
	composeRel           string
	smokeWorkloadRel     string
	reliabilityWorkloads string
	evidenceWorkloads    string
}

var profileCatalog = map[string]profileDefaults{
	"hetero-small": {
		composeRel:           filepath.Join("testbench", "docker-compose.host-master.yml"),
		smokeWorkloadRel:     filepath.Join("testbench", "workloads", "heterogeneous-smoke.json"),
		reliabilityWorkloads: "heterogeneous-smoke",
		evidenceWorkloads:    "heterogeneous-smoke,deterministic-full",
	},
	"recovery-lab": {
		composeRel:           filepath.Join("testbench", "docker-compose.host-master.yml"),
		smokeWorkloadRel:     filepath.Join("testbench", "workloads", "heterogeneous-smoke.json"),
		reliabilityWorkloads: "heterogeneous-smoke",
		evidenceWorkloads:    "heterogeneous-smoke,deterministic-full",
	},
}

var schedulerCatalog = map[string]struct{}{
	"current": {},
	"rr":      {},
	"rts":     {},
}

type normalizedRunOptions struct {
	suite                Suite
	repoRoot             string
	outputDir            string
	profile              string
	scheduler            string
	activeScheduler      string
	masterURL            string
	prometheusURL        string
	composeFile          string
	smokeWorkloadPath    string
	reliabilityWorkloads string
	evidenceWorkloads    string
	keepEnvironment      bool
	enableUISmoke        bool
	mode                 string
	timeout              time.Duration
	stdout               io.Writer
	stderr               io.Writer
	extraEnv             map[string]string
}

type normalizedCleanupOptions struct {
	repoRoot    string
	composeFile string
	stdout      io.Writer
	stderr      io.Writer
	extraEnv    map[string]string
}

func normalizeSuite(raw string) (Suite, error) {
	suite := Suite(strings.ToLower(strings.TrimSpace(raw)))
	switch suite {
	case SuiteSmoke, SuiteReliability, SuiteUISmoke, SuiteEvidence, SuiteFull:
		return suite, nil
	default:
		return "", fmt.Errorf("unsupported test suite %q", raw)
	}
}

func normalizeProfile(raw string) (string, profileDefaults, error) {
	profile := strings.ToLower(strings.TrimSpace(raw))
	if profile == "" {
		profile = defaultProfile
	}
	cfg, ok := profileCatalog[profile]
	if !ok {
		return "", profileDefaults{}, fmt.Errorf("unsupported profile %q", raw)
	}
	return profile, cfg, nil
}

func normalizeScheduler(raw string) (string, error) {
	scheduler := strings.ToLower(strings.TrimSpace(raw))
	if scheduler == "" {
		scheduler = defaultScheduler
	}
	if _, ok := schedulerCatalog[scheduler]; !ok {
		return "", fmt.Errorf("unsupported scheduler %q", raw)
	}
	return scheduler, nil
}

func normalizeActiveScheduler(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(value, "rr"):
		return "rr"
	case strings.Contains(value, "rts"):
		return "rts"
	default:
		return ""
	}
}

func normalizeRunOptions(suiteName string, opts RunOptions, now time.Time) (normalizedRunOptions, error) {
	suite, err := normalizeSuite(suiteName)
	if err != nil {
		return normalizedRunOptions{}, err
	}

	repoRoot, err := resolveRepoRoot(opts.RepoRoot)
	if err != nil {
		return normalizedRunOptions{}, err
	}

	profile, profileCfg, err := normalizeProfile(opts.Profile)
	if err != nil {
		return normalizedRunOptions{}, err
	}

	scheduler, err := normalizeScheduler(opts.Scheduler)
	if err != nil {
		return normalizedRunOptions{}, err
	}

	outputDir, err := resolveOutputDir(repoRoot, opts.OutputDir, suite, now)
	if err != nil {
		return normalizedRunOptions{}, err
	}

	composeFile, err := resolveComposeFile(repoRoot, opts.ComposeFile, profileCfg.composeRel)
	if err != nil {
		return normalizedRunOptions{}, err
	}

	smokeWorkloadPath, err := normalizeWorkloadPath(repoRoot, opts.Workload, profileCfg.smokeWorkloadRel)
	if err != nil {
		return normalizedRunOptions{}, err
	}

	reliabilityWorkloads, err := normalizeCampaignWorkloads(repoRoot, opts.Workload, profileCfg.reliabilityWorkloads)
	if err != nil {
		return normalizedRunOptions{}, err
	}

	evidenceWorkloads, err := normalizeCampaignWorkloads(repoRoot, opts.Workload, profileCfg.evidenceWorkloads)
	if err != nil {
		return normalizedRunOptions{}, err
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultRunTimeout
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	keepEnvironment := opts.KeepEnvironment || opts.KeepEnv
	enableUISmoke := opts.EnableUISmoke || opts.UISmoke || suite == SuiteUISmoke

	mode := strings.TrimSpace(opts.Mode)
	if mode == "" {
		mode = ModeInteractive
	}
	if mode != ModeInteractive && mode != ModeNonInteractive {
		return normalizedRunOptions{}, fmt.Errorf("unsupported mode %q", opts.Mode)
	}

	return normalizedRunOptions{
		suite:                suite,
		repoRoot:             repoRoot,
		outputDir:            outputDir,
		profile:              profile,
		scheduler:            scheduler,
		activeScheduler:      normalizeActiveScheduler(opts.ActiveScheduler),
		masterURL:            normalizeURL(opts.MasterURL, defaultMasterURL),
		prometheusURL:        normalizeURL(opts.PrometheusURL, defaultPrometheusURL),
		composeFile:          composeFile,
		smokeWorkloadPath:    smokeWorkloadPath,
		reliabilityWorkloads: reliabilityWorkloads,
		evidenceWorkloads:    evidenceWorkloads,
		keepEnvironment:      keepEnvironment,
		enableUISmoke:        enableUISmoke,
		mode:                 mode,
		timeout:              timeout,
		stdout:               stdout,
		stderr:               stderr,
		extraEnv:             cloneStringMap(opts.ExtraEnv),
	}, nil
}

func normalizeCleanupOptions(opts CleanupOptions) (normalizedCleanupOptions, error) {
	repoRoot, err := resolveRepoRoot(opts.RepoRoot)
	if err != nil {
		return normalizedCleanupOptions{}, err
	}

	_, profileCfg, err := normalizeProfile(opts.Profile)
	if err != nil {
		return normalizedCleanupOptions{}, err
	}

	composeFile, err := resolveComposeFile(repoRoot, opts.ComposeFile, profileCfg.composeRel)
	if err != nil {
		return normalizedCleanupOptions{}, err
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	return normalizedCleanupOptions{
		repoRoot:    repoRoot,
		composeFile: composeFile,
		stdout:      stdout,
		stderr:      stderr,
		extraEnv:    cloneStringMap(opts.ExtraEnv),
	}, nil
}

func resolveOutputDir(repoRoot string, rawOutputDir string, suite Suite, now time.Time) (string, error) {
	if repoRoot == "" {
		return "", fmt.Errorf("repo root is required")
	}

	if strings.TrimSpace(rawOutputDir) == "" {
		runID := fmt.Sprintf("%s-%s", now.Format("20060102-150405"), suite)
		return filepath.Join(repoRoot, "results", "testbench", runID), nil
	}
	return resolvePath(repoRoot, rawOutputDir)
}

func resolveComposeFile(repoRoot string, rawComposeFile string, defaultComposeRel string) (string, error) {
	composePath := strings.TrimSpace(rawComposeFile)
	if composePath == "" {
		composePath = defaultComposeRel
	}
	resolved, err := resolvePath(repoRoot, composePath)
	if err != nil {
		return "", err
	}
	if err := requireFile(resolved); err != nil {
		return "", fmt.Errorf("compose file: %w", err)
	}
	return resolved, nil
}

func normalizeWorkloadPath(repoRoot string, rawWorkload string, defaultWorkloadRel string) (string, error) {
	workload := strings.TrimSpace(rawWorkload)
	if workload == "" {
		workload = defaultWorkloadRel
	} else if !strings.Contains(workload, string(filepath.Separator)) && !strings.HasSuffix(workload, ".json") {
		workload = filepath.Join("testbench", "workloads", workload+".json")
	}

	resolved, err := resolvePath(repoRoot, workload)
	if err != nil {
		return "", err
	}
	if err := requireFile(resolved); err != nil {
		return "", fmt.Errorf("workload file: %w", err)
	}
	return resolved, nil
}

func normalizeCampaignWorkloads(repoRoot string, rawWorkload string, defaultWorkloads string) (string, error) {
	if strings.TrimSpace(rawWorkload) == "" {
		return defaultWorkloads, nil
	}

	parts := strings.Split(rawWorkload, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}

		if strings.HasSuffix(item, ".json") || strings.Contains(item, string(filepath.Separator)) {
			path, err := resolvePath(repoRoot, item)
			if err != nil {
				return "", err
			}
			if err := requireFile(path); err != nil {
				return "", fmt.Errorf("campaign workload: %w", err)
			}
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			names = append(names, name)
			continue
		}

		path := filepath.Join(repoRoot, "testbench", "workloads", item+".json")
		if err := requireFile(path); err != nil {
			return "", fmt.Errorf("campaign workload %q not found", item)
		}
		names = append(names, item)
	}

	if len(names) == 0 {
		return "", fmt.Errorf("no valid campaign workloads provided")
	}
	return strings.Join(names, ","), nil
}

func resolveRepoRoot(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return findRepoRoot(explicit)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}
	return findRepoRoot(cwd)
}

func findRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	dir = filepath.Clean(dir)

	for {
		if looksLikeRepoRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not locate repository root from %q", start)
}

func looksLikeRepoRoot(dir string) bool {
	return fileExists(filepath.Join(dir, "master", "go.mod")) &&
		fileExists(filepath.Join(dir, "testbench", "scripts", "run_workload.py")) &&
		fileExists(filepath.Join(dir, "Makefile"))
}

func resolvePath(repoRoot string, rawPath string) (string, error) {
	if strings.TrimSpace(rawPath) == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(rawPath) {
		return filepath.Clean(rawPath), nil
	}
	return filepath.Join(repoRoot, filepath.Clean(rawPath)), nil
}

func normalizeURL(raw string, fallback string) string {
	url := strings.TrimSpace(raw)
	if url == "" {
		url = fallback
	}
	return strings.TrimRight(url, "/")
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%q is a directory", path)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
