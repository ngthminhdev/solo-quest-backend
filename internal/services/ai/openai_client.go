package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
	"solo_quest_backend/pkg/logger"
)

type chatMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature *float32      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type usageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatCompletionsResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   *usageInfo   `json:"usage,omitempty"`
}

type OpenAIClient struct {
	config *Config
	client *http.Client
}

func NewOpenAIClient(cfg *Config) (*OpenAIClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	// Hardcoded 600s ceiling; actual wait is governed by the caller's context
	// deadline (worker derives a 600s AI context). Avoids prematurely cutting
	// slow AI calls at the previous short env timeout.
	return &OpenAIClient{
		config: cfg,
		client: &http.Client{
			Timeout: AIRequestTimeout,
		},
	}, nil
}

func (c *OpenAIClient) GenerateText(ctx context.Context, req GenerateTextRequest) (*GenerateTextResponse, error) {
	if !c.config.Enabled {
		return &GenerateTextResponse{
			Text: "AI is disabled",
		}, nil
	}

	if c.config.APIKey == "" {
		return nil, fmt.Errorf("missing AI API key")
	}

	messages := []chatMessage{}
	if req.SystemPrompt != "" {
		messages = append(messages, chatMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}
	messages = append(messages, chatMessage{
		Role:    "user",
		Content: req.UserPrompt,
	})

	var temp *float32
	if req.Temperature != 0 {
		temp = &req.Temperature
	}
	var maxT *int
	if req.MaxTokens > 0 {
		maxT = &req.MaxTokens
	}

	payload := chatCompletionsRequest{
		Model:       c.config.Model,
		Messages:    messages,
		Temperature: temp,
		MaxTokens:   maxT,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	u := c.config.GetChatCompletionsURL()
	parsedURL, _ := url.Parse(u)
	host := ""
	if parsedURL != nil {
		host = parsedURL.Host
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	startTime := time.Now()
	resp, err := c.client.Do(httpReq)
	latency := time.Since(startTime)

	if err != nil {
		logger.L.Error("AI request failed",
			zap.String("model", c.config.Model),
			zap.String("host", host),
			zap.Duration("latency", latency),
			zap.Error(err),
		)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.L.Error("AI request returned non-200 status",
			zap.String("model", c.config.Model),
			zap.String("host", host),
			zap.Int("status_code", resp.StatusCode),
			zap.Duration("latency", latency),
		)
		return nil, fmt.Errorf("API request failed with status code %d", resp.StatusCode)
	}

	var chatResp chatCompletionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		logger.L.Error("AI request failed to decode response",
			zap.String("model", c.config.Model),
			zap.String("host", host),
			zap.Duration("latency", latency),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to decode response payload: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		logger.L.Error("AI request returned empty choices",
			zap.String("model", c.config.Model),
			zap.String("host", host),
			zap.Duration("latency", latency),
		)
		return nil, fmt.Errorf("empty choices returned from AI client")
	}

	respModel := chatResp.Model
	if respModel == "" {
		respModel = c.config.Model
	}

	choice := chatResp.Choices[0]
	content := choice.Message.Content
	reasoningContentLen := len(choice.Message.ReasoningContent)

	// Check if content is empty
	if content == "" {
		logger.L.Error("AI response content is empty",
			zap.String("model", respModel),
			zap.String("host", host),
			zap.Duration("latency", latency),
			zap.String("finish_reason", choice.FinishReason),
			zap.Int("choices_count", len(chatResp.Choices)),
			zap.Int("content_len", 0),
			zap.Int("reasoning_content_len", reasoningContentLen),
		)
		return nil, fmt.Errorf("AI response content is empty: model=%s finish_reason=%s choices=%d content_len=0 reasoning_content_len=%d",
			respModel, choice.FinishReason, len(chatResp.Choices), reasoningContentLen)
	}

	logger.L.Info("AI request succeeded",
		zap.String("model", respModel),
		zap.String("host", host),
		zap.Duration("latency", latency),
		zap.String("finish_reason", choice.FinishReason),
		zap.Int("content_len", len(content)),
	)

	return &GenerateTextResponse{
		Text:         content,
		Model:        respModel,
		LatencyMs:    int(latency.Milliseconds()),
		FinishReason: choice.FinishReason,
	}, nil
}
