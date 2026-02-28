package benchmark

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"master/internal/scheduler"
	"master/internal/telemetry"
	pb "master/proto"
)

const (
	ProfileAll      = "all"
	ProfileShowcase = "showcase"
	ProfileSteady   = "steady"
	ProfileGPUSpike = "gpu-spike"
)

var canonicalTaskTypes = []string{
	scheduler.TaskTypeCPULight,
	scheduler.TaskTypeCPUHeavy,
	scheduler.TaskTypeMemoryHeavy,
	scheduler.TaskTypeGPUInference,
	scheduler.TaskTypeGPUTraining,
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
	ReqGPU        float64       `json:"req_gpu"`
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
	TotalGPU        float64            `json:"total_gpu"`
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
	GPUUtilizationPct     float64        `json:"gpu_utilization_pct"`
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

// RunSuite executes scheduler benchmarks for one profile or all profiles.
func RunSuite(profileName string, seed int64) (*SuiteResult, error) {
	profiles := predefinedProfiles()
	selected := make([]WorkloadProfile, 0)

	if profileName == "" || profileName == ProfileAll {
		names := make([]string, 0, len(profiles))
		for name := range profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			selected = append(selected, profiles[name])
		}
	} else {
		profile, exists := profiles[profileName]
		if !exists {
			return nil, fmt.Errorf("unknown benchmark profile '%s'", profileName)
		}
		selected = append(selected, profile)
	}

	suite := &SuiteResult{
		GeneratedAt:      time.Now(),
		Seed:             seed,
		RequestedProfile: profileName,
		Profiles:         make([]ProfileResult, 0, len(selected)),
	}

	for idx, profile := range selected {
		profileSeed := seed + int64(idx*997)
		result, err := runProfile(profile, profileSeed)
		if err != nil {
			return nil, fmt.Errorf("profile %s: %w", profile.Name, err)
		}
		suite.Profiles = append(suite.Profiles, result)
	}

	return suite, nil
}

// WriteArtifacts writes JSON/CSV/HTML benchmark outputs and returns the output directory.
func WriteArtifacts(suite *SuiteResult, outputBase string) (string, error) {
	if suite == nil {
		return "", fmt.Errorf("suite result is nil")
	}

	runID := suite.GeneratedAt.Format("20060102-150405")
	outputDir := filepath.Join(outputBase, runID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	if err := writeSummaryJSON(suite, filepath.Join(outputDir, "summary.json")); err != nil {
		return "", err
	}
	if err := writeMetricsCSV(suite, filepath.Join(outputDir, "metrics.csv")); err != nil {
		return "", err
	}
	if err := writeTaskRunsCSV(suite, filepath.Join(outputDir, "task_runs.csv")); err != nil {
		return "", err
	}
	if err := writeReportHTML(suite, filepath.Join(outputDir, "report.html")); err != nil {
		return "", err
	}
	if err := writeReportMarkdown(suite, filepath.Join(outputDir, "README.md")); err != nil {
		return "", err
	}

	return outputDir, nil
}

// AvailableProfiles lists benchmark profile identifiers.
func AvailableProfiles() []string {
	profiles := predefinedProfiles()
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetWorkloadProfile returns one predefined workload profile for live submissions.
func GetWorkloadProfile(name string) (WorkloadProfile, error) {
	profiles := predefinedProfiles()
	profile, ok := profiles[name]
	if !ok {
		return WorkloadProfile{}, fmt.Errorf("unknown workload profile '%s'", name)
	}
	return profile, nil
}

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
	AllocatedGPU     float64
	Running          []*runningTask
	AssignedCount    int
}

type simState struct {
	workers         map[string]*simWorker
	busyCPUSeconds  float64
	busyMemSeconds  float64
	busyGPUSeconds  float64
	totalCPUSeconds float64
	totalMemSeconds float64
	totalGPUSeconds float64
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
			GPUAvail:     worker.Profile.TotalGPU - worker.AllocatedGPU,
			Load:         workerLoad(worker),
		})
	}
	return views, nil
}

func (s *simTelemetrySource) GetWorkerLoad(workerID string) float64 {
	worker, ok := s.sim.workers[workerID]
	if !ok || !worker.Profile.InitialIsActive {
		return 0
	}
	return workerLoad(worker)
}

func runProfile(profile WorkloadProfile, seed int64) (ProfileResult, error) {
	paramsFile, err := writeRuntimeParams(profile)
	if err != nil {
		return ProfileResult{}, err
	}
	defer os.Remove(paramsFile)

	rrScheduler := scheduler.NewRoundRobinScheduler()
	rrResult, err := runSimulation(profile, rrScheduler, "Round-Robin", seed)
	if err != nil {
		return ProfileResult{}, err
	}

	tauStore := telemetry.NewInMemoryTauStore()
	for _, taskType := range canonicalTaskTypes {
		tauStore.SetTau(taskType, defaultTauForType(taskType))
	}
	for _, task := range profile.Tasks {
		tauStore.SetTau(task.TaskType, task.TauSeconds)
	}

	rtsFallback := scheduler.NewRoundRobinScheduler()
	rtsTelemetry := &simTelemetrySource{sim: newSimState(profile.Workers)}
	rtsScheduler := scheduler.NewRTSScheduler(rtsFallback, tauStore, rtsTelemetry, paramsFile, nil, 2.0)
	defer rtsScheduler.Shutdown()

	rtsResult, err := runSimulationWithTelemetry(profile, rtsScheduler, "RTS", seed, rtsTelemetry)
	if err != nil {
		return ProfileResult{}, err
	}

	winner, slaImprovement, waitReduction, makespanReduction, throughputGain := compareResults(rtsResult.Metrics, rrResult.Metrics)

	return ProfileResult{
		Profile:              profile.Name,
		Description:          profile.Description,
		TaskCount:            len(profile.Tasks),
		SchedulerResults:     []SchedulerResult{rtsResult, rrResult},
		Winner:               winner,
		SLAImprovementPct:    slaImprovement,
		WaitP95ReductionPct:  waitReduction,
		MakespanReductionPct: makespanReduction,
		ThroughputGainPct:    throughputGain,
	}, nil
}

