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
	"strings"
	"sync"
	"time"

	"worker/internal/logstream"
	workermetrics "worker/internal/metrics"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-units"
)

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
	CPUSeconds      float64
	PeakMemoryBytes uint64
	IOBytes         uint64
}

// getBaseOutputDir returns the base output directory, using CLOUDAI_OUTPUT_DIR env var if set
func getBaseOutputDir() string {
	if dir := os.Getenv("CLOUDAI_OUTPUT_DIR"); dir != "" {
		return dir
	}
	return "/var/cloudai/outputs"
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

	usageSnapshot, usageErr := e.collectContainerUsage(ctx, containerID)
	if usageErr != nil {
		log.Printf("[Task %s] Warning: failed to collect container usage metrics: %v", taskID, usageErr)
	} else {
		workermetrics.Get().ObserveContainerUsage(taskType, usageSnapshot.CPUSeconds, usageSnapshot.PeakMemoryBytes, usageSnapshot.IOBytes)
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

func (e *TaskExecutor) collectContainerUsage(ctx context.Context, containerID string) (containerUsageSnapshot, error) {
	statsReader, err := e.dockerClient.ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		return containerUsageSnapshot{}, err
	}
	defer statsReader.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(statsReader.Body).Decode(&stats); err != nil {
		return containerUsageSnapshot{}, fmt.Errorf("decode stats response: %w", err)
	}

	peakMemoryBytes := maxUint64(
		stats.MemoryStats.MaxUsage,
		stats.MemoryStats.Usage,
		stats.MemoryStats.CommitPeak,
		stats.MemoryStats.Commit,
		stats.MemoryStats.PrivateWorkingSet,
	)

	ioBytes := sumBlkioBytes(stats.BlkioStats.IoServiceBytesRecursive)
	if ioBytes == 0 {
		ioBytes = stats.StorageStats.ReadSizeBytes + stats.StorageStats.WriteSizeBytes
	}

	return containerUsageSnapshot{
		CPUSeconds:      float64(stats.CPUStats.CPUUsage.TotalUsage) / 1e9,
		PeakMemoryBytes: peakMemoryBytes,
		IOBytes:         ioBytes,
	}, nil
}

// pullImage pulls a Docker image from registry
func (e *TaskExecutor) pullImage(ctx context.Context, imageName string) error {
	// Deterministic benchmark workflows are preloaded into each worker's DinD daemon.
	// If the image is already available locally, skip the registry pull and use the
	// local copy directly.
	if _, _, err := e.dockerClient.ImageInspectWithRaw(ctx, imageName); err == nil {
		log.Printf("Image %s already available locally; skipping pull", imageName)
		return nil
	}

	out, err := e.dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()

	// Read pull output (required to complete pull)
	_, err = io.Copy(io.Discard, out)
	return err
}

func sumBlkioBytes(entries []container.BlkioStatEntry) uint64 {
	var readWriteTotal uint64
	var matchedReadWrite bool
	var totalEntry uint64

	for _, entry := range entries {
		switch {
		case strings.EqualFold(entry.Op, "read"), strings.EqualFold(entry.Op, "write"):
			readWriteTotal += entry.Value
			matchedReadWrite = true
		case strings.EqualFold(entry.Op, "total"):
			totalEntry += entry.Value
		}
	}

	if matchedReadWrite {
		return readWriteTotal
	}
	if totalEntry > 0 {
		return totalEntry
	}

	var fallback uint64
	for _, entry := range entries {
		fallback += entry.Value
	}
	return fallback
}

func maxUint64(values ...uint64) uint64 {
	var max uint64
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
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

	// Prepare host config with resource limits and volume mount
	hostConfig := &container.HostConfig{
		Resources: container.Resources{},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: outputDir,
				Target: "/output",
			},
		},
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

// collectOutputFiles collects all files from the output directory
func (e *TaskExecutor) collectOutputFiles(outputDir string) ([]string, error) {
	var files []string

	// Check if directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return files, nil // No output directory, return empty list
	}

	// Walk through directory and collect all file paths
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, only collect files
		if !info.IsDir() {
			// Get relative path from output directory
			relPath, err := filepath.Rel(outputDir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
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
