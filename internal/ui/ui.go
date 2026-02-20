package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/sammwy/teragen/internal/ai"
	"github.com/sammwy/teragen/internal/core"
	"github.com/sammwy/teragen/internal/ui/theme"
)

// ── Palette ──────────────────────────────────────────────────────────────────

const (
	colCyan   = theme.ColorCyan
	colRose   = theme.ColorRose
	colPurple = theme.ColorPurple
	colGreen  = theme.ColorGreen
	colOrange = theme.ColorAccent
	colRed    = theme.ColorRed
	colDim    = theme.ColorDim
	colFg     = theme.ColorFg
	colSubtle = theme.ColorDim
	colYellow = theme.ColorYellow
	colBlack  = theme.ColorBlack
)

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	// ── Header ────────────────────────────────────────────────────────────────
	headerLogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colCyan))

	headerSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colDim))

	headerBarStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// ── Status bar ────────────────────────────────────────────────────────────
	statusBarBase = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colSubtle))

	statusModelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colCyan)).
				Bold(true)

	statusProviderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colPurple)).
				Bold(true)

	statusSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colDim))

	statusTokenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colOrange)).
				Bold(true)

	statusHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colSubtle))

	statusDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colDim))

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
				Foreground(lipgloss.Color(colCyan)).
				Bold(true)

	// ── Message roles — accent colours ─────────────────────────────────────────
	accentUser    = lipgloss.Color(colGreen)
	accentAgent   = lipgloss.Color(colCyan)
	accentCommand = lipgloss.Color(colYellow)
	accentError   = lipgloss.Color(colRed)
	accentSystem  = lipgloss.Color(colPurple)

	// ── Message body ─────────────────────────────────────────────────────────
	bodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colFg))

	// ── Token count hint ─────────────────────────────────────────────────────
	tokenHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colRose)).
			Italic(true)

	// ── Spinner text ─────────────────────────────────────────────────────────
	spinnerTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colSubtle)).
				Italic(true)

	// ── Tabs ────────────────────────────────────────────────────────────────
	tabActiveStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(colCyan)).
			Foreground(lipgloss.Color(colBlack)).
			Bold(true).
			Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(colDim)).
				Foreground(lipgloss.Color(colFg)).
				Padding(0, 1)

	headerCwdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colSubtle))

	pathStartStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colPurple))
	pathMiddleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colFg))
	pathDotsStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	pathEndStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colRose)).Bold(true)
	pathSepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
)

// ── Entry role ────────────────────────────────────────────────────────────────

type role int

const (
	roleUser role = iota
	roleAgent
	roleCommand
	roleError
	roleSystem
)

// ── Chat entry ────────────────────────────────────────────────────────────────

type chatEntry struct {
	role       role
	label      string
	body       string
	tokenCount int
	timestamp  string

	// Cache for rendered output
	renderedBody  string
	renderedWidth int
}

// ── Tea messages ─────────────────────────────────────────────────────────────

type processResultMsg struct {
	input    string
	cmdName  string
	response string
	tokens   int
	err      error
	stream   bool
	done     bool
}

type model struct {
	core          *core.Core
	activeAgentID string
	viewport      viewport.Model
	textInput     textinput.Model
	spinner       spinner.Model
	entries       []chatEntry
	processing    bool
	sessionTokens int
	width         int
	height        int

	inputHistory []string
	historyIndex int

	eventCh chan core.Event

	// Markdown renderer
	renderer *glamour.TermRenderer
}

