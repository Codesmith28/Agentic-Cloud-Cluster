package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-units"
)

// TaskExecutor handles Docker container execution
type TaskExecutor struct {
	dockerClient *client.Client
	mu           sync.RWMutex
	containers   map[string]string // task_id -> container_id
}

// TaskResult contains the execution result
type TaskResult struct {
	TaskID   string
	Status   string // success, failed
	Logs     string
	ExitCode int64
	Error    error
}

// NewTaskExecutor creates a new task executor
func NewTaskExecutor() (*TaskExecutor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &TaskExecutor{
		dockerClient: cli,
		containers:   make(map[string]string),
	}, nil
}

// ExecuteTask pulls and runs a Docker container for the task with resource constraints
func (e *TaskExecutor) ExecuteTask(ctx context.Context, taskID, dockerImage, command string, reqCPU, reqMemory, reqGPU float64) *TaskResult {
	result := &TaskResult{
		TaskID: taskID,
		Status: "failed",
	}

	log.Printf("[Task %s] Starting execution...", taskID)

	// Pull the image
	log.Printf("[Task %s] Pulling image: %s", taskID, dockerImage)
	if err := e.pullImage(ctx, dockerImage); err != nil {
		result.Error = fmt.Errorf("failed to pull image: %w", err)
		result.Logs = fmt.Sprintf("Error pulling image: %v", err)
		return result
	}

	// Create container with resource limits
	log.Printf("[Task %s] Creating container with resource limits (CPU: %.2f, Memory: %.2fGB, GPU: %.2f)...",
		taskID, reqCPU, reqMemory, reqGPU)
	containerID, err := e.createContainer(ctx, dockerImage, command, taskID, reqCPU, reqMemory, reqGPU)
	if err != nil {
		result.Error = fmt.Errorf("failed to create container: %w", err)
		result.Logs = fmt.Sprintf("Error creating container: %v", err)
		return result
	}

	// Store container mapping
	e.mu.Lock()
	e.containers[taskID] = containerID
	e.mu.Unlock()

	defer func() {
		e.cleanup(ctx, containerID)
		e.mu.Lock()
		delete(e.containers, taskID)
		e.mu.Unlock()
	}()

	// Start container
	log.Printf("[Task %s] Starting container: %s", taskID, containerID[:12])
	if err := e.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		result.Error = fmt.Errorf("failed to start container: %w", err)
		result.Logs = fmt.Sprintf("Error starting container: %v", err)
		return result
	}

	// Collect logs
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
	}

	return result
}

// pullImage pulls a Docker image from registry
func (e *TaskExecutor) pullImage(ctx context.Context, imageName string) error {
	out, err := e.dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()

	// Read pull output (required to complete pull)
	_, err = io.Copy(io.Discard, out)
	return err
}

// createContainer creates a Docker container with resource limits
func (e *TaskExecutor) createContainer(ctx context.Context, image, command, taskID string, reqCPU, reqMemory, reqGPU float64) (string, error) {
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

	// Prepare host config with resource limits
	hostConfig := &container.HostConfig{
		Resources: container.Resources{},
	}

	// Set CPU limit (in nano CPUs: 1 CPU = 1e9 nano CPUs)
	if reqCPU > 0 {
		hostConfig.Resources.NanoCPUs = int64(reqCPU * 1e9)
	}

	// Set Memory limit (convert GB to bytes)
	if reqMemory > 0 {
		hostConfig.Resources.Memory = int64(reqMemory * units.GiB)
	}

	// Set GPU devices (if requested)
	if reqGPU > 0 {
		// Note: This is a simplified GPU allocation
		// In production, you'd use nvidia-docker runtime and proper device requests
		hostConfig.Runtime = "nvidia"
		// For proper GPU support, you'd need:
		// hostConfig.DeviceRequests = []container.DeviceRequest{
		//     {
		//         Count: int(reqGPU),
		//         Capabilities: [][]string{{"gpu"}},
		//     },
		// }
		log.Printf("[Task %s] GPU support requested but simplified implementation", taskID)
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

// collectLogs streams container logs
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
		logBuffer.WriteString(line + "\n")
	}

	return logBuffer.String(), scanner.Err()
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

// StreamLogs streams live logs from a container
// Returns a channel that receives log lines and an error channel
func (e *TaskExecutor) StreamLogs(ctx context.Context, containerID string) (<-chan string, <-chan error) {
	logChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(logChan)
		defer close(errChan)
		// Inspect container to determine if it was created with a TTY
		inspect, inspectErr := e.dockerClient.ContainerInspect(ctx, containerID)
		if inspectErr != nil {
			errChan <- fmt.Errorf("failed to inspect container: %w", inspectErr)
			return
		}

		// If container has TTY enabled, request logs without docker multiplexing/timestamps
		// so we can stream raw output promptly. Otherwise use stdcopy demux for multiplexed streams.
		logReader, err := e.dockerClient.ContainerLogs(ctx, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: false,
		})
		if err != nil {
			errChan <- fmt.Errorf("failed to get container logs: %w", err)
			return
		}
		defer logReader.Close()

		// If the container was created with a TTY, the logs are a raw stream (not multiplexed).
		if inspect.Config != nil && inspect.Config.Tty {
			// Stream raw bytes and forward promptly. Use a small buffer to avoid waiting for full lines.
			reader := bufio.NewReader(logReader)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Read up to newline; if none, ReadString will block until some data arrives.
				line, rerr := reader.ReadString('\n')
				if len(line) > 0 {
					select {
					case logChan <- strings.TrimRight(line, "\n"):
					case <-ctx.Done():
						return
					}
				}

				if rerr != nil {
					if rerr == io.EOF {
						return
					}
					errChan <- fmt.Errorf("error reading logs: %w", rerr)
					return
				}
			}
		}

		// Non-TTY container: logs are multiplexed (stdout/stderr). Demultiplex using stdcopy.
		// Create pipes for stdout and stderr
		stdoutReader, stdoutWriter := io.Pipe()
		stderrReader, stderrWriter := io.Pipe()

		// Start demultiplexing in background
		demuxDone := make(chan error, 1)
		go func() {
			_, derr := stdcopy.StdCopy(stdoutWriter, stderrWriter, logReader)
			stdoutWriter.Close()
			stderrWriter.Close()
			demuxDone <- derr
		}()

		// Helper function to read lines from a reader and forward
		readLines := func(reader io.Reader, done chan struct{}) {
			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				line := scanner.Text()
				select {
				case logChan <- line:
				case <-ctx.Done():
					return
				}
			}
			close(done)
		}

		stdoutDone := make(chan struct{})
		stderrDone := make(chan struct{})

		go readLines(stdoutReader, stdoutDone)
		go readLines(stderrReader, stderrDone)

		// Wait for either context cancellation or demux completion
		select {
		case <-ctx.Done():
			return
		case derr := <-demuxDone:
			if derr != nil && derr != io.EOF {
				errChan <- fmt.Errorf("error demultiplexing logs: %w", derr)
			}
			// Wait for both readers to finish
			<-stdoutDone
			<-stderrDone
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
