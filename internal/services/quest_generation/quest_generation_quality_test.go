package quest_generation_test

import (
	"context"
	"encoding/json"
	"strings"
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

// ─── helpers ─────────────────────────────────────────────────────────────────

func makeQuestRules(movement, learning, sleep, review *int) []quest_generation.QuestRuleContext {
	allDays := []int{1, 2, 3, 4, 5, 6, 7}
	rules := []quest_generation.QuestRuleContext{}
	if movement != nil {
		rules = append(rules, quest_generation.QuestRuleContext{
			ID: "rule_movement", Type: "movement", Enabled: true, Difficulty: "medium",
			MaxPerDay: movement, Weekdays: allDays,
			ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "10:00", End: "17:00"},
		})
	}
	if learning != nil {
		rules = append(rules, quest_generation.QuestRuleContext{
			ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium",
			MaxPerDay: learning, Weekdays: allDays,
			ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "19:00", End: "22:00"},
		})
	}
	if sleep != nil {
		rules = append(rules, quest_generation.QuestRuleContext{
			ID: "rule_sleep", Type: "sleep", Enabled: true, Difficulty: "easy",
			MaxPerDay: sleep, Weekdays: allDays,
			ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "22:00", End: "23:30"},
		})
	}
	if review != nil {
		rules = append(rules, quest_generation.QuestRuleContext{
			ID: "rule_review", Type: "review", Enabled: true, Difficulty: "easy",
			MaxPerDay: review, Weekdays: allDays,
			ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "21:00", End: "22:00"},
		})
	}
	return rules
}

func ptr(v int) *int { return &v }

// ─── 1. Enabled categories: learning disabled → no learning quest ─────────────

func TestRuleBasedGenerator_EnabledCategories_LearningDisabled(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:   6,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"movement", "sleep", "review"}, // NO learning
		Rules:             makeQuestRules(ptr(2), ptr(2), ptr(1), ptr(1)),
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, q := range quests {
		if q.Type == models.QuestTypeLearning {
			t.Errorf("expected no learning quest when learning is disabled, got: %s", q.Title)
		}
	}
}

// ─── 2. Enabled categories: movement disabled → no movement quest ─────────────

func TestRuleBasedGenerator_EnabledCategories_MovementDisabled(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:   6,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"learning", "sleep", "review"}, // NO movement
		Rules:             makeQuestRules(ptr(2), ptr(2), ptr(1), ptr(1)),
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, q := range quests {
		if q.Type == models.QuestTypeMovement {
			t.Errorf("expected no movement quest when movement is disabled, got: %s", q.Title)
		}
	}
}

// ─── 3. Respect daily_quest_count ────────────────────────────────────────────

func TestRuleBasedGenerator_DailyQuestCount(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	const dailyLimit = 6
	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:   dailyLimit,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"movement", "learning", "sleep", "review"},
		Rules:             makeQuestRules(ptr(3), ptr(2), ptr(1), ptr(1)),
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(quests) > dailyLimit {
		t.Errorf("expected at most %d quests, got %d", dailyLimit, len(quests))
	}
}

// ─── 4. Respect max_per_day per rule ─────────────────────────────────────────

func TestRuleBasedGenerator_RespectMaxPerDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:   8,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"learning", "sleep", "review"},
		// learning max_per_day=1
		Rules: makeQuestRules(nil, ptr(1), ptr(1), ptr(1)),
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	learningCount := 0
	for _, q := range quests {
		if q.Type == models.QuestTypeLearning {
			learningCount++
		}
	}
	if learningCount > 1 {
		t.Errorf("expected at most 1 learning quest (max_per_day=1), got %d", learningCount)
	}
}

// ─── 5. force=false twice → no duplicate ─────────────────────────────────────

func TestGenerationService_ForceFalse_NoDuplicate(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	mockAI.quests = []models.Quest{
		{Title: "Quest A", Source: models.QuestSourceAI, Date: today, Type: models.QuestTypeLearning},
	}

	preferAI := true
	force := false
	req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI, Force: &force}

	result1, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	firstCount := len(result1.Quests)

	result2, err := service.GenerateToday(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if !result2.ExistingReturned {
		t.Error("expected ExistingReturned=true on second force=false call")
	}

	var dbCount int64
	db.Model(&models.Quest{}).Where("user_id = ?", userID).Count(&dbCount)
	if int(dbCount) != firstCount {
		t.Errorf("expected %d quests after 2 force=false calls, got %d (possible duplicate)", firstCount, dbCount)
	}
}

