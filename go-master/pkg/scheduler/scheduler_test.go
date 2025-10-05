package scheduler

import (
	"testing"
	"time"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
	"github.com/Codesmith28/CloudAI/pkg/taskqueue"
	"github.com/Codesmith28/CloudAI/pkg/workerregistry"
)

func TestNewScheduler(t *testing.T) {
	tq := taskqueue.NewTaskQueue()
	reg := workerregistry.NewRegistry()

	scheduler := NewScheduler(tq, reg)

	if scheduler == nil {
		t.Fatal("Expected scheduler to be created")
	}

	if scheduler.taskQueue != tq {
		t.Error("Task queue not properly initialized")
	}

	if scheduler.registry != reg {
		t.Error("Registry not properly initialized")
	}

	if scheduler.interval == 0 {
		t.Error("Interval should be set to default value")
	}

	if scheduler.batchSize == 0 {
		t.Error("Batch size should be set to default value")
	}
}

func TestSchedulerConfiguration(t *testing.T) {
	tq := taskqueue.NewTaskQueue()
	reg := workerregistry.NewRegistry()
	scheduler := NewScheduler(tq, reg)

	// Test SetInterval
	newInterval := 10 * time.Second
	scheduler.SetInterval(newInterval)
	if scheduler.interval != newInterval {
		t.Errorf("Expected interval %v, got %v", newInterval, scheduler.interval)
	}

	// Test SetBatchSize
	newBatchSize := 20
	scheduler.SetBatchSize(newBatchSize)
	if scheduler.batchSize != newBatchSize {
		t.Errorf("Expected batch size %d, got %d", newBatchSize, scheduler.batchSize)
	}
}

func TestCanFit(t *testing.T) {
	tests := []struct {
		name     string
		task     *pb.Task
		worker   *pb.Worker
		expected bool
	}{
		{
			name: "Task fits perfectly",
			task: &pb.Task{
				Id:     "task-1",
				CpuReq: 2.0,
				MemMb:  4096,
				GpuReq: 0,
			},
			worker: &pb.Worker{
				Id:       "worker-1",
				FreeCpu:  2.0,
				FreeMem:  4096,
				FreeGpus: 0,
			},
			expected: true,
		},
		{
			name: "Task has extra space",
			task: &pb.Task{
				Id:     "task-2",
				CpuReq: 2.0,
				MemMb:  4096,
				GpuReq: 0,
			},
			worker: &pb.Worker{
				Id:       "worker-2",
				FreeCpu:  4.0,
				FreeMem:  8192,
				FreeGpus: 1,
			},
			expected: true,
		},
		{
			name: "Insufficient CPU",
			task: &pb.Task{
				Id:     "task-3",
				CpuReq: 8.0,
				MemMb:  4096,
				GpuReq: 0,
			},
			worker: &pb.Worker{
				Id:       "worker-3",
				FreeCpu:  4.0,
				FreeMem:  8192,
				FreeGpus: 0,
			},
			expected: false,
		},
		{
			name: "Insufficient memory",
			task: &pb.Task{
				Id:     "task-4",
				CpuReq: 2.0,
				MemMb:  16384,
				GpuReq: 0,
			},
			worker: &pb.Worker{
				Id:       "worker-4",
				FreeCpu:  4.0,
				FreeMem:  8192,
				FreeGpus: 0,
			},
			expected: false,
		},
		{
			name: "Insufficient GPU",
			task: &pb.Task{
				Id:     "task-5",
				CpuReq: 2.0,
				MemMb:  4096,
				GpuReq: 2,
			},
			worker: &pb.Worker{
				Id:       "worker-5",
				FreeCpu:  4.0,
				FreeMem:  8192,
				FreeGpus: 1,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canFit(tt.task, tt.worker)
			if result != tt.expected {
				t.Errorf("canFit() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Test greedy algorithm
func TestGreedyFindBestWorker(t *testing.T) {
	task := &pb.Task{
		Id:     "task-1",
		CpuReq: 2.0,
		MemMb:  4096,
		GpuReq: 0,
	}

	workers := []*pb.Worker{
		{
			Id:       "worker-1",
			FreeCpu:  2.0,
			FreeMem:  8192,
			FreeGpus: 0,
		},
		{
			Id:       "worker-2",
			FreeCpu:  6.0, // Most free CPU
			FreeMem:  12288,
			FreeGpus: 0,
		},
		{
			Id:       "worker-3",
			FreeCpu:  4.0,
			FreeMem:  8192,
			FreeGpus: 0,
		},
	}

	workerID, err := greedyFindBestWorker(task, workers)
	if err != nil {
		t.Fatalf("greedyFindBestWorker() error: %v", err)
	}

	// Should select worker-2 (has most free CPU)
	if workerID != "worker-2" {
		t.Errorf("Expected worker-2, got %s", workerID)
	}
}

func TestGreedyFindBestWorkerNoSuitableWorker(t *testing.T) {
	task := &pb.Task{
		Id:     "task-1",
		CpuReq: 16.0, // Too much CPU
		MemMb:  4096,
		GpuReq: 0,
	}

	workers := []*pb.Worker{
		{
			Id:       "worker-1",
			FreeCpu:  2.0,
			FreeMem:  8192,
			FreeGpus: 0,
		},
		{
			Id:       "worker-2",
			FreeCpu:  4.0,
			FreeMem:  12288,
			FreeGpus: 0,
		},
	}

	_, err := greedyFindBestWorker(task, workers)
	if err == nil {
		t.Error("Expected error when no suitable worker found")
	}
}

func TestGreedySelectsWorkerWithMostFreeCPU(t *testing.T) {
	task := &pb.Task{
		Id:     "task-1",
		CpuReq: 1.0,
		MemMb:  1024,
		GpuReq: 0,
	}

	workers := []*pb.Worker{
		{Id: "worker-1", FreeCpu: 2.0, FreeMem: 2048, FreeGpus: 0},
		{Id: "worker-2", FreeCpu: 10.0, FreeMem: 2048, FreeGpus: 0}, // Should be selected
		{Id: "worker-3", FreeCpu: 5.0, FreeMem: 2048, FreeGpus: 0},
	}

	workerID, err := greedyFindBestWorker(task, workers)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if workerID != "worker-2" {
		t.Errorf("Expected worker-2 (most free CPU), got %s", workerID)
	}
}
