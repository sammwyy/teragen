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

	p.Register(Command{
		Name:        "undo",
		Description: "Revert the last file changes made by the agent",
		Handler: func(ctx context.Context, a *agent.Agent, args []string) (CommandResult, error) {
			if a.ActiveChatID == "" {
				return CommandResult{Output: "No active chat"}, nil
			}

			chat, err := a.Workspace.LoadChat(a.ActiveChatID)
			if err != nil || chat.LastSnapshotID == "" {
				return CommandResult{Output: "No snapshots to undo"}, nil
			}

			snap, err := a.Workspace.LoadSnapshot(chat.LastSnapshotID)
			if err != nil {
				return CommandResult{Output: fmt.Sprintf("Error loading snapshot: %v", err)}, nil
			}

			err = a.Evaluator.ApplySnapshot(snap, true)
			if err != nil {
				return CommandResult{Output: fmt.Sprintf("Error applying undo: %v", err)}, nil
			}

			// Update last snapshot
			chat.LastSnapshotID = snap.PrevID
			a.Workspace.SaveChat(chat)
			a.LastSnapshotID = snap.PrevID

			return CommandResult{Output: fmt.Sprintf("Undone snapshot %s. Back to %s.", snap.ID, snap.PrevID)}, nil
		},
	})

	p.Register(Command{
		Name:        "redo",
		Description: "Redo the next file changes",
		Handler: func(ctx context.Context, a *agent.Agent, args []string) (CommandResult, error) {
			if a.ActiveChatID == "" {
				return CommandResult{Output: "No active chat"}, nil
			}

			chat, err := a.Workspace.LoadChat(a.ActiveChatID)
			if err != nil {
				return CommandResult{}, err
			}

			// Redo needs finding a snap where PrevID == LastSnapshotID
			snaps, err := a.Workspace.ListSnapshots(a.ActiveChatID)
			if err != nil {
				return CommandResult{Output: "Error listing snapshots"}, nil
			}

			var nextSnapID string
			for _, s := range snaps {
				if s.PrevID == chat.LastSnapshotID {
					nextSnapID = s.ID
					break
				}
			}

			if nextSnapID == "" {
				return CommandResult{Output: "Nothing to redo"}, nil
			}

			snap, _ := a.Workspace.LoadSnapshot(nextSnapID)
			err = a.Evaluator.ApplySnapshot(snap, false)
			if err != nil {
				return CommandResult{Output: fmt.Sprintf("Error applying redo: %v", err)}, nil
			}

			chat.LastSnapshotID = snap.ID
			a.Workspace.SaveChat(chat)
			a.LastSnapshotID = snap.ID

			return CommandResult{Output: fmt.Sprintf("Redone snapshot %s", snap.ID)}, nil
		},
	})
}
