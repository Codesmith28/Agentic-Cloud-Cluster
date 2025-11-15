package logstream

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// LogStreamManager manages log broadcasters for all running tasks
type LogStreamManager struct {
	broadcasters map[string]*TaskLogBroadcaster // taskID -> broadcaster
	dockerClient *client.Client
	mu           sync.RWMutex
}

// NewLogStreamManager creates a new log stream manager
func NewLogStreamManager(dockerClient *client.Client) *LogStreamManager {
	return &LogStreamManager{
		broadcasters: make(map[string]*TaskLogBroadcaster),
		dockerClient: dockerClient,
	}
}

// StartTask starts log broadcasting for a new task
func (m *LogStreamManager) StartTask(taskID, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists
	if _, exists := m.broadcasters[taskID]; exists {
		return fmt.Errorf("log broadcaster already exists for task %s", taskID)
	}

	// Create new broadcaster
	broadcaster := NewTaskLogBroadcaster(taskID, containerID, m.dockerClient)
	m.broadcasters[taskID] = broadcaster

	return nil
}

// Subscribe subscribes to logs for a task
// Returns a channel that receives log lines
func (m *LogStreamManager) Subscribe(ctx context.Context, taskID string, sendRecent bool) (<-chan LogLine, error) {
	m.mu.RLock()
	broadcaster, exists := m.broadcasters[taskID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no log broadcaster found for task %s", taskID)
	}

	// Generate unique subscriber ID
	subscriberID := uuid.New().String()

	return broadcaster.Subscribe(subscriberID, ctx, sendRecent)
}

// Unsubscribe removes a subscription (optional, context cancellation also works)
func (m *LogStreamManager) Unsubscribe(taskID, subscriberID string) {
	m.mu.RLock()
	broadcaster, exists := m.broadcasters[taskID]
	m.mu.RUnlock()

	if exists {
		broadcaster.Unsubscribe(subscriberID)
	}
}

// StopTask stops log broadcasting for a task and cleans up
func (m *LogStreamManager) StopTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if broadcaster, exists := m.broadcasters[taskID]; exists {
		broadcaster.Stop()
		delete(m.broadcasters, taskID)
	}
}

// GetTaskInfo returns information about a task's log broadcaster
func (m *LogStreamManager) GetTaskInfo(taskID string) (exists bool, subscriberCount int, isRunning bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	broadcaster, exists := m.broadcasters[taskID]
	if !exists {
		return false, 0, false
	}

	return true, broadcaster.GetSubscriberCount(), broadcaster.IsRunning()
}

// StopAll stops all broadcasters and cleans up
func (m *LogStreamManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, broadcaster := range m.broadcasters {
		broadcaster.Stop()
	}
	m.broadcasters = make(map[string]*TaskLogBroadcaster)
}

// GetActiveTaskIDs returns a list of all task IDs that have active log broadcasters
func (m *LogStreamManager) GetActiveTaskIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	taskIDs := make([]string, 0, len(m.broadcasters))
	for taskID := range m.broadcasters {
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}
