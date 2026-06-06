package services_test

import (
	"testing"

	"github.com/google/uuid"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestReminderSettingService_EnsureDefaults(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	err := svc.EnsureDefaultReminderSettingsForUser(userID)
	if err != nil {
		t.Fatalf("failed to ensure defaults: %v", err)
	}

	settings, err := svc.GetReminderSettingsByUserID(userID)
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}

	if len(settings) != 7 {
		t.Errorf("expected 7 settings, got %d", len(settings))
	}

	types := map[string]dto.ReminderSettingResponse{}
	for _, s := range settings {
		types[s.Type] = s
	}

	if water, ok := types["water"]; ok {
		if water.Frequency != "interval" {
			t.Errorf("expected water frequency 'interval', got '%s'", water.Frequency)
		}
		if water.Status != "enabled" {
			t.Errorf("expected water status 'enabled', got '%s'", water.Status)
		}
		if water.Title != "Uống nước" {
			t.Errorf("expected water title 'Uống nước', got '%s'", water.Title)
		}
		if water.IntervalMinutes == nil || *water.IntervalMinutes != 90 {
			t.Errorf("expected water interval_minutes 90, got %v", water.IntervalMinutes)
		}
		if water.MaxPerDay == nil || *water.MaxPerDay != 8 {
			t.Errorf("expected water max_per_day 8, got %v", water.MaxPerDay)
		}
	} else {
		t.Error("missing water reminder setting")
	}

	if learning, ok := types["learning"]; ok {
		if learning.Frequency != "fixed" {
			t.Errorf("expected learning frequency 'fixed', got '%s'", learning.Frequency)
		}
		if learning.StartTime == nil || *learning.StartTime != "20:00" {
			t.Errorf("expected learning start_time '20:00', got '%v'", learning.StartTime)
		}
		if learning.Status != "enabled" {
			t.Errorf("expected learning status 'enabled', got '%s'", learning.Status)
		}
	} else {
		t.Error("missing learning reminder setting")
	}

	if custom, ok := types["custom"]; ok {
		if custom.Status != "disabled" {
			t.Errorf("expected custom status 'disabled', got '%s'", custom.Status)
		}
	} else {
		t.Error("missing custom reminder setting")
	}
}

func TestReminderSettingService_EnsureDefaultsIdempotent(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	err := svc.EnsureDefaultReminderSettingsForUser(userID)
	if err != nil {
		t.Fatalf("first ensure failed: %v", err)
	}

	err = svc.EnsureDefaultReminderSettingsForUser(userID)
	if err != nil {
		t.Fatalf("second ensure failed: %v", err)
	}

	var count int64
	db.Model(&models.ReminderSetting{}).Where("user_id = ?", userID).Count(&count)
	if count != 7 {
		t.Errorf("expected 7 settings after double ensure, got %d", count)
	}
}

func TestReminderSettingService_EnsureDefaultsDoesNotOverwrite(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	err := svc.EnsureDefaultReminderSettingsForUser(userID)
	if err != nil {
		t.Fatalf("ensure failed: %v", err)
	}

	freq := "smart"
	status := "disabled"
	req := dto.UpdateReminderSettingRequest{
		Frequency: &freq,
		Status:    &status,
	}
	_, err = svc.UpdateReminderSetting(userID, "water", &req)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	err = svc.EnsureDefaultReminderSettingsForUser(userID)
	if err != nil {
		t.Fatalf("second ensure failed: %v", err)
	}

	settings, _ := svc.GetReminderSettingsByUserID(userID)
	for _, s := range settings {
		if s.Type == "water" {
			if s.Frequency != "smart" {
				t.Errorf("ensure overwrote water frequency: expected 'smart', got '%s'", s.Frequency)
			}
			if s.Status != "disabled" {
				t.Errorf("ensure overwrote water status: expected 'disabled', got '%s'", s.Status)
			}
		}
	}
}

func TestReminderSettingService_SnakeCaseTypes(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	svc.EnsureDefaultReminderSettingsForUser(userID)
	settings, _ := svc.GetReminderSettingsByUserID(userID)

	expectedTypes := map[string]bool{
		"water":        false,
		"break_time":   false,
		"movement":     false,
		"learning":     false,
		"sleep":        false,
		"daily_review": false,
		"custom":       false,
	}

	for _, s := range settings {
		if _, ok := expectedTypes[s.Type]; ok {
			expectedTypes[s.Type] = true
		}
	}

	for typ, found := range expectedTypes {
		if !found {
			t.Errorf("expected type '%s' in settings", typ)
		}
	}
}

