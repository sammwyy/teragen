// Package setup implements the first-run configuration wizard for teragen.
//
// The wizard is a BubbleTea TUI that guides the user through:
//  1. Choosing an AI provider (OpenAI / OpenRouter / Custom)
//  2. Entering the API token (masked)
//  3. Entering the agent model (Tab → live modal fetched from /models API)
//  4. (Custom only) entering the base URL before fetching models
//
// When all fields are collected the wizard persists them via the config package
// and returns, handing control back to the main chat UI.
package setup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sammwy/teragen/internal/ai"
	"github.com/sammwy/teragen/internal/config"
	"github.com/sammwy/teragen/internal/ui/common"
	"github.com/sammwy/teragen/internal/ui/theme"
)

// ── View IDs ─────────────────────────────────────────────────────────────────

type view int

const (
	viewProviderPick view = iota // step 1 – choose a provider
	viewForm                     // step 2/3/4 – fill in the fields
)

// ── Field IDs (order inside viewForm) ────────────────────────────────────────

type field int

const (
	fieldToken field = iota
	fieldModel
	fieldBaseURL // only visible for openai-custom
)

// ── Tea messages ──────────────────────────────────────────────────────────────

// DoneMsg is sent when setup completes successfully.
type DoneMsg struct {
	Provider config.ProviderConfig
	Token    string
}

// modelsFetchedMsg is sent by the background goroutine when the /models
// endpoint responds (successfully or not).
type modelsFetchedMsg struct {
	models []string
	err    error
}

// ── Async command ─────────────────────────────────────────────────────────────

// fetchModelsCmd is a BubbleTea Command that fetches models in the background.
// It goes through ListModelsForSetup → NewClient → provider-specific client.ListModels()
// so no URL branching lives here.
func fetchModelsCmd(providerType, baseURL, token string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		models, err := ai.ListModelsForSetup(ctx, providerType, baseURL, token)
		return modelsFetchedMsg{models: models, err: err}
	}
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the BubbleTea model for the setup wizard.
type Model struct {
	width  int
	height int

	// current top-level view
	current view

	// ── provider picker ──
	providerCursor int

	// ── form ──
	selectedProvider Provider
	activeField      field
	inputs           [3]textinput.Model // token, model, baseURL

	// ── model picker modal ──
	showModelModal bool
	modalLoading   bool   // true while waiting for fetch
	modalFetchErr  string // non-empty if last fetch failed
	modalCursor    int
	modalList      []string
	modalScrollOff int
	modalPageSize  int

	// ── form state ──
	err string
}

// NewModel returns a freshly initialised setup wizard model.
func NewModel() Model {
	tokenIn := textinput.New()
	tokenIn.Placeholder = "sk-…"
	tokenIn.EchoMode = textinput.EchoPassword
	tokenIn.EchoCharacter = '●'
	tokenIn.CharLimit = 256
	tokenIn.Width = 32

	modelIn := textinput.New()
	modelIn.Placeholder = "e.g. gpt-4o  (Tab to browse)"
	modelIn.CharLimit = 128
	modelIn.Width = 32

	urlIn := textinput.New()
	urlIn.Placeholder = "https://your-server/v1"
	urlIn.CharLimit = 256
	urlIn.Width = 32

	return Model{
		current:        viewProviderPick,
		providerCursor: 0,
		activeField:    fieldToken,
		inputs:         [3]textinput.Model{tokenIn, modelIn, urlIn},
		modalPageSize:  10,
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case DoneMsg:
		return m, tea.Quit

	// ── Models fetched from API ───────────────────────────────────────────────
	case modelsFetchedMsg:
		m.modalLoading = false
		if msg.err != nil {
			m.modalFetchErr = msg.err.Error()
			// Fall back to built-in list so the modal is still usable
			m.modalList = ModelsFor(m.selectedProvider.Type)
		} else {
			m.modalFetchErr = ""
			m.modalList = msg.models
		}
		m.modalCursor = 0
		m.modalScrollOff = 0
		m.showModelModal = true
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.current {
		case viewProviderPick:
			return m.updateProviderPick(msg)
		case viewForm:
			return m.updateForm(msg)
		}
	}

	return m, nil
}