func runSimulation(profile WorkloadProfile, sched scheduler.Scheduler, schedulerName string, seed int64) (SchedulerResult, error) {
	return runSimulationWithTelemetry(profile, sched, schedulerName, seed, nil)
}

func runSimulationWithTelemetry(profile WorkloadProfile, sched scheduler.Scheduler, schedulerName string, seed int64, telemetrySource *simTelemetrySource) (SchedulerResult, error) {
	if sched == nil {
		return SchedulerResult{}, fmt.Errorf("scheduler is nil")
	}
	if len(profile.Workers) == 0 {
		return SchedulerResult{}, fmt.Errorf("profile has no workers")
	}

	rng := rand.New(rand.NewSource(seed))
	tasks := make([]*simTask, 0, len(profile.Tasks))
	for idx, task := range profile.Tasks {
		if !scheduler.ValidateTaskType(task.TaskType) {
			return SchedulerResult{}, fmt.Errorf("invalid task type in profile %s task %d: %s", profile.Name, idx, task.TaskType)
		}
		sla := task.SLAMultiplier
		if sla <= 0 {
			sla = 2.0
		}
		arrival := task.ArrivalOffset
		deadline := arrival + time.Duration(sla*task.TauSeconds*float64(time.Second))
		jitter := 0.9 + rng.Float64()*0.2
		submittedAt := simulationEpochUnix + int64(arrival.Seconds())
		tasks = append(tasks, &simTask{
			Task:         task,
			Arrival:      arrival,
			Deadline:     deadline,
			JitterFactor: jitter,
			ProtoTask: &pb.Task{
				TaskId:        task.TaskID,
				TaskName:      task.TaskName,
				DockerImage:   task.DockerImage,
				Command:       task.Command,
				ReqCpu:        task.ReqCPU,
				ReqMemory:     task.ReqMemory,
				ReqStorage:    task.ReqStorage,
				ReqGpu:        task.ReqGPU,
				TaskType:      task.TaskType,
				SlaMultiplier: sla,
				SubmittedAt:   submittedAt,
				UserId:        "benchmark",
			},
		})
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Arrival < tasks[j].Arrival })

	sim := newSimState(profile.Workers)
	if telemetrySource != nil {
		telemetrySource.sim = sim
	}
	sched.Reset()

	nextArrivalIndex := 0
	queue := make([]*simTask, 0)
	taskRuns := make([]TaskRun, 0, len(tasks))
	decisionMS := make([]float64, 0, len(tasks)*2)
	lastEvent := time.Duration(0)
	current := time.Duration(0)
	unschedulable := 0

	for {
		nextArrival := time.Duration(math.MaxInt64)
		if nextArrivalIndex < len(tasks) {
			nextArrival = tasks[nextArrivalIndex].Arrival
		}

		nextCompletion := sim.nextCompletionTime()
		nextEvent := minDuration(nextArrival, nextCompletion)

		if nextEvent == time.Duration(math.MaxInt64) {
			if len(queue) > 0 {
				unschedulable += len(queue)
			}
			break
		}

		sim.accumulateUtilization(lastEvent, nextEvent)
		current = nextEvent
		lastEvent = nextEvent

		completed := sim.releaseCompleted(current)
		for _, run := range completed {
			taskRuns = append(taskRuns, TaskRun{
				TaskID:      run.Task.Task.TaskID,
				TaskType:    run.Task.Task.TaskType,
				WorkerID:    run.WorkerID,
				ArrivalSec:  run.Task.Arrival.Seconds(),
				StartSec:    run.StartAt.Seconds(),
				FinishSec:   run.FinishAt.Seconds(),
				WaitSec:     (run.StartAt - run.Task.Arrival).Seconds(),
				RuntimeSec:  (run.FinishAt - run.StartAt).Seconds(),
				DeadlineSec: run.Task.Deadline.Seconds(),
				SLASuccess:  run.FinishAt <= run.Task.Deadline,
			})
		}

		for nextArrivalIndex < len(tasks) && tasks[nextArrivalIndex].Arrival <= current {
			queue = append(queue, tasks[nextArrivalIndex])
			nextArrivalIndex++
		}

		queue = scheduleQueue(sim, queue, current, sched, &decisionMS)

		if len(queue) > 0 && nextArrivalIndex >= len(tasks) && !sim.hasRunning() {
			unschedulable += len(queue)
			break
		}
	}

	metrics := buildMetrics(tasks, taskRuns, decisionMS, sim, unschedulable)
	metrics.AssignmentsByWorker = sim.assignmentCounts()
	metrics.WorkerBalanceScore = workerBalanceScore(metrics.AssignmentsByWorker)

	return SchedulerResult{
		SchedulerName: schedulerName,
		Metrics:       metrics,
		TaskRuns:      taskRuns,
	}, nil
}

