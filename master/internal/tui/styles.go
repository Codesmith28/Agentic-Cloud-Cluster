package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7C3AED") // purple
	colorSecondary = lipgloss.Color("#06B6D4") // cyan
	colorSuccess   = lipgloss.Color("#10B981") // green
	colorWarning   = lipgloss.Color("#F59E0B") // amber
	colorError     = lipgloss.Color("#EF4444") // red
	colorMuted     = lipgloss.Color("#6B7280") // gray
	colorBg        = lipgloss.Color("#1E1E2E") // dark bg
	colorFg        = lipgloss.Color("#CDD6F4") // light fg
	colorBorder    = lipgloss.Color("#45475A") // subtle border

	// Panel styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	// Tab bar
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorMuted)

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorFg).
			Bold(true).
			Underline(true)

	// Status rail (top bar)
	statusRailStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(colorFg)

	// Command bar
	commandBarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(colorBorder).
			Padding(0, 1)

	// Labels
	labelStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorFg).
			Bold(true)

	// Status indicators
	activeStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	inactiveStyle = lipgloss.NewStyle().
			Foreground(colorError)

	// Help text
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	// Spinner / progress
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// Transcript / log area
	transcriptStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)
