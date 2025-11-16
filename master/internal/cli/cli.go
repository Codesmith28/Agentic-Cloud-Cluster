package cli

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"master/internal/server"
	pb "master/proto"
)

// CLI handles the command-line interface for the master
type CLI struct {
	masterServer *server.MasterServer
	reader       *bufio.Reader
}

// NewCLI creates a new CLI instance
func NewCLI(srv *server.MasterServer) *CLI {
	return &CLI{
		masterServer: srv,
		reader:       bufio.NewReader(os.Stdin),
	}
}

// Run starts the interactive CLI
func (c *CLI) Run() {
	c.printBanner()

	for {
		fmt.Print("\nmaster> ")
		input, err := c.reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		command := parts[0]

		switch command {
		case "help":
			c.printHelp()
		case "status":
			c.showStatus()
		case "workers":
			c.listWorkers()
		case "stats":
			if len(parts) < 2 {
				fmt.Println("Usage: stats <worker_id>")
				fmt.Println("Example: stats worker-1")
				continue
			}
			c.showWorkerStats(parts[1])
		case "register":
			if len(parts) < 3 {
				fmt.Println("Usage: register <worker_id> <worker_ip:port>")
				fmt.Println("Example: register worker-1 192.168.1.100:50052")
				continue
			}
			c.registerWorker(parts[1], parts[2])
		case "unregister":
			if len(parts) < 2 {
				fmt.Println("Usage: unregister <worker_id>")
				continue
			}
			c.unregisterWorker(parts[1])
		case "task":
			if len(parts) < 3 {
				fmt.Println("Usage: task <worker_id> <docker_image> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]")
				fmt.Println("  worker_id: specific worker ID to assign the task to")
				fmt.Println("  docker_image: Docker image to run")
				fmt.Println("  -cpu_cores: CPU cores to allocate (default: 1.0)")
				fmt.Println("  -mem: Memory in GB (default: 0.5)")
				fmt.Println("  -storage: Storage in GB (default: 1.0)")
				fmt.Println("  -gpu_cores: GPU cores to allocate (default: 0.0)")
				fmt.Println("Examples:")
				fmt.Println("  task worker-1 docker.io/user/sample-task:latest")
				fmt.Println("  task worker-2 docker.io/user/sample-task:latest -cpu_cores 2.0 -mem 1.0 -gpu_cores 1.0")
				continue
			}
			c.assignTask(parts)
		case "monitor":
			if len(parts) < 2 {
				fmt.Println("Usage: monitor <task_id> [user_id]")
				fmt.Println("  task_id: ID of the task to monitor")
				fmt.Println("  user_id: (optional) User ID for authorization (default: admin)")
				fmt.Println("Example: monitor task-123 user-1")
				continue
			}
			userID := "admin"
			if len(parts) >= 3 {
				userID = parts[2]
			}
			c.monitorTask(parts[1], userID)
		case "cancel":
			if len(parts) < 2 {
				fmt.Println("Usage: cancel <task_id>")
				fmt.Println("  task_id: ID of the task to cancel")
				fmt.Println("Example: cancel task-123")
				continue
			}
			c.cancelTask(parts[1])
		case "exit", "quit":
			fmt.Println("Shutting down master...")
			return
		default:
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
		}
	}
}

func (c *CLI) printBanner() {
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  CloudAI Master Node - Interactive CLI")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("Type 'help' for available commands")
}

