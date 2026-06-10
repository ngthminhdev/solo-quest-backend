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
// Verifies the service respects whatever the generator returns without padding.
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

	// Settings with DailyQuestCount=5; movement + learning enabled
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
		{
			ID:             "rule_learning",
			Type:           "learning",
			Enabled:        true,
			Difficulty:     "easy",
			ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			Priority:       5,
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
	// rule-based generator's rest-day logic) — the service must NOT pad beyond that.
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

	if result.GeneratedCount != 2 {
		t.Errorf("rest day: expected 2 quests (rule-based cap), got %d", result.GeneratedCount)
	}

	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 2 {
		t.Errorf("rest day: expected 2 quests in DB, got %d", len(dbQuests))
	}
}

// errForceFallback is a sentinel error to force the fallback path via mockGenerator.
var errForceFallback = fmt.Errorf("forced fallback for test")

// ---------------------------------------------------------------------------
// TestGenerationService_DedupRemovesDuplicateLearningFromTopUp
// When AI includes a generic learning quest and the rule-based top-up also
// produces a generic learning quest, the duplicate must be removed from the DB.
// ---------------------------------------------------------------------------

func TestGenerationService_DedupRemovesDuplicateLearningFromTopUp(t *testing.T) {
	db, userID, service, mockAI, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// AI returns 3 quests: movement + generic-learning + sleep → shortfall=2 for DailyQuestCount=5
	mockAI.quests = []models.Quest{
		{ID: uuid.New(), Title: "Vận động buổi sáng", Type: models.QuestTypeMovement, Source: models.QuestSourceAI, Date: today},
		// "học tập" triggers isGenericLearning in questplan → canonical key = "learning:generic"
		{ID: uuid.New(), Title: "Học tập chủ đề bạn thích hôm nay", Type: models.QuestTypeLearning, Source: models.QuestSourceAI, Date: today},
		{ID: uuid.New(), Title: "Đi ngủ đúng giờ", Type: models.QuestTypeSleep, Source: models.QuestSourceAI, Date: today},
	}

	// Rule top-up provides 2 quests, one of which is also a generic learning duplicate.
	// The duplicate must be removed; only the non-duplicate tops up.
	mockRule.quests = []models.Quest{
		// "học tập" → also "learning:generic" → duplicate of AI quest above
		{Title: "Học tập và ghi lại 3 ý chính", Type: models.QuestTypeLearning, Source: models.QuestSourceConfigBased, Date: today},
		{Title: "Uống đủ nước", Type: models.QuestTypeWater, Source: models.QuestSourceConfigBased, Date: today},
	}

	preferAI := true
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{PreferAI: &preferAI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 AI + 1 kept top-up (water); 1 rule-based learning deduped out
	if result.GeneratedCount != 4 {
		t.Errorf("expected 4 quests after dedup (3 AI + 1 rule water), got %d", result.GeneratedCount)
	}

	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 4 {
		t.Fatalf("expected 4 quests in DB after dedup, got %d", len(dbQuests))
	}

	learningCount := 0
	for _, q := range dbQuests {
		if q.Type == models.QuestTypeLearning {
			learningCount++
		}
	}
	if learningCount != 1 {
		t.Errorf("expected exactly 1 learning quest after dedup, got %d", learningCount)
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
