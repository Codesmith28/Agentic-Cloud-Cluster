package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/constants"
	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/envutil"
	"worker/internal/logstream"
	workermetrics "worker/internal/metrics"
	"worker/internal/system"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-units"
)

const (
	maxLogBytes  = 10 * 1024 * 1024 // 10 MB cap for collected logs
	maxPidsLimit = constants.DefaultContainerPIDLimit
)

// validTaskID matches alphanumeric, hyphens, and underscores only.
var validTaskID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_\-]{0,253}$`)

// validDockerImage matches standard Docker image references (registry/repo:tag@digest).
var validDockerImage = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-/]*(:[a-zA-Z0-9._\-]+)?(@sha256:[a-f0-9]{64})?$`)

// ValidateTaskID checks that a task ID is safe for use in file paths and container names.
func ValidateTaskID(taskID string) error {
	if taskID == "" {
		return fmt.Errorf("task ID must not be empty")
	}
	if !validTaskID.MatchString(taskID) {
		return fmt.Errorf("task ID %q contains invalid characters", taskID)
	}
	if strings.Contains(taskID, "..") {
		return fmt.Errorf("task ID %q must not contain '..'", taskID)
	}
	return nil
}

// ValidateDockerImage checks that a Docker image reference is well-formed.
func ValidateDockerImage(img string) error {
	if img == "" {
		return fmt.Errorf("docker image must not be empty")
	}
	if !validDockerImage.MatchString(img) {
		return fmt.Errorf("docker image %q is not a valid image reference", img)
	}
	return nil
}

// TaskExecutor handles Docker container execution
type TaskExecutor struct {
	dockerClient *client.Client
	logStreamMgr *logstream.LogStreamManager
	mu           sync.RWMutex
	containers   map[string]string // task_id -> container_id
}

// TaskResult contains the execution result
type TaskResult struct {
	TaskID         string
	Status         string // success, failed
	Logs           string
	ExitCode       int64
	Error          error
	ResultLocation string   // Path to output directory on worker
	OutputFiles    []string // List of output files relative to ResultLocation
}

type containerUsageSnapshot struct {
	cpuSeconds      float64
	memoryPeakBytes uint64
	ioBytes         uint64
}

// getBaseOutputDir returns the base output directory using configured environment variables or default.
func getBaseOutputDir() string {
	return envutil.GetEnv(constants.EnvOutputDir, envutil.GetEnv(constants.EnvLegacyOutputDir, constants.DefaultOutputDir))
}

// NewTaskExecutor creates a new task executor
func NewTaskExecutor() (*TaskExecutor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &TaskExecutor{
		dockerClient: cli,
		logStreamMgr: logstream.NewLogStreamManager(cli),
		containers:   make(map[string]string),
	}, nil
}