func scheduleQueue(sim *simState, queue []*simTask, now time.Duration, sched scheduler.Scheduler, decisionMS *[]float64) []*simTask {
	for {
		if len(queue) == 0 {
			return queue
		}

		assignedAny := false
		remaining := make([]*simTask, 0, len(queue))
		for _, task := range queue {
			workerInfos := sim.buildWorkerInfos()
			start := time.Now()
			workerID := sched.SelectWorker(task.ProtoTask, workerInfos)
			elapsed := time.Since(start)
			*decisionMS = append(*decisionMS, float64(elapsed.Microseconds())/1000.0)

			if workerID == "" || !sim.canAssign(workerID, task) {
				remaining = append(remaining, task)
				continue
			}

			runtime := sim.estimateRuntime(workerID, task)
			sim.assign(workerID, &runningTask{
				Task:     task,
				WorkerID: workerID,
				StartAt:  now,
				FinishAt: now + runtime,
			})
			assignedAny = true
		}

		queue = remaining
		if !assignedAny {
			return queue
		}
	}
}

func newSimState(profiles []WorkerProfile) *simState {
	workers := make(map[string]*simWorker, len(profiles))
	for _, profile := range profiles {
		if profile.SpeedByTask == nil {
			profile.SpeedByTask = map[string]float64{}
		}
		if !profile.InitialIsActive {
			profile.InitialIsActive = true
		}
		workers[profile.WorkerID] = &simWorker{Profile: profile, Running: make([]*runningTask, 0)}
	}
	return &simState{workers: workers}
}

func (s *simState) buildWorkerInfos() map[string]*scheduler.WorkerInfo {
	workerInfos := make(map[string]*scheduler.WorkerInfo, len(s.workers))
	for id, worker := range s.workers {
		workerInfos[id] = &scheduler.WorkerInfo{
			WorkerID:         id,
			IsActive:         worker.Profile.InitialIsActive,
			WorkerIP:         id,
			AvailableCPU:     worker.Profile.TotalCPU - worker.AllocatedCPU,
			AvailableMemory:  worker.Profile.TotalMemory - worker.AllocatedMemory,
			AvailableStorage: worker.Profile.TotalStorage - worker.AllocatedStorage,
			AvailableGPU:     worker.Profile.TotalGPU - worker.AllocatedGPU,
		}
	}
	return workerInfos
}

func (s *simState) canAssign(workerID string, task *simTask) bool {
	worker, ok := s.workers[workerID]
	if !ok || !worker.Profile.InitialIsActive {
		return false
	}
	return worker.Profile.TotalCPU-worker.AllocatedCPU >= task.Task.ReqCPU &&
		worker.Profile.TotalMemory-worker.AllocatedMemory >= task.Task.ReqMemory &&
		worker.Profile.TotalStorage-worker.AllocatedStorage >= task.Task.ReqStorage &&
		worker.Profile.TotalGPU-worker.AllocatedGPU >= task.Task.ReqGPU
}

func (s *simState) assign(workerID string, run *runningTask) {
	worker := s.workers[workerID]
	worker.AllocatedCPU += run.Task.Task.ReqCPU
	worker.AllocatedMemory += run.Task.Task.ReqMemory
	worker.AllocatedStorage += run.Task.Task.ReqStorage
	worker.AllocatedGPU += run.Task.Task.ReqGPU
	worker.Running = append(worker.Running, run)
	worker.AssignedCount++
}

func (s *simState) releaseCompleted(now time.Duration) []*runningTask {
	completed := make([]*runningTask, 0)
	for _, worker := range s.workers {
		if len(worker.Running) == 0 {
			continue
		}
		kept := make([]*runningTask, 0, len(worker.Running))
		for _, run := range worker.Running {
			if run.FinishAt <= now {
				worker.AllocatedCPU -= run.Task.Task.ReqCPU
				worker.AllocatedMemory -= run.Task.Task.ReqMemory
				worker.AllocatedStorage -= run.Task.Task.ReqStorage
				worker.AllocatedGPU -= run.Task.Task.ReqGPU
				if worker.AllocatedCPU < 0 {
					worker.AllocatedCPU = 0
				}
				if worker.AllocatedMemory < 0 {
					worker.AllocatedMemory = 0
				}
				if worker.AllocatedStorage < 0 {
					worker.AllocatedStorage = 0
				}
				if worker.AllocatedGPU < 0 {
					worker.AllocatedGPU = 0
				}
				completed = append(completed, run)
			} else {
				kept = append(kept, run)
			}
		}
		worker.Running = kept
	}
	return completed
}

func (s *simState) estimateRuntime(workerID string, task *simTask) time.Duration {
	worker := s.workers[workerID]
	speed := worker.Profile.SpeedByTask[task.Task.TaskType]
	if speed <= 0 {
		speed = 1.0
	}
	load := workerLoad(worker)
	base := task.Task.TauSeconds
	if base <= 0 {
		base = defaultTauForType(task.Task.TaskType)
	}
	runtimeSec := base * speed * (1.0 + 0.35*load) * task.JitterFactor
	if runtimeSec < 1.0 {
		runtimeSec = 1.0
	}
	return time.Duration(runtimeSec * float64(time.Second))
}

func (s *simState) nextCompletionTime() time.Duration {
	next := time.Duration(math.MaxInt64)
	for _, worker := range s.workers {
		for _, run := range worker.Running {
			if run.FinishAt < next {
				next = run.FinishAt
			}
		}
	}
	return next
}

func (s *simState) hasRunning() bool {
	for _, worker := range s.workers {
		if len(worker.Running) > 0 {
			return true
		}
	}
	return false
}

