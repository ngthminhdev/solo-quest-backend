package quest_generation_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/questplan"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

func setupLearningRoadmapData(t *testing.T, db *gorm.DB) (userID, roadmapID, step1ID, step2ID uuid.UUID) {
	t.Helper()

	userID = uuid.New()
	quietTime := "22:00"
	user := models.UserProfile{
		ID:                     userID,
		DisplayName:            "Learning Test User",
		Level:                  1,
		CurrentLevelExp:        0,
		NextLevelExp:           100,
		TotalExp:               0,
		RewardPoints:           100,
		StreakDays:             0,
		BestStreak:             0,
		StreakShields:          2,
		TotalCompletedQuests:   0,
		TotalSkippedQuests:     0,
		HasCompletedOnboarding: false,
		QuietAfterTime:         &quietTime,
	}
	db.Create(&user)

	roadmapID = uuid.New()
	rm := models.LearningRoadmap{
		ID:               roadmapID,
		Title:            "Test Flutter Roadmap",
		Description:      "A test roadmap",
		Category:         "Flutter",
		Difficulty:       "intermediate",
		EstimatedMinutes: 120,
		TotalSteps:       2,
		Source:           "system",
		Enabled:          true,
	}
	db.Create(&rm)

	step1ID = uuid.New()
	s1 := models.LearningRoadmapStep{
		ID:               step1ID,
		RoadmapID:        roadmapID,
		Title:            "Riverpod Basics",
		Description:      "Learn Riverpod fundamentals",
		OrderIndex:       0,
		EstimatedMinutes: 30,
		Enabled:          true,
	}
	db.Create(&s1)

	step2ID = uuid.New()
	s2 := models.LearningRoadmapStep{
		ID:               step2ID,
		RoadmapID:        roadmapID,
		Title:            "State Management Patterns",
		Description:      "Advanced state management",
		OrderIndex:       1,
		EstimatedMinutes: 30,
		Enabled:          true,
	}
	db.Create(&s2)

	now := timeutil.NowUTC()
	ulr := models.UserLearningRoadmap{
		UserID:    userID,
		RoadmapID: roadmapID,
		Status:    models.UserLearningRoadmapStatusTracking,
		StartedAt: now,
	}
	db.Create(&ulr)

	return
}

func TestBuildLearningMetadata_WithActivePath(t *testing.T) {
	path := &quest_generation.ActiveLearningPathDetail{
		RoadmapID:            uuid.New().String(),
		StepID:               uuid.New().String(),
		RoadmapTitle:         "Flutter",
		CurrentStepTitle:     "Riverpod",
		StepOrderIndex:       2,
		TotalSteps:           7,
	}
	meta := quest_generation.BuildLearningMetadata(path)
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(meta, &m); err != nil {
		t.Fatal("failed to unmarshal metadata:", err)
	}
	if m["learning_roadmap_id"] == "" {
		t.Error("expected learning_roadmap_id to be set")
	}
	if m["learning_step_id"] == "" {
		t.Error("expected learning_step_id to be set")
	}
	if m["learning_step_order_index"].(float64) != 2 {
		t.Errorf("expected step_order_index=2, got %v", m["learning_step_order_index"])
	}
}

func TestBuildLearningMetadata_NilPath(t *testing.T) {
	meta := quest_generation.BuildLearningMetadata(nil)
	if meta != nil {
		t.Error("expected nil metadata for nil path")
	}
}

func TestBuildLearningMetadata_EmptyIDs(t *testing.T) {
	path := &quest_generation.ActiveLearningPathDetail{
		RoadmapTitle:     "Flutter",
		CurrentStepTitle: "Riverpod",
	}
	meta := quest_generation.BuildLearningMetadata(path)
	if meta != nil {
		t.Error("expected nil metadata when RoadmapID/StepID are empty")
	}
}

func TestParseLearningMetadata_Valid(t *testing.T) {
	meta, _ := json.Marshal(map[string]string{
		"learning_roadmap_id": uuid.New().String(),
		"learning_step_id":    uuid.New().String(),
	})
	q := models.Quest{LearningMetadata: datatypes.JSON(meta)}
	result, ok := quest_generation.ParseLearningMetadata(q)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if result.LearningStepID == "" {
		t.Error("expected non-empty step_id")
	}
}

