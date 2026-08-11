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

package telemetry

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	workermetrics "worker/internal/metrics"
	"worker/internal/system"
	pb "worker/proto"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Monitor handles telemetry and heartbeat reporting
type Monitor struct {
	workerID     string
	masterAddr   string
	interval     time.Duration
	runningTasks map[string]*pb.RunningTask
	stopChan     chan struct{}
	mu           sync.RWMutex // Protects runningTasks and masterAddr
}

// NewMonitor creates a new telemetry monitor
func NewMonitor(workerID string, interval time.Duration) *Monitor {
	return &Monitor{
		workerID:     workerID,
		masterAddr:   "", // Will be set when master registers
		interval:     interval,
		runningTasks: make(map[string]*pb.RunningTask),
		stopChan:     make(chan struct{}),
	}
}

// SetMasterAddress updates the master address (used when master registers)
func (m *Monitor) SetMasterAddress(masterAddr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.masterAddr = masterAddr
	log.Printf("Updated master address to: %s", masterAddr)
}

// Start begins sending periodic heartbeats to the master
func (m *Monitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	log.Printf("Starting telemetry monitor (interval: %v)", m.interval)

	for {
		select {
		case <-ticker.C:
			if err := m.sendHeartbeat(ctx); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
			}
		case <-m.stopChan:
			fmt.Println("  ✓ Telemetry monitor stopped")
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop stops the telemetry monitor
func (m *Monitor) Stop() {
	close(m.stopChan)
}

// AddTask adds a task attempt to the running tasks list.
func (m *Monitor) AddTask(taskID, attemptID string, attemptNo int32, cpuAlloc, memAlloc float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runningTasks[taskID] = &pb.RunningTask{
		TaskId:          taskID,
		CpuAllocated:    cpuAlloc,
		MemoryAllocated: memAlloc,
		Status:          "running",
		AttemptId:       attemptID,
		AttemptNo:       attemptNo,
	}
	workermetrics.Get().SetRunningTasks(len(m.runningTasks))
	log.Printf("Task %s added to monitoring (total tasks: %d)", taskID, len(m.runningTasks))
}

// RemoveTask removes a task from the running tasks list
func (m *Monitor) RemoveTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runningTasks, taskID)
	workermetrics.Get().SetRunningTasks(len(m.runningTasks))
	log.Printf("Task %s removed from monitoring (total tasks: %d)", taskID, len(m.runningTasks))
}

// GetRunningTasks returns a snapshot of currently tracked running task attempts.
func (m *Monitor) GetRunningTasks() []*pb.RunningTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*pb.RunningTask, 0, len(m.runningTasks))
	for _, task := range m.runningTasks {
		if task == nil {
			continue
		}
		taskCopy := *task
		tasks = append(tasks, &taskCopy)
	}
	return tasks
}

// sendHeartbeat sends a heartbeat message to the master
func (m *Monitor) sendHeartbeat(ctx context.Context) error {
	// Skip heartbeat if master address is not set yet
	m.mu.RLock()
	masterAddr := m.masterAddr
	m.mu.RUnlock()

	if masterAddr == "" {
		return nil // Silently skip, master hasn't registered yet
	}

	conn, err := grpc.DialContext(
		ctx,
		masterAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)

	// Get current resource usage
	cpuUsage, memUsage, storageUsage := m.getResourceUsage()

	// Convert running tasks map to slice (with lock)
	m.mu.RLock()
	tasks := make([]*pb.RunningTask, 0, len(m.runningTasks))
	for _, task := range m.runningTasks {
		tasks = append(tasks, task)
	}
	m.mu.RUnlock()

	heartbeat := &pb.Heartbeat{
		WorkerId:     m.workerID,
		CpuUsage:     cpuUsage,
		MemoryUsage:  memUsage,
		StorageUsage: storageUsage,
		RunningTasks: tasks,
	}

	ack, err := client.SendHeartbeat(ctx, heartbeat)
	if err != nil {
		return err
	}

	if ack.Success {
		workermetrics.Get().RecordHeartbeat(cpuUsage, memUsage, storageUsage)
		log.Printf("Heartbeat sent: CPU=%.1f%%, Memory=%.1f%%, Storage=%.1f%%, Tasks=%d",
			cpuUsage*100.0, memUsage*100.0, storageUsage*100.0, len(tasks))
	}

	return nil
}

// getResourceUsage returns normalized usage fractions in [0.0, 1.0].
func (m *Monitor) getResourceUsage() (cpuUsage, memoryUsage, storageUsage float64) {
	// CPU usage over a short sample interval
	cpuPercents, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercents) > 0 {
		cpuUsage = clampUnitInterval(cpuPercents[0] / 100.0)
	}

	// Memory usage
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		memoryUsage = clampUnitInterval(vmStat.UsedPercent / 100.0)
	}

	availableStorage, err := system.GetAvailableStorage()
	if err == nil {
		totalStorage, totalErr := system.GetSystemResources()
		if totalErr == nil && totalStorage.TotalStorage > 0 {
			storageUsage = clampUnitInterval(1.0 - (availableStorage / totalStorage.TotalStorage))
		}
	}

	return cpuUsage, memoryUsage, storageUsage
}

func clampUnitInterval(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

// RegisterWorker registers the worker with the master
func RegisterWorker(ctx context.Context, masterAddr string, info *pb.WorkerInfo) error {
	conn, err := grpc.DialContext(
		ctx,
		masterAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	ack, err := client.RegisterWorker(ctx, info)
	if err != nil {
		return err
	}

	if !ack.Success {
		return err
	}

	log.Printf("✓ Worker registered: %s", ack.Message)
	return nil
}

// ReportTaskResult sends task completion result to master
func ReportTaskResult(ctx context.Context, masterAddr string, result *pb.TaskResult) error {
	conn, err := grpc.DialContext(
		ctx,
		masterAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)
	ack, err := client.ReportTaskCompletion(ctx, result)
	if err != nil {
		return err
	}

	if !ack.Success {
		return err
	}

	log.Printf("✓ Task result reported: %s", ack.Message)
	return nil
}