func (s *simState) accumulateUtilization(last, now time.Duration) {
	if now <= last {
		return
	}
	deltaSec := (now - last).Seconds()
	if deltaSec <= 0 {
		return
	}

	for _, worker := range s.workers {
		s.busyCPUSeconds += worker.AllocatedCPU * deltaSec
		s.busyMemSeconds += worker.AllocatedMemory * deltaSec
		s.busyGPUSeconds += worker.AllocatedGPU * deltaSec
		s.totalCPUSeconds += worker.Profile.TotalCPU * deltaSec
		s.totalMemSeconds += worker.Profile.TotalMemory * deltaSec
		s.totalGPUSeconds += worker.Profile.TotalGPU * deltaSec
	}
}

func (s *simState) assignmentCounts() map[string]int {
	counts := make(map[string]int, len(s.workers))
	for _, worker := range s.workers {
		counts[worker.Profile.WorkerID] = worker.AssignedCount
	}
	return counts
}

func workerLoad(worker *simWorker) float64 {
	if worker == nil {
		return 0
	}
	wCPU := worker.Profile.TotalCPU
	wMem := worker.Profile.TotalMemory / 10.0
	wGPU := worker.Profile.TotalGPU * 2.0
	totalW := wCPU + wMem + wGPU
	if totalW <= 0 {
		return 0
	}
	load := (wCPU*safeDiv(worker.AllocatedCPU, maxFloat(worker.Profile.TotalCPU, 1.0)) +
		wMem*safeDiv(worker.AllocatedMemory, maxFloat(worker.Profile.TotalMemory, 1.0)) +
		wGPU*safeDiv(worker.AllocatedGPU, maxFloat(worker.Profile.TotalGPU, 1.0))) / totalW
	if load < 0 {
		return 0
	}
	return load
}

func buildMetrics(allTasks []*simTask, taskRuns []TaskRun, decisionsMS []float64, sim *simState, unschedulable int) SchedulerMetrics {
	waits := make([]float64, 0, len(taskRuns))
	runtimes := make([]float64, 0, len(taskRuns))
	slaSuccess := 0
	minArrival := math.MaxFloat64
	maxFinish := 0.0

	for _, run := range taskRuns {
		waits = append(waits, run.WaitSec)
		runtimes = append(runtimes, run.RuntimeSec)
		if run.SLASuccess {
			slaSuccess++
		}
		if run.ArrivalSec < minArrival {
			minArrival = run.ArrivalSec
		}
		if run.FinishSec > maxFinish {
			maxFinish = run.FinishSec
		}
	}

	totalTasks := len(allTasks)
	completed := len(taskRuns)
	makespan := 0.0
	if completed > 0 {
		makespan = maxFinish - minArrival
		if makespan < 0 {
			makespan = 0
		}
	}

	throughput := 0.0
	if makespan > 0 {
		throughput = float64(completed) / (makespan / 60.0)
	}

	cpuUtil := safeDiv(sim.busyCPUSeconds, sim.totalCPUSeconds) * 100.0
	memUtil := safeDiv(sim.busyMemSeconds, sim.totalMemSeconds) * 100.0
	gpuUtil := safeDiv(sim.busyGPUSeconds, sim.totalGPUSeconds) * 100.0
	if math.IsNaN(gpuUtil) || math.IsInf(gpuUtil, 0) {
		gpuUtil = 0.0
	}

	return SchedulerMetrics{
		TotalTasks:            totalTasks,
		CompletedTasks:        completed,
		SLASuccessRatePct:     safeDiv(float64(slaSuccess), float64(maxInt(totalTasks, 1))) * 100.0,
		AvgQueueWaitSec:       mean(waits),
		P95QueueWaitSec:       percentile(waits, 95),
		AvgRuntimeSec:         mean(runtimes),
		MakespanSec:           makespan,
		ThroughputTasksPerMin: throughput,
		CPUUtilizationPct:     cpuUtil,
		MemoryUtilizationPct:  memUtil,
		GPUUtilizationPct:     gpuUtil,
		AvgDecisionMS:         mean(decisionsMS),
		P95DecisionMS:         percentile(decisionsMS, 95),
		UnschedulableTasks:    unschedulable,
	}
}

func compareResults(rts, rr SchedulerMetrics) (winner string, slaImprovement, waitReduction, makespanReduction, throughputGain float64) {
	slaImprovement = pctChange(rts.SLASuccessRatePct, rr.SLASuccessRatePct)
	waitReduction = pctReduction(rts.P95QueueWaitSec, rr.P95QueueWaitSec)
	makespanReduction = pctReduction(rts.MakespanSec, rr.MakespanSec)
	throughputGain = pctChange(rts.ThroughputTasksPerMin, rr.ThroughputTasksPerMin)

	maxThroughput := maxFloat(rts.ThroughputTasksPerMin, rr.ThroughputTasksPerMin)
	throughputNormRTS := safeDiv(rts.ThroughputTasksPerMin, maxFloat(maxThroughput, 1e-9))
	throughputNormRR := safeDiv(rr.ThroughputTasksPerMin, maxFloat(maxThroughput, 1e-9))
	maxWait := maxFloat(rts.P95QueueWaitSec, rr.P95QueueWaitSec)
	waitScoreRTS := 1.0 - safeDiv(rts.P95QueueWaitSec, maxFloat(maxWait, 1e-9))
	waitScoreRR := 1.0 - safeDiv(rr.P95QueueWaitSec, maxFloat(maxWait, 1e-9))

	rtsScore := 0.45*(rts.SLASuccessRatePct/100.0) + 0.20*throughputNormRTS + 0.15*waitScoreRTS + 0.10*(rts.CPUUtilizationPct/100.0) + 0.10*rts.WorkerBalanceScore
	rrScore := 0.45*(rr.SLASuccessRatePct/100.0) + 0.20*throughputNormRR + 0.15*waitScoreRR + 0.10*(rr.CPUUtilizationPct/100.0) + 0.10*rr.WorkerBalanceScore
	if rtsScore >= rrScore {
		winner = "RTS"
	} else {
		winner = "Round-Robin"
	}

	return winner, slaImprovement, waitReduction, makespanReduction, throughputGain
}

