package logstream

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// LogLine represents a single log entry
type LogLine struct {
	Content   string
	Timestamp time.Time
}

// Subscriber represents a client listening to logs
type Subscriber struct {
	ID      string
	Channel chan LogLine
	ctx     context.Context
}

// TaskLogBroadcaster manages log streaming for a single task
// It reads logs once from Docker and broadcasts to multiple subscribers
type TaskLogBroadcaster struct {
	taskID        string
	containerID   string
	dockerClient  *client.Client
	subscribers   map[string]*Subscriber
	recentLogs    []LogLine // Ring buffer of recent logs
	maxRecentLogs int
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	isRunning     bool
	startOnce     sync.Once
}

// NewTaskLogBroadcaster creates a new log broadcaster for a task
func NewTaskLogBroadcaster(taskID, containerID string, dockerClient *client.Client) *TaskLogBroadcaster {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskLogBroadcaster{
		taskID:        taskID,
		containerID:   containerID,
		dockerClient:  dockerClient,
		subscribers:   make(map[string]*Subscriber),
		recentLogs:    make([]LogLine, 0, 1000), // Keep last 1000 log lines
		maxRecentLogs: 1000,
		ctx:           ctx,
		cancel:        cancel,
		isRunning:     false,
	}
}

// Subscribe adds a new subscriber to receive logs
// If sendRecent is true, sends buffered recent logs first
func (b *TaskLogBroadcaster) Subscribe(subscriberID string, subscriberCtx context.Context, sendRecent bool) (<-chan LogLine, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Create subscriber channel with buffer to prevent blocking
	subChan := make(chan LogLine, 100)

	subscriber := &Subscriber{
		ID:      subscriberID,
		Channel: subChan,
		ctx:     subscriberCtx,
	}

	b.subscribers[subscriberID] = subscriber

	// Start log streaming if not already started
	b.startOnce.Do(func() {
		go b.streamLogs()
	})

	// Send recent logs to new subscriber if requested
	if sendRecent && len(b.recentLogs) > 0 {
		go func() {
			for _, logLine := range b.recentLogs {
				select {
				case subChan <- logLine:
				case <-subscriberCtx.Done():
					return
				case <-time.After(time.Second):
					// Skip if subscriber is slow
					return
				}
			}
		}()
	}

	return subChan, nil
}

// Unsubscribe removes a subscriber
func (b *TaskLogBroadcaster) Unsubscribe(subscriberID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sub, exists := b.subscribers[subscriberID]; exists {
		close(sub.Channel)
		delete(b.subscribers, subscriberID)
	}

	// If no more subscribers, we can optionally stop streaming
	// But keep it running to maintain the buffer for future subscribers
}

// streamLogs is the main goroutine that reads from Docker and broadcasts
func (b *TaskLogBroadcaster) streamLogs() {
	b.mu.Lock()
	b.isRunning = true
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.isRunning = false
		// Close all subscriber channels
		for _, sub := range b.subscribers {
			close(sub.Channel)
		}
		b.subscribers = make(map[string]*Subscriber)
		b.mu.Unlock()
	}()

	// Inspect container to determine if it has TTY
	inspect, err := b.dockerClient.ContainerInspect(b.ctx, b.containerID)
	if err != nil {
		b.broadcastError(fmt.Errorf("failed to inspect container: %w", err))
		return
	}

	// Get container logs with follow enabled
	logReader, err := b.dockerClient.ContainerLogs(b.ctx, b.containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true, // Stream logs in real-time
		Timestamps: false,
		Since:      "", // Start from beginning
	})
	if err != nil {
		b.broadcastError(fmt.Errorf("failed to get container logs: %w", err))
		return
	}
	defer logReader.Close()

	// Handle TTY vs non-TTY containers
	if inspect.Config != nil && inspect.Config.Tty {
		b.streamRawLogs(logReader)
	} else {
		b.streamMultiplexedLogs(logReader)
	}
}

// streamRawLogs handles logs from TTY containers (raw stream)
func (b *TaskLogBroadcaster) streamRawLogs(logReader io.ReadCloser) {
	reader := bufio.NewReader(logReader)
	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			logLine := LogLine{
				Content:   strings.TrimRight(line, "\n"),
				Timestamp: time.Now(),
			}
			b.broadcast(logLine)
		}

		if err != nil {
			if err == io.EOF {
				return
			}
			b.broadcastError(fmt.Errorf("error reading logs: %w", err))
			return
		}
	}
}

// streamMultiplexedLogs handles logs from non-TTY containers (multiplexed stdout/stderr)
func (b *TaskLogBroadcaster) streamMultiplexedLogs(logReader io.ReadCloser) {
	// Create pipes for demultiplexing
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	// Demultiplex in background
	go func() {
		_, _ = stdcopy.StdCopy(stdoutWriter, stderrWriter, logReader)
		stdoutWriter.Close()
		stderrWriter.Close()
	}()

	// Read from both stdout and stderr
	var wg sync.WaitGroup
	wg.Add(2)

	// Read stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			logLine := LogLine{
				Content:   scanner.Text(),
				Timestamp: time.Now(),
			}
			b.broadcast(logLine)
		}
	}()

	// Read stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrReader)
		for scanner.Scan() {
			logLine := LogLine{
				Content:   scanner.Text(),
				Timestamp: time.Now(),
			}
			b.broadcast(logLine)
		}
	}()

	wg.Wait()
}

// broadcast sends a log line to all subscribers and stores in buffer
func (b *TaskLogBroadcaster) broadcast(logLine LogLine) {
	b.mu.Lock()

	// Add to recent logs buffer (ring buffer behavior)
	if len(b.recentLogs) >= b.maxRecentLogs {
		// Remove oldest log
		b.recentLogs = b.recentLogs[1:]
	}
	b.recentLogs = append(b.recentLogs, logLine)

	// Get current subscribers
	subscribers := make([]*Subscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subscribers = append(subscribers, sub)
	}
	b.mu.Unlock()

	// Send to all subscribers (non-blocking)
	for _, sub := range subscribers {
		select {
		case sub.Channel <- logLine:
			// Sent successfully
		case <-sub.ctx.Done():
			// Subscriber context cancelled, will be cleaned up
		default:
			// Subscriber is slow/blocked, skip this log to prevent blocking
			// Consider unsubscribing slow clients
		}
	}
}

// broadcastError sends an error to all subscribers
func (b *TaskLogBroadcaster) broadcastError(err error) {
	errorLine := LogLine{
		Content:   fmt.Sprintf("ERROR: %v", err),
		Timestamp: time.Now(),
	}
	b.broadcast(errorLine)
}

// Stop stops the broadcaster and cleans up
func (b *TaskLogBroadcaster) Stop() {
	b.cancel()

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subscribers {
		close(sub.Channel)
	}
	b.subscribers = make(map[string]*Subscriber)
}

// GetSubscriberCount returns the number of active subscribers
func (b *TaskLogBroadcaster) GetSubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// IsRunning returns whether the broadcaster is actively streaming
func (b *TaskLogBroadcaster) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.isRunning
}
