package services_test

import (
	"testing"
	"time"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestSaveDailyCheckin_CreatesCheckin(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyCheckinService(db)

	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")

	req := dto.SaveDailyCheckinRequest{
		Date:                dateStr,
		EnergyLevel:         "high",
		StressLevel:         "low",
		FocusLevel:          "medium",
		DayIntensity:        "normal",
		MainFocusToday:      "Backend work",
		Note:                "Feeling good",
		AvailableTimeBlocks: []string{"morning", "evening"},
	}

	resp, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.EnergyLevel != "high" {
		t.Errorf("expected energy_level 'high', got '%s'", resp.EnergyLevel)
	}
	if resp.Date != dateStr {
		t.Errorf("expected date '%s', got '%s'", dateStr, resp.Date)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeMorningCheckin).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 morningCheckin log, got %d", logCount)
	}
}

func TestSaveDailyCheckin_UpsertsExisting(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyCheckinService(db)

	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")

	req := dto.SaveDailyCheckinRequest{
		Date:           dateStr,
		EnergyLevel:    "high",
		StressLevel:    "low",
		FocusLevel:     "medium",
		DayIntensity:   "normal",
		MainFocusToday: "First save",
	}

	_, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	req.MainFocusToday = "Updated save"
	req.EnergyLevel = "low"
	_, err = svc.Save(userID, req)
	if err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	var count int64
	db.Model(&models.DailyCheckin{}).Where("user_id = ? AND date = ?", userID, now.UTC().Truncate(24*time.Hour)).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 checkin row, got %d", count)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeMorningCheckin).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 morningCheckin log (no duplicate), got %d", logCount)
	}
}

func TestGetToday_ReturnsFalseWhenMissing(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyCheckinService(db)

	result, err := svc.GetToday(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.HasCheckedIn {
		t.Error("expected has_checked_in to be false")
	}
	if result.Item != nil {
		t.Error("expected item to be nil")
	}
}

func TestGetToday_ReturnsTrueWhenExists(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyCheckinService(db)

	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")

	svc.Save(userID, dto.SaveDailyCheckinRequest{
		Date:         dateStr,
		EnergyLevel:  "high",
		StressLevel:  "low",
		FocusLevel:   "medium",
		DayIntensity: "normal",
	})

	result, err := svc.GetToday(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.HasCheckedIn {
		t.Error("expected has_checked_in to be true")
	}
	if result.Item == nil {
		t.Fatal("expected item to be non-nil")
	}
}

func TestSaveDailyCheckin_InvalidEnumRejected(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyCheckinService(db)

	tests := []dto.SaveDailyCheckinRequest{
		{EnergyLevel: "invalid", StressLevel: "low", FocusLevel: "medium", DayIntensity: "normal"},
		{EnergyLevel: "high", StressLevel: "invalid", FocusLevel: "medium", DayIntensity: "normal"},
		{EnergyLevel: "high", StressLevel: "low", FocusLevel: "invalid", DayIntensity: "normal"},
		{EnergyLevel: "high", StressLevel: "low", FocusLevel: "medium", DayIntensity: "invalid"},
	}

	for i, req := range tests {
		_, err := svc.Save(userID, req)
		if err == nil {
			t.Errorf("test %d: expected error for invalid enum", i)
		}
	}
}