// ExecuteTask pulls and runs a Docker container for the task with resource constraints.
func (e *TaskExecutor) ExecuteTask(ctx context.Context, taskID, dockerImage, command string, reqCPU, reqMemory float64, taskType string) *TaskResult {
	result := &TaskResult{
		TaskID: taskID,
		Status: "failed",
	}

	// Validate inputs before proceeding
	if err := ValidateTaskID(taskID); err != nil {
		result.Error = fmt.Errorf("invalid task ID: %w", err)
		result.Logs = fmt.Sprintf("Rejected: %v", err)
		return result
	}
	if err := ValidateDockerImage(dockerImage); err != nil {
		result.Error = fmt.Errorf("invalid docker image: %w", err)
		result.Logs = fmt.Sprintf("Rejected: %v", err)
		return result
	}

	executionStarted := time.Now()
	workermetrics.Get().IncTaskStart(taskType)

	log.Printf("[Task %s] Starting execution...", taskID)

	// Pull the image
	log.Printf("[Task %s] Pulling image: %s", taskID, dockerImage)
	pullStarted := time.Now()
	if err := e.pullImage(ctx, dockerImage); err != nil {
		workermetrics.Get().IncDockerError("image_pull", taskType)
		result.Error = fmt.Errorf("failed to pull image: %w", err)
		result.Logs = fmt.Sprintf("Error pulling image: %v", err)
		return result
	}
	workermetrics.Get().ObserveImagePull(taskType, pullStarted)

	// Create container with resource limits
	log.Printf("[Task %s] Creating container with resource limits (CPU: %.2f, Memory: %.2fGB)...",
		taskID, reqCPU, reqMemory)
	createStarted := time.Now()
	containerID, err := e.createContainer(ctx, dockerImage, command, taskID, reqCPU, reqMemory)
	if err != nil {
		workermetrics.Get().IncDockerError("container_create", taskType)
		result.Error = fmt.Errorf("failed to create container: %w", err)
		result.Logs = fmt.Sprintf("Error creating container: %v", err)
		return result
	}
	workermetrics.Get().ObserveContainerCreate(taskType, createStarted)

	// Store container mapping
	e.mu.Lock()
	e.containers[taskID] = containerID
	e.mu.Unlock()

	defer func() {
		// Stop log streaming when task completes
		e.logStreamMgr.StopTask(taskID)

		e.cleanup(ctx, containerID)
		e.mu.Lock()
		delete(e.containers, taskID)
		e.mu.Unlock()
	}()

	// Start container
	log.Printf("[Task %s] Starting container: %s", taskID, containerID[:12])
	if err := e.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		workermetrics.Get().IncDockerError("container_start", taskType)
		result.Error = fmt.Errorf("failed to start container: %w", err)
		result.Logs = fmt.Sprintf("Error starting container: %v", err)
		return result
	}

	// Start log streaming for this task
	if err := e.logStreamMgr.StartTask(taskID, containerID); err != nil {
		log.Printf("[Task %s] Warning: failed to start log streaming: %v", taskID, err)
	}

	// Collect logs for final result
	logs, err := e.collectLogs(ctx, containerID)
	if err != nil {
		log.Printf("[Task %s] Warning: failed to collect logs: %v", taskID, err)
	}
	result.Logs = logs

	// Wait for container to complete
	log.Printf("[Task %s] Waiting for completion...", taskID)
	statusCh, errCh := e.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		if err != nil {
			workermetrics.Get().IncDockerError("container_wait", taskType)
			result.Error = fmt.Errorf("error waiting for container: %w", err)
			result.Status = "failed"
			return result
		}
	case status := <-statusCh:
		result.ExitCode = status.StatusCode
		if status.StatusCode == 0 {
			result.Status = "success"
			log.Printf("[Task %s] ✓ Completed successfully", taskID)
		} else {
			result.Status = "failed"
			result.Error = fmt.Errorf("container exited with code %d", status.StatusCode)
			log.Printf("[Task %s] ✗ Failed with exit code %d", taskID, status.StatusCode)
		}

		// Print task completion banner
		log.Println(" ")
		log.Println("═══════════════════════════════════════════════════════")
		if status.StatusCode == 0 {
			log.Println("  ✅ TASK COMPLETED SUCCESSFULLY")
		} else {
			log.Println("  ❌ TASK FAILED")
		}
		log.Println("═══════════════════════════════════════════════════════")
		log.Printf("  Task ID:           %s", taskID)
		log.Printf("  Docker Image:      %s", dockerImage)
		log.Printf("  Command:           %s", command)
		log.Printf("  Exit Code:         %d", status.StatusCode)
		log.Println("───────────────────────────────────────────────────────")
		log.Println("  Resources Released:")
		log.Printf("    • CPU Cores:     %.2f cores", reqCPU)
		log.Printf("    • Memory:        %.2f GB", reqMemory)
		log.Println("═══════════════════════════════════════════════════════")
		log.Println("")
	}
	workermetrics.Get().ObserveTaskRuntime(taskType, result.Status, executionStarted)
	if usage, err := e.collectContainerUsage(containerID); err != nil {
		log.Printf("[Task %s] Warning: failed to collect container usage stats: %v", taskID, err)
	} else {
		workermetrics.Get().ObserveContainerUsage(taskType, usage.cpuSeconds, usage.memoryPeakBytes, usage.ioBytes)
	}

	// Collect output files
	outputDir := filepath.Join(getBaseOutputDir(), taskID)
	outputFiles, err := e.collectOutputFiles(outputDir)
	if err != nil {
		log.Printf("[Task %s] Warning: failed to collect output files: %v", taskID, err)
	} else {
		result.ResultLocation = outputDir
		result.OutputFiles = outputFiles
		if len(outputFiles) > 0 {
			log.Printf("[Task %s] ✓ Collected %d output file(s)", taskID, len(outputFiles))
		}
	}

	return result
}

// pullImage pulls a Docker image from registry
func (e *TaskExecutor) pullImage(ctx context.Context, imageName string) error {
	out, err := e.dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		// In offline/dev environments, allow execution to continue when the image
		// is already present locally but registry pull is unavailable.
		if _, _, inspectErr := e.dockerClient.ImageInspectWithRaw(ctx, imageName); inspectErr == nil {
			log.Printf("⚠️  Image pull failed for %s; using local image instead: %v", imageName, err)
			return nil
		}
		return err
	}
	defer out.Close()

	// Read pull output (required to complete pull)
	_, err = io.Copy(io.Discard, out)
	return err
}

