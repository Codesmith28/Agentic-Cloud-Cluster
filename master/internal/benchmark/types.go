package benchmark

import (
	"context"
	"time"

	"master/internal/scheduler"
	pb "master/proto"
)

const (
	ProfileAll      = "all"
	ProfileShowcase = "showcase"
	ProfileSteady   = "steady"
	ProfileBursty   = "bursty"
)

var canonicalTaskTypes = []string{
	scheduler.TaskTypeCPULight,
	scheduler.TaskTypeCPUHeavy,
	scheduler.TaskTypeMemoryHeavy,
	scheduler.TaskTypeMixed,
}

const simulationEpochUnix = int64(1735689600) // 2025-01-01T00:00:00Z

// WorkloadTask defines one task event in a benchmark workload timeline.
type WorkloadTask struct {
	TaskID        string        `json:"task_id"`
	TaskName      string        `json:"task_name"`
	ArrivalOffset time.Duration `json:"arrival_offset"`
	DockerImage   string        `json:"docker_image"`
	Command       string        `json:"command"`
	ReqCPU        float64       `json:"req_cpu"`
	ReqMemory     float64       `json:"req_memory"`
	ReqStorage    float64       `json:"req_storage"`
	TaskType      string        `json:"task_type"`
	SLAMultiplier float64       `json:"sla_multiplier"`
	TauSeconds    float64       `json:"tau_seconds"`
}

// WorkerProfile defines capacity and speed behavior for a simulated worker.
type WorkerProfile struct {
	WorkerID        string             `json:"worker_id"`
	TotalCPU        float64            `json:"total_cpu"`
	TotalMemory     float64            `json:"total_memory"`
	TotalStorage    float64            `json:"total_storage"`
	SpeedByTask     map[string]float64 `json:"speed_by_task"`
	Penalty         float64            `json:"penalty"`
	InitialIsActive bool               `json:"initial_is_active"`
}

// WorkloadProfile defines a benchmark scenario.
type WorkloadProfile struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Workers     []WorkerProfile `json:"workers"`
	Tasks       []WorkloadTask  `json:"tasks"`
}

// TaskRun captures one task execution result under one scheduler.
type TaskRun struct {
	TaskID      string  `json:"task_id"`
	TaskType    string  `json:"task_type"`
	WorkerID    string  `json:"worker_id"`
	ArrivalSec  float64 `json:"arrival_sec"`
	StartSec    float64 `json:"start_sec"`
	FinishSec   float64 `json:"finish_sec"`
	WaitSec     float64 `json:"wait_sec"`
	RuntimeSec  float64 `json:"runtime_sec"`
	DeadlineSec float64 `json:"deadline_sec"`
	SLASuccess  bool    `json:"sla_success"`
	DecisionMS  float64 `json:"decision_ms"`
}

// SchedulerMetrics contains aggregate benchmark stats.
type SchedulerMetrics struct {
	TotalTasks            int            `json:"total_tasks"`
	CompletedTasks        int            `json:"completed_tasks"`
	SLASuccessRatePct     float64        `json:"sla_success_rate_pct"`
	AvgQueueWaitSec       float64        `json:"avg_queue_wait_sec"`
	P95QueueWaitSec       float64        `json:"p95_queue_wait_sec"`
	AvgRuntimeSec         float64        `json:"avg_runtime_sec"`
	MakespanSec           float64        `json:"makespan_sec"`
	ThroughputTasksPerMin float64        `json:"throughput_tasks_per_min"`
	CPUUtilizationPct     float64        `json:"cpu_utilization_pct"`
	MemoryUtilizationPct  float64        `json:"memory_utilization_pct"`
	WorkerBalanceScore    float64        `json:"worker_balance_score"`
	AvgDecisionMS         float64        `json:"avg_decision_ms"`
	P95DecisionMS         float64        `json:"p95_decision_ms"`
	AssignmentsByWorker   map[string]int `json:"assignments_by_worker"`
	UnschedulableTasks    int            `json:"unschedulable_tasks"`
}

// SchedulerResult stores benchmark results for one scheduler.
type SchedulerResult struct {
	SchedulerName string           `json:"scheduler_name"`
	Metrics       SchedulerMetrics `json:"metrics"`
	TaskRuns      []TaskRun        `json:"task_runs"`
}

// ProfileResult stores side-by-side scheduler results for one workload profile.
type ProfileResult struct {
	Profile              string            `json:"profile"`
	Description          string            `json:"description"`
	TaskCount            int               `json:"task_count"`
	SchedulerResults     []SchedulerResult `json:"scheduler_results"`
	Winner               string            `json:"winner"`
	SLAImprovementPct    float64           `json:"sla_improvement_pct"`
	WaitP95ReductionPct  float64           `json:"wait_p95_reduction_pct"`
	MakespanReductionPct float64           `json:"makespan_reduction_pct"`
	ThroughputGainPct    float64           `json:"throughput_gain_pct"`
}

// SuiteResult aggregates all profile comparisons.
type SuiteResult struct {
	GeneratedAt      time.Time       `json:"generated_at"`
	Seed             int64           `json:"seed"`
	RequestedProfile string          `json:"requested_profile"`
	Profiles         []ProfileResult `json:"profiles"`
}

// Internal simulation types

type simTask struct {
	Task         WorkloadTask
	Arrival      time.Duration
	Deadline     time.Duration
	JitterFactor float64
	ProtoTask    *pb.Task
}

type runningTask struct {
	Task     *simTask
	WorkerID string
	StartAt  time.Duration
	FinishAt time.Duration
}

type simWorker struct {
	Profile          WorkerProfile
	AllocatedCPU     float64
	AllocatedMemory  float64
	AllocatedStorage float64
	Running          []*runningTask
	AssignedCount    int
}

type simState struct {
	workers         map[string]*simWorker
	busyCPUSeconds  float64
	busyMemSeconds  float64
	totalCPUSeconds float64
	totalMemSeconds float64
}

type simTelemetrySource struct {
	sim *simState
}

func (s *simTelemetrySource) GetWorkerViews(ctx context.Context) ([]scheduler.WorkerView, error) {
	views := make([]scheduler.WorkerView, 0, len(s.sim.workers))
	for _, worker := range s.sim.workers {
		if !worker.Profile.InitialIsActive {
			continue
		}
		views = append(views, scheduler.WorkerView{
			ID:           worker.Profile.WorkerID,
			CPUAvail:     worker.Profile.TotalCPU - worker.AllocatedCPU,
			MemAvail:     worker.Profile.TotalMemory - worker.AllocatedMemory,
			StorageAvail: worker.Profile.TotalStorage - worker.AllocatedStorage,
			Load:         workerLoad(worker),
		})
	}
	return views, nil
}

func (s *simTelemetrySource) GetWorkerLoad(workerID string) float64 {
	worker, exists := s.sim.workers[workerID]
	if !exists {
		return 0
	}
	return workerLoad(worker)
}
