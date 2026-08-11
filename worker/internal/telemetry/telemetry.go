package telemetry

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "worker/proto"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
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

// AddTask adds a task to the running tasks list
func (m *Monitor) AddTask(taskID string, cpuAlloc, memAlloc, gpuAlloc float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runningTasks[taskID] = &pb.RunningTask{
		TaskId:          taskID,
		CpuAllocated:    cpuAlloc,
		MemoryAllocated: memAlloc,
		GpuAllocated:    gpuAlloc,
		Status:          "running",
	}
	log.Printf("Task %s added to monitoring (total tasks: %d)", taskID, len(m.runningTasks))
}

// RemoveTask removes a task from the running tasks list
func (m *Monitor) RemoveTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runningTasks, taskID)
	log.Printf("Task %s removed from monitoring (total tasks: %d)", taskID, len(m.runningTasks))
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
	cpuUsage, memUsage, gpuUsage := m.getResourceUsage()

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
		GpuUsage:     gpuUsage,
		RunningTasks: tasks,
	}

	ack, err := client.SendHeartbeat(ctx, heartbeat)
	if err != nil {
		return err
	}

	if ack.Success {
		log.Printf("Heartbeat sent: CPU=%.1f%%, Memory=%.1f%%, GPU=%.1f%%, Tasks=%d",
			cpuUsage, memUsage, gpuUsage, len(tasks))
	}

	return nil
}

// getResourceUsage returns actual CPU, memory, and GPU usage of the machine
func (m *Monitor) getResourceUsage() (cpuPercent, memoryPercent, gpuPercent float64) {
	// CPU usage over a short sample interval
	cpuPercents, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	// Memory usage
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		memoryPercent = vmStat.UsedPercent
	}

	// GPU usage (NVIDIA only via nvidia-smi)
	gpuPercent = m.getGPUUsage()

	return cpuPercent, memoryPercent, gpuPercent
}

// getGPUUsage returns GPU utilization percentage using NVML
// Returns the average utilization across all GPUs if multiple GPUs present
func (m *Monitor) getGPUUsage() float64 {
	// Initialize NVML
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		// NVML not available - no GPU or drivers not loaded
		return 0.0
	}
	defer nvml.Shutdown()

	// Get device count
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS || count == 0 {
		return 0.0
	}

	// Get average utilization across all GPUs
	var totalUtil float64
	validDevices := 0

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		// Get utilization rates
		util, ret := device.GetUtilizationRates()
		if ret == nvml.SUCCESS {
			totalUtil += float64(util.Gpu)
			validDevices++
		}
	}

	if validDevices == 0 {
		return 0.0
	}

	// Return average utilization
	return totalUtil / float64(validDevices)
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
