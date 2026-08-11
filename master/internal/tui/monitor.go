package tui

import (
	"context"
	"fmt"

	"master/internal/controlplane"

	tea "github.com/charmbracelet/bubbletea"
)

// logLineMsg carries a single log line from the monitor stream.
type logLineMsg struct {
	line string
}

// monitorDoneMsg signals that monitoring has completed.
type monitorDoneMsg struct {
	taskID string
	status string
	err    error
}

// monitorEvent is sent over the channel from the streaming goroutine.
// If done is true, the stream has ended.
type monitorEvent struct {
	line   string
	done   bool
	status string
	err    error
}

// startMonitor launches a background goroutine that streams task logs into ch,
// then returns a Cmd that reads the first event from the channel.
func startMonitor(exec *controlplane.Executor, taskID string, ctx context.Context, ch chan monitorEvent) tea.Cmd {
	go func() {
		if exec == nil {
			ch <- monitorEvent{done: true, err: fmt.Errorf("executor not available")}
			return
		}
		var lastStatus string
		err := exec.StreamTaskLogs(ctx, taskID, func(line string, complete bool, status string) error {
			lastStatus = status
			if line != "" {
				ch <- monitorEvent{line: line}
			}
			return ctx.Err()
		})
		ch <- monitorEvent{done: true, status: lastStatus, err: err}
	}()

	return waitForMonitorEvent(taskID, ch)
}

// waitForMonitorEvent returns a Cmd that blocks on the channel and converts
// the next event into either a logLineMsg or monitorDoneMsg.
func waitForMonitorEvent(taskID string, ch chan monitorEvent) tea.Cmd {
	return func() tea.Msg {
		ev := <-ch
		if ev.done {
			return monitorDoneMsg{taskID: taskID, status: ev.status, err: ev.err}
		}
		return logLineMsg{line: ev.line}
	}
}
