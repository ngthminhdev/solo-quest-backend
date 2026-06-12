package quest_generation_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

func TestContextBuilder_BuildsContextWithOnboarding(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Set up user profile
	var profile models.UserProfile
	if err := db.Where("id = ?", userID).First(&profile).Error; err != nil {
		t.Fatal(err)
	}
	profile.DisplayName = "Thanh Test"
	activity := "Developer"
	profile.MainActivity = &activity
	profile.MainGoals = datatypes.JSON([]byte(`["Uống nước", "Học tập"]`))
	profile.HealthLimitations = datatypes.JSON([]byte(`["Đau mỏi cổ vai gáy"]`))
	db.Save(&profile)

	// Set up onboarding answers
	obAnswers := map[string]interface{}{
		"main_activity":             "Developer",
		"work_schedule_type":        "weekdays",
		"work_start_time":           "09:00",
		"work_end_time":             "18:00",
		"wake_up_time":              "07:00",
		"target_sleep_time":         "23:00",
		"quiet_after_time":          "22:00",
		"free_time_start":           "19:00",
		"free_time_end":             "22:00",
		"learning_time_preference":  "evening",
		"learning_time_preferences": []string{"evening", "night"},
		"movement_time_preference":  "morning",
		"movement_time_preferences": []string{"morning", "afternoon"},
	}
	obJSON, _ := json.Marshal(obAnswers)
	onboarding := models.OnboardingAnswer{
		UserID:    userID,
		Answers:   datatypes.JSON(obJSON),
		Completed: true,
	}
	db.Create(&onboarding)

	// Set up Quest Settings
	catsJSON, _ := json.Marshal([]string{"movement", "learning"})
	rules := []dto.QuestRuleResponse{
		{
			ID:             "rule_movement",
			Type:           "movement",
			Title:          "Vận động",
			Description:    "Khuyến khích vận động thể chất",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       5,
		},
	}
	rulesJSON, _ := json.Marshal(rules)
	settings := models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   5,
		Difficulty:        "normal",
		EnabledCategories: datatypes.JSON(catsJSON),
		PreferredDuration: "short",
		Rules:             datatypes.JSON(rulesJSON),
	}
	db.Create(&settings)

	// Build context
	builder := quest_generation.NewUserQuestContextBuilder(db)
	qctx, err := builder.Build(context.Background(), userID, timeutil.TodayVN())
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	// Verify assertions
	if qctx.DisplayName != "Thanh Test" {
		t.Errorf("expected DisplayName 'Thanh Test', got '%s'", qctx.DisplayName)
	}
	if qctx.MainActivity != "Developer" {
		t.Errorf("expected MainActivity 'Developer', got '%s'", qctx.MainActivity)
	}
	if len(qctx.MainGoals) != 2 || qctx.MainGoals[0] != "Uống nước" {
		t.Errorf("unexpected MainGoals: %v", qctx.MainGoals)
	}
	if len(qctx.HealthLimitations) != 1 || qctx.HealthLimitations[0] != "Đau mỏi cổ vai gáy" {
		t.Errorf("unexpected HealthLimitations: %v", qctx.HealthLimitations)
	}
	if qctx.WorkScheduleType != "weekdays" || qctx.WorkStartTime != "09:00" || qctx.WorkEndTime != "18:00" {
		t.Errorf("unexpected work schedule values")
	}
	if len(qctx.LearningTimePreferences) != 2 || qctx.LearningTimePreferences[0] != "evening" || qctx.LearningTimePreferences[1] != "night" {
		t.Errorf("unexpected LearningTimePreferences: %v", qctx.LearningTimePreferences)
	}
	if qctx.LearningTimePreference != "evening" {
		t.Errorf("expected LearningTimePreference 'evening', got '%s'", qctx.LearningTimePreference)
	}
	if len(qctx.MovementTimePreferences) != 2 || qctx.MovementTimePreferences[0] != "morning" || qctx.MovementTimePreferences[1] != "afternoon" {
		t.Errorf("unexpected MovementTimePreferences: %v", qctx.MovementTimePreferences)
	}
	if qctx.MovementTimePreference != "morning" {
		t.Errorf("expected MovementTimePreference 'morning', got '%s'", qctx.MovementTimePreference)
	}
	if qctx.DailyQuestCount != 5 || qctx.Difficulty != "normal" || qctx.PreferredDuration != "short" {
		t.Errorf("unexpected quest settings values")
	}
	if len(qctx.EnabledCategories) != 2 || qctx.EnabledCategories[0] != "movement" {
		t.Errorf("unexpected EnabledCategories: %v", qctx.EnabledCategories)
	}
	if len(qctx.Rules) != 1 || qctx.Rules[0].ID != "rule_movement" {
		t.Errorf("unexpected rules context size or ID")
	}
}

