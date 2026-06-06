package ai_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/pkg/logger"

	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	logger.InitForTest()
	// Load root env if running in subfolders
	_ = godotenv.Load("../../../.env")
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load(".env")
	os.Exit(m.Run())
}

func TestConfig_DefaultsAndEnv(t *testing.T) {
	origEnabled := os.Getenv("AI_ENABLED")
	origAPIKey := os.Getenv("AI_API_KEY")
	origModel := os.Getenv("AI_MODEL")
	origBaseURL := os.Getenv("AI_BASE_URL")
	origTimeout := os.Getenv("AI_TIMEOUT_SECONDS")

	os.Unsetenv("AI_ENABLED")
	os.Unsetenv("AI_API_KEY")
	os.Unsetenv("AI_MODEL")
	os.Unsetenv("AI_BASE_URL")
	os.Unsetenv("AI_TIMEOUT_SECONDS")

	t.Cleanup(func() {
		if origEnabled != "" {
			os.Setenv("AI_ENABLED", origEnabled)
		} else {
			os.Unsetenv("AI_ENABLED")
		}
		if origAPIKey != "" {
			os.Setenv("AI_API_KEY", origAPIKey)
		} else {
			os.Unsetenv("AI_API_KEY")
		}
		if origModel != "" {
			os.Setenv("AI_MODEL", origModel)
		} else {
			os.Unsetenv("AI_MODEL")
		}
		if origBaseURL != "" {
			os.Setenv("AI_BASE_URL", origBaseURL)
		} else {
			os.Unsetenv("AI_BASE_URL")
		}
		if origTimeout != "" {
			os.Setenv("AI_TIMEOUT_SECONDS", origTimeout)
		} else {
			os.Unsetenv("AI_TIMEOUT_SECONDS")
		}
	})

	cfg := ai.LoadConfig()
	if cfg.Enabled != false {
		t.Error("expected default Enabled to be false")
	}
	if cfg.Model != "mimo-v2.5-pro" {
		t.Errorf("expected default Model to be 'mimo-v2.5-pro', got '%s'", cfg.Model)
	}
	if cfg.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default BaseURL to be 'https://api.openai.com/v1', got '%s'", cfg.BaseURL)
	}
	if cfg.TimeoutSeconds != 10 {
		t.Errorf("expected default TimeoutSeconds to be 10, got %d", cfg.TimeoutSeconds)
	}

	os.Setenv("AI_ENABLED", "true")
	os.Setenv("AI_API_KEY", "custom-key")
	os.Setenv("AI_MODEL", "custom-model")
	os.Setenv("AI_BASE_URL", "https://custom-api.com/v1/")
	os.Setenv("AI_TIMEOUT_SECONDS", "5")

	cfg2 := ai.LoadConfig()
	if cfg2.Enabled != true {
		t.Error("expected Enabled to be true")
	}
	if cfg2.APIKey != "custom-key" {
		t.Errorf("expected APIKey to be 'custom-key', got '%s'", cfg2.APIKey)
	}
	if cfg2.Model != "custom-model" {
		t.Errorf("expected Model to be 'custom-model', got '%s'", cfg2.Model)
	}
	if cfg2.BaseURL != "https://custom-api.com/v1" {
		t.Errorf("expected BaseURL normalized without trailing slash, got '%s'", cfg2.BaseURL)
	}
	if cfg2.GetChatCompletionsURL() != "https://custom-api.com/v1/chat/completions" {
		t.Errorf("unexpected chat completions URL: %s", cfg2.GetChatCompletionsURL())
	}
	if cfg2.TimeoutSeconds != 5 {
		t.Errorf("expected TimeoutSeconds to be 5, got %d", cfg2.TimeoutSeconds)
	}
}

func TestConfig_Validation(t *testing.T) {
	cfg := &ai.Config{
		Enabled: false,
		APIKey:  "",
		BaseURL: "https://api.openai.com/v1",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("validation should pass when disabled: %v", err)
	}

	cfg.Enabled = true
	if err := cfg.Validate(); err == nil {
		t.Error("expected error when API key is missing and AI is enabled")
	}

	cfg.APIKey = "some-key"
	cfg.BaseURL = "invalid-url-no-scheme"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid BaseURL")
	}
}

