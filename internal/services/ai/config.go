package ai

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ProviderOpenAI = "openai"
	ProviderGemini = "gemini"
)

// AIRequestTimeout is the hard HTTP-client ceiling for AI provider requests.
// It is intentionally hardcoded (not env-driven) and large so a slow
// OpenAI-compatible endpoint has room to respond; the actual per-call wait is
// governed by the caller's request/worker context deadline (e.g. the quest
// generation worker derives a 600s AI context). This replaces the previous
// short (≤30s) client timeout that was cutting AI calls prematurely.
const AIRequestTimeout = 600 * time.Second

type Config struct {
	Enabled  bool
	Provider string // "openai" (default) or "gemini"

	// OpenAI / OpenAI-compatible
	APIKey  string
	Model   string
	BaseURL string

	// Gemini
	GeminiAPIKey string
	GeminiModel  string

	// Shared
	TimeoutSeconds      int
	QuestTimeoutSeconds int
	QuestMaxTokens      int
}

func LoadConfig() *Config {
	enabled := false
	if val, ok := os.LookupEnv("AI_ENABLED"); ok {
		enabled = (strings.ToLower(val) == "true" || val == "1")
	}

	provider := strings.ToLower(os.Getenv("AI_PROVIDER"))
	if provider == "" {
		provider = ProviderOpenAI
	}

	// OpenAI / OpenAI-compatible
	apiKey := os.Getenv("AI_API_KEY")
	model := os.Getenv("AI_MODEL")
	if model == "" {
		model = "mimo-v2.5-pro"
	}
	baseURL := os.Getenv("AI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Gemini
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	geminiModel := os.Getenv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-2.5-flash"
	}

	timeoutSec := 10
	if val, ok := os.LookupEnv("AI_TIMEOUT_SECONDS"); ok {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			timeoutSec = t
		}
	}

	questTimeoutSec := timeoutSec
	if val, ok := os.LookupEnv("AI_QUEST_TIMEOUT_SECONDS"); ok {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			questTimeoutSec = t
		}
	}

	questMaxTokens := 32000
	if val, ok := os.LookupEnv("AI_QUEST_MAX_TOKENS"); ok {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			questMaxTokens = t
		}
	}

	return &Config{
		Enabled:             enabled,
		Provider:            provider,
		APIKey:              apiKey,
		Model:               model,
		BaseURL:             baseURL,
		GeminiAPIKey:        geminiAPIKey,
		GeminiModel:         geminiModel,
		TimeoutSeconds:      timeoutSec,
		QuestTimeoutSeconds: questTimeoutSec,
		QuestMaxTokens:      questMaxTokens,
	}
}

func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	switch c.Provider {
	case ProviderGemini:
		if c.GeminiAPIKey == "" {
			return fmt.Errorf("GEMINI_API_KEY is required when AI_PROVIDER=gemini")
		}
	default: // openai
		if c.APIKey == "" {
			return fmt.Errorf("AI_API_KEY is required when AI_ENABLED is true")
		}
		if c.BaseURL == "" {
			return fmt.Errorf("AI_BASE_URL is required when AI_ENABLED is true")
		}
		u, err := url.Parse(c.BaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("AI_BASE_URL is invalid: %w", err)
		}
	}
	return nil
}

// GetChatCompletionsURL returns the OpenAI-compatible chat completions endpoint.
func (c *Config) GetChatCompletionsURL() string {
	return c.BaseURL + "/chat/completions"
}

// GetGeminiURL returns the Gemini generateContent endpoint with the API key embedded.
func (c *Config) GetGeminiURL() string {
	return fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.GeminiModel, c.GeminiAPIKey,
	)
}
