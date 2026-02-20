package theme

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	ColorCyan    = "#00F5FF"
	ColorRose    = "#FF79C6"
	ColorPurple  = "#BD93F9"
	ColorMagenta = "#FF79C6"
	ColorDim     = "#6272A4"
	ColorFg      = "#F8F8F2"
	ColorBg      = "#0D0D1A"
	ColorAccent  = "#FFB86C"
	ColorBlack   = "#282A36"
	ColorGreen   = "#50FA7B"
	ColorYellow  = "#F1FA8C"
	ColorRed     = "#FF5555"
)

var (
	Purple = lipgloss.Color(ColorPurple)
	Rose   = lipgloss.Color(ColorRose)

	GradientBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}

	// Base style for any boxed component
	BoxStyle = lipgloss.NewStyle().
			Border(GradientBorder).
			BorderTopForeground(Purple).
			BorderBottomForeground(Rose).
			BorderLeftForeground(Purple).
			BorderRightForeground(Purple)
)

// GetGradientColor returns a color between Purple and Rose based on the percentage (0.0 to 1.0).
func GetGradientColor(percent float64) lipgloss.Color {
	c1, _ := colorful.Hex(ColorPurple)
	c2, _ := colorful.Hex(ColorRose)
	return lipgloss.Color(c1.BlendLuv(c2, percent).Hex())
}

// GetStyledGradientBorder returns a Border where each side character is already colored.
func GetStyledGradientBorder(height int) lipgloss.Border {
	b := GradientBorder
	b.Top = lipgloss.NewStyle().Foreground(Purple).Render(b.Top)
	b.Bottom = lipgloss.NewStyle().Foreground(Rose).Render(b.Bottom)
	b.TopLeft = lipgloss.NewStyle().Foreground(Purple).Render(b.TopLeft)
	b.TopRight = lipgloss.NewStyle().Foreground(Purple).Render(b.TopRight)
	b.BottomLeft = lipgloss.NewStyle().Foreground(Rose).Render(b.BottomLeft)
	b.BottomRight = lipgloss.NewStyle().Foreground(Rose).Render(b.BottomRight)
	// Left and right are done line-by-line in the caller using GetGradientColor
	return b
}
