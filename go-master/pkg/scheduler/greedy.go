package scheduler

import (
	"fmt"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
)

// greedyFindBestWorker implements greedy best-fit algorithm
// Strategy: Select worker with sufficient resources and maximum free CPU
// This reduces resource fragmentation by keeping larger workers available for larger tasks
func greedyFindBestWorker(task *pb.Task, workers []*pb.Worker) (string, error) {
	var bestWorker *pb.Worker
	maxFreeCPU := float64(-1)

	for _, worker := range workers {
		// Check if worker can fit the task
		if !canFit(task, worker) {
			continue
		}

		// Select worker with most free CPU (reduces fragmentation)
		if worker.FreeCpu > maxFreeCPU {
			maxFreeCPU = worker.FreeCpu
			bestWorker = worker
		}
	}

	if bestWorker == nil {
		return "", fmt.Errorf("no worker has sufficient resources (need: CPU=%.2f, Mem=%dMB, GPU=%d)",
			task.CpuReq, task.MemMb, task.GpuReq)
	}

	return bestWorker.Id, nil
}
