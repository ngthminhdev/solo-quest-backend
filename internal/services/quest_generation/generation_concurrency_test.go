package quest_generation_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

// TestConcurrentGenerateToday_DoesNotExceedTarget verifies that concurrent
// generate-today requests do not create more quests than target_count.
func TestConcurrentGenerateToday_DoesNotExceedTarget(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Set target to 5
	db.Model(&models.QuestSettings{}).Where("user_id = ?", userID).Update("daily_quest_count", 5)

	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	ruleGenerator := quest_generation.NewRuleBasedGenerator(db)
	service := quest_generation.NewGenerationService(db, contextBuilder, nil, ruleGenerator)

	// Launch 3 concurrent requests
	var wg sync.WaitGroup
	results := make([]*quest_generation.GenerateTodayResult, 3)
	errors := make([]error, 3)

	preferAI := false
	req := quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
	}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errors[idx] = service.GenerateToday(context.Background(), userID, req)
		}(i)
	}

	wg.Wait()

	// At least one should succeed
	successCount := 0
	for i := 0; i < 3; i++ {
		if errors[i] == nil {
			successCount++
		}
	}
	assert.Greater(t, successCount, 0, "at least one request should succeed")

	// Verify total quest count does not exceed target
	var totalQuests int64
	db.Model(&models.Quest{}).Where("user_id = ? AND date = ?", userID, timeutil.TodayVN()).Count(&totalQuests)
	assert.LessOrEqual(t, int(totalQuests), 5, "total quests should not exceed target")
}

// TestConcurrentGenerateWithExplicitDate_DoesNotExceedTarget verifies that
// concurrent generate?date=YYYY-MM-DD requests do not exceed target_count.
func TestConcurrentGenerateWithExplicitDate_DoesNotExceedTarget(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Set target to 6
	db.Model(&models.QuestSettings{}).Where("user_id = ?", userID).Update("daily_quest_count", 6)

	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	ruleGenerator := quest_generation.NewRuleBasedGenerator(db)
	service := quest_generation.NewGenerationService(db, contextBuilder, nil, ruleGenerator)

	explicitDate := "2026-06-15"
	datePtr := &explicitDate

	var wg sync.WaitGroup
	results := make([]*quest_generation.GenerateTodayResult, 3)
	errors := make([]error, 3)

	preferAI := false
	req := quest_generation.GenerateTodayRequest{
		Date:     datePtr,
		PreferAI: &preferAI,
	}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errors[idx] = service.GenerateToday(context.Background(), userID, req)
		}(i)
	}

	wg.Wait()

	successCount := 0
	for i := 0; i < 3; i++ {
		if errors[i] == nil {
			successCount++
		}
	}
	assert.Greater(t, successCount, 0, "at least one request should succeed")

	// Verify total quest count for the explicit date
	parsedDate, _ := timeutil.ParseDateVN(explicitDate)
	start, end := timeutil.DayRangeVN(parsedDate)
	var totalQuests int64
	db.Model(&models.Quest{}).Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).Count(&totalQuests)
	assert.LessOrEqual(t, int(totalQuests), 6, "total quests should not exceed target")
}

// TestTargetCountPresent_ExistingAtTarget verifies target_count is present when
// returning existing quests at target.
func TestTargetCountPresent_ExistingAtTarget(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// Update target to 4
	db.Model(&models.QuestSettings{}).Where("user_id = ?", userID).Update("daily_quest_count", 4)

	// Create 4 quests to meet target
	for i := 0; i < 4; i++ {
		db.Create(&models.Quest{
			ID:     uuid.New(),
			UserID: userID,
			Date:   today,
			Status: models.QuestStatusPending,
			Type:   models.QuestTypeDaily,
			Title:  "Quest",
		})
	}

	preferAI := false
	force := false
	req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI, Force: &force}

	result, err := service.GenerateToday(context.Background(), userID, req)
	assert.NoError(t, err)
	assert.Equal(t, 4, result.TargetCount, "target_count must be present")
	assert.Equal(t, 4, result.ExistingCount, "existing_count must be present")
	assert.True(t, result.ExistingReturned)
}

