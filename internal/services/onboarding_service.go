package services

import (
	"context"
	"encoding/json"
	"fmt"

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
	PreferredFreeTimes       []string `json:"preferred_free_times"`
	ActivityLevel            string   `json:"activity_level"`
	LastWorkout              string   `json:"last_workout"`
	HealthLimitations        []string `json:"health_limitations"`
	MainGoals                []string `json:"main_goals" binding:"required"`
	WakeUpTime               string   `json:"wake_up_time"`
	TargetSleepTime          string   `json:"target_sleep_time"`
	FreeTimeStart            string   `json:"free_time_start"`
	FreeTimeEnd              string   `json:"free_time_end"`
	LearningTimePreference   string   `json:"learning_time_preference"`
	LearningTimePreferences  []string `json:"learning_time_preferences"`
	MovementTimePreference   string   `json:"movement_time_preference"`
	MovementTimePreferences  []string `json:"movement_time_preferences"`
	BreakReminderInterval    int      `json:"break_reminder_interval"`
	BreakDuration            string   `json:"break_duration"`
	WaterReminderMode        string   `json:"water_reminder_mode"`
	QuietAfterTime           string   `json:"quiet_after_time"`
	PreferredRewards         []string `json:"preferred_rewards"`
	PreferredReviewTime      string   `json:"preferred_review_time"`
}

type OnboardingService struct {
	db                   *gorm.DB
	questSettingsService *QuestSettingsService
}

func NewOnboardingService(db *gorm.DB) *OnboardingService {
	return &OnboardingService{
		db:                   db,
		questSettingsService: NewQuestSettingsService(db),
	}
}