func TestParseLearningMetadata_EmptyMetadata(t *testing.T) {
	q := models.Quest{}
	_, ok := quest_generation.ParseLearningMetadata(q)
	if ok {
		t.Error("expected ok=false for empty metadata")
	}
}

func TestParseLearningMetadata_NoStepID(t *testing.T) {
	meta, _ := json.Marshal(map[string]string{
		"learning_roadmap_id": uuid.New().String(),
	})
	q := models.Quest{LearningMetadata: datatypes.JSON(meta)}
	_, ok := quest_generation.ParseLearningMetadata(q)
	if ok {
		t.Error("expected ok=false when learning_step_id is missing")
	}
}

func TestBridgeToQuestplanCandidate_RoadmapStepID(t *testing.T) {
	stepID := uuid.New().String()
	meta, _ := json.Marshal(map[string]string{
		"learning_roadmap_id": uuid.New().String(),
		"learning_step_id":    stepID,
	})
	q := models.Quest{
		Type:             models.QuestTypeLearning,
		Title:            "Học tập: Riverpod Basics",
		Source:           models.QuestSourceAI,
		LearningMetadata: datatypes.JSON(meta),
	}

	// Note: bridgeToQuestplanCandidate is unexported but we can test
	// the questplan.CanonicalKey path via questplan package
	c := questplan.Candidate{
		Title:         q.Title,
		Type:          questplan.QuestType(string(q.Type)),
		RoadmapStepID: stepID,
	}
	key := questplan.CanonicalKey(c)
	if key != "learning:step:"+stepID {
		t.Errorf("expected key 'learning:step:%s', got '%s'", stepID, key)
	}
}

func TestCanonicalKey_DeduplicatesSameStep(t *testing.T) {
	stepID := uuid.New().String()

	aiQuest := questplan.Candidate{
		Title:         "AI Generated Learning Quest",
		Description:   "Learn about Riverpod",
		Type:          questplan.TypeLearning,
		Source:        questplan.SourceAI,
		RoadmapStepID: stepID,
	}
	ruleBasedQuest := questplan.Candidate{
		Title:         "Học tập: Riverpod Basics",
		Description:   "Learn Riverpod fundamentals",
		Type:          questplan.TypeLearning,
		Source:        questplan.SourceRuleBased,
		RoadmapStepID: stepID,
	}

	aiKey := questplan.CanonicalKey(aiQuest)
	rbKey := questplan.CanonicalKey(ruleBasedQuest)

	if aiKey != rbKey {
		t.Errorf("AI and rule-based quests for same step should have same canonical key: '%s' vs '%s'", aiKey, rbKey)
	}
	if aiKey != "learning:step:"+stepID {
		t.Errorf("expected key 'learning:step:%s', got '%s'", stepID, aiKey)
	}
}

func TestCanonicalKey_DifferentSteps_DifferentKeys(t *testing.T) {
	step1 := uuid.New().String()
	step2 := uuid.New().String()

	q1 := questplan.Candidate{
		Title:         "Learn A",
		Type:          questplan.TypeLearning,
		RoadmapStepID: step1,
	}
	q2 := questplan.Candidate{
		Title:         "Learn B",
		Type:          questplan.TypeLearning,
		RoadmapStepID: step2,
	}

	k1 := questplan.CanonicalKey(q1)
	k2 := questplan.CanonicalKey(q2)

	if k1 == k2 {
		t.Error("different steps should have different canonical keys")
	}
}

func TestCanonicalKey_NoStepID_FallsBackToTitle(t *testing.T) {
	q := questplan.Candidate{
		Title: "Learn something specific",
		Type:  questplan.TypeLearning,
	}

	key := questplan.CanonicalKey(q)
	if !strings.HasPrefix(key, "learning:") {
		t.Errorf("expected key to start with 'learning:', got '%s'", key)
	}
	if strings.Contains(key, "step:") {
		t.Error("key should not contain 'step:' when no RoadmapStepID")
	}
}

