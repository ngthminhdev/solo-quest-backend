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

var validReviewMoods = map[string]bool{
	"very_bad": true, "bad": true, "normal": true, "good": true, "very_good": true,
}

var validReviewEnergyLevels = map[string]bool{
	"low": true, "medium": true, "high": true,
}

var validReviewTomorrowPriorities = map[string]bool{
	"learning": true, "health": true, "work": true, "habit": true, "rest": true,
}

type DailyReviewService struct {
	db *gorm.DB
}

func NewDailyReviewService(db *gorm.DB) *DailyReviewService {
	return &DailyReviewService{db: db}
}

func (s *DailyReviewService) GetToday(userID uuid.UUID) (*dto.DailyReviewStatusResponse, error) {
	today := timeutil.TodayVN()
	return s.getByDate(userID, today)
}

func (s *DailyReviewService) GetByDate(userID uuid.UUID, dateStr string) (*dto.DailyReviewStatusResponse, error) {
	date, err := timeutil.ParseDateVN(dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}
	return s.getByDate(userID, date)
}

func (s *DailyReviewService) getByDate(userID uuid.UUID, date time.Time) (*dto.DailyReviewStatusResponse, error) {
	var review models.DailyReview
	start, end := timeutil.DayRangeVN(date)
	err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).First(&review).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &dto.DailyReviewStatusResponse{
			Item:        nil,
			HasReviewed: false,
			Date:        timeutil.FormatDateVN(date),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	resp := toReviewResponse(&review)
	return &dto.DailyReviewStatusResponse{
		Item:        &resp,
		HasReviewed: true,
		Date:        timeutil.FormatDateVN(date),
	}, nil
}

func (s *DailyReviewService) GetSummary(userID uuid.UUID, dateStr string) (*dto.DailyReviewSummaryResponse, error) {
	var date time.Time
	var err error
	if dateStr != "" {
		date, err = timeutil.ParseDateVN(dateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %w", err)
		}
	} else {
		date = timeutil.TodayVN()
	}

	return s.computeSummary(userID, date)
}

func (s *DailyReviewService) computeSummary(userID uuid.UUID, date time.Time) (*dto.DailyReviewSummaryResponse, error) {
	start, end := timeutil.DayRangeVN(date)

	var totalQuests int64
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Count(&totalQuests)

	var completedQuests int64
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, start, end, models.QuestStatusCompleted).
		Count(&completedQuests)

	var skippedQuests int64
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, start, end, models.QuestStatusSkipped).
		Count(&skippedQuests)

	var pendingQuests int64
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ? AND status IN ?", userID, start, end, []string{
			string(models.QuestStatusPending),
			string(models.QuestStatusActive),
			string(models.QuestStatusSnoozed),
		}).
		Count(&pendingQuests)

	var earnedExp int
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, start, end, models.QuestStatusCompleted).
		Select("COALESCE(SUM(xp_reward), 0)").
		Scan(&earnedExp)

	completionRate := 0.0
	if totalQuests > 0 {
		completionRate = float64(completedQuests) / float64(totalQuests)
	}

	var typeResults []struct {
		Type  string
		Count int
	}
	s.db.Model(&models.Quest{}).
		Select("type, count(*) as count").
		Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, start, end, models.QuestStatusCompleted).
		Group("type").
		Find(&typeResults)

	completedByType := make(map[string]int)
	for _, r := range typeResults {
		completedByType[r.Type] = r.Count
	}

	return &dto.DailyReviewSummaryResponse{
		Date:                timeutil.FormatDateVN(date),
		CompletedQuestCount: int(completedQuests),
		SkippedQuestCount:   int(skippedQuests),
		PendingQuestCount:   int(pendingQuests),
		TotalQuestCount:     int(totalQuests),
		EarnedExp:           earnedExp,
		CompletionRate:      completionRate,
		CompletedByType:     completedByType,
	}, nil
}

