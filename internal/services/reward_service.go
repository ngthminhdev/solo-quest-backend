package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

var (
	ErrRewardNotFound           = errors.New("reward not found")
	ErrRewardAlreadyClaimed     = errors.New("reward already claimed")
	ErrInsufficientRewardPoints = errors.New("insufficient reward points")
	ErrInvalidRewardStatus      = errors.New("invalid reward status")
)

type RewardService struct {
	db *gorm.DB
}

func NewRewardService(db *gorm.DB) *RewardService {
	return &RewardService{db: db}
}

func (s *RewardService) GetRewards(userID uuid.UUID) (*dto.RewardListResponse, error) {
	var rewards []models.Reward
	err := s.db.Where("user_id = ?", userID).
		Order("cost_points ASC").
		Find(&rewards).Error
	if err != nil {
		return nil, err
	}

	var user models.UserProfile
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}

	items := make([]dto.RewardItem, len(rewards))
	for i, r := range rewards {
		items[i] = dto.RewardItem{
			ID:          r.ID,
			Title:       r.Title,
			Description: r.Description,
			Type:        string(r.Type),
			Status:      string(r.Status),
			CostPoints:  r.CostPoints,
			IconText:    r.IconText,
			ClaimedAt:   r.ClaimedAt,
			CanClaim:    r.Status == models.RewardStatusAvailable && user.RewardPoints >= r.CostPoints,
			CreatedAt:   r.CreatedAt.UTC(),
			UpdatedAt:   r.UpdatedAt.UTC(),
		}
	}

	return &dto.RewardListResponse{
		Items: items,
		Wallet: dto.RewardWallet{
			RewardPoints: user.RewardPoints,
		},
	}, nil
}

func (s *RewardService) ClaimReward(userID uuid.UUID, rewardID uuid.UUID) (*dto.ClaimRewardResponse, error) {
	var reward models.Reward
	err := s.db.Where("id = ? AND user_id = ?", rewardID, userID).First(&reward).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRewardNotFound
		}
		return nil, err
	}

	if reward.Status != models.RewardStatusAvailable {
		return nil, ErrRewardAlreadyClaimed
	}

	now := timeutil.NowUTC()

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var user models.UserProfile
	if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if user.RewardPoints < reward.CostPoints {
		tx.Rollback()
		return nil, ErrInsufficientRewardPoints
	}

	user.RewardPoints -= reward.CostPoints
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	reward.Status = models.RewardStatusClaimed
	reward.ClaimedAt = &now
	reward.UpdatedAt = now
	if err := tx.Save(&reward).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	redemption := models.RewardRedemption{
		UserID:    userID,
		RewardID:  rewardID,
		Cost:      reward.CostPoints,
		CreatedAt: now,
	}
	if err := tx.Create(&redemption).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	rewardTx := models.XPTransaction{
		UserID:       userID,
		Amount:       -reward.CostPoints,
		Currency:     models.XPCurrencyRewardPoints,
		Source:       models.XPSourceTypeRewardClaim,
		ReferenceID:  &rewardID,
		Description:  fmt.Sprintf("Đổi thưởng: %s", reward.Title),
		BalanceAfter: user.RewardPoints,
		CreatedAt:    now,
	}
	if err := tx.Create(&rewardTx).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	logEntry := models.LogEntry{
		UserID:        userID,
		Type:          models.LogEntryTypeRewardClaimed,
		Title:         fmt.Sprintf("Đổi thưởng: %s", reward.Title),
		Content:       fmt.Sprintf("-%d điểm thưởng", reward.CostPoints),
		PointsChanged: -reward.CostPoints,
		CreatedAt:     now,
	}
	if err := tx.Create(&logEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &dto.ClaimRewardResponse{
		Reward: dto.RewardItem{
			ID:         reward.ID,
			Title:      reward.Title,
			Description: reward.Description,
			Type:       string(reward.Type),
			Status:     string(reward.Status),
			CostPoints: reward.CostPoints,
			IconText:   reward.IconText,
			ClaimedAt:  reward.ClaimedAt,
			CanClaim:   false,
			CreatedAt:  reward.CreatedAt.UTC(),
			UpdatedAt:  reward.UpdatedAt.UTC(),
		},
		Redemption: dto.RedemptionItem{
			ID:          redemption.ID,
			RewardID:    rewardID,
			RewardTitle: reward.Title,
			RewardType:  string(reward.Type),
			IconText:    reward.IconText,
			PointsSpent: reward.CostPoints,
			CreatedAt:   now,
		},
		Transaction: dto.XPTransactionItem{
			ID:           rewardTx.ID,
			Amount:       rewardTx.Amount,
			Currency:     string(rewardTx.Currency),
			Source:       string(rewardTx.Source),
			ReferenceID:  rewardTx.ReferenceID,
			Description:  rewardTx.Description,
			BalanceAfter: rewardTx.BalanceAfter,
			CreatedAt:    now,
		},
		Profile: dto.ProfileSummary{
			ID:           user.ID,
			RewardPoints: user.RewardPoints,
		},
		Message: "reward claimed successfully",
	}, nil
}

func (s *RewardService) GetRedemptions(userID uuid.UUID, filter dto.RedemptionFilter) (*dto.RedemptionListResponse, error) {
	var redemptions []models.RewardRedemption
	err := s.db.Where("user_id = ?", userID).
		Preload("Reward").
		Order("created_at DESC").
		Offset(filter.Offset).
		Limit(filter.Limit).
		Find(&redemptions).Error
	if err != nil {
		return nil, err
	}

	items := make([]dto.RedemptionItem, len(redemptions))
	for i, r := range redemptions {
		items[i] = dto.RedemptionItem{
			ID:          r.ID,
			RewardID:    r.RewardID,
			RewardTitle: r.Reward.Title,
			RewardType:  string(r.Reward.Type),
			IconText:    r.Reward.IconText,
			PointsSpent: r.Cost,
			CreatedAt:   r.CreatedAt.UTC(),
		}
	}

	return &dto.RedemptionListResponse{
		Items:  items,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}