// ─── 6 & 11. Replace pending only: completed/skipped/snoozed preserved ────────

func TestGenerationService_SkippedSnoozedPreservedAfterForce(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()

	completed := models.Quest{ID: uuid.New(), UserID: userID, Title: "Completed Q",
		Date: today, Status: models.QuestStatusCompleted, Type: models.QuestTypeMovement}
	skipped := models.Quest{ID: uuid.New(), UserID: userID, Title: "Skipped Q",
		Date: today, Status: models.QuestStatusSkipped, Type: models.QuestTypeSleep}
	snoozed := models.Quest{ID: uuid.New(), UserID: userID, Title: "Snoozed Q",
		Date: today, Status: models.QuestStatusSnoozed, Type: models.QuestTypeReview}
	pending := models.Quest{ID: uuid.New(), UserID: userID, Title: "Pending To Replace",
		Date: today, Status: models.QuestStatusPending, Type: models.QuestTypeLearning}
	for _, q := range []models.Quest{completed, skipped, snoozed, pending} {
		db.Create(&q)
	}

	mockAI.quests = []models.Quest{
		{Title: "New Quest", Source: models.QuestSourceAI, Date: today, Type: models.QuestTypeLearning},
	}

	preferAI := true
	force := true
	result, err := service.GenerateToday(context.Background(), userID,
		quest_generation.GenerateTodayRequest{PreferAI: &preferAI, Force: &force})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PreservedCount != 3 {
		t.Errorf("expected PreservedCount=3, got %d", result.PreservedCount)
	}
	if result.ReplacedPendingCount != 1 {
		t.Errorf("expected ReplacedPendingCount=1, got %d", result.ReplacedPendingCount)
	}

	checkStatus := func(id uuid.UUID, expected models.QuestStatus) {
		t.Helper()
		var q models.Quest
		if err := db.First(&q, id).Error; err != nil {
			t.Errorf("quest %s was deleted: %v", id, err)
			return
		}
		if q.Status != expected {
			t.Errorf("quest %s: expected status %s, got %s", id, expected, q.Status)
		}
	}
	checkStatus(completed.ID, models.QuestStatusCompleted)
	checkStatus(skipped.ID, models.QuestStatusSkipped)
	checkStatus(snoozed.ID, models.QuestStatusSnoozed)

	var pendingCheck models.Quest
	if err := db.First(&pendingCheck, pending.ID).Error; err == nil {
		t.Error("pending quest should have been deleted after force=true")
	}
}

// ─── 7. Learning path context: title includes current step ───────────────────

func TestRuleBasedGenerator_LearningPathContext(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:   3,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"learning", "review"},
		ActiveLearningPath: &quest_generation.ActiveLearningPathDetail{
			RoadmapTitle:     "Flutter Roadmap",
			CurrentStepTitle: "Riverpod State Management",
			Description:      "Học quản lý state với Riverpod.",
		},
		Rules: makeQuestRules(nil, ptr(2), nil, ptr(1)),
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var lq *models.Quest
	for i := range quests {
		if quests[i].Type == models.QuestTypeLearning {
			lq = &quests[i]
			break
		}
	}
	if lq == nil {
		t.Fatal("expected a learning quest to be generated")
	}
	if !strings.Contains(lq.Title, "Riverpod State Management") {
		t.Errorf("expected title to contain current step 'Riverpod State Management', got: %s", lq.Title)
	}
}

// ─── 8. Learning fallback has specific action (not generic) ─────────────────

