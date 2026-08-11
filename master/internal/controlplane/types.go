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

package controlplane

import "time"

// DashboardSnapshot is the top-level overview data for the TUI dashboard.
// It wraps the cluster snapshot with additional host-level info.
type DashboardSnapshot struct {
	Timestamp          time.Time
	TotalWorkers       int
	ActiveWorkers      int
	InactiveWorkers    int
	RunningTasks       int
	QueuedTasks        int
	SchedulerName      string
	// Cluster resource totals
	ClusterCPUTotal      float64
	ClusterCPUAllocated  float64
	ClusterCPUUtil       float64 // percentage
	ClusterMemTotal      float64
	ClusterMemAllocated  float64
	ClusterMemUtil       float64
	ClusterStorTotal     float64
	ClusterStorAllocated float64
	ClusterStorUtil      float64
	// Master host resources
	HostResources MasterHostResources
	// Recent events
	RecentEvents []RecentEvent
}

// WorkerRow is a single row in the worker panel table.
type WorkerRow struct {
	WorkerID       string
	WorkerIP       string
	IsActive       bool
	LastHeartbeat  string // human-readable "5s ago"
	CPUUsage       float64
	MemUsage       float64
	StorUsage      float64
	TotalCPU       float64
	TotalMem       float64
	TotalStor      float64
	AllocatedCPU   float64
	AllocatedMem   float64
	AllocatedStor  float64
	AvailableCPU   float64
	AvailableMem   float64
	AvailableStor  float64
	TaskCount      int
	RunningTaskIDs []string
}

// TaskRow is a single row in the task panel table.
type TaskRow struct {
	TaskID      string
	TaskName    string
	UserID      string
	DockerImage string
	Status      string
	WorkerID    string // assigned worker, may be empty
	ReqCPU      float64
	ReqMem      float64
	ReqStor     float64
	CreatedAt   time.Time
	SLAMult     float64
	TaskType    string
}

// QueueRow is a single row in the queue panel.
type QueueRow struct {
	TaskID       string
	DockerImage  string
	UserID       string
	ReqCPU       float64
	ReqMem       float64
	ReqStor      float64
	QueuedAt     time.Time
	TimeInQueue  time.Duration
	Retries      int
	LastError    string
	TargetWorker string
}

// MasterHostResources holds live resource info for the master host machine.
type MasterHostResources struct {
	CPUPercent  float64
	MemPercent  float64
	MemUsedGB   float64
	MemTotalGB  float64
	StorUsedGB  float64
	StorTotalGB float64
	StorPercent float64
	NumCPU      int
	Hostname    string
	SampledAt   time.Time
}

// RecentEvent represents a notable cluster event for the activity feed.
type RecentEvent struct {
	Timestamp time.Time
	Category  string // "worker", "task", "scheduler", "system"
	Message   string
	Level     string // "info", "warn", "error"
}

// CommandOutcome is the result of executing a CLI/TUI command through the shared executor.
type CommandOutcome struct {
	// Text transcript for display (what the old CLI would have printed)
	Transcript string
	// Structured effects that the UI can act on
	Effects []UIEffect
	// Error, if the command failed
	Err error
}

// UIEffect is a structured action the UI should take after a command.
type UIEffect struct {
	Type    UIEffectType
	Payload string
}

// UIEffectType enumerates the possible UI side-effects from a command.
type UIEffectType int

const (
	EffectNone UIEffectType = iota
	EffectFocusWorker                // Payload = workerID
	EffectFocusTask                  // Payload = taskID
	EffectOpenMonitor                // Payload = taskID
	EffectToast                      // Payload = message
	EffectRefresh                    // Trigger a data refresh
	EffectSwitchPane                 // Payload = pane name
)

// String returns a human-readable name for the effect type.
func (e UIEffectType) String() string {
	switch e {
	case EffectFocusWorker:
		return "focus_worker"
	case EffectFocusTask:
		return "focus_task"
	case EffectOpenMonitor:
		return "open_monitor"
	case EffectToast:
		return "toast"
	case EffectRefresh:
		return "refresh"
	case EffectSwitchPane:
		return "switch_pane"
	default:
		return "none"
	}
}

// Pane identifies a named panel in the TUI.
type Pane int

const (
	PaneOverview Pane = iota
	PaneWorkers
	PaneTasks
	PaneQueue
	PaneLogs
	PaneActivity
)

// String returns the display name of the pane.
func (p Pane) String() string {
	switch p {
	case PaneOverview:
		return "Overview"
	case PaneWorkers:
		return "Workers"
	case PaneTasks:
		return "Tasks"
	case PaneQueue:
		return "Queue"
	case PaneLogs:
		return "Logs"
	case PaneActivity:
		return "Activity"
	default:
		return "Unknown"
	}
}

// PaneCount is the total number of panes.
const PaneCount = 6