func writeSummaryJSON(suite *SuiteResult, path string) error {
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary json: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write summary json: %w", err)
	}
	return nil
}

func writeMetricsCSV(suite *SuiteResult, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create metrics csv: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	headers := []string{
		"profile", "scheduler", "total_tasks", "completed_tasks", "unschedulable_tasks",
		"sla_success_rate_pct", "avg_queue_wait_sec", "p95_queue_wait_sec", "avg_runtime_sec",
		"makespan_sec", "throughput_tasks_per_min", "cpu_utilization_pct", "memory_utilization_pct",
		"gpu_utilization_pct", "worker_balance_score", "avg_decision_ms", "p95_decision_ms",
	}
	if err := w.Write(headers); err != nil {
		return fmt.Errorf("write metrics csv headers: %w", err)
	}

	for _, profile := range suite.Profiles {
		for _, result := range profile.SchedulerResults {
			m := result.Metrics
			row := []string{
				profile.Profile,
				result.SchedulerName,
				fmt.Sprintf("%d", m.TotalTasks),
				fmt.Sprintf("%d", m.CompletedTasks),
				fmt.Sprintf("%d", m.UnschedulableTasks),
				fmt.Sprintf("%.3f", m.SLASuccessRatePct),
				fmt.Sprintf("%.3f", m.AvgQueueWaitSec),
				fmt.Sprintf("%.3f", m.P95QueueWaitSec),
				fmt.Sprintf("%.3f", m.AvgRuntimeSec),
				fmt.Sprintf("%.3f", m.MakespanSec),
				fmt.Sprintf("%.3f", m.ThroughputTasksPerMin),
				fmt.Sprintf("%.3f", m.CPUUtilizationPct),
				fmt.Sprintf("%.3f", m.MemoryUtilizationPct),
				fmt.Sprintf("%.3f", m.GPUUtilizationPct),
				fmt.Sprintf("%.3f", m.WorkerBalanceScore),
				fmt.Sprintf("%.6f", m.AvgDecisionMS),
				fmt.Sprintf("%.6f", m.P95DecisionMS),
			}
			if err := w.Write(row); err != nil {
				return fmt.Errorf("write metrics csv row: %w", err)
			}
		}
	}
	return nil
}

func writeTaskRunsCSV(suite *SuiteResult, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create task runs csv: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	headers := []string{"profile", "scheduler", "task_id", "task_type", "worker_id", "arrival_sec", "start_sec", "finish_sec", "wait_sec", "runtime_sec", "deadline_sec", "sla_success"}
	if err := w.Write(headers); err != nil {
		return fmt.Errorf("write task runs csv headers: %w", err)
	}

	for _, profile := range suite.Profiles {
		for _, result := range profile.SchedulerResults {
			for _, run := range result.TaskRuns {
				row := []string{
					profile.Profile,
					result.SchedulerName,
					run.TaskID,
					run.TaskType,
					run.WorkerID,
					fmt.Sprintf("%.3f", run.ArrivalSec),
					fmt.Sprintf("%.3f", run.StartSec),
					fmt.Sprintf("%.3f", run.FinishSec),
					fmt.Sprintf("%.3f", run.WaitSec),
					fmt.Sprintf("%.3f", run.RuntimeSec),
					fmt.Sprintf("%.3f", run.DeadlineSec),
					fmt.Sprintf("%t", run.SLASuccess),
				}
				if err := w.Write(row); err != nil {
					return fmt.Errorf("write task runs csv row: %w", err)
				}
			}
		}
	}
	return nil
}

