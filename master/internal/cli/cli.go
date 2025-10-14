package cli

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"master/internal/server"
	pb "master/proto/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
		case "task":
			if len(parts) < 3 {
				fmt.Println("Usage: task <worker_id> <docker_image>")
				continue
			}
			c.assignTask(parts[1], parts[2])
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
	fmt.Println("  task <worker_id> <docker_img>  - Assign task to a worker")
	fmt.Println("  exit/quit                      - Shutdown master node")
	fmt.Println("\nExample:")
	fmt.Println("  task worker-1 docker.io/user/sample-task:latest")
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

func (c *CLI) assignTask(workerID, dockerImage string) {
	workers := c.masterServer.GetWorkers()

	if _, exists := workers[workerID]; !exists {
		fmt.Printf("❌ Error: Worker '%s' not found. Use 'workers' command to see registered workers.\n", workerID)
		return
	}

	fmt.Printf("Assigning task to worker %s...\n", workerID)
	fmt.Printf("Docker Image: %s\n", dockerImage)

	// Generate task ID
	taskID := fmt.Sprintf("task-%d", time.Now().Unix())

	task := &pb.Task{
		TaskId:      taskID,
		DockerImage: dockerImage,
		Command:     "", // Empty for now
		ReqCpu:      1.0,
		ReqMemory:   0.5,
		ReqStorage:  1.0,
		ReqGpu:      0.0,
	}

	// Get worker's address and send task
	worker := workers[workerID]
	workerAddr := fmt.Sprintf("%s:50052", worker.Info.WorkerIp) // Worker listens on 50052

	err := c.sendTaskToWorker(workerAddr, task)
	if err != nil {
		fmt.Printf("❌ Failed to assign task: %v\n", err)
		return
	}

	fmt.Printf("✅ Task %s assigned successfully!\n", taskID)
}

func (c *CLI) sendTaskToWorker(workerAddr string, task *pb.Task) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, workerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("failed to connect to worker: %w", err)
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	ack, err := client.AssignTask(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to assign task: %w", err)
	}

	if !ack.Success {
		return fmt.Errorf("task assignment rejected: %s", ack.Message)
	}

	return nil
}
