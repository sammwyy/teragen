package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	ColorCyan    = "#A1E3F9"
	ColorRose    = "#F9B7FF"
	ColorPurple  = "#C5B4E3"
	ColorMagenta = "#FFBEE3"
	ColorDim     = "#94A3B8"
	ColorFg      = "#F1F5F9"
	ColorBg      = "#0F172A"
	ColorAccent  = "#FED7AA"
	ColorBlack   = "#1E293B"
	ColorGreen   = "#BBF7D0"
	ColorYellow  = "#FEF08A"
	ColorRed     = "#FECACA"

	// New names for consistency with previous edits if they expect them
	ColorVibrantCyan  = "#A1E3F9"
	ColorElectricBlue = "#BAE6FD"
	ColorDeepPurple   = "#DDD6FE"
	ColorVibrantPink  = "#F9B7FF"
	ColorGoldenOrange = "#FED7AA"

	// Role accents
	AccentUser    = ColorGreen
	AccentAgent   = ColorCyan
	AccentCommand = ColorYellow
	AccentError   = ColorRed
	AccentSystem  = ColorPurple
)

var (
	Purple = lipgloss.Color(ColorPurple)
	Rose   = lipgloss.Color(ColorRose)

	// Palettes
	PalettePrimary   = []string{ColorPurple, ColorCyan, ColorMagenta}
	PaletteSecondary = []string{ColorRose, ColorPurple}
	PaletteAccent    = []string{ColorAccent, ColorRose}
	PaletteUser      = []string{"#BFFFC7", "#8FFFFF", "#A1E3F9"}
	PaletteAgent     = []string{"#FF85FF", "#D69DFF", "#B89CFF"}

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

// GetGradientColor returns a color between two hex strings.
func GetGradientColor(start, end string, percent float64) lipgloss.Color {
	c1, _ := colorful.Hex(start)
	c2, _ := colorful.Hex(end)
	return lipgloss.Color(c1.BlendLuv(c2, percent).Hex())
}

// GetMultiGradientColor returns a color from a palette of multiple colors.
func GetMultiGradientColor(palette []string, percent float64) lipgloss.Color {
	if len(palette) == 0 {
		return lipgloss.Color("#FFFFFF")
	}
	if len(palette) == 1 {
		return lipgloss.Color(palette[0])
	}
	if percent >= 1.0 {
		return lipgloss.Color(palette[len(palette)-1])
	}
	if percent <= 0.0 {
		return lipgloss.Color(palette[0])
	}

	idx := float64(len(palette)-1) * percent
	i := int(idx)
	f := idx - float64(i)
	return GetGradientColor(palette[i], palette[i+1], f)
}

// RenderGradientText applies a horizontal gradient to a string.
func RenderGradientText(text string, palette []string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, r := range runes {
		percent := float64(i) / float64(len(runes))
		color := GetMultiGradientColor(palette, percent)
		sb.WriteString(lipgloss.NewStyle().Foreground(color).Render(string(r)))
	}
	return sb.String()
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
	return b
}
