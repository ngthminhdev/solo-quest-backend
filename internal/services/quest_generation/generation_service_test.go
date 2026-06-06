package quest_generation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

type mockGenerator struct {
	quests []models.Quest
	err    error
	called bool
}

func (m *mockGenerator) GenerateDailyQuests(ctx context.Context, qctx *quest_generation.UserQuestContext) ([]models.Quest, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	// Return copy to avoid modifying original mock state
	copied := make([]models.Quest, len(m.quests))
	copy(copied, m.quests)
	return copied, nil
}

func setupServiceTest(t *testing.T) (*gorm.DB, uuid.UUID, *quest_generation.GenerationService, *mockGenerator, *mockGenerator) {
	db := testutils.SetupTestDB(t)

	userID := testutils.BootstrapTestUser(t, db)

	// Set up user profile
	var profile models.UserProfile
	if err := db.Where("id = ?", userID).First(&profile).Error; err != nil {
		t.Fatal(err)
	}
	profile.DisplayName = "Test User"
	activity := "Developer"
	profile.MainActivity = &activity
	profile.MainGoals = datatypes.JSON([]byte(`["Uống nước"]`))
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
		"learning_time_preferences": []string{"evening"},
		"movement_time_preference":  "morning",
		"movement_time_preferences": []string{"morning"},
	}
	obJSON, _ := json.Marshal(obAnswers)
	onboarding := models.OnboardingAnswer{
		UserID:    userID,
		Answers:   datatypes.JSON(obJSON),
		Completed: true,
	}
	db.Create(&onboarding)

	// Set up Quest Settings
	catsJSON, _ := json.Marshal([]string{"water", "learning"})
	rules := []dto.QuestRuleResponse{
		{
			ID:             "rule_water",
			Type:           "water",
			Title:          "Uống nước",
			Description:    "Nhắc uống nước",
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

	// Mock Generators
	mockAI := &mockGenerator{}
	mockRule := &mockGenerator{}

	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, mockRule)

	return db, userID, service, mockAI, mockRule
}

func TestGenerationService_GenerateToday_ForceFalse(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	// Seed an existing quest
	today := timeutil.TodayVN()
	existingQuest := models.Quest{
		ID:     uuid.New(),
		UserID: userID,
		Title:  "Existing Quest",
		Date:   today,
		Status: models.QuestStatusPending,
	}
	db.Create(&existingQuest)

	preferAI := true
	force := false
	req := quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
		Force:    &force,
	}

	result, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inserted {
		t.Error("expected Inserted to be false")
	}
	if !result.ExistingReturned {
		t.Error("expected ExistingReturned to be true")
	}
	if result.Source != "existing" {
		t.Errorf("expected Source 'existing', got %s", result.Source)
	}
	if mockAI.called {
		t.Error("AI generator should not have been called")
	}

	// Verify no duplicates created
	var count int64
	db.Model(&models.Quest{}).Where("user_id = ?", userID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 quest in database, got %d", count)
	}
}

func TestGenerationService_GenerateToday_AISuccess(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	mockAI.quests = []models.Quest{
		{
			Title:  "AI Quest 1",
			Source: models.QuestSourceAI,
			Date:   today,
		},
		{
			Title:  "AI Quest 2",
			Source: models.QuestSourceAI,
			Date:   today,
		},
	}

	preferAI := true
	req := quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
	}

	result, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Inserted {
		t.Error("expected Inserted to be true")
	}
	if result.ExistingReturned {
		t.Error("expected ExistingReturned to be false")
	}
	if result.Source != "ai" {
		t.Errorf("expected Source 'ai', got %s", result.Source)
	}
	if result.FallbackUsed {
		t.Error("expected FallbackUsed to be false")
	}
	if result.GeneratedCount != 2 {
		t.Errorf("expected GeneratedCount 2, got %d", result.GeneratedCount)
	}

	// Check DB
	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 2 {
		t.Fatalf("expected 2 quests in DB, got %d", len(dbQuests))
	}
	for _, q := range dbQuests {
		if q.Source != models.QuestSourceAI {
			t.Errorf("expected source to be ai, got %s", q.Source)
		}
	}
}

