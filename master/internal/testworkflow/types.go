package testworkflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Suite identifies a supported E2E workflow suite.
type Suite string

const (
	SuiteSmoke       Suite = "smoke"
	SuiteReliability Suite = "reliability"
	SuiteUISmoke     Suite = "ui-smoke"
	SuiteEvidence    Suite = "evidence"
	SuiteFull        Suite = "full"
)

const (
	ModeInteractive    = "interactive"
	ModeNonInteractive = "non-interactive"
)

// SuiteInfo describes one available suite.
type SuiteInfo struct {
	Name        Suite  `json:"name"`
	Description string `json:"description"`
}

var suiteCatalog = []SuiteInfo{
	{Name: SuiteSmoke, Description: "Core end-to-end smoke coverage"},
	{Name: SuiteReliability, Description: "Failure and recovery campaign coverage"},
	{Name: SuiteUISmoke, Description: "API-level UI smoke checks"},
	{Name: SuiteEvidence, Description: "Evidence benchmark campaign"},
	{Name: SuiteFull, Description: "go test plus smoke, reliability, ui-smoke, evidence"},
}

// RunOptions controls suite execution.
type RunOptions struct {
	Profile         string
	OutputDir       string
	KeepEnv         bool
	UISmoke         bool
	Scheduler       string
	ActiveScheduler string
	MasterURL       string
	PrometheusURL   string
	ComposeFile     string
	Mode            string

	RepoRoot        string
	Workload        string
	KeepEnvironment bool
	EnableUISmoke   bool
	Timeout         time.Duration
	Stdout          io.Writer
	Stderr          io.Writer
	ExtraEnv        map[string]string
}

// CleanupOptions controls explicit environment teardown.
type CleanupOptions struct {
	ComposeFile string
	RepoRoot    string
	Profile     string
	Stdout      io.Writer
	Stderr      io.Writer
	ExtraEnv    map[string]string
}

// StepResult captures one subprocess execution.
type StepResult struct {
	Name       string        `json:"name"`
	Command    string        `json:"command"`
	WorkingDir string        `json:"working_dir"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Duration   time.Duration `json:"duration"`
	ExitCode   int           `json:"exit_code"`
	StdoutLog  string        `json:"stdout_log,omitempty"`
	StderrLog  string        `json:"stderr_log,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// RunResult summarizes one suite run.
type RunResult struct {
	Suite          Suite         `json:"suite"`
	OutputDir      string        `json:"output_dir"`
	SummaryPath    string        `json:"summary_path,omitempty"`
	AssertionsPath string        `json:"assertions_path,omitempty"`
	StartedAt      time.Time     `json:"started_at"`
	FinishedAt     time.Time     `json:"finished_at"`
	Duration       time.Duration `json:"duration"`
	Success        bool          `json:"success"`
	Steps          []StepResult  `json:"steps,omitempty"`
	SubRuns        []RunResult   `json:"sub_runs,omitempty"`
	CleanupError   string        `json:"cleanup_error,omitempty"`
	Error          string        `json:"error,omitempty"`
}

// Engine executes shared test workflow suites.
type Engine struct {
	projectRoot string
	now         func() time.Time
	runner      commandRunner
}

// ListSuites returns suite descriptions in deterministic order.
func ListSuites() []SuiteInfo {
	out := make([]SuiteInfo, len(suiteCatalog))
	copy(out, suiteCatalog)
	return out
}

// ListSuiteNames returns supported suite names in deterministic order.
func ListSuiteNames() []string {
	details := ListSuites()
	out := make([]string, 0, len(details))
	for _, detail := range details {
		out = append(out, string(detail.Name))
	}
	return out
}

// NewEngine creates a workflow engine with OS-backed subprocess execution.
func NewEngine() *Engine {
	root, _ := resolveRepoRoot("")
	return &Engine{
		projectRoot: root,
		now:         time.Now,
		runner:      osCommandRunner{},
	}
}

// New creates a workflow engine for a specific repository root.
func New(projectRoot string) (*Engine, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		return nil, fmt.Errorf("project root is required")
	}
	resolved, err := findRepoRoot(root)
	if err != nil {
		return nil, err
	}
	return &Engine{
		projectRoot: resolved,
		now:         time.Now,
		runner:      osCommandRunner{},
	}, nil
}

// NewFromWorkingDir creates an engine by detecting the repository from cwd.
func NewFromWorkingDir() (*Engine, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root := DetectProjectRoot(cwd)
	if root == "" {
		return nil, fmt.Errorf("unable to locate project root from %s", cwd)
	}
	return New(root)
}

// DetectProjectRoot walks upward from start to locate the repository root.
func DetectProjectRoot(start string) string {
	root, err := findRepoRoot(start)
	if err != nil {
		return ""
	}
	return root
}

// RunSuite executes one selected suite.
func RunSuite(ctx context.Context, suite string, opts RunOptions) (*RunResult, error) {
	return NewEngine().RunSuite(ctx, suite, opts)
}

// Cleanup tears down the testbench environment.
func Cleanup(ctx context.Context, opts CleanupOptions) error {
	return NewEngine().Cleanup(ctx, opts)
}
