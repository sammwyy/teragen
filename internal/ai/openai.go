package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sammwy/teragen/internal/config"
)

type OpenAICustomClient struct {
	Config  config.ProviderConfig
	Token   string
	BaseURL string
}

func NewOpenAICustomClient(cfg config.ProviderConfig, token string, baseURL string) *OpenAICustomClient {
	if baseURL == "" {
		baseURL = cfg.BaseURL
	}
	return &OpenAICustomClient{
		Config:  cfg,
		Token:   token,
		BaseURL: baseURL,
	}
}

func NewOpenAIClient(cfg config.ProviderConfig, token string) *OpenAICustomClient {
	return NewOpenAICustomClient(cfg, token, "https://api.openai.com/v1")
}

func NewOpenRouterClient(cfg config.ProviderConfig, token string) *OpenAICustomClient {
	return NewOpenAICustomClient(cfg, token, "https://openrouter.ai/api/v1")
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *OpenAICustomClient) ChatCompletion(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return CompletionResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return CompletionResponse{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openAIResponse
		json.Unmarshal(body, &errResp)
		if errResp.Error.Message != "" {
			return CompletionResponse{}, fmt.Errorf("openai error: %s", errResp.Error.Message)
		}
		return CompletionResponse{}, fmt.Errorf("openai error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var aiResp openAIResponse
	if err := json.Unmarshal(body, &aiResp); err != nil {
		return CompletionResponse{}, err
	}

	if len(aiResp.Choices) == 0 {
		return CompletionResponse{}, fmt.Errorf("no choices returned")
	}

	return CompletionResponse{
		Content:      aiResp.Choices[0].Message.Content,
		ToolCalls:    aiResp.Choices[0].Message.ToolCalls,
		InputTokens:  aiResp.Usage.PromptTokens,
		OutputTokens: aiResp.Usage.CompletionTokens,
	}, nil
}

func (c *OpenAICustomClient) StreamCompletion(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error) {
	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)
	req.Stream = true

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamEvent)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			ch <- StreamEvent{Err: fmt.Errorf("openai error: status %d, body: %s", resp.StatusCode, string(body))}
			return
		}

		// Actually, let's use a more robust way to read SSE
		importBuf := ""

		for {
			line := make([]byte, 4096)
			n, err := resp.Body.Read(line)
			if err != nil {
				if err != io.EOF {
					ch <- StreamEvent{Err: err}
				}
				return
			}

			importBuf += string(line[:n])
			for {
				start := strings.Index(importBuf, "data: ")
				if start == -1 {
					break
				}
				end := strings.Index(importBuf[start:], "\n")
				if end == -1 {
					break
				}

				data := strings.TrimSpace(importBuf[start+6 : start+end])
				importBuf = importBuf[start+end+1:]

				if data == "[DONE]" {
					ch <- StreamEvent{Done: true}
					return
				}

				var aiResp openAIResponse
				if err := json.Unmarshal([]byte(data), &aiResp); err != nil {
					continue
				}

				if len(aiResp.Choices) > 0 || aiResp.Usage.TotalTokens > 0 {
					content := ""
					var toolCalls []ToolCall
					if len(aiResp.Choices) > 0 {
						content = aiResp.Choices[0].Delta.Content
						for _, tc := range aiResp.Choices[0].Delta.ToolCalls {
							toolCalls = append(toolCalls, ToolCall{
								Index: tc.Index,
								ID:    tc.ID,
								Type:  tc.Type,
								Function: ToolCallFunction{
									Name:      tc.Function.Name,
									Arguments: tc.Function.Arguments,
								},
							})
						}
					}

					tokens := aiResp.Usage.TotalTokens
					if tokens == 0 {
						tokens = aiResp.Usage.CompletionTokens
					}

					ch <- StreamEvent{
						Content:      content,
						ToolCalls:    toolCalls,
						InputTokens:  aiResp.Usage.PromptTokens,
						OutputTokens: aiResp.Usage.CompletionTokens,
					}
				}
			}
		}
	}()

	return ch, nil
}

// This mirrors the ChatCompletion pattern: the BaseURL is set once at
// construction (openai → api.openai.com, openrouter → openrouter.ai,
// custom → user-supplied). No branching needed here.
func (c *OpenAICustomClient) ListModels(ctx context.Context) ([]string, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("base URL is not configured for this provider")
	}

	url := fmt.Sprintf("%s/models", c.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openAIModelsResponse
		json.Unmarshal(body, &errResp)
		if errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var mr openAIModelsResponse
	if err := json.Unmarshal(body, &mr); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return filterChatModels(mr.Data), nil
}
