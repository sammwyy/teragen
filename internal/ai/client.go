package ai

import (
	"context"

	"github.com/sammwy/teragen/internal/config"
)

type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Tokens    int    `json:"tokens,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type CompletionRequest struct {
	Messages      []Message      `json:"messages"`
	Model         string         `json:"model"`
	MaxTokens     int            `json:"max_tokens"`
	Temperature   float64        `json:"temperature"`
	TopP          float64        `json:"top_p"`
	Stream        bool           `json:"stream"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

type CompletionResponse struct {
	Content     string
	TotalTokens int
}

type StreamEvent struct {
	Content string
	Done    bool
	Tokens  int
	Err     error
}

type AIClient interface {
	// ChatCompletion sends a chat request and returns the assistant reply and token usage.
	ChatCompletion(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// StreamCompletion sends a chat request and returns a channel of stream events.
	StreamCompletion(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error)

	// ListModels returns the models available for this provider.
	// The implementation should use the client's own BaseURL and Token;
	// no provider-specific branching should live outside the implementation.
	ListModels(ctx context.Context) ([]string, error)
}

// NewClient constructs the right AIClient implementation for cfg.Type.
// The provider-specific base URL is baked into each constructor so this
// factory never hard-codes URLs itself.
func NewClient(cfg config.ProviderConfig, token string) (AIClient, error) {
	switch cfg.Type {
	case "openai":
		return NewOpenAIClient(cfg, token), nil
	case "openrouter":
		return NewOpenRouterClient(cfg, token), nil
	case "openai-custom":
		return NewOpenAICustomClient(cfg, token, ""), nil
	default:
		// Treat any unknown type as a generic OpenAI-compatible custom endpoint.
		return NewOpenAICustomClient(cfg, token, ""), nil
	}
}
