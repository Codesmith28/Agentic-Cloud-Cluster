package telemetry

import (
	"context"
	"log"
	"runtime"
	"time"

	pb "worker/proto/pb"

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
}

// NewMonitor creates a new telemetry monitor
func NewMonitor(workerID, masterAddr string, interval time.Duration) *Monitor {
	return &Monitor{
		workerID:     workerID,
		masterAddr:   masterAddr,
		interval:     interval,
		runningTasks: make(map[string]*pb.RunningTask),
		stopChan:     make(chan struct{}),
	}
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
			log.Println("Stopping telemetry monitor")
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

// AddTask adds a task to the running tasks list
func (m *Monitor) AddTask(taskID string, cpuAlloc, memAlloc float64) {
	m.runningTasks[taskID] = &pb.RunningTask{
		TaskId:          taskID,
		CpuAllocated:    cpuAlloc,
		MemoryAllocated: memAlloc,
		Status:          "running",
	}
}

// RemoveTask removes a task from the running tasks list
func (m *Monitor) RemoveTask(taskID string) {
	delete(m.runningTasks, taskID)
}

// sendHeartbeat sends a heartbeat message to the master
func (m *Monitor) sendHeartbeat(ctx context.Context) error {
	conn, err := grpc.DialContext(
		ctx,
		m.masterAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewMasterWorkerClient(conn)

	// Get current resource usage
	cpuUsage, memUsage := m.getResourceUsage()

	// Convert running tasks map to slice
	tasks := make([]*pb.RunningTask, 0, len(m.runningTasks))
	for _, task := range m.runningTasks {
		tasks = append(tasks, task)
	}

	heartbeat := &pb.Heartbeat{
		WorkerId:     m.workerID,
		CpuUsage:     cpuUsage,
		MemoryUsage:  memUsage,
		StorageUsage: 0.0, // TODO: Implement storage monitoring
		RunningTasks: tasks,
	}

	ack, err := client.SendHeartbeat(ctx, heartbeat)
	if err != nil {
		return err
	}

	if ack.Success {
		log.Printf("Heartbeat sent: CPU=%.1f%%, Memory=%.1fMB, Tasks=%d",
			cpuUsage, memUsage, len(tasks))
	}

	return nil
}

// getResourceUsage returns current CPU and memory usage
func (m *Monitor) getResourceUsage() (cpu, memory float64) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// CPU usage (simplified - returns number of goroutines as proxy)
	cpu = float64(runtime.NumGoroutine()) * 10.0 // Simplified metric
	if cpu > 100 {
		cpu = 100
	}

	// Memory usage in MB
	memory = float64(memStats.Alloc) / 1024 / 1024

	return cpu, memory
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
