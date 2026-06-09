package ai

import "fmt"

// NewClient creates an AI client based on cfg.Provider.
// Defaults to OpenAI if provider is unset or unrecognised.
func NewClient(cfg *Config) (Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	switch cfg.Provider {
	case ProviderGemini:
		return NewGeminiClient(cfg)
	default:
		return NewOpenAIClient(cfg)
	}
}