func TestContextBuilder_FallbackWhenPluralMissing(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Set up onboarding answers with singular time preferences only
	obAnswers := map[string]interface{}{
		"learning_time_preference": "afternoon",
		"movement_time_preference": "evening",
	}
	obJSON, _ := json.Marshal(obAnswers)
	onboarding := models.OnboardingAnswer{
		UserID:    userID,
		Answers:   datatypes.JSON(obJSON),
		Completed: true,
	}
	db.Create(&onboarding)

	// Build context
	builder := quest_generation.NewUserQuestContextBuilder(db)
	qctx, err := builder.Build(context.Background(), userID, timeutil.TodayVN())
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	// Verify fallback logic
	if len(qctx.LearningTimePreferences) != 1 || qctx.LearningTimePreferences[0] != "afternoon" {
		t.Errorf("expected fallback LearningTimePreferences to be ['afternoon'], got %v", qctx.LearningTimePreferences)
	}
	if qctx.LearningTimePreference != "afternoon" {
		t.Errorf("expected LearningTimePreference 'afternoon', got '%s'", qctx.LearningTimePreference)
	}

	if len(qctx.MovementTimePreferences) != 1 || qctx.MovementTimePreferences[0] != "evening" {
		t.Errorf("expected fallback MovementTimePreferences to be ['evening'], got %v", qctx.MovementTimePreferences)
	}
	if qctx.MovementTimePreference != "evening" {
		t.Errorf("expected MovementTimePreference 'evening', got '%s'", qctx.MovementTimePreference)
	}
}

func TestRuleBasedGenerator_GeneratesQuests(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()

	qctx := &quest_generation.UserQuestContext{
		UserID:                  userID,
		LocalDate:               timeutil.TodayVN().AddDate(0, 0, 1), // tomorrow avoids past-reminder shift
		Timezone:                "Asia/Ho_Chi_Minh",
		DailyQuestCount:         3,
		Difficulty:              "normal",
		PreferredDuration:       "medium",
		EnabledCategories:       []string{"water", "learning"},
		LearningTimePreferences: []string{"evening"},
		Rules: []quest_generation.QuestRuleContext{
			{
				ID:          "rule_water",
				Type:        "water",
				Title:       "Uống nước",
				Description: "Nhắc uống nước",
				Enabled:     true,
				Difficulty:  "easy",
				Priority:    5,
				Weekdays:    []int{1, 2, 3, 4, 5, 6, 7},
			},
			{
				ID:          "rule_learning",
				Type:        "learning",
				Title:       "Học tập",
				Description: "Quest học tập",
				Enabled:     true,
				Difficulty:  "medium",
				Priority:    4,
				Weekdays:    []int{1, 2, 3, 4, 5, 6, 7},
			},
		},
	}

	generator := quest_generation.NewRuleBasedGenerator(db)
	quests, err := generator.GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("failed to generate quests: %v", err)
	}

	if len(quests) == 0 {
		t.Fatal("expected quests to be generated")
	}

	// Verify all generated quests were saved to the DB
	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != len(quests) {
		t.Errorf("expected %d quests in DB, got %d", len(quests), len(dbQuests))
	}

	for _, q := range dbQuests {
		if q.Source != models.QuestSourceConfigBased {
			t.Errorf("expected source to be configBased, got %s", q.Source)
		}
	}
}