func (s *DailyReviewService) Save(userID uuid.UUID, req dto.SaveDailyReviewRequest) (*dto.DailyReviewResponse, *dto.DailyReviewSummaryResponse, error) {
	if !validReviewMoods[req.Mood] {
		return nil, nil, fmt.Errorf("invalid mood, must be very_bad, bad, normal, good, or very_good")
	}
	if !validReviewEnergyLevels[req.EnergyLevel] {
		return nil, nil, fmt.Errorf("invalid energy_level, must be low, medium, or high")
	}
	if req.Satisfaction < 1 || req.Satisfaction > 5 {
		return nil, nil, fmt.Errorf("satisfaction must be between 1 and 5")
	}
	if !validReviewTomorrowPriorities[req.TomorrowPriority] {
		return nil, nil, fmt.Errorf("invalid tomorrow_priority, must be learning, health, work, habit, or rest")
	}
	if len(req.Reflection) > 200 {
		return nil, nil, fmt.Errorf("reflection must be at most 200 characters")
	}

	now := timeutil.NowUTC()
	var date time.Time
	if req.Date != "" {
		var err error
		date, err = timeutil.ParseDateVN(req.Date)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid date format: %w", err)
		}
	} else {
		date = timeutil.TodayVN()
	}

	summary, err := s.computeSummary(userID, date)
	if err != nil {
		return nil, nil, err
	}

	var existing models.DailyReview
	start, end := timeutil.DayRangeVN(date)
	err = s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).First(&existing).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if isNew {
		review := models.DailyReview{
			UserID:              userID,
			Date:                date,
			Mood:                req.Mood,
			EnergyLevel:         req.EnergyLevel,
			Satisfaction:        req.Satisfaction,
			Reflection:          req.Reflection,
			TomorrowPriority:    req.TomorrowPriority,
			AISummary:           "Đã ghi nhận review hôm nay. AI summary sẽ được cập nhật sau.",
			CompletedQuestCount: summary.CompletedQuestCount,
			SkippedQuestCount:   summary.SkippedQuestCount,
			EarnedExp:           summary.EarnedExp,
			CompletionRate:      summary.CompletionRate,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := tx.Create(&review).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}

		logEntry := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeDailyReview,
			Title:     "Daily review",
			Content:   "Đã hoàn thành review cuối ngày",
			CreatedAt: now,
		}
		if err := tx.Create(&logEntry).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}

		if err := tx.Commit().Error; err != nil {
			return nil, nil, err
		}

		resp := toReviewResponse(&review)
		return &resp, summary, nil
	}

	if err != nil {
		return nil, nil, err
	}

	existing.Mood = req.Mood
	existing.EnergyLevel = req.EnergyLevel
	existing.Satisfaction = req.Satisfaction
	existing.Reflection = req.Reflection
	existing.TomorrowPriority = req.TomorrowPriority
	existing.AISummary = "Đã ghi nhận review hôm nay. AI summary sẽ được cập nhật sau."
	existing.CompletedQuestCount = summary.CompletedQuestCount
	existing.SkippedQuestCount = summary.SkippedQuestCount
	existing.EarnedExp = summary.EarnedExp
	existing.CompletionRate = summary.CompletionRate
	existing.UpdatedAt = now

	if err := tx.Save(&existing).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, nil, err
	}

	resp := toReviewResponse(&existing)
	return &resp, summary, nil
}

func toReviewResponse(r *models.DailyReview) dto.DailyReviewResponse {
	updatedAt := r.UpdatedAt
	return dto.DailyReviewResponse{
		ID:                  r.ID,
		UserID:              r.UserID,
		Date:                timeutil.FormatDateVN(r.Date),
		Mood:                r.Mood,
		EnergyLevel:         r.EnergyLevel,
		Satisfaction:        r.Satisfaction,
		Reflection:          r.Reflection,
		TomorrowPriority:    r.TomorrowPriority,
		AISummary:           r.AISummary,
		CompletedQuestCount: r.CompletedQuestCount,
		SkippedQuestCount:   r.SkippedQuestCount,
		EarnedExp:           r.EarnedExp,
		CompletionRate:      r.CompletionRate,
		CreatedAt:           r.CreatedAt.UTC(),
		UpdatedAt:           &updatedAt,
	}
}