// createContainer creates a Docker container with resource limits.
func (e *TaskExecutor) createContainer(ctx context.Context, image, command, taskID string, reqCPU, reqMemory float64) (string, error) {
	// Prepare container config
	containerConfig := &container.Config{
		Image: image,
	}

	// Use a TTY so many programs flush stdout line-by-line instead of block-buffering
	// when their stdout is not a TTY. This improves live log streaming behavior.
	containerConfig.Tty = true
	containerConfig.AttachStdout = true
	containerConfig.AttachStderr = true

	// Add command if provided
	if command != "" {
		containerConfig.Cmd = []string{"/bin/sh", "-c", command}
	}

	// Create output directory on host with secure permissions
	outputDir := filepath.Join(getBaseOutputDir(), taskID)
	if err := os.MkdirAll(outputDir, 0700); err != nil { // drwx------ (owner only)
		return "", fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}
	log.Printf("[Task %s] ✓ Created secure output directory: %s", taskID, outputDir)

	// Prepare host config with resource limits, volume mount, and security hardening
	networkMode := system.ResolveWorkerContainerNetworkMode()
	pidsLimit := int64(maxPidsLimit)
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkMode),
		Resources: container.Resources{
			PidsLimit: &pidsLimit,
		},
		SecurityOpt: []string{"no-new-privileges"},
		ReadonlyRootfs: false, // tasks may need to write; /output is bind-mounted
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: outputDir,
				Target: "/output",
			},
		},
		CapDrop: []string{"ALL"},
		CapAdd:  []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID"},
	}

	// Set CPU limit (in nano CPUs: 1 CPU = 1e9 nano CPUs)
	if reqCPU > 0 {
		hostConfig.Resources.NanoCPUs = int64(reqCPU * 1e9)
	}

	// Set Memory limit (convert GB to bytes)
	if reqMemory > 0 {
		hostConfig.Resources.Memory = int64(reqMemory * units.GiB)
	}

	resp, err := e.dockerClient.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		fmt.Sprintf("task-%s", taskID),
	)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// collectLogs streams container logs with a size cap to prevent OOM.
func (e *TaskExecutor) collectLogs(ctx context.Context, containerID string) (string, error) {
	logReader, err := e.dockerClient.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	if err != nil {
		return "", err
	}
	defer logReader.Close()

	var logBuffer bytes.Buffer
	scanner := bufio.NewScanner(logReader)

	for scanner.Scan() {
		line := scanner.Text()
		// Remove Docker log header (first 8 bytes)
		if len(line) > 8 {
			line = line[8:]
		}
		if logBuffer.Len()+len(line)+1 > maxLogBytes {
			logBuffer.WriteString("\n... [log output truncated at 10 MB] ...\n")
			break
		}
		logBuffer.WriteString(line + "\n")
	}

	return logBuffer.String(), scanner.Err()
}

// collectOutputFiles collects all files from the output directory.
// It validates that all files are within the expected directory to prevent traversal.
func (e *TaskExecutor) collectOutputFiles(outputDir string) ([]string, error) {
	var files []string

	// Check if directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return files, nil // No output directory, return empty list
	}

	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve output directory: %w", err)
	}

	// Walk through directory and collect all file paths
	err = filepath.Walk(absOutputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(absPath, absOutputDir) {
			return fmt.Errorf("path %q escapes output directory", path)
		}

		// Skip directories, only collect files
		if !info.IsDir() {
			relPath, err := filepath.Rel(absOutputDir, absPath)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

func (e *TaskExecutor) collectContainerUsage(containerID string) (containerUsageSnapshot, error) {
	snapshot := containerUsageSnapshot{}
	statsCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stats, err := e.fetchContainerStats(statsCtx, containerID)
	if err != nil {
		return snapshot, err
	}

	snapshot.cpuSeconds = float64(stats.CPUStats.CPUUsage.TotalUsage) / float64(time.Second)
	if stats.MemoryStats.MaxUsage > 0 {
		snapshot.memoryPeakBytes = stats.MemoryStats.MaxUsage
	} else {
		snapshot.memoryPeakBytes = stats.MemoryStats.Usage
	}
	snapshot.ioBytes = sumContainerIOBytes(stats.BlkioStats.IoServiceBytesRecursive)

	return snapshot, nil
}

func (e *TaskExecutor) fetchContainerStats(ctx context.Context, containerID string) (container.StatsResponse, error) {
	statsReader, err := e.dockerClient.ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		statsReader, err = e.dockerClient.ContainerStats(ctx, containerID, false)
		if err != nil {
			return container.StatsResponse{}, err
		}
	}
	defer statsReader.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(statsReader.Body).Decode(&stats); err != nil {
		return container.StatsResponse{}, err
	}
	return stats, nil
}

func sumContainerIOBytes(entries []container.BlkioStatEntry) uint64 {
	var allOpsTotal uint64
	var readWriteTotal uint64

	for _, entry := range entries {
		allOpsTotal += entry.Value
		switch strings.ToLower(entry.Op) {
		case "read", "write":
			readWriteTotal += entry.Value
		}
	}

	if readWriteTotal > 0 {
		return readWriteTotal
	}
	return allOpsTotal
}

// cleanup removes the container
func (e *TaskExecutor) cleanup(ctx context.Context, containerID string) {
	timeoutSecs := 5
	if err := e.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSecs}); err != nil {
		log.Printf("Warning: failed to stop container %s: %v", containerID[:12], err)
	}

	if err := e.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		log.Printf("Warning: failed to remove container %s: %v", containerID[:12], err)
	}
}