func TestCanonicalKey_GenericLearning(t *testing.T) {
	q := questplan.Candidate{
		Title:       "Chọn một chủ đề và ghi lại 3 ý chính",
		Description: "Chọn một chủ đề bạn muốn học hôm nay",
		Type:        questplan.TypeLearning,
	}

	key := questplan.CanonicalKey(q)
	if key != "learning:generic" {
		t.Errorf("expected 'learning:generic', got '%s'", key)
	}
}

func TestRuleBasedGenerator_LearningQuestHasMetadata(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID, roadmapID, step1ID, _ := setupLearningRoadmapData(t, db)

	gen := quest_generation.NewRuleBasedGenerator(db)
	ctx := contextForLearningPath(userID, roadmapID, step1ID)

	quests, err := gen.GenerateDailyQuests(testCtx(), ctx)
	if err != nil {
		t.Fatal("failed to generate quests:", err)
	}

	var learningFound bool
	for _, q := range quests {
		if q.Type == models.QuestTypeLearning {
			learningFound = true
			if len(q.LearningMetadata) == 0 {
				t.Error("learning quest should have LearningMetadata")
			} else {
				var meta struct {
					LearningRoadmapID string `json:"learning_roadmap_id"`
					LearningStepID    string `json:"learning_step_id"`
				}
				if err := json.Unmarshal(q.LearningMetadata, &meta); err != nil {
					t.Fatal("failed to parse LearningMetadata:", err)
				}
				if meta.LearningRoadmapID != roadmapID.String() {
					t.Errorf("expected roadmap_id %s, got %s", roadmapID, meta.LearningRoadmapID)
				}
				if meta.LearningStepID != step1ID.String() {
					t.Errorf("expected step_id %s, got %s", step1ID, meta.LearningStepID)
				}
			}
			break
		}
	}

	if !learningFound {
		t.Error("expected at least one learning quest")
	}
}

