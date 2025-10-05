package scheduler

import (
	"context"
	"log"
	"time"

	pb "github.com/Codesmith28/CloudAI/pkg/api"
	"github.com/Codesmith28/CloudAI/pkg/taskqueue"
	"github.com/Codesmith28/CloudAI/pkg/workerregistry"
)

// Scheduler is the main scheduler that coordinates task scheduling
// It can delegate to different scheduling algorithms (greedy, agentic, etc.)
type Scheduler struct {
	taskQueue *taskqueue.TaskQueue
	registry  *workerregistry.Registry
	interval  time.Duration
	batchSize int
}

// NewScheduler creates a new scheduler instance
func NewScheduler(tq *taskqueue.TaskQueue, reg *workerregistry.Registry) *Scheduler {
	return &Scheduler{
		taskQueue: tq,
		registry:  reg,
		interval:  5 * time.Second, // Run every 5 seconds
		batchSize: 10,              // Schedule up to 10 tasks per cycle
	}
}

// Start begins the main scheduling loop
func (s *Scheduler) Start(ctx context.Context) error {
	log.Println("Starting scheduler...")

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Scheduler stopped")
			return ctx.Err()

		case <-ticker.C:
			if err := s.ScheduleOnce(ctx); err != nil {
				log.Printf("Scheduling error: %v", err)
			}
		}
	}
}

// ScheduleOnce performs a single scheduling cycle
// Currently uses greedy algorithm, will add agentic scheduling in future
func (s *Scheduler) ScheduleOnce(ctx context.Context) error {
	// Get pending tasks
	pendingTasks := s.taskQueue.PeekPending()
	if len(pendingTasks) == 0 {
		return nil
	}

	// Get current worker snapshot
	workers := s.registry.GetSnapshot()
	if len(workers) == 0 {
		log.Println("No active workers available")
		return nil
	}

	log.Printf("Scheduling batch: %d pending tasks, %d workers", len(pendingTasks), len(workers))

	// Schedule up to batchSize tasks using greedy algorithm
	// TODO: In future, check if agentic scheduler is available and use it
	scheduled := 0
	for i := 0; i < len(pendingTasks) && i < s.batchSize; i++ {
		task := pendingTasks[i]

		// Use greedy algorithm to find best worker
		workerID, err := greedyFindBestWorker(task, workers)
		if err != nil {
			log.Printf("No suitable worker for task %s: %v", task.Id, err)
			continue
		}

		// Reserve resources on the selected worker
		err = s.registry.Reserve(task.Id, workerID, task.CpuReq, task.MemMb, task.GpuReq, 5*time.Minute)
		if err != nil {
			log.Printf("Failed to reserve resources for task %s: %v", task.Id, err)
			continue
		}

		// Update task status to scheduled
		s.taskQueue.UpdateStatus(task.Id, "SCHEDULED")

		// Remove from queue
		s.taskQueue.DequeueBatch(1)

		scheduled++
		log.Printf("✓ Scheduled task %s to worker %s (CPU=%.2f, Mem=%dMB, GPU=%d)",
			task.Id, workerID, task.CpuReq, task.MemMb, task.GpuReq)
	}

	if scheduled > 0 {
		log.Printf("Scheduled %d/%d tasks in this cycle", scheduled, len(pendingTasks))
	}

	return nil
}

// SetInterval configures the scheduling frequency
func (s *Scheduler) SetInterval(interval time.Duration) {
	s.interval = interval
}

// SetBatchSize configures max tasks per cycle
func (s *Scheduler) SetBatchSize(size int) {
	s.batchSize = size
}

// canFit checks if a task can fit on a worker based on resources
func canFit(task *pb.Task, worker *pb.Worker) bool {
	return worker.FreeCpu >= task.CpuReq &&
		worker.FreeMem >= task.MemMb &&
		worker.FreeGpus >= task.GpuReq
}