func TestParseQuestCandidateResponse(t *testing.T) {
	rawJSON := `{"quests": [{"type": "water", "title": "Uống nước", "description": "Uống một cốc nước", "difficulty": "easy", "estimated_minutes": 2, "xp_reward": 5, "tags": ["sức khỏe"], "reason": "Bù nước", "instruction": "Uống nước", "reminder_time": "08:30"}]}`
	resp, err := quest_generation.ParseQuestCandidateResponse(rawJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Quests) != 1 || resp.Quests[0].Title != "Uống nước" {
		t.Errorf("unexpected parsed result: %+v", resp)
	}

	wrappedJSON := "```json\n" + rawJSON + "\n```"
	resp2, err := quest_generation.ParseQuestCandidateResponse(wrappedJSON)
	if err != nil {
		t.Fatalf("unexpected error with markdown fences: %v", err)
	}
	if len(resp2.Quests) != 1 || resp2.Quests[0].Title != "Uống nước" {
		t.Errorf("unexpected parsed result from markdown fences: %+v", resp2)
	}

	_, err = quest_generation.ParseQuestCandidateResponse(`invalid json`)
	if err == nil {
		t.Error("expected error for invalid json")
	}

	_, err = quest_generation.ParseQuestCandidateResponse(`{"quests": []}`)
	if err == nil {
		t.Error("expected error for empty quests")
	}
}

