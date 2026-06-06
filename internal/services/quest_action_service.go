package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/pkg/logger"
)

var (
	ErrQuestNotFound         = errors.New("quest not found")
	ErrInvalidQuestStatus    = errors.New("invalid quest status for this action")
	ErrQuestAlreadyCompleted = errors.New("quest already completed")
	ErrInvalidSnoozeDuration = errors.New("invalid snooze duration, must be 5, 10, 15, 30, or 60 minutes")
)

var validSnoozeDurations = map[int]bool{
	5:  true,
	10: true,
	15: true,
	30: true,
	60: true,
}

type CompleteQuestResult struct {
	Quest                    *models.Quest           `json:"quest"`
	EXPTransaction           *models.XPTransaction   `json:"exp_transaction"`
	RewardPointsTransaction *models.XPTransaction   `json:"reward_points_transaction"`
	Profile                  *models.UserProfile     `json:"profile"`
	Message                  string                  `json:"message"`
}

type QuestActionService struct {
	db *gorm.DB
}

func NewQuestActionService(db *gorm.DB) *QuestActionService {
	return &QuestActionService{db: db}
}

func (s *QuestActionService) StartQuest(userID uuid.UUID, questID uuid.UUID) (*models.Quest, error) {
	var quest models.Quest
	err := s.db.Where("id = ? AND user_id = ?", questID, userID).First(&quest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestNotFound
		}
		return nil, err
	}

	now := timeutil.NowUTC()
	if quest.Status == models.QuestStatusSnoozed && quest.SnoozedUntil != nil && quest.SnoozedUntil.After(now) {
		return nil, ErrInvalidQuestStatus
	}

	if quest.Status != models.QuestStatusPending && quest.Status != models.QuestStatusSnoozed {
		return nil, ErrInvalidQuestStatus
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	quest.Status = models.QuestStatusActive
	quest.StartedAt = &now
	quest.UpdatedAt = now

	if err := tx.Save(&quest).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	action := models.QuestAction{
		QuestID:   questID,
		UserID:    userID,
		Action:    models.QuestActionStart,
		CreatedAt: now,
	}
	if err := tx.Create(&action).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	logEntry := models.LogEntry{
		UserID:    userID,
		Type:       models.LogEntryTypeQuestStarted,
		Title:      fmt.Sprintf("Bắt đầu quest: %s", quest.Title),
		Content:    "",
		QuestID:    &questID,
		QuestType:  &quest.Type,
		CreatedAt:  now,
	}
	if err := tx.Create(&logEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	logger.L.Info("quest started",
		zap.String("quest_id", questID.String()),
		zap.String("user_id", userID.String()),
	)

	return &quest, nil
}

func (s *QuestActionService) CompleteQuest(userID uuid.UUID, questID uuid.UUID, note string) (*CompleteQuestResult, error) {
	var quest models.Quest
	err := s.db.Where("id = ? AND user_id = ?", questID, userID).First(&quest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestNotFound
		}
		return nil, err
	}

	if quest.Status == models.QuestStatusCompleted {
		return nil, ErrQuestAlreadyCompleted
	}

	now := timeutil.NowUTC()
	if quest.Status == models.QuestStatusSnoozed && quest.SnoozedUntil != nil && quest.SnoozedUntil.After(now) {
		return nil, ErrInvalidQuestStatus
	}

	if quest.Status != models.QuestStatusActive && quest.Status != models.QuestStatusPending && quest.Status != models.QuestStatusSnoozed {
		return nil, ErrInvalidQuestStatus
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	quest.Status = models.QuestStatusCompleted
	quest.CompletedAt = &now
	quest.UpdatedAt = now

	if err := tx.Save(&quest).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	action := models.QuestAction{
		QuestID:   questID,
		UserID:    userID,
		Action:    models.QuestActionComplete,
		Note:      note,
		CreatedAt: now,
	}
	if err := tx.Create(&action).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	var user models.UserProfile
	if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	user.TotalExp += quest.XPReward
	user.CurrentLevelExp += quest.XPReward
	user.TotalCompletedQuests += 1
	user.RewardPoints += quest.XPReward

	leveledUp := false
	if user.CurrentLevelExp >= user.NextLevelExp {
		user.Level += 1
		user.CurrentLevelExp = 0
		user.NextLevelExp = user.NextLevelExp * 2
		leveledUp = true
	}

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	expTx := models.XPTransaction{
		UserID:       userID,
		Amount:       quest.XPReward,
		Currency:     models.XPCurrencyXP,
		Source:       models.XPSourceTypeQuestCompletion,
		ReferenceID:  &questID,
		Description:  fmt.Sprintf("Hoàn thành quest: %s", quest.Title),
		BalanceAfter: user.TotalExp,
		CreatedAt:    now,
	}
	if err := tx.Create(&expTx).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	rewardTx := models.XPTransaction{
		UserID:       userID,
		Amount:       quest.XPReward,
		Currency:     models.XPCurrencyRewardPoints,
		Source:       models.XPSourceTypeQuestCompletion,
		ReferenceID:  &questID,
		Description:  fmt.Sprintf("Nhận điểm thưởng từ quest: %s", quest.Title),
		BalanceAfter: user.RewardPoints,
		CreatedAt:    now,
	}
	if err := tx.Create(&rewardTx).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	logEntry := models.LogEntry{
		UserID:         userID,
		Type:            models.LogEntryTypeQuestCompleted,
		Title:           fmt.Sprintf("Hoàn thành quest: %s", quest.Title),
		Content:         fmt.Sprintf("+%d EXP", quest.XPReward),
		QuestID:         &questID,
		QuestType:       &quest.Type,
		ExpChanged:      quest.XPReward,
		PointsChanged:   quest.XPReward,
		CreatedAt:       now,
	}
	if err := tx.Create(&logEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if leveledUp {
		levelUpLog := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeLevelUp,
			Title:     fmt.Sprintf("Lên cấp %d", user.Level),
			Content:   fmt.Sprintf("Chúc mừng! Bạn đã đạt cấp %d", user.Level),
			CreatedAt: now,
		}
		if err := tx.Create(&levelUpLog).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	logger.L.Info("quest completed",
		zap.String("quest_id", questID.String()),
		zap.String("user_id", userID.String()),
		zap.Int("exp_earned", quest.XPReward),
	)

	return &CompleteQuestResult{
		Quest:                    &quest,
		EXPTransaction:           &expTx,
		RewardPointsTransaction: &rewardTx,
		Profile:                  &user,
		Message:                  "quest completed successfully",
	}, nil
}

func (s *QuestActionService) SkipQuest(userID uuid.UUID, questID uuid.UUID, reason string) (*models.Quest, error) {
	var quest models.Quest
	err := s.db.Where("id = ? AND user_id = ?", questID, userID).First(&quest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestNotFound
		}
		return nil, err
	}

	if quest.Status == models.QuestStatusCompleted {
		return nil, ErrQuestAlreadyCompleted
	}

	if quest.Status == models.QuestStatusSkipped {
		return nil, ErrInvalidQuestStatus
	}

	now := timeutil.NowUTC()

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	quest.Status = models.QuestStatusSkipped
	quest.UpdatedAt = now

	if err := tx.Save(&quest).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	action := models.QuestAction{
		QuestID:   questID,
		UserID:    userID,
		Action:    models.QuestActionSkip,
		Reason:    reason,
		CreatedAt: now,
	}
	if err := tx.Create(&action).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	var user models.UserProfile
	if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	user.TotalSkippedQuests += 1
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	logEntry := models.LogEntry{
		UserID:    userID,
		Type:       models.LogEntryTypeQuestSkipped,
		Title:      fmt.Sprintf("Bỏ qua quest: %s", quest.Title),
		Content:    reason,
		QuestID:    &questID,
		QuestType:  &quest.Type,
		CreatedAt:  now,
	}
	if err := tx.Create(&logEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	logger.L.Info("quest skipped",
		zap.String("quest_id", questID.String()),
		zap.String("user_id", userID.String()),
	)

	return &quest, nil
}

func (s *QuestActionService) SnoozeQuest(userID uuid.UUID, questID uuid.UUID, minutes int) (*models.Quest, error) {
	if !validSnoozeDurations[minutes] {
		return nil, ErrInvalidSnoozeDuration
	}

	var quest models.Quest
	err := s.db.Where("id = ? AND user_id = ?", questID, userID).First(&quest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestNotFound
		}
		return nil, err
	}

	if quest.Status == models.QuestStatusCompleted || quest.Status == models.QuestStatusSkipped {
		return nil, ErrInvalidQuestStatus
	}

	now := timeutil.NowUTC()
	snoozedUntil := now.Add(time.Duration(minutes) * time.Minute)

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	quest.Status = models.QuestStatusSnoozed
	quest.SnoozedUntil = &snoozedUntil
	quest.UpdatedAt = now

	if err := tx.Save(&quest).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	action := models.QuestAction{
		QuestID:       questID,
		UserID:        userID,
		Action:        models.QuestActionSnooze,
		SnoozeMinutes: minutes,
		CreatedAt:     now,
	}
	if err := tx.Create(&action).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	logEntry := models.LogEntry{
		UserID:    userID,
		Type:       models.LogEntryTypeQuestSnoozed,
		Title:      fmt.Sprintf("Nhắc lại quest: %s", quest.Title),
		Content:    fmt.Sprintf("Nhắc lại sau %d phút", minutes),
		QuestID:    &questID,
		QuestType:  &quest.Type,
		CreatedAt:  now,
	}
	if err := tx.Create(&logEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	logger.L.Info("quest snoozed",
		zap.String("quest_id", questID.String()),
		zap.String("user_id", userID.String()),
		zap.Int("minutes", minutes),
	)

	return &quest, nil
}