func writeReportMarkdown(suite *SuiteResult, path string) error {
	var b strings.Builder
	b.WriteString("# Scheduling Benchmark Report\n\n")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", suite.GeneratedAt.Format(time.RFC3339)))
	for _, profile := range suite.Profiles {
		b.WriteString(fmt.Sprintf("## %s\n\n", profile.Profile))
		b.WriteString(profile.Description + "\n\n")
		b.WriteString("| Scheduler | SLA % | P95 Wait (s) | Throughput (tasks/min) | Makespan (s) | CPU Util % | Balance |\n")
		b.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")
		for _, result := range profile.SchedulerResults {
			m := result.Metrics
			b.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %.2f | %.2f | %.2f | %.3f |\n",
				result.SchedulerName, m.SLASuccessRatePct, m.P95QueueWaitSec, m.ThroughputTasksPerMin, m.MakespanSec, m.CPUUtilizationPct, m.WorkerBalanceScore))
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("Winner: **%s**\n\n", profile.Winner))
		b.WriteString(fmt.Sprintf("- SLA improvement (RTS vs RR): %.2f%%\n", profile.SLAImprovementPct))
		b.WriteString(fmt.Sprintf("- P95 queue wait reduction: %.2f%%\n", profile.WaitP95ReductionPct))
		b.WriteString(fmt.Sprintf("- Makespan reduction: %.2f%%\n", profile.MakespanReductionPct))
		b.WriteString(fmt.Sprintf("- Throughput gain: %.2f%%\n\n", profile.ThroughputGainPct))
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func writeReportHTML(suite *SuiteResult, path string) error {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Scheduling Benchmark Report</title>")
	b.WriteString("<style>body{font-family:Helvetica,Arial,sans-serif;margin:24px;background:#f8fafc;color:#111827}h1,h2{margin:0 0 12px}section{background:white;padding:16px;border-radius:12px;margin-bottom:16px;box-shadow:0 1px 4px rgba(0,0,0,.08)}table{width:100%;border-collapse:collapse;margin-top:8px}th,td{padding:8px;border-bottom:1px solid #e5e7eb;text-align:left}th{text-transform:uppercase;font-size:12px;color:#6b7280}.bar-wrap{margin:8px 0}.label{font-size:12px;color:#4b5563;margin-bottom:4px}.bar{height:14px;border-radius:999px;display:inline-block}.rts{background:#059669}.rr{background:#f97316}.row{margin-bottom:10px}.legend{font-size:12px;color:#6b7280}.pill{display:inline-block;padding:4px 10px;border-radius:999px;background:#e5e7eb;font-size:12px}</style></head><body>")
	b.WriteString(fmt.Sprintf("<h1>Scheduling Benchmark Report</h1><p>Generated %s</p>", html.EscapeString(suite.GeneratedAt.Format(time.RFC3339))))

	for _, profile := range suite.Profiles {
		var rts *SchedulerResult
		var rr *SchedulerResult
		for i := range profile.SchedulerResults {
			if profile.SchedulerResults[i].SchedulerName == "RTS" {
				rts = &profile.SchedulerResults[i]
			} else if profile.SchedulerResults[i].SchedulerName == "Round-Robin" {
				rr = &profile.SchedulerResults[i]
			}
		}
		if rts == nil || rr == nil {
			continue
		}

		b.WriteString("<section>")
		b.WriteString(fmt.Sprintf("<h2>%s</h2>", html.EscapeString(profile.Profile)))
		b.WriteString(fmt.Sprintf("<p>%s</p>", html.EscapeString(profile.Description)))
		b.WriteString(fmt.Sprintf("<p><span class=\"pill\">Winner: %s</span></p>", html.EscapeString(profile.Winner)))
		b.WriteString("<table><thead><tr><th>Scheduler</th><th>SLA %</th><th>P95 Wait (s)</th><th>Throughput</th><th>Makespan (s)</th><th>CPU Util %</th><th>Balance</th></tr></thead><tbody>")
		for _, result := range profile.SchedulerResults {
			m := result.Metrics
			b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.3f</td></tr>", html.EscapeString(result.SchedulerName), m.SLASuccessRatePct, m.P95QueueWaitSec, m.ThroughputTasksPerMin, m.MakespanSec, m.CPUUtilizationPct, m.WorkerBalanceScore))
		}
		b.WriteString("</tbody></table>")

		b.WriteString("<div class=\"legend\">Bars are normalized within this profile.</div>")
		b.WriteString(renderMetricBars("SLA Success %", rts.Metrics.SLASuccessRatePct, rr.Metrics.SLASuccessRatePct, true))
		b.WriteString(renderMetricBars("Throughput (tasks/min)", rts.Metrics.ThroughputTasksPerMin, rr.Metrics.ThroughputTasksPerMin, true))
		b.WriteString(renderMetricBars("P95 Queue Wait (s)", rts.Metrics.P95QueueWaitSec, rr.Metrics.P95QueueWaitSec, false))
		b.WriteString(renderMetricBars("Makespan (s)", rts.Metrics.MakespanSec, rr.Metrics.MakespanSec, false))
		b.WriteString("</section>")
	}

	b.WriteString("</body></html>")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func renderMetricBars(label string, rts, rr float64, higherIsBetter bool) string {
	maxVal := maxFloat(rts, rr)
	if maxVal <= 0 {
		maxVal = 1.0
	}
	rtsWidth := 0.0
	rrWidth := 0.0
	if higherIsBetter {
		rtsWidth = (rts / maxVal) * 100.0
		rrWidth = (rr / maxVal) * 100.0
	} else {
		minPositive := minPositive(rts, rr)
		if minPositive <= 0 {
			minPositive = 1.0
		}
		rtsWidth = (minPositive / maxFloat(rts, minPositive)) * 100.0
		rrWidth = (minPositive / maxFloat(rr, minPositive)) * 100.0
	}
	if rtsWidth < 2 {
		rtsWidth = 2
	}
	if rrWidth < 2 {
		rrWidth = 2
	}

	return fmt.Sprintf(
		"<div class=\"bar-wrap\"><div class=\"label\">%s</div><div class=\"row\">RTS %.2f<div class=\"bar rts\" style=\"width:%.2f%%\"></div></div><div class=\"row\">Round-Robin %.2f<div class=\"bar rr\" style=\"width:%.2f%%\"></div></div></div>",
		html.EscapeString(label), rts, rtsWidth, rr, rrWidth,
	)
}

func writeRuntimeParams(profile WorkloadProfile) (string, error) {
	params := scheduler.GetDefaultGAParams()
	params.AffinityMatrix = make(map[string]map[string]float64)
	params.PenaltyVector = make(map[string]float64)

	for _, taskType := range canonicalTaskTypes {
		params.AffinityMatrix[taskType] = make(map[string]float64)
		for _, worker := range profile.Workers {
			speed := worker.SpeedByTask[taskType]
			if speed <= 0 {
				speed = 1.0
			}
			affinity := clamp(1.5/speed, -10.0, 10.0)
			params.AffinityMatrix[taskType][worker.WorkerID] = affinity
		}
	}

	for _, worker := range profile.Workers {
		penalty := worker.Penalty
		if penalty < 0 {
			penalty = 0
		}
		params.PenaltyVector[worker.WorkerID] = penalty
	}

	file, err := os.CreateTemp("", "benchmark-ga-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp params file: %w", err)
	}
	defer file.Close()
	if err := params.SaveToFile(file.Name()); err != nil {
		return "", fmt.Errorf("save runtime params: %w", err)
	}
	return file.Name(), nil
}