func TestRuleBasedGenerator_LearningFallbackHasSpecificAction(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	qctx := &quest_generation.UserQuestContext{
		UserID:             uuid.New(),
		LocalDate:          timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:    2,
		Difficulty:         "normal",
		PreferredDuration:  "medium",
		EnabledCategories:  []string{"learning"},
		ActiveLearningPath: nil, // no active path → fallback
		Rules:              makeQuestRules(nil, ptr(1), nil, nil),
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(quests) == 0 {
		t.Fatal("expected fallback learning quest to be generated")
	}

	lq := quests[0]
	if lq.Type != models.QuestTypeLearning {
		t.Fatalf("expected learning quest type, got %s", lq.Type)
	}

	// Old generic description should be gone
	if lq.Description == "Dành một khoảng thời gian ngắn để học hoặc ôn lại nội dung quan trọng." {
		t.Error("fallback learning quest still uses old generic description — fix not applied")
	}
	// New description should contain a concrete action
	if !strings.Contains(lq.Description, "Ghi lại") {
		t.Errorf("fallback description should reference 'Ghi lại' action, got: %s", lq.Description)
	}
	// Instruction should be non-empty and specific
	if lq.Instruction == "" {
		t.Error("fallback learning quest should have a specific instruction")
	}
}

// ─── 9. Response sorted by reminder_time ascending ───────────────────────────

func TestGenerationService_SortByReminderTime(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	t9 := time.Date(today.Year(), today.Month(), today.Day(), 9, 0, 0, 0, timeutil.LocationVN)
	t14 := time.Date(today.Year(), today.Month(), today.Day(), 14, 0, 0, 0, timeutil.LocationVN)
	t21 := time.Date(today.Year(), today.Month(), today.Day(), 21, 30, 0, 0, timeutil.LocationVN)

	// Supply in reverse order
	mockAI.quests = []models.Quest{
		{Title: "Evening Quest", Source: models.QuestSourceAI, Date: today, Type: models.QuestTypeReview, ReminderTime: &t21},
		{Title: "Morning Quest", Source: models.QuestSourceAI, Date: today, Type: models.QuestTypeMovement, ReminderTime: &t9},
		{Title: "Afternoon Quest", Source: models.QuestSourceAI, Date: today, Type: models.QuestTypeLearning, ReminderTime: &t14},
	}

	preferAI := true
	result, err := service.GenerateToday(context.Background(), userID,
		quest_generation.GenerateTodayRequest{PreferAI: &preferAI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	quests := result.Quests
	for i := 1; i < len(quests); i++ {
		prev, curr := quests[i-1], quests[i]
		if prev.ReminderTime == nil || curr.ReminderTime == nil {
			continue
		}
		if prev.ReminderTime.After(*curr.ReminderTime) {
			t.Errorf("quests not sorted by reminder_time: [%d] %s (%s) is after [%d] %s (%s)",
				i-1, prev.Title, prev.ReminderTime.Format("15:04"),
				i, curr.Title, curr.ReminderTime.Format("15:04"))
		}
	}
}

// ─── 10. Low energy caps difficulty (no hard quests) and shortens duration ────

func TestRuleBasedGenerator_LowEnergyCapsDifficultyAndDuration(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	tomorrow := timeutil.TodayVN().AddDate(0, 0, 1)

	qctx := &quest_generation.UserQuestContext{
		UserID:            userID,
		LocalDate:         tomorrow,
		DailyQuestCount:   6,
		Difficulty:        "normal",
		PreferredDuration: "long", // would normally produce long quests
		EnabledCategories: []string{"movement", "learning", "sleep", "review"},
		Rules:             makeQuestRules(ptr(2), ptr(1), ptr(1), ptr(1)),
		TodayCheckIn: &quest_generation.TodayCheckInDetail{
			EnergyLevel: "low", Availability: "normal",
		},
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(quests) == 0 {
		t.Fatal("expected at least some quests")
	}

	// Low energy: no hard difficulty allowed
	for _, q := range quests {
		if q.Difficulty == models.QuestDifficultyHard {
			t.Errorf("expected no hard difficulty when energy is low, got %s: %s", q.Difficulty, q.Title)
		}
	}

	// Low energy: duration should be capped to "short" equivalent
	// short movement base = 10 * 0.7 = 7, short learning base = 20 * 0.7 = 14
	// "long" movement would be 10 * 1.5 = 15; with low energy capped to "short" = 7
	for _, q := range quests {
		if q.Type == models.QuestTypeMovement && q.EstimatedMinutes > 10 {
			t.Errorf("expected low energy to shorten movement duration (<=10min), got %dmin", q.EstimatedMinutes)
		}
		if q.Type == models.QuestTypeLearning && q.EstimatedMinutes > 20 {
			t.Errorf("expected low energy to shorten learning duration (<=20min), got %dmin", q.EstimatedMinutes)
		}
	}
}

// ─── 11. Low availability reduces quest count ────────────────────────────────

func TestRuleBasedGenerator_BusyAvailabilityReducesCount(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	const dailyCount = 8
	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:   dailyCount,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"movement", "learning", "sleep", "review"},
		Rules:             makeQuestRules(ptr(3), ptr(2), ptr(1), ptr(1)),
		TodayCheckIn: &quest_generation.TodayCheckInDetail{
			EnergyLevel: "normal", Availability: "busy",
		},
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	threshold := dailyCount*7/10 + 1 // ~70% of daily count, +1 for rounding tolerance
	if len(quests) > threshold {
		t.Errorf("expected at most ~%d quests when availability=busy, got %d",
			threshold-1, len(quests))
	}
}

// ─── 12. ExistingQuestTypeCount prevents over-generating preserved type ───────

func TestRuleBasedGenerator_ExistingTypeCountPreventsExtraReview(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:   4,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"learning", "review"},
		// 1 review quest already preserved → max_per_day=1 should mean 0 new review quests
		ExistingQuestTypeCount: map[string]int{"review": 1},
		ExistingQuestTitles:    []string{"Daily review"},
		Rules:                  makeQuestRules(nil, ptr(1), nil, ptr(1)),
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, q := range quests {
		if q.Type == models.QuestTypeReview {
			t.Errorf("expected 0 new review quests (1 preserved + max_per_day=1), got one: %s", q.Title)
		}
	}
}

// ─── 13. ExistingQuestTitles dedup in rule-based ─────────────────────────────

func TestRuleBasedGenerator_ExistingTitlesDedup(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	qctx := &quest_generation.UserQuestContext{
		UserID:              uuid.New(),
		LocalDate:           timeutil.TodayVN().AddDate(0, 0, 1),
		DailyQuestCount:     4,
		Difficulty:          "normal",
		PreferredDuration:   "medium",
		EnabledCategories:   []string{"learning", "review"},
		ExistingQuestTitles: []string{"Daily review"}, // existing preserved title
		Rules:               makeQuestRules(nil, ptr(2), nil, ptr(1)),
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, q := range quests {
		if strings.EqualFold(q.Title, "Daily review") {
			t.Errorf("generated quest with title 'Daily review' that already exists in ExistingQuestTitles")
		}
	}
}

// ─── 14. Full service: learning path loaded from DB ──────────────────────────

func TestGenerationService_LearningPathContextFromDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	catsJSON, _ := json.Marshal([]string{"learning"})
	rules := []dto.QuestRuleResponse{
		{
			ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium",
			MaxPerDay: ptr(1), ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7},
			ActiveTimeRange: &dto.TimeRangeResponse{Start: "19:00", End: "22:00"},
		},
	}
	rulesJSON, _ := json.Marshal(rules)
	db.Create(&models.QuestSettings{
		UserID: userID, DailyQuestCount: 3, Difficulty: "normal",
		EnabledCategories: datatypes.JSON(catsJSON), PreferredDuration: "medium",
		Rules: datatypes.JSON(rulesJSON),
	})

	roadmap := models.LearningRoadmap{ID: uuid.New(), Title: "Go Roadmap", Description: "Learn Go"}
	db.Create(&roadmap)

	step := models.LearningRoadmapStep{
		ID: uuid.New(), RoadmapID: roadmap.ID, Title: "Goroutines và Channels",
		Description: "Học concurrency.", OrderIndex: 1,
	}
	db.Create(&step)

	userRoadmap := models.UserLearningRoadmap{
		ID: uuid.New(), UserID: userID, RoadmapID: roadmap.ID, Status: "tracking",
	}
	db.Create(&userRoadmap)

	builder := quest_generation.NewUserQuestContextBuilder(db)
	qctx, err := builder.Build(context.Background(), userID, timeutil.TodayVN().AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}
	if qctx.ActiveLearningPath == nil {
		t.Fatal("expected ActiveLearningPath to be loaded from DB")
	}
	if qctx.ActiveLearningPath.CurrentStepTitle != "Goroutines và Channels" {
		t.Errorf("expected CurrentStepTitle 'Goroutines và Channels', got '%s'",
			qctx.ActiveLearningPath.CurrentStepTitle)
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("generation error: %v", err)
	}
	var found bool
	for _, q := range quests {
		if q.Type == models.QuestTypeLearning && strings.Contains(q.Title, "Goroutines") {
			found = true
			break
		}
	}
	if !found {
		titles := make([]string, len(quests))
		for i, q := range quests {
			titles[i] = q.Title
		}
		t.Errorf("expected learning quest with 'Goroutines' in title, got: %v", titles)
	}
}
