package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

func TestLearningRoadmapIntegration_WithActiveRoadmap(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	ctx := context.Background()
	userID := uuid.New()

	// Create user profile
	profile := models.UserProfile{
		ID:          userID,
		DisplayName: "Test User",
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create learning roadmap
	roadmap := models.LearningRoadmap{
		ID:               uuid.New(),
		Title:            "Backend Development",
		Description:      "Learn backend development",
		Category:         "programming",
		Difficulty:       "normal",
		EstimatedMinutes: 600,
		TotalSteps:       5,
		Source:           models.LearningRoadmapSourceSystem,
		Enabled:          true,
	}
	if err := db.Create(&roadmap).Error; err != nil {
		t.Fatalf("Failed to create roadmap: %v", err)
	}

	// Create roadmap steps
	step1 := models.LearningRoadmapStep{
		ID:               uuid.New(),
		RoadmapID:        roadmap.ID,
		Title:            "Setup Development Environment",
		Description:      "Install and configure tools",
		OrderIndex:       0,
		EstimatedMinutes: 60,
		Enabled:          true,
	}
	step2 := models.LearningRoadmapStep{
		ID:               uuid.New(),
		RoadmapID:        roadmap.ID,
		Title:            "Learn Go Basics",
		Description:      "Variables, functions, and control flow",
		OrderIndex:       1,
		EstimatedMinutes: 120,
		Enabled:          true,
	}
	if err := db.Create(&step1).Error; err != nil {
		t.Fatalf("Failed to create step1: %v", err)
	}
	if err := db.Create(&step2).Error; err != nil {
		t.Fatalf("Failed to create step2: %v", err)
	}

	// User starts tracking the roadmap
	userRoadmap := models.UserLearningRoadmap{
		ID:        uuid.New(),
		UserID:    userID,
		RoadmapID: roadmap.ID,
		Status:    models.UserLearningRoadmapStatusTracking,
		StartedAt: time.Now(),
	}
	if err := db.Create(&userRoadmap).Error; err != nil {
		t.Fatalf("Failed to create user roadmap: %v", err)
	}

	// User completes step 1
	progress1 := models.UserLearningRoadmapStepProgress{
		ID:          uuid.New(),
		UserID:      userID,
		RoadmapID:   roadmap.ID,
		StepID:      step1.ID,
		Completed:   true,
		CompletedAt: timePtr(time.Now()),
	}
	if err := db.Create(&progress1).Error; err != nil {
		t.Fatalf("Failed to create progress: %v", err)
	}

	// Create quest settings with learning enabled
	enabledCats := []string{"learning", "movement", "sleep", "review"}
	enabledCatsJSON, _ := json.Marshal(enabledCats)

	// Create default rules
	min90 := 90
	max2 := 2
	max1 := 1
	rules := []map[string]interface{}{
		{
			"id":                   "rule_learning",
			"type":                 "learning",
			"title":                "Học tập",
			"description":          "Quest học tập theo mục tiêu của bạn",
			"enabled":              true,
			"difficulty":           "medium",
			"min_interval_minutes": min90,
			"max_per_day":          max2,
			"active_time_range":    map[string]string{"start": "19:00", "end": "22:00"},
			"active_weekdays":      []int{1, 2, 3, 4, 5, 6, 7},
			"priority":             4,
		},
		{
			"id":                   "rule_sleep",
			"type":                 "sleep",
			"title":                "Giấc ngủ",
			"description":          "Nhắc bạn chuẩn bị ngủ đúng giờ",
			"enabled":              true,
			"difficulty":           "easy",
			"max_per_day":          max1,
			"active_time_range":    map[string]string{"start": "22:00", "end": "23:30"},
			"active_weekdays":      []int{1, 2, 3, 4, 5, 6, 7},
			"priority":             3,
		},
	}
	rulesJSON, _ := json.Marshal(rules)

	settings := models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   6,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: datatypes.JSON(enabledCatsJSON),
		Rules:             datatypes.JSON(rulesJSON),
		RestDayEnabled:    false,
	}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("Failed to create settings: %v", err)
	}

	// Build context
	builder := quest_generation.NewUserQuestContextBuilder(db)
	today := timeutil.TodayVN()
	qctx, err := builder.Build(ctx, userID, today)
	if err != nil {
		t.Fatalf("Failed to build context: %v", err)
	}

	// Verify roadmap context is loaded
	if qctx.ActiveLearningPath == nil {
		t.Fatal("Expected ActiveLearningPath to be loaded, got nil")
	}
	if qctx.ActiveLearningPath.RoadmapID != roadmap.ID.String() {
		t.Errorf("Expected roadmap ID %s, got %s", roadmap.ID, qctx.ActiveLearningPath.RoadmapID)
	}
	if qctx.ActiveLearningPath.StepID != step2.ID.String() {
		t.Errorf("Expected current step ID %s, got %s", step2.ID, qctx.ActiveLearningPath.StepID)
	}
	if qctx.ActiveLearningPath.CurrentStepTitle != step2.Title {
		t.Errorf("Expected current step title %s, got %s", step2.Title, qctx.ActiveLearningPath.CurrentStepTitle)
	}
	if qctx.ActiveLearningPath.CompletedSteps != 1 {
		t.Errorf("Expected 1 completed step, got %d", qctx.ActiveLearningPath.CompletedSteps)
	}
	if qctx.ActiveLearningPath.TotalSteps != 2 {
		t.Errorf("Expected 2 total steps, got %d", qctx.ActiveLearningPath.TotalSteps)
	}

	// Generate quests using rule-based generator
	ruleGen := quest_generation.NewRuleBasedGenerator(db)
	quests, err := ruleGen.GenerateDailyQuests(ctx, qctx)
	if err != nil {
		t.Fatalf("Failed to generate quests: %v", err)
	}

	// Verify at least one learning quest was generated
	learningQuests := 0
	linkedQuests := 0
	for _, q := range quests {
		if q.Type == models.QuestTypeLearning {
			learningQuests++
			if len(q.LearningMetadata) > 0 {
				linkedQuests++
				var meta map[string]interface{}
				json.Unmarshal(q.LearningMetadata, &meta)
				if meta["learning_roadmap_id"] != roadmap.ID.String() {
					t.Errorf("Expected learning_roadmap_id %s, got %v", roadmap.ID, meta["learning_roadmap_id"])
				}
				if meta["learning_step_id"] != step2.ID.String() {
					t.Errorf("Expected learning_step_id %s, got %v", step2.ID, meta["learning_step_id"])
				}
			}
		}
	}

	if learningQuests == 0 {
		t.Error("Expected at least one learning quest to be generated")
	}
	if linkedQuests == 0 {
		t.Error("Expected at least one learning quest to be linked to roadmap")
	}
}