func predefinedProfiles() map[string]WorkloadProfile {
	workers := defaultWorkers()
	return map[string]WorkloadProfile{
		ProfileShowcase: buildShowcaseProfile(workers),
		ProfileSteady:   buildSteadyProfile(workers),
		ProfileGPUSpike: buildGPUSpikeProfile(workers),
	}
}

func defaultWorkers() []WorkerProfile {
	allTypes := func(mult map[string]float64) map[string]float64 {
		out := make(map[string]float64, len(canonicalTaskTypes))
		for _, t := range canonicalTaskTypes {
			if v, ok := mult[t]; ok {
				out[t] = v
			} else {
				out[t] = 1.0
			}
		}
		return out
	}

	return []WorkerProfile{
		{
			WorkerID:        "worker-cpu-a",
			TotalCPU:        20,
			TotalMemory:     32,
			TotalStorage:    500,
			TotalGPU:        0,
			InitialIsActive: true,
			Penalty:         0.2,
			SpeedByTask: allTypes(map[string]float64{
				scheduler.TaskTypeCPULight:    0.80,
				scheduler.TaskTypeCPUHeavy:    0.65,
				scheduler.TaskTypeMemoryHeavy: 1.15,
				scheduler.TaskTypeMixed:       0.95,
			}),
		},
		{
			WorkerID:        "worker-mem-a",
			TotalCPU:        14,
			TotalMemory:     72,
			TotalStorage:    500,
			TotalGPU:        0,
			InitialIsActive: true,
			Penalty:         0.1,
			SpeedByTask: allTypes(map[string]float64{
				scheduler.TaskTypeMemoryHeavy: 0.62,
				scheduler.TaskTypeMixed:       0.88,
				scheduler.TaskTypeCPUHeavy:    1.05,
			}),
		},
		{
			WorkerID:        "worker-gpu-a",
			TotalCPU:        24,
			TotalMemory:     64,
			TotalStorage:    700,
			TotalGPU:        4,
			InitialIsActive: true,
			Penalty:         0.05,
			SpeedByTask: allTypes(map[string]float64{
				scheduler.TaskTypeGPUInference: 0.52,
				scheduler.TaskTypeGPUTraining:  0.58,
				scheduler.TaskTypeMixed:        0.88,
				scheduler.TaskTypeCPUHeavy:     1.20,
			}),
		},
		{
			WorkerID:        "worker-balanced",
			TotalCPU:        16,
			TotalMemory:     40,
			TotalStorage:    500,
			TotalGPU:        1,
			InitialIsActive: true,
			Penalty:         0.15,
			SpeedByTask:     allTypes(map[string]float64{}),
		},
	}
}

func buildShowcaseProfile(workers []WorkerProfile) WorkloadProfile {
	tasks := make([]WorkloadTask, 0)
	slot := 0
	for tick := 0; tick < 42; tick++ {
		arrival := time.Duration(tick*8) * time.Second
		tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "cpu-light", arrival))
		slot++
		if tick%2 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "memory-heavy", arrival+2*time.Second))
			slot++
		}
		if tick%3 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "cpu-heavy", arrival+3*time.Second))
			slot++
		}
		if tick%4 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "mixed", arrival+4*time.Second))
			slot++
		}
		if tick%5 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "gpu-inference", arrival+5*time.Second))
			slot++
		}
		if tick%11 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "gpu-training", arrival+6*time.Second))
			slot++
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ArrivalOffset < tasks[j].ArrivalOffset })
	return WorkloadProfile{
		Name:        ProfileShowcase,
		Description: "Burst-heavy mixed workload with CPU, memory, GPU inference, and periodic GPU training spikes.",
		Workers:     cloneWorkers(workers),
		Tasks:       tasks,
	}
}

func buildSteadyProfile(workers []WorkerProfile) WorkloadProfile {
	tasks := make([]WorkloadTask, 0)
	slot := 0
	for tick := 0; tick < 60; tick++ {
		arrival := time.Duration(tick*6) * time.Second
		tasks = append(tasks, newTask(fmt.Sprintf("steady-%03d", slot), "cpu-light", arrival))
		slot++
		if tick%3 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("steady-%03d", slot), "mixed", arrival+1*time.Second))
			slot++
		}
		if tick%5 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("steady-%03d", slot), "memory-heavy", arrival+2*time.Second))
			slot++
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ArrivalOffset < tasks[j].ArrivalOffset })
	return WorkloadProfile{
		Name:        ProfileSteady,
		Description: "Steady-state stream emphasizing queue latency and throughput consistency over long horizons.",
		Workers:     cloneWorkers(workers),
		Tasks:       tasks,
	}
}

func buildGPUSpikeProfile(workers []WorkerProfile) WorkloadProfile {
	tasks := make([]WorkloadTask, 0)
	slot := 0
	for tick := 0; tick < 34; tick++ {
		arrival := time.Duration(tick*9) * time.Second
		tasks = append(tasks, newTask(fmt.Sprintf("gpu-%03d", slot), "cpu-light", arrival))
		slot++
		tasks = append(tasks, newTask(fmt.Sprintf("gpu-%03d", slot), "gpu-inference", arrival+1*time.Second))
		slot++
		if tick%2 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("gpu-%03d", slot), "gpu-inference", arrival+2*time.Second))
			slot++
		}
		if tick%6 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("gpu-%03d", slot), "gpu-training", arrival+3*time.Second))
			slot++
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ArrivalOffset < tasks[j].ArrivalOffset })
	return WorkloadProfile{
		Name:        ProfileGPUSpike,
		Description: "GPU-first workload with recurring inference bursts and periodic training surges.",
		Workers:     cloneWorkers(workers),
		Tasks:       tasks,
	}
}

