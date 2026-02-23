package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sammwy/teragen/internal/ai"
	"github.com/sammwy/teragen/internal/config"
	"github.com/sammwy/teragen/internal/evaluator"
	"github.com/sammwy/teragen/internal/workspace"
)

type Agent struct {
	ActiveProvider *config.ProviderConfig
	Client         ai.AIClient
	Config         *config.AppConfig
	Evaluator      *evaluator.Evaluator
	History        []ai.Message
	Workspace      workspace.Workspace
	ID             string
	ActiveChatID   string
	SystemPrompt   string
	LastSnapshotID string
	mu             sync.Mutex
}

func NewAgent(ws workspace.Workspace, session ...workspace.AgentSession) (*Agent, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, err
	}

	var activeSession workspace.AgentSession
	if len(session) > 0 && session[0].ID != "" {
		activeSession = session[0]
	} else {
		agents, err := ws.LoadAgents()
		if err != nil {
			return nil, err
		}
		if len(agents) > 0 {
			activeSession = agents[0]
		} else {
			activeSession = workspace.AgentSession{ID: "1", ActiveChatID: "1"}
		}
	}

	history := []ai.Message{}
	if activeSession.ActiveChatID != "" {
		chat, _ := ws.LoadChat(activeSession.ActiveChatID)
		if chat != nil {
			history = chat.Messages
		}
	}

	sp, _ := config.LoadSystemPrompt(appCfg)

	agent := &Agent{
		Config:       appCfg,
		Evaluator:    evaluator.NewEvaluator(ws),
		History:      history,
		Workspace:    ws,
		ID:           activeSession.ID,
		ActiveChatID: activeSession.ActiveChatID,
		SystemPrompt: sp,
	}

	if appCfg.ActiveProviderID != "" {
		providers, _ := config.LoadProviders()
		for _, p := range providers {
			if p.ID == appCfg.ActiveProviderID {
				agent.SetActiveProvider(p)
				break
			}
		}
	}

	return agent, nil
}

func (a *Agent) SetActiveProvider(p config.ProviderConfig) error {
	token, err := config.GetSecret(p.ID)
	if err != nil {
		return fmt.Errorf("could not get token for provider %s: %v", p.ID, err)
	}

	client, err := ai.NewClient(p, token)
	if err != nil {
		return err
	}

	a.ActiveProvider = &p
	a.Client = client
	a.Config.ActiveProviderID = p.ID
	return config.SaveAppConfig(a.Config)
}

