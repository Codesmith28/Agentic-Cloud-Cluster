package controlplane

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"master/internal/benchmark"
	pb "master/proto"
)

func (e *Executor) cmdBenchmark(parts []string) CommandOutcome {
	profile := benchmark.ProfileAll
	seed := time.Now().Unix()
	outputBase := filepath.Join("..", "results", "benchmarks")

	if len(parts) >= 2 {
		if parts[1] == "list" {
			var b strings.Builder
			b.WriteString("Available benchmark profiles:\n")
			for _, name := range benchmark.AvailableProfiles() {
				b.WriteString(fmt.Sprintf("  - %s\n", name))
			}
			b.WriteString("  - all\n")
			return CommandOutcome{Transcript: b.String()}
		}
		profile = parts[1]
	}

	for i := 2; i < len(parts); i++ {
		switch parts[i] {
		case "-seed":
			if i+1 < len(parts) {
				if parsed, err := strconv.ParseInt(parts[i+1], 10, 64); err == nil {
					seed = parsed
					i++
				}
			}
		case "-out":
			if i+1 < len(parts) {
				outputBase = parts[i+1]
				i++
			}
		}
	}

	var b strings.Builder
	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("  📊 SCHEDULER BENCHMARK SUITE\n")
	b.WriteString("═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  Profile: %s\n", profile))
	b.WriteString(fmt.Sprintf("  Seed:    %d\n", seed))
	b.WriteString(fmt.Sprintf("  Output:  %s\n", outputBase))
	b.WriteString("───────────────────────────────────────────────────────\n")

	suite, err := benchmark.RunSuite(profile, seed)
	if err != nil {
		b.WriteString(fmt.Sprintf("❌ Benchmark failed: %v\n", err))
		return CommandOutcome{Transcript: b.String(), Err: err}
	}

	outputDir, err := benchmark.WriteArtifacts(suite, outputBase)
	if err != nil {
		b.WriteString(fmt.Sprintf("❌ Failed to write benchmark artifacts: %v\n", err))
		return CommandOutcome{Transcript: b.String(), Err: err}
	}

	sortedResults := make([]benchmark.SchedulerResult, 0)
	for _, profileResult := range suite.Profiles {
		b.WriteString("\n-------------------------------------------------------\n")
		b.WriteString(fmt.Sprintf("Profile: %s\n", profileResult.Profile))
		b.WriteString(profileResult.Description + "\n")
		b.WriteString(fmt.Sprintf("Winner:  %s\n", profileResult.Winner))
		b.WriteString("-------------------------------------------------------\n")
		b.WriteString("Scheduler      SLA%    P95 Wait(s)  Throughput/min  Makespan(s)  CPU Util%  Balance\n")

		sortedResults = sortedResults[:0]
		sortedResults = append(sortedResults, profileResult.SchedulerResults...)
		sort.Slice(sortedResults, func(i, j int) bool {
			return sortedResults[i].SchedulerName < sortedResults[j].SchedulerName
		})
		for _, result := range sortedResults {
			m := result.Metrics
			b.WriteString(fmt.Sprintf("%-13s %-7.2f %-12.2f %-15.2f %-12.2f %-9.2f %.3f\n",
				result.SchedulerName,
				m.SLASuccessRatePct,
				m.P95QueueWaitSec,
				m.ThroughputTasksPerMin,
				m.MakespanSec,
				m.CPUUtilizationPct,
				m.WorkerBalanceScore,
			))
		}
	}

	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("✅ Benchmark complete\n")
	b.WriteString(fmt.Sprintf("   Report folder: %s\n", outputDir))
	b.WriteString(fmt.Sprintf("   HTML report:   %s\n", filepath.Join(outputDir, "report.html")))
	b.WriteString("═══════════════════════════════════════════════════════\n")
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdWorkloadSubmit(parts []string) CommandOutcome {
	if len(parts) < 2 {
		return CommandOutcome{
			Transcript: "Usage: workload-submit <profile> [-speed <factor>] [-limit <n>] [-dry-run]\nExample: workload-submit showcase -speed 10 -limit 30\nUse 'benchmark list' to view profiles",
		}
	}

	profileName := parts[1]
	speedFactor := 10.0
	limit := -1
	dryRun := false

	for i := 2; i < len(parts); i++ {
		switch parts[i] {
		case "-speed":
			if i+1 < len(parts) {
				if parsed, err := strconv.ParseFloat(parts[i+1], 64); err == nil && parsed > 0 {
					speedFactor = parsed
					i++
				}
			}
		case "-limit":
			if i+1 < len(parts) {
				if parsed, err := strconv.Atoi(parts[i+1]); err == nil && parsed > 0 {
					limit = parsed
					i++
				}
			}
		case "-dry-run":
			dryRun = true
		}
	}

	profile, err := benchmark.GetWorkloadProfile(profileName)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ %v", err),
			Err:        err,
		}
	}

	tasks := append([]benchmark.WorkloadTask(nil), profile.Tasks...)
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ArrivalOffset < tasks[j].ArrivalOffset })
	if limit > 0 && limit < len(tasks) {
		tasks = tasks[:limit]
	}

	var b strings.Builder
	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	b.WriteString("  🧪 PREDEFINED WORKLOAD SUBMISSION\n")
	b.WriteString("═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  Profile:       %s\n", profile.Name))
	b.WriteString(fmt.Sprintf("  Description:   %s\n", profile.Description))
	b.WriteString(fmt.Sprintf("  Tasks:         %d\n", len(tasks)))
	b.WriteString(fmt.Sprintf("  Speed factor:  %.2fx\n", speedFactor))
	mode := "submit"
	if dryRun {
		mode = "dry-run"
	}
	b.WriteString(fmt.Sprintf("  Mode:          %s\n", mode))
	b.WriteString("───────────────────────────────────────────────────────\n")

	previousOffset := time.Duration(0)
	successCount := 0
	failureCount := 0

	for idx, wt := range tasks {
		delta := wt.ArrivalOffset - previousOffset
		if delta < 0 {
			delta = 0
		}
		previousOffset = wt.ArrivalOffset

		sleepFor := time.Duration(float64(delta) / speedFactor)
		if sleepFor > 0 {
			time.Sleep(sleepFor)
		}

		taskID := fmt.Sprintf("wl-%s-%03d-%d", profile.Name, idx, time.Now().UnixNano())
		taskName := wt.TaskName
		if taskName == "" {
			taskName = strings.ReplaceAll(wt.TaskType, "-", "_")
		}

		task := &pb.Task{
			TaskId:        taskID,
			TaskName:      taskName,
			DockerImage:   wt.DockerImage,
			Command:       wt.Command,
			ReqCpu:        wt.ReqCPU,
			ReqMemory:     wt.ReqMemory,
			ReqStorage:    wt.ReqStorage,
			TaskType:      wt.TaskType,
			SlaMultiplier: wt.SLAMultiplier,
			UserId:        "benchmark",
			SubmittedAt:   time.Now().Unix(),
		}

		if dryRun {
			b.WriteString(fmt.Sprintf("[%03d/%03d] %s type=%s cpu=%.1f mem=%.1f storage=%.1f offset=%s\n",
				idx+1, len(tasks), taskID, wt.TaskType, wt.ReqCPU, wt.ReqMemory, wt.ReqStorage, wt.ArrivalOffset))
			successCount++
			continue
		}

		ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		ack, submitErr := e.srv.SubmitTask(ctx, task)
		ctxCancel()
		if submitErr != nil || (ack != nil && !ack.Success) {
			failureCount++
			if submitErr != nil {
				b.WriteString(fmt.Sprintf("[%03d/%03d] ❌ %s submit error: %v\n", idx+1, len(tasks), taskID, submitErr))
			} else {
				b.WriteString(fmt.Sprintf("[%03d/%03d] ❌ %s rejected: %s\n", idx+1, len(tasks), taskID, ack.Message))
			}
			continue
		}

		successCount++
		b.WriteString(fmt.Sprintf("[%03d/%03d] ✅ %s queued\n", idx+1, len(tasks), taskID))
	}

	b.WriteString("\n═══════════════════════════════════════════════════════\n")
	if dryRun {
		b.WriteString(fmt.Sprintf("✅ Dry-run complete (%d task events generated)\n", successCount))
	} else {
		b.WriteString(fmt.Sprintf("✅ Workload submission complete: %d queued, %d failed\n", successCount, failureCount))
		b.WriteString("   Use 'queue' and 'list-tasks running' to monitor execution\n")
	}
	b.WriteString("═══════════════════════════════════════════════════════\n")
	return CommandOutcome{
		Transcript: b.String(),
		Effects:    []UIEffect{{Type: EffectRefresh}},
	}
}
