package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"solo_quest_backend/pkg/logger"
)

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	MigrationsPath = filepath.Join(projectRoot, "migrations")
}

func getTestDatabaseURL(t *testing.T) string {
	t.Helper()

	_ = godotenv.Load("../../.env.test")

	logger.InitForTest()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping migration test")
	}

	if !strings.Contains(databaseURL, "soloquest_test") {
		t.Fatalf("DATABASE_URL must contain soloquest_test for safety, got: %s", databaseURL)
	}

	return databaseURL
}

func openTestDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}

	return db
}

func dropAllTables(t *testing.T, db *sql.DB) {
	t.Helper()

	tables := []string{
		"app_settings",
		"reward_redemptions",
		"rewards",
		"xp_transactions",
		"log_entries",
		"daily_reviews",
		"daily_checkins",
		"quest_actions",
		"quests",
		"onboarding_answers",
		"auth_accounts",
		"user_profiles",
		"schema_migrations",
	}

	for _, table := range tables {
		db.Exec("DROP TABLE IF EXISTS " + table + " CASCADE")
	}
}

func TestRunMigrations_OnEmptyDB(t *testing.T) {
	databaseURL := getTestDatabaseURL(t)
	db := openTestDB(t, databaseURL)
	defer db.Close()

	dropAllTables(t, db)

	err := RunMigrations(databaseURL)
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	expectedTables := []string{
		"user_profiles",
		"auth_accounts",
		"onboarding_answers",
		"quests",
		"quest_actions",
		"daily_checkins",
		"daily_reviews",
		"log_entries",
		"xp_transactions",
		"rewards",
		"reward_redemptions",
		"app_settings",
	}

	for _, table := range expectedTables {
		var exists bool
		err := db.QueryRow(
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("failed to check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %s to exist after migration", table)
		}
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	databaseURL := getTestDatabaseURL(t)
	db := openTestDB(t, databaseURL)
	defer db.Close()

	dropAllTables(t, db)

	err := RunMigrations(databaseURL)
	if err != nil {
		t.Fatalf("first RunMigrations failed: %v", err)
	}

	err = RunMigrations(databaseURL)
	if err != nil {
		t.Fatalf("second RunMigrations should be idempotent, got error: %v", err)
	}
}

func TestGetMigrationVersion(t *testing.T) {
	databaseURL := getTestDatabaseURL(t)
	db := openTestDB(t, databaseURL)
	defer db.Close()

	dropAllTables(t, db)

	err := RunMigrations(databaseURL)
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	version, dirty, err := GetMigrationVersion(databaseURL)
	if err != nil {
		t.Fatalf("GetMigrationVersion failed: %v", err)
	}

	if version == 0 {
		t.Error("expected migration version > 0")
	}

	if dirty {
		t.Error("expected migration to not be dirty")
	}
}
