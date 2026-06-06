package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestStartQuest_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	questActionService := services.NewQuestActionService(db)

	result, err := questActionService.StartQuest(userID, quest.ID)
	if err != nil {
		t.Fatal("failed to start quest:", err)
	}

	if result.Status != models.QuestStatusActive {
		t.Errorf("expected status 'active', got '%s'", result.Status)
	}

	if result.StartedAt == nil {
		t.Error("expected started_at to be set")
	}

	var actionCount int64
	db.Model(&models.QuestAction{}).Where("quest_id = ? AND action = ?", quest.ID, models.QuestActionStart).Count(&actionCount)
	if actionCount != 1 {
		t.Errorf("expected 1 start action, got %d", actionCount)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeQuestStarted).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 questStarted log, got %d", logCount)
	}
}

func TestStartQuest_AlreadyActive(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusActive)

	questActionService := services.NewQuestActionService(db)

	_, err := questActionService.StartQuest(userID, quest.ID)
	if err != services.ErrInvalidQuestStatus {
		t.Errorf("expected ErrInvalidQuestStatus, got %v", err)
	}
}

func TestStartQuest_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	questActionService := services.NewQuestActionService(db)

	_, err := questActionService.StartQuest(userID, uuid.New())
	if err != services.ErrQuestNotFound {
		t.Errorf("expected ErrQuestNotFound, got %v", err)
	}
}

func TestCompleteQuest_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusActive)

	questActionService := services.NewQuestActionService(db)

	result, err := questActionService.CompleteQuest(userID, quest.ID, "Done!")
	if err != nil {
		t.Fatal("failed to complete quest:", err)
	}

	if result.Quest.Status != models.QuestStatusCompleted {
		t.Errorf("expected status 'completed', got '%s'", result.Quest.Status)
	}

	if result.Quest.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}

	var actionCount int64
	db.Model(&models.QuestAction{}).Where("quest_id = ? AND action = ?", quest.ID, models.QuestActionComplete).Count(&actionCount)
	if actionCount != 1 {
		t.Errorf("expected 1 complete action, got %d", actionCount)
	}

	var user models.UserProfile
	db.Where("id = ?", userID).First(&user)
	if user.TotalExp != quest.XPReward {
		t.Errorf("expected total_exp %d, got %d", quest.XPReward, user.TotalExp)
	}
	if user.RewardPoints != 100+quest.XPReward {
		t.Errorf("expected reward_points %d, got %d", 100+quest.XPReward, user.RewardPoints)
	}
	if user.TotalCompletedQuests != 1 {
		t.Errorf("expected total_completed_quests 1, got %d", user.TotalCompletedQuests)
	}

	var expTxCount int64
	db.Model(&models.XPTransaction{}).Where("user_id = ? AND currency = ? AND source = ?", userID, models.XPCurrencyXP, models.XPSourceTypeQuestCompletion).Count(&expTxCount)
	if expTxCount != 1 {
		t.Errorf("expected 1 exp transaction, got %d", expTxCount)
	}

	var rewardTxCount int64
	db.Model(&models.XPTransaction{}).Where("user_id = ? AND currency = ? AND source = ?", userID, models.XPCurrencyRewardPoints, models.XPSourceTypeQuestCompletion).Count(&rewardTxCount)
	if rewardTxCount != 1 {
		t.Errorf("expected 1 reward points transaction, got %d", rewardTxCount)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeQuestCompleted).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 questCompleted log, got %d", logCount)
	}
}

func TestCompleteQuest_DoubleAward(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusActive)

	questActionService := services.NewQuestActionService(db)

	_, err := questActionService.CompleteQuest(userID, quest.ID, "First completion")
	if err != nil {
		t.Fatal("first completion failed:", err)
	}

	_, err = questActionService.CompleteQuest(userID, quest.ID, "Second completion")
	if err != services.ErrQuestAlreadyCompleted {
		t.Errorf("expected ErrQuestAlreadyCompleted, got %v", err)
	}

	var expTxCount int64
	db.Model(&models.XPTransaction{}).Where("user_id = ? AND currency = ?", userID, models.XPCurrencyXP).Count(&expTxCount)
	if expTxCount != 1 {
		t.Errorf("expected 1 exp transaction (no double award), got %d", expTxCount)
	}
}

func TestCompleteQuest_PendingQuest(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	questActionService := services.NewQuestActionService(db)

	result, err := questActionService.CompleteQuest(userID, quest.ID, "")
	if err != nil {
		t.Fatal("failed to complete pending quest:", err)
	}

	if result.Quest.Status != models.QuestStatusCompleted {
		t.Errorf("expected status 'completed', got '%s'", result.Quest.Status)
	}
}