func TestCandidateValidator_Validate(t *testing.T) {
	validator := quest_generation.NewCandidateValidator()

	qctx := &quest_generation.UserQuestContext{
		UserID:              uuid.New(),
		LocalDate:           timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:     3,
		Difficulty:          "normal",
		EnabledCategories:   []string{"sleep", "learning"},
		QuietAfterTime:      "22:00",
		ExistingQuestTitles: []string{"Quest đã tồn tại"},
		Rules: []quest_generation.QuestRuleContext{
			{
				ID:         "rule_sleep",
				Type:       "sleep",
				Enabled:    true,
				Difficulty: "easy",
				ActiveTimeRange: &quest_generation.TimeRangeContext{
					Start: "08:00",
					End:   "22:00",
				},
			},
			{
				ID:         "rule_learning",
				Type:       "learning",
				Enabled:    false,
				Difficulty: "medium",
			},
		},
	}

	validCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "sleep",
			Title:            "Chuẩn bị đi ngủ",
			Description:      "Đi ngủ",
			Difficulty:       "easy",
			EstimatedMinutes: 5,
			XPReward:         5,
			ReminderTime:     "08:30",
		},
	}
	err := validator.Validate(qctx, validCandidates)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	disabledRuleCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "learning",
			Title:            "Học tập",
			Description:      "Học tập",
			Difficulty:       "medium",
			EstimatedMinutes: 20,
			XPReward:         10,
			ReminderTime:     "19:00",
		},
	}
	err = validator.Validate(qctx, disabledRuleCandidates)
	if err == nil {
		t.Error("expected validation error for disabled rule")
	}

	disabledCatCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "movement",
			Title:            "Vận động",
			Description:      "Vận động",
			Difficulty:       "medium",
			EstimatedMinutes: 10,
			XPReward:         10,
			ReminderTime:     "10:00",
		},
	}
	err = validator.Validate(qctx, disabledCatCandidates)
	if err == nil {
		t.Error("expected validation error for disabled category")
	}

	invalidDiffCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "sleep",
			Title:            "Đi ngủ",
			Description:      "Đi ngủ",
			Difficulty:       "super-hard",
			EstimatedMinutes: 5,
			XPReward:         10,
			ReminderTime:     "08:30",
		},
	}
	err = validator.Validate(qctx, invalidDiffCandidates)
	if err == nil {
		t.Error("expected validation error for invalid difficulty")
	}

	duplicateCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "sleep",
			Title:            "Đi ngủ",
			Description:      "Đi ngủ 1",
			Difficulty:       "easy",
			EstimatedMinutes: 5,
			XPReward:         5,
			ReminderTime:     "08:30",
		},
		{
			Type:             "sleep",
			Title:            "Đi ngủ",
			Description:      "Đi ngủ 2",
			Difficulty:       "easy",
			EstimatedMinutes: 5,
			XPReward:         5,
			ReminderTime:     "09:30",
		},
	}
	err = validator.Validate(qctx, duplicateCandidates)
	if err == nil {
		t.Error("expected validation error for duplicate title in request")
	}

	existingTitleCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "sleep",
			Title:            "Quest đã tồn tại",
			Description:      "Đi ngủ",
			Difficulty:       "easy",
			EstimatedMinutes: 5,
			XPReward:         5,
			ReminderTime:     "08:30",
		},
	}
	err = validator.Validate(qctx, existingTitleCandidates)
	if err == nil {
		t.Error("expected validation error for duplicate existing quest title")
	}

	tooManyCandidates := []quest_generation.QuestCandidate{
		{Type: "sleep", Title: "Quest 1", Difficulty: "easy", EstimatedMinutes: 5, XPReward: 5, ReminderTime: "08:30"},
		{Type: "sleep", Title: "Quest 2", Difficulty: "easy", EstimatedMinutes: 5, XPReward: 5, ReminderTime: "09:30"},
		{Type: "sleep", Title: "Quest 3", Difficulty: "easy", EstimatedMinutes: 5, XPReward: 5, ReminderTime: "10:30"},
		{Type: "sleep", Title: "Quest 4", Difficulty: "easy", EstimatedMinutes: 5, XPReward: 5, ReminderTime: "11:30"},
	}
	err = validator.Validate(qctx, tooManyCandidates)
	if err == nil {
		t.Error("expected validation error for exceeding daily_quest_count")
	}

	invalidTimeCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "sleep",
			Title:            "Đi ngủ",
			Description:      "Đi ngủ",
			Difficulty:       "easy",
			EstimatedMinutes: 5,
			XPReward:         5,
			ReminderTime:     "25:00",
		},
	}
	err = validator.Validate(qctx, invalidTimeCandidates)
	if err == nil {
		t.Error("expected validation error for invalid reminder_time format")
	}

	outsideRangeCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "sleep",
			Title:            "Đi ngủ",
			Description:      "Đi ngủ",
			Difficulty:       "easy",
			EstimatedMinutes: 5,
			XPReward:         5,
			ReminderTime:     "07:30",
		},
	}
	err = validator.Validate(qctx, outsideRangeCandidates)
	if err == nil {
		t.Error("expected validation error for reminder_time outside active_time_range")
	}

	afterQuietTimeCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "sleep",
			Title:            "Đi ngủ đêm",
			Description:      "Đi ngủ",
			Difficulty:       "easy",
			EstimatedMinutes: 5,
			XPReward:         5,
			ReminderTime:     "22:30",
		},
	}
	err = validator.Validate(qctx, afterQuietTimeCandidates)
	if err == nil {
		t.Error("expected validation error for reminder_time after quiet_after_time")
	}

	// Sleep rule rejection test case: sleep rule 22:00-23:30, rejects 21:30
	sleepQctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN(),
		DailyQuestCount:   3,
		Difficulty:        "normal",
		EnabledCategories: []string{"sleep"},
		QuietAfterTime:    "23:59",
		Rules: []quest_generation.QuestRuleContext{
			{
				ID:      "rule_sleep",
				Type:    "sleep",
				Enabled: true,
				ActiveTimeRange: &quest_generation.TimeRangeContext{
					Start: "22:00",
					End:   "23:30",
				},
			},
		},
	}
	sleepOutsideCandidates := []quest_generation.QuestCandidate{
		{
			Type:             "sleep",
			Title:            "Tắt đèn và thiết bị",
			Description:      "Đi ngủ",
			Difficulty:       "easy",
			EstimatedMinutes: 10,
			XPReward:         5,
			ReminderTime:     "21:30",
		},
	}
	err = validator.Validate(sleepQctx, sleepOutsideCandidates)
	if err == nil {
		t.Error("expected validation error for sleep quest at 21:30 with sleep rule 22:00-23:30")
	}
}

