package ai_test

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"solo_quest_backend/internal/services/ai"
)

func TestGeminiClient_RealSmokeTest(t *testing.T) {
	// Load .env.test so this test can run independently with Gemini creds
	_ = godotenv.Overload("../../../.env.test")
	_ = godotenv.Overload("../../.env.test")
	_ = godotenv.Overload(".env.test")

	if os.Getenv("AI_PROVIDER") != "gemini" {
		t.Skip("skipping Gemini smoke test: AI_PROVIDER != gemini")
	}
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("skipping Gemini smoke test: GEMINI_API_KEY not set")
	}
	if os.Getenv("AI_ENABLED") != "true" {
		t.Skip("skipping Gemini smoke test: AI_ENABLED != true")
	}

	cfg := ai.LoadConfig()
	client, err := ai.NewGeminiClient(cfg)
	if err != nil {
		t.Fatalf("failed to create Gemini client: %v", err)
	}

	resp, err := client.GenerateText(context.Background(), ai.GenerateTextRequest{
		UserPrompt:  "Return exactly: pong",
		MaxTokens:   50,
		Temperature: 0.1,
	})
	if err != nil {
		t.Fatalf("Gemini request failed: %v", err)
	}

	t.Logf("Gemini response: model=%s latency=%dms finish=%s text=%q",
		resp.Model, resp.LatencyMs, resp.FinishReason, resp.Text)

	if resp.Text == "" {
		t.Error("expected non-empty response text from Gemini")
	}
}

func TestNewClient_Factory(t *testing.T) {
	_ = godotenv.Overload("../../../.env.test")
	_ = godotenv.Overload("../../.env.test")
	_ = godotenv.Overload(".env.test")

	cfg := ai.LoadConfig()

	client, err := ai.NewClient(cfg)
	if err != nil {
		// Validation error means creds missing — skip, not fail
		t.Skipf("NewClient returned error (likely missing creds): %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client from factory")
	}

	t.Logf("factory created client for provider=%q model=%q", cfg.Provider, cfg.GeminiModel)
}
