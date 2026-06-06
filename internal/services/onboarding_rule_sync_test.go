package services_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func findRuleInSettings(t *testing.T, db *gorm.DB, userID uuid.UUID, ruleType string) *dto.QuestRuleResponse {
	t.Helper()
	var settings models.QuestSettings
	if err := db.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		t.Fatalf("failed to find settings: %v", err)
	}

	var rules []dto.QuestRuleResponse
	if err := json.Unmarshal(settings.Rules, &rules); err != nil {
		t.Fatalf("failed to unmarshal rules: %v", err)
	}

	for _, r := range rules {
		if r.Type == ruleType {
			return &r
		}
	}
	return nil
}

func getEnabledCategories(t *testing.T, db *gorm.DB, userID uuid.UUID) []string {
	t.Helper()
	var settings models.QuestSettings
	if err := db.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		t.Fatalf("failed to find settings: %v", err)
	}

	var cats []string
	if err := json.Unmarshal(settings.EnabledCategories, &cats); err != nil {
		t.Fatalf("failed to unmarshal categories: %v", err)
	}
	return cats
}

func contains(arr []string, val string) bool {
	for _, item := range arr {
		if item == val {
			return true
		}
	}
	return false
}

func setupSyncTest(t *testing.T) (*gorm.DB, uuid.UUID, *services.OnboardingService) {
	db := testutils.SetupTestDB(t)
	bootstrapService := services.NewBootstrapService(db)
	onboardingService := services.NewOnboardingService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("bootstrap failed:", err)
	}

	devUserID := bootstrapService.GetDevUserID()
	return db, devUserID, onboardingService
}

func TestSyncQuestSettingsFromOnboarding_CategoriesAndGoals(t *testing.T) {
	db, devUserID, onboardingService := setupSyncTest(t)
	defer testutils.CleanupTestDB(t, db)

	// Test 1: Canonical and Legacy goals
	req := &services.OnboardingRequest{
		DisplayName:  "Test User",
		Gender:       "Nam",
		MainActivity: "Developer",
		MainGoals:    []string{"water", "movement", "learning", "Giấc ngủ", "Tập trung"}, // focus -> breakTime
	}

	_, _, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	enabledCats := getEnabledCategories(t, db, devUserID)
	expectedCats := []string{"water", "movement", "learning", "sleep", "breakTime"}

	for _, exp := range expectedCats {
		if !contains(enabledCats, exp) {
			t.Errorf("expected category '%s' to be enabled, enabled categories: %v", exp, enabledCats)
		}
	}

	// Verify consistent rules[].enabled
	for _, exp := range expectedCats {
		rule := findRuleInSettings(t, db, devUserID, exp)
		if rule == nil || !rule.Enabled {
			t.Errorf("expected rule for '%s' to be enabled", exp)
		}
	}

	// Rules not in main_goals should be disabled
	ruleReview := findRuleInSettings(t, db, devUserID, "review")
	if ruleReview != nil && ruleReview.Enabled {
		t.Error("expected review rule to be disabled")
	}
}

func TestSyncQuestSettingsFromOnboarding_BreakTimeRule(t *testing.T) {
	db, devUserID, onboardingService := setupSyncTest(t)
	defer testutils.CleanupTestDB(t, db)

	// 1. Valid range
	req := &services.OnboardingRequest{
		DisplayName:   "Test User",
		Gender:        "Nam",
		MainActivity:  "Developer",
		MainGoals:     []string{"focus"},
		WorkStartTime: "08:30",
		WorkEndTime:   "16:30",
	}

	_, _, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule := findRuleInSettings(t, db, devUserID, "breakTime")
	if rule.ActiveTimeRange == nil || rule.ActiveTimeRange.Start != "08:30" || rule.ActiveTimeRange.End != "16:30" {
		t.Errorf("unexpected breakTime active range: %+v", rule.ActiveTimeRange)
	}

	// 2. Overnight shift / invalid (start >= end) => should preserve existing
	req2 := &services.OnboardingRequest{
		DisplayName:   "Test User",
		Gender:        "Nam",
		MainActivity:  "Developer",
		MainGoals:     []string{"focus"},
		WorkStartTime: "22:00",
		WorkEndTime:   "06:00",
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req2)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule2 := findRuleInSettings(t, db, devUserID, "breakTime")
	// Should keep previous 08:30 - 16:30 range from previous save
	if rule2.ActiveTimeRange == nil || rule2.ActiveTimeRange.Start != "08:30" || rule2.ActiveTimeRange.End != "16:30" {
		t.Errorf("expected breakTime active range to be preserved, got: %+v", rule2.ActiveTimeRange)
	}
}

