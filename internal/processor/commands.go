package processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/sammwy/teragen/internal/agent"
	"github.com/sammwy/teragen/internal/ai"
	"github.com/sammwy/teragen/internal/config"
	"github.com/sammwy/teragen/internal/workspace"
)

func (p *Processor) registerProviderCommands() {
	p.Register(Command{
		Name:        "provider",
		Description: "Manage providers: /provider <list|toggle|remove|set-token> [args]",
		Handler: func(ctx context.Context, a *agent.Agent, args []string) (CommandResult, error) {
			if len(args) == 0 {
				return CommandResult{}, fmt.Errorf("usage: /provider <list|toggle|remove|set-token>")
			}

			var lines []string
			sub := args[0]

			switch sub {
			case "list":
				providers, _ := config.LoadProviders()
				lines = append(lines, "Registered providers:")
				for _, pr := range providers {
					activeMark := "  "
					if a.ActiveProvider != nil && a.ActiveProvider.ID == pr.ID {
						activeMark = "✔ "
					}
					shortID := pr.ID
					if len(shortID) > 8 {
						shortID = shortID[:8]
					}
					lines = append(lines, fmt.Sprintf("%s[%s] %s (%s) — %s", activeMark, shortID, pr.DisplayName, pr.Type, pr.Model))
				}

			case "toggle":
				if len(args) < 2 {
					return CommandResult{}, fmt.Errorf("usage: /provider toggle <id>")
				}
				id := args[1]
				providers, _ := config.LoadProviders()
				for _, pr := range providers {
					if strings.HasPrefix(pr.ID, id) {
						if err := a.SetActiveProvider(pr); err != nil {
							return CommandResult{}, err
						}
						lines = append(lines, fmt.Sprintf("Active provider set to: %s (%s)", pr.DisplayName, pr.Model))
						return CommandResult{Output: strings.Join(lines, "\n")}, nil
					}
				}
				return CommandResult{}, fmt.Errorf("provider not found: %s", id)

			case "remove":
				if len(args) < 2 {
					return CommandResult{}, fmt.Errorf("usage: /provider remove <id>")
				}
				if err := config.RemoveProvider(args[1]); err != nil {
					return CommandResult{}, err
				}
				lines = append(lines, fmt.Sprintf("Provider %s removed.", args[1]))

			case "set-token":
				if len(args) < 3 {
					return CommandResult{}, fmt.Errorf("usage: /provider set-token <id> <token>")
				}
				if err := config.SetSecret(args[1], args[2]); err != nil {
					return CommandResult{}, err
				}
				lines = append(lines, fmt.Sprintf("Token stored securely for provider %s.", args[1]))

			default:
				return CommandResult{}, fmt.Errorf("unknown subcommand: %s", sub)
			}

			return CommandResult{Output: strings.Join(lines, "\n")}, nil
		},
	})

	p.Register(Command{
		Name:        "model",
		Description: "Set model for active provider: /model <name>",
		Handler: func(ctx context.Context, a *agent.Agent, args []string) (CommandResult, error) {
			if a.ActiveProvider == nil {
				return CommandResult{}, fmt.Errorf("no active provider selected")
			}
			if len(args) == 0 {
				return CommandResult{Output: fmt.Sprintf("Current model: %s", a.ActiveProvider.Model)}, nil
			}
			a.ActiveProvider.Model = args[0]
			if err := config.SaveProvider(a.ActiveProvider); err != nil {
				return CommandResult{}, err
			}
			return CommandResult{Output: fmt.Sprintf("Model changed to: %s", args[0])}, nil
		},
	})
	p.Register(Command{
		Name:        "clear",
		Description: "Clear visual chat history",
		Handler: func(ctx context.Context, a *agent.Agent, args []string) (CommandResult, error) {
			return CommandResult{Output: "__CLEAR_VISUAL__"}, nil
		},
	})

	p.Register(Command{
		Name:        "chat",
		Description: "Manage chat: /chat <clear|new|switch|delete|copy> [args]",
		Handler: func(ctx context.Context, a *agent.Agent, args []string) (CommandResult, error) {
			if len(args) == 0 {
				return CommandResult{}, fmt.Errorf("usage: /chat <clear|new|switch|delete|copy>")
			}
			sub := args[0]
			switch sub {
			case "clear":
				a.History = nil
				if a.ActiveChatID != "" {
					a.Workspace.SaveChat(&workspace.ChatSession{
						ID:       a.ActiveChatID,
						Messages: nil,
					})
				}
				return CommandResult{Output: "__CLEAR_INTERNAL__"}, nil

			case "new":
				newID := a.Workspace.NextChatID()
				a.ActiveChatID = newID
				a.History = nil
				a.Workspace.SaveChat(&workspace.ChatSession{ID: newID, Messages: nil})
				// Update agents
				agents, _ := a.Workspace.LoadAgents()
				if len(agents) > 0 {
					agents[0].ActiveChatID = newID
					a.Workspace.SaveAgents(agents)
				}
				return CommandResult{Output: "__CHAT_SWITCHED__" + newID}, nil

			case "switch":
				if len(args) < 2 {
					return CommandResult{}, fmt.Errorf("usage: /chat switch <id>")
				}
				id := args[1]
				chat, err := a.Workspace.LoadChat(id)
				if err != nil {
					return CommandResult{}, fmt.Errorf("chat not found: %s", id)
				}
				a.ActiveChatID = id
				a.History = chat.Messages
				// Update agents
				agents, _ := a.Workspace.LoadAgents()
				if len(agents) > 0 {
					agents[0].ActiveChatID = id
					a.Workspace.SaveAgents(agents)
				}
				return CommandResult{Output: "__CHAT_SWITCHED__" + id}, nil

			case "delete":
				if len(args) < 2 {
					return CommandResult{}, fmt.Errorf("usage: /chat delete <id>")
				}
				id := args[1]
				if err := a.Workspace.DeleteChat(id); err != nil {
					return CommandResult{}, err
				}
				return CommandResult{Output: fmt.Sprintf("Chat %s deleted.", id)}, nil

			case "copy":
				if len(args) < 2 {
					return CommandResult{}, fmt.Errorf("usage: /chat copy <new_id>")
				}
				newID := args[1]
				// Save current first
				if a.ActiveChatID != "" {
					a.Workspace.SaveChat(&workspace.ChatSession{ID: a.ActiveChatID, Messages: a.History})
				}
				// Copy to new
				newHistory := make([]ai.Message, len(a.History))
				copy(newHistory, a.History)
				err := a.Workspace.SaveChat(&workspace.ChatSession{ID: newID, Messages: newHistory})
				if err != nil {
					return CommandResult{}, err
				}
				return CommandResult{Output: fmt.Sprintf("Chat copied to %s.", newID)}, nil

			default:
				return CommandResult{}, fmt.Errorf("unknown chat subcommand: %s", sub)
			}
		},
	})

	p.Register(Command{
		Name:        "settings",
		Description: "Manage settings: /settings <key> <value>",
		Handler: func(ctx context.Context, a *agent.Agent, args []string) (CommandResult, error) {
			if len(args) == 0 {
				return CommandResult{Output: "Available settings: shell"}, nil
			}

			key := args[0]
			if len(args) < 2 {
				// Show current value
				switch key {
				case "shell":
					return CommandResult{Output: fmt.Sprintf("Current shell: %s", a.Config.ActiveShell)}, nil
				default:
					return CommandResult{}, fmt.Errorf("unknown setting: %s", key)
				}
			}

			value := args[1]
			switch key {
			case "shell":
				found := false
				var validShells []string
				for _, s := range a.Config.AvailableShells {
					validShells = append(validShells, s.ID)
					if s.ID == value {
						found = true
						break
					}
				}

				if !found {
					return CommandResult{}, fmt.Errorf("invalid shell: %s. Valid values: %s", value, strings.Join(validShells, ", "))
				}

				a.Config.ActiveShell = value
				if err := config.SaveAppConfig(a.Config); err != nil {
					return CommandResult{}, err
				}

				// Refresh system prompt with new shell
				sp, _ := config.LoadSystemPrompt(a.Config)
				a.SystemPrompt = sp

				return CommandResult{Output: fmt.Sprintf("Shell updated to: %s", value)}, nil

			default:
				return CommandResult{}, fmt.Errorf("unknown setting: %s", key)
			}
		},
	})
}