func (c *CLI) printHelp() {
	fmt.Println("\nAvailable commands:")
	fmt.Println("  help                           - Show this help message")
	fmt.Println("  status                         - Show cluster status")
	fmt.Println("  workers                        - List all registered workers")
	fmt.Println("  stats <worker_id>              - Show detailed stats for a worker")
	fmt.Println("  register <id> <ip:port>        - Manually register a worker")
	fmt.Println("  unregister <id>                - Unregister a worker")
	fmt.Println("  task <worker_id> <docker_img> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]  - Assign task to specific worker")
	fmt.Println("  monitor <task_id> [user_id]    - Monitor live logs for a task (press any key to exit)")
	fmt.Println("  cancel <task_id>               - Cancel a running task")
	fmt.Println("  exit/quit                      - Shutdown master node")
	fmt.Println("\nExamples:")
	fmt.Println("  register worker-2 192.168.1.100:50052")
	fmt.Println("  stats worker-1")
	fmt.Println("  task worker-1 docker.io/user/sample-task:latest")
	fmt.Println("  task worker-2 docker.io/user/sample-task:latest -cpu_cores 2.0 -mem 1.0 -gpu_cores 1.0")
	fmt.Println("  monitor task-123 user-1")
	fmt.Println("  cancel task-123")
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

func (c *CLI) listWorkers() {
	workers := c.masterServer.GetWorkers()

	if len(workers) == 0 {
		fmt.Println("No workers registered yet.")
		return
	}

	fmt.Println("\n╔═══ Registered Workers ═══")
	for id, w := range workers {
		status := "🟢 Active"
		if !w.IsActive {
			status = "🔴 Inactive"
		}

		fmt.Printf("║ %s\n", id)
		fmt.Printf("║   Status: %s\n", status)
		fmt.Printf("║   IP: %s\n", w.Info.WorkerIp)
		fmt.Printf("║   Resources: CPU=%.1f, Memory=%.1fGB, GPU=%.1f\n",
			w.Info.TotalCpu, w.Info.TotalMemory, w.Info.TotalGpu)
		fmt.Printf("║   Running Tasks: %d\n", len(w.RunningTasks))
		fmt.Println("║")
	}
	fmt.Println("╚═══════════════════════")
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

	// Function to render the worker stats
	renderStats := func() {
		worker, exists := c.masterServer.GetWorkerStats(workerID)
		if !exists {
			fmt.Print("\033[15A") // Move up
			fmt.Print("\r")
			for i := 0; i < 15; i++ {
				fmt.Print(clearLine + "\r\n")
			}
			fmt.Print("\033[15A")
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
		// Box has 13 lines + 1 blank line + 1 instruction line = 15 lines total
		fmt.Print("\033[15A")
		fmt.Print("\r") // Move to beginning of line

		// Clear and redraw stats box (no right border)
		fmt.Printf("%s╔═══════════════════════════════════════════════════\n", clearLine)
		fmt.Printf("%s║ Worker: %s\n", clearLine, workerID)
		fmt.Printf("%s╠═══════════════════════════════════════════════════\n", clearLine)
		fmt.Printf("%s║ Status:          %s\n", clearLine, status)
		fmt.Printf("%s║ Address:         %s\n", clearLine, worker.Info.WorkerIp)
		fmt.Printf("%s║ Last Seen:       %s\n", clearLine, lastSeen)
		fmt.Printf("%s║\n", clearLine)
		fmt.Printf("%s║ Resources:\n", clearLine)
		fmt.Printf("%s║   CPU:           %.2f cores (%.1f%% used)\n", clearLine, worker.Info.TotalCpu, worker.LatestCPU)
		fmt.Printf("%s║   Memory:        %.2f GB (%.2f%% used)\n", clearLine, worker.Info.TotalMemory, worker.LatestMemory)
		fmt.Printf("%s║   GPU:           %.2f cores (%.1f%% used)\n", clearLine, worker.Info.TotalGpu, worker.LatestGPU)
		fmt.Printf("%s║\n", clearLine)
		fmt.Printf("%s║ Running Tasks:   %d\n", clearLine, worker.TaskCount)
		fmt.Printf("%s╚═══════════════════════════════════════════════════", clearLine)
		// Print instruction on the line after the box (stays fixed)
		fmt.Print("\n\n(Press any key to exit)")
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

func (c *CLI) assignTask(parts []string) {
	workerID := parts[1]
	dockerImage := parts[2]

	// Default resource requirements
	reqCPU := 1.0
	reqMemory := 0.5
	reqStorage := 1.0
	reqGPU := 0.0

	// Parse flags
	for i := 3; i < len(parts); i++ {
		switch parts[i] {
		case "-cpu_cores":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqCPU = val
					i++ // Skip the value
				}
			}
		case "-mem":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqMemory = val
					i++ // Skip the value
				}
			}
		case "-storage":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqStorage = val
					i++ // Skip the value
				}
			}
		case "-gpu_cores":
			if i+1 < len(parts) {
				if val, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
					reqGPU = val
					i++ // Skip the value
				}
			}
		}
	}

	// Generate task ID
	taskID := fmt.Sprintf("task-%d", time.Now().Unix())

	// Command is empty - the container will use its default CMD/ENTRYPOINT
	// If user wants to override, they can pass -cmd flag (future feature)
	command := ""

	// Display task details before sending
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  📤 SENDING TASK TO WORKER")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  Task ID:           %s\n", taskID)
	fmt.Printf("  Target Worker:     %s\n", workerID)
	fmt.Printf("  Docker Image:      %s\n", dockerImage)
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  Resource Requirements:")
	fmt.Printf("    • CPU Cores:     %.2f cores\n", reqCPU)
	fmt.Printf("    • Memory:        %.2f GB\n", reqMemory)
	fmt.Printf("    • Storage:       %.2f GB\n", reqStorage)
	fmt.Printf("    • GPU Cores:     %.2f cores\n", reqGPU)
	fmt.Println("═══════════════════════════════════════════════════════")

	task := &pb.Task{
		TaskId:         taskID,
		DockerImage:    dockerImage,
		Command:        command,
		ReqCpu:         reqCPU,
		ReqMemory:      reqMemory,
		ReqStorage:     reqStorage,
		ReqGpu:         reqGPU,
		TargetWorkerId: workerID, // Always required
		UserId:         "admin",  // Default user for CLI tasks (can be made configurable)
	}

	err := c.assignTaskViaMaster(task)
	if err != nil {
		fmt.Printf("\n❌ Failed to assign task: %v\n", err)
		return
	}

	fmt.Printf("\n✅ Task %s assigned successfully!\n", taskID)
}