func NewUI(c *core.Core, activeAgentID string) *model {
	ti := textinput.New()
	ti.Placeholder = "Type a message or /command..."
	ti.Focus()
	ti.CharLimit = 4000
	ti.PromptStyle = inputPromptStyle
	ti.Prompt = "  ❯ "

	vp := viewport.New(80, 20)

	// Create renderer with a fixed dark style (Dracula-like) to avoid terminal round-trips
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(80),
	)

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colCyan))

	m := &model{
		core:          c,
		activeAgentID: activeAgentID,
		textInput:     ti,
		viewport:      vp,
		spinner:       sp,
		entries:       []chatEntry{},
		eventCh:       make(chan core.Event, 100),
		width:         80,
		height:        24,
		renderer:      r,
	}

	// Hydro-hydrate...
	m.relayout()

	// Hydrate history from agent if available
	if activeAgentID != "" {
		if a, ok := c.Agents[activeAgentID]; ok {
			for _, msg := range a.History {
				r := roleAgent
				label := "Agent"
				if msg.Role == "user" {
					r = roleUser
					label = "You"
				}
				m.entries = append(m.entries, chatEntry{
					role:       r,
					label:      label,
					body:       msg.Content,
					tokenCount: msg.Tokens,
					timestamp:  msg.Timestamp,
				})
			}
		}
	}

	c.Subscribe(m)
	return m
}

func (m *model) OnEvent(e core.Event) {
	m.eventCh <- e
}

func waitForEvent(ch chan core.Event) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		waitForEvent(m.eventCh),
	)
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.relayout()
		return m, nil

	case core.Event:
		cmds = append(cmds, waitForEvent(m.eventCh))

		switch msg.Type {
		case core.EventError:
			err := msg.Data.(error)
			m.entries = append(m.entries, chatEntry{
				role:  roleError,
				label: "Error",
				body:  err.Error(),
			})
			m.processing = false

		case core.EventCommandResult:
			resp := msg.Data.(string)
			m.entries = append(m.entries, chatEntry{
				role:  roleCommand,
				label: "Command",
				body:  resp,
			})
			m.processing = false

		case core.EventChatVisualClear:
			m.entries = []chatEntry{}
			m.processing = false

		case core.EventChatInternalClear:
			m.entries = []chatEntry{}
			m.processing = false

		case core.EventChatSwitched:
			newID := msg.Data.(string)
			m.activeAgentID = newID
			m.hydrateHistory()
			m.processing = false

		case core.EventStreamChunk:
			ev := msg.Data.(ai.StreamEvent)
			lastIdx := len(m.entries) - 1
			if lastIdx >= 0 && m.entries[lastIdx].role == roleAgent && m.processing {
				m.entries[lastIdx].body += ev.Content
				m.entries[lastIdx].tokenCount = ev.Tokens
				// Invalidate cache
				m.entries[lastIdx].renderedWidth = 0
			} else {
				m.entries = append(m.entries, chatEntry{
					role:       roleAgent,
					label:      "Agent",
					body:       ev.Content,
					tokenCount: ev.Tokens,
					timestamp:  time.Now().Format(time.RFC3339),
				})
			}

		case core.EventStreamDone:
			ev := msg.Data.(ai.StreamEvent)
			m.processing = false
			// Recalculate total tokens from agent history
			if a, ok := m.core.Agents[m.activeAgentID]; ok {
				total := 0
				for _, hm := range a.History {
					total += hm.Tokens
				}
				m.sessionTokens = total
			}
			// Last chunk update if any tokens reported
			lastIdx := len(m.entries) - 1
			if lastIdx >= 0 && m.entries[lastIdx].role == roleAgent {
				m.entries[lastIdx].tokenCount = ev.Tokens
				m.entries[lastIdx].renderedWidth = 0
			}
		}

		m.refreshViewport()
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.processing {
			m.refreshViewport() // re-render the thinking spinner
		}
		return m, cmd

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyUp:
			if len(m.inputHistory) > 0 {
				if m.historyIndex > 0 {
					m.historyIndex--
					m.textInput.SetValue(m.inputHistory[m.historyIndex])
					m.textInput.CursorEnd()
				}
			}
			return m, nil

		case tea.KeyShiftUp:
			m.viewport.LineUp(1)
			return m, nil

		case tea.KeyShiftRight:
			if m.processing {
				return m, nil
			}
			return m.switchAgent(1)

		case tea.KeyShiftLeft:
			if m.processing {
				return m, nil
			}
			return m.switchAgent(-1)

		case tea.KeyDown:
			if len(m.inputHistory) > 0 {
				if m.historyIndex < len(m.inputHistory)-1 {
					m.historyIndex++
					m.textInput.SetValue(m.inputHistory[m.historyIndex])
					m.textInput.CursorEnd()
				} else {
					m.historyIndex = len(m.inputHistory)
					m.textInput.SetValue("")
				}
			}
			return m, nil

		case tea.KeyShiftDown:
			m.viewport.LineDown(1)
			return m, nil

		case tea.KeyEnter:
			if m.processing {
				return m, nil
			}
			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				return m, nil
			}

			// Add to history
			m.inputHistory = append(m.inputHistory, input)
			m.historyIndex = len(m.inputHistory)

			// Add user entry
			uTokens := estimateTokens(input)
			m.sessionTokens += uTokens
			ts := time.Now().Format(time.RFC3339)
			if strings.HasPrefix(input, "/") {
				parts := strings.Fields(input[1:])
				m.entries = append(m.entries, chatEntry{
					role:       roleUser,
					label:      "You",
					body:       "/" + strings.Join(parts, " "),
					tokenCount: uTokens,
					timestamp:  ts,
				})
			} else {
				m.entries = append(m.entries, chatEntry{
					role:       roleUser,
					label:      "You",
					body:       input,
					tokenCount: uTokens,
					timestamp:  ts,
				})
			}

			m.textInput.SetValue("")
			m.processing = true
			m.refreshViewport()

			// Create agent if in [+] state
			if m.activeAgentID == "+" {
				newID, _, err := m.core.CreateAgent()
				if err != nil {
					m.entries = append(m.entries, chatEntry{role: roleError, label: "Error", body: err.Error()})
					m.processing = false
					m.refreshViewport()
					return m, nil
				}
				m.activeAgentID = newID
			}

			// Dispatch asynchronously through core
			m.core.DispatchPrompt(context.Background(), m.activeAgentID, input)

			cmds = append(cmds, m.spinner.Tick)
			return m, tea.Batch(cmds...)

		default:
			var tiCmd tea.Cmd
			m.textInput, tiCmd = m.textInput.Update(msg)
			return m, tiCmd
		}
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, vpCmd
}