func TestSyncQuestSettingsFromOnboarding_LearningRule(t *testing.T) {
	db, devUserID, onboardingService := setupSyncTest(t)
	defer testutils.CleanupTestDB(t, db)

	// 1. Single preference (afternoon: 13:30 - 17:30)
	req := &services.OnboardingRequest{
		DisplayName:             "Test User",
		Gender:                  "Nam",
		MainActivity:            "Developer",
		MainGoals:               []string{"learning"},
		LearningTimePreferences: []string{"afternoon"},
	}

	_, _, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule := findRuleInSettings(t, db, devUserID, "learning")
	if rule.ActiveTimeRange == nil || rule.ActiveTimeRange.Start != "13:30" || rule.ActiveTimeRange.End != "17:30" {
		t.Errorf("unexpected learning active range: %+v", rule.ActiveTimeRange)
	}

	// 2. Multi-preferences covering range (morning: 08:00-11:30 and evening: 19:30-22:00) => 08:00 - 22:00
	req2 := &services.OnboardingRequest{
		DisplayName:             "Test User",
		Gender:                  "Nam",
		MainActivity:            "Developer",
		MainGoals:               []string{"learning"},
		LearningTimePreferences: []string{"morning", "evening"},
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req2)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule2 := findRuleInSettings(t, db, devUserID, "learning")
	if rule2.ActiveTimeRange == nil || rule2.ActiveTimeRange.Start != "08:00" || rule2.ActiveTimeRange.End != "22:00" {
		t.Errorf("unexpected covering range: %+v", rule2.ActiveTimeRange)
	}

	// 3. Prefer free_time_start/end if preferences include evening/flexible
	req3 := &services.OnboardingRequest{
		DisplayName:             "Test User",
		Gender:                  "Nam",
		MainActivity:            "Developer",
		MainGoals:               []string{"learning"},
		LearningTimePreferences: []string{"evening", "flexible"},
		FreeTimeStart:           "20:00",
		FreeTimeEnd:             "23:00",
		QuietAfterTime:          "22:30", // Clamps to quiet after time
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req3)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule3 := findRuleInSettings(t, db, devUserID, "learning")
	if rule3.ActiveTimeRange == nil || rule3.ActiveTimeRange.Start != "20:00" || rule3.ActiveTimeRange.End != "22:30" {
		t.Errorf("unexpected learning active range with free times & clamp: %+v", rule3.ActiveTimeRange)
	}
}

func TestSyncQuestSettingsFromOnboarding_MovementRule(t *testing.T) {
	db, devUserID, onboardingService := setupSyncTest(t)
	defer testutils.CleanupTestDB(t, db)

	// 1. Movement preference (morning + afternoon) clamped to quiet time
	req := &services.OnboardingRequest{
		DisplayName:             "Test User",
		Gender:                  "Nam",
		MainActivity:            "Developer",
		MainGoals:               []string{"movement"},
		MovementTimePreferences: []string{"morning", "afternoon"},
		QuietAfterTime:          "16:00", // Clamps end to 16:00
	}

	_, _, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule := findRuleInSettings(t, db, devUserID, "movement")
	if rule.ActiveTimeRange == nil || rule.ActiveTimeRange.Start != "08:00" || rule.ActiveTimeRange.End != "16:00" {
		t.Errorf("unexpected movement range: %+v", rule.ActiveTimeRange)
	}

	// 2. Health limitations caps difficulty to "easy"
	req2 := &services.OnboardingRequest{
		DisplayName:             "Test User",
		Gender:                  "Nam",
		MainActivity:            "Developer",
		MainGoals:               []string{"movement"},
		MovementTimePreferences: []string{"morning"},
		HealthLimitations:       []string{"Đau mỏi cổ vai gáy", "knee_pain"},
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req2)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule2 := findRuleInSettings(t, db, devUserID, "movement")
	if rule2.Difficulty != "easy" {
		t.Errorf("expected difficulty to be capped to 'easy', got: %s", rule2.Difficulty)
	}
}

