package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"master/internal/controlplane"
	"master/internal/server"
	"master/internal/storage"

	"github.com/chzyer/readline"
)

// CLI handles the command-line interface for the master
type CLI struct {
	masterServer *server.MasterServer
	fileStorage  *storage.FileStorageService
	rl           *readline.Instance
	executor     *controlplane.Executor
}

// NewCLI creates a new CLI instance
func NewCLI(srv *server.MasterServer, fs *storage.FileStorageService, exec *controlplane.Executor) *CLI {
	rl, err := readline.New("master> ")
	if err != nil {
		log.Fatalf("Failed to create readline instance: %v", err)
	}
	return &CLI{
		masterServer: srv,
		fileStorage:  fs,
		rl:           rl,
		executor:     exec,
	}
}

// Run starts the interactive CLI
func (c *CLI) Run() {
	defer c.rl.Close()
	c.printBanner()

	for {
		input, err := c.rl.Readline()
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nShutting down master...")
				return
			}
			log.Printf("Error reading input: %v", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		command := parts[0]

		// Commands that need interactive terminal control stay in CLI
		switch command {
		case "status":
			c.showStatus()
			continue
		case "stats":
			if len(parts) < 2 {
				fmt.Println("Usage: stats <worker_id>")
				continue
			}
			c.showWorkerStats(parts[1])
			continue
		case "internal-state":
			c.liveInternalState()
			continue
		case "monitor":
			if len(parts) < 2 {
				fmt.Println("Usage: monitor <task_id>")
				continue
			}
			c.monitorTask(parts[1])
			continue
		case "test":
			c.runTestCommand(parts)
			continue
		case "exit", "quit":
			fmt.Println("Shutting down master...")
			return
		}

		// All other commands go through shared executor
		outcome := c.executor.ExecuteCommand(input)
		if outcome.Transcript != "" {
			fmt.Println(outcome.Transcript)
		}
		if outcome.Err != nil && outcome.Transcript == "" {
			fmt.Printf("Error: %v\n", outcome.Err)
		}
	}
}

func (c *CLI) printBanner() {
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  CloudAI Master Node - Interactive CLI")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("Type 'help' for available commands")
}

func (c *CLI) showStatus() {
	// ANSI escape codes
	const (
		clearScreen   = "\033[2J"
		moveCursor    = "\033[H"
		saveCursor    = "\0337"
		restoreCursor = "\0338"
		clearLine     = "\033[2K"
	)

	// Print initial view
	fmt.Print("\n")

	// Create a ticker for updates (refresh every 2 seconds)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Channel to detect user input (to exit the live view)
	done := make(chan bool)

	// Goroutine to listen for any key press
	go func() {
		reader := bufio.NewReader(os.Stdin)
		reader.ReadByte() // Wait for any key press
		done <- true
	}()

	// Print instructions
	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║    Live Cluster Status Monitor        ║")
	fmt.Println("║    Press any key to exit...           ║")
	fmt.Println("╚═══════════════════════════════════════╝")
	fmt.Println()

	// Function to render the status
	renderStatus := func() {
		workers := c.masterServer.GetWorkers()

		activeCount := 0
		totalTasks := 0
		for _, w := range workers {
			if w.IsActive {
				activeCount++
			}
			totalTasks += len(w.RunningTasks)
		}

		// Move cursor up to redraw (5 lines for the status box)
		fmt.Print("\033[5A") // Move up 5 lines
		fmt.Print("\r")      // Return to start of line

		// Clear and redraw status box
		fmt.Print(clearLine + "\r╔═══ Cluster Status ═══\n")
		fmt.Printf(clearLine+"\r║ Total Workers: %d\n", len(workers))
		fmt.Printf(clearLine+"\r║ Active Workers: %d\n", activeCount)
		fmt.Printf(clearLine+"\r║ Running Tasks: %d\n", totalTasks)
		fmt.Print(clearLine + "\r╚══════════════════════\n")
	}

	// Initial render
	fmt.Println("╔═══ Cluster Status ═══")
	fmt.Println("║ Total Workers: 0")
	fmt.Println("║ Active Workers: 0")
	fmt.Println("║ Running Tasks: 0")
	fmt.Println("╚══════════════════════")

	// Update loop
	for {
		select {
		case <-ticker.C:
			renderStatus()
		case <-done:
			fmt.Println("\nExiting status monitor...")
			return
		}
	}
}

