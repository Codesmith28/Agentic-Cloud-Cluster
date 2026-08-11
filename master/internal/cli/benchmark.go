package cli

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

func (c *CLI) runBenchmark(parts []string) {
	profile := benchmark.ProfileAll
	seed := time.Now().Unix()
	outputBase := filepath.Join("..", "results", "benchmarks")

	if len(parts) >= 2 {
		if parts[1] == "list" {
			fmt.Println("Available benchmark profiles:")
			for _, name := range benchmark.AvailableProfiles() {
				fmt.Printf("  - %s\n", name)
			}
			fmt.Println("  - all")
			return
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

	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  📊 SCHEDULER BENCHMARK SUITE")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  Profile: %s\n", profile)
	fmt.Printf("  Seed:    %d\n", seed)
	fmt.Printf("  Output:  %s\n", outputBase)
	fmt.Println("───────────────────────────────────────────────────────")

	suite, err := benchmark.RunSuite(profile, seed)
	if err != nil {
		fmt.Printf("❌ Benchmark failed: %v\n", err)
		return
	}

	outputDir, err := benchmark.WriteArtifacts(suite, outputBase)
	if err != nil {
		fmt.Printf("❌ Failed to write benchmark artifacts: %v\n", err)
		return
	}

	for _, profileResult := range suite.Profiles {
		fmt.Println("\n-------------------------------------------------------")
		fmt.Printf("Profile: %s\n", profileResult.Profile)
		fmt.Println(profileResult.Description)
		fmt.Printf("Winner:  %s\n", profileResult.Winner)
		fmt.Println("-------------------------------------------------------")
		fmt.Println("Scheduler      SLA%    P95 Wait(s)  Throughput/min  Makespan(s)  CPU Util%  Balance")

		results := append([]benchmark.SchedulerResult(nil), profileResult.SchedulerResults...)
		sort.Slice(results, func(i, j int) bool {
			return results[i].SchedulerName < results[j].SchedulerName
		})
		for _, result := range results {
			m := result.Metrics
			fmt.Printf("%-13s %-7.2f %-12.2f %-15.2f %-12.2f %-9.2f %.3f\n",
				result.SchedulerName,
				m.SLASuccessRatePct,
				m.P95QueueWaitSec,
				m.ThroughputTasksPerMin,
				m.MakespanSec,
				m.CPUUtilizationPct,
				m.WorkerBalanceScore,
			)
		}

		fmt.Printf("\nRTS vs RR improvements: SLA %+0.2f%% | P95 wait %+0.2f%% | Makespan %+0.2f%% | Throughput %+0.2f%%\n",
			profileResult.SLAImprovementPct,
			profileResult.WaitP95ReductionPct,
			profileResult.MakespanReductionPct,
			profileResult.ThroughputGainPct,
		)
	}

	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("✅ Benchmark complete")
	fmt.Printf("   Report folder: %s\n", outputDir)
	fmt.Printf("   HTML report:   %s\n", filepath.Join(outputDir, "report.html"))
	fmt.Printf("   Summary JSON:  %s\n", filepath.Join(outputDir, "summary.json"))
	fmt.Printf("   Metrics CSV:   %s\n", filepath.Join(outputDir, "metrics.csv"))
	fmt.Println("═══════════════════════════════════════════════════════")
}

func (c *CLI) submitWorkload(parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: workload-submit <profile> [-speed <factor>] [-limit <n>] [-dry-run]")
		fmt.Println("Example: workload-submit showcase -speed 10 -limit 30")
		fmt.Println("Use 'benchmark list' to view profiles")
		return
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
		fmt.Printf("❌ %v\n", err)
		return
	}

	tasks := append([]benchmark.WorkloadTask(nil), profile.Tasks...)
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ArrivalOffset < tasks[j].ArrivalOffset })
	if limit > 0 && limit < len(tasks) {
		tasks = tasks[:limit]
	}

	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  🧪 PREDEFINED WORKLOAD SUBMISSION")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  Profile:       %s\n", profile.Name)
	fmt.Printf("  Description:   %s\n", profile.Description)
	fmt.Printf("  Tasks:         %d\n", len(tasks))
	fmt.Printf("  Speed factor:  %.2fx\n", speedFactor)
	fmt.Printf("  Mode:          %s\n", map[bool]string{true: "dry-run", false: "submit"}[dryRun])
	fmt.Println("───────────────────────────────────────────────────────")

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
			ReqGpu:        wt.ReqGPU,
			TaskType:      wt.TaskType,
			SlaMultiplier: wt.SLAMultiplier,
			UserId:        "benchmark",
			SubmittedAt:   time.Now().Unix(),
		}

		if dryRun {
			fmt.Printf("[%03d/%03d] %s type=%s cpu=%.1f mem=%.1f gpu=%.1f offset=%s\n",
				idx+1, len(tasks), taskID, wt.TaskType, wt.ReqCPU, wt.ReqMemory, wt.ReqGPU, wt.ArrivalOffset)
			successCount++
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ack, submitErr := c.masterServer.SubmitTask(ctx, task)
		cancel()
		if submitErr != nil || (ack != nil && !ack.Success) {
			failureCount++
			if submitErr != nil {
				fmt.Printf("[%03d/%03d] ❌ %s submit error: %v\n", idx+1, len(tasks), taskID, submitErr)
			} else {
				fmt.Printf("[%03d/%03d] ❌ %s rejected: %s\n", idx+1, len(tasks), taskID, ack.Message)
			}
			continue
		}

		successCount++
		fmt.Printf("[%03d/%03d] ✅ %s queued\n", idx+1, len(tasks), taskID)
	}

	fmt.Println("\n═══════════════════════════════════════════════════════")
	if dryRun {
		fmt.Printf("✅ Dry-run complete (%d task events generated)\n", successCount)
	} else {
		fmt.Printf("✅ Workload submission complete: %d queued, %d failed\n", successCount, failureCount)
		fmt.Println("   Use 'queue' and 'list-tasks running' to monitor execution")
	}
	fmt.Println("═══════════════════════════════════════════════════════")
}