func TestGenerationService_GenerateToday_AIFailsFallbackSucceeds(t *testing.T) {
	db, userID, service, mockAI, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	mockAI.err = errors.New("candidate validation failed")

	mockRule.quests = []models.Quest{
		{
			Title:  "Rule Quest 1",
			Source: models.QuestSourceConfigBased,
			Date:   today,
		},
	}

	preferAI := true
	req := quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
	}

	result, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Inserted {
		t.Error("expected Inserted to be true")
	}
	if result.Source != "rule_based" {
		t.Errorf("expected Source 'rule_based', got %s", result.Source)
	}
	if !result.FallbackUsed {
		t.Error("expected FallbackUsed to be true")
	}
	if result.AIErrorType != "validation_failed" {
		t.Errorf("expected AIErrorType 'validation_failed', got %s", result.AIErrorType)
	}
	if result.GeneratedCount != 1 {
		t.Errorf("expected GeneratedCount 1, got %d", result.GeneratedCount)
	}

	// Check DB
	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 1 {
		t.Fatalf("expected 1 quest in DB, got %d", len(dbQuests))
	}
}

func TestGenerationService_GenerateToday_PreferRuleBasedDirectly(t *testing.T) {
	db, userID, service, mockAI, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	mockRule.quests = []models.Quest{
		{
			Title:  "Rule Quest 1",
			Source: models.QuestSourceConfigBased,
			Date:   today,
		},
	}

	preferAI := false
	req := quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
	}

	result, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Inserted {
		t.Error("expected Inserted to be true")
	}
	if result.Source != "rule_based" {
		t.Errorf("expected Source 'rule_based', got %s", result.Source)
	}
	if result.FallbackUsed {
		t.Error("expected FallbackUsed to be false")
	}
	if mockAI.called {
		t.Error("AI generator should not have been called")
	}
}

func TestGenerationService_GenerateToday_ForceTrueReplacePendingOnly(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// Seed 1 completed quest (should be preserved) and 2 pending quests (should be replaced)
	preservedQuest := models.Quest{
		ID:     uuid.New(),
		UserID: userID,
		Title:  "Preserved Completed Quest",
		Date:   today,
		Status: models.QuestStatusCompleted,
	}
	db.Create(&preservedQuest)

	pendingQuest1 := models.Quest{
		ID:     uuid.New(),
		UserID: userID,
		Title:  "Pending Quest 1",
		Date:   today,
		Status: models.QuestStatusPending,
	}
	db.Create(&pendingQuest1)

	pendingQuest2 := models.Quest{
		ID:     uuid.New(),
		UserID: userID,
		Title:  "Pending Quest 2",
		Date:   today,
		Status: models.QuestStatusPending,
	}
	db.Create(&pendingQuest2)

	// Since DailyQuestCount=5, and preservedCount=1, remainingCapacity should be 5-1 = 4.
	mockAI.quests = []models.Quest{
		{
			Title:  "New AI Quest 1",
			Source: models.QuestSourceAI,
			Date:   today,
		},
		{
			Title:  "New AI Quest 2",
			Source: models.QuestSourceAI,
			Date:   today,
		},
	}

	preferAI := true
	force := true
	req := quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
		Force:    &force,
	}

	result, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Inserted {
		t.Error("expected Inserted to be true")
	}
	if result.PreservedCount != 1 {
		t.Errorf("expected PreservedCount 1, got %d", result.PreservedCount)
	}
	if result.ReplacedPendingCount != 2 {
		t.Errorf("expected ReplacedPendingCount 2, got %d", result.ReplacedPendingCount)
	}
	if result.GeneratedCount != 2 {
		t.Errorf("expected GeneratedCount 2, got %d", result.GeneratedCount)
	}

	// Verify DB state
	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 3 { // 1 preserved completed + 2 newly generated = 3 total
		t.Fatalf("expected 3 total quests in DB, got %d", len(dbQuests))
	}

	// Verify pending quests were deleted
	var deletedPendingCount int64
	db.Model(&models.Quest{}).Where("id IN ?", []uuid.UUID{pendingQuest1.ID, pendingQuest2.ID}).Count(&deletedPendingCount)
	if deletedPendingCount != 0 {
		t.Error("pending quests were not deleted")
	}

	// Verify completed quest is still there
	var preservedCheck models.Quest
	if err := db.First(&preservedCheck, preservedQuest.ID).Error; err != nil {
		t.Fatal("preserved quest was deleted or cannot be found:", err)
	}
}

