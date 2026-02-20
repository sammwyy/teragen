package main

import (
	"fmt"
	"os"

	"github.com/sammwy/teragen/internal/agent"
	"github.com/sammwy/teragen/internal/config"
	"github.com/sammwy/teragen/internal/core"
	"github.com/sammwy/teragen/internal/processor"
	"github.com/sammwy/teragen/internal/ui/chat"
	"github.com/sammwy/teragen/internal/ui/setup"
	"github.com/sammwy/teragen/internal/workspace"
)

func main() {
	// ── First-run detection ──────────────────────────────────────────────────
	// If config.json doesn't exist (or has no active provider), run the setup
	// wizard before starting the main chat UI.
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if appCfg.ActiveProviderID == "" {
		// No provider configured → run the first-run wizard.
		_, _, err := setup.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Setup error: %v\n", err)
			os.Exit(1)
		}
	}

	ws := workspace.NewWorkspace(".")
	sessions, err := ws.LoadAgents()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading agents: %v\n", err)
		os.Exit(1)
	}

	activeAgentID := "1"
	if len(sessions) > 0 {
		activeAgentID = sessions[0].ID
	}

	cr := core.NewCore(nil) // We'll set the processor properly
	for _, s := range sessions {
		ag, err := agent.NewAgent(s)
		if err != nil {
			continue
		}
		cr.RegisterAgent(s.ID, ag)
	}

	// We need a processor that knows about the "active" agent?
	// Actually, the current Processor struct has a single Agent pointer.
	// This might need a refactor too if Processor is shared.
	// But according to the user's plan: "el core es el que maneja la instancia del agente...
	// la tui tiene el agent id y se suscribe".
	// The Processor belongs to the Core.

	// Let's ensure Core.Processor is initialized correctly.
	// For now, the Processor in Core is used for commands.
	// Command handlers usually take the agent they are operating on as an argument.
	// Look at internal/processor/processor.go: type CommandHandler func(...) (... , *agent.Agent, ...)

	// So we can just create a dummy processor and Core will pass the right agent.
	proc := processor.NewProcessor(nil)
	cr.Processor = proc

	tui := chat.NewUI(cr, activeAgentID)

	if err := tui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}
}
