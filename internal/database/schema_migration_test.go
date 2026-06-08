package database

import (
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func TestConsolidatedMigration_PreservesData(t *testing.T) {
	databaseURL := getTestDatabaseURL(t)
	db := openTestDB(t, databaseURL)
	defer db.Close()

	dropAllTables(t, db)

	// Run the single consolidated migration (all tables + final schema)
	err := RunMigrations(databaseURL)
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Create a test user
	userID := "00000000-0000-0000-0000-000000000001"
	_, err = db.Exec(`INSERT INTO user_profiles (id, display_name) VALUES ($1, 'Test User')`, userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// Insert a daily_checkin with enum string values
	_, err = db.Exec(`
		INSERT INTO daily_checkins (id, user_id, date, energy_level, stress_level, focus_level, day_intensity, main_focus_today)
		VALUES (gen_random_uuid(), $1, '2026-01-01', 'medium', 'low', 'high', 'normal', 'test focus')
	`, userID)
	if err != nil {
		t.Fatalf("failed to insert test checkin: %v", err)
	}

	// Insert a daily_review
	_, err = db.Exec(`
		INSERT INTO daily_reviews (id, user_id, date, mood, difficulty_rating, energy_level_int, satisfaction_level)
		VALUES (gen_random_uuid(), $1, '2026-01-01', 'good', 3, 4, 5)
	`, userID)
	if err != nil {
		t.Fatalf("failed to insert test review: %v", err)
	}

	// Verify the data is preserved correctly
	var energyLevel, stressLevel, focusLevel, dayIntensity string
	err = db.QueryRow(`
		SELECT energy_level, stress_level, focus_level, day_intensity
		FROM daily_checkins WHERE user_id = $1
	`, userID).Scan(&energyLevel, &stressLevel, &focusLevel, &dayIntensity)
	if err != nil {
		t.Fatalf("failed to query checkin: %v", err)
	}

	if energyLevel != "medium" {
		t.Errorf("expected energy_level='medium', got '%s'", energyLevel)
	}
	if stressLevel != "low" {
		t.Errorf("expected stress_level='low', got '%s'", stressLevel)
	}
	if focusLevel != "high" {
		t.Errorf("expected focus_level='high', got '%s'", focusLevel)
	}
	if dayIntensity != "normal" {
		t.Errorf("expected day_intensity='normal', got '%s'", dayIntensity)
	}

	// Verify the review data is preserved
	var mood string
	var difficultyRating, energyLevelInt, satisfactionLevel int
	err = db.QueryRow(`
		SELECT mood, difficulty_rating, energy_level_int, satisfaction_level
		FROM daily_reviews WHERE user_id = $1
	`, userID).Scan(&mood, &difficultyRating, &energyLevelInt, &satisfactionLevel)
	if err != nil {
		t.Fatalf("failed to query review: %v", err)
	}

	if mood != "good" {
		t.Errorf("expected mood='good', got '%s'", mood)
	}
	if difficultyRating != 3 {
		t.Errorf("expected difficulty_rating=3, got %d", difficultyRating)
	}
}

func TestEnumDriftMigration_AlreadyCorrectData(t *testing.T) {
	databaseURL := getTestDatabaseURL(t)
	db := openTestDB(t, databaseURL)
	defer db.Close()

	dropAllTables(t, db)

	err := RunMigrations(databaseURL)
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Create test user
	userID := "00000000-0000-0000-0000-000000000002"
	_, err = db.Exec(`INSERT INTO user_profiles (id, display_name) VALUES ($1, 'Test User 2')`, userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// Insert data with correct enum strings (post-migration state)
	_, err = db.Exec(`
		INSERT INTO daily_checkins (id, user_id, date, energy_level, stress_level, focus_level, day_intensity, main_focus_today)
		VALUES (gen_random_uuid(), $1, '2026-01-02', 'high', 'low', 'veryHigh', 'busy', 'test focus')
	`, userID)
	if err != nil {
		t.Fatalf("failed to insert checkin with enum strings: %v", err)
	}

	// Verify data is unchanged
	var energyLevel, stressLevel, focusLevel, dayIntensity string
	err = db.QueryRow(`
		SELECT energy_level, stress_level, focus_level, day_intensity
		FROM daily_checkins WHERE user_id = $1
	`, userID).Scan(&energyLevel, &stressLevel, &focusLevel, &dayIntensity)
	if err != nil {
		t.Fatalf("failed to query checkin: %v", err)
	}

	if energyLevel != "high" {
		t.Errorf("expected energy_level='high', got '%s'", energyLevel)
	}
	if stressLevel != "low" {
		t.Errorf("expected stress_level='low', got '%s'", stressLevel)
	}
	if focusLevel != "veryHigh" {
		t.Errorf("expected focus_level='veryHigh', got '%s'", focusLevel)
	}
	if dayIntensity != "busy" {
		t.Errorf("expected day_intensity='busy', got '%s'", dayIntensity)
	}
}

func TestRefuseUnsafeCleanup(t *testing.T) {
	// Verify that the test infrastructure refuses to operate on non-test databases
	unsafeURLs := []string{
		"postgres://postgres@localhost:5432/soloquest?sslmode=disable",
		"postgres://postgres@localhost:5432/production?sslmode=disable",
		"postgres://postgres@localhost:5432/soloquest_dev?sslmode=disable",
	}

	for _, url := range unsafeURLs {
		if containsTestDBName(url) {
			t.Errorf("URL '%s' should NOT be considered safe for test operations", url)
		}
	}

	safeURLs := []string{
		"postgres://postgres@localhost:5432/soloquest_test?sslmode=disable",
		"postgres://postgres@localhost:5432/my_soloquest_test?sslmode=disable",
	}

	for _, url := range safeURLs {
		if !containsTestDBName(url) {
			t.Errorf("URL '%s' SHOULD be considered safe for test operations", url)
		}
	}
}

func containsTestDBName(url string) bool {
	return strings.Contains(url, "soloquest_test")
}
