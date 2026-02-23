package chat

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/sammwy/teragen/internal/ui/theme"
)

var (
	// ── Header ────────────────────────────────────────────────────────────────
	headerLogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(theme.ColorVibrantCyan))

	headerSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorDim))

	headerBarStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// ── Status bar ────────────────────────────────────────────────────────────
	statusBarBase = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorDim))

	statusModelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorVibrantCyan)).
				Bold(true)

	statusProviderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorElectricBlue)).
				Bold(true)

	statusEngineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorVibrantPink)).
				Bold(true)

	statusSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorDim))

	statusTokenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorGoldenOrange)).
				Bold(true)

	statusHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorDim))

	statusDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorDim))

	// ── Input area ────────────────────────────────────────────────────────────
	inputAreaStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(2).
			PaddingTop(0).
			PaddingBottom(0)

	inputActiveAreaStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				PaddingRight(2).
				PaddingTop(0).
				PaddingBottom(0)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorVibrantCyan)).
				Bold(true)

	// ── Message roles — accent colours ─────────────────────────────────────────
	accentUser    = lipgloss.Color(theme.ColorGreen)
	accentAgent   = lipgloss.Color(theme.ColorVibrantCyan)
	accentCommand = lipgloss.Color(theme.ColorGoldenOrange)
	accentError   = lipgloss.Color(theme.ColorRed)
	accentSystem  = lipgloss.Color(theme.ColorDeepPurple)

	// ── Message body ─────────────────────────────────────────────────────────
	bodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorFg))

	// ── Token count hint ─────────────────────────────────────────────────────
	tokenHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorRose)).
			Italic(true)

	// ── Spinner text ─────────────────────────────────────────────────────────
	spinnerTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.ColorDim)).
				Italic(true)

	// ── Tabs ────────────────────────────────────────────────────────────────
	tabActiveStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(theme.ColorVibrantCyan)).
			Foreground(lipgloss.Color(theme.ColorBlack)).
			Bold(true).
			Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(theme.ColorDim)).
				Foreground(lipgloss.Color(theme.ColorFg)).
				Padding(0, 1)

	headerCwdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorDim))

	pathStartStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorDeepPurple))
	pathMiddleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorFg))
	pathDotsStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorDim))
	pathEndStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorVibrantPink)).Bold(true)
	pathSepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorDim))
)
