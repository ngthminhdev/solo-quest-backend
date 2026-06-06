package services_test

import (
	"testing"
	"time"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestSaveDailyCheckin_CreatesCheckin(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyCheckinService(db)

	dateStr := time.Now().In(timeutil.LocationVN).Format("2006-01-02")

	req := dto.SaveDailyCheckinRequest{
		Date:         dateStr,
		Mood:         "good",
		EnergyLevel:  "high",
		Availability: "free",
		Priority:     "learning",
	}

	resp, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Mood != "good" {
		t.Errorf("expected mood 'good', got '%s'", resp.Mood)
	}
	if resp.EnergyLevel != "high" {
		t.Errorf("expected energy_level 'high', got '%s'", resp.EnergyLevel)
	}
	if resp.Availability != "free" {
		t.Errorf("expected availability 'free', got '%s'", resp.Availability)
	}
	if resp.Priority != "learning" {
		t.Errorf("expected priority 'learning', got '%s'", resp.Priority)
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

	now := time.Now().In(timeutil.LocationVN)
	dateStr := now.Format("2006-01-02")

	req := dto.SaveDailyCheckinRequest{
		Date:         dateStr,
		Mood:         "normal",
		EnergyLevel:  "medium",
		Availability: "normal",
		Priority:     "learning",
	}

	_, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	req.Mood = "good"
	req.EnergyLevel = "high"
	_, err = svc.Save(userID, req)
	if err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	var count int64
	db.Model(&models.DailyCheckin{}).Where("user_id = ? AND date = ?", userID, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)).Count(&count)
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

	now := time.Now().In(timeutil.LocationVN)
	dateStr := now.Format("2006-01-02")

	svc.Save(userID, dto.SaveDailyCheckinRequest{
		Date:         dateStr,
		Mood:         "normal",
		EnergyLevel:  "high",
		Availability: "normal",
		Priority:     "learning",
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

	tests := []struct {
		name string
		req  dto.SaveDailyCheckinRequest
	}{
		{"invalid mood", dto.SaveDailyCheckinRequest{Mood: "veryLow", EnergyLevel: "medium", Availability: "normal", Priority: "learning"}},
		{"invalid energy_level", dto.SaveDailyCheckinRequest{Mood: "normal", EnergyLevel: "very_high", Availability: "normal", Priority: "learning"}},
		{"invalid availability", dto.SaveDailyCheckinRequest{Mood: "normal", EnergyLevel: "medium", Availability: "unknown", Priority: "learning"}},
		{"invalid priority", dto.SaveDailyCheckinRequest{Mood: "normal", EnergyLevel: "medium", Availability: "normal", Priority: "school"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Save(userID, tt.req)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestSaveDailyCheckin_WithDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyCheckinService(db)

	req := dto.SaveDailyCheckinRequest{
		Date:         "2026-06-03",
		Mood:         "good",
		EnergyLevel:  "high",
		Availability: "free",
		Priority:     "learning",
	}

	resp, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Date != "2026-06-03" {
		t.Errorf("expected date '2026-06-03', got '%s'", resp.Date)
	}
}

func TestSaveDailyCheckin_OldFieldsNotRequired(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyCheckinService(db)

	// Request with only the 4 new fields - no old fields
	req := dto.SaveDailyCheckinRequest{
		Mood:         "normal",
		EnergyLevel:  "medium",
		Availability: "normal",
		Priority:     "learning",
	}

	resp, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Mood != "normal" {
		t.Errorf("expected mood 'normal', got '%s'", resp.Mood)
	}
	if resp.EnergyLevel != "medium" {
		t.Errorf("expected energy_level 'medium', got '%s'", resp.EnergyLevel)
	}
	if resp.Availability != "normal" {
		t.Errorf("expected availability 'normal', got '%s'", resp.Availability)
	}
	if resp.Priority != "learning" {
		t.Errorf("expected priority 'learning', got '%s'", resp.Priority)
	}
}
