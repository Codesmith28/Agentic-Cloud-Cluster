package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"master/internal/controlplane"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FocusArea tracks which area of the UI has focus.
type FocusArea int

const (
	FocusPane FocusArea = iota
	FocusCommandBar
)

// tickMsg triggers periodic data refresh.
type tickMsg time.Time

// commandResultMsg carries the result of an async command execution.
type commandResultMsg struct {
	outcome controlplane.CommandOutcome
}

// Model is the top-level Bubble Tea model for the TUI.
type Model struct {
	// Layout
	width  int
	height int

	// Pane state
	activePane controlplane.Pane
	focus      FocusArea

	// Command bar
	cmdInput   textinput.Model
	transcript []string // command history/output

	// Data (populated by executor)
	dashboard controlplane.DashboardSnapshot
	workers   []controlplane.WorkerRow
	tasks     []controlplane.TaskRow
	queue     []controlplane.QueueRow

	// Sub-components
	spinner spinner.Model

	// Executor reference (set after creation)
	executor *controlplane.Executor

	// State flags
	loading  bool
	quitting bool

	// Monitor state
	monitorTaskID string
	monitorCancel context.CancelFunc
	monitorCh     chan monitorEvent
	monitoring    bool
}

// New creates a new TUI model.
func New(exec *controlplane.Executor) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a command... (help for list)"
	ti.CharLimit = 256

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	return Model{
		activePane: controlplane.PaneOverview,
		focus:      FocusPane,
		cmdInput:   ti,
		transcript: []string{},
		spinner:    sp,
		executor:   exec,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.refreshData(),
		tickEvery(2*time.Second),
	)
}

