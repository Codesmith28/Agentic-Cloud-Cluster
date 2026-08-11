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

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"master/internal/testworkflow"
)

func (c *CLI) runTestCommand(parts []string) {
	if len(parts) < 2 {
		c.showTestHelp()
		return
	}

	engine := testworkflow.NewEngine()

	switch parts[1] {
	case "list":
		fmt.Println("Available test suites:")
		for _, suite := range engine.ListSuites() {
			fmt.Printf("  - %s: %s\n", suite.Name, suite.Description)
		}
	case "cleanup":
		opts := testworkflow.CleanupOptions{
			ComposeFile: resolveTestComposeFile(),
			Profile:     "hetero-small",
			Stdout:      os.Stdout,
			Stderr:      os.Stderr,
		}
		if err := engine.Cleanup(context.Background(), opts); err != nil {
			fmt.Printf("❌ Cleanup failed: %v\n", err)
			return
		}
		fmt.Println("✅ Test workflow environment cleaned up")
	case "run":
		if len(parts) < 3 {
			c.showTestHelp()
			return
		}
		suite := strings.ToLower(strings.TrimSpace(parts[2]))
		if !isSupportedTestSuite(suite) {
			fmt.Printf("❌ unsupported test suite: %s\n", parts[2])
			c.showTestHelp()
			return
		}
		opts, err := parseTestRunOptions(parts[3:])
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return
		}
		opts.Mode = testworkflow.ModeInteractive
		opts.MasterURL = resolveHTTPBaseURL()
		opts.PrometheusURL = strings.TrimSpace(os.Getenv("PROMETHEUS_URL"))
		opts.Stdout = os.Stdout
		opts.Stderr = os.Stderr
		opts.ComposeFile = resolveTestComposeFile()
		activeScheduler := ""
		if c.masterServer != nil {
			activeScheduler = c.masterServer.GetSchedulerName()
		}
		opts.Scheduler = resolveRunScheduler(opts.Scheduler, activeScheduler)
		opts.ExtraEnv = defaultTestExtraEnv(opts.ComposeFile)

		fmt.Printf("▶ Running %s suite...\n", suite)
		result, runErr := engine.RunSuite(context.Background(), suite, opts)
		if result != nil {
			fmt.Printf("Artifacts: %s\n", result.OutputDir)
		}
		if runErr != nil {
			fmt.Printf("❌ %s suite failed: %v\n", suite, runErr)
			return
		}
		fmt.Printf("✅ %s suite completed\n", suite)
	default:
		c.showTestHelp()
	}
}

func parseTestRunOptions(args []string) (testworkflow.RunOptions, error) {
	opts := testworkflow.RunOptions{
		Profile:       "hetero-small",
		Scheduler:     "current",
		EnableUISmoke: false,
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
			return opts, fmt.Errorf("unknown test run option: %s", args[i])
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

func (c *CLI) showTestHelp() {
	fmt.Println("Usage:")
	fmt.Println("  test list")
	fmt.Println("  test run <smoke|reliability|ui-smoke|evidence|full> [-profile <hetero-small|recovery-lab>] [-out <dir>] [-keep-env] [-ui-smoke] [-scheduler <current|RR|RTS>]")
	fmt.Println("  test cleanup")
}

func resolveHTTPBaseURL() string {
	port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
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

func resolveTestComposeFile() string {
	value := strings.TrimSpace(os.Getenv("TESTBENCH_COMPOSE_FILE"))
	if value != "" {
		return value
	}
	return "testbench/docker-compose.host-master.yml"
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

func defaultTestExtraEnv(composeFile string) map[string]string {
	if !strings.Contains(strings.ToLower(composeFile), "host-master") {
		return nil
	}
	return map[string]string{
		"WORKER_SPECS": "worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054",
	}
}