func TestGenerationService_GenerateToday_ForceTruePreservedExceedsCapacity(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// Seed 6 completed quests (exceeds DailyQuestCount=5)
	for i := 0; i < 6; i++ {
		q := models.Quest{
			ID:     uuid.New(),
			UserID: userID,
			Title:  "Preserved Completed",
			Date:   today,
			Status: models.QuestStatusCompleted,
		}
		db.Create(&q)
	}

	// Seed 1 pending quest
	pendingQuest := models.Quest{
		ID:     uuid.New(),
		UserID: userID,
		Title:  "Pending Quest to delete",
		Date:   today,
		Status: models.QuestStatusPending,
	}
	db.Create(&pendingQuest)

	preferAI := true
	force := true
	req := quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
		Force:    &force,
	}

	result, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inserted {
		t.Error("expected Inserted to be false")
	}
	if !result.ExistingReturned {
		t.Error("expected ExistingReturned to be true")
	}
	if result.PreservedCount != 6 {
		t.Errorf("expected PreservedCount 6, got %d", result.PreservedCount)
	}
	if result.ReplacedPendingCount != 1 {
		t.Errorf("expected ReplacedPendingCount 1, got %d", result.ReplacedPendingCount)
	}
	if result.GeneratedCount != 0 {
		t.Errorf("expected GeneratedCount 0, got %d", result.GeneratedCount)
	}
	if mockAI.called {
		t.Error("AI generator should not have been called since capacity was 0")
	}

	// Verify DB state
	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 6 { // 6 preserved, 1 pending deleted, 0 generated
		t.Errorf("expected 6 quests in DB, got %d", len(dbQuests))
	}
}

func TestGenerationService_GenerateToday_TransactionRollback(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// Seed a pending quest
	pendingQuest := models.Quest{
		ID:     uuid.New(),
		UserID: userID,
		Title:  "Pending Quest to delete",
		Date:   today,
		Status: models.QuestStatusPending,
	}
	db.Create(&pendingQuest)

	// AI returns duplicate ID which causes GORM unique constraint violation on insert
	dupID := uuid.New()
	mockAI.quests = []models.Quest{
		{
			ID:     dupID,
			Title:  "AI Quest 1",
			Source: models.QuestSourceAI,
			Date:   today,
		},
		{
			ID:     dupID, // Duplicate ID
			Title:  "AI Quest 2",
			Source: models.QuestSourceAI,
			Date:   today,
		},
	}

	preferAI := true
	force := true
	req := quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
		Force:    &force,
	}

	_, err := service.GenerateToday(context.Background(), userID, req)
	if err == nil {
		t.Fatal("expected error due to duplicate ID, got nil")
	}

	// Verify rollback:
	// 1. Pending quest should NOT be deleted
	var checkPending models.Quest
	if err := db.First(&checkPending, pendingQuest.ID).Error; err != nil {
		t.Error("pending quest was deleted despite transaction rollback:", err)
	}

	// 2. Newly generated quests should NOT exist in the database
	var checkNewCount int64
	db.Model(&models.Quest{}).Where("title LIKE ?", "AI Quest%").Count(&checkNewCount)
	if checkNewCount != 0 {
		t.Errorf("found %d generated quests in DB despite transaction rollback", checkNewCount)
	}
}
