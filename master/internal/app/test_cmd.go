package app

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"master/internal/config"
)

// RunTestCommand executes non-interactive CLI test suites.
func RunTestCommand(cfg *config.Config, args []string) int {
	if len(args) == 0 {
		printTestUsage()
		return 2
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "list":
		fmt.Println("Available test profiles:")
		fmt.Println("  - default:   1-to-1 exact replay of dataset test split")
		fmt.Println("  - bursty:    High-intensity resource burst stress test (heaviest tasks)")
		fmt.Println("  - long-tail: Long-running task workload to evaluate tail-latency & SLA attainment")
		fmt.Println("  - all:       Sequential execution of default, bursty, and long-tail suites")
		return 0
	case "run":
		if len(args) < 2 {
			printTestUsage()
			return 2
		}
		profile := strings.ToLower(strings.TrimSpace(args[1]))
		if profile != "default" && profile != "bursty" && profile != "long-tail" && profile != "all" {
			fmt.Printf("❌ Unsupported test profile: %s (choose: default, bursty, long-tail, all)\n", args[1])
			printTestUsage()
			return 2
		}

		projectRoot := detectProjectRoot()
		runnerScript := filepath.Join(projectRoot, "testbench", "runner.py")
		pythonBin := "python3"
		venvPython := filepath.Join(projectRoot, "venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			pythonBin = venvPython
		}

		cmdArgs := []string{runnerScript, "--profile", profile}
		for i := 2; i < len(args); i++ {
			cmdArgs = append(cmdArgs, args[i])
		}

		fmt.Printf("▶ Executing testbench runner: %s %s\n", pythonBin, strings.Join(cmdArgs, " "))
		cmd := exec.Command(pythonBin, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil {
			log.Printf("Test run failed: %v", err)
			return 1
		}
		return 0
	default:
		printTestUsage()
		return 2
	}
}

func printTestUsage() {
	fmt.Println("Usage:")
	fmt.Println("  master test list")
	fmt.Println("  master test run <default|bursty|long-tail|all> [--dataset <path>] [--mapping <yaml>] [--seed <num>] [--schedulers <RR,RTS,PPO>]")
}