func (m *model) switchAgent(delta int) (tea.Model, tea.Cmd) {
	ids := m.core.GetAgentList()
	// Sequence: [id1, id2, ..., idN, "+"]
	nav := append([]string(nil), ids...)
	nav = append(nav, "+")

	currentIndex := -1
	for i, id := range nav {
		if id == m.activeAgentID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		// Fallback to first agent
		m.activeAgentID = nav[0]
		m.hydrateHistory()
		return m, nil
	}

	// 1. Check if the current agent should be deleted if we leave it
	// (Only for real agents that are empty and unsaved)
	if m.activeAgentID != "+" {
		currentAgent := m.core.Agents[m.activeAgentID]
		if currentAgent != nil && currentAgent.ActiveChatID == "" && len(currentAgent.History) == 0 {
			m.core.UnregisterAgent(m.activeAgentID)
			// Refresh nav after deletion
			ids = m.core.GetAgentList()
			nav = append([]string(nil), ids...)
			nav = append(nav, "+")
			// We need to re-find our index or just adjust it.
			// Since we deleted the current one, the list shifted.
			// But it's easier to just calculate the next target from the OLD list
			// and then wrap it around the NEW list.
		}
	}

	targetIndex := (currentIndex + delta) % len(nav)
	if targetIndex < 0 {
		targetIndex = len(nav) - 1
	}

	m.activeAgentID = nav[targetIndex]
	m.hydrateHistory()
	m.refreshViewport()
	return m, nil
}

