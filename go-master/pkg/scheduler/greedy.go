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
