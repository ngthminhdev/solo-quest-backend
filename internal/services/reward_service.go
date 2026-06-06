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
	ErrInvalidRewardType        = errors.New("invalid reward type")
	ErrRewardNotOwned           = errors.New("reward does not belong to user")
)

type RewardService struct {
	db *gorm.DB
}

func NewRewardService(db *gorm.DB) *RewardService {
	return &RewardService{db: db}
}

func (s *RewardService) GetRewards(userID uuid.UUID) (*dto.RewardListResponse, error) {
	var rewards []models.Reward
	err := s.db.Where("user_id = ? AND status = ?", userID, models.RewardStatusAvailable).
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
		items[i] = toRewardItem(r, user.RewardPoints >= r.CostPoints)
	}

	return &dto.RewardListResponse{
		Items: items,
		Wallet: dto.RewardWallet{
			RewardPoints: user.RewardPoints,
		},
	}, nil
}

func toRewardItem(r models.Reward, canClaim bool) dto.RewardItem {
	return dto.RewardItem{
		ID:              r.ID,
		Title:           r.Title,
		Description:     r.Description,
		Type:            string(r.Type),
		Status:          string(r.Status),
		CostPoints:      r.CostPoints,
		IconText:        r.IconText,
		DurationMinutes: r.DurationMinutes,
		CooldownMinutes: r.CooldownMinutes,
		ClaimCount:      r.ClaimCount,
		ImageURL:        r.ImageURL,
		ClaimedAt:       r.ClaimedAt,
		CanClaim:        canClaim,
		CreatedAt:       r.CreatedAt.UTC(),
		UpdatedAt:       r.UpdatedAt.UTC(),
	}
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
		Reward: toRewardItem(reward, false),
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

func (s *RewardService) CreateReward(userID uuid.UUID, req *dto.CreateRewardRequest) (*dto.RewardItem, error) {
	if req.Title == "" {
		return nil, errors.New("title must not be empty")
	}
	if !models.IsValidRewardType(req.Type) {
		return nil, ErrInvalidRewardType
	}
	if req.CostPoints < 0 {
		return nil, errors.New("cost_points must be >= 0")
	}

	reward := models.Reward{
		UserID:          userID,
		Title:           req.Title,
		Description:     req.Description,
		Type:            models.RewardType(req.Type),
		CostPoints:      req.CostPoints,
		IconText:        req.IconText,
		Status:          models.RewardStatusAvailable,
		DurationMinutes: req.DurationMinutes,
		CooldownMinutes: req.CooldownMinutes,
		ClaimCount:      req.ClaimCount,
	}
	if reward.IconText == "" {
		reward.IconText = "🎁"
	}

	if err := s.db.Create(&reward).Error; err != nil {
		return nil, err
	}

	item := toRewardItem(reward, true)
	return &item, nil
}

func (s *RewardService) UpdateReward(userID, rewardID uuid.UUID, req *dto.UpdateRewardRequest) (*dto.RewardItem, error) {
	var reward models.Reward
	if err := s.db.Where("id = ? AND user_id = ?", rewardID, userID).First(&reward).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRewardNotFound
		}
		return nil, err
	}

	if req.Title != nil {
		if *req.Title == "" {
			return nil, errors.New("title must not be empty")
		}
		reward.Title = *req.Title
	}
	if req.Description != nil {
		reward.Description = *req.Description
	}
	if req.Type != nil {
		if !models.IsValidRewardType(*req.Type) {
			return nil, ErrInvalidRewardType
		}
		reward.Type = models.RewardType(*req.Type)
	}
	if req.CostPoints != nil {
		if *req.CostPoints < 0 {
			return nil, errors.New("cost_points must be >= 0")
		}
		reward.CostPoints = *req.CostPoints
	}
	if req.IconText != nil {
		reward.IconText = *req.IconText
	}
	if req.Status != nil {
		if !isValidRewardStatus(*req.Status) {
			return nil, ErrInvalidRewardStatus
		}
		reward.Status = models.RewardStatus(*req.Status)
	}
	if req.DurationMinutes != nil {
		reward.DurationMinutes = req.DurationMinutes
	}
	if req.CooldownMinutes != nil {
		reward.CooldownMinutes = req.CooldownMinutes
	}
	if req.ClaimCount != nil {
		reward.ClaimCount = req.ClaimCount
	}

	if err := s.db.Save(&reward).Error; err != nil {
		return nil, err
	}

	item := toRewardItem(reward, reward.Status == models.RewardStatusAvailable)
	return &item, nil
}

func (s *RewardService) DeleteReward(userID, rewardID uuid.UUID) error {
	var reward models.Reward
	if err := s.db.Where("id = ? AND user_id = ?", rewardID, userID).First(&reward).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRewardNotFound
		}
		return err
	}

	reward.Status = models.RewardStatusExpired
	return s.db.Save(&reward).Error
}

func isValidRewardStatus(s string) bool {
	switch models.RewardStatus(s) {
	case models.RewardStatusAvailable, models.RewardStatusClaimed, models.RewardStatusRedeemed, models.RewardStatusExpired:
		return true
	}
	return false
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