func (m *model) hydrateHistory() {
	m.entries = []chatEntry{}
	m.sessionTokens = 0

	if m.activeAgentID == "+" {
		return
	}

	if a, ok := m.core.Agents[m.activeAgentID]; ok {
		for _, msg := range a.History {
			m.sessionTokens += msg.Tokens
			r := roleAgent
			label := "Agent"
			if msg.Role == "user" {
				r = roleUser
				label = "You"
			}
			m.entries = append(m.entries, chatEntry{
				role:       r,
				label:      label,
				body:       msg.Content,
				tokenCount: msg.Tokens,
				timestamp:  msg.Timestamp,
			})
		}
	}
}

// ── Layout ────────────────────────────────────────────────────────────────────

// ── Layout ────────────────────────────────────────────────────────────────────

func (m *model) relayout() {
	// top_border=1, inputArea=1, statusBar=1 => 3 reserved
	reserved := 3
	vpHeight := m.height - reserved
	if vpHeight < 3 {
		vpHeight = 3
	}
	vpWidth := m.width
	if vpWidth < 40 {
		vpWidth = 40
	}
	m.viewport.Width = vpWidth - 2 // space for vertical borders
	m.viewport.Height = vpHeight
	m.textInput.Width = clamp(vpWidth-10, 20, vpWidth-10)

	// Update renderer word wrap
	if m.renderer != nil {
		innerW := m.viewport.Width - 4
		if innerW < 20 {
			innerW = 20
		}
		// Glamour doesn't have an easy "SetWidth" so we recreate it if needed,
		// but standard dark style is already fast and doesn't query terminal.
		r, _ := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(innerW),
		)
		m.renderer = r
	}

	// Invalidate all caches on resize
	for i := range m.entries {
		m.entries[i].renderedWidth = 0
	}

	m.refreshViewport()
}

func (m *model) refreshViewport() {
	w := m.viewport.Width
	if w < 10 {
		w = 80
	}
	m.viewport.SetContent(m.renderEntries(w))
	m.viewport.GotoBottom()
}

// ── Entry rendering ───────────────────────────────────────────────────────────

func (m *model) renderEntries(width int) string {
	var sb strings.Builder
	for i := range m.entries {
		if i > 0 {
			// Extra blank line between messages for breathing room
			sb.WriteString("\n\n")
		}
		sb.WriteString(m.renderEntry(&m.entries[i], width))
	}
	if m.processing {
		sb.WriteString("\n\n")
		sb.WriteString(m.renderSpinnerEntry(width))
	}
	return sb.String()
}

// accentInfo returns the accent color and a display label for a role.
func accentInfo(r role) (lipgloss.Color, string) {
	switch r {
	case roleUser:
		return accentUser, "You"
	case roleAgent:
		return accentAgent, "Agent"
	case roleCommand:
		return accentCommand, "Command"
	case roleError:
		return accentError, "Error"
	default:
		return accentSystem, "System"
	}
}

func (m *model) renderEntry(e *chatEntry, width int) string {
	// Use cache if available
	if e.renderedWidth == width && e.renderedBody != "" {
		return e.renderedBody
	}

	accent, _ := accentInfo(e.role)

	// ── Left border bar ────────────────────────────────────────────────────
	barStyle := lipgloss.NewStyle().
		Border(lipgloss.Border{Left: "▌"}, false, false, false, true).
		BorderLeftForeground(accent).
		PaddingLeft(1)

	// ── Badge (Role Label) ────────────────────────────────────────────────
	badgeStyle := lipgloss.NewStyle().
		Background(accent).
		Foreground(lipgloss.Color(colBlack)).
		Bold(true).
		Padding(0, 1)

	badge := badgeStyle.Render(e.label)

	headerTxt := badge
	if e.timestamp != "" {
		headerTxt += " " + lipgloss.NewStyle().Foreground(lipgloss.Color(colFg)).Render(formatTimestamp(e.timestamp))
	}
	if e.tokenCount > 0 {
		headerTxt += " " + lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Render(fmt.Sprintf("(%s tokens)", fmtTokens(e.tokenCount)))
	}

	// ── Inner content width ────────────────────────────────────────────────
	// 4 for "▌ " border and some breathing room
	innerW := width - 4
	if innerW < 20 {
		innerW = 20
	}

	// ── Body (Markdown) ───────────────────────────────────────────────────
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
		"",
		bodyRendered,
	)

	e.renderedBody = barStyle.Render(content)
	e.renderedWidth = width
	return e.renderedBody
}