func TestMockClient_DeterministicResponse(t *testing.T) {
	mock := ai.NewMockClient("hello mock", nil)
	resp, err := mock.GenerateText(context.Background(), ai.GenerateTextRequest{UserPrompt: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "hello mock" {
		t.Errorf("expected text 'hello mock', got '%s'", resp.Text)
	}

	mockErr := ai.NewMockClient("", errors.New("some error"))
	_, err = mockErr.GenerateText(context.Background(), ai.GenerateTextRequest{UserPrompt: "test"})
	if err == nil || err.Error() != "some error" {
		t.Errorf("expected error 'some error', got %v", err)
	}
}

func TestOpenAIClient_RequestMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer mock-api-key" {
			t.Errorf("unexpected Authorization header: '%s'", auth)
		}
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("unexpected Content-Type header: '%s'", contentType)
		}

		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path called: %s", r.URL.Path)
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if reqBody["model"] != "mock-model" {
			t.Errorf("expected model 'mock-model', got '%v'", reqBody["model"])
		}

		msgs := reqBody["messages"].([]interface{})
		if len(msgs) != 2 {
			t.Errorf("expected 2 messages, got %d", len(msgs))
		}
		sysMsg := msgs[0].(map[string]interface{})
		userMsg := msgs[1].(map[string]interface{})
		if sysMsg["role"] != "system" || sysMsg["content"] != "sys prompt" {
			t.Errorf("invalid system message: %v", sysMsg)
		}
		if userMsg["role"] != "user" || userMsg["content"] != "user prompt" {
			t.Errorf("invalid user message: %v", userMsg)
		}

		if reqBody["temperature"].(float64) != 0.5 {
			t.Errorf("expected temperature 0.5, got %v", reqBody["temperature"])
		}
		if int(reqBody["max_tokens"].(float64)) != 50 {
			t.Errorf("expected max_tokens 50, got %v", reqBody["max_tokens"])
		}

		resp := map[string]interface{}{
			"id":    "chatcmpl-test",
			"model": "mock-model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": "assistant response text",
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &ai.Config{
		Enabled:        true,
		APIKey:         "mock-api-key",
		Model:          "mock-model",
		BaseURL:        server.URL,
		TimeoutSeconds: 5,
	}

	client, err := ai.NewOpenAIClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.GenerateText(context.Background(), ai.GenerateTextRequest{
		SystemPrompt: "sys prompt",
		UserPrompt:   "user prompt",
		Temperature:  0.5,
		MaxTokens:    50,
	})
	if err != nil {
		t.Fatalf("GenerateText failed: %v", err)
	}

	if resp.Text != "assistant response text" {
		t.Errorf("expected text 'assistant response text', got '%s'", resp.Text)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got '%s'", resp.FinishReason)
	}
}

func TestOpenAIClient_ErrorHandling(t *testing.T) {
	serverNon2xx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer serverNon2xx.Close()

	cfg := &ai.Config{
		Enabled:        true,
		APIKey:         "key",
		Model:          "model",
		BaseURL:        serverNon2xx.URL,
		TimeoutSeconds: 2,
	}
	client, _ := ai.NewOpenAIClient(cfg)
	_, err := client.GenerateText(context.Background(), ai.GenerateTextRequest{UserPrompt: "test"})
	if err == nil || !strings.Contains(err.Error(), "status code 500") {
		t.Errorf("expected 500 status code error, got: %v", err)
	}

	serverEmptyChoices := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":      "chatcmpl-test",
			"choices": []interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer serverEmptyChoices.Close()

	cfg.BaseURL = serverEmptyChoices.URL
	client, _ = ai.NewOpenAIClient(cfg)
	_, err = client.GenerateText(context.Background(), ai.GenerateTextRequest{UserPrompt: "test"})
	if err == nil || !strings.Contains(err.Error(), "empty choices") {
		t.Errorf("expected empty choices error, got: %v", err)
	}

	serverInvalidJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not { a json }"))
	}))
	defer serverInvalidJSON.Close()

	cfg.BaseURL = serverInvalidJSON.URL
	client, _ = ai.NewOpenAIClient(cfg)
	_, err = client.GenerateText(context.Background(), ai.GenerateTextRequest{UserPrompt: "test"})
	if err == nil || !strings.Contains(err.Error(), "decode response payload") {
		t.Errorf("expected decode error, got: %v", err)
	}

	// Test empty content with reasoning_content present
	serverEmptyContent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "chatcmpl-test",
			"model": "test-model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":              "assistant",
						"content":           "",
						"reasoning_content": "some hidden reasoning that should not be exposed",
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer serverEmptyContent.Close()

	cfg.BaseURL = serverEmptyContent.URL
	client, _ = ai.NewOpenAIClient(cfg)
	_, err = client.GenerateText(context.Background(), ai.GenerateTextRequest{UserPrompt: "test"})
	if err == nil {
		t.Error("expected error when content is empty")
	}
	if !strings.Contains(err.Error(), "content is empty") {
		t.Errorf("expected 'content is empty' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "finish_reason=stop") {
		t.Errorf("expected error to include finish_reason, got: %v", err)
	}
	if !strings.Contains(err.Error(), "reasoning_content_len=") {
		t.Errorf("expected error to include reasoning_content_len, got: %v", err)
	}
	// Verify reasoning_content itself is NOT exposed in error message
	if strings.Contains(err.Error(), "some hidden reasoning") {
		t.Error("error should not expose reasoning_content text")
	}
}

func TestOpenAIClient_RealSmokeTest(t *testing.T) {
	enabledStr := os.Getenv("AI_ENABLED")
	apiKey := os.Getenv("AI_API_KEY")
	baseURL := os.Getenv("AI_BASE_URL")
	model := os.Getenv("AI_MODEL")

	if enabledStr != "true" || apiKey == "" || baseURL == "" || model == "" {
		t.Skip("skipping real AI smoke test, missing env configuration")
	}

	cfg := ai.LoadConfig()
	client, err := ai.NewOpenAIClient(cfg)
	if err != nil {
		t.Fatalf("failed to create real client: %v", err)
	}

	resp, err := client.GenerateText(context.Background(), ai.GenerateTextRequest{
		UserPrompt:  "Return exactly: pong",
		MaxTokens:   50,
		Temperature: 0.1,
	})
	if err != nil {
		t.Fatalf("real AI request failed: %v", err)
	}

	t.Logf("Real AI response: %+v", resp)
	if resp.Text == "" {
		t.Error("expected non-empty response text from real AI call")
	}
}
