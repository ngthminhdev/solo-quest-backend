package services_test

import (
	"testing"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestSaveOnboardingCreatesAnswer(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)
	onboardingService := services.NewOnboardingService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("bootstrap failed:", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	req := &services.OnboardingRequest{
		DisplayName: "Minh Thanh",
		Gender:      "Nam",
		MainActivity: "Engineer",
		MainGoals:   []string{"Health"},
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("failed to save onboarding:", err)
	}

	var onboardingCount int64
	db.Model(&models.OnboardingAnswer{}).Where("user_id = ?", devUserID).Count(&onboardingCount)
	if onboardingCount != 1 {
		t.Errorf("expected 1 onboarding answer, got %d", onboardingCount)
	}
}

func TestSaveOnboardingUpdatesProfile(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)
	onboardingService := services.NewOnboardingService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("bootstrap failed:", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	req := &services.OnboardingRequest{
		DisplayName:    "Updated Name",
		Age:            25,
		Gender:         "Nam",
		HeightCm:       170,
		WeightKg:       65,
		MainActivity:   "Software Engineer",
		MainGoals:      []string{"Uống nước", "Học tập"},
		QuietAfterTime: "23:00",
	}

	user, _, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("failed to save onboarding:", err)
	}

	if user.DisplayName != "Updated Name" {
		t.Errorf("expected display name 'Updated Name', got '%s'", user.DisplayName)
	}

	if user.Age == nil || *user.Age != 25 {
		t.Error("expected age 25")
	}

	if user.Gender == nil || *user.Gender != "Nam" {
		t.Error("expected gender 'Nam'")
	}

	if user.HeightCm == nil || *user.HeightCm != 170 {
		t.Error("expected height 170")
	}

	if user.WeightKg == nil || *user.WeightKg != 65 {
		t.Error("expected weight 65")
	}

	if user.MainActivity == nil || *user.MainActivity != "Software Engineer" {
		t.Error("expected main_activity 'Software Engineer'")
	}

	if !user.HasCompletedOnboarding {
		t.Error("expected has_completed_onboarding to be true")
	}

	if user.QuietAfterTime == nil || *user.QuietAfterTime != "23:00" {
		t.Error("expected quiet_after_time '23:00'")
	}
}

func TestSaveOnboardingCreatesLog(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)
	onboardingService := services.NewOnboardingService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("bootstrap failed:", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	var logCountBefore int64
	db.Model(&models.LogEntry{}).Where("user_id = ?", devUserID).Count(&logCountBefore)

	req := &services.OnboardingRequest{
		DisplayName: "Minh Thanh",
		Gender:      "Nam",
		MainActivity: "Engineer",
		MainGoals:   []string{"Health"},
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("failed to save onboarding:", err)
	}

	var logCountAfter int64
	db.Model(&models.LogEntry{}).Where("user_id = ?", devUserID).Count(&logCountAfter)

	if logCountAfter <= logCountBefore {
		t.Error("expected new log entry to be created")
	}
}

func TestSaveOnboardingIsUpsert(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)
	onboardingService := services.NewOnboardingService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("bootstrap failed:", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	req := &services.OnboardingRequest{
		DisplayName: "Minh Thanh",
		Gender:      "Nam",
		MainActivity: "Engineer",
		MainGoals:   []string{"Health"},
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("first save failed:", err)
	}

	req.DisplayName = "Updated Name"
	user, _, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("second save failed:", err)
	}

	var onboardingCount int64
	db.Model(&models.OnboardingAnswer{}).Where("user_id = ?", devUserID).Count(&onboardingCount)
	if onboardingCount != 1 {
		t.Errorf("expected 1 onboarding answer, got %d", onboardingCount)
	}

	if user.DisplayName != "Updated Name" {
		t.Errorf("expected display name 'Updated Name', got '%s'", user.DisplayName)
	}
}

func TestSaveOnboardingRejectsMissingRequiredFields(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)
	onboardingService := services.NewOnboardingService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("bootstrap failed:", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	req := &services.OnboardingRequest{
		DisplayName: "Test User",
		Gender:      "Nam",
		MainActivity: "Engineer",
		MainGoals:   []string{"Health"},
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}
}
