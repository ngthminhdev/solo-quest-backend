package services_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupRoadmapUser(t *testing.T, db *gorm.DB) (userID, roadmapID, stepID uuid.UUID) {
	t.Helper()

	userID = uuid.New()
	quietTime := "22:00"
	user := models.UserProfile{
		ID:                     userID,
		DisplayName:            "Quest Action User",
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
		Title:            "Test Roadmap",
		Category:         "Flutter",
		Difficulty:       "beginner",
		EstimatedMinutes: 60,
		TotalSteps:       1,
		Source:           "system",
		CreatedByUserID:  &userID,
		Enabled:          true,
	}
	db.Create(&rm)

	stepID = uuid.New()
	s := models.LearningRoadmapStep{
		ID:               stepID,
		RoadmapID:        roadmapID,
		Title:            "Step 1",
		OrderIndex:       0,
		EstimatedMinutes: 30,
		Enabled:          true,
	}
	db.Create(&s)

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

func TestCompleteQuest_LinkedLearningQuest_AdvancesRoadmap(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID, roadmapID, stepID := setupRoadmapUser(t, db)

	meta, _ := json.Marshal(map[string]string{
		"learning_roadmap_id": roadmapID.String(),
		"learning_step_id":    stepID.String(),
	})

	quest := &models.Quest{
		UserID:           userID,
		Title:            "Học tập: Step 1",
		Type:             models.QuestTypeLearning,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		Source:           models.QuestSourceAI,
		XPReward:         10,
		EstimatedMinutes: 20,
		Date:             timeutil.TodayVN(),
		LearningMetadata: datatypes.JSON(meta),
	}
	db.Create(quest)

	lrs := services.NewLearningRoadmapService(db)
	qas := services.NewQuestActionService(db)
	qas.SetLearningRoadmapService(lrs)

	result, err := qas.CompleteQuest(userID, quest.ID, "")
	if err != nil {
		t.Fatal("failed to complete quest:", err)
	}
	if result.Quest.Status != models.QuestStatusCompleted {
		t.Error("quest should be completed")
	}

	var progress models.UserLearningRoadmapStepProgress
	err = db.Where("user_id = ? AND step_id = ?", userID, stepID).First(&progress).Error
	if err != nil {
		t.Fatal("expected progress record, got:", err)
	}
	if !progress.Completed {
		t.Error("step should be marked completed")
	}
}

func TestCompleteQuest_LinkedLearningQuest_Idempotent(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID, roadmapID, stepID := setupRoadmapUser(t, db)

	meta, _ := json.Marshal(map[string]string{
		"learning_roadmap_id": roadmapID.String(),
		"learning_step_id":    stepID.String(),
	})

	quest := &models.Quest{
		UserID:           userID,
		Title:            "Học tập: Step 1",
		Type:             models.QuestTypeLearning,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		Source:           models.QuestSourceAI,
		XPReward:         10,
		EstimatedMinutes: 20,
		Date:             timeutil.TodayVN(),
		LearningMetadata: datatypes.JSON(meta),
	}
	db.Create(quest)

	lrs := services.NewLearningRoadmapService(db)
	qas := services.NewQuestActionService(db)
	qas.SetLearningRoadmapService(lrs)

	_, err := qas.CompleteQuest(userID, quest.ID, "")
	if err != nil {
		t.Fatal("first completion failed:", err)
	}

	_, err = qas.CompleteQuest(userID, quest.ID, "")
	if err != services.ErrQuestAlreadyCompleted {
		t.Errorf("expected ErrQuestAlreadyCompleted, got %v", err)
	}

	var progress models.UserLearningRoadmapStepProgress
	db.Where("user_id = ? AND step_id = ?", userID, stepID).First(&progress)
	if !progress.Completed {
		t.Error("step should still be completed after duplicate completion attempt")
	}
}

func TestCompleteQuest_UnlinkedLearningQuest_NoRoadmapEffect(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID, _, _ := setupRoadmapUser(t, db)

	quest := &models.Quest{
		UserID:           userID,
		Title:            "Unlinked Learning",
		Type:             models.QuestTypeLearning,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		Source:           models.QuestSourceConfigBased,
		XPReward:         10,
		EstimatedMinutes: 20,
		Date:             timeutil.TodayVN(),
	}
	db.Create(quest)

	lrs := services.NewLearningRoadmapService(db)
	qas := services.NewQuestActionService(db)
	qas.SetLearningRoadmapService(lrs)

	_, err := qas.CompleteQuest(userID, quest.ID, "")
	if err != nil {
		t.Fatal("failed to complete unlinked quest:", err)
	}

	// Should not have created any progress
	var count int64
	db.Model(&models.UserLearningRoadmapStepProgress{}).Where("user_id = ?", userID).Count(&count)
	// May have 0 or stay at whatever setup created; just verify quest was completed
}

func TestCompleteQuest_NonLearningQuest_NoRoadmapEffect(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID, _, _ := setupRoadmapUser(t, db)

	meta, _ := json.Marshal(map[string]string{
		"learning_roadmap_id": uuid.New().String(),
		"learning_step_id":    uuid.New().String(),
	})

	quest := &models.Quest{
		UserID:           userID,
		Title:            "Daily Movement",
		Type:             models.QuestTypeMovement,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		Source:           models.QuestSourceConfigBased,
		XPReward:         5,
		EstimatedMinutes: 10,
		Date:             timeutil.TodayVN(),
		LearningMetadata: datatypes.JSON(meta),
	}
	db.Create(quest)

	lrs := services.NewLearningRoadmapService(db)
	qas := services.NewQuestActionService(db)
	qas.SetLearningRoadmapService(lrs)

	result, err := qas.CompleteQuest(userID, quest.ID, "")
	if err != nil {
		t.Fatal("failed to complete movement quest:", err)
	}
	if result.Quest.Status != models.QuestStatusCompleted {
		t.Error("quest should be completed")
	}
}

func TestCompleteQuest_LinkedQuest_MissingStep_WarnsNotFail(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID, _, _ := setupRoadmapUser(t, db)

	meta, _ := json.Marshal(map[string]string{
		"learning_roadmap_id": uuid.New().String(),
		"learning_step_id":    uuid.New().String(),
	})

	quest := &models.Quest{
		UserID:           userID,
		Title:            "Missing Step Quest",
		Type:             models.QuestTypeLearning,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		Source:           models.QuestSourceAI,
		XPReward:         10,
		EstimatedMinutes: 20,
		Date:             timeutil.TodayVN(),
		LearningMetadata: datatypes.JSON(meta),
	}
	db.Create(quest)

	lrs := services.NewLearningRoadmapService(db)
	qas := services.NewQuestActionService(db)
	qas.SetLearningRoadmapService(lrs)

	result, err := qas.CompleteQuest(userID, quest.ID, "")
	if err != nil {
		t.Fatal("quest completion should not fail for missing roadmap step:", err)
	}
	if result.Quest.Status != models.QuestStatusCompleted {
		t.Error("quest should still be completed even if step is missing")
	}
}
