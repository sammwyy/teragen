package ai

import (
	"context"
	"sort"
	"strings"

	"github.com/sammwy/teragen/internal/config"
)

// filterChatModels filters a raw model ID list, removing models that are
// clearly not chat-completion capable (embeddings, moderation, TTS, image, etc.).
// Using a denylist is safer than an allowlist: new chat model families are
// included by default without any code changes.
func filterChatModels(data []struct {
	ID string `json:"id"`
}) []string {
	denySubstrings := []string{
		"embed", "moderat", "whisper", "tts", "dall-e",
		"davinci-002", "babbage-002", "text-search", "code-search",
		"similarity", "edit", "instruct", "vision", // vision-only variants handled separately
	}

	var out []string
	for _, m := range data {
		lower := strings.ToLower(m.ID)
		skip := false
		for _, d := range denySubstrings {
			if strings.Contains(lower, d) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, m.ID)
		}
	}
	sort.Strings(out)
	return out
}

// NewClientForSetup builds a temporary AIClient from partial configuration
// data collected during the setup wizard (no ID needed yet).
// This is the single place the wizard uses to talk to the API — it goes
// through the same NewClient factory, so any future provider automatically
// gets picked up.
func NewClientForSetup(providerType, baseURL, token string) (AIClient, error) {
	cfg := config.ProviderConfig{
		Type:    providerType,
		BaseURL: baseURL,
	}
	return NewClient(cfg, token)
}

// ListModelsForSetup is a convenience wrapper used by the setup TUI.
// It constructs a temporary client and calls ListModels on it.
func ListModelsForSetup(ctx context.Context, providerType, baseURL, token string) ([]string, error) {
	client, err := NewClientForSetup(providerType, baseURL, token)
	if err != nil {
		return nil, err
	}
	return client.ListModels(ctx)
}
