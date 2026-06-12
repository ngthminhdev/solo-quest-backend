package quest_generation_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

	// Set up Quest Settings. Enable several real quest categories (incl. review,
	// the universal soft filler) so the smart/last-resort fallback can fill the
	// daily target using ENABLED categories — quests are never padded from
	// reminder-only or disabled categories.
	catsJSON, _ := json.Marshal([]string{"movement", "learning", "sleep", "review"})
	rules := []dto.QuestRuleResponse{
		{
			ID:             "rule_movement",
			Type:           "movement",
			Title:          "Vận động",
			Description:    "Khuyến khích vận động thể chất",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"},
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       5,
		},
		{
			ID:             "rule_learning",
			Type:           "learning",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"},
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       5,
		},
		{
			ID:             "rule_sleep",
			Type:           "sleep",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveTimeRange: &dto.TimeRangeResponse{Start: "20:00", End: "23:30"},
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       4,
		},
		{
			ID:             "rule_review",
			Type:           "review",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveTimeRange: &dto.TimeRangeResponse{Start: "21:00", End: "23:00"},
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       3,
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

	// Seed an existing quest (target is 5, so we have room for 4 more)
	today := timeutil.TodayVN()
	existingQuest := models.Quest{
		ID:     uuid.New(),
		UserID: userID,
		Title:  "Existing Quest",
		Date:   today,
		Status: models.QuestStatusPending,
	}
	db.Create(&existingQuest)

	// Setup mock AI to return 4 quests
	mockAI.quests = []models.Quest{
		{Title: "AI Quest 1", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "AI Quest 2", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "AI Quest 3", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "AI Quest 4", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
	}

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

	// New behavior: should generate missing quests
	if !result.Inserted {
		t.Error("expected Inserted to be true")
	}
	if result.ExistingReturned {
		t.Error("expected ExistingReturned to be false")
	}
	if result.GeneratedCount != 4 {
		t.Errorf("expected GeneratedCount 4, got %d", result.GeneratedCount)
	}
	if result.PreservedCount != 1 {
		t.Errorf("expected PreservedCount 1, got %d", result.PreservedCount)
	}
	if !mockAI.called {
		t.Error("AI generator should have been called")
	}

	// Verify total count is 5 (1 existing + 4 generated)
	var count int64
	db.Model(&models.Quest{}).Where("user_id = ?", userID).Count(&count)
	if count != 5 {
		t.Errorf("expected 5 quests in database, got %d", count)
	}
}

func TestGenerationService_GenerateToday_AISuccess(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	// AI fully satisfies the daily target (5), so no fallback fill is needed.
	mockAI.quests = []models.Quest{
		{Title: "AI Quest 1", Source: models.QuestSourceAI, Date: today},
		{Title: "AI Quest 2", Source: models.QuestSourceAI, Date: today},
		{Title: "AI Quest 3", Source: models.QuestSourceAI, Date: today},
		{Title: "AI Quest 4", Source: models.QuestSourceAI, Date: today},
		{Title: "AI Quest 5", Source: models.QuestSourceAI, Date: today},
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
	if result.GeneratedCount != 5 {
		t.Errorf("expected GeneratedCount 5, got %d", result.GeneratedCount)
	}

	// Check DB
	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 5 {
		t.Fatalf("expected 5 quests in DB, got %d", len(dbQuests))
	}
	for _, q := range dbQuests {
		if q.Source != models.QuestSourceAI {
			t.Errorf("expected source to be ai, got %s", q.Source)
		}
	}
}

func TestGenerationService_GenerateToday_AIFailsFallbackSucceeds(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	mockAI.err = errors.New("candidate validation failed")

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
	// AI produced zero valid quests, so the day is filled by the smart fallback;
	// the reported source is rule_based (non-AI generated).
	if result.Source != "rule_based" {
		t.Errorf("expected Source 'rule_based', got %s", result.Source)
	}
	if !result.FallbackUsed {
		t.Error("expected FallbackUsed to be true")
	}
	if result.AIErrorType != "validation_failed" {
		t.Errorf("expected AIErrorType 'validation_failed', got %s", result.AIErrorType)
	}
	// The job no longer fails on AI failure: the smart fallback fills the full
	// target (5) deterministically.
	if result.GeneratedCount != 5 {
		t.Errorf("expected GeneratedCount 5 (smart fallback fill), got %d", result.GeneratedCount)
	}

	// Check DB
	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 5 {
		t.Fatalf("expected 5 quests in DB, got %d", len(dbQuests))
	}
}

func TestGenerationService_GenerateToday_PreferRuleBasedDirectly(t *testing.T) {
	db, userID, service, mockAI, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	// Rule-based fully satisfies the daily target (5), so no smart-fallback fill
	// is needed and fallback_used stays false.
	mockRule.quests = []models.Quest{
		{Title: "Rule Quest 1", Source: models.QuestSourceConfigBased, Date: today},
		{Title: "Rule Quest 2", Source: models.QuestSourceConfigBased, Date: today},
		{Title: "Rule Quest 3", Source: models.QuestSourceConfigBased, Date: today},
		{Title: "Rule Quest 4", Source: models.QuestSourceConfigBased, Date: today},
		{Title: "Rule Quest 5", Source: models.QuestSourceConfigBased, Date: today},
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
	// Remaining capacity is 4 (target 5 - 1 preserved). AI returns 2, the smart
	// fallback fills the remaining 2 so exactly the capacity is met.
	if result.GeneratedCount != 4 {
		t.Errorf("expected GeneratedCount 4, got %d", result.GeneratedCount)
	}

	// Verify DB state
	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 5 { // 1 preserved completed + 4 newly generated = 5 total
		t.Fatalf("expected 5 total quests in DB, got %d", len(dbQuests))
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

// TestGenerationService_GenerateToday_SkipsAdvisoryLockOnSQLite is a regression
// test for the bug where pg_advisory_xact_lock() (which returns void) was scanned
// into a bool, producing: sql: Scan error ... couldn't convert "" into type bool.
//
// The advisory lock must only run on PostgreSQL; unit tests run on SQLite where
// the lock is skipped entirely. This test asserts the test DB is SQLite and that
// GenerateToday completes without the historical bool-scan failure.
func TestGenerationService_GenerateToday_SkipsAdvisoryLockOnSQLite(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	// Guard: unit tests must run on SQLite, never attempt pg_advisory_xact_lock.
	if dialect := db.Dialector.Name(); dialect != "sqlite" {
		t.Fatalf("expected sqlite test dialect, got %q", dialect)
	}

	today := timeutil.TodayVN()
	mockAI.quests = []models.Quest{
		{Title: "AI Quest 1", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "AI Quest 2", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
	}

	preferAI := true
	force := false
	req := quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
		Force:    &force,
	}

	result, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		// Specifically catch the original regression.
		if strings.Contains(err.Error(), "couldn't convert") {
			t.Fatalf("advisory lock bool-scan regression resurfaced: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Inserted {
		t.Error("expected Inserted to be true")
	}
	if !mockAI.called {
		t.Error("AI generator should have been called")
	}
}
