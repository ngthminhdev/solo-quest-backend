package config_test

import (
	"os"
	"testing"

	"solo_quest_backend/internal/config"
)

func TestFCMConfigDefaultsDisabled(t *testing.T) {
	os.Unsetenv("FCM_ENABLED")
	os.Unsetenv("FCM_PROJECT_ID")
	os.Unsetenv("FCM_CREDENTIALS_PATH")
	os.Unsetenv("FCM_CREDENTIALS_JSON")
	os.Unsetenv("FCM_DRY_RUN")

	cfg := config.Load()

	if cfg.FCM.Enabled {
		t.Error("expected FCM disabled by default")
	}
}

func TestFCMConfigEnabledWithProjectID(t *testing.T) {
	os.Setenv("FCM_ENABLED", "true")
	os.Setenv("FCM_PROJECT_ID", "test-project")
	os.Unsetenv("FCM_DRY_RUN")
	defer func() {
		os.Unsetenv("FCM_ENABLED")
		os.Unsetenv("FCM_PROJECT_ID")
	}()

	cfg := config.Load()

	if !cfg.FCM.Enabled {
		t.Error("expected FCM enabled when FCM_ENABLED=true and FCM_PROJECT_ID is set")
	}
	if cfg.FCM.ProjectID != "test-project" {
		t.Errorf("expected ProjectID 'test-project', got '%s'", cfg.FCM.ProjectID)
	}
}

func TestFCMConfigEnabledNoProjectIDDisables(t *testing.T) {
	os.Setenv("FCM_ENABLED", "true")
	os.Unsetenv("FCM_PROJECT_ID")
	os.Unsetenv("FCM_DRY_RUN")
	defer func() {
		os.Unsetenv("FCM_ENABLED")
	}()

	cfg := config.Load()

	if cfg.FCM.Enabled {
		t.Error("expected FCM disabled when FCM_ENABLED=true but FCM_PROJECT_ID is empty")
	}
}

func TestFCMConfigDryRunDefaultTrue(t *testing.T) {
	os.Setenv("FCM_ENABLED", "true")
	os.Setenv("FCM_PROJECT_ID", "test-project")
	os.Unsetenv("FCM_DRY_RUN")
	defer func() {
		os.Unsetenv("FCM_ENABLED")
		os.Unsetenv("FCM_PROJECT_ID")
	}()

	cfg := config.Load()

	if !cfg.FCM.DryRun {
		t.Error("expected FCM dry run true by default")
	}
}

func TestFCMConfigDryRunFromEnv(t *testing.T) {
	os.Setenv("FCM_ENABLED", "true")
	os.Setenv("FCM_PROJECT_ID", "test-project")
	os.Setenv("FCM_DRY_RUN", "false")
	defer func() {
		os.Unsetenv("FCM_ENABLED")
		os.Unsetenv("FCM_PROJECT_ID")
		os.Unsetenv("FCM_DRY_RUN")
	}()

	cfg := config.Load()

	if cfg.FCM.DryRun {
		t.Error("expected FCM dry run false when FCM_DRY_RUN=false")
	}
}

func TestFCMConfigCredentialsPath(t *testing.T) {
	os.Setenv("FCM_ENABLED", "true")
	os.Setenv("FCM_PROJECT_ID", "test-project")
	os.Setenv("FCM_CREDENTIALS_PATH", "/path/to/creds.json")
	defer func() {
		os.Unsetenv("FCM_ENABLED")
		os.Unsetenv("FCM_PROJECT_ID")
		os.Unsetenv("FCM_CREDENTIALS_PATH")
	}()

	cfg := config.Load()

	if cfg.FCM.CredentialsPath != "/path/to/creds.json" {
		t.Errorf("expected CredentialsPath '/path/to/creds.json', got '%s'", cfg.FCM.CredentialsPath)
	}
}

func TestFCMConfigCredentialsJSON(t *testing.T) {
	os.Setenv("FCM_ENABLED", "true")
	os.Setenv("FCM_PROJECT_ID", "test-project")
	os.Setenv("FCM_CREDENTIALS_JSON", `{"type":"service_account"}`)
	defer func() {
		os.Unsetenv("FCM_ENABLED")
		os.Unsetenv("FCM_PROJECT_ID")
		os.Unsetenv("FCM_CREDENTIALS_JSON")
	}()

	cfg := config.Load()

	if cfg.FCM.CredentialsJSON != `{"type":"service_account"}` {
		t.Errorf("expected CredentialsJSON set, got '%s'", cfg.FCM.CredentialsJSON)
	}
}
