package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
	"solo_quest_backend/pkg/logger"
)

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature     *float32 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
}

type geminiRequest struct {
	SystemInstruction *geminiSystemInstruction `json:"system_instruction,omitempty"`
	Contents          []geminiContent          `json:"contents"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiResponse struct {
	Candidates   []geminiCandidate `json:"candidates"`
	ModelVersion string            `json:"modelVersion"`
}

type GeminiClient struct {
	config *Config
	client *http.Client
}

func NewGeminiClient(cfg *Config) (*GeminiClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	// Hardcoded 600s ceiling; actual wait is governed by the caller's context
	// deadline. See AIRequestTimeout.
	return &GeminiClient{
		config: cfg,
		client: &http.Client{
			Timeout: AIRequestTimeout,
		},
	}, nil
}

func (c *GeminiClient) GenerateText(ctx context.Context, req GenerateTextRequest) (*GenerateTextResponse, error) {
	if !c.config.Enabled {
		return &GenerateTextResponse{Text: "AI is disabled"}, nil
	}
	if c.config.GeminiAPIKey == "" {
		return nil, fmt.Errorf("missing Gemini API key")
	}

	body := geminiRequest{
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: req.UserPrompt}}},
		},
	}

	if req.SystemPrompt != "" {
		body.SystemInstruction = &geminiSystemInstruction{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	genCfg := &geminiGenerationConfig{}
	if req.Temperature != 0 {
		t := req.Temperature
		genCfg.Temperature = &t
	}
	if req.MaxTokens > 0 {
		maxT := req.MaxTokens
		genCfg.MaxOutputTokens = &maxT
	}
	if genCfg.Temperature != nil || genCfg.MaxOutputTokens != nil {
		body.GenerationConfig = genCfg
	}

	payloadBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	u := c.config.GetGeminiURL()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	startTime := time.Now()
	resp, err := c.client.Do(httpReq)
	latency := time.Since(startTime)

	if err != nil {
		logger.L.Error("Gemini request failed",
			zap.String("model", c.config.GeminiModel),
			zap.Duration("latency", latency),
			zap.Error(err),
		)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		logger.L.Error("Gemini request returned non-200",
			zap.String("model", c.config.GeminiModel),
			zap.Int("status_code", resp.StatusCode),
			zap.Duration("latency", latency),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("Gemini API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var gemResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gemResp); err != nil {
		return nil, fmt.Errorf("failed to decode Gemini response: %w", err)
	}

	if len(gemResp.Candidates) == 0 {
		return nil, fmt.Errorf("empty candidates returned from Gemini")
	}

	candidate := gemResp.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("empty content parts in Gemini response")
	}

	text := candidate.Content.Parts[0].Text
	if text == "" {
		return nil, fmt.Errorf("empty text in Gemini response: finish_reason=%s", candidate.FinishReason)
	}

	model := gemResp.ModelVersion
	if model == "" {
		model = c.config.GeminiModel
	}

	logger.L.Info("Gemini request succeeded",
		zap.String("model", model),
		zap.Duration("latency", latency),
		zap.String("finish_reason", candidate.FinishReason),
		zap.Int("content_len", len(text)),
	)

	return &GenerateTextResponse{
		Text:         text,
		Model:        model,
		LatencyMs:    int(latency.Milliseconds()),
		FinishReason: candidate.FinishReason,
	}, nil
}
