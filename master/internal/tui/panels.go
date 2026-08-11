package tui

import (
	"fmt"
	"strings"
	"time"

	"master/internal/controlplane"

	"github.com/charmbracelet/lipgloss"
)

// renderBar draws a labelled progress bar with percentage and absolute values.
func renderBar(label string, used, total float64, width int) string {
	pct := 0.0
	if total > 0 {
		pct = used / total
	}
	filled := int(float64(width) * pct)
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("%-8s [%s] %5.1f%%  %.1f/%.1f", label, bar, pct*100, used, total)
}

// truncate shortens s to max characters, appending "…" if truncated.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

// statusEmoji returns a coloured emoji for a task status string.
func statusEmoji(status string) string {
	switch strings.ToLower(status) {
	case "running":
		return "▶️"
	case "pending":
		return "⏳"
	case "completed", "done", "success":
		return "✅"
	case "failed", "error":
		return "❌"
	default:
		return "•"
	}
}

// ── Overview ────────────────────────────────────────────────────────────────

func (m Model) renderOverview() string {
	d := m.dashboard

	// Cluster stats section
	clusterLines := []string{
		labelStyle.Render("Cluster Status"),
		"",
		fmt.Sprintf("  %s %s    %s %s    %s %s",
			labelStyle.Render("Workers:"),
			valueStyle.Render(fmt.Sprintf("%d/%d active", d.ActiveWorkers, d.TotalWorkers)),
			labelStyle.Render("Running:"),
			valueStyle.Render(itoa(d.RunningTasks)),
			labelStyle.Render("Queued:"),
			valueStyle.Render(itoa(d.QueuedTasks)),
		),
	}
	if d.SchedulerName != "" {
		clusterLines = append(clusterLines,
			fmt.Sprintf("  %s %s", labelStyle.Render("Scheduler:"), valueStyle.Render(d.SchedulerName)),
		)
	}

	// Resource bars
	barWidth := 20
	resourceLines := []string{
		"",
		labelStyle.Render("Cluster Resources"),
		"",
		"  " + renderBar("CPU", d.ClusterCPUAllocated, d.ClusterCPUTotal, barWidth),
		"  " + renderBar("Memory", d.ClusterMemAllocated, d.ClusterMemTotal, barWidth),
		"  " + renderBar("Storage", d.ClusterStorAllocated, d.ClusterStorTotal, barWidth),
	}

	// Host info
	h := d.HostResources
	hostLines := []string{
		"",
		labelStyle.Render("Master Host"),
		"",
		fmt.Sprintf("  %s %s", labelStyle.Render("Hostname:"), valueStyle.Render(h.Hostname)),
		fmt.Sprintf("  %s %s", labelStyle.Render("CPUs:"), valueStyle.Render(itoa(h.NumCPU))),
		fmt.Sprintf("  %s %s", labelStyle.Render("Memory:"), valueStyle.Render(fmt.Sprintf("%.1f/%.1f GB (%.0f%%)", h.MemUsedGB, h.MemTotalGB, h.MemPercent))),
		fmt.Sprintf("  %s %s", labelStyle.Render("Storage:"), valueStyle.Render(fmt.Sprintf("%.1f/%.1f GB (%.0f%%)", h.StorUsedGB, h.StorTotalGB, h.StorPercent))),
	}

	// Timestamp
	ts := ""
	if !d.Timestamp.IsZero() {
		ts = "\n" + helpStyle.Render("Last updated: "+d.Timestamp.Format(time.Kitchen))
	}

	leftCol := strings.Join(clusterLines, "\n") + "\n" + strings.Join(resourceLines, "\n")
	rightCol := strings.Join(hostLines, "\n")

	var body string
	if m.width > 80 {
		left := lipgloss.NewStyle().Width(m.width/2 - 2).Render(leftCol)
		right := lipgloss.NewStyle().Width(m.width/2 - 2).Render(rightCol)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	} else {
		body = leftCol + "\n" + rightCol
	}

	return panelStyle.Width(m.width - 4).Render(body + ts)
}

// ── Workers ─────────────────────────────────────────────────────────────────

func (m Model) renderWorkers() string {
	if len(m.workers) == 0 {
		return panelStyle.Width(m.width - 4).Render(
			helpStyle.Render("No workers registered"),
		)
	}

	// Header
	header := fmt.Sprintf("  %-14s %-8s %-16s %-18s %-18s %-18s %s",
		"ID", "Status", "IP", "CPU", "Memory", "Storage", "Tasks",
	)
	lines := []string{labelStyle.Render(header)}
	lines = append(lines, labelStyle.Render(strings.Repeat("─", min(m.width-6, 120))))

	for _, w := range m.workers {
		status := activeStyle.Render("🟢")
		if !w.IsActive {
			status = inactiveStyle.Render("🔴")
		}

		cpu := fmt.Sprintf("%.1f/%.1f", w.AllocatedCPU, w.TotalCPU)
		mem := fmt.Sprintf("%.1f/%.1f GB", w.AllocatedMem, w.TotalMem)
		stor := fmt.Sprintf("%.1f/%.1f GB", w.AllocatedStor, w.TotalStor)

		line := fmt.Sprintf("  %-14s %s  %-16s %-18s %-18s %-18s %d",
			truncate(w.WorkerID, 14),
			status,
			truncate(w.WorkerIP, 16),
			cpu, mem, stor,
			w.TaskCount,
		)
		lines = append(lines, line)
	}

	return panelStyle.Width(m.width - 4).Render(strings.Join(lines, "\n"))
}

