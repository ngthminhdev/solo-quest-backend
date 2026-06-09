package quest_generation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

// TestGenerationService_TopUpAfterPartialAI verifies that when the AI returns
// fewer valid quests than the remaining capacity, the rule-based generator only
// tops up the missing amount instead of replacing the whole batch.
func TestGenerationService_TopUpAfterPartialAI(t *testing.T) {
	db, userID, service, mockAI, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// Settings DailyQuestCount=5, no preserved quests -> remaining capacity 5.
	// AI returns only 2 valid quests -> shortfall of 3.
	mockAI.quests = []models.Quest{
		{ID: uuid.New(), Title: "AI Quest 1", Source: models.QuestSourceAI, Date: today},
		{ID: uuid.New(), Title: "AI Quest 2", Source: models.QuestSourceAI, Date: today},
	}

	// Rule generator can supply more than the shortfall; the service must only
	// take what is missing (3).
	mockRule.quests = []models.Quest{
		{Title: "Rule Quest 1", Source: models.QuestSourceConfigBased, Date: today},
		{Title: "Rule Quest 2", Source: models.QuestSourceConfigBased, Date: today},
		{Title: "Rule Quest 3", Source: models.QuestSourceConfigBased, Date: today},
		{Title: "Rule Quest 4", Source: models.QuestSourceConfigBased, Date: today},
		{Title: "Rule Quest 5", Source: models.QuestSourceConfigBased, Date: today},
	}

	preferAI := true
	req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI}

	result, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Source != "ai" {
		t.Errorf("expected Source 'ai' (partial AI kept), got %s", result.Source)
	}
	if !result.FallbackUsed {
		t.Error("expected FallbackUsed true because rule-based top-up was applied")
	}
	if result.GeneratedCount != 5 {
		t.Errorf("expected GeneratedCount 5 (2 AI + 3 top-up), got %d", result.GeneratedCount)
	}

	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	if len(dbQuests) != 5 {
		t.Fatalf("expected 5 quests in DB, got %d", len(dbQuests))
	}

	aiCount, ruleCount := 0, 0
	for _, q := range dbQuests {
		switch q.Source {
		case models.QuestSourceAI:
			aiCount++
		case models.QuestSourceConfigBased:
			ruleCount++
		}
	}
	if aiCount != 2 {
		t.Errorf("expected 2 AI quests preserved, got %d", aiCount)
	}
	if ruleCount != 3 {
		t.Errorf("expected exactly 3 rule-based top-up quests, got %d", ruleCount)
	}
}