func TestReminderSettingService_UpdateWaterInterval(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	svc.EnsureDefaultReminderSettingsForUser(userID)

	freq := "interval"
	interval := 60
	startTime := "08:00"
	endTime := "22:00"
	req := dto.UpdateReminderSettingRequest{
		Frequency:       &freq,
		IntervalMinutes: &interval,
		StartTime:       &startTime,
		EndTime:         &endTime,
	}

	result, err := svc.UpdateReminderSetting(userID, "water", &req)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	if result.Frequency != "interval" {
		t.Errorf("expected frequency 'interval', got '%s'", result.Frequency)
	}
	if result.IntervalMinutes == nil || *result.IntervalMinutes != 60 {
		t.Errorf("expected interval_minutes 60, got %v", result.IntervalMinutes)
	}
	if result.StartTime == nil || *result.StartTime != "08:00" {
		t.Errorf("expected start_time '08:00', got '%v'", result.StartTime)
	}
	if result.EndTime == nil || *result.EndTime != "22:00" {
		t.Errorf("expected end_time '22:00', got '%v'", result.EndTime)
	}
}

func TestReminderSettingService_UpdateLearningFixed(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	svc.EnsureDefaultReminderSettingsForUser(userID)

	startTime := "19:00"
	req := dto.UpdateReminderSettingRequest{
		StartTime: &startTime,
	}

	result, err := svc.UpdateReminderSetting(userID, "learning", &req)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	if result.StartTime == nil || *result.StartTime != "19:00" {
		t.Errorf("expected start_time '19:00', got '%v'", result.StartTime)
	}
	if result.Frequency != "fixed" {
		t.Errorf("expected frequency 'fixed' preserved, got '%s'", result.Frequency)
	}
}

func TestReminderSettingService_PartialUpdatePreservesOtherFields(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	svc.EnsureDefaultReminderSettingsForUser(userID)

	interval := 120
	req := dto.UpdateReminderSettingRequest{
		IntervalMinutes: &interval,
	}

	result, err := svc.UpdateReminderSetting(userID, "water", &req)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	if result.IntervalMinutes == nil || *result.IntervalMinutes != 120 {
		t.Errorf("expected interval_minutes 120, got %v", result.IntervalMinutes)
	}
	if result.Frequency != "interval" {
		t.Errorf("expected frequency 'interval' preserved, got '%s'", result.Frequency)
	}
	if result.Status != "enabled" {
		t.Errorf("expected status 'enabled' preserved, got '%s'", result.Status)
	}
	if result.StartTime == nil || *result.StartTime != "08:00" {
		t.Errorf("expected start_time '08:00' preserved, got '%v'", result.StartTime)
	}
}

func TestReminderSettingService_ToggleLearning(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	svc.EnsureDefaultReminderSettingsForUser(userID)

	result, err := svc.ToggleReminderSetting(userID, "learning", "disabled")
	if err != nil {
		t.Fatalf("failed to toggle: %v", err)
	}

	if result.Status != "disabled" {
		t.Errorf("expected status 'disabled', got '%s'", result.Status)
	}

	result, err = svc.ToggleReminderSetting(userID, "learning", "enabled")
	if err != nil {
		t.Fatalf("failed to toggle back: %v", err)
	}

	if result.Status != "enabled" {
		t.Errorf("expected status 'enabled', got '%s'", result.Status)
	}
}

func TestReminderSettingService_CreateOnPatch(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	startTime := "08:00"
	req := dto.UpdateReminderSettingRequest{
		StartTime: &startTime,
	}

	result, err := svc.UpdateReminderSetting(userID, "water", &req)
	if err != nil {
		t.Fatalf("failed to create via patch: %v", err)
	}

	if result.Type != "water" {
		t.Errorf("expected type 'water', got '%s'", result.Type)
	}
	if result.StartTime == nil || *result.StartTime != "08:00" {
		t.Errorf("expected start_time '08:00', got '%v'", result.StartTime)
	}
}

func TestReminderSettingService_InvalidReminderType(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	req := dto.UpdateReminderSettingRequest{}
	_, err := svc.UpdateReminderSetting(userID, "nonexistent", &req)
	if err != services.ErrInvalidReminderType {
		t.Errorf("expected ErrInvalidReminderType, got %v", err)
	}
}