func (m *model) renderSpinnerEntry(width int) string {
	accent := accentAgent

	// ── Left border bar ────────────────────────────────────────────────────
	barStyle := lipgloss.NewStyle().
		Border(lipgloss.Border{Left: "▌"}, false, false, false, true).
		BorderLeftForeground(accent).
		PaddingLeft(1)

	badge := lipgloss.NewStyle().
		Background(accent).
		Foreground(lipgloss.Color(colBlack)).
		Bold(true).
		Padding(0, 1).
		Render("Agent")

	thinking := spinnerTextStyle.Render(m.spinner.View() + "  thinking…")

	content := lipgloss.JoinVertical(lipgloss.Left,
		badge,
		thinking,
	)

	return barStyle.Render(content)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *model) View() string {
	w := m.width
	if w < 10 {
		w = 80
	}

	// ── Header (Top Border) ───────────────────────────────────────────────────
	topLineStyle := lipgloss.NewStyle().Foreground(theme.Purple)

	logo := topLineStyle.Render("─") + " " + headerLogoStyle.Render("✦ TERAGEN") + " " + topLineStyle.Render("─")

	// Agent Tabs
	var tabs []string
	ids := m.core.GetAgentList()
	for _, id := range ids {
		if id == m.activeAgentID {
			tabs = append(tabs, tabActiveStyle.Render(id))
		} else {
			tabs = append(tabs, tabInactiveStyle.Render(id))
		}
	}
	// Add [+] tab
	if m.activeAgentID == "+" {
		tabs = append(tabs, tabActiveStyle.Render("+"))
	} else {
		tabs = append(tabs, tabInactiveStyle.Render("+"))
	}
	tabsStr := topLineStyle.Render("─") + " " + strings.Join(tabs, " ") + " " + topLineStyle.Render("─")

	logoLen := lipgloss.Width(logo)
	tabsLen := lipgloss.Width(tabsStr)

	// CWD section with dynamic trimming
	cwd, _ := os.Getwd()
	// Available width for path: Total - Logo - Tabs - corners - separators
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

	// ── Chat Content with Gradient Side Borders ───────────────────────────────
	chatLines := strings.Split(m.viewport.View(), "\n")
	var chatWithBorders strings.Builder
	vh := m.viewport.Height

	for i := 0; i < vh; i++ {
		// Calculate gradient color for this line
		percent := float64(i) / float64(vh-1)
		lineColor := theme.GetGradientColor(percent)
		verticalBorder := lipgloss.NewStyle().Foreground(lineColor).Render("│")

		content := ""
		if i < len(chatLines) {
			content = chatLines[i]
		}
		// Pad content to viewport width
		currLen := lipgloss.Width(content)
		padding := m.viewport.Width - currLen
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

	// ── Input area ────────────────────────────────────────────────────────────
	var inputContent string
	if m.processing {
		inputContent = spinnerTextStyle.Render(m.spinner.View() + "  Processing…")
	} else {
		inputContent = m.textInput.View()
	}

	inputPadding := w - 2 - lipgloss.Width(inputContent)
	if inputPadding < 0 {
		inputPadding = 0
	}
	inputVerticalBorder := lipgloss.NewStyle().Foreground(theme.Rose).Render("│")
	inputLine := inputVerticalBorder + " " + inputContent + strings.Repeat(" ", inputPadding-1) + inputVerticalBorder

	// ── Bottom Border (Status) ────────────────────────────────────────────────
	bottomLine := m.renderBottomBorder(w)

	return lipgloss.JoinVertical(lipgloss.Left,
		topLine,
		chatWithBorders.String(),
		inputLine,
		bottomLine,
	)
}

func (m model) renderBottomBorder(width int) string {
	bottomBorderStyle := lipgloss.NewStyle().Foreground(theme.Rose)

	// Left: Chat ID | Provider > Model
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
		tokenStr = statusTokenStyle.Render(fmt.Sprintf("%s tokens", fmtTokens(m.sessionTokens))) + " " + statusDividerStyle.Render("·") + " "
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

// ── Run ───────────────────────────────────────────────────────────────────────

func (m *model) Run() error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	var out strings.Builder
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			out.WriteRune('\n')
		}
		out.WriteString(wrapLine(line, width))
	}
	return out.String()
}