// ── Provider pick ─────────────────────────────────────────────────────────────

func (m Model) updateProviderPick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerCursor > 0 {
			m.providerCursor--
		}
	case "down", "j":
		if m.providerCursor < len(AvailableProviders)-1 {
			m.providerCursor++
		}
	case "enter":
		m.selectedProvider = AvailableProviders[m.providerCursor]
		m.current = viewForm
		m.activeField = fieldToken
		m.inputs[fieldToken].Focus()
	}
	return m, nil
}

// ── Form ──────────────────────────────────────────────────────────────────────

func (m Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Modal open ───────────────────────────────────────────────────────────
	if m.showModelModal {
		return m.updateModal(msg)
	}

	// ── While loading (Tab pressed but fetch not done yet) ───────────────────
	if m.modalLoading {
		// Block all keys except ctrl+c (already handled above)
		return m, nil
	}

	// ── Normal form ──────────────────────────────────────────────────────────
	switch msg.String() {
	case "tab":
		if m.activeField == fieldModel {
			return m.openModelModal()
		}
		return m.advanceField()

	case "shift+tab":
		return m.retreatField()

	case "enter":
		return m.advanceField()

	case "esc":
		m.current = viewProviderPick
		m.err = ""
		for i := range m.inputs {
			m.inputs[i].Blur()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.inputs[m.activeField], cmd = m.inputs[m.activeField].Update(msg)
	return m, cmd
}

// openModelModal decides the base URL and fires the async fetch.
func (m Model) openModelModal() (tea.Model, tea.Cmd) {
	token := strings.TrimSpace(m.inputs[fieldToken].Value())

	// For custom providers the user supplies the URL manually.
	// For known providers (openai, openrouter) the base URL is baked into
	// their client constructors — we pass an empty string and let NewClient
	// pick the right one via NewOpenAIClient / NewOpenRouterClient.
	var baseURL string
	if m.selectedProvider.Type == "openai-custom" {
		baseURL = strings.TrimSpace(m.inputs[fieldBaseURL].Value())
		if baseURL == "" {
			m.err = "Fill in the Base URL first, then press Tab to browse models."
			return m, nil
		}
	}
	// baseURL == "" for openai / openrouter is intentional:
	// NewOpenAIClient and NewOpenRouterClient override it in their constructors.

	m.err = ""
	m.modalLoading = true
	m.showModelModal = false
	m.modalFetchErr = ""
	m.modalList = nil
	m.inputs[fieldModel].Blur()

	return m, fetchModelsCmd(m.selectedProvider.Type, baseURL, token)
}

// updateModal handles keystrokes while the model picker modal is open.
func (m Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showModelModal = false
		m.inputs[fieldModel].Focus()

	case "up", "k":
		if m.modalCursor > 0 {
			m.modalCursor--
			if m.modalCursor < m.modalScrollOff {
				m.modalScrollOff--
			}
		}
	case "down", "j":
		if m.modalCursor < len(m.modalList)-1 {
			m.modalCursor++
			if m.modalCursor >= m.modalScrollOff+m.modalPageSize {
				m.modalScrollOff++
			}
		}
	case "enter":
		if len(m.modalList) > 0 {
			m.inputs[fieldModel].SetValue(m.modalList[m.modalCursor])
		}
		m.showModelModal = false
		m.inputs[fieldModel].Focus()

	// Allow re-fetching by pressing 'r' inside the modal
	case "r":
		m.showModelModal = false
		return m.openModelModal()
	}
	return m, nil
}

// ── Field navigation ──────────────────────────────────────────────────────────

func (m Model) advanceField() (tea.Model, tea.Cmd) {
	m.err = ""
	switch m.activeField {
	case fieldToken:
		if strings.TrimSpace(m.inputs[fieldToken].Value()) == "" {
			m.err = "API Token cannot be empty."
			return m, nil
		}
		m.inputs[fieldToken].Blur()
		m.activeField = fieldModel
		m.inputs[fieldModel].Focus()

	case fieldModel:
		if strings.TrimSpace(m.inputs[fieldModel].Value()) == "" {
			m.err = "Model cannot be empty."
			return m, nil
		}
		if m.selectedProvider.Type == "openai-custom" {
			m.inputs[fieldModel].Blur()
			m.activeField = fieldBaseURL
			m.inputs[fieldBaseURL].Focus()
		} else {
			return m.saveAndFinish()
		}

	case fieldBaseURL:
		if strings.TrimSpace(m.inputs[fieldBaseURL].Value()) == "" {
			m.err = "Base URL cannot be empty for a Custom provider."
			return m, nil
		}
		return m.saveAndFinish()
	}
	return m, nil
}

func (m Model) retreatField() (tea.Model, tea.Cmd) {
	switch m.activeField {
	case fieldModel:
		m.inputs[fieldModel].Blur()
		m.activeField = fieldToken
		m.inputs[fieldToken].Focus()
	case fieldBaseURL:
		m.inputs[fieldBaseURL].Blur()
		m.activeField = fieldModel
		m.inputs[fieldModel].Focus()
	}
	return m, nil
}

// ── Persist & finish ──────────────────────────────────────────────────────────

func (m Model) saveAndFinish() (tea.Model, tea.Cmd) {
	token := strings.TrimSpace(m.inputs[fieldToken].Value())
	model := strings.TrimSpace(m.inputs[fieldModel].Value())
	baseURL := strings.TrimSpace(m.inputs[fieldBaseURL].Value())

	if m.selectedProvider.BaseURL != "" && baseURL == "" {
		baseURL = m.selectedProvider.BaseURL
	}

	provCfg := config.ProviderConfig{
		Type:        m.selectedProvider.Type,
		DisplayName: m.selectedProvider.Name,
		Model:       model,
		BaseURL:     baseURL,
		MaxTokens:   4096,
		TopP:        1.0,
		Temperature: 0.7,
	}

	if err := config.SaveProvider(&provCfg); err != nil {
		m.err = fmt.Sprintf("Failed to save provider: %v", err)
		return m, nil
	}
	if err := config.SetSecret(provCfg.ID, token); err != nil {
		m.err = fmt.Sprintf("Failed to save token: %v", err)
		return m, nil
	}
	appCfg := &config.AppConfig{ActiveProviderID: provCfg.ID}
	if err := config.SaveAppConfig(appCfg); err != nil {
		m.err = fmt.Sprintf("Failed to save config: %v", err)
		return m, nil
	}

	return m, func() tea.Msg {
		return DoneMsg{Provider: provCfg, Token: token}
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	switch m.current {
	case viewProviderPick:
		return m.viewProviderPick()
	case viewForm:
		return m.viewForm()
	}
	return ""
}

// ── Provider pick sub-view ────────────────────────────────────────────────────

func (m Model) viewProviderPick() string {
	w := m.safeWidth()

	// ── Content ──
	title := centeredBanner()
	subtitle := centerLine("First-run setup wizard")
	step := stepStyle.Render("Step 1 of 2  —  Choose your provider")

	// Calculate natural width for centering and separator
	contentMaxWidth := lipgloss.Width(step)
	if contentMaxWidth < 44 {
		contentMaxWidth = 44
	}
	sep := dimStyle.Render(strings.Repeat("─", contentMaxWidth))

	var items strings.Builder
	for i, p := range AvailableProviders {
		prefix := "  "
		if i == m.providerCursor {
			prefix = cursorStyle.Render("→ ")
		}

		item := prefix + p.Name
		if i == m.providerCursor {
			items.WriteString(selectedItemStyle.Render(item))
		} else {
			items.WriteString(itemStyle.Render(item))
		}
		items.WriteRune('\n')
	}

	providerHeader := labelActiveStyle.Width(contentMaxWidth).Align(lipgloss.Center).Render("Available Providers")
	providerList := lipgloss.JoinVertical(lipgloss.Left,
		providerHeader,
		strings.TrimRight(items.String(), "\n"),
	)

	hint := hintStyle.Render("↑/↓  navigate    enter  select    ctrl+c  quit")

	// ── Assembly ──
	body := lipgloss.JoinVertical(lipgloss.Center,
		title,
		subtitle,
		"",
		step,
		sep,
		"",
		providerList,
		"",
		hint,
	)

	// Wrap in box and place in screen center
	box := boxStyle.Render(body)
	return lipgloss.Place(w, m.safeHeight(), lipgloss.Center, lipgloss.Center, box)
}

// ── Form sub-view ─────────────────────────────────────────────────────────────

func (m Model) viewForm() string {
	w := m.safeWidth()

	// ── Content ──
	title := centeredBanner()
	subtitle := centerLine("First-run setup wizard")
	step := stepStyle.Render("Step 2 of 2  —  Configure provider")

	// Calculate natural width for centering and separator
	contentMaxWidth := lipgloss.Width(step)
	if contentMaxWidth < 44 {
		contentMaxWidth = 44
	}
	sep := dimStyle.Render(strings.Repeat("─", contentMaxWidth))

	var rows []string

	// Provider (read-only)
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Provider"),
		successStyle.Render("✔ "+m.selectedProvider.Name),
	))

	// Token
	tokenLabel := labelStyle.Render("API Token")
	if m.activeField == fieldToken {
		tokenLabel = labelActiveStyle.Render("API Token")
	}
	tokenVal := m.inputs[fieldToken].View()
	if m.activeField != fieldToken && m.inputs[fieldToken].Value() != "" {
		tokenVal = successStyle.Render("✔ ") + dimStyle.Render("(set)")
	}
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, tokenLabel, tokenVal))

	// Model
	if m.activeField >= fieldModel || m.inputs[fieldToken].Value() != "" {
		modelLabel := labelStyle.Render("Agent model")
		if m.activeField == fieldModel {
			modelLabel = labelActiveStyle.Render("Agent model")
		}

		var modelVal string
		switch {
		case m.modalLoading:
			modelVal = accentStyle.Render("⟳ Fetching models…")
		case m.activeField != fieldModel && m.inputs[fieldModel].Value() != "":
			modelVal = successStyle.Render("✔ ") + inputValueStyle.Render(m.inputs[fieldModel].Value())
		default:
			modelVal = m.inputs[fieldModel].View()
		}

		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, modelLabel, modelVal))

		if m.activeField == fieldModel && !m.modalLoading {
			rows = append(rows, tipStyle.Render("  Tip: Press Tab to browse available models"))
		}
	}

	// Base URL (custom only)
	if m.selectedProvider.Type == "openai-custom" &&
		(m.activeField >= fieldBaseURL || m.inputs[fieldModel].Value() != "") {
		urlLabel := labelStyle.Render("Base URL")
		if m.activeField == fieldBaseURL {
			urlLabel = labelActiveStyle.Render("Base URL")
		}
		urlVal := m.inputs[fieldBaseURL].View()
		if m.activeField != fieldBaseURL && m.inputs[fieldBaseURL].Value() != "" {
			urlVal = successStyle.Render("✔ ") + inputValueStyle.Render(m.inputs[fieldBaseURL].Value())
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, urlLabel, urlVal))
	}

	// Error
	if m.err != "" {
		rows = append(rows, "", errorStyle.Render("✖ "+m.err))
	}

	var hintLine string
	if m.modalLoading {
		hintLine = "Fetching models from the API…  ctrl+c  quit"
	} else {
		hintLine = "enter/tab  next    shift+tab  back    esc  change provider    ctrl+c  quit"
	}

	formBody := lipgloss.JoinVertical(lipgloss.Left, rows...)
	hint := hintStyle.Render(hintLine)

	// ── Assembly ──
	body := lipgloss.JoinVertical(lipgloss.Center,
		title,
		subtitle,
		"",
		step,
		sep,
		"",
		formBody,
		"",
		hint,
	)

	box := boxStyle.Render(body)
	bg := lipgloss.Place(w, m.safeHeight(), lipgloss.Center, lipgloss.Center, box)

	if m.showModelModal {
		modal := m.renderModal()
		return lipgloss.Place(w, m.safeHeight(), lipgloss.Center, lipgloss.Center, modal)
	}

	return bg
}

