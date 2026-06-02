package testutils

import (
	"testing"

	"solo_quest_backend/internal/config"
)

func LoadTestConfig(t *testing.T) *config.Config {
	t.Helper()

	cfg := config.Load()

	if cfg.AppEnv != "test" {
		t.Fatalf("expected APP_ENV=test, got '%s'", cfg.AppEnv)
	}

	if !contains(cfg.DatabaseURL, "soloquest_test") {
		t.Fatalf("DATABASE_URL must contain soloquest_test, got '%s'", cfg.DatabaseURL)
	}

	return cfg
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
