package cmd

import "charm.land/lipgloss/v2"

// Styles shared by the tasks CLI output and the tasks TUI. Both are lipgloss v2;
// they used to straddle v1 and v2, which meant two colour-profile detectors.
var (
	tasksAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	tasksOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	tasksWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	tasksErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	tasksMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	tasksDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	tasksBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).Padding(0, 1)
	tasksHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
)