func (c *CLI) assignTaskViaMaster(task *pb.Task) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ack, err := c.masterServer.AssignTask(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to assign task: %w", err)
	}

	if !ack.Success {
		return fmt.Errorf("task assignment failed: %s", ack.Message)
	}

	return nil
}

func (c *CLI) registerWorker(workerID, workerIP string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get master info
	masterID, masterAddress := c.masterServer.GetMasterInfo()
	if masterID == "" || masterAddress == "" {
		fmt.Println("❌ Master info not set. Cannot register worker.")
		return
	}

	// Use ManualRegisterAndNotify to both register and notify the worker
	err := c.masterServer.ManualRegisterAndNotify(ctx, workerID, workerIP, masterID, masterAddress)
	if err != nil {
		fmt.Printf("❌ Failed to register worker: %v\n", err)
		return
	}

	fmt.Printf("✅ Worker %s registered with address %s\n", workerID, workerIP)
	fmt.Println("   Master is notifying worker... Check logs for confirmation.")
}

func (c *CLI) unregisterWorker(workerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.masterServer.UnregisterWorker(ctx, workerID)
	if err != nil {
		fmt.Printf("❌ Failed to unregister worker: %v\n", err)
		return
	}

	fmt.Printf("✅ Worker %s has been unregistered\n", workerID)
}

func (c *CLI) monitorTask(taskID, userID string) {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to detect user input (to exit the live view)
	done := make(chan bool, 1)

	// Goroutine to listen for any key press
	go func() {
		reader := bufio.NewReader(os.Stdin)
		reader.ReadByte() // Wait for any key press
		done <- true
		cancel()
	}()

	// Channel to signal streaming completion
	streamDone := make(chan error, 1)

	// Start streaming logs in goroutine
	go func() {
		err := c.masterServer.StreamTaskLogsFromWorker(ctx, taskID, userID, func(logLine string, isComplete bool) {
			if logLine != "" {
				fmt.Println(logLine)
			}
			if isComplete {
				fmt.Printf("\n%s%s═══════════════════════════════════════════════════════%s\n", bold, green, reset)
				fmt.Printf("%s%s  Task Completed%s\n", bold, green, reset)
				fmt.Printf("%s%s═══════════════════════════════════════════════════════%s\n", bold, green, reset)
			}
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

func (c *CLI) cancelTask(taskID string) {
	// ANSI escape codes
	const (
		bold   = "\033[1m"
		reset  = "\033[0m"
		red    = "\033[31m"
		green  = "\033[32m"
		yellow = "\033[33m"
	)

	fmt.Println()
	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", bold, red, reset)
	fmt.Printf("%s%s  🛑 CANCELLING TASK%s\n", bold, red, reset)
	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", bold, red, reset)
	fmt.Printf("%s  Task ID:%s %s\n", bold, reset, taskID)
	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", bold, red, reset)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ack, err := c.masterServer.CancelTask(ctx, &pb.TaskID{TaskId: taskID})
	if err != nil {
		fmt.Printf("\n%s❌ Error cancelling task:%s %v\n", red, reset, err)
		return
	}

	if !ack.Success {
		fmt.Printf("\n%s❌ Failed to cancel task:%s %s\n", red, reset, ack.Message)
		return
	}

	fmt.Printf("\n%s✅ Task cancelled successfully!%s\n", green, reset)
	fmt.Printf("%s   %s%s\n", yellow, ack.Message, reset)
}
