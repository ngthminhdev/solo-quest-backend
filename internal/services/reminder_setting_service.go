package services

import (
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/pkg/logger"
)

var (
	ErrInvalidReminderType   = errors.New("invalid reminder type")
	ErrInvalidFrequency      = errors.New("invalid frequency")
	ErrInvalidReminderStatus = errors.New("invalid status, must be enabled or disabled")
	ErrInvalidTimeFormat     = errors.New("invalid time format, expected HH:MM")
	ErrInvalidInterval       = errors.New("interval_minutes must be positive")
	ErrInvalidMaxPerDay      = errors.New("max_per_day must be positive")
)

func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }

type defaultReminderSetting struct {
	Type            string
	Title           string
	Description     string
	Frequency       string
	Status          string
	StartTime       *string
	EndTime         *string
	IntervalMinutes *int
	MaxPerDay       *int
	SmartEnabled    bool
}

var defaultReminderSettings = []defaultReminderSetting{
	{
		Type:            "water",
		Title:           "Uống nước",
		Description:     "Nhắc uống nước nhỏ giọt thay vì mục tiêu lớn.",
		Frequency:       "interval",
		Status:          "enabled",
		StartTime:       strPtr("08:00"),
		EndTime:         strPtr("22:00"),
		IntervalMinutes: intPtr(90),
		MaxPerDay:       intPtr(8),
		SmartEnabled:    false,
	},
	{
		Type:            "break_time",
		Title:           "Nghỉ mắt & nghỉ giải lao",
		Description:     "Nhắc nghỉ sau thời gian tập trung.",
		Frequency:       "interval",
		Status:          "enabled",
		StartTime:       strPtr("09:00"),
		EndTime:         strPtr("18:00"),
		IntervalMinutes: intPtr(90),
		MaxPerDay:       nil,
		SmartEnabled:    false,
	},
	{
		Type:         "movement",
		Title:        "Vận động nhẹ",
		Description:  "Nhắc đứng dậy và vận động nhẹ trong ngày.",
		Frequency:    "random_in_range",
		Status:       "enabled",
		StartTime:    strPtr("10:00"),
		EndTime:      strPtr("17:00"),
		MaxPerDay:    intPtr(3),
		SmartEnabled: false,
	},
	{
		Type:         "learning",
		Title:        "Học tập",
		Description:  "Nhắc bạn dành thời gian học vào buổi tối.",
		Frequency:    "fixed",
		Status:       "enabled",
		StartTime:    strPtr("20:00"),
		SmartEnabled: false,
	},
	{
		Type:         "sleep",
		Title:        "Chuẩn bị ngủ",
		Description:  "Nhắc bạn chuẩn bị ngủ đúng giờ.",
		Frequency:    "fixed",
		Status:       "enabled",
		StartTime:    strPtr("22:30"),
		SmartEnabled: false,
	},
	{
		Type:         "daily_review",
		Title:        "Tổng kết ngày",
		Description:  "Nhắc bạn nhìn lại ngày hôm nay.",
		Frequency:    "fixed",
		Status:       "enabled",
		StartTime:    strPtr("21:30"),
		SmartEnabled: false,
	},
	{
		Type:         "custom",
		Title:        "Tùy chỉnh",
		Description:  "Nhắc nhở cá nhân do bạn cấu hình.",
		Frequency:    "fixed",
		Status:       "disabled",
		SmartEnabled: false,
	},
}

type ReminderSettingService struct {
	db *gorm.DB
}

func NewReminderSettingService(db *gorm.DB) *ReminderSettingService {
	return &ReminderSettingService{db: db}
}

func (s *ReminderSettingService) EnsureDefaultReminderSettingsForUser(userID uuid.UUID) error {
	for _, def := range defaultReminderSettings {
		var existing models.ReminderSetting
		err := s.db.Where("user_id = ? AND type = ?", userID, def.Type).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		setting := models.ReminderSetting{
			UserID:          userID,
			Type:            models.ReminderType(def.Type),
			Title:           def.Title,
			Description:     def.Description,
			Frequency:       models.ReminderFrequency(def.Frequency),
			Status:          models.ReminderStatus(def.Status),
			StartTime:       def.StartTime,
			EndTime:         def.EndTime,
			IntervalMinutes: def.IntervalMinutes,
			MaxPerDay:       def.MaxPerDay,
			SmartEnabled:    def.SmartEnabled,
		}

		if createErr := s.db.Create(&setting).Error; createErr != nil {
			logger.L.Error("failed to seed reminder setting",
				zap.String("type", def.Type),
				zap.String("user_id", userID.String()),
				zap.Error(createErr),
			)
			return createErr
		}
	}

	logger.L.Info("default reminder settings seeded",
		zap.String("user_id", userID.String()),
	)
	return nil
}

