package chat

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/sammwy/teragen/internal/ui/common"
	"github.com/sammwy/teragen/internal/ui/theme"
)

func (m *Model) relayout() {
	reserved := 3
	vpHeight := m.height - reserved
	if vpHeight < 3 {
		vpHeight = 3
	}
	vpWidth := m.width
	if vpWidth < 40 {
		vpWidth = 40
	}
	m.viewport.Width = vpWidth - 2
	m.viewport.Height = vpHeight
	m.textInput.Width = common.Clamp(vpWidth-10, 20, vpWidth-10)

	if m.renderer != nil {
		innerW := m.viewport.Width - 4
		if innerW < 20 {
			innerW = 20
		}
		r, _ := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(innerW),
		)
		m.renderer = r
	}

	for i := range m.entries {
		m.entries[i].renderedWidth = 0
	}

	m.refreshViewport()
}

func (m *Model) refreshViewport() {
	w := m.viewport.Width
	if w < 10 {
		w = 80
	}
	m.viewport.SetContent(m.renderEntries(w))
	m.viewport.GotoBottom()
}

func (m *Model) renderEntries(width int) string {
	var sb strings.Builder
	for i := range m.entries {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(m.renderEntry(&m.entries[i], width))
	}
	if m.busyAgents[m.activeAgentID] {
		sb.WriteString("\n\n")
		sb.WriteString(m.renderSpinnerEntry(width))
	}
	return sb.String()
}

func accentInfo(r role) (lipgloss.Color, []string) {
	switch r {
	case roleUser:
		return lipgloss.Color(theme.ColorGreen), theme.PaletteUser
	case roleAgent:
		return lipgloss.Color(theme.ColorCyan), theme.PaletteAgent
	case roleCommand:
		return lipgloss.Color(theme.ColorYellow), theme.PaletteAccent
	case roleError:
		return lipgloss.Color(theme.ColorRed), []string{theme.ColorRed, theme.ColorVibrantPink}
	default:
		return lipgloss.Color(theme.ColorPurple), theme.PaletteSecondary
	}
}

func (m *Model) renderEntry(e *chatEntry, width int) string {
	if e.renderedWidth == width && e.renderedBody != "" {
		return e.renderedBody
	}

	accent, palette := accentInfo(e.role)

	barStyle := lipgloss.NewStyle().
		Border(lipgloss.Border{Left: "▌"}, false, false, false, true).
		BorderLeftForeground(accent).
		PaddingLeft(1)

	// Gradient tag
	tagText := e.label
	tagRendered := ""
	for i, r := range []rune(tagText) {
		percent := float64(i) / float64(len(tagText))
		color := theme.GetMultiGradientColor(palette, percent)
		tagRendered += lipgloss.NewStyle().
			Background(color).
			Foreground(lipgloss.Color(theme.ColorBlack)).
			Bold(true).
			Render(string(r))
	}

	headerTxt := tagRendered
	if e.timestamp != "" {
		headerTxt += " " + lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorFg)).Render(common.FormatTimestamp(e.timestamp))
	}
	if e.tokenCount > 0 {
		tokensTxt := fmt.Sprintf("(%s tokens)", common.FmtTokens(e.tokenCount))
		headerTxt += " " + tokenHintStyle.Render(tokensTxt)
	}

	innerW := width - 4
	if innerW < 20 {
		innerW = 20
	}

	var bodyRendered string
	if (e.role == roleAgent || e.role == roleUser) && m.renderer != nil {
		rendered, err := m.renderer.Render(e.body)
		if err == nil {
			bodyRendered = strings.TrimSpace(rendered)
		}
	}

	if bodyRendered == "" {
		wrapped := wordWrap(e.body, innerW)
		bodyRendered = bodyStyle.Render(wrapped)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		headerTxt,
		bodyRendered,
	)

	e.renderedBody = barStyle.Render(content)
	e.renderedWidth = width
	return e.renderedBody
}

func (m *Model) renderSpinnerEntry(width int) string {
	accent, palette := accentInfo(roleAgent)

	barStyle := lipgloss.NewStyle().
		Border(lipgloss.Border{Left: "▌"}, false, false, false, true).
		BorderLeftForeground(accent).
		PaddingLeft(1)

	tagText := "  AGENT  "
	tagRendered := ""
	for i, r := range []rune(tagText) {
		percent := float64(i) / float64(len(tagText))
		color := theme.GetMultiGradientColor(palette, percent)
		tagRendered += lipgloss.NewStyle().
			Background(color).
			Foreground(lipgloss.Color(theme.ColorBlack)).
			Bold(true).
			Render(string(r))
	}

	thinking := spinnerTextStyle.Render(m.spinner.View() + "  thinking…")

	content := lipgloss.JoinVertical(lipgloss.Left,
		tagRendered,
		thinking,
	)

	return barStyle.Render(content)
}