func (c *CLI) showWorkerStats(workerID string) {
	// First check if worker exists
	_, exists := c.masterServer.GetWorkerStats(workerID)
	if !exists {
		fmt.Printf("❌ Worker '%s' not found\n", workerID)
		return
	}

	// ANSI escape codes
	const clearLine = "\033[2K"

	// Create a ticker for updates (refresh every 2 seconds)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Channel to detect user input (to exit the live view)
	done := make(chan bool)

	// Goroutine to listen for any key press
	go func() {
		reader := bufio.NewReader(os.Stdin)
		reader.ReadByte() // Wait for any key press
		done <- true
	}()

	// Track if this is the first render
	firstRender := true

	// Function to render the worker stats
	renderStats := func() {
		worker, exists := c.masterServer.GetWorkerStats(workerID)
		if !exists {
			if !firstRender {
				fmt.Print("\033[21A") // Move up
			}
			fmt.Print("\r")
			for i := 0; i < 21; i++ {
				fmt.Print(clearLine + "\r\n")
			}
			if !firstRender {
				fmt.Print("\033[21A")
			}
			fmt.Println(clearLine + "\r❌ Worker disconnected or removed")
			return
		}

		status := "🟢 Active"
		if !worker.IsActive {
			status = "🔴 Inactive"
		}

		// Calculate time since last heartbeat
		lastSeen := "Never"
		if worker.LastHeartbeat > 0 {
			duration := time.Now().Unix() - worker.LastHeartbeat
			if duration < 60 {
				lastSeen = fmt.Sprintf("%d seconds ago", duration)
			} else if duration < 3600 {
				lastSeen = fmt.Sprintf("%d minutes ago", duration/60)
			} else {
				lastSeen = fmt.Sprintf("%d hours ago", duration/3600)
			}
		}

		// Move cursor up to the start of the stats box
		// Box has 19 lines + 1 blank line + 1 instruction line = 21 lines total
		// Only move cursor up if this is NOT the first render
		if !firstRender {
			fmt.Print("\033[21A")
			fmt.Print("\r") // Move to beginning of line
		} else {
			fmt.Print("\n") // Add initial spacing
		}

		// Clear and redraw stats box (no right border)
		fmt.Printf("%s╔═══════════════════════════════════════════════════\n", clearLine)
		fmt.Printf("%s║ Worker: %s\n", clearLine, workerID)
		fmt.Printf("%s╠═══════════════════════════════════════════════════\n", clearLine)
		fmt.Printf("%s║ Status:          %s\n", clearLine, status)
		fmt.Printf("%s║ Address:         %s\n", clearLine, worker.Info.WorkerIp)
		fmt.Printf("%s║ Last Seen:       %s\n", clearLine, lastSeen)
		fmt.Printf("%s║\n", clearLine)
		fmt.Printf("%s║ Resources (Total / Allocated / Available):\n", clearLine)
		fmt.Printf("%s║   CPU:           %.2f / %.2f / %.2f cores (%.1f%% used)\n", clearLine,
			worker.Info.TotalCpu, worker.AllocatedCPU, worker.AvailableCPU, worker.LatestCPU)
		fmt.Printf("%s║   Memory:        %.2f / %.2f / %.2f GB (%.2f%% used)\n", clearLine,
			worker.Info.TotalMemory, worker.AllocatedMemory, worker.AvailableMemory, worker.LatestMemory)
		fmt.Printf("%s║   Storage:       %.2f / %.2f / %.2f GB\n", clearLine,
			worker.Info.TotalStorage, worker.AllocatedStorage, worker.AvailableStorage)
		fmt.Printf("%s║\n", clearLine)
		fmt.Printf("%s║ Resource Utilization:\n", clearLine)
		cpuUtilPct := 0.0
		memUtilPct := 0.0
		storageUtilPct := 0.0
		if worker.Info.TotalCpu > 0 {
			cpuUtilPct = (worker.AllocatedCPU / worker.Info.TotalCpu) * 100
		}
		if worker.Info.TotalMemory > 0 {
			memUtilPct = (worker.AllocatedMemory / worker.Info.TotalMemory) * 100
		}
		if worker.Info.TotalStorage > 0 {
			storageUtilPct = (worker.AllocatedStorage / worker.Info.TotalStorage) * 100
		}
		fmt.Printf("%s║   CPU Allocated:   %.1f%%\n", clearLine, cpuUtilPct)
		fmt.Printf("%s║   Mem Allocated:   %.1f%%\n", clearLine, memUtilPct)
		fmt.Printf("%s║   Storage Alloc.:  %.1f%%\n", clearLine, storageUtilPct)
		fmt.Printf("%s║\n", clearLine)
		fmt.Printf("%s║ Running Tasks:   %d\n", clearLine, worker.TaskCount)
		fmt.Printf("%s╚═══════════════════════════════════════════════════", clearLine)
		// Print instruction on the line after the box (stays fixed)
		fmt.Print("\n\n(Press any key to exit)")

		// Mark that first render is complete
		if firstRender {
			firstRender = false
		}
	}

	// Initial render - call renderStats immediately to avoid "Loading..." flash
	renderStats()

	// Update loop
	for {
		select {
		case <-ticker.C:
			renderStats()
		case <-done:
			fmt.Print("\033[2B") // Move down 2 lines past the instruction
			fmt.Println("\nExiting worker stats monitor...")
			return
		}
	}
}

