package setup

import "github.com/charmbracelet/lipgloss"

// ── Palette ──────────────────────────────────────────────────────────────────
const (
	colorCyan     = "#00F5FF"
	colorRose     = "#FF6EC7"
	colorPurple   = "#BD93F9"
	colorMagenta  = "#FF79C6"
	colorDim      = "#6272A4"
	colorFg       = "#F8F8F2"
	colorBg       = "#0D0D1A"
	colorSelected = "#00F5FF"
	colorBorder   = "#44475A"
)

// ── Base styles ───────────────────────────────────────────────────────────────
var (
	// Outer box
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder)).
			Padding(1, 3)

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
			Bold(true)

	// Normal label inside a form row
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim)).
			Width(16)

	// Selected / active label
	labelActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorCyan)).
				Bold(true).
				Width(16)

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
			Foreground(lipgloss.Color("#FF5555"))

	// Success text
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B"))

	// Tip text
	tipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim)).
			Italic(true)

	// Modal box
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorRose)).
			Padding(1, 2).
			Background(lipgloss.Color(colorBg))

	// Modal title
	modalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorRose)).
			MarginBottom(1)

	// Filled input value
	inputValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg))

	// Key hint bar
	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim)).
			MarginTop(1)
)
