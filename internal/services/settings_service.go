package services

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
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

func (s *SettingsService) UpdateSettingsByUserID(userID uuid.UUID, req *dto.UpdateSettingsRequest) (*models.AppSettings, error) {
	settings, err := s.GetSettingsByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			settings = &models.AppSettings{
				UserID: userID,
			}
		} else {
			return nil, err
		}
	}

	if req.NotificationsEnabled != nil {
		settings.NotificationsEnabled = *req.NotificationsEnabled
	}
	if req.DailyReminderTime != nil {
		settings.DailyReminderTime = req.DailyReminderTime
	}
	if req.QuietAfterTime != nil {
		settings.QuietAfterTime = *req.QuietAfterTime
	}
	if req.QuietHoursEnabled != nil {
		settings.QuietHoursEnabled = *req.QuietHoursEnabled
	}
	if req.QuietStartTime != nil {
		settings.QuietStartTime = req.QuietStartTime
	}
	if req.QuietEndTime != nil {
		settings.QuietEndTime = req.QuietEndTime
	}
	if req.Timezone != nil {
		settings.Timezone = *req.Timezone
	}

	if err := s.db.Save(settings).Error; err != nil {
		return nil, err
	}

	return settings, nil
}