func (s *OnboardingService) SaveOnboarding(userID uuid.UUID, req *OnboardingRequest) (*models.UserProfile, *models.OnboardingAnswer, error) {
	if len(req.PreferredFreeTimes) == 0 && req.FreeTimePreference != "" {
		req.PreferredFreeTimes = []string{req.FreeTimePreference}
	}
	if req.FreeTimePreference == "" && len(req.PreferredFreeTimes) > 0 {
		req.FreeTimePreference = req.PreferredFreeTimes[0]
	}

	if len(req.LearningTimePreferences) == 0 && req.LearningTimePreference != "" {
		req.LearningTimePreferences = []string{req.LearningTimePreference}
	}
	if req.LearningTimePreference == "" && len(req.LearningTimePreferences) > 0 {
		req.LearningTimePreference = req.LearningTimePreferences[0]
	}

	if len(req.MovementTimePreferences) == 0 && req.MovementTimePreference != "" {
		req.MovementTimePreferences = []string{req.MovementTimePreference}
	}
	if req.MovementTimePreference == "" && len(req.MovementTimePreferences) > 0 {
		req.MovementTimePreference = req.MovementTimePreferences[0]
	}

	answersJSON, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}

	// Start transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, nil, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var onboarding models.OnboardingAnswer
	err = tx.Where("user_id = ?", userID).First(&onboarding).Error
	if err == gorm.ErrRecordNotFound {
		onboarding = models.OnboardingAnswer{
			UserID:    userID,
			Answers:   datatypes.JSON(answersJSON),
			Completed: true,
		}
		if err := tx.Create(&onboarding).Error; err != nil {
			tx.Rollback()
			logger.L.Error("failed to create onboarding answer", zap.Error(err))
			return nil, nil, err
		}
	} else if err != nil {
		tx.Rollback()
		return nil, nil, err
	} else {
		onboarding.Answers = datatypes.JSON(answersJSON)
		onboarding.Completed = true
		if err := tx.Save(&onboarding).Error; err != nil {
			tx.Rollback()
			logger.L.Error("failed to update onboarding answer", zap.Error(err))
			return nil, nil, err
		}
	}

	mainGoalsJSON, _ := json.Marshal(req.MainGoals)
	healthLimitationsJSON, _ := json.Marshal(req.HealthLimitations)
	preferredRewardsJSON, _ := json.Marshal(req.PreferredRewards)

	var user models.UserProfile
	if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
		tx.Rollback()
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

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		logger.L.Error("failed to update user profile", zap.Error(err))
		return nil, nil, err
	}

	var settings models.AppSettings
	err = tx.Where("user_id = ?", userID).First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		settings = models.AppSettings{
			UserID:         userID,
			QuietAfterTime: req.QuietAfterTime,
		}
		if err := tx.Create(&settings).Error; err != nil {
			tx.Rollback()
			logger.L.Error("failed to create app settings", zap.Error(err))
			return nil, nil, err
		}
	} else if err == nil && req.QuietAfterTime != "" {
		settings.QuietAfterTime = req.QuietAfterTime
		if err := tx.Save(&settings).Error; err != nil {
			tx.Rollback()
			logger.L.Error("failed to save app settings", zap.Error(err))
			return nil, nil, err
		}
	}

	logEntry := models.LogEntry{
		UserID:  userID,
		Type:    models.LogEntryTypeSystem,
		Title:   "Hoàn thành onboarding",
		Content: "Thông tin cá nhân và mục tiêu đã được cập nhật",
	}
	if err := tx.Create(&logEntry).Error; err != nil {
		logger.L.Error("failed to create log entry", zap.Error(err))
		// Don't fail onboarding if log entry creation fails
	}

	// 5. Sync default quest settings and rules inside transaction
	ctx := context.Background()
	if err := SyncQuestSettingsFromOnboarding(ctx, tx, userID, req); err != nil {
		tx.Rollback()
		logger.L.Error("failed to sync quest settings from onboarding", zap.Error(err))
		return nil, nil, fmt.Errorf("failed to sync quest settings: %w", err)
	}

	// Generate schedule blocks from work schedule if the user has none yet
	var scheduleBlockCount int64
	if err := tx.Model(&models.ScheduleBlock{}).Where("user_id = ?", userID).Count(&scheduleBlockCount).Error; err != nil {
		logger.L.Error("failed to count schedule blocks during onboarding", zap.Error(err))
	} else if scheduleBlockCount == 0 {
		if req.WorkScheduleType != "" {
			var days []int
			var isBusy bool
			var isFlexible bool
			var title string

			switch req.WorkScheduleType {
			case "weekdays":
				days = []int{1, 2, 3, 4, 5}
				isBusy = true
				isFlexible = false
				title = "Lịch làm việc/học tập"
			case "full_week":
				days = []int{1, 2, 3, 4, 5, 6, 7}
				isBusy = true
				isFlexible = false
				title = "Lịch làm việc/học tập"
			case "night_shift":
				days = []int{1, 2, 3, 4, 5}
				isBusy = true
				isFlexible = false
				title = "Lịch làm việc/học tập"
			case "flexible":
				days = []int{1, 2, 3, 4, 5}
				isBusy = false
				isFlexible = true
				title = "Lịch làm việc/học tập (Linh hoạt)"
			}

			if len(days) > 0 {
				startTime := req.WorkStartTime
				endTime := req.WorkEndTime

				if startTime == "" {
					if req.WorkScheduleType == "night_shift" {
						startTime = "22:00"
					} else {
						startTime = "09:00"
					}
				}
				if endTime == "" {
					if req.WorkScheduleType == "night_shift" {
						endTime = "06:00"
					} else {
						endTime = "17:00"
					}
				}

				// Determine block type based on MainActivity
				blockType := models.ScheduleBlockTypeWork
				if req.MainActivity == "study" || req.MainActivity == "Học tập" || req.MainActivity == "student" {
					blockType = models.ScheduleBlockTypeStudy
				}

				daysJSON, err := json.Marshal(days)
				if err == nil {
					block := models.ScheduleBlock{
						UserID:     userID,
						Title:      title,
						Type:       blockType,
						DaysOfWeek: datatypes.JSON(daysJSON),
						StartTime:  startTime,
						EndTime:    endTime,
						IsBusy:     isBusy,
						IsFlexible: isFlexible,
						Enabled:    true,
					}
					if err := tx.Create(&block).Error; err != nil {
						logger.L.Error("failed to create initial schedule block from onboarding", zap.Error(err))
					}
				}
			}
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, nil, err
	}

	return &user, &onboarding, nil
}

func (s *OnboardingService) GetOnboardingStatus(userID uuid.UUID) (bool, string, error) {
	var user models.UserProfile
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return false, "", err
	}

	return user.HasCompletedOnboarding, user.DisplayName, nil
}
