package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (c *CLI) runTestCommand(parts []string) {
	if len(parts) < 2 {
		c.showTestHelp()
		return
	}

	switch parts[1] {
	case "list":
		fmt.Println("Available test profiles:")
		fmt.Println("  - default:   1-to-1 exact replay of dataset test split")
		fmt.Println("  - bursty:    High-intensity resource burst stress test (heaviest tasks)")
		fmt.Println("  - long-tail: Long-running task workload to evaluate tail-latency & SLA attainment")
		fmt.Println("  - all:       Sequential execution of default, bursty, and long-tail suites")
	case "run":
		if len(parts) < 3 {
			c.showTestHelp()
			return
		}
		profile := strings.ToLower(strings.TrimSpace(parts[2]))
		if profile != "default" && profile != "bursty" && profile != "long-tail" && profile != "all" {
			fmt.Printf("❌ Unsupported test profile: %s (choose: default, bursty, long-tail, all)\n", parts[2])
			c.showTestHelp()
			return
		}

		fmt.Printf("▶ Executing benchmark suite for profile '%s'...\n", profile)

		// Locate project root and python binary
		projectRoot, _ := os.Getwd()
		runnerScript := filepath.Join(projectRoot, "testbench", "runner.py")
		if _, err := os.Stat(runnerScript); os.IsNotExist(err) {
			runnerScript = filepath.Join(filepath.Dir(projectRoot), "testbench", "runner.py")
		}

		pythonBin := "python3"
		venvPython := filepath.Join(projectRoot, "venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			pythonBin = venvPython
		}

		cmdArgs := []string{runnerScript, "--profile", profile}
		for i := 3; i < len(parts); i++ {
			cmdArgs = append(cmdArgs, parts[i])
		}

		cmd := exec.Command(pythonBin, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil {
			fmt.Printf("❌ Benchmark execution failed: %v\n", err)
			return
		}
		fmt.Printf("✅ Benchmark suite '%s' completed\n", profile)
	default:
		c.showTestHelp()
	}
}

func (c *CLI) showTestHelp() {
	fmt.Println("Usage:")
	fmt.Println("  test list")
	fmt.Println("  test run <default|bursty|long-tail|all> [--dataset <path>] [--mapping <yaml>] [--seed <num>] [--schedulers <RR,RTS,PPO>]")
}
