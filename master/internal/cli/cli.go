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
	fmt.Println("  register <id> <ip:port>        - Manually register a worker")
	fmt.Println("  unregister <id>                - Unregister a worker")
	fmt.Println("  task <worker_id> <docker_img> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]  - Assign task to specific worker")
	fmt.Println("  exit/quit                      - Shutdown master node")
	fmt.Println("\nExamples:")
	fmt.Println("  register worker-2 192.168.1.100:50052")
	fmt.Println("  task worker-1 docker.io/user/sample-task:latest")
	fmt.Println("  task worker-2 docker.io/user/sample-task:latest -cpu_cores 2.0 -mem 1.0 -gpu_cores 1.0")
}

func (c *CLI) showStatus() {
	workers := c.masterServer.GetWorkers()

	fmt.Println("\n╔═══ Cluster Status ═══")
	fmt.Printf("║ Total Workers: %d\n", len(workers))

	activeCount := 0
	totalTasks := 0
	for _, w := range workers {
		if w.IsActive {
			activeCount++
		}
		totalTasks += len(w.RunningTasks)
	}

	fmt.Printf("║ Active Workers: %d\n", activeCount)
	fmt.Printf("║ Running Tasks: %d\n", totalTasks)
	fmt.Println("╚══════════════════════")
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

	fmt.Printf("Assigning task to worker %s...\n", workerID)
	fmt.Printf("Docker Image: %s\n", dockerImage)
	fmt.Printf("Resources: CPU=%.1f cores, Memory=%.1fGB, Storage=%.1fGB, GPU=%.1f cores\n",
		reqCPU, reqMemory, reqStorage, reqGPU)

	// Generate task ID
	taskID := fmt.Sprintf("task-%d", time.Now().Unix())

	// Construct Docker run command with resource limits
	command := c.buildDockerCommand(dockerImage, reqCPU, reqMemory, reqStorage, reqGPU)

	task := &pb.Task{
		TaskId:         taskID,
		DockerImage:    dockerImage,
		Command:        command,
		ReqCpu:         reqCPU,
		ReqMemory:      reqMemory,
		ReqStorage:     reqStorage,
		ReqGpu:         reqGPU,
		TargetWorkerId: workerID, // Always required
	}

	err := c.assignTaskViaMaster(task)
	if err != nil {
		fmt.Printf("❌ Failed to assign task: %v\n", err)
		return
	}

	fmt.Printf("✅ Task %s assigned successfully!\n", taskID)
}

func (c *CLI) buildDockerCommand(dockerImage string, cpu, memory, storage, gpu float64) string {
	// Build Docker run command with resource constraints
	cmd := fmt.Sprintf("docker run --rm")

	// Add CPU limit
	if cpu > 0 {
		cmd += fmt.Sprintf(" --cpus=%.1f", cpu)
	}

	// Add memory limit
	if memory > 0 {
		cmd += fmt.Sprintf(" --memory=%.1fg", memory)
	}

	// Add GPU support if requested
	if gpu > 0 {
		cmd += fmt.Sprintf(" --gpus=%.1f", gpu)
	}

	// Add the image
	cmd += fmt.Sprintf(" %s", dockerImage)

	return cmd
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

	err := c.masterServer.ManualRegisterWorker(ctx, workerID, workerIP)
	if err != nil {
		fmt.Printf("❌ Failed to register worker: %v\n", err)
		return
	}

	fmt.Printf("✅ Worker %s registered with address %s\n", workerID, workerIP)
	fmt.Println("   Note: Worker will send full specs when it connects")
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
