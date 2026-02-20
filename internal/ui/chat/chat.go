package chat

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/sammwy/teragen/internal/ai"
	"github.com/sammwy/teragen/internal/core"
	"github.com/sammwy/teragen/internal/ui/common"
)

type role int

const (
	roleUser role = iota
	roleAgent
	roleCommand
	roleError
	roleSystem
)

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

type Model struct {
	core          *core.Core
	activeAgentID string
	viewport      viewport.Model
	textInput     textinput.Model
	spinner       spinner.Model
	entries       []chatEntry
	busyAgents    map[string]bool
	sessionTokens int
	width         int
	height        int

	inputHistory []string
	historyIndex int

	eventCh chan core.Event

	// Markdown renderer
	renderer *glamour.TermRenderer
}

func NewUI(c *core.Core, activeAgentID string) *Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message or /command..."
	ti.Focus()
	ti.CharLimit = 4000
	ti.PromptStyle = inputPromptStyle
	ti.Prompt = "  ❯ "

	vp := viewport.New(80, 20)

	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(80),
	)

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = spinnerTextStyle

	m := &Model{
		core:          c,
		activeAgentID: activeAgentID,
		textInput:     ti,
		viewport:      vp,
		spinner:       sp,
		entries:       []chatEntry{},
		busyAgents:    make(map[string]bool),
		eventCh:       make(chan core.Event, 100),
		width:         80,
		height:        24,
		renderer:      r,
	}

	m.relayout()

	if activeAgentID != "" {
		m.hydrateHistory()
	}

	c.Subscribe(m)
	return m
}

func (m *Model) Run() error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m *Model) OnEvent(e core.Event) {
	m.eventCh <- e
}

func waitForEvent(ch chan core.Event) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		waitForEvent(m.eventCh),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.busyAgents[msg.AgentID] = false
			if msg.AgentID == m.activeAgentID {
				m.entries = append(m.entries, chatEntry{
					role:  roleError,
					label: "Error",
					body:  err.Error(),
				})
			}

		case core.EventCommandResult:
			resp := msg.Data.(string)
			m.busyAgents[msg.AgentID] = false
			if msg.AgentID == m.activeAgentID {
				m.entries = append(m.entries, chatEntry{
					role:  roleCommand,
					label: "Command",
					body:  resp,
				})
			}

		case core.EventChatVisualClear:
			if msg.AgentID == m.activeAgentID {
				m.entries = []chatEntry{}
			}
			m.busyAgents[msg.AgentID] = false

		case core.EventChatInternalClear:
			if msg.AgentID == m.activeAgentID {
				m.entries = []chatEntry{}
			}
			m.busyAgents[msg.AgentID] = false

		case core.EventChatSwitched:
			newID := msg.Data.(string)
			m.activeAgentID = newID
			m.hydrateHistory()
			m.busyAgents[msg.AgentID] = false

		case core.EventStreamChunk:
			ev := msg.Data.(ai.StreamEvent)
			m.busyAgents[msg.AgentID] = true
			if msg.AgentID == m.activeAgentID {
				lastIdx := len(m.entries) - 1
				if lastIdx >= 0 && m.entries[lastIdx].role == roleAgent && m.busyAgents[msg.AgentID] {
					m.entries[lastIdx].body += ev.Content
					if ev.Tokens > 0 {
						m.entries[lastIdx].tokenCount = ev.Tokens
						if a, ok := m.core.Agents[m.activeAgentID]; ok {
							total := 0
							for _, hm := range a.History {
								total += hm.Tokens
							}
							m.sessionTokens = total
						}
					}
					m.entries[lastIdx].renderedWidth = 0
				} else {
					m.entries = append(m.entries, chatEntry{
						role:       roleAgent,
						label:      "  AGENT  ",
						body:       ev.Content,
						tokenCount: ev.Tokens,
						timestamp:  time.Now().Format(time.RFC3339),
					})
				}
			}

		case core.EventStreamDone:
			ev := msg.Data.(ai.StreamEvent)
			m.busyAgents[msg.AgentID] = false
			if msg.AgentID == m.activeAgentID {
				if a, ok := m.core.Agents[m.activeAgentID]; ok {
					total := 0
					for _, hm := range a.History {
						total += hm.Tokens
					}
					m.sessionTokens = total
				}
				lastIdx := len(m.entries) - 1
				if lastIdx >= 0 && m.entries[lastIdx].role == roleAgent {
					if ev.Tokens > 0 {
						m.entries[lastIdx].tokenCount = ev.Tokens
					}
					m.entries[lastIdx].renderedWidth = 0
				}
			}
		}

		m.refreshViewport()
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.busyAgents[m.activeAgentID] {
			m.refreshViewport()
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
			return m.switchAgent(1)

		case tea.KeyShiftLeft:
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
			if m.busyAgents[m.activeAgentID] {
				return m, nil
			}
			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				return m, nil
			}

			m.inputHistory = append(m.inputHistory, input)
			m.historyIndex = len(m.inputHistory)

			uTokens := common.EstimateTokens(input)
			m.sessionTokens += uTokens
			ts := time.Now().Format(time.RFC3339)
			if strings.HasPrefix(input, "/") {
				parts := strings.Fields(input[1:])
				m.entries = append(m.entries, chatEntry{
					role:       roleUser,
					label:      "   YOU   ",
					body:       "/" + strings.Join(parts, " "),
					tokenCount: uTokens,
					timestamp:  ts,
				})
			} else {
				m.entries = append(m.entries, chatEntry{
					role:       roleUser,
					label:      "   YOU   ",
					body:       input,
					tokenCount: uTokens,
					timestamp:  ts,
				})
			}

			m.textInput.SetValue("")
			m.busyAgents[m.activeAgentID] = true
			m.refreshViewport()

			if m.activeAgentID == "+" {
				newID, _, err := m.core.CreateAgent()
				if err != nil {
					m.entries = append(m.entries, chatEntry{role: roleError, label: "Error", body: err.Error()})
					m.busyAgents[m.activeAgentID] = false
					m.refreshViewport()
					return m, nil
				}
				m.activeAgentID = newID
			}

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

func (m *Model) hydrateHistory() {
	m.entries = []chatEntry{}
	m.sessionTokens = 0

	if m.activeAgentID == "+" {
		return
	}

	if a, ok := m.core.Agents[m.activeAgentID]; ok {
		for _, msg := range a.History {
			m.sessionTokens += msg.Tokens
			r := roleAgent
			label := "  AGENT  "
			if msg.Role == "user" {
				r = roleUser
				label = "   YOU   "
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

func (m *Model) switchAgent(delta int) (tea.Model, tea.Cmd) {
	ids := m.core.GetAgentList()
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
		m.activeAgentID = nav[0]
		m.hydrateHistory()
		return m, nil
	}

	if m.activeAgentID != "+" {
		currentAgent := m.core.Agents[m.activeAgentID]
		if currentAgent != nil && currentAgent.ActiveChatID == "" && len(currentAgent.History) == 0 {
			m.core.UnregisterAgent(m.activeAgentID)
			ids = m.core.GetAgentList()
			nav = append([]string(nil), ids...)
			nav = append(nav, "+")
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