func (m *Model) View() string {
	w := m.width
	if w < 10 {
		w = 80
	}

	topLineStyle := lipgloss.NewStyle().Foreground(theme.Purple)

	banner := theme.RenderGradientText("✦ TERAGEN", theme.PalettePrimary)
	logo := topLineStyle.Render("─") + " " + banner + " " + topLineStyle.Render("─")

	var tabs []string
	ids := m.core.GetAgentList()
	for _, id := range ids {
		if id == m.activeAgentID {
			tabs = append(tabs, tabActiveStyle.Render(id))
		} else {
			tabs = append(tabs, tabInactiveStyle.Render(id))
		}
	}
	if m.activeAgentID == "+" {
		tabs = append(tabs, tabActiveStyle.Render("+"))
	} else {
		tabs = append(tabs, tabInactiveStyle.Render("+"))
	}
	tabsStr := topLineStyle.Render("─") + " " + strings.Join(tabs, " ") + " " + topLineStyle.Render("─")

	logoLen := lipgloss.Width(logo)
	tabsLen := lipgloss.Width(tabsStr)

	cwd, _ := os.Getwd()
	availablePathWidth := w - logoLen - tabsLen - 8
	if availablePathWidth < 10 {
		availablePathWidth = 10
	}
	renderedPath := m.renderPath(cwd, availablePathWidth)
	cwdTag := topLineStyle.Render("┤") + " " + renderedPath + " " + topLineStyle.Render("├")
	cwdLen := lipgloss.Width(cwdTag)

	middleEmpty := w - logoLen - cwdLen - tabsLen - 2
	if middleEmpty < 0 {
		middleEmpty = 0
	}

	topLine := topLineStyle.Render("╭") +
		logo + cwdTag + topLineStyle.Render(strings.Repeat("─", middleEmpty)) +
		tabsStr + topLineStyle.Render("╮")

	chatLines := strings.Split(m.viewport.View(), "\n")
	var chatWithBorders strings.Builder
	vh := m.viewport.Height

	for i := 0; i < vh; i++ {
		percent := float64(i) / float64(vh-1)
		lineColor := theme.GetMultiGradientColor(theme.PalettePrimary, percent)
		verticalBorder := lipgloss.NewStyle().Foreground(lineColor).Render("│")

		content := ""
		if i < len(chatLines) {
			content = chatLines[i]
		}
		padding := m.viewport.Width - lipgloss.Width(content)
		if padding < 0 {
			padding = 0
		}

		chatWithBorders.WriteString(verticalBorder)
		chatWithBorders.WriteString(content)
		chatWithBorders.WriteString(strings.Repeat(" ", padding))
		chatWithBorders.WriteString(verticalBorder)
		if i < vh-1 {
			chatWithBorders.WriteRune('\n')
		}
	}

	inputArea := m.renderInputArea(w)
	bottomBorder := m.renderBottomBorder(w)

	return lipgloss.JoinVertical(lipgloss.Left,
		topLine,
		chatWithBorders.String(),
		inputArea,
		bottomBorder,
	)
}

func (m *Model) renderInputArea(width int) string {
	verticalBorder := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorVibrantPink)).Render("│")

	var inputContent string
	if m.busyAgents[m.activeAgentID] {
		inputContent = spinnerTextStyle.Render(m.spinner.View() + "  Processing…")
	} else {
		inputContent = m.textInput.View()
	}

	inputPadding := width - 2 - lipgloss.Width(inputContent)
	if inputPadding < 0 {
		inputPadding = 0
	}
	inputLine := verticalBorder + " " + inputContent + strings.Repeat(" ", inputPadding-1) + verticalBorder

	return inputLine
}

func (m *Model) renderBottomBorder(width int) string {
	bottomBorderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorVibrantPink))

	var leftTag string
	chatID := "?"

	ag, ok := m.core.Agents[m.activeAgentID]
	if ok {
		chatID = ag.ActiveChatID
		leftTag = fmt.Sprintf(" #%s ", chatID)

		if ag.ActiveProvider != nil {
			leftTag += "│ " + statusProviderStyle.Render(ag.ActiveProvider.DisplayName) +
				statusSepStyle.Render(" › ") +
				statusModelStyle.Render(ag.ActiveProvider.Model) + " "
		} else {
			leftTag += "│ No provider "
		}
	} else {
		leftTag = " #? │ No Agent "
	}

	hint := statusHintStyle.Render("[Enter] Submit [ESC] Quit")
	tokenStr := ""
	if m.sessionTokens > 0 {
		tokenStr = statusTokenStyle.Render(fmt.Sprintf("%s tokens", common.FmtTokens(m.sessionTokens))) + " " + statusDividerStyle.Render("·") + " "
	}

	rightTag := " " + tokenStr + hint + " "

	leftLen := lipgloss.Width(leftTag)
	rightLen := lipgloss.Width(rightTag)
	middleLen := width - 2 - leftLen - rightLen
	if middleLen < 0 {
		middleLen = 0
	}

	return bottomBorderStyle.Render("╰") + leftTag +
		bottomBorderStyle.Render(strings.Repeat("─", middleLen)) +
		rightTag + bottomBorderStyle.Render("╯")
}