// TestTargetCountPresent_GeneratedMissing verifies target_count is present when
// generating missing quests.
func TestTargetCountPresent_GeneratedMissing(t *testing.T) {
	db, userID, _, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// Create 2 quests, below target of 5
	for i := 0; i < 2; i++ {
		db.Create(&models.Quest{
			ID:     uuid.New(),
			UserID: userID,
			Date:   today,
			Status: models.QuestStatusPending,
			Type:   models.QuestTypeDaily,
			Title:  "Quest",
		})
	}

	// Mock AI to return 3 quests
	mockAI.quests = []models.Quest{
		{Title: "AI Quest 1", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "AI Quest 2", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "AI Quest 3", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
	}

	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, nil)

	preferAI := true
	req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI}

	result, err := service.GenerateToday(context.Background(), userID, req)
	assert.NoError(t, err)
	assert.Equal(t, 5, result.TargetCount, "target_count must be present")
	assert.Equal(t, 5, result.ExistingCount, "existing_count must equal target after generation")
	assert.Equal(t, 3, result.GeneratedCount)
	assert.Equal(t, 2, result.PreservedCount)
	assert.True(t, result.Inserted)
}

// TestTargetCountPresent_ForceRegenerate verifies target_count is present when
// force regenerating.
func TestTargetCountPresent_ForceRegenerate(t *testing.T) {
	db, userID, _, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// Create 3 pending, 1 completed
	for i := 0; i < 3; i++ {
		db.Create(&models.Quest{
			ID:     uuid.New(),
			UserID: userID,
			Date:   today,
			Status: models.QuestStatusPending,
			Type:   models.QuestTypeDaily,
			Title:  "Pending",
		})
	}
	db.Create(&models.Quest{
		ID:     uuid.New(),
		UserID: userID,
		Date:   today,
		Status: models.QuestStatusCompleted,
		Type:   models.QuestTypeDaily,
		Title:  "Completed",
	})

	// Mock AI to return 4 new quests
	mockAI.quests = []models.Quest{
		{Title: "AI Quest 1", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "AI Quest 2", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "AI Quest 3", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "AI Quest 4", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
	}

	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, nil)

	preferAI := true
	force := true
	req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI, Force: &force}

	result, err := service.GenerateToday(context.Background(), userID, req)
	assert.NoError(t, err)
	assert.Equal(t, 5, result.TargetCount, "target_count must be present")
	assert.Equal(t, 5, result.ExistingCount, "existing_count must equal target after regeneration")
	assert.Equal(t, 4, result.GeneratedCount, "should generate 4 new")
	assert.Equal(t, 1, result.PreservedCount, "should preserve 1 completed")
	assert.Equal(t, 3, result.ReplacedPendingCount, "should replace 3 pending")
	assert.True(t, result.Inserted)
}

// TestTargetCountPresent_FallbackGeneration verifies target_count is present
// when AI fails and falls back to rule-based.
func TestTargetCountPresent_FallbackGeneration(t *testing.T) {
	db, userID, _, _, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	// Setup mock rule generator to return 5 quests for fallback
	mockRule.quests = []models.Quest{
		{Title: "Rule Quest 1", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "Rule Quest 2", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "Rule Quest 3", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "Rule Quest 4", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
		{Title: "Rule Quest 5", Type: models.QuestTypeDaily, Status: models.QuestStatusPending, Date: today},
	}

	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	
	// Mock AI that always fails
	mockAI := &failingMockGenerator{err: fmt.Errorf("AI timeout")}
	service := quest_generation.NewGenerationService(db, contextBuilder, mockAI, mockRule)

	preferAI := true
	req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI}

	result, err := service.GenerateToday(context.Background(), userID, req)
	assert.NoError(t, err, "fallback should succeed")
	assert.Equal(t, 5, result.TargetCount, "target_count must be present")
	assert.Equal(t, 5, result.ExistingCount, "existing_count must equal target")
	assert.True(t, result.FallbackUsed, "fallback should be used")
	assert.Equal(t, "rule_based", result.Source)
}

type failingMockGenerator struct {
	err error
}

func (m *failingMockGenerator) GenerateDailyQuests(ctx context.Context, qctx *quest_generation.UserQuestContext) ([]models.Quest, error) {
	return nil, m.err
}