func TestReminderSettingService_InvalidFrequency(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	freq := "yearly"
	req := dto.UpdateReminderSettingRequest{
		Frequency: &freq,
	}
	_, err := svc.UpdateReminderSetting(userID, "learning", &req)
	if err != services.ErrInvalidFrequency {
		t.Errorf("expected ErrInvalidFrequency, got %v", err)
	}
}

func TestReminderSettingService_InvalidStatus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	status := "active"
	req := dto.UpdateReminderSettingRequest{
		Status: &status,
	}
	_, err := svc.UpdateReminderSetting(userID, "learning", &req)
	if err != services.ErrInvalidReminderStatus {
		t.Errorf("expected ErrInvalidReminderStatus, got %v", err)
	}
}

func TestReminderSettingService_InvalidInterval(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	interval := -1
	req := dto.UpdateReminderSettingRequest{
		IntervalMinutes: &interval,
	}
	_, err := svc.UpdateReminderSetting(userID, "water", &req)
	if err != services.ErrInvalidInterval {
		t.Errorf("expected ErrInvalidInterval, got %v", err)
	}
}

func TestReminderSettingService_InvalidMaxPerDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	maxPerDay := 0
	req := dto.UpdateReminderSettingRequest{
		MaxPerDay: &maxPerDay,
	}
	_, err := svc.UpdateReminderSetting(userID, "water", &req)
	if err != services.ErrInvalidMaxPerDay {
		t.Errorf("expected ErrInvalidMaxPerDay, got %v", err)
	}
}

func TestReminderSettingService_ToggleInvalidType(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	_, err := svc.ToggleReminderSetting(userID, "nonexistent", "disabled")
	if err != services.ErrInvalidReminderType {
		t.Errorf("expected ErrInvalidReminderType, got %v", err)
	}
}

func TestReminderSettingService_ToggleInvalidStatus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	_, err := svc.ToggleReminderSetting(userID, "learning", "active")
	if err != services.ErrInvalidReminderStatus {
		t.Errorf("expected ErrInvalidReminderStatus, got %v", err)
	}
}

func TestReminderSettingService_GetOrCreate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	setting, err := svc.GetOrCreateReminderSetting(userID, "sleep")
	if err != nil {
		t.Fatalf("failed to get or create: %v", err)
	}

	if string(setting.Type) != "sleep" {
		t.Errorf("expected type 'sleep', got '%s'", setting.Type)
	}
	if string(setting.Status) != "enabled" {
		t.Errorf("expected default status 'enabled', got '%s'", setting.Status)
	}
	if string(setting.Frequency) != "fixed" {
		t.Errorf("expected default frequency 'fixed', got '%s'", setting.Frequency)
	}

	createdID := setting.ID

	setting2, err := svc.GetOrCreateReminderSetting(userID, "sleep")
	if err != nil {
		t.Fatalf("second get or create failed: %v", err)
	}

	if setting2.ID != createdID {
		t.Errorf("second call should return existing setting, got different ID")
	}
}

func TestReminderSettingService_AutoSeedsOnGet(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "new@example.com")
	svc := services.NewReminderSettingService(db)

	settings, err := svc.GetReminderSettingsByUserID(userID)
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}

	if len(settings) != 7 {
		t.Errorf("expected 7 settings after auto-seed, got %d", len(settings))
	}
}

func TestReminderSettingService_AllFrequenciesValid(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	svc.EnsureDefaultReminderSettingsForUser(userID)

	frequencies := []string{"fixed", "interval", "random_in_range", "smart"}
	for _, freq := range frequencies {
		f := freq
		req := dto.UpdateReminderSettingRequest{Frequency: &f}
		_, err := svc.UpdateReminderSetting(userID, "learning", &req)
		if err != nil {
			t.Errorf("expected no error for frequency '%s', got %v", freq, err)
		}
	}
}

func TestReminderSettingService_AllReminderTypesValid(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewReminderSettingService(db)

	validTypes := []string{"water", "break_time", "movement", "learning", "sleep", "daily_review", "custom"}
	for _, rt := range validTypes {
		startTime := "12:00"
		req := dto.UpdateReminderSettingRequest{StartTime: &startTime}
		_, err := svc.UpdateReminderSetting(userID, rt, &req)
		if err != nil {
			t.Errorf("expected no error for type '%s', got %v", rt, err)
		}
	}
}