func wrapLine(line string, width int) string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}
	var out strings.Builder
	col := 0
	for i, w := range words {
		wl := utf8.RuneCountInString(w)
		if i > 0 {
			if col+1+wl > width {
				out.WriteRune('\n')
				col = 0
			} else {
				out.WriteRune(' ')
				col++
			}
		}
		out.WriteString(w)
		col += wl
	}
	return out.String()
}

func (m *model) renderPath(path string, maxWidth int) string {
	sep := string(os.PathSeparator)
	parts := strings.Split(path, sep)
	if len(parts) == 0 {
		return ""
	}

	type node struct {
		text    string
		isDots  bool
		isStart bool
		isEnd   bool
	}

	nodes := make([]node, len(parts))
	for i, p := range parts {
		nodes[i] = node{text: p, isStart: i == 0, isEnd: i == len(parts)-1}
	}

	calcWidth := func(ns []node) int {
		total := 0
		for i, n := range ns {
			total += utf8.RuneCountInString(n.text)
			if i < len(ns)-1 {
				total += 1
			}
		}
		return total
	}

	for calcWidth(nodes) > maxWidth && len(nodes) > 2 {
		mid := len(nodes) / 2
		removed := false
		// Try to find a middle part to turn into dots
		for d := 0; d < len(nodes); d++ {
			// Search outwards from middle
			i := mid + d
			if i > 0 && i < len(nodes)-1 && !nodes[i].isDots {
				nodes[i].text = "..."
				nodes[i].isDots = true
				removed = true
				break
			}
			i = mid - d
			if i > 0 && i < len(nodes)-1 && !nodes[i].isDots {
				nodes[i].text = "..."
				nodes[i].isDots = true
				removed = true
				break
			}
		}

		if !removed {
			// All middle parts are dots, start removing dots
			for i := 1; i < len(nodes)-1; i++ {
				if nodes[i].isDots {
					nodes = append(nodes[:i], nodes[i+1:]...)
					removed = true
					break
				}
			}
		}

		if !removed {
			break
		}
	}

	var sb strings.Builder
	for i, n := range nodes {
		style := pathMiddleStyle
		if n.isDots {
			style = pathDotsStyle
		} else if n.isStart {
			style = pathStartStyle
		} else if n.isEnd {
			style = pathEndStyle
		}

		sb.WriteString(style.Render(n.text))
		if i < len(nodes)-1 {
			sb.WriteString(pathSepStyle.Render(sep))
		}
	}
	return sb.String()
}

func fmtTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func estimateTokens(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	return int(float64(words)*1.3) + 1
}

func formatTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	now := time.Now()
	if t.Year() != now.Year() {
		return t.Format("02/01/2006 15:04")
	}
	if t.YearDay() != now.YearDay() {
		return t.Format("02/01 15:04")
	}
	return t.Format("15:04")
}
