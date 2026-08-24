package benchmark

import (
	"fmt"
	"sort"
	"time"

	"master/internal/scheduler"
)

// AvailableProfiles returns the list of valid benchmark profile names.
func AvailableProfiles() []string {
	return []string{
		ProfileShowcase,
		ProfileSteady,
		ProfileBursty,
	}
}

// GetWorkloadProfile returns a predefined WorkloadProfile by name.
func GetWorkloadProfile(name string) (WorkloadProfile, error) {
	profiles := predefinedProfiles()
	profile, ok := profiles[name]
	if !ok {
		return WorkloadProfile{}, fmt.Errorf("unknown workload profile '%s'", name)
	}
	return profile, nil
}

func predefinedProfiles() map[string]WorkloadProfile {
	workers := defaultWorkers()
	return map[string]WorkloadProfile{
		ProfileShowcase: buildShowcaseProfile(workers),
		ProfileSteady:   buildSteadyProfile(workers),
		ProfileBursty:   buildBurstyProfile(workers),
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
			InitialIsActive: true,
			Penalty:         0.1,
			SpeedByTask: allTypes(map[string]float64{
				scheduler.TaskTypeMemoryHeavy: 0.62,
				scheduler.TaskTypeMixed:       0.88,
				scheduler.TaskTypeCPUHeavy:    1.05,
			}),
		},
		{
			WorkerID:        "worker-storage-a",
			TotalCPU:        12,
			TotalMemory:     48,
			TotalStorage:    1200,
			InitialIsActive: true,
			Penalty:         0.05,
			SpeedByTask: allTypes(map[string]float64{
				scheduler.TaskTypeMemoryHeavy: 0.80,
				scheduler.TaskTypeMixed:       0.86,
				scheduler.TaskTypeCPUHeavy:    1.10,
			}),
		},
		{
			WorkerID:        "worker-balanced",
			TotalCPU:        16,
			TotalMemory:     40,
			TotalStorage:    500,
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
		tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "cpu-light", arrival, slot))
		slot++
		if tick%2 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "memory-heavy", arrival+2*time.Second, slot))
			slot++
		}
		if tick%3 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "cpu-heavy", arrival+3*time.Second, slot))
			slot++
		}
		if tick%4 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "mixed", arrival+4*time.Second, slot))
			slot++
		}
		if tick%5 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "cpu-heavy", arrival+5*time.Second, slot))
			slot++
		}
		if tick%11 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("showcase-%03d", slot), "memory-heavy", arrival+6*time.Second, slot))
			slot++
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ArrivalOffset < tasks[j].ArrivalOffset })
	return WorkloadProfile{
		Name:        ProfileShowcase,
		Description: "Burst-heavy mixed workload with CPU, memory, and periodic resource pressure spikes.",
		Workers:     cloneWorkers(workers),
		Tasks:       tasks,
	}
}

func buildSteadyProfile(workers []WorkerProfile) WorkloadProfile {
	tasks := make([]WorkloadTask, 0)
	slot := 0
	for tick := 0; tick < 60; tick++ {
		arrival := time.Duration(tick*6) * time.Second
		tasks = append(tasks, newTask(fmt.Sprintf("steady-%03d", slot), "cpu-light", arrival, slot))
		slot++
		if tick%3 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("steady-%03d", slot), "mixed", arrival+1*time.Second, slot))
			slot++
		}
		if tick%5 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("steady-%03d", slot), "memory-heavy", arrival+2*time.Second, slot))
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

func buildBurstyProfile(workers []WorkerProfile) WorkloadProfile {
	tasks := make([]WorkloadTask, 0)
	slot := 0
	for tick := 0; tick < 34; tick++ {
		arrival := time.Duration(tick*7) * time.Second
		tasks = append(tasks, newTask(fmt.Sprintf("burst-%03d", slot), "cpu-light", arrival, slot))
		slot++
		tasks = append(tasks, newTask(fmt.Sprintf("burst-%03d", slot), "mixed", arrival+1*time.Second, slot))
		slot++
		if tick%2 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("burst-%03d", slot), "memory-heavy", arrival+2*time.Second, slot))
			slot++
		}
		if tick%3 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("burst-%03d", slot), "cpu-heavy", arrival+3*time.Second, slot))
			slot++
		}
		if tick%5 == 0 {
			tasks = append(tasks, newTask(fmt.Sprintf("burst-%03d", slot), "mixed", arrival+4*time.Second, slot))
			slot++
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ArrivalOffset < tasks[j].ArrivalOffset })
	return WorkloadProfile{
		Name:        ProfileBursty,
		Description: "Bursty heterogeneous workload with recurring CPU and memory surges.",
		Workers:     cloneWorkers(workers),
		Tasks:       tasks,
	}
}

