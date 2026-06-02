package services

import (
	"encoding/json"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/pkg/logger"
)

type OnboardingRequest struct {
	DisplayName              string   `json:"display_name" binding:"required"`
	Age                      int      `json:"age"`
	Gender                   string   `json:"gender" binding:"required"`
	HeightCm                 float64  `json:"height_cm"`
	WeightKg                 float64  `json:"weight_kg"`
	MainActivity             string   `json:"main_activity" binding:"required"`
	WorkScheduleType         string   `json:"work_schedule_type"`
	WorkStartTime            string   `json:"work_start_time"`
	WorkEndTime              string   `json:"work_end_time"`
	FreeTimePreference       string   `json:"free_time_preference"`
	ActivityLevel            string   `json:"activity_level"`
	LastWorkout              string   `json:"last_workout"`
	HealthLimitations        []string `json:"health_limitations"`
	MainGoals                []string `json:"main_goals" binding:"required"`
	WakeUpTime               string   `json:"wake_up_time"`
	TargetSleepTime          string   `json:"target_sleep_time"`
	FreeTimeStart            string   `json:"free_time_start"`
	FreeTimeEnd              string   `json:"free_time_end"`
	LearningTimePreference   string   `json:"learning_time_preference"`
	MovementTimePreference   string   `json:"movement_time_preference"`
	BreakReminderInterval    int      `json:"break_reminder_interval"`
	BreakDuration            string   `json:"break_duration"`
	WaterReminderMode        string   `json:"water_reminder_mode"`
	QuietAfterTime           string   `json:"quiet_after_time"`
	PreferredRewards         []string `json:"preferred_rewards"`
}

type OnboardingService struct {
	db *gorm.DB
}

func NewOnboardingService(db *gorm.DB) *OnboardingService {
	return &OnboardingService{db: db}
}

func (s *OnboardingService) SaveOnboarding(userID uuid.UUID, req *OnboardingRequest) (*models.UserProfile, *models.OnboardingAnswer, error) {
	answersJSON, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}

	var onboarding models.OnboardingAnswer
	err = s.db.Where("user_id = ?", userID).First(&onboarding).Error
	if err == gorm.ErrRecordNotFound {
		onboarding = models.OnboardingAnswer{
			UserID:    userID,
			Answers:   datatypes.JSON(answersJSON),
			Completed: true,
		}
		if err := s.db.Create(&onboarding).Error; err != nil {
			logger.L.Error("failed to create onboarding answer", zap.Error(err))
			return nil, nil, err
		}
	} else if err != nil {
		return nil, nil, err
	} else {
		onboarding.Answers = datatypes.JSON(answersJSON)
		onboarding.Completed = true
		if err := s.db.Save(&onboarding).Error; err != nil {
			logger.L.Error("failed to update onboarding answer", zap.Error(err))
			return nil, nil, err
		}
	}

	mainGoalsJSON, _ := json.Marshal(req.MainGoals)
	healthLimitationsJSON, _ := json.Marshal(req.HealthLimitations)
	preferredRewardsJSON, _ := json.Marshal(req.PreferredRewards)

	var user models.UserProfile
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, nil, err
	}

	user.DisplayName = req.DisplayName
	user.Age = &req.Age
	user.Gender = &req.Gender
	user.HeightCm = &req.HeightCm
	user.WeightKg = &req.WeightKg
	user.MainActivity = &req.MainActivity
	user.MainGoals = datatypes.JSON(mainGoalsJSON)
	user.HealthLimitations = datatypes.JSON(healthLimitationsJSON)
	user.PreferredRewards = datatypes.JSON(preferredRewardsJSON)
	user.HasCompletedOnboarding = true

	if req.QuietAfterTime != "" {
		user.QuietAfterTime = &req.QuietAfterTime
	}

	if err := s.db.Save(&user).Error; err != nil {
		logger.L.Error("failed to update user profile", zap.Error(err))
		return nil, nil, err
	}

	var settings models.AppSettings
	err = s.db.Where("user_id = ?", userID).First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		settings = models.AppSettings{
			UserID:         userID,
			QuietAfterTime: req.QuietAfterTime,
		}
		s.db.Create(&settings)
	} else if err == nil && req.QuietAfterTime != "" {
		settings.QuietAfterTime = req.QuietAfterTime
		s.db.Save(&settings)
	}

	logEntry := models.LogEntry{
		UserID:  userID,
		Type:    models.LogEntryTypeSystem,
		Title:   "Hoàn thành onboarding",
		Content: "Thông tin cá nhân và mục tiêu đã được cập nhật",
	}
	if err := s.db.Create(&logEntry).Error; err != nil {
		logger.L.Error("failed to create log entry", zap.Error(err))
	}

	// TODO: generate quest rules from onboarding goals
	// TODO: generate reminder settings from onboarding preferences
	// TODO: generate schedule blocks from work schedule

	return &user, &onboarding, nil
}

func (s *OnboardingService) GetOnboardingStatus(userID uuid.UUID) (bool, string, error) {
	var user models.UserProfile
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return false, "", err
	}

	return user.HasCompletedOnboarding, user.DisplayName, nil
}
