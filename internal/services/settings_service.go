package services

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
)

type SettingsService struct {
	db *gorm.DB
}

func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{db: db}
}

func (s *SettingsService) GetSettingsByUserID(userID uuid.UUID) (*models.AppSettings, error) {
	var settings models.AppSettings
	err := s.db.Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}