const deterministicWorkflowImage = "agentic-cloud-cluster/workflow-deterministic:v1"

var deterministicWorkflowArgsByType = map[string][]string{
	scheduler.TaskTypeCPULight: {
		"--iterations 380000 --seed 1001",
		"--iterations 450000 --seed 1002",
		"--iterations 520000 --seed 1003",
		"--iterations 590000 --seed 1004",
		"--iterations 660000 --seed 1005",
	},
	scheduler.TaskTypeCPUHeavy: {
		"--iterations 3200000 --seed 2001",
		"--iterations 3700000 --seed 2002",
		"--iterations 4200000 --seed 2003",
		"--iterations 4700000 --seed 2004",
		"--iterations 5200000 --seed 2005",
	},
	scheduler.TaskTypeMemoryHeavy: {
		"--memory-mib 320 --passes 2 --seed 3001",
		"--memory-mib 384 --passes 2 --seed 3002",
		"--memory-mib 448 --passes 3 --seed 3003",
		"--memory-mib 512 --passes 3 --seed 3004",
		"--memory-mib 640 --passes 3 --seed 3005",
	},
	scheduler.TaskTypeMixed: {
		"--iterations 1400000 --memory-mib 160 --passes 2 --seed 4001",
		"--iterations 1700000 --memory-mib 192 --passes 2 --seed 4002",
		"--iterations 2000000 --memory-mib 224 --passes 2 --seed 4003",
		"--iterations 2300000 --memory-mib 256 --passes 2 --seed 4004",
		"--iterations 2600000 --memory-mib 288 --passes 3 --seed 4005",
	},
}

func deterministicWorkflowCommand(taskType string, variant int) string {
	profile := taskType
	argsByType, ok := deterministicWorkflowArgsByType[taskType]
	if !ok || len(argsByType) == 0 {
		profile = scheduler.TaskTypeMixed
		argsByType = deterministicWorkflowArgsByType[scheduler.TaskTypeMixed]
	}
	argSet := argsByType[variant%len(argsByType)]
	return fmt.Sprintf("/usr/local/bin/workflow %s %s", profile, argSet)
}

func newTask(taskID, taskType string, arrival time.Duration, variant int) WorkloadTask {
	command := deterministicWorkflowCommand(taskType, variant)

	switch taskType {
	case scheduler.TaskTypeCPULight:
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: deterministicWorkflowImage, Command: command, ReqCPU: 1.0, ReqMemory: 1.0, ReqStorage: 1.0, TaskType: scheduler.TaskTypeCPULight, SLAMultiplier: 2.0, TauSeconds: 8}
	case scheduler.TaskTypeCPUHeavy:
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: deterministicWorkflowImage, Command: command, ReqCPU: 4.0, ReqMemory: 4.0, ReqStorage: 2.0, TaskType: scheduler.TaskTypeCPUHeavy, SLAMultiplier: 2.2, TauSeconds: 24}
	case scheduler.TaskTypeMemoryHeavy:
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: deterministicWorkflowImage, Command: command, ReqCPU: 2.0, ReqMemory: 12.0, ReqStorage: 3.0, TaskType: scheduler.TaskTypeMemoryHeavy, SLAMultiplier: 2.2, TauSeconds: 28}
	case scheduler.TaskTypeMixed:
		return WorkloadTask{TaskID: taskID, TaskName: taskType, ArrivalOffset: arrival, DockerImage: deterministicWorkflowImage, Command: command, ReqCPU: 3.0, ReqMemory: 6.0, ReqStorage: 2.0, TaskType: scheduler.TaskTypeMixed, SLAMultiplier: 2.1, TauSeconds: 20}
	default:
		return WorkloadTask{TaskID: taskID, TaskName: scheduler.TaskTypeMixed, ArrivalOffset: arrival, DockerImage: deterministicWorkflowImage, Command: deterministicWorkflowCommand(scheduler.TaskTypeMixed, variant), ReqCPU: 2.0, ReqMemory: 4.0, ReqStorage: 2.0, TaskType: scheduler.TaskTypeMixed, SLAMultiplier: 2.0, TauSeconds: 20}
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
	case scheduler.TaskTypeMixed:
		return 20
	default:
		return 20
	}
}
