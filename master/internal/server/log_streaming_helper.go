package server

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "master/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// LogStreamHandler is a function type that handles incoming log lines
// logLine: the log content
// isComplete: true if this is the final log (task completed)
// status: current task status (running, success, failed, etc.)
type LogStreamHandler func(logLine string, isComplete bool, status string) error

// StreamTaskLogsUnified is a unified function to stream logs from a worker
// This can be used by both CLI and web interface
// It handles:
// - Checking if task is completed (fetch from DB)
// - Connecting to worker
// - Streaming live logs
// - Automatic reconnection on failure (optional)
func (s *MasterServer) StreamTaskLogsUnified(ctx context.Context, taskID, userID string, handler LogStreamHandler) error {
	s.mu.RLock()

	// First, check if task is completed and logs are in database
	if s.resultDB != nil {
		result, err := s.resultDB.GetResult(ctx, taskID)
		if err == nil && result != nil {
			// Task is completed, send stored logs
			s.mu.RUnlock()

			// Split logs by newlines and send them
			lines := splitLogLines(result.Logs)
			for i, line := range lines {
				isLastLine := i == len(lines)-1
				if err := handler(line, isLastLine, result.Status); err != nil {
					return err
				}

				// Small delay to simulate streaming
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(10 * time.Millisecond):
				}
			}
			return nil
		}
	}

	// Task might be running, try to stream from worker
	// Get task from database to find the worker
	var workerID string
	if s.assignmentDB != nil {
		assignment, err := s.assignmentDB.GetAssignmentByTaskID(ctx, taskID)
		if err != nil {
			s.mu.RUnlock()
			return fmt.Errorf("failed to find assignment for task: %w", err)
		}
		workerID = assignment.WorkerID
	} else {
		s.mu.RUnlock()
		return fmt.Errorf("database not available")
	}

	// Get worker info
	worker, exists := s.workers[workerID]
	if !exists {
		s.mu.RUnlock()
		return fmt.Errorf("worker %s not found", workerID)
	}

	workerIP := worker.Info.WorkerIp
	s.mu.RUnlock()

	// Connect to worker
	conn, err := grpc.Dial(workerIP, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to worker: %w", err)
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)

	// Request log stream with follow=true for live streaming
	stream, err := client.StreamTaskLogs(ctx, &pb.TaskLogRequest{
		TaskId: taskID,
		UserId: userID,
		Follow: true,
	})
	if err != nil {
		return fmt.Errorf("failed to start log stream: %w", err)
	}

	log.Printf("[StreamLogs] Started streaming logs for task %s from worker %s", taskID, workerID)

	// Stream logs
	for {
		select {
		case <-ctx.Done():
			log.Printf("[StreamLogs] Context cancelled for task %s", taskID)
			return ctx.Err()
		default:
		}

		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				log.Printf("[StreamLogs] Stream ended (EOF) for task %s", taskID)
				return nil
			}
			return fmt.Errorf("error receiving log chunk: %w", err)
		}

		// Call handler with log content
		if err := handler(chunk.Content, chunk.IsComplete, chunk.Status); err != nil {
			return fmt.Errorf("handler error: %w", err)
		}

		if chunk.IsComplete {
			log.Printf("[StreamLogs] Task %s completed with status: %s", taskID, chunk.Status)

			// Update task status in database if completed
			if s.taskDB != nil && chunk.Status != "running" {
				if err := s.taskDB.UpdateTaskStatus(ctx, taskID, chunk.Status); err != nil {
					log.Printf("[StreamLogs] Warning: failed to update task status: %v", err)
				}
			}
			return nil
		}
	}
}

// splitLogLines splits log content by newlines intelligently
func splitLogLines(logs string) []string {
	if logs == "" {
		return []string{}
	}

	lines := []string{}
	currentLine := ""

	for _, char := range logs {
		if char == '\n' {
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = ""
			}
		} else {
			currentLine += string(char)
		}
	}

	// Add remaining line if any
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}