func TestSyncQuestSettingsFromOnboarding_SleepRule(t *testing.T) {
	db, devUserID, onboardingService := setupSyncTest(t)
	defer testutils.CleanupTestDB(t, db)

	// 1. target_sleep_time=23:00, quiet_after_time=22:00 => range 21:00-22:00
	req := &services.OnboardingRequest{
		DisplayName:     "Test User",
		Gender:          "Nam",
		MainActivity:    "Developer",
		MainGoals:       []string{"sleep"},
		TargetSleepTime: "23:00",
		QuietAfterTime:  "22:00",
	}

	_, _, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule := findRuleInSettings(t, db, devUserID, "sleep")
	if rule.ActiveTimeRange == nil || rule.ActiveTimeRange.Start != "21:00" || rule.ActiveTimeRange.End != "22:00" {
		t.Errorf("unexpected sleep active range: %+v", rule.ActiveTimeRange)
	}

	// 2. Invalid sleep start time (before 18:00) => should preserve existing
	req2 := &services.OnboardingRequest{
		DisplayName:     "Test User",
		Gender:          "Nam",
		MainActivity:    "Developer",
		MainGoals:       []string{"sleep"},
		TargetSleepTime: "18:30", // start = 17:30 < 18:00
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req2)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule2 := findRuleInSettings(t, db, devUserID, "sleep")
	// Keeps 21:00-22:00
	if rule2.ActiveTimeRange == nil || rule2.ActiveTimeRange.Start != "21:00" || rule2.ActiveTimeRange.End != "22:00" {
		t.Errorf("expected sleep range to be preserved, got: %+v", rule2.ActiveTimeRange)
	}
}

func TestSyncQuestSettingsFromOnboarding_ReviewRule(t *testing.T) {
	db, devUserID, onboardingService := setupSyncTest(t)
	defer testutils.CleanupTestDB(t, db)

	// 1. With PreferredReviewTime = 20:30 => 20:30-21:00
	req := &services.OnboardingRequest{
		DisplayName:         "Test User",
		Gender:              "Nam",
		MainActivity:        "Developer",
		MainGoals:           []string{"discipline"},
		PreferredReviewTime: "20:30",
	}

	_, _, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule := findRuleInSettings(t, db, devUserID, "review")
	if rule.ActiveTimeRange == nil || rule.ActiveTimeRange.Start != "20:30" || rule.ActiveTimeRange.End != "21:00" {
		t.Errorf("unexpected review active range: %+v", rule.ActiveTimeRange)
	}

	// 2. Without PreferredReviewTime, fallback to target_sleep_time - 60m
	req2 := &services.OnboardingRequest{
		DisplayName:     "Test User",
		Gender:          "Nam",
		MainActivity:    "Developer",
		MainGoals:       []string{"discipline"},
		TargetSleepTime: "22:00",
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req2)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule2 := findRuleInSettings(t, db, devUserID, "review")
	if rule2.ActiveTimeRange == nil || rule2.ActiveTimeRange.Start != "21:00" || rule2.ActiveTimeRange.End != "22:00" {
		t.Errorf("unexpected fallback review active range: %+v", rule2.ActiveTimeRange)
	}
}

func TestSyncQuestSettingsFromOnboarding_WaterRule(t *testing.T) {
	db, devUserID, onboardingService := setupSyncTest(t)
	defer testutils.CleanupTestDB(t, db)

	// 1. wake_up_time = 07:00 => start = 07:30
	// target_sleep_time = 23:00 => end = 23:00
	// water_reminder_mode = intense => maxPerDay = 12
	req := &services.OnboardingRequest{
		DisplayName:       "Test User",
		Gender:            "Nam",
		MainActivity:      "Developer",
		MainGoals:         []string{"water"},
		WakeUpTime:        "07:00",
		TargetSleepTime:   "23:00",
		WaterReminderMode: "intense",
	}

	_, _, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatalf("failed to save onboarding: %v", err)
	}

	rule := findRuleInSettings(t, db, devUserID, "water")
	if rule.ActiveTimeRange == nil || rule.ActiveTimeRange.Start != "07:30" || rule.ActiveTimeRange.End != "23:00" {
		t.Errorf("unexpected water active range: %+v", rule.ActiveTimeRange)
	}
	if rule.MaxPerDay == nil || *rule.MaxPerDay != 12 {
		t.Errorf("expected max_per_day to be 12, got: %v", rule.MaxPerDay)
	}
}