func TestRuleBasedGenerator_NoRoadmap_NoMetadata(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	gen := quest_generation.NewRuleBasedGenerator(db)
	ctx := &quest_generation.UserQuestContext{
		UserID:            userID,
		LocalDate:         timeutil.TodayVN(),
		Timezone:          "Asia/Ho_Chi_Minh",
		DisplayName:       "Test User",
		DailyQuestCount:   4,
		EnabledCategories: []string{"learning"},
		Rules: []quest_generation.QuestRuleContext{
			{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium", Weekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		},
		ActiveLearningPath: nil,
	}

	quests, err := gen.GenerateDailyQuests(testCtx(), ctx)
	if err != nil {
		t.Fatal("failed to generate quests:", err)
	}

	for _, q := range quests {
		if q.Type == models.QuestTypeLearning && len(q.LearningMetadata) > 0 {
			t.Error("learning quest without roadmap should not have LearningMetadata")
		}
	}
}

func TestRepairAndValidate_GenericLearningDroppedWithActivePath(t *testing.T) {
	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN(),
		EnabledCategories: []string{"learning"},
		Rules: []quest_generation.QuestRuleContext{
			{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium", Weekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		},
		ActiveLearningPath: &quest_generation.ActiveLearningPathDetail{
			RoadmapID:        uuid.New().String(),
			StepID:           uuid.New().String(),
			RoadmapTitle:     "Flutter",
			CurrentStepTitle: "Riverpod",
		},
	}

	candidates := []quest_generation.QuestCandidate{
		{Type: "learning", Title: "Chọn một chủ đề và ghi lại 3 ý chính", Description: "generic", Difficulty: "easy", EstimatedMinutes: 10, ReminderTime: "20:00"},
	}

	kept, report := quest_generation.RepairAndValidateCandidates(qctx, candidates, time.Now())
	if len(kept) != 0 {
		t.Errorf("expected 0 kept, got %d", len(kept))
	}
	if report.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", report.DroppedCount)
	}
}

func TestRepairAndValidate_LearningCapWithActivePath(t *testing.T) {
	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN(),
		EnabledCategories: []string{"learning"},
		Rules: []quest_generation.QuestRuleContext{
			{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium", Weekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		},
		ActiveLearningPath: &quest_generation.ActiveLearningPathDetail{
			RoadmapID:        uuid.New().String(),
			StepID:           uuid.New().String(),
			RoadmapTitle:     "Flutter",
			CurrentStepTitle: "Riverpod",
		},
	}

	candidates := []quest_generation.QuestCandidate{
		{Type: "learning", Title: "Learn Riverpod Basics", Description: "step 1", Difficulty: "easy", EstimatedMinutes: 10, ReminderTime: "20:00"},
		{Type: "learning", Title: "Learn Riverpod Advanced", Description: "step 2", Difficulty: "easy", EstimatedMinutes: 10, ReminderTime: "20:00"},
	}

	kept, report := quest_generation.RepairAndValidateCandidates(qctx, candidates, time.Now())
	if len(kept) != 1 {
		t.Errorf("expected 1 kept, got %d", len(kept))
	}
	if report.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", report.DroppedCount)
	}
}

func TestRepairAndValidate_LearningCapWithoutActivePath(t *testing.T) {
	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN(),
		EnabledCategories: []string{"learning"},
		Rules: []quest_generation.QuestRuleContext{
			{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium", Weekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		},
		ActiveLearningPath: nil,
	}

	candidates := []quest_generation.QuestCandidate{
		{Type: "learning", Title: "Learn Flutter Widgets", Description: "specific", Difficulty: "easy", EstimatedMinutes: 10, ReminderTime: "20:00"},
		{Type: "learning", Title: "Learn Dart Syntax", Description: "specific", Difficulty: "easy", EstimatedMinutes: 10, ReminderTime: "20:00"},
	}

	kept, report := quest_generation.RepairAndValidateCandidates(qctx, candidates, time.Now())
	if len(kept) > 1 {
		t.Errorf("expected at most 1 learning quest without path, got %d (dropped: %d)", len(kept), report.DroppedCount)
	}
}

func TestRepairAndValidate_LegacyActivePath_NoCap(t *testing.T) {
	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         timeutil.TodayVN(),
		EnabledCategories: []string{"learning"},
		Rules: []quest_generation.QuestRuleContext{
			{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium"},
		},
		ActiveLearningPath: &quest_generation.ActiveLearningPathDetail{
			RoadmapTitle:     "Flutter",
			CurrentStepTitle: "Riverpod",
		},
	}

	candidates := []quest_generation.QuestCandidate{
		{Type: "learning", Title: "Learn A", Description: "desc", Difficulty: "easy", EstimatedMinutes: 10, ReminderTime: "20:00"},
		{Type: "learning", Title: "Learn B", Description: "desc", Difficulty: "easy", EstimatedMinutes: 10, ReminderTime: "20:00"},
	}

	kept, _ := quest_generation.RepairAndValidateCandidates(qctx, candidates, time.Now())
	if len(kept) != 1 {
		t.Errorf("legacy ActiveLearningPath (no StepID) should cap at 1 learning, got %d kept", len(kept))
	}
}

func contextForLearningPath(userID, roadmapID, stepID uuid.UUID) *quest_generation.UserQuestContext {
	return &quest_generation.UserQuestContext{
		UserID:            userID,
		LocalDate:         timeutil.TodayVN(),
		Timezone:          "Asia/Ho_Chi_Minh",
		DisplayName:       "Test User",
		DailyQuestCount:   4,
		EnabledCategories: []string{"learning"},
		Rules: []quest_generation.QuestRuleContext{
			{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium", Weekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		},
		ActiveLearningPath: &quest_generation.ActiveLearningPathDetail{
			RoadmapID:           roadmapID.String(),
			StepID:              stepID.String(),
			RoadmapTitle:        "Flutter",
			CurrentStepTitle:    "Riverpod",
			StepOrderIndex:      0,
			CompletedSteps:      0,
			TotalSteps:          2,
			RoadmapCategory:     "Flutter",
			StepEstimatedMinutes: 30,
		},
	}
}

func testCtx() context.Context {
	return context.Background()
}

func init() {
	// suppress timezone warnings in tests
}