// ── Model picker modal ────────────────────────────────────────────────────────

func (m Model) renderModal() string {
	maxWidth := m.safeWidth()
	w := common.Clamp(maxWidth-10, 30, 50)
	title := modalTitleStyle.Width(w).Render("✦ SELECT MODEL ✦")
	instruction := modalInstructionStyle.Width(w).Render("Choose a model for your provider")
	sepLen := common.Clamp(w-4, 5, 46)
	sepLine := dimStyle.Render(strings.Repeat("─", sepLen))
	sep := lipgloss.PlaceHorizontal(w, lipgloss.Center, sepLine)

	// Fetch error banner
	var errBanner string
	if m.modalFetchErr != "" {
		short := m.modalFetchErr
		if len(short) > 60 {
			short = short[:60] + "…"
		}
		errBanner = lipgloss.JoinVertical(lipgloss.Center,
			errorStyle.Width(w).Align(lipgloss.Center).Render("⚠ API error: "+short),
			dimStyle.Width(w).Align(lipgloss.Center).Render("Showing built-in model list instead"),
			"",
		)
	}

	var list strings.Builder
	lo := m.modalScrollOff
	hi := lo + m.modalPageSize
	if hi > len(m.modalList) {
		hi = len(m.modalList)
	}

	// Add up arrow if scrolled down
	if lo > 0 {
		list.WriteString(dimStyle.Render("       ▲ more items above") + "\n")
	} else {
		list.WriteString("\n")
	}

	for i := lo; i < hi; i++ {
		prefix := "  "
		if i == m.modalCursor {
			prefix = cursorStyle.Render("→ ")
		}

		item := prefix + m.modalList[i]
		if i == m.modalCursor {
			list.WriteString(selectedItemStyle.Render(item))
		} else {
			list.WriteString(itemStyle.Render(item))
		}
		list.WriteRune('\n')
	}

	// Add down arrow if more items below
	if hi < len(m.modalList) {
		list.WriteString(dimStyle.Render("       ▼ more items below") + "\n")
	} else {
		list.WriteString("\n")
	}

	var scrollHint string
	if len(m.modalList) > m.modalPageSize {
		scrollHint = lipgloss.PlaceHorizontal(w, lipgloss.Center,
			dimStyle.Render(fmt.Sprintf("%d / %d", m.modalCursor+1, len(m.modalList))),
		)
	}

	hint := lipgloss.PlaceHorizontal(w, lipgloss.Center,
		hintStyle.Render("enter  select    r  refresh    esc  close"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		instruction,
		sep,
		"",
		errBanner,
		strings.TrimRight(list.String(), "\n"),
		"",
		scrollHint,
		hint,
	)

	return modalStyle.Width(w).Render(content)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m Model) safeWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

func (m Model) safeHeight() int {
	if m.height > 0 {
		return m.height
	}
	return 24
}

func centeredBanner() string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.ColorCyan)).
		Render("✦  T E R A G E N  ✦")
}

func centerLine(s string) string {
	return subtitleStyle.Render(s)
}

// ── Entry point ───────────────────────────────────────────────────────────────

// Run launches the setup wizard and blocks until it completes.
func Run() (config.ProviderConfig, string, error) {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return config.ProviderConfig{}, "", err
	}

	providers, err := config.LoadProviders()
	if err != nil || len(providers) == 0 {
		return config.ProviderConfig{}, "", fmt.Errorf("setup did not save a provider")
	}

	saved := providers[len(providers)-1]
	token, _ := config.GetSecret(saved.ID)
	return saved, token, nil
}
