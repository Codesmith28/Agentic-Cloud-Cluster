package benchmark

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"master/internal/scheduler"
	"master/internal/telemetry"
	pb "master/proto"
)

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
		worker.Profile.TotalStorage-worker.AllocatedStorage >= task.Task.ReqStorage
}

func (s *simState) assign(workerID string, run *runningTask) {
	worker := s.workers[workerID]
	worker.AllocatedCPU += run.Task.Task.ReqCPU
	worker.AllocatedMemory += run.Task.Task.ReqMemory
	worker.AllocatedStorage += run.Task.Task.ReqStorage
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
				if worker.AllocatedCPU < 0 {
					worker.AllocatedCPU = 0
				}
				if worker.AllocatedMemory < 0 {
					worker.AllocatedMemory = 0
				}
				if worker.AllocatedStorage < 0 {
					worker.AllocatedStorage = 0
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
		s.totalCPUSeconds += worker.Profile.TotalCPU * deltaSec
		s.totalMemSeconds += worker.Profile.TotalMemory * deltaSec
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
	wStorage := worker.Profile.TotalStorage / 100.0
	totalW := wCPU + wMem + wStorage
	if totalW <= 0 {
		return 0
	}
	load := (wCPU*safeDiv(worker.AllocatedCPU, maxFloat(worker.Profile.TotalCPU, 1.0)) +
		wMem*safeDiv(worker.AllocatedMemory, maxFloat(worker.Profile.TotalMemory, 1.0)) +
		wStorage*safeDiv(worker.AllocatedStorage, maxFloat(worker.Profile.TotalStorage, 1.0))) / totalW
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

// Math and stats helper functions

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
