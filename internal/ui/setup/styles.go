package setup

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/sammwy/teragen/internal/ui/theme"
)

// ── Palette ──────────────────────────────────────────────────────────────────
const (
	colorCyan     = theme.ColorCyan
	colorRose     = theme.ColorRose
	colorPurple   = theme.ColorPurple
	colorMagenta  = theme.ColorMagenta
	colorDim      = theme.ColorDim
	colorFg       = theme.ColorFg
	colorBg       = theme.ColorBg
	colorSelected = theme.ColorCyan
	colorAccent   = theme.ColorAccent
)

// ── Base styles ───────────────────────────────────────────────────────────────
var (
	// Outer box
	boxStyle = theme.BoxStyle.Copy().Padding(1, 4)

	// Title  ── big centred header
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorCyan)).
			MarginBottom(1)

	// Subtitle line (e.g. "Step 1 of N – …")
	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPurple)).
			Italic(true).
			MarginBottom(1)

	// Step label
	stepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMagenta)).
			Bold(true).
			MarginBottom(1)

	// Normal label inside a form row
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim)).
			Width(20)

	// Selected / active label
	labelActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorCyan)).
				Bold(true).
				Width(20)

	// Normal list item
	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg)).
			PaddingLeft(2)

	// Selected list item
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorSelected)).
				Bold(true).
				PaddingLeft(0)

	// Cursor indicator
	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorRose)).
			Bold(true)

	// Dimmed / hint text
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim))

	// Error text
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorRed))

	// Success text
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorGreen))

	// Tip text
	tipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim)).
			Italic(true).
			MarginTop(1)

	// Modal box
	modalStyle = theme.BoxStyle.Copy().Padding(1, 2).Background(lipgloss.Color(colorBg))

	// Modal title
	modalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorRose)).
			Align(lipgloss.Center).
			MarginBottom(1)

	// Modal instructions
	modalInstructionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorDim)).
				Italic(true).
				Align(lipgloss.Center).
				MarginBottom(1)

	// Filled input value
	inputValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg))

	// Key hint bar
	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim)).
			MarginTop(1)

	// Accent style
	accentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent))
)
