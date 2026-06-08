package ai

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Enabled             bool
	APIKey              string
	Model               string
	BaseURL             string
	TimeoutSeconds      int
	QuestTimeoutSeconds int // For quest generation, defaults to TimeoutSeconds
	QuestMaxTokens      int // For quest generation, defaults to 12000
}

func LoadConfig() *Config {
	enabled := false
	if val, ok := os.LookupEnv("AI_ENABLED"); ok {
		enabled = (strings.ToLower(val) == "true" || val == "1")
	}

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

	timeoutSec := 10
	if val, ok := os.LookupEnv("AI_TIMEOUT_SECONDS"); ok {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			timeoutSec = t
		}
	}

	// Quest-specific timeout, defaults to general timeout
	questTimeoutSec := timeoutSec
	if val, ok := os.LookupEnv("AI_QUEST_TIMEOUT_SECONDS"); ok {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			questTimeoutSec = t
		}
	}

	// Quest-specific max tokens, defaults to 12000 for reasoning models
	questMaxTokens := 32000
	if val, ok := os.LookupEnv("AI_QUEST_MAX_TOKENS"); ok {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			questMaxTokens = t
		}
	}

	return &Config{
		Enabled:             enabled,
		APIKey:              apiKey,
		Model:               model,
		BaseURL:             baseURL,
		TimeoutSeconds:      timeoutSec,
		QuestTimeoutSeconds: questTimeoutSec,
		QuestMaxTokens:      questMaxTokens,
	}
}

func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
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
	return nil
}

func (c *Config) GetChatCompletionsURL() string {
	return c.BaseURL + "/chat/completions"
}