func (a *Agent) Stream(ctx context.Context, input string) (<-chan ai.StreamEvent, error) {
	if a.Client == nil {
		return nil, fmt.Errorf("no active provider selected. Use /provider to set one.")
	}

	now := time.Now().Format(time.RFC3339)
	a.mu.Lock()
	a.History = append(a.History, ai.Message{
		Role:        "user",
		Content:     input,
		InputTokens: estimateTokens(input),
		Timestamp:   now,
	})
	a.mu.Unlock()

	ch := make(chan ai.StreamEvent)

	// Set evaluator action callback
	a.Evaluator.OnAction = func(action string) {
		ch <- ai.StreamEvent{Action: action}
	}
	// Loop for tool call turns
	go func() {
		defer close(ch)

		if a.ActiveChatID == "" {
			a.ActiveChatID = a.Workspace.NextChatID()
		}

		for {
			// Prepare messages
			messages := []ai.Message{}
			if a.SystemPrompt != "" {
				messages = append(messages, ai.Message{
					Role:    "system",
					Content: a.SystemPrompt,
				})
			}
			for _, m := range a.History {
				messages = append(messages, ai.Message{
					Role:       m.Role,
					Content:    m.Content,
					ToolCalls:  m.ToolCalls,
					ToolCallID: m.ToolCallID,
				})
			}

			req := ai.CompletionRequest{
				Messages:    messages,
				Model:       a.ActiveProvider.Model,
				MaxTokens:   a.ActiveProvider.MaxTokens,
				Temperature: a.ActiveProvider.Temperature,
				TopP:        a.ActiveProvider.TopP,
				Stream:      true,
				StreamOptions: &ai.StreamOptions{
					IncludeUsage: true,
				},
				Tools: a.Evaluator.ToolRegistry.GetDefinitions(),
			}

			rawCh, err := a.Client.StreamCompletion(ctx, req)
			if err != nil {
				ch <- ai.StreamEvent{Err: err}
				return
			}

			fullContent := ""
			turnInputTokens := 0
			turnOutputTokens := 0
			var accumulatedToolCalls []ai.ToolCall

			triggerMsgIndex := 0
			assistantMsgIndex := 0

			a.mu.Lock()
			triggerMsgIndex = len(a.History) - 1
			assistantMsgIndex = len(a.History)
			a.History = append(a.History, ai.Message{
				Role:      "assistant",
				Content:   "",
				Timestamp: time.Now().Format(time.RFC3339),
			})
			a.mu.Unlock()

			for ev := range rawCh {
				if ev.Err != nil {
					ch <- ev
					return
				}

				fullContent += ev.Content
				if ev.InputTokens > 0 {
					turnInputTokens = ev.InputTokens
					a.History[triggerMsgIndex].InputTokens = turnInputTokens
				}
				if ev.OutputTokens > 0 {
					turnOutputTokens = ev.OutputTokens
				}

				// Accumulate tool calls
				for _, tc := range ev.ToolCalls {
					found := false
					for i, existing := range accumulatedToolCalls {
						if existing.Index == tc.Index {
							if tc.ID != "" {
								accumulatedToolCalls[i].ID = tc.ID
							}
							if tc.Type != "" {
								accumulatedToolCalls[i].Type = tc.Type
							}
							accumulatedToolCalls[i].Function.Name += tc.Function.Name
							accumulatedToolCalls[i].Function.Arguments += tc.Function.Arguments
							found = true
							break
						}
					}
					if !found {
						accumulatedToolCalls = append(accumulatedToolCalls, tc)
					}
				}

				// Update the in-memory history
				a.mu.Lock()
				a.History[assistantMsgIndex].Content = fullContent
				a.History[assistantMsgIndex].OutputTokens = turnOutputTokens
				a.History[assistantMsgIndex].ToolCalls = accumulatedToolCalls
				if turnInputTokens > 0 {
					a.History[triggerMsgIndex].InputTokens = turnInputTokens
				}
				a.mu.Unlock()

				ch <- ev
			}

			if len(accumulatedToolCalls) > 0 {
				// Start snapshot session for this chat if not exists
				if a.Evaluator.ActiveSnapshot == nil {
					a.Evaluator.ActiveSnapshot = a.Evaluator.StartSnapshotSession(a.ActiveChatID)
				}

				// Execute tool calls
				for _, call := range accumulatedToolCalls {
					// Emit starting feedback
					ch <- ai.StreamEvent{
						ToolName: call.Function.Name,
						ToolArgs: call.Function.Arguments,
					}

					res, err := a.Evaluator.ToolRegistry.Call(a.ID, call)
					if err != nil {
						res = fmt.Sprintf("Error: %v", err)
					}

					// Emit real-time feedback
					ch <- ai.StreamEvent{
						ToolResult: res,
						ToolName:   call.Function.Name,
						ToolArgs:   call.Function.Arguments,
					}

					a.mu.Lock()
					a.History = append(a.History, ai.Message{
						Role:       "tool",
						ToolCallID: call.ID,
						Content:    res,
						Timestamp:  time.Now().Format(time.RFC3339),
					})
					a.mu.Unlock()
				}
				// Continue loop for another LLM turn
			} else {
				// No more tools, we're done with this turn
				break
			}
		}

		// Final snapshot commit
		if a.Evaluator.ActiveSnapshot != nil {
			chat, _ := a.Workspace.LoadChat(a.ActiveChatID)
			prevSnapID := ""
			if chat != nil {
				prevSnapID = chat.LastSnapshotID
			}

			snap, err := a.Evaluator.ActiveSnapshot.Commit(prevSnapID)
			if err == nil {
				a.Evaluator.ActiveSnapshot = nil
				a.LastSnapshotID = snap.ID

				// Link snapshot to the last assistant message
				a.mu.Lock()
				for i := len(a.History) - 1; i >= 0; i-- {
					if a.History[i].Role == "assistant" {
						a.History[i].SnapshotID = snap.ID
						break
					}
				}
				a.mu.Unlock()

				// Emit snapshot to UI
				ch <- ai.StreamEvent{Snapshot: snap}
			}
		}

		// Final save to workspace
		inputTotal := 0
		outputTotal := 0
		a.mu.Lock()
		historyToSave := make([]ai.Message, len(a.History))
		copy(historyToSave, a.History)
		a.mu.Unlock()

		for _, m := range historyToSave {
			inputTotal += m.InputTokens
			outputTotal += m.OutputTokens
		}

		a.Workspace.SaveChat(&workspace.ChatSession{
			ID:                a.ActiveChatID,
			Messages:          historyToSave,
			TotalInputTokens:  inputTotal,
			TotalOutputTokens: outputTotal,
			LastSnapshotID:    a.LastSnapshotID,
		})

		// Save agent list
		agents, _ := a.Workspace.LoadAgents()
		found := false
		for i, sa := range agents {
			if sa.ID == a.ID {
				agents[i].ActiveChatID = a.ActiveChatID
				found = true
				break
			}
		}
		if !found {
			agents = append(agents, workspace.AgentSession{
				ID:           a.ID,
				ActiveChatID: a.ActiveChatID,
			})
		}
		a.Workspace.SaveAgents(agents)
		ch <- ai.StreamEvent{Done: true}
	}()

	return ch, nil
}

func (a *Agent) executeToolCalls(calls []ai.ToolCall) ([]ai.Message, error) {
	var results []ai.Message
	for _, call := range calls {
		res, err := a.Evaluator.ToolRegistry.Call(a.ID, call)
		if err != nil {
			res = fmt.Sprintf("Error: %v", err)
		}
		results = append(results, ai.Message{
			Role:       "tool",
			ToolCallID: call.ID,
			Content:    res,
			Timestamp:  time.Now().Format(time.RFC3339),
		})
	}
	return results, nil
}

func estimateTokens(text string) int {
	// Simple estimation: 1 token ~= 4 chars or ~0.75 words.
	// We'll use a middle ground: (word count * 1.3) + 1
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	return int(float64(words)*1.3) + 1
}