func TestCandidateMapper_Map(t *testing.T) {
	// Use tomorrow so the "past reminder → next safe slot" safety check never fires,
	// letting us assert the exact mapped time.
	tomorrow := timeutil.TodayVN().AddDate(0, 0, 1)
	qctx := &quest_generation.UserQuestContext{
		UserID:    uuid.New(),
		LocalDate: tomorrow,
	}

	candidate := quest_generation.QuestCandidate{
		Type:             "water",
		Title:            "Uống nước",
		Description:      "Uống cốc nước đầy",
		Difficulty:       "easy",
		EstimatedMinutes: 10,
		XPReward:         5,
		Tags:             []string{"sức khỏe", "nước"},
		Reason:           "Để cơ thể khỏe mạnh",
		Instruction:      "Uống 250ml",
		ReminderTime:     "08:30",
	}

	quest, err := quest_generation.MapCandidateToQuest(qctx, candidate)
	if err != nil {
		t.Fatalf("unexpected mapping error: %v", err)
	}

	if quest.UserID != qctx.UserID {
		t.Errorf("expected UserID %v, got %v", qctx.UserID, quest.UserID)
	}
	if quest.Title != "Uống nước" {
		t.Errorf("expected Title 'Uống nước', got '%s'", quest.Title)
	}
	if quest.Description != "Uống cốc nước đầy" {
		t.Errorf("expected Description, got '%s'", quest.Description)
	}
	if quest.Type != models.QuestTypeWater {
		t.Errorf("expected Type water, got '%s'", quest.Type)
	}
	if quest.Status != models.QuestStatusPending {
		t.Errorf("expected status pending, got '%s'", quest.Status)
	}
	if quest.Difficulty != models.QuestDifficultyEasy {
		t.Errorf("expected difficulty easy, got '%s'", quest.Difficulty)
	}
	if quest.Source != models.QuestSourceAI {
		t.Errorf("expected source AI, got '%s'", quest.Source)
	}
	if quest.XPReward != 5 {
		t.Errorf("expected XP 5, got %d", quest.XPReward)
	}
	if quest.EstimatedMinutes != 10 {
		t.Errorf("expected EstimatedMinutes 10, got %d", quest.EstimatedMinutes)
	}
	if quest.Reason != "Để cơ thể khỏe mạnh" || quest.Instruction != "Uống 250ml" {
		t.Errorf("unexpected Reason/Instruction")
	}

	expectedTime := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 8, 30, 0, 0, timeutil.LocationVN)
	if !quest.ReminderTime.Equal(expectedTime) {
		t.Errorf("expected ReminderTime %v, got %v", expectedTime, quest.ReminderTime)
	}
}