// Close closes the Docker client
func (e *TaskExecutor) Close() error {
	if e.dockerClient != nil {
		return e.dockerClient.Close()
	}
	return nil
}

// GetContainerID returns the container ID for a given task ID
func (e *TaskExecutor) GetContainerID(taskID string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	containerID, exists := e.containers[taskID]
	return containerID, exists
}

// StreamLogs subscribes to live logs from a container via the log stream manager
// Returns a channel that receives log lines and an error channel
// This uses the broadcaster pattern to support multiple subscribers efficiently
func (e *TaskExecutor) StreamLogs(ctx context.Context, taskID string) (<-chan string, <-chan error) {
	logChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(logChan)
		defer close(errChan)

		// Subscribe to logs via the manager (sends recent logs + live stream)
		logLineChan, err := e.logStreamMgr.Subscribe(ctx, taskID, true)
		if err != nil {
			errChan <- fmt.Errorf("failed to subscribe to logs: %w", err)
			return
		}

		// Forward log lines to the string channel
		for {
			select {
			case logLine, ok := <-logLineChan:
				if !ok {
					// Log stream closed
					return
				}
				// Send the log content
				select {
				case logChan <- logLine.Content:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return logChan, errChan
}

// GetContainerStatus returns the status of a container
func (e *TaskExecutor) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	inspect, err := e.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	if inspect.State.Running {
		return "running", nil
	} else if inspect.State.Status == "exited" {
		if inspect.State.ExitCode == 0 {
			return "completed", nil
		}
		return "failed", nil
	}

	return inspect.State.Status, nil
}

// CancelTask stops and removes a running task's container
func (e *TaskExecutor) CancelTask(ctx context.Context, taskID string) error {
	e.mu.RLock()
	containerID, exists := e.containers[taskID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task %s not found or not running", taskID)
	}

	log.Printf("[Task %s] Cancelling task (container: %s)...", taskID, containerID[:12])

	// Stop the container with a timeout
	timeoutSecs := 10
	if err := e.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSecs}); err != nil {
		log.Printf("[Task %s] Warning: failed to stop container gracefully: %v", taskID, err)
		// Try to kill it forcefully
		if killErr := e.dockerClient.ContainerKill(ctx, containerID, "SIGKILL"); killErr != nil {
			return fmt.Errorf("failed to kill container: %w", killErr)
		}
	}

	// Remove the container
	if err := e.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		log.Printf("[Task %s] Warning: failed to remove container: %v", taskID, err)
	}

	// Remove from tracking
	e.mu.Lock()
	delete(e.containers, taskID)
	e.mu.Unlock()

	log.Printf("[Task %s] ✓ Task cancelled successfully", taskID)
	return nil
}

// GetLogStreamManager returns the log stream manager for direct access
func (e *TaskExecutor) GetLogStreamManager() *logstream.LogStreamManager {
	return e.logStreamMgr
}

// GetRunningTasks returns a list of all currently running task IDs
func (e *TaskExecutor) GetRunningTasks() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tasks := make([]string, 0, len(e.containers))
	for taskID := range e.containers {
		tasks = append(tasks, taskID)
	}
	return tasks
}