func TestLearningRoadmapIntegration_NoActiveRoadmap(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	ctx := context.Background()
	userID := uuid.New()

	// Create user profile
	profile := models.UserProfile{
		ID:          userID,
		DisplayName: "Test User",
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create quest settings with learning enabled
	enabledCats := []string{"learning", "movement", "sleep", "review"}
	enabledCatsJSON, _ := json.Marshal(enabledCats)
	settings := models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   6,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: datatypes.JSON(enabledCatsJSON),
		RestDayEnabled:    false,
	}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("Failed to create settings: %v", err)
	}

	// Build context
	builder := quest_generation.NewUserQuestContextBuilder(db)
	today := timeutil.TodayVN()
	qctx, err := builder.Build(ctx, userID, today)
	if err != nil {
		t.Fatalf("Failed to build context: %v", err)
	}

	// Verify no roadmap context
	if qctx.ActiveLearningPath != nil {
		t.Error("Expected no ActiveLearningPath, but got one")
	}

	// Generate quests using rule-based generator
	ruleGen := quest_generation.NewRuleBasedGenerator(db)
	quests, err := ruleGen.GenerateDailyQuests(ctx, qctx)
	if err != nil {
		t.Fatalf("Failed to generate quests: %v", err)
	}

	// Verify learning quests have no roadmap metadata
	for _, q := range quests {
		if q.Type == models.QuestTypeLearning {
			if len(q.LearningMetadata) > 0 {
				t.Error("Expected no learning metadata when no roadmap exists")
			}
		}
	}
}

func TestLearningRoadmapIntegration_LearningDisabled(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	ctx := context.Background()
	userID := uuid.New()

	// Create user profile
	profile := models.UserProfile{
		ID:          userID,
		DisplayName: "Test User",
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create roadmap but learning is disabled
	roadmap := models.LearningRoadmap{
		ID:               uuid.New(),
		Title:            "Backend Development",
		Category:         "programming",
		TotalSteps:       1,
		Source:           models.LearningRoadmapSourceSystem,
		Enabled:          true,
	}
	if err := db.Create(&roadmap).Error; err != nil {
		t.Fatalf("Failed to create roadmap: %v", err)
	}

	step := models.LearningRoadmapStep{
		ID:         uuid.New(),
		RoadmapID:  roadmap.ID,
		Title:      "Step 1",
		OrderIndex: 0,
		Enabled:    true,
	}
	if err := db.Create(&step).Error; err != nil {
		t.Fatalf("Failed to create step: %v", err)
	}

	userRoadmap := models.UserLearningRoadmap{
		ID:        uuid.New(),
		UserID:    userID,
		RoadmapID: roadmap.ID,
		Status:    models.UserLearningRoadmapStatusTracking,
		StartedAt: time.Now(),
	}
	if err := db.Create(&userRoadmap).Error; err != nil {
		t.Fatalf("Failed to create user roadmap: %v", err)
	}

	// Create quest settings WITHOUT learning enabled
	enabledCats := []string{"movement", "sleep", "review"}
	enabledCatsJSON, _ := json.Marshal(enabledCats)
	settings := models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   6,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: datatypes.JSON(enabledCatsJSON),
		RestDayEnabled:    false,
	}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("Failed to create settings: %v", err)
	}

	// Build context
	builder := quest_generation.NewUserQuestContextBuilder(db)
	today := timeutil.TodayVN()
	qctx, err := builder.Build(ctx, userID, today)
	if err != nil {
		t.Fatalf("Failed to build context: %v", err)
	}

	// Generate quests
	ruleGen := quest_generation.NewRuleBasedGenerator(db)
	quests, err := ruleGen.GenerateDailyQuests(ctx, qctx)
	if err != nil {
		t.Fatalf("Failed to generate quests: %v", err)
	}

	// Verify no learning quests generated
	for _, q := range quests {
		if q.Type == models.QuestTypeLearning {
			t.Error("Expected no learning quests when learning category is disabled")
		}
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