func TestSkipQuest_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	questActionService := services.NewQuestActionService(db)

	result, err := questActionService.SkipQuest(userID, quest.ID, "Đang bận")
	if err != nil {
		t.Fatal("failed to skip quest:", err)
	}

	if result.Status != models.QuestStatusSkipped {
		t.Errorf("expected status 'skipped', got '%s'", result.Status)
	}

	var actionCount int64
	db.Model(&models.QuestAction{}).Where("quest_id = ? AND action = ?", quest.ID, models.QuestActionSkip).Count(&actionCount)
	if actionCount != 1 {
		t.Errorf("expected 1 skip action, got %d", actionCount)
	}

	var user models.UserProfile
	db.Where("id = ?", userID).First(&user)
	if user.TotalSkippedQuests != 1 {
		t.Errorf("expected total_skipped_quests 1, got %d", user.TotalSkippedQuests)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeQuestSkipped).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 questSkipped log, got %d", logCount)
	}
}

func TestSkipQuest_CompletedQuest(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusCompleted)

	questActionService := services.NewQuestActionService(db)

	_, err := questActionService.SkipQuest(userID, quest.ID, "Reason")
	if err != services.ErrQuestAlreadyCompleted {
		t.Errorf("expected ErrQuestAlreadyCompleted, got %v", err)
	}
}

func TestSnoozeQuest_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	questActionService := services.NewQuestActionService(db)

	result, err := questActionService.SnoozeQuest(userID, quest.ID, 15)
	if err != nil {
		t.Fatal("failed to snooze quest:", err)
	}

	if result.Status != models.QuestStatusSnoozed {
		t.Errorf("expected status 'snoozed', got '%s'", result.Status)
	}

	if result.SnoozedUntil == nil {
		t.Fatal("expected snoozed_until to be set")
	}

	expectedMin := time.Now().Add(15 * time.Minute).Add(-5 * time.Second)
	expectedMax := time.Now().Add(15 * time.Minute).Add(5 * time.Second)
	if result.SnoozedUntil.Before(expectedMin) || result.SnoozedUntil.After(expectedMax) {
		t.Errorf("snoozed_until not in expected range")
	}

	var actionCount int64
	db.Model(&models.QuestAction{}).Where("quest_id = ? AND action = ?", quest.ID, models.QuestActionSnooze).Count(&actionCount)
	if actionCount != 1 {
		t.Errorf("expected 1 snooze action, got %d", actionCount)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeQuestSnoozed).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 questSnoozed log, got %d", logCount)
	}
}

func TestSnoozeQuest_InvalidMinutes(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	questActionService := services.NewQuestActionService(db)

	_, err := questActionService.SnoozeQuest(userID, quest.ID, 7)
	if err != services.ErrInvalidSnoozeDuration {
		t.Errorf("expected ErrInvalidSnoozeDuration, got %v", err)
	}

	_, err = questActionService.SnoozeQuest(userID, quest.ID, 0)
	if err != services.ErrInvalidSnoozeDuration {
		t.Errorf("expected ErrInvalidSnoozeDuration for 0, got %v", err)
	}

	_, err = questActionService.SnoozeQuest(userID, quest.ID, 120)
	if err != services.ErrInvalidSnoozeDuration {
		t.Errorf("expected ErrInvalidSnoozeDuration for 120, got %v", err)
	}
}

func TestCompleteQuest_CreatesLevelUpLog(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Set user close to leveling up (NextLevelExp=100, so need 90+ current exp)
	var user models.UserProfile
	db.Where("id = ?", userID).First(&user)
	user.CurrentLevelExp = 90
	db.Save(&user)

	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusActive)
	// quest gives 10 XP, so 90+10=100 => level up
	quest.XPReward = 10
	db.Save(quest)

	questActionService := services.NewQuestActionService(db)

	result, err := questActionService.CompleteQuest(userID, quest.ID, "")
	if err != nil {
		t.Fatal("failed to complete quest:", err)
	}

	if result.Profile.Level != 2 {
		t.Errorf("expected level 2, got %d", result.Profile.Level)
	}

	var levelUpLogCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLevelUp).Count(&levelUpLogCount)
	if levelUpLogCount != 1 {
		t.Errorf("expected 1 level_up log, got %d", levelUpLogCount)
	}
}

func TestCompleteQuest_NoLevelUpLogWhenNotLeveling(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusActive)

	questActionService := services.NewQuestActionService(db)

	_, err := questActionService.CompleteQuest(userID, quest.ID, "")
	if err != nil {
		t.Fatal("failed to complete quest:", err)
	}

	var levelUpLogCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLevelUp).Count(&levelUpLogCount)
	if levelUpLogCount != 0 {
		t.Errorf("expected 0 level_up logs, got %d", levelUpLogCount)
	}
}

func TestSnoozeQuest_CompletedQuest(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusCompleted)

	questActionService := services.NewQuestActionService(db)

	_, err := questActionService.SnoozeQuest(userID, quest.ID, 15)
	if err != services.ErrInvalidQuestStatus {
		t.Errorf("expected ErrInvalidQuestStatus, got %v", err)
	}
}