// tickEvery returns a Cmd that sends a tickMsg after the duration.
func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		cmds = append(cmds, m.refreshData(), tickEvery(2*time.Second))

	case commandResultMsg:
		m.loading = false
		if msg.outcome.Transcript != "" {
			lines := strings.Split(msg.outcome.Transcript, "\n")
			m.transcript = append(m.transcript, lines...)
			// Keep last 100 lines
			if len(m.transcript) > 100 {
				m.transcript = m.transcript[len(m.transcript)-100:]
			}
		}
		if msg.outcome.Err != nil {
			m.transcript = append(m.transcript, "Error: "+msg.outcome.Err.Error())
		}
		// Handle effects
		for _, effect := range msg.outcome.Effects {
			switch effect.Type {
			case controlplane.EffectRefresh:
				cmds = append(cmds, m.refreshData())
			case controlplane.EffectSwitchPane:
				m.activePane = paneFromString(effect.Payload)
			case controlplane.EffectFocusWorker:
				m.activePane = controlplane.PaneWorkers
			case controlplane.EffectFocusTask:
				m.activePane = controlplane.PaneTasks
			case controlplane.EffectOpenMonitor:
				m.monitorTaskID = effect.Payload
				m.monitoring = true
				m.activePane = controlplane.PaneLogs
				ctx, cancel := context.WithCancel(context.Background())
				m.monitorCancel = cancel
				m.monitorCh = make(chan monitorEvent, 64)
				m.transcript = append(m.transcript, fmt.Sprintf("📡 Monitoring task %s...", effect.Payload))
				cmds = append(cmds, startMonitor(m.executor, effect.Payload, ctx, m.monitorCh))
			}
		}

	case logLineMsg:
		m.transcript = append(m.transcript, msg.line)
		if len(m.transcript) > 100 {
			m.transcript = m.transcript[len(m.transcript)-100:]
		}
		if m.monitoring && m.monitorCh != nil {
			cmds = append(cmds, waitForMonitorEvent(m.monitorTaskID, m.monitorCh))
		}

	case monitorDoneMsg:
		m.monitoring = false
		m.monitorCancel = nil
		m.monitorCh = nil
		if msg.err != nil && msg.err != context.Canceled {
			m.transcript = append(m.transcript, fmt.Sprintf("❌ Monitor error: %v", msg.err))
		} else {
			m.transcript = append(m.transcript, fmt.Sprintf("✅ Task %s completed: %s", msg.taskID, msg.status))
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case dashboardMsg:
		m.dashboard = msg.snapshot
		m.workers = msg.workers
		m.tasks = msg.tasks
		m.queue = msg.queue
	}

	// Update text input if focused
	if m.focus == FocusCommandBar {
		var cmd tea.Cmd
		m.cmdInput, cmd = m.cmdInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKey processes keyboard input.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys always work
	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	// Command bar focused
	if m.focus == FocusCommandBar {
		switch msg.String() {
		case "esc":
			if m.monitoring && m.monitorCancel != nil {
				m.monitorCancel()
				m.monitoring = false
				m.transcript = append(m.transcript, "⏹ Monitoring stopped")
			}
			m.focus = FocusPane
			m.cmdInput.Blur()
			return m, nil
		case "enter":
			input := strings.TrimSpace(m.cmdInput.Value())
			m.cmdInput.SetValue("")
			if input == "" {
				return m, nil
			}
			if input == "exit" || input == "quit" {
				m.quitting = true
				return m, tea.Quit
			}
			m.transcript = append(m.transcript, "❯ "+input)
			m.loading = true
			return m, m.executeCommand(input)
		default:
			var cmd tea.Cmd
			m.cmdInput, cmd = m.cmdInput.Update(msg)
			return m, cmd
		}
	}

	// Pane navigation
	switch msg.String() {
	case "esc":
		if m.monitoring && m.monitorCancel != nil {
			m.monitorCancel()
			m.monitoring = false
			m.transcript = append(m.transcript, "⏹ Monitoring stopped")
		}
		return m, nil
	case "tab":
		m.activePane = controlplane.Pane((int(m.activePane) + 1) % controlplane.PaneCount)
		return m, nil
	case "shift+tab":
		m.activePane = controlplane.Pane((int(m.activePane) - 1 + controlplane.PaneCount) % controlplane.PaneCount)
		return m, nil
	case "/":
		m.focus = FocusCommandBar
		m.cmdInput.Focus()
		return m, textinput.Blink
	case "r":
		return m, m.refreshData()
	case "q":
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

// dashboardMsg carries refreshed data to the model.
type dashboardMsg struct {
	snapshot controlplane.DashboardSnapshot
	workers  []controlplane.WorkerRow
	tasks    []controlplane.TaskRow
	queue    []controlplane.QueueRow
}

// refreshData fetches fresh data from the executor.
func (m Model) refreshData() tea.Cmd {
	return func() tea.Msg {
		if m.executor == nil {
			return dashboardMsg{}
		}
		tasks, _ := m.executor.GetTaskRows("")
		return dashboardMsg{
			snapshot: m.executor.GetDashboard(),
			workers:  m.executor.GetWorkerRows(),
			tasks:    tasks,
			queue:    m.executor.GetQueueRows(),
		}
	}
}

// executeCommand runs a command through the executor asynchronously.
func (m Model) executeCommand(input string) tea.Cmd {
	return func() tea.Msg {
		if m.executor == nil {
			return commandResultMsg{outcome: controlplane.CommandOutcome{
				Transcript: "Executor not available",
			}}
		}
		outcome := m.executor.ExecuteCommand(input)
		return commandResultMsg{outcome: outcome}
	}
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return "Shutting down master...\n"
	}

	if m.width == 0 {
		return "Initializing..."
	}

	// Layout: status rail (top) + tab bar + main content (middle) + command bar (bottom)
	statusRail := m.renderStatusRail()
	tabBar := m.renderTabBar()
	mainContent := m.renderMainPane()
	cmdBar := m.renderCommandBar()

	// Calculate available height
	statusHeight := lipgloss.Height(statusRail)
	tabHeight := lipgloss.Height(tabBar)
	cmdBarHeight := lipgloss.Height(cmdBar)
	contentHeight := m.height - statusHeight - tabHeight - cmdBarHeight

	if contentHeight < 3 {
		contentHeight = 3
	}

	// Constrain main content
	mainContent = lipgloss.NewStyle().
		Height(contentHeight).
		MaxHeight(contentHeight).
		Width(m.width).
		Render(mainContent)

	return lipgloss.JoinVertical(lipgloss.Left,
		statusRail,
		tabBar,
		mainContent,
		cmdBar,
	)
}

// renderStatusRail renders the top status bar.
func (m Model) renderStatusRail() string {
	left := statusRailStyle.Render("☁ CloudAI Master")

	status := ""
	if m.loading {
		status = m.spinner.View() + " "
	}
	status += labelStyle.Render("Workers: ") + valueStyle.Render(itoa(m.dashboard.ActiveWorkers)+"/"+itoa(m.dashboard.TotalWorkers))
	status += "  " + labelStyle.Render("Tasks: ") + valueStyle.Render(itoa(m.dashboard.RunningTasks))
	status += "  " + labelStyle.Render("Queued: ") + valueStyle.Render(itoa(m.dashboard.QueuedTasks))
	if m.dashboard.SchedulerName != "" {
		status += "  " + labelStyle.Render("Sched: ") + valueStyle.Render(m.dashboard.SchedulerName)
	}

	right := statusRailStyle.Render(status)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return left + strings.Repeat(" ", gap) + right
}

// renderTabBar renders the pane tab bar.
func (m Model) renderTabBar() string {
	var tabs []string
	for i := 0; i < controlplane.PaneCount; i++ {
		p := controlplane.Pane(i)
		if p == m.activePane {
			tabs = append(tabs, activeTabStyle.Render(p.String()))
		} else {
			tabs = append(tabs, tabStyle.Render(p.String()))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// renderMainPane renders the currently active pane content.
func (m Model) renderMainPane() string {
	switch m.activePane {
	case controlplane.PaneOverview:
		return m.renderOverview()
	case controlplane.PaneWorkers:
		return m.renderWorkers()
	case controlplane.PaneTasks:
		return m.renderTasks()
	case controlplane.PaneQueue:
		return m.renderQueue()
	case controlplane.PaneLogs:
		return m.renderLogs()
	case controlplane.PaneActivity:
		return m.renderActivity()
	default:
		return ""
	}
}

// renderCommandBar renders the bottom command input area.
func (m Model) renderCommandBar() string {
	// Show last few transcript lines + input
	transcriptLines := 3
	var recent []string
	if len(m.transcript) > transcriptLines {
		recent = m.transcript[len(m.transcript)-transcriptLines:]
	} else {
		recent = m.transcript
	}

	transcript := transcriptStyle.Render(strings.Join(recent, "\n"))
	prompt := m.cmdInput.View()
	help := helpStyle.Render("  tab:pane  /:cmd  r:refresh  ctrl+c:exit")

	content := lipgloss.JoinVertical(lipgloss.Left, transcript, prompt, help)

	style := commandBarStyle
	if m.focus == FocusCommandBar {
		style = style.BorderForeground(colorPrimary)
	}

	return style.Width(m.width - 2).Render(content)
}

// paneFromString converts a string to a Pane.
func paneFromString(s string) controlplane.Pane {
	switch strings.ToLower(s) {
	case "overview":
		return controlplane.PaneOverview
	case "workers":
		return controlplane.PaneWorkers
	case "tasks":
		return controlplane.PaneTasks
	case "queue":
		return controlplane.PaneQueue
	case "logs":
		return controlplane.PaneLogs
	case "activity":
		return controlplane.PaneActivity
	default:
		return controlplane.PaneOverview
	}
}

// itoa is a quick int-to-string helper.
func itoa(i int) string {
	return strconv.Itoa(i)
}

// NewProgram creates a Bubble Tea program with fullscreen + alt screen.
func NewProgram(m Model) *tea.Program {
	return tea.NewProgram(m, tea.WithAltScreen())
}
