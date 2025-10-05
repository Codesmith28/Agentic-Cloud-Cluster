package master

import (
	"context"
	"testing"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
	"github.com/Codesmith28/CloudAI/pkg/taskqueue"
	"github.com/Codesmith28/CloudAI/pkg/workerregistry"
)

func TestSubmitTask(t *testing.T) {
	// Setup
	taskQueue := taskqueue.NewTaskQueue()
	registry := workerregistry.NewRegistry()
	server := NewMasterServer(taskQueue, registry)

	// Test task submission
	req := &pb.SubmitTaskRequest{
		Task: &pb.Task{
			TaskType:     "test",
			CpuReq:       1.0,
			MemMb:        256,
			GpuReq:       0,
			Priority:     5,
			EstimatedSec: 10,
		},
	}

	resp, err := server.SubmitTask(context.Background(), req)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	if resp.TaskId == "" {
		t.Error("Expected non-empty task ID")
	}

	if resp.Message != "task submitted successfully" {
		t.Errorf("Unexpected message: %s", resp.Message)
	}

	// Verify task is in queue
	status, err := taskQueue.GetStatus(resp.TaskId)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}

	if status != "PENDING" {
		t.Errorf("Expected status PENDING, got %s", status)
	}
}

func TestRegisterWorker(t *testing.T) {
	// Setup
	taskQueue := taskqueue.NewTaskQueue()
	registry := workerregistry.NewRegistry()
	server := NewMasterServer(taskQueue, registry)

	// Test worker registration
	req := &pb.RegisterWorkerRequest{
		Worker: &pb.Worker{
			Id:       "worker-1",
			TotalCpu: 4.0,
			TotalMem: 8192,
			Gpus:     1,
		},
	}

	resp, err := server.RegisterWorker(context.Background(), req)
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success, got failure: %s", resp.Message)
	}

	// Verify worker is in registry
	workers := registry.GetSnapshot()
	if len(workers) != 1 {
		t.Errorf("Expected 1 worker, got %d", len(workers))
	}

	if workers[0].Id != "worker-1" {
		t.Errorf("Expected worker-1, got %s", workers[0].Id)
	}
}

func TestGetTaskStatus(t *testing.T) {
	// Setup
	taskQueue := taskqueue.NewTaskQueue()
	registry := workerregistry.NewRegistry()
	server := NewMasterServer(taskQueue, registry)

	// Submit a task first
	submitReq := &pb.SubmitTaskRequest{
		Task: &pb.Task{
			TaskType: "test",
			CpuReq:   1.0,
			MemMb:    256,
		},
	}

	submitResp, _ := server.SubmitTask(context.Background(), submitReq)

	// Test get status
	statusReq := &pb.GetTaskStatusRequest{
		TaskId: submitResp.TaskId,
	}

	statusResp, err := server.GetTaskStatus(context.Background(), statusReq)
	if err != nil {
		t.Fatalf("GetTaskStatus failed: %v", err)
	}

	if statusResp.Status != "PENDING" {
		t.Errorf("Expected PENDING, got %s", statusResp.Status)
	}
}

func TestListWorkers(t *testing.T) {
	// Setup
	taskQueue := taskqueue.NewTaskQueue()
	registry := workerregistry.NewRegistry()
	server := NewMasterServer(taskQueue, registry)

	// Register some workers
	for i := 0; i < 3; i++ {
		req := &pb.RegisterWorkerRequest{
			Worker: &pb.Worker{
				Id:       string(rune('A' + i)),
				TotalCpu: 4.0,
				TotalMem: 8192,
			},
		}
		server.RegisterWorker(context.Background(), req)
	}

	// Test list workers
	listReq := &pb.ListWorkersRequest{}
	listResp, err := server.ListWorkers(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListWorkers failed: %v", err)
	}

	if len(listResp.Workers) != 3 {
		t.Errorf("Expected 3 workers, got %d", len(listResp.Workers))
	}
}
