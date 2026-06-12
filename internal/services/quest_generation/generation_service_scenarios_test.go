package quest_generation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

// ---------------------------------------------------------------------------
// TestGenerationService_NormalDay
// Verifies the happy path: AI returns a full batch for today, no fallback needed.
// ---------------------------------------------------------------------------

func TestGenerationService_NormalDay(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	mockAI.quests = []models.Quest{
		{ID: uuid.New(), Title: "Uống nước buổi sáng", Type: models.QuestTypeWater, Source: models.QuestSourceAI, Date: today},
		{ID: uuid.New(), Title: "Vận động 10 phút", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
		{ID: uuid.New(), Title: "Học tập 20 phút", Type: models.QuestTypeLearning, Source: models.QuestSourceAI, Date: today},
		{ID: uuid.New(), Title: "Nghỉ ngơi giữa giờ", Type: models.QuestTypeBreak, Source: models.QuestSourceAI, Date: today},
		{ID: uuid.New(), Title: "Chuẩn bị đi ngủ", Type: models.QuestTypeSleep, Source: models.QuestSourceAI, Date: today},
	}

	preferAI := true
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{PreferAI: &preferAI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Inserted {
		t.Error("expected Inserted=true on normal day")
	}
	if result.Source != "ai" {
		t.Errorf("expected Source=ai, got %s", result.Source)
	}
	if result.FallbackUsed {
		t.Error("expected FallbackUsed=false when AI fills the full batch")
	}
	if result.GeneratedCount != 5 {
		t.Errorf("expected 5 quests generated, got %d", result.GeneratedCount)
	}

	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 5 {
		t.Fatalf("expected 5 quests in DB, got %d", len(dbQuests))
	}
}

// ---------------------------------------------------------------------------
// TestGenerationService_RestDay
// On a rest day the rule-based generator caps output below DailyQuestCount.
// The smart fallback then fills the remaining slots so exactly needed_count
// quests are saved.
// ---------------------------------------------------------------------------

func TestGenerationService_RestDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Minimal onboarding answers
	obJSON, _ := json.Marshal(map[string]interface{}{
		"work_start_time":   "09:00",
		"work_end_time":     "18:00",
		"wake_up_time":      "07:00",
		"target_sleep_time": "23:00",
		"quiet_after_time":  "22:00",
	})
	db.Create(&models.OnboardingAnswer{UserID: userID, Answers: datatypes.JSON(obJSON), Completed: true})

	// Settings with DailyQuestCount=5; movement + learning + sleep + review enabled
	// (review is the soft filler so the fallback can reach the target using ENABLED
	// categories only).
	catsJSON, _ := json.Marshal([]string{"movement", "learning", "sleep", "review"})
	rulesJSON, _ := json.Marshal([]dto.QuestRuleResponse{
		{
			ID:             "rule_movement",
			Type:           "movement",
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
	})
	db.Create(&models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   5,
		Difficulty:        "easy",
		EnabledCategories: datatypes.JSON(catsJSON),
		Rules:             datatypes.JSON(rulesJSON),
	})

	// Simulate a rest day by making mockRule return only 2 quests (capped by the
	// rule-based generator's rest-day logic). The smart fallback fills the
	// remaining 3 so exactly the daily target (5) is saved.
	today := timeutil.TodayVN()
	mockRule := &mockGenerator{
		quests: []models.Quest{
			{Title: "Đi bộ nhẹ 5 phút", Type: models.QuestTypeMovement, Source: models.QuestSourceConfigBased, Date: today},
			{Title: "Đọc sách nhẹ nhàng", Type: models.QuestTypeLearning, Source: models.QuestSourceConfigBased, Date: today},
		},
	}
	mockAI := &mockGenerator{err: errForceFallback}

	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, mockRule)

	preferAI := false
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{PreferAI: &preferAI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.GeneratedCount != 5 {
		t.Errorf("rest day: expected 5 quests (2 rule-based + 3 smart fallback), got %d", result.GeneratedCount)
	}

	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 5 {
		t.Errorf("rest day: expected 5 quests in DB, got %d", len(dbQuests))
	}
}

// errForceFallback is a sentinel error to force the fallback path via mockGenerator.
var errForceFallback = fmt.Errorf("forced fallback for test")

// ---------------------------------------------------------------------------
// TestGenerationService_SmartFallbackFillsWithoutDuplicatingLearning
// When AI returns a partial batch including a learning quest, the smart fallback
// fills the remaining slots without exceeding the learning cap (no second
// learning quest when there is no active roadmap), and saves exactly the target.
// ---------------------------------------------------------------------------

func TestGenerationService_SmartFallbackFillsWithoutDuplicatingLearning(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// AI returns 3 quests: movement + learning + sleep → shortfall=2 for DailyQuestCount=5.
	mockAI.quests = []models.Quest{
		{ID: uuid.New(), Title: "Vận động buổi sáng", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
		{ID: uuid.New(), Title: "Học tập chủ đề bạn thích hôm nay", Type: models.QuestTypeLearning, Source: models.QuestSourceAI, Date: today},
		{ID: uuid.New(), Title: "Đi ngủ đúng giờ", Type: models.QuestTypeSleep, Source: models.QuestSourceAI, Date: today},
	}

	preferAI := true
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{PreferAI: &preferAI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 AI + 2 smart fallback = exactly the target (5).
	if result.GeneratedCount != 5 {
		t.Errorf("expected 5 quests (3 AI + 2 smart fallback), got %d", result.GeneratedCount)
	}

	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 5 {
		t.Fatalf("expected 5 quests in DB, got %d", len(dbQuests))
	}

	learningCount := 0
	waterCount := 0
	for _, q := range dbQuests {
		if q.Type == models.QuestTypeLearning {
			learningCount++
		}
		if q.Type == models.QuestTypeWater {
			waterCount++
		}
	}
	// No active roadmap → learning cap is 1, so the fallback must not add a
	// second learning quest.
	if learningCount != 1 {
		t.Errorf("expected exactly 1 learning quest (cap respected), got %d", learningCount)
	}
	if waterCount != 0 {
		t.Errorf("expected 0 water quests, got %d", waterCount)
	}
}

// ---------------------------------------------------------------------------
// TestGenerationService_ReviewIncludedWhenEnabled
// When "review" (or "daily_review") is in EnabledCategories and AI returns a
// review quest, it must be present in the stored result.
// ---------------------------------------------------------------------------

func TestGenerationService_ReviewIncludedWhenEnabled(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	obJSON, _ := json.Marshal(map[string]interface{}{
		"work_start_time":   "09:00",
		"work_end_time":     "18:00",
		"wake_up_time":      "07:00",
		"target_sleep_time": "23:00",
		"quiet_after_time":  "22:00",
	})
	db.Create(&models.OnboardingAnswer{UserID: userID, Answers: datatypes.JSON(obJSON), Completed: true})

	// review is explicitly enabled in categories
	catsJSON, _ := json.Marshal([]string{"movement", "learning", "review"})
	rulesJSON, _ := json.Marshal([]dto.QuestRuleResponse{
		{
			ID:             "rule_movement",
			Type:           "movement",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       5,
		},
		{
			ID:             "rule_review",
			Type:           "review",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       5,
		},
	})
	db.Create(&models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   3,
		Difficulty:        "easy",
		EnabledCategories: datatypes.JSON(catsJSON),
		Rules:             datatypes.JSON(rulesJSON),
	})

	today := timeutil.TodayVN()
	mockAI := &mockGenerator{
		quests: []models.Quest{
			{ID: uuid.New(), Title: "Đi bộ nhẹ", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
			{ID: uuid.New(), Title: "Nhìn lại ngày hôm nay", Type: models.QuestTypeReview, Source: models.QuestSourceAI, Date: today},
			{ID: uuid.New(), Title: "Chuẩn bị đi ngủ", Type: models.QuestTypeSleep, Source: models.QuestSourceAI, Date: today},
		},
	}
	mockRule := &mockGenerator{}

	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, mockRule)

	preferAI := true
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{PreferAI: &preferAI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.GeneratedCount == 0 {
		t.Fatal("expected quests to be generated")
	}

	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)

	foundReview := false
	for _, q := range dbQuests {
		if q.Type == models.QuestTypeReview {
			foundReview = true
			break
		}
	}
	if !foundReview {
		t.Error("expected a review quest in DB when review category is enabled")
	}
}

// ---------------------------------------------------------------------------
// TestGenerationService_BrokenCaseFallback
// Verifies when quest_settings daily_quest_count is missing/0 and user has 2
// pending quests, target_count fallbacks to 6, 4 missing quests are generated,
// and response existing_count == len(quests) == 6.
// ---------------------------------------------------------------------------
func TestGenerationService_BrokenCaseFallback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Minimal onboarding answers
	obJSON, _ := json.Marshal(map[string]interface{}{
		"work_start_time":   "09:00",
		"work_end_time":     "18:00",
		"wake_up_time":      "07:00",
		"target_sleep_time": "23:00",
		"quiet_after_time":  "22:00",
	})
	db.Create(&models.OnboardingAnswer{UserID: userID, Answers: datatypes.JSON(obJSON), Completed: true})

	// User settings has daily_quest_count set to 0 (which triggers fallback)
	catsJSON, _ := json.Marshal([]string{"movement", "learning"})
	rulesJSON, _ := json.Marshal([]dto.QuestRuleResponse{
		{
			ID:             "rule_movement",
			Type:           "movement",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       5,
		},
	})
	settings := &models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   0, // triggers fallback to 6
		Difficulty:        "normal",
		EnabledCategories: datatypes.JSON(catsJSON),
		Rules:             datatypes.JSON(rulesJSON),
	}
	db.Create(settings)
	db.Model(settings).Update("daily_quest_count", 0)

	today := timeutil.TodayVN()

	// Seed 2 pending quests
	db.Create(&models.Quest{ID: uuid.New(), UserID: userID, Title: "Pending 1", Type: models.QuestTypeMovement, Status: models.QuestStatusPending, Date: today})
	db.Create(&models.Quest{ID: uuid.New(), UserID: userID, Title: "Pending 2", Type: models.QuestTypeMovement, Status: models.QuestStatusPending, Date: today})

	// Mock generators: AI returns 4 quests (since target will fallback to 6, capacity = 6 - 2 = 4)
	mockAI := &mockGenerator{
		quests: []models.Quest{
			{ID: uuid.New(), Title: "AI 1", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
			{ID: uuid.New(), Title: "AI 2", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
			{ID: uuid.New(), Title: "AI 3", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
			{ID: uuid.New(), Title: "AI 4", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
		},
	}
	mockRule := &mockGenerator{}

	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, mockRule)

	preferAI := true
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{PreferAI: &preferAI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assertions
	if result.TargetCount != 6 {
		t.Errorf("expected TargetCount=6 (fallback), got %d", result.TargetCount)
	}
	if result.ExistingCount != 6 {
		t.Errorf("expected ExistingCount=6, got %d", result.ExistingCount)
	}
	if len(result.Quests) != 6 {
		t.Errorf("expected len(quests)=6, got %d", len(result.Quests))
	}
	if result.GeneratedCount != 4 {
		t.Errorf("expected GeneratedCount=4, got %d", result.GeneratedCount)
	}
	if result.PreservedCount != 2 {
		t.Errorf("expected PreservedCount=2, got %d", result.PreservedCount)
	}
}

// ---------------------------------------------------------------------------
// TestGenerationService_ResponseBranches
// Verifies all response branches (new generation, existing at target,
// existing below target, force regenerate, explicit date generation)
// and asserts invariants on all branches.
// ---------------------------------------------------------------------------
func TestGenerationService_ResponseBranches(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Onboarding answers
	obJSON, _ := json.Marshal(map[string]interface{}{
		"work_start_time":   "09:00",
		"work_end_time":     "18:00",
		"wake_up_time":      "07:00",
		"target_sleep_time": "23:00",
		"quiet_after_time":  "22:00",
	})
	db.Create(&models.OnboardingAnswer{UserID: userID, Answers: datatypes.JSON(obJSON), Completed: true})

	// Settings: Target count = 5
	catsJSON, _ := json.Marshal([]string{"movement", "learning"})
	rulesJSON, _ := json.Marshal([]dto.QuestRuleResponse{
		{
			ID:             "rule_movement",
			Type:           "movement",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       5,
		},
	})
	db.Create(&models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   5,
		Difficulty:        "normal",
		EnabledCategories: datatypes.JSON(catsJSON),
		Rules:             datatypes.JSON(rulesJSON),
	})

	today := timeutil.TodayVN()

	// Helper assertion to check invariants
	assertInvariants := func(t *testing.T, res *quest_generation.GenerateTodayResult, req quest_generation.GenerateTodayRequest) {
		t.Helper()
		if res.TargetCount <= 0 {
			t.Errorf("invariant fail: TargetCount <= 0 (%d)", res.TargetCount)
		}
		if res.ExistingCount != len(res.Quests) {
			t.Errorf("invariant fail: ExistingCount (%d) != len(Quests) (%d)", res.ExistingCount, len(res.Quests))
		}
		if res.GeneratedCount < 0 {
			t.Errorf("invariant fail: GeneratedCount < 0 (%d)", res.GeneratedCount)
		}
		if res.PreservedCount < 0 {
			t.Errorf("invariant fail: PreservedCount < 0 (%d)", res.PreservedCount)
		}
		if res.ExistingReturned && res.GeneratedCount != 0 {
			t.Errorf("invariant fail: ExistingReturned is true but GeneratedCount = %d", res.GeneratedCount)
		}
		// check "already exist" message condition
		isForceNoPendingSlots := res.GeneratedCount == 0 && res.PreservedCount > 0 && req.Force != nil && *req.Force
		willHaveAlreadyExistMessage := res.ExistingReturned && !isForceNoPendingSlots
		if willHaveAlreadyExistMessage && res.ExistingCount < res.TargetCount {
			t.Errorf("invariant fail: 'already exist' but ExistingCount (%d) < TargetCount (%d)", res.ExistingCount, res.TargetCount)
		}
	}

	// 1. Branch: New generation (empty DB)
	{
		mockAI := &mockGenerator{
			quests: []models.Quest{
				{ID: uuid.New(), Title: "AI 1", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
				{ID: uuid.New(), Title: "AI 2", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
				{ID: uuid.New(), Title: "AI 3", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
				{ID: uuid.New(), Title: "AI 4", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
				{ID: uuid.New(), Title: "AI 5", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
			},
		}
		contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
		service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, &mockGenerator{})

		preferAI := true
		req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI}
		res, err := service.GenerateToday(context.Background(), userID, req)
		if err != nil {
			t.Fatalf("new generation failed: %v", err)
		}
		assertInvariants(t, res, req)
		if res.ExistingReturned {
			t.Error("expected ExistingReturned=false for new generation")
		}
		if res.GeneratedCount != 5 {
			t.Errorf("expected GeneratedCount=5, got %d", res.GeneratedCount)
		}
	}

	// 2. Branch: Existing at target (already has 5 quests, no force)
	{
		contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
		service := quest_generation.NewGenerationService(db, contextBuilder, &mockGenerator{}, &mockGenerator{})

		preferAI := true
		req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI}
		res, err := service.GenerateToday(context.Background(), userID, req)
		if err != nil {
			t.Fatalf("existing at target failed: %v", err)
		}
		assertInvariants(t, res, req)
		if !res.ExistingReturned {
			t.Error("expected ExistingReturned=true")
		}
		if res.ExistingCount != 5 {
			t.Errorf("expected ExistingCount=5, got %d", res.ExistingCount)
		}
	}

	// Delete existing quests for next test
	db.Where("user_id = ?", userID).Delete(&models.Quest{})

	// 3. Branch: Existing below target (has 2, needs to generate 3 more)
	db.Create(&models.Quest{ID: uuid.New(), UserID: userID, Title: "Existing 1", Type: models.QuestTypeMovement, Status: models.QuestStatusPending, Date: today})
	db.Create(&models.Quest{ID: uuid.New(), UserID: userID, Title: "Existing 2", Type: models.QuestTypeMovement, Status: models.QuestStatusPending, Date: today})

	{
		mockAI := &mockGenerator{
			quests: []models.Quest{
				{ID: uuid.New(), Title: "AI 3", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
				{ID: uuid.New(), Title: "AI 4", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
				{ID: uuid.New(), Title: "AI 5", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
			},
		}
		contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
		service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, &mockGenerator{})

		preferAI := true
		req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI}
		res, err := service.GenerateToday(context.Background(), userID, req)
		if err != nil {
			t.Fatalf("existing below target failed: %v", err)
		}
		assertInvariants(t, res, req)
		if res.ExistingReturned {
			t.Error("expected ExistingReturned=false because we generated missing quests")
		}
		if res.GeneratedCount != 3 {
			t.Errorf("expected GeneratedCount=3, got %d", res.GeneratedCount)
		}
		if res.ExistingCount != 5 {
			t.Errorf("expected ExistingCount=5, got %d", res.ExistingCount)
		}
	}

	// 4. Branch: Force regenerate (force=true)
	{
		db.Where("user_id = ?", userID).Delete(&models.Quest{})
		db.Create(&models.Quest{ID: uuid.New(), UserID: userID, Title: "Existing Pending 1", Type: models.QuestTypeMovement, Status: models.QuestStatusPending, Date: today})
		db.Create(&models.Quest{ID: uuid.New(), UserID: userID, Title: "Existing Pending 2", Type: models.QuestTypeMovement, Status: models.QuestStatusPending, Date: today})

		mockAI := &mockGenerator{
			quests: []models.Quest{
				{ID: uuid.New(), Title: "New AI 1", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
				{ID: uuid.New(), Title: "New AI 2", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
			},
		}
		contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
		service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, &mockGenerator{})

		preferAI := true
		force := true
		req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI, Force: &force}
		res, err := service.GenerateToday(context.Background(), userID, req)
		if err != nil {
			t.Fatalf("force regenerate failed: %v", err)
		}
		assertInvariants(t, res, req)
		if res.ExistingReturned {
			t.Error("expected ExistingReturned=false for force generation")
		}
		if res.ReplacedPendingCount != 2 {
			t.Errorf("expected 2 pending quests to be replaced/deleted, got %d", res.ReplacedPendingCount)
		}
	}

	// 5. Branch: Explicit date generation
	{
		futureDateStr := "2026-06-20"
		tDate, _ := timeutil.ParseDateVN(futureDateStr)
		mockAI := &mockGenerator{
			quests: []models.Quest{
				{ID: uuid.New(), Title: "Future AI 1", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: tDate},
				{ID: uuid.New(), Title: "Future AI 2", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: tDate},
				{ID: uuid.New(), Title: "Future AI 3", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: tDate},
				{ID: uuid.New(), Title: "Future AI 4", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: tDate},
				{ID: uuid.New(), Title: "Future AI 5", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: tDate},
			},
		}
		contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
		service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, &mockGenerator{})

		preferAI := true
		req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI, Date: &futureDateStr}
		res, err := service.GenerateToday(context.Background(), userID, req)
		if err != nil {
			t.Fatalf("explicit date generation failed: %v", err)
		}
		assertInvariants(t, res, req)
		if res.Date != futureDateStr {
			t.Errorf("expected Date=%s, got %s", futureDateStr, res.Date)
		}
		if res.GeneratedCount != 5 {
			t.Errorf("expected 5 generated, got %d", res.GeneratedCount)
		}
	}
}
