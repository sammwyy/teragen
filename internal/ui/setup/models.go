package setup

// Provider describes a supported AI backend.
type Provider struct {
	Name    string // Display name shown in the picker
	Type    string // Internal type string used when saving ProviderConfig
	BaseURL string // Non-empty for non-OpenAI providers
}

// AvailableProviders is the ordered list shown in the first setup step.
var AvailableProviders = []Provider{
	{Name: "OpenAI", Type: "openai", BaseURL: "https://api.openai.com/v1"},
	{Name: "OpenRouter", Type: "openrouter", BaseURL: "https://openrouter.ai/api/v1"},
	{Name: "Custom (OpenAI Compatible)", Type: "openai-custom", BaseURL: ""},
}

// defaultModels lists commonly-used models for each provider type so the user
// can pick from a Tab-modal instead of typing the model name manually.
var defaultModels = map[string][]string{
	"openai": {
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"o1",
		"o1-mini",
		"o3-mini",
	},
	"openrouter": {
		"openai/gpt-4o",
		"openai/gpt-4o-mini",
		"anthropic/claude-3.5-sonnet",
		"anthropic/claude-3-haiku",
		"google/gemini-2.0-flash-001",
		"google/gemini-flash-1.5",
		"meta-llama/llama-3.3-70b-instruct",
		"mistralai/mistral-7b-instruct",
		"deepseek/deepseek-r1",
	},
	"openai-custom": {
		"custom-model",
		"gpt-4o",
		"gpt-4",
		"gpt-3.5-turbo",
		"llama3",
		"mistral",
	},
}

// ModelsFor returns the suggested model list for a given provider type.
func ModelsFor(providerType string) []string {
	if list, ok := defaultModels[providerType]; ok {
		return list
	}
	return []string{"gpt-4o", "gpt-3.5-turbo"}
}
