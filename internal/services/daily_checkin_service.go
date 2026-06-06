package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

var validCheckinMoods = map[string]bool{
	"very_bad": true, "bad": true, "normal": true, "good": true, "very_good": true,
}

var validEnergyLevels = map[string]bool{
	"low": true, "medium": true, "high": true,
}

var validAvailabilities = map[string]bool{
	"busy": true, "normal": true, "free": true,
}

var validPriorities = map[string]bool{
	"learning": true, "health": true, "work": true, "habit": true, "rest": true,
}

type DailyCheckinService struct {
	db *gorm.DB
}

func NewDailyCheckinService(db *gorm.DB) *DailyCheckinService {
	return &DailyCheckinService{db: db}
}

func (s *DailyCheckinService) GetToday(userID uuid.UUID) (*dto.DailyCheckinStatusResponse, error) {
	today := timeutil.TodayVN()
	return s.getByDate(userID, today)
}

func (s *DailyCheckinService) GetByDate(userID uuid.UUID, dateStr string) (*dto.DailyCheckinStatusResponse, error) {
	date, err := timeutil.ParseDateVN(dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}
	return s.getByDate(userID, date)
}

func (s *DailyCheckinService) getByDate(userID uuid.UUID, date time.Time) (*dto.DailyCheckinStatusResponse, error) {
	var checkin models.DailyCheckin
	start, end := timeutil.DayRangeVN(date)
	err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).First(&checkin).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &dto.DailyCheckinStatusResponse{
			Item:         nil,
			HasCheckedIn: false,
			Date:         timeutil.FormatDateVN(date),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	resp := toCheckinResponse(&checkin)
	return &dto.DailyCheckinStatusResponse{
		Item:         &resp,
		HasCheckedIn: true,
		Date:         timeutil.FormatDateVN(date),
	}, nil
}

func (s *DailyCheckinService) Save(userID uuid.UUID, req dto.SaveDailyCheckinRequest) (*dto.DailyCheckinResponse, error) {
	if !validCheckinMoods[req.Mood] {
		return nil, fmt.Errorf("invalid mood, must be very_bad, bad, normal, good, or very_good")
	}
	if !validEnergyLevels[req.EnergyLevel] {
		return nil, fmt.Errorf("invalid energy_level, must be low, medium, or high")
	}
	if !validAvailabilities[req.Availability] {
		return nil, fmt.Errorf("invalid availability, must be busy, normal, or free")
	}
	if !validPriorities[req.Priority] {
		return nil, fmt.Errorf("invalid priority, must be learning, health, work, habit, or rest")
	}

	now := timeutil.NowUTC()
	var date time.Time
	if req.Date != "" {
		var err error
		date, err = timeutil.ParseDateVN(req.Date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %w", err)
		}
	} else {
		date = timeutil.TodayVN()
	}

	var existing models.DailyCheckin
	start, end := timeutil.DayRangeVN(date)
	err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).First(&existing).Error

	isNew := errors.Is(err, gorm.ErrRecordNotFound)

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if isNew {
		checkin := models.DailyCheckin{
			UserID:      userID,
			Date:        date,
			Mood:        req.Mood,
			EnergyLevel: req.EnergyLevel,
			Availability: req.Availability,
			Priority:    req.Priority,
			CreatedAt:   now,
			UpdatedAt:   now,
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

	existing.Mood = req.Mood
	existing.EnergyLevel = req.EnergyLevel
	existing.Availability = req.Availability
	existing.Priority = req.Priority
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
	updatedAt := c.UpdatedAt
	return dto.DailyCheckinResponse{
		ID:           c.ID,
		UserID:       c.UserID,
		Date:         timeutil.FormatDateVN(c.Date),
		Mood:         c.Mood,
		EnergyLevel:  c.EnergyLevel,
		Availability: c.Availability,
		Priority:     c.Priority,
		CreatedAt:    c.CreatedAt.UTC(),
		UpdatedAt:    &updatedAt,
	}
}
