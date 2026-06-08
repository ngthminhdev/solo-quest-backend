package services

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
)

type DeviceTokenService struct {
	db *gorm.DB
}

func NewDeviceTokenService(db *gorm.DB) *DeviceTokenService {
	return &DeviceTokenService{db: db}
}

func (s *DeviceTokenService) UpsertToken(userID uuid.UUID, req *dto.RegisterDeviceTokenRequest) (*models.DeviceToken, error) {
	now := time.Now()
	token := models.DeviceToken{
		UserID:     userID,
		Token:      req.Token,
		Platform:   req.Platform,
		DeviceName: req.DeviceName,
		IsActive:   true,
		LastUsedAt: &now,
		DeletedAt:  gorm.DeletedAt{},
	}

	err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"is_active", "deleted_at", "last_used_at",
			"platform", "device_name", "updated_at",
		}),
	}).Create(&token).Error
	if err != nil {
		return nil, err
	}

	return &token, nil
}

func (s *DeviceTokenService) GetByID(userID, deviceTokenID uuid.UUID) (*models.DeviceToken, error) {
	var token models.DeviceToken
	err := s.db.Where("id = ? AND user_id = ? AND deleted_at IS NULL", deviceTokenID, userID).First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *DeviceTokenService) GetActiveTokensByUserID(userID uuid.UUID) ([]*models.DeviceToken, error) {
	var tokens []*models.DeviceToken
	err := s.db.Where("user_id = ? AND is_active = true AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

func (s *DeviceTokenService) ListActiveTokens() ([]*models.DeviceToken, error) {
	var tokens []*models.DeviceToken
	err := s.db.Where("is_active = true AND deleted_at IS NULL").Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

func (s *DeviceTokenService) DeactivateToken(userID, deviceTokenID uuid.UUID) error {
	return s.db.Model(&models.DeviceToken{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", deviceTokenID, userID).
		Update("is_active", false).Error
}

func (s *DeviceTokenService) DeactivateByRawToken(token string) error {
	return s.db.Model(&models.DeviceToken{}).
		Where("token = ? AND deleted_at IS NULL", token).
		Update("is_active", false).Error
}

func (s *DeviceTokenService) ListActiveUserIDs() ([]uuid.UUID, error) {
	var userIDs []uuid.UUID
	err := s.db.Model(&models.DeviceToken{}).
		Where("is_active = true AND deleted_at IS NULL").
		Distinct("user_id").
		Pluck("user_id", &userIDs).Error
	if err != nil {
		return nil, err
	}
	return userIDs, nil
}