func (s *ReminderSettingService) GetReminderSettingsByUserID(userID uuid.UUID) ([]dto.ReminderSettingResponse, error) {
	var count int64
	s.db.Model(&models.ReminderSetting{}).Where("user_id = ?", userID).Count(&count)
	if count == 0 {
		if err := s.EnsureDefaultReminderSettingsForUser(userID); err != nil {
			return nil, err
		}
	}

	var settings []models.ReminderSetting
	err := s.db.Where("user_id = ?", userID).Order("type ASC").Find(&settings).Error
	if err != nil {
		return nil, err
	}

	responses := make([]dto.ReminderSettingResponse, len(settings))
	for i, setting := range settings {
		responses[i] = toReminderSettingResponse(setting)
	}
	return responses, nil
}

func (s *ReminderSettingService) GetReminderSettingByType(userID uuid.UUID, reminderType string) (*models.ReminderSetting, error) {
	var setting models.ReminderSetting
	err := s.db.Where("user_id = ? AND type = ?", userID, reminderType).First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func (s *ReminderSettingService) GetOrCreateReminderSetting(userID uuid.UUID, reminderType string) (*models.ReminderSetting, error) {
	setting, err := s.GetReminderSettingByType(userID, reminderType)
	if err == nil {
		return setting, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	setting = &models.ReminderSetting{
		UserID:       userID,
		Type:         models.ReminderType(reminderType),
		Title:        defaultTitleForType(reminderType),
		Description:  defaultDescriptionForType(reminderType),
		Frequency:    models.ReminderFrequencyFixed,
		Status:       models.ReminderStatusEnabled,
		SmartEnabled: false,
	}
	if createErr := s.db.Create(setting).Error; createErr != nil {
		return nil, createErr
	}
	return setting, nil
}

func (s *ReminderSettingService) UpdateReminderSetting(userID uuid.UUID, reminderType string, req *dto.UpdateReminderSettingRequest) (*dto.ReminderSettingResponse, error) {
	if !models.IsValidReminderType(reminderType) {
		return nil, ErrInvalidReminderType
	}

	if req.Frequency != nil && !models.IsValidReminderFrequency(*req.Frequency) {
		return nil, ErrInvalidFrequency
	}

	if req.Status != nil && !models.IsValidReminderStatus(*req.Status) {
		return nil, ErrInvalidReminderStatus
	}

	if req.IntervalMinutes != nil && *req.IntervalMinutes <= 0 {
		return nil, ErrInvalidInterval
	}

	if req.MaxPerDay != nil && *req.MaxPerDay <= 0 {
		return nil, ErrInvalidMaxPerDay
	}

	setting, err := s.GetOrCreateReminderSetting(userID, reminderType)
	if err != nil {
		return nil, err
	}

	if req.Frequency != nil {
		setting.Frequency = models.ReminderFrequency(*req.Frequency)
	}
	if req.Status != nil {
		setting.Status = models.ReminderStatus(*req.Status)
	}
	if req.StartTime != nil {
		setting.StartTime = req.StartTime
	}
	if req.EndTime != nil {
		setting.EndTime = req.EndTime
	}
	if req.IntervalMinutes != nil {
		setting.IntervalMinutes = req.IntervalMinutes
	}
	if req.MaxPerDay != nil {
		setting.MaxPerDay = req.MaxPerDay
	}
	if req.SmartEnabled != nil {
		setting.SmartEnabled = *req.SmartEnabled
	}

	if err := s.db.Save(setting).Error; err != nil {
		return nil, err
	}

	resp := toReminderSettingResponse(*setting)
	return &resp, nil
}

func (s *ReminderSettingService) ToggleReminderSetting(userID uuid.UUID, reminderType string, status string) (*dto.ReminderSettingResponse, error) {
	if !models.IsValidReminderType(reminderType) {
		return nil, ErrInvalidReminderType
	}

	if !models.IsValidReminderStatus(status) {
		return nil, ErrInvalidReminderStatus
	}

	setting, err := s.GetOrCreateReminderSetting(userID, reminderType)
	if err != nil {
		return nil, err
	}

	setting.Status = models.ReminderStatus(status)

	if err := s.db.Save(setting).Error; err != nil {
		return nil, err
	}

	resp := toReminderSettingResponse(*setting)
	return &resp, nil
}

func toReminderSettingResponse(s models.ReminderSetting) dto.ReminderSettingResponse {
	return dto.ReminderSettingResponse{
		ID:              s.ID,
		Type:            string(s.Type),
		Title:           s.Title,
		Description:     s.Description,
		Frequency:       string(s.Frequency),
		Status:          string(s.Status),
		StartTime:       s.StartTime,
		EndTime:         s.EndTime,
		IntervalMinutes: s.IntervalMinutes,
		MaxPerDay:       s.MaxPerDay,
		SmartEnabled:    s.SmartEnabled,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

func defaultTitleForType(t string) string {
	for _, def := range defaultReminderSettings {
		if def.Type == t {
			return def.Title
		}
	}
	return t
}

func defaultDescriptionForType(t string) string {
	for _, def := range defaultReminderSettings {
		if def.Type == t {
			return def.Description
		}
	}
	return ""
}