// ── Tasks ───────────────────────────────────────────────────────────────────

func (m Model) renderTasks() string {
	if len(m.tasks) == 0 {
		return panelStyle.Width(m.width - 4).Render(
			helpStyle.Render("No tasks in the system"),
		)
	}

	// Group tasks by status priority: running > pending > completed > failed > other
	order := []string{"running", "pending", "completed", "done", "success", "failed", "error"}
	groups := make(map[string][]controlplane.TaskRow)
	for _, t := range m.tasks {
		key := strings.ToLower(t.Status)
		groups[key] = append(groups[key], t)
	}

	var lines []string

	rendered := make(map[string]bool)
	for _, key := range order {
		tasks, ok := groups[key]
		if !ok || rendered[key] {
			continue
		}
		rendered[key] = true

		sectionHeader := valueStyle.Render(fmt.Sprintf("  %s %s (%d)", statusEmoji(key), strings.Title(key), len(tasks)))
		lines = append(lines, "", sectionHeader)

		for _, t := range tasks {
			img := truncate(t.DockerImage, 28)
			worker := t.WorkerID
			if worker == "" {
				worker = "—"
			}
			line := fmt.Sprintf("    %-12s %-28s %-12s  cpu:%.1f mem:%.1f stor:%.1f",
				truncate(t.TaskID, 12),
				img,
				truncate(worker, 12),
				t.ReqCPU, t.ReqMem, t.ReqStor,
			)
			lines = append(lines, line)
		}
	}

	// Render any remaining statuses not in the priority order
	for key, tasks := range groups {
		if rendered[key] {
			continue
		}
		sectionHeader := valueStyle.Render(fmt.Sprintf("  %s %s (%d)", statusEmoji(key), key, len(tasks)))
		lines = append(lines, "", sectionHeader)
		for _, t := range tasks {
			img := truncate(t.DockerImage, 28)
			worker := t.WorkerID
			if worker == "" {
				worker = "—"
			}
			line := fmt.Sprintf("    %-12s %-28s %-12s  cpu:%.1f mem:%.1f stor:%.1f",
				truncate(t.TaskID, 12),
				img,
				truncate(worker, 12),
				t.ReqCPU, t.ReqMem, t.ReqStor,
			)
			lines = append(lines, line)
		}
	}

	return panelStyle.Width(m.width - 4).Render(strings.Join(lines, "\n"))
}

// ── Queue ───────────────────────────────────────────────────────────────────

func (m Model) renderQueue() string {
	if len(m.queue) == 0 {
		return panelStyle.Width(m.width - 4).Render(
			helpStyle.Render("Queue is empty"),
		)
	}

	header := fmt.Sprintf("  %-12s %-24s %-10s %-14s %-8s %s",
		"Task", "Image", "User", "Waiting", "Retries", "Target",
	)
	lines := []string{labelStyle.Render(header)}
	lines = append(lines, labelStyle.Render(strings.Repeat("─", min(m.width-6, 100))))

	for _, q := range m.queue {
		waiting := q.TimeInQueue.Truncate(time.Second).String()
		target := q.TargetWorker
		if target == "" {
			target = "any"
		}
		line := fmt.Sprintf("  %-12s %-24s %-10s %-14s %-8d %s",
			truncate(q.TaskID, 12),
			truncate(q.DockerImage, 24),
			truncate(q.UserID, 10),
			waiting,
			q.Retries,
			truncate(target, 14),
		)
		lines = append(lines, line)

		if q.LastError != "" {
			errLine := "    " + lipgloss.NewStyle().Foreground(colorError).Render("err: "+truncate(q.LastError, 60))
			lines = append(lines, errLine)
		}
	}

	return panelStyle.Width(m.width - 4).Render(strings.Join(lines, "\n"))
}

// ── Logs ────────────────────────────────────────────────────────────────────

func (m Model) renderLogs() string {
	if len(m.transcript) == 0 {
		return panelStyle.Width(m.width - 4).Render(
			helpStyle.Render("No command output yet. Press / to enter a command."),
		)
	}

	// Show as many lines as fit, scrolled to bottom
	maxLines := m.height - 12
	if maxLines < 5 {
		maxLines = 5
	}

	start := 0
	if len(m.transcript) > maxLines {
		start = len(m.transcript) - maxLines
	}
	visible := m.transcript[start:]

	return panelStyle.Width(m.width - 4).Render(strings.Join(visible, "\n"))
}

// ── Activity ────────────────────────────────────────────────────────────────

func (m Model) renderActivity() string {
	events := m.dashboard.RecentEvents
	if len(events) == 0 {
		return panelStyle.Width(m.width - 4).Render(
			helpStyle.Render("No recent activity"),
		)
	}

	var lines []string
	for _, e := range events {
		ts := e.Timestamp.Format("15:04:05")

		var levelStyle lipgloss.Style
		switch e.Level {
		case "error":
			levelStyle = lipgloss.NewStyle().Foreground(colorError)
		case "warn":
			levelStyle = lipgloss.NewStyle().Foreground(colorWarning)
		default:
			levelStyle = lipgloss.NewStyle().Foreground(colorSuccess)
		}

		category := labelStyle.Render(fmt.Sprintf("[%s]", e.Category))
		line := fmt.Sprintf("  %s %s %s %s",
			helpStyle.Render(ts),
			levelStyle.Render(strings.ToUpper(e.Level)),
			category,
			e.Message,
		)
		lines = append(lines, line)
	}

	return panelStyle.Width(m.width - 4).Render(strings.Join(lines, "\n"))
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
