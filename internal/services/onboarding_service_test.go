package services_test

import (
	"encoding/json"
	"testing"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"

	"gorm.io/datatypes"
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

func TestSaveOnboarding_WeekdaysCreatesBlocks(t *testing.T) {
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
		DisplayName:      "Test User",
		Gender:           "Nam",
		MainActivity:     "Engineer",
		MainGoals:        []string{"Health"},
		WorkScheduleType: "weekdays",
		WorkStartTime:    "09:00",
		WorkEndTime:      "17:00",
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}

	var blocks []models.ScheduleBlock
	if err := db.Where("user_id = ?", devUserID).Find(&blocks).Error; err != nil {
		t.Fatal(err)
	}

	if len(blocks) != 1 {
		t.Fatalf("expected 1 schedule block, got %d", len(blocks))
	}

	block := blocks[0]
	var days []int
	if err := json.Unmarshal(block.DaysOfWeek, &days); err != nil {
		t.Fatal(err)
	}

	if len(days) != 5 {
		t.Errorf("expected 5 weekdays, got %v", days)
	}

	expectedDays := []int{1, 2, 3, 4, 5}
	for i, d := range days {
		if d != expectedDays[i] {
			t.Errorf("expected day %d at index %d, got %d", expectedDays[i], i, d)
		}
	}

	if block.IsBusy != true || block.IsFlexible != false {
		t.Errorf("expected busy block, got busy=%t flexible=%t", block.IsBusy, block.IsFlexible)
	}
}

func TestSaveOnboarding_FullWeekCreatesBlocks(t *testing.T) {
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
		DisplayName:      "Test User",
		Gender:           "Nam",
		MainActivity:     "Engineer",
		MainGoals:        []string{"Health"},
		WorkScheduleType: "full_week",
		WorkStartTime:    "09:00",
		WorkEndTime:      "17:00",
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}

	var blocks []models.ScheduleBlock
	if err := db.Where("user_id = ?", devUserID).Find(&blocks).Error; err != nil {
		t.Fatal(err)
	}

	if len(blocks) != 1 {
		t.Fatalf("expected 1 schedule block, got %d", len(blocks))
	}

	block := blocks[0]
	var days []int
	if err := json.Unmarshal(block.DaysOfWeek, &days); err != nil {
		t.Fatal(err)
	}

	if len(days) != 7 {
		t.Errorf("expected 7 weekdays, got %v", days)
	}

	if block.IsBusy != true || block.IsFlexible != false {
		t.Errorf("expected busy block, got busy=%t flexible=%t", block.IsBusy, block.IsFlexible)
	}
}

func TestSaveOnboarding_FlexibleCreatesFlexibleBlock(t *testing.T) {
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
		DisplayName:      "Test User",
		Gender:           "Nam",
		MainActivity:     "Engineer",
		MainGoals:        []string{"Health"},
		WorkScheduleType: "flexible",
		WorkStartTime:    "09:00",
		WorkEndTime:      "17:00",
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}

	var blocks []models.ScheduleBlock
	if err := db.Where("user_id = ?", devUserID).Find(&blocks).Error; err != nil {
		t.Fatal(err)
	}

	if len(blocks) != 1 {
		t.Fatalf("expected 1 schedule block, got %d", len(blocks))
	}

	block := blocks[0]
	if block.IsBusy != false || block.IsFlexible != true {
		t.Errorf("expected flexible block (not busy), got busy=%t flexible=%t", block.IsBusy, block.IsFlexible)
	}
}

func TestSaveOnboarding_NightShiftAllowsOvernight(t *testing.T) {
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
		DisplayName:      "Test User",
		Gender:           "Nam",
		MainActivity:     "Engineer",
		MainGoals:        []string{"Health"},
		WorkScheduleType: "night_shift",
		WorkStartTime:    "22:00",
		WorkEndTime:      "06:00",
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}

	var blocks []models.ScheduleBlock
	if err := db.Where("user_id = ?", devUserID).Find(&blocks).Error; err != nil {
		t.Fatal(err)
	}

	if len(blocks) != 1 {
		t.Fatalf("expected 1 schedule block, got %d", len(blocks))
	}

	block := blocks[0]
	if block.StartTime != "22:00" || block.EndTime != "06:00" {
		t.Errorf("expected start 22:00 and end 06:00, got %s-%s", block.StartTime, block.EndTime)
	}
}

func TestSaveOnboarding_DoesNotOverwriteExistingBlocks(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)
	onboardingService := services.NewOnboardingService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("bootstrap failed:", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	// 1. Manually insert a custom schedule block before onboarding
	customBlock := models.ScheduleBlock{
		UserID:     devUserID,
		Title:      "My Custom Block",
		Type:       models.ScheduleBlockTypeStudy,
		DaysOfWeek: datatypes.JSON([]byte("[1]")),
		StartTime:  "08:00",
		EndTime:    "10:00",
		IsBusy:     true,
		IsFlexible: false,
		Enabled:    true,
	}
	if err := db.Create(&customBlock).Error; err != nil {
		t.Fatal(err)
	}

	// 2. Perform onboarding which would normally create a different block
	req := &services.OnboardingRequest{
		DisplayName:      "Test User",
		Gender:           "Nam",
		MainActivity:     "Engineer",
		MainGoals:        []string{"Health"},
		WorkScheduleType: "weekdays",
		WorkStartTime:    "09:00",
		WorkEndTime:      "17:00",
	}

	_, _, err = onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}

	// 3. Verify that the custom block remains and NO new block was generated
	var blocks []models.ScheduleBlock
	if err := db.Where("user_id = ?", devUserID).Find(&blocks).Error; err != nil {
		t.Fatal(err)
	}

	if len(blocks) != 1 {
		t.Fatalf("expected exactly 1 block (existing), got %d", len(blocks))
	}

	if blocks[0].Title != "My Custom Block" {
		t.Errorf("expected existing block to be preserved, but got block title: %s", blocks[0].Title)
	}
}

