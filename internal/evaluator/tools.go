package evaluator

import (
	"encoding/json"
	"fmt"

	"github.com/sammwy/teragen/internal/ai"
)

type ToolHandler func(agentID string, args map[string]any) (string, error)

type ToolEntry struct {
	Definition ai.Tool
	Handler    ToolHandler
}

type ToolRegistry struct {
	tools map[string]ToolEntry
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolEntry),
	}
}

func (r *ToolRegistry) Register(name, description string, parameters any, handler ToolHandler) {
	r.tools[name] = ToolEntry{
		Definition: ai.Tool{
			Type: "function",
			Function: ai.Function{
				Name:        name,
				Description: description,
				Parameters:  parameters,
			},
		},
		Handler: handler,
	}
}

func (r *ToolRegistry) GetDefinitions() []ai.Tool {
	var defs []ai.Tool
	for _, t := range r.tools {
		defs = append(defs, t.Definition)
	}
	return defs
}

func (r *ToolRegistry) Call(agentID string, call ai.ToolCall) (string, error) {
	entry, ok := r.tools[call.Function.Name]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", call.Function.Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	return entry.Handler(agentID, args)
}