func newTask(taskID, taskType string, arrival time.Duration) WorkloadTask {
	image := "docker.io/library/alpine:3.19"
	makeCmd := func(sec int) string {
		return fmt.Sprintf("sh -c \"sleep %d; echo %s done\"", sec, taskType)
	}

	switch taskType {
	case "cpu-light":
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: image, Command: makeCmd(8), ReqCPU: 1.0, ReqMemory: 1.0, ReqStorage: 1.0, ReqGPU: 0, TaskType: scheduler.TaskTypeCPULight, SLAMultiplier: 2.0, TauSeconds: 8}
	case "cpu-heavy":
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: image, Command: makeCmd(24), ReqCPU: 4.0, ReqMemory: 4.0, ReqStorage: 2.0, ReqGPU: 0, TaskType: scheduler.TaskTypeCPUHeavy, SLAMultiplier: 2.2, TauSeconds: 24}
	case "memory-heavy":
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: image, Command: makeCmd(28), ReqCPU: 2.0, ReqMemory: 12.0, ReqStorage: 3.0, ReqGPU: 0, TaskType: scheduler.TaskTypeMemoryHeavy, SLAMultiplier: 2.2, TauSeconds: 28}
	case "gpu-inference":
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: image, Command: makeCmd(18), ReqCPU: 2.0, ReqMemory: 6.0, ReqStorage: 2.0, ReqGPU: 1.0, TaskType: scheduler.TaskTypeGPUInference, SLAMultiplier: 2.0, TauSeconds: 18}
	case "gpu-training":
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: image, Command: makeCmd(45), ReqCPU: 4.0, ReqMemory: 10.0, ReqStorage: 4.0, ReqGPU: 2.0, TaskType: scheduler.TaskTypeGPUTraining, SLAMultiplier: 2.4, TauSeconds: 45}
	case "mixed":
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: image, Command: makeCmd(20), ReqCPU: 3.0, ReqMemory: 6.0, ReqStorage: 2.0, ReqGPU: 0.5, TaskType: scheduler.TaskTypeMixed, SLAMultiplier: 2.1, TauSeconds: 20}
	default:
		return WorkloadTask{TaskID: taskID, TaskName: "mixed", ArrivalOffset: arrival, DockerImage: image, Command: makeCmd(20), ReqCPU: 2.0, ReqMemory: 4.0, ReqStorage: 2.0, ReqGPU: 0, TaskType: scheduler.TaskTypeMixed, SLAMultiplier: 2.0, TauSeconds: 20}
	}
}

func cloneWorkers(workers []WorkerProfile) []WorkerProfile {
	out := make([]WorkerProfile, len(workers))
	for i, worker := range workers {
		copyMap := make(map[string]float64, len(worker.SpeedByTask))
		for taskType, speed := range worker.SpeedByTask {
			copyMap[taskType] = speed
		}
		worker.SpeedByTask = copyMap
		out[i] = worker
	}
	return out
}

func defaultTauForType(taskType string) float64 {
	switch taskType {
	case scheduler.TaskTypeCPULight:
		return 8
	case scheduler.TaskTypeCPUHeavy:
		return 24
	case scheduler.TaskTypeMemoryHeavy:
		return 28
	case scheduler.TaskTypeGPUInference:
		return 18
	case scheduler.TaskTypeGPUTraining:
		return 45
	case scheduler.TaskTypeMixed:
		return 20
	default:
		return 20
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyVals := make([]float64, len(values))
	copy(copyVals, values)
	sort.Float64s(copyVals)
	if p <= 0 {
		return copyVals[0]
	}
	if p >= 100 {
		return copyVals[len(copyVals)-1]
	}
	idx := int(math.Ceil((p/100.0)*float64(len(copyVals)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(copyVals) {
		idx = len(copyVals) - 1
	}
	return copyVals[idx]
}

func pctChange(newVal, base float64) float64 {
	if math.Abs(base) < 1e-9 {
		if math.Abs(newVal) < 1e-9 {
			return 0
		}
		return 100
	}
	return ((newVal - base) / base) * 100.0
}

func pctReduction(newVal, base float64) float64 {
	if math.Abs(base) < 1e-9 {
		return 0
	}
	return ((base - newVal) / base) * 100.0
}

func safeDiv(a, b float64) float64 {
	if math.Abs(b) < 1e-9 {
		return 0
	}
	return a / b
}

func clamp(value, minVal, maxVal float64) float64 {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

func minPositive(values ...float64) float64 {
	min := math.MaxFloat64
	for _, v := range values {
		if v > 0 && v < min {
			min = v
		}
	}
	if min == math.MaxFloat64 {
		return 0
	}
	return min
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func workerBalanceScore(counts map[string]int) float64 {
	if len(counts) == 0 {
		return 0
	}

	values := make([]float64, 0, len(counts))
	total := 0.0
	for _, count := range counts {
		values = append(values, float64(count))
		total += float64(count)
	}
	meanVal := safeDiv(total, float64(len(values)))
	if meanVal <= 0 {
		return 0
	}

	var variance float64
	for _, value := range values {
		diff := value - meanVal
		variance += diff * diff
	}
	variance /= float64(len(values))
	stddev := math.Sqrt(variance)

	score := 1.0 - clamp(stddev/meanVal, 0, 1)
	return clamp(score, 0, 1)
}
