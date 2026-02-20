package setup

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/sammwy/teragen/internal/ui/theme"
)

// ── Base styles ───────────────────────────────────────────────────────────────
var (
	// Outer box
	boxStyle = theme.BoxStyle.Copy().Padding(1, 4)

	// Title  ── big centred header
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(theme.ColorCyan)).
			MarginBottom(1)

	// Subtitle line (e.g. "Step 1 of N – …")
	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorPurple)).
			Italic(true).
			MarginBottom(1)

	// Step label
	stepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorMagenta)).
			Bold(true).
			MarginBottom(1)

	// Normal label inside a form row
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorDim)).
			Width(20)

	// Selected / active label
	labelActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorCyan)).
				Bold(true).
				Width(20)

	// Normal list item
	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorFg)).
			PaddingLeft(2)

	// Selected list item
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorCyan)).
				Bold(true).
				PaddingLeft(0)

	// Cursor indicator
	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorRose)).
			Bold(true)

	// Dimmed / hint text
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorDim))

	// Error text
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorRed))

	// Success text
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorGreen))

	// Tip text
	tipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorDim)).
			Italic(true).
			MarginTop(1)

	// Modal box
	modalStyle = theme.BoxStyle.Copy().Padding(1, 2).Background(lipgloss.Color(theme.ColorBg))

	// Modal title
	modalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(theme.ColorRose)).
			Align(lipgloss.Center).
			MarginBottom(1)

	// Modal instructions
	modalInstructionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorDim)).
				Italic(true).
				Align(lipgloss.Center).
				MarginBottom(1)

	// Filled input value
	inputValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorFg))

	// Key hint bar
	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorDim)).
			MarginTop(1)

	// Accent style
	accentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorAccent))
)
