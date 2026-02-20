package agent

import (
	"context"
	"fmt"
	"strings"
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
	Workspace      *workspace.Workspace
	ID             string
	ActiveChatID   string
	SystemPrompt   string
}

func NewAgent(session ...workspace.AgentSession) (*Agent, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, err
	}

	ws := workspace.NewWorkspace(".")
	if err := ws.EnsureDirectories(); err != nil {
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
		Evaluator:    evaluator.NewEvaluator(),
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
	userTokens := estimateTokens(input)
	a.History = append(a.History, ai.Message{
		Role:      "user",
		Content:   input,
		Tokens:    userTokens,
		Timestamp: now,
	})

	// Add partial assistant message for streaming
	assistantMsgIndex := len(a.History)
	a.History = append(a.History, ai.Message{
		Role:      "assistant",
		Content:   "",
		Timestamp: time.Now().Format(time.RFC3339),
	})

	// Prepare messages
	messages := []ai.Message{}
	if a.SystemPrompt != "" {
		messages = append(messages, ai.Message{
			Role:    "system",
			Content: a.SystemPrompt,
		})
	}

	// Filter history to send only role/content
	for _, m := range a.History {
		messages = append(messages, ai.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	req := ai.CompletionRequest{
		Messages:    messages[:len(messages)-1], // Exclude the empty assistant message
		Model:       a.ActiveProvider.Model,
		MaxTokens:   a.ActiveProvider.MaxTokens,
		Temperature: a.ActiveProvider.Temperature,
		TopP:        a.ActiveProvider.TopP,
		Stream:      true,
		StreamOptions: &ai.StreamOptions{
			IncludeUsage: true,
		},
	}

	rawCh, err := a.Client.StreamCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan ai.StreamEvent)
	go func() {
		defer close(ch)
		fullContent := ""
		totalTokens := 0

		for ev := range rawCh {
			if ev.Err != nil {
				ch <- ev
				return
			}

			fullContent += ev.Content
			if ev.Tokens > 0 {
				totalTokens = ev.Tokens
			}

			// Update the in-memory history
			a.History[assistantMsgIndex].Content = fullContent
			a.History[assistantMsgIndex].Tokens = totalTokens

			ch <- ev
		}

		// Final save to workspace
		if a.ActiveChatID == "" {
			a.ActiveChatID = a.Workspace.NextChatID()
		}

		// Calculate total session tokens
		sessionTotal := 0
		for _, m := range a.History {
			sessionTotal += m.Tokens
		}

		a.Workspace.SaveChat(&workspace.ChatSession{
			ID:          a.ActiveChatID,
			Messages:    a.History,
			TotalTokens: sessionTotal,
		})

		// Save agent list (this might be inefficient if done on every message,
		// but necessary to persist the ActiveChatID for newly created chats)
		agents, _ := a.Workspace.LoadAgents()
		found := false
		for i, sa := range agents {
			if sa.ID == a.ID { // We need an ID field in Agent struct too
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
	}()

	return ch, nil
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
