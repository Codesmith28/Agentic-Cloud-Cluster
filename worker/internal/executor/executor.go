package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// TaskExecutor handles Docker container execution
type TaskExecutor struct {
	dockerClient *client.Client
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
	}, nil
}

// ExecuteTask pulls and runs a Docker container for the task
func (e *TaskExecutor) ExecuteTask(ctx context.Context, taskID, dockerImage string) *TaskResult {
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

	// Create container
	log.Printf("[Task %s] Creating container...", taskID)
	containerID, err := e.createContainer(ctx, dockerImage, taskID)
	if err != nil {
		result.Error = fmt.Errorf("failed to create container: %w", err)
		result.Logs = fmt.Sprintf("Error creating container: %v", err)
		return result
	}
	defer e.cleanup(ctx, containerID)

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

// createContainer creates a Docker container
func (e *TaskExecutor) createContainer(ctx context.Context, image, taskID string) (string, error) {
	resp, err := e.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: image,
		},
		nil,
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
