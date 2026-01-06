package ui

import "github.com/charmbracelet/lipgloss"

// Color constants for consistent theming
const (
	ColorPurple      = "#7D56F4"
	ColorWhite       = "#FAFAFA"
	ColorGray        = "#666666"
	ColorLightGray   = "#888888"
	ColorDarkGray    = "#555555"
	ColorGreen       = "#04B575"
	ColorOrange      = "#FFA500"
	ColorCyan        = "#00CED1"
	ColorRed         = "#FF5F87"
	ColorYellow      = "#FFD700"
	ColorProjectLabel = "#9CA3AF"
	ColorProjectName  = "#A78BFA"
)

// UI styles for the TUI components
var (
	// Project name styles
	projectLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorProjectLabel))
	projectNameStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorProjectName))
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorWhite)).
			Background(lipgloss.Color(ColorPurple)).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorWhite)).
			Background(lipgloss.Color(ColorPurple))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorGreen))

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorOrange))

	waitingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorCyan))

	idleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorLightGray))

	stoppedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorRed))

	previewStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorPurple)).
			Padding(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorGray))

	sessionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorYellow))

	listPaneStyle = lipgloss.NewStyle().
			BorderRight(true).
			BorderStyle(lipgloss.Border{Right: "│"}).
			BorderForeground(lipgloss.Color(ColorDarkGray))

	previewPaneStyle = lipgloss.NewStyle()

	listSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorWhite)).
				Background(lipgloss.Color(ColorPurple)).
				Bold(true)

	searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorDarkGray)).
			Foreground(lipgloss.Color(ColorGray)).
			Padding(0, 1)

	selectedPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorWhite)).
				Bold(true)

	metaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorLightGray))
)