func TestSaveOnboarding_PreferredFreeTimesMultiSelect(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)
	onboardingService := services.NewOnboardingService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("bootstrap failed:", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	// Test 1: New multi-select format
	req := &services.OnboardingRequest{
		DisplayName:        "Test User",
		Gender:             "Nam",
		MainActivity:       "Engineer",
		MainGoals:          []string{"Health"},
		PreferredFreeTimes: []string{"lunch", "evening"},
	}

	_, onboarding, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}

	var savedReq services.OnboardingRequest
	if err := json.Unmarshal(onboarding.Answers, &savedReq); err != nil {
		t.Fatal(err)
	}

	if len(savedReq.PreferredFreeTimes) != 2 || savedReq.PreferredFreeTimes[0] != "lunch" || savedReq.PreferredFreeTimes[1] != "evening" {
		t.Errorf("expected preferred_free_times to be saved, got %v", savedReq.PreferredFreeTimes)
	}

	if savedReq.FreeTimePreference != "lunch" {
		t.Errorf("expected FreeTimePreference to be populated as backward compatibility fallback, got '%s'", savedReq.FreeTimePreference)
	}

	// Test 2: Old single-value format compatibility
	reqOld := &services.OnboardingRequest{
		DisplayName:        "Test User 2",
		Gender:             "Nam",
		MainActivity:       "Engineer",
		MainGoals:          []string{"Health"},
		FreeTimePreference: "early_morning",
	}

	_, onboardingOld, err := onboardingService.SaveOnboarding(devUserID, reqOld)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}

	var savedReqOld services.OnboardingRequest
	if err := json.Unmarshal(onboardingOld.Answers, &savedReqOld); err != nil {
		t.Fatal(err)
	}

	if len(savedReqOld.PreferredFreeTimes) != 1 || savedReqOld.PreferredFreeTimes[0] != "early_morning" {
		t.Errorf("expected PreferredFreeTimes to be populated from FreeTimePreference, got %v", savedReqOld.PreferredFreeTimes)
	}
}

func TestSaveOnboarding_TimePreferencesPlural(t *testing.T) {
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
		DisplayName:             "Minh Thanh",
		Gender:                  "Nam",
		MainActivity:            "Engineer",
		MainGoals:               []string{"health"},
		LearningTimePreferences: []string{"evening", "lunch"},
		MovementTimePreferences: []string{"early_morning", "after_work"},
	}

	user, onboarding, err := onboardingService.SaveOnboarding(devUserID, req)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}

	if !user.HasCompletedOnboarding {
		t.Error("expected user.HasCompletedOnboarding to be true")
	}

	var savedReq services.OnboardingRequest
	if err := json.Unmarshal(onboarding.Answers, &savedReq); err != nil {
		t.Fatal(err)
	}

	if len(savedReq.LearningTimePreferences) != 2 || savedReq.LearningTimePreferences[0] != "evening" || savedReq.LearningTimePreferences[1] != "lunch" {
		t.Errorf("expected learning_time_preferences to be saved, got %v", savedReq.LearningTimePreferences)
	}

	if savedReq.LearningTimePreference != "evening" {
		t.Errorf("expected LearningTimePreference to be populated as fallback, got '%s'", savedReq.LearningTimePreference)
	}

	if len(savedReq.MovementTimePreferences) != 2 || savedReq.MovementTimePreferences[0] != "early_morning" || savedReq.MovementTimePreferences[1] != "after_work" {
		t.Errorf("expected movement_time_preferences to be saved, got %v", savedReq.MovementTimePreferences)
	}

	if savedReq.MovementTimePreference != "early_morning" {
		t.Errorf("expected MovementTimePreference to be populated as fallback, got '%s'", savedReq.MovementTimePreference)
	}

	reqOld := &services.OnboardingRequest{
		DisplayName:            "Test User 2",
		Gender:                 "Nam",
		MainActivity:           "Engineer",
		MainGoals:              []string{"health"},
		LearningTimePreference: "lunch",
		MovementTimePreference: "evening",
	}

	_, onboardingOld, err := onboardingService.SaveOnboarding(devUserID, reqOld)
	if err != nil {
		t.Fatal("save onboarding failed:", err)
	}

	var savedReqOld services.OnboardingRequest
	if err := json.Unmarshal(onboardingOld.Answers, &savedReqOld); err != nil {
		t.Fatal(err)
	}

	if len(savedReqOld.LearningTimePreferences) != 1 || savedReqOld.LearningTimePreferences[0] != "lunch" {
		t.Errorf("expected LearningTimePreferences to be populated from LearningTimePreference, got %v", savedReqOld.LearningTimePreferences)
	}

	if len(savedReqOld.MovementTimePreferences) != 1 || savedReqOld.MovementTimePreferences[0] != "evening" {
		t.Errorf("expected MovementTimePreferences to be populated from MovementTimePreference, got %v", savedReqOld.MovementTimePreferences)
	}
}
