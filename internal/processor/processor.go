package processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/sammwy/teragen/internal/agent"
	"github.com/sammwy/teragen/internal/ai"
)

// CommandResult is the structured output of a command handler.
type CommandResult struct {
	Output string // text to display in the chat UI
}

// CommandHandler processes a command and returns a result string + error.
type CommandHandler func(ctx context.Context, a *agent.Agent, args []string) (CommandResult, error)

type Command struct {
	Name        string
	Description string
	Handler     CommandHandler
}

type Processor struct {
	Agent    *agent.Agent
	Commands map[string]Command
}

func NewProcessor(a *agent.Agent) *Processor {
	p := &Processor{
		Agent:    a,
		Commands: make(map[string]Command),
	}
	p.registerDefaultCommands()
	return p
}

func (p *Processor) Register(cmd Command) {
	p.Commands[cmd.Name] = cmd
}

// Process handles either a /command or blocks for a plain chat message (legacy).
// For streaming use Stream instead.
func (p *Processor) Process(ctx context.Context, a *agent.Agent, input string) (string, int, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", 0, nil
	}

	if strings.HasPrefix(input, "/") {
		parts := strings.Fields(input[1:])
		if len(parts) == 0 {
			return "", 0, nil
		}

		cmdName := parts[0]
		args := parts[1:]

		if cmd, ok := p.Commands[cmdName]; ok {
			result, err := cmd.Handler(ctx, a, args)
			if err != nil {
				return "", 0, err
			}
			return result.Output, 0, nil
		}
		return "", 0, fmt.Errorf("unknown command: /%s  —  type /help for a list", cmdName)
	}

	return "", 0, fmt.Errorf("use Stream for chat messages")
}

func (p *Processor) Stream(ctx context.Context, input string) (<-chan ai.StreamEvent, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	if strings.HasPrefix(input, "/") {
		return nil, fmt.Errorf("streaming not supported for commands")
	}

	return p.Agent.Stream(ctx, input)
}

func (p *Processor) registerDefaultCommands() {
	p.registerProviderCommands()

	p.Register(Command{
		Name:        "help",
		Description: "Show available commands",
		Handler: func(ctx context.Context, a *agent.Agent, args []string) (CommandResult, error) {
			var lines []string
			lines = append(lines, "Available commands:")
			for _, cmd := range p.Commands {
				lines = append(lines, fmt.Sprintf("  /%s — %s", cmd.Name, cmd.Description))
			}
			return CommandResult{Output: strings.Join(lines, "\n")}, nil
		},
	})
}