func TestQuestGeneration_WaterMovementSleep(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Set up reminder settings specifically for this split test
	db.Exec("DELETE FROM reminder_settings WHERE user_id = ?", userID)

	min90 := 90
	max8 := 8
	max3 := 3
	start08 := "08:00"
	end22 := "22:00"
	start09 := "09:00"
	end18 := "18:00"
	start10 := "10:00"
	end17 := "17:00"
	start20 := "20:00"
	start22 := "22:30"
	start21 := "21:30"

	reminders := []models.ReminderSetting{
		{
			UserID:          userID,
			Type:            models.ReminderTypeWater,
			Title:           "Uống nước",
			Frequency:       models.ReminderFrequencyInterval,
			Status:          models.ReminderStatusEnabled,
			StartTime:       &start08,
			EndTime:         &end22,
			IntervalMinutes: &min90,
			MaxPerDay:       &max8,
		},
		{
			UserID:          userID,
			Type:            models.ReminderTypeBreakTime,
			Title:           "Nghỉ giải lao",
			Frequency:       models.ReminderFrequencyInterval,
			Status:          models.ReminderStatusEnabled,
			StartTime:       &start09,
			EndTime:         &end18,
			IntervalMinutes: &min90,
		},
		{
			UserID:    userID,
			Type:      models.ReminderTypeMovement,
			Title:     "Vận động nhẹ",
			Frequency: models.ReminderFrequencyRandomInRange,
			Status:    models.ReminderStatusEnabled,
			StartTime: &start10,
			EndTime:   &end17,
			MaxPerDay: &max3,
		},
		{
			UserID:    userID,
			Type:      models.ReminderTypeLearning,
			Title:     "Học tập",
			Frequency: models.ReminderFrequencyFixed,
			Status:    models.ReminderStatusEnabled,
			StartTime: &start20,
		},
		{
			UserID:    userID,
			Type:      models.ReminderTypeSleep,
			Title:     "Chuẩn bị ngủ",
			Frequency: models.ReminderFrequencyFixed,
			Status:    models.ReminderStatusEnabled,
			StartTime: &start22,
		},
		{
			UserID:    userID,
			Type:      models.ReminderTypeDailyReview,
			Title:     "Tổng kết ngày",
			Frequency: models.ReminderFrequencyFixed,
			Status:    models.ReminderStatusEnabled,
			StartTime: &start21,
		},
	}
	for _, r := range reminders {
		db.Create(&r)
	}

	// Set up Quest Settings with enabled categories
	catsJSON, _ := json.Marshal([]string{"water", "breakTime", "movement", "learning", "sleep", "review"})
	defaultRules := []dto.QuestRuleResponse{
		{ID: "rule_water", Type: "water", Enabled: true, Difficulty: "easy", MaxPerDay: &max8, ActiveTimeRange: &dto.TimeRangeResponse{Start: "08:00", End: "22:00"}},
		{ID: "rule_break_time", Type: "breakTime", Enabled: true, Difficulty: "easy", MaxPerDay: &max8, ActiveTimeRange: &dto.TimeRangeResponse{Start: "09:00", End: "18:00"}},
		{ID: "rule_movement", Type: "movement", Enabled: true, Difficulty: "medium", MaxPerDay: &max3, ActiveTimeRange: &dto.TimeRangeResponse{Start: "10:00", End: "17:00"}},
		{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium", MaxPerDay: &max3, ActiveTimeRange: &dto.TimeRangeResponse{Start: "19:00", End: "22:00"}},
		{ID: "rule_sleep", Type: "sleep", Enabled: true, Difficulty: "easy", MaxPerDay: &max8, ActiveTimeRange: &dto.TimeRangeResponse{Start: "22:00", End: "23:30"}},
		{ID: "rule_review", Type: "review", Enabled: true, Difficulty: "easy", MaxPerDay: &max8, ActiveTimeRange: &dto.TimeRangeResponse{Start: "21:00", End: "23:00"}},
	}
	rulesJSON, _ := json.Marshal(defaultRules)
	settings := models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   5,
		Difficulty:        "normal",
		EnabledCategories: datatypes.JSON(catsJSON),
		PreferredDuration: "medium",
		Rules:             datatypes.JSON(rulesJSON),
	}
	db.Create(&settings)

	// Build user quest context
	builder := quest_generation.NewUserQuestContextBuilder(db)
	qctx, err := builder.Build(context.Background(), userID, timeutil.TodayVN())
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	// Generate quests via RuleBasedGenerator
	generator := quest_generation.NewRuleBasedGenerator(db)
	quests, err := generator.GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("failed to generate quests: %v", err)
	}

	// Assertions based on reminder types behavior rules
	waterCount := 0
	breakCount := 0
	movementCount := 0
	learningCount := 0
	sleepCount := 0
	reviewCount := 0

	for _, q := range quests {
		switch string(q.Type) {
		case "water":
			waterCount++
		case "breakTime":
			breakCount++
		case "movement":
			movementCount++
			if q.Title != "Vận động nhẹ 10 phút" {
				t.Errorf("expected movement quest title 'Vận động nhẹ 10 phút', got '%s'", q.Title)
			}
		case "learning":
			learningCount++
		case "sleep":
			sleepCount++
		case "review":
			reviewCount++
		}
	}

	// 1. Water is a reminder habit and must never be generated as a daily quest.
	if waterCount != 0 {
		t.Errorf("expected 0 water quests (water is never generated), got %d", waterCount)
	}
	// 2. break_time can generate a quest when explicitly enabled
	if breakCount == 0 {
		t.Errorf("expected at least 1 breakTime quest when enabled, got %d", breakCount)
	}
	// 3. movement from random_in_range has at most 1 movement quest
	if movementCount > 1 {
		t.Errorf("expected at most 1 movement quest, got %d", movementCount)
	}
	// 4. learning and sleep and review are generated normally
	t.Logf("Counts - learning: %d, sleep: %d, review: %d", learningCount, sleepCount, reviewCount)
}