func (c *CLI) liveInternalState() {
	// ANSI escape codes
	const clearScreen = "\033[2J"
	const moveCursorHome = "\033[H"

	// Clear screen and move to home
	fmt.Print(clearScreen + moveCursorHome)

	// Create a ticker for updates (refresh every 2 seconds)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Channel to detect user input (to exit the live view)
	done := make(chan bool)

	// Goroutine to listen for any key press
	go func() {
		reader := bufio.NewReader(os.Stdin)
		reader.ReadByte() // Wait for any key press
		done <- true
	}()

	// Function to render the internal state
	renderState := func() {
		// Move cursor to home and clear screen
		fmt.Print(moveCursorHome)

		// Get and print the state
		output := c.masterServer.DumpInMemoryState()
		fmt.Print(output)

		// Print instruction at the bottom
		fmt.Print("(Press any key to exit)")
	}

	// Initial render
	renderState()

	// Update loop
	for {
		select {
		case <-ticker.C:
			renderState()
		case <-done:
			fmt.Print(clearScreen + moveCursorHome)
			fmt.Println("Exiting internal state monitor...")
			return
		}
	}
}

func (c *CLI) monitorTask(taskID string) {
	// ANSI escape codes for terminal control
	const (
		clearScreen = "\033[2J"
		moveCursor  = "\033[H"
		bold        = "\033[1m"
		reset       = "\033[0m"
		cyan        = "\033[36m"
		green       = "\033[32m"
		yellow      = "\033[33m"
		red         = "\033[31m"
	)

	// Get userID from task in database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userID, err := c.masterServer.GetUserIDForTask(ctx, taskID)
	if err != nil {
		fmt.Printf("\n%s❌ Failed to get task information: %v%s\n", red, err, reset)
		return
	}

	// Clear screen and show header
	fmt.Print(clearScreen + moveCursor)
	fmt.Printf("%s%s═══════════════════════════════════════════════════════%s\n", bold, cyan, reset)
	fmt.Printf("%s%s  TASK MONITOR - Live Logs%s\n", bold, cyan, reset)
	fmt.Printf("%s%s═══════════════════════════════════════════════════════%s\n", bold, cyan, reset)
	fmt.Printf("%sTask ID:%s %s\n", bold, reset, taskID)
	fmt.Printf("%sUser ID:%s %s\n", bold, reset, userID)
	fmt.Printf("%s%s───────────────────────────────────────────────────────%s\n", bold, cyan, reset)
	fmt.Printf("%s%sPress any key to exit%s\n\n", yellow, bold, reset)

	// Create context that can be cancelled
	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()

	// Channel to detect user input (to exit the live view)
	done := make(chan bool, 1)

	// Goroutine to listen for any key press
	go func() {
		reader := bufio.NewReader(os.Stdin)
		reader.ReadByte() // Wait for any key press
		done <- true
		streamCancel()
	}()

	// Channel to signal streaming completion
	streamDone := make(chan error, 1)

	// Start streaming logs in goroutine
	go func() {
		err := c.masterServer.StreamTaskLogsUnified(streamCtx, taskID, userID, func(logLine string, isComplete bool, status string) error {
			if logLine != "" {
				fmt.Println(logLine)
			}
			if isComplete {
				fmt.Printf("\n%s%s═══════════════════════════════════════════════════════%s\n", bold, green, reset)
				fmt.Printf("%s%s  Task Completed - Status: %s%s\n", bold, green, status, reset)
				fmt.Printf("%s%s═══════════════════════════════════════════════════════%s\n", bold, green, reset)
			}
			return nil
		})
		streamDone <- err
	}()

	// Wait for either user input or stream completion
	select {
	case <-done:
		fmt.Printf("\n%s%s═══════════════════════════════════════════════════════%s\n", bold, yellow, reset)
		fmt.Printf("%s%s  Monitoring Stopped by User%s\n", bold, yellow, reset)
		fmt.Printf("%s%s═══════════════════════════════════════════════════════%s\n", bold, yellow, reset)
	case err := <-streamDone:
		if err != nil {
			fmt.Printf("\n%s%s═══════════════════════════════════════════════════════%s\n", bold, red, reset)
			fmt.Printf("%s%s  Error: %v%s\n", bold, red, err, reset)
			fmt.Printf("%s%s═══════════════════════════════════════════════════════%s\n", bold, red, reset)
		}
		// Wait for user to press a key before returning to CLI
		fmt.Printf("\n%sPress any key to return to CLI...%s\n", yellow, reset)
		reader := bufio.NewReader(os.Stdin)
		reader.ReadByte()
	}
}
