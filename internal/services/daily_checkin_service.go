package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

var validEnergyLevels = map[string]bool{
	"veryLow": true, "low": true, "medium": true, "high": true, "veryHigh": true,
}

var validDayIntensities = map[string]bool{
	"light": true, "normal": true, "busy": true, "overloaded": true,
}

type DailyCheckinService struct {
	db *gorm.DB
}

func NewDailyCheckinService(db *gorm.DB) *DailyCheckinService {
	return &DailyCheckinService{db: db}
}

// TODO: support user-specific timezone in the future
func (s *DailyCheckinService) GetToday(userID uuid.UUID) (*dto.DailyCheckinStatusResponse, error) {
	today := timeutil.TodayUTC()
	return s.getByDate(userID, today)
}

func (s *DailyCheckinService) GetByDate(userID uuid.UUID, dateStr string) (*dto.DailyCheckinStatusResponse, error) {
	date, err := timeutil.ParseDateUTC(dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}
	return s.getByDate(userID, date)
}

func (s *DailyCheckinService) getByDate(userID uuid.UUID, date time.Time) (*dto.DailyCheckinStatusResponse, error) {
	var checkin models.DailyCheckin
	start, end := timeutil.DayRangeUTC(date)
	err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).First(&checkin).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &dto.DailyCheckinStatusResponse{
			Item:         nil,
			HasCheckedIn: false,
			Date:         timeutil.FormatDateUTC(date),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	resp := toCheckinResponse(&checkin)
	return &dto.DailyCheckinStatusResponse{
		Item:         &resp,
		HasCheckedIn: true,
		Date:         timeutil.FormatDateUTC(date),
	}, nil
}

func (s *DailyCheckinService) Save(userID uuid.UUID, req dto.SaveDailyCheckinRequest) (*dto.DailyCheckinResponse, error) {
	if !validEnergyLevels[req.EnergyLevel] {
		return nil, fmt.Errorf("invalid energy_level, must be veryLow, low, medium, high, or veryHigh")
	}
	if !validEnergyLevels[req.StressLevel] {
		return nil, fmt.Errorf("invalid stress_level, must be veryLow, low, medium, high, or veryHigh")
	}
	if !validEnergyLevels[req.FocusLevel] {
		return nil, fmt.Errorf("invalid focus_level, must be veryLow, low, medium, high, or veryHigh")
	}
	if !validDayIntensities[req.DayIntensity] {
		return nil, fmt.Errorf("invalid day_intensity, must be light, normal, busy, or overloaded")
	}

	now := timeutil.NowUTC()
	var date time.Time
	if req.Date != "" {
		var err error
		date, err = timeutil.ParseDateUTC(req.Date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %w", err)
		}
	} else {
		date = timeutil.TodayUTC()
	}

	blocksJSON, err := json.Marshal(req.AvailableTimeBlocks)
	if err != nil {
		blocksJSON = []byte("[]")
	}

	var existing models.DailyCheckin
	start, end := timeutil.DayRangeUTC(date)
	err = s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).First(&existing).Error

	isNew := errors.Is(err, gorm.ErrRecordNotFound)

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if isNew {
		checkin := models.DailyCheckin{
			UserID:              userID,
			Date:                date,
			EnergyLevel:         req.EnergyLevel,
			StressLevel:         req.StressLevel,
			FocusLevel:          req.FocusLevel,
			DayIntensity:        req.DayIntensity,
			MainFocusToday:      req.MainFocusToday,
			Note:                req.Note,
			AvailableTimeBlocks: blocksJSON,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := tx.Create(&checkin).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		logEntry := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeMorningCheckin,
			Title:     "Check-in buổi sáng",
			Content:   "Đã ghi nhận trạng thái đầu ngày",
			CreatedAt: now,
		}
		if err := tx.Create(&logEntry).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		if err := tx.Commit().Error; err != nil {
			return nil, err
		}

		resp := toCheckinResponse(&checkin)
		return &resp, nil
	}

	if err != nil {
		return nil, err
	}

	existing.EnergyLevel = req.EnergyLevel
	existing.StressLevel = req.StressLevel
	existing.FocusLevel = req.FocusLevel
	existing.DayIntensity = req.DayIntensity
	existing.MainFocusToday = req.MainFocusToday
	existing.Note = req.Note
	existing.AvailableTimeBlocks = blocksJSON
	existing.UpdatedAt = now

	if err := tx.Save(&existing).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	resp := toCheckinResponse(&existing)
	return &resp, nil
}

func toCheckinResponse(c *models.DailyCheckin) dto.DailyCheckinResponse {
	var blocks []string
	if c.AvailableTimeBlocks != nil {
		json.Unmarshal(c.AvailableTimeBlocks, &blocks)
	}

	updatedAt := c.UpdatedAt
	return dto.DailyCheckinResponse{
		ID:                  c.ID,
		UserID:              c.UserID,
		Date:                timeutil.FormatDateUTC(c.Date),
		EnergyLevel:         c.EnergyLevel,
		StressLevel:         c.StressLevel,
		FocusLevel:          c.FocusLevel,
		DayIntensity:        c.DayIntensity,
		MainFocusToday:      c.MainFocusToday,
		Note:                c.Note,
		AvailableTimeBlocks: blocks,
		CreatedAt:           c.CreatedAt.UTC(),
		UpdatedAt:           &updatedAt,
	}
}
