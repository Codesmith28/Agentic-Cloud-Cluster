package benchmark

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"master/internal/scheduler"
)

// WriteArtifacts writes JSON/CSV/HTML benchmark outputs and returns the output directory.
func WriteArtifacts(suite *SuiteResult, outputBase string) (string, error) {
	if suite == nil {
		return "", fmt.Errorf("suite result is nil")
	}

	timestamp := suite.GeneratedAt.UTC().Format("20060102-150405")
	outputDir := filepath.Join(outputBase, fmt.Sprintf("run-%s-%s", suite.RequestedProfile, timestamp))
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("create benchmark output dir: %w", err)
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
	if err := writeReportMarkdown(suite, filepath.Join(outputDir, "README.md")); err != nil {
		return "", err
	}
	if err := writeReportHTML(suite, filepath.Join(outputDir, "report.html")); err != nil {
		return "", err
	}

	return outputDir, nil
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
		"worker_balance_score", "avg_decision_ms", "p95_decision_ms",
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
