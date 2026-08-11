// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
