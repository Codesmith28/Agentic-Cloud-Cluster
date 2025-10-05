package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Connect to master server
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewSchedulerServiceClient(conn)
	ctx := context.Background()

	fmt.Println("=================================")
	fmt.Println("CloudAI Master API Test Client")
	fmt.Println("=================================\n")

	// Test 1: Submit a task
	fmt.Println("1. Submitting a task...")
	submitReq := &pb.SubmitTaskRequest{
		Task: &pb.Task{
			TaskType:     "test-compute",
			CpuReq:       2.0,
			MemMb:        1024,
			GpuReq:       0,
			Priority:     5,
			EstimatedSec: 60,
		},
	}

	submitResp, err := client.SubmitTask(ctx, submitReq)
	if err != nil {
		log.Fatalf("SubmitTask failed: %v", err)
	}
	fmt.Printf("   ✅ Task submitted: ID=%s\n", submitResp.TaskId)
	fmt.Printf("   Message: %s\n\n", submitResp.Message)

	taskID := submitResp.TaskId

	// Test 2: Get task status
	fmt.Println("2. Getting task status...")
	statusReq := &pb.GetTaskStatusRequest{
		TaskId: taskID,
	}

	statusResp, err := client.GetTaskStatus(ctx, statusReq)
	if err != nil {
		log.Fatalf("GetTaskStatus failed: %v", err)
	}
	fmt.Printf("   ✅ Task status: %s\n", statusResp.Status)
	fmt.Printf("   Message: %s\n\n", statusResp.Message)

	// Test 3: Register a worker
	fmt.Println("3. Registering a worker...")
	registerReq := &pb.RegisterWorkerRequest{
		Worker: &pb.Worker{
			Id:       "test-worker-1",
			TotalCpu: 8.0,
			TotalMem: 16384,
			Gpus:     1,
		},
	}

	registerResp, err := client.RegisterWorker(ctx, registerReq)
	if err != nil {
		log.Fatalf("RegisterWorker failed: %v", err)
	}
	fmt.Printf("   ✅ Worker registered: Success=%v\n", registerResp.Success)
	fmt.Printf("   Message: %s\n\n", registerResp.Message)

	// Test 4: Send heartbeat
	fmt.Println("4. Sending worker heartbeat...")
	heartbeatReq := &pb.HeartbeatRequest{
		Worker: &pb.Worker{
			Id:       "test-worker-1",
			FreeCpu:  6.0,
			FreeMem:  12288,
			FreeGpus: 1,
		},
		RunningTaskIds: []string{},
	}

	heartbeatResp, err := client.Heartbeat(ctx, heartbeatReq)
	if err != nil {
		log.Fatalf("Heartbeat failed: %v", err)
	}
	fmt.Printf("   ✅ Heartbeat acknowledged: Success=%v\n\n", heartbeatResp.Success)

	// Test 5: List workers
	fmt.Println("5. Listing all workers...")
	listReq := &pb.ListWorkersRequest{}

	listResp, err := client.ListWorkers(ctx, listReq)
	if err != nil {
		log.Fatalf("ListWorkers failed: %v", err)
	}
	fmt.Printf("   ✅ Found %d worker(s):\n", len(listResp.Workers))
	for _, w := range listResp.Workers {
		fmt.Printf("      - ID: %s, CPU: %.1f/%.1f, Mem: %d/%d MB, GPU: %d/%d\n",
			w.Id, w.FreeCpu, w.TotalCpu, w.FreeMem, w.TotalMem, w.FreeGpus, w.Gpus)
	}
	fmt.Println()

	// Test 6: Submit another task
	fmt.Println("6. Submitting another task...")
	submitReq2 := &pb.SubmitTaskRequest{
		Task: &pb.Task{
			TaskType:     "ml-training",
			CpuReq:       4.0,
			MemMb:        4096,
			GpuReq:       1,
			Priority:     10,
			EstimatedSec: 300,
		},
	}

	submitResp2, err := client.SubmitTask(ctx, submitReq2)
	if err != nil {
		log.Fatalf("SubmitTask failed: %v", err)
	}
	fmt.Printf("   ✅ Task submitted: ID=%s\n\n", submitResp2.TaskId)

	// Test 7: Report task completion
	fmt.Println("7. Reporting task completion...")
	completionReq := &pb.TaskCompletionRequest{
		TaskId:            taskID,
		WorkerId:          "test-worker-1",
		Success:           true,
		ActualDurationSec: 55,
		ResourceUsage: map[string]float64{
			"avg_cpu":     1.8,
			"peak_mem_mb": 950.0,
		},
	}

	completionResp, err := client.ReportTaskCompletion(ctx, completionReq)
	if err != nil {
		log.Fatalf("ReportTaskCompletion failed: %v", err)
	}
	fmt.Printf("   ✅ Completion acknowledged: %v\n\n", completionResp.Acknowledged)

	// Test 8: Verify task status changed
	fmt.Println("8. Verifying task status after completion...")
	time.Sleep(100 * time.Millisecond) // Small delay for status update
	statusResp2, err := client.GetTaskStatus(ctx, &pb.GetTaskStatusRequest{TaskId: taskID})
	if err != nil {
		log.Fatalf("GetTaskStatus failed: %v", err)
	}
	fmt.Printf("   ✅ Task status: %s\n\n", statusResp2.Status)

	// Test 9: Cancel a task
	fmt.Println("9. Canceling the second task...")
	cancelReq := &pb.CancelTaskRequest{
		TaskId: submitResp2.TaskId,
	}

	cancelResp, err := client.CancelTask(ctx, cancelReq)
	if err != nil {
		log.Fatalf("CancelTask failed: %v", err)
	}
	fmt.Printf("   ✅ Task cancelled: Success=%v\n", cancelResp.Success)
	fmt.Printf("   Message: %s\n\n", cancelResp.Message)

	fmt.Println("=================================")
	fmt.Println("✅ All tests completed successfully!")
	fmt.Println("=================================")
}
