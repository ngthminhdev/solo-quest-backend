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

var validMoods = map[string]bool{
	"veryBad": true, "bad": true, "neutral": true, "good": true, "veryGood": true,
}

type DailyReviewService struct {
	db *gorm.DB
}

func NewDailyReviewService(db *gorm.DB) *DailyReviewService {
	return &DailyReviewService{db: db}
}

// TODO: support user-specific timezone in the future
func (s *DailyReviewService) GetToday(userID uuid.UUID) (*dto.DailyReviewStatusResponse, error) {
	today := timeutil.TodayUTC()
	return s.getByDate(userID, today)
}

func (s *DailyReviewService) GetByDate(userID uuid.UUID, dateStr string) (*dto.DailyReviewStatusResponse, error) {
	date, err := timeutil.ParseDateUTC(dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}
	return s.getByDate(userID, date)
}

func (s *DailyReviewService) getByDate(userID uuid.UUID, date time.Time) (*dto.DailyReviewStatusResponse, error) {
	var review models.DailyReview
	start, end := timeutil.DayRangeUTC(date)
	err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).First(&review).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &dto.DailyReviewStatusResponse{
			Item:        nil,
			HasReviewed: false,
			Date:        timeutil.FormatDateUTC(date),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	resp := toReviewResponse(&review)
	return &dto.DailyReviewStatusResponse{
		Item:        &resp,
		HasReviewed: true,
		Date:        timeutil.FormatDateUTC(date),
	}, nil
}

func (s *DailyReviewService) GetSummary(userID uuid.UUID, dateStr string) (*dto.DailyReviewSummaryResponse, error) {
	var date time.Time
	var err error
	if dateStr != "" {
		date, err = timeutil.ParseDateUTC(dateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %w", err)
		}
	} else {
		date = timeutil.TodayUTC()
	}

	return s.computeSummary(userID, date)
}

func (s *DailyReviewService) computeSummary(userID uuid.UUID, date time.Time) (*dto.DailyReviewSummaryResponse, error) {
	start, end := timeutil.DayRangeUTC(date)

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
		Date:                timeutil.FormatDateUTC(date),
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
	if !validMoods[req.Mood] {
		return nil, nil, fmt.Errorf("invalid mood, must be veryBad, bad, neutral, good, or veryGood")
	}
	if req.DifficultyRating != nil && (*req.DifficultyRating < 1 || *req.DifficultyRating > 5) {
		return nil, nil, fmt.Errorf("difficulty_rating must be between 1 and 5")
	}
	if req.EnergyLevel != nil && (*req.EnergyLevel < 1 || *req.EnergyLevel > 5) {
		return nil, nil, fmt.Errorf("energy_level must be between 1 and 5")
	}
	if req.SatisfactionLevel != nil && (*req.SatisfactionLevel < 1 || *req.SatisfactionLevel > 5) {
		return nil, nil, fmt.Errorf("satisfaction_level must be between 1 and 5")
	}

	now := timeutil.NowUTC()
	var date time.Time
	if req.Date != "" {
		var err error
		date, err = timeutil.ParseDateUTC(req.Date)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid date format: %w", err)
		}
	} else {
		date = timeutil.TodayUTC()
	}

	summary, err := s.computeSummary(userID, date)
	if err != nil {
		return nil, nil, err
	}

	helpfulJSON, _ := json.Marshal(req.HelpfulQuests)
	annoyingJSON, _ := json.Marshal(req.AnnoyingQuests)
	adjustmentsJSON, _ := json.Marshal(req.TomorrowAdjustments)

	diffRating := 0
	if req.DifficultyRating != nil {
		diffRating = *req.DifficultyRating
	}
	energyLvl := 0
	if req.EnergyLevel != nil {
		energyLvl = *req.EnergyLevel
	}
	satisfLvl := 0
	if req.SatisfactionLevel != nil {
		satisfLvl = *req.SatisfactionLevel
	}

	var existing models.DailyReview
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
		review := models.DailyReview{
			UserID:               userID,
			Date:                 date,
			Mood:                 req.Mood,
			DifficultyRating:     diffRating,
			EnergyLevel:          energyLvl,
			SatisfactionLevel:    satisfLvl,
			CompletedQuestCount:  summary.CompletedQuestCount,
			SkippedQuestCount:    summary.SkippedQuestCount,
			EarnedExp:            summary.EarnedExp,
			CompletionRate:       summary.CompletionRate,
			HelpfulQuests:        helpfulJSON,
			AnnoyingQuests:       annoyingJSON,
			BestMoment:           req.BestMoment,
			Challenge:            req.Challenge,
			ImprovementTomorrow:  req.ImprovementTomorrow,
			TomorrowAdjustments:  adjustmentsJSON,
			Note:                 req.Note,
			CreatedAt:            now,
			UpdatedAt:            now,
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
	existing.DifficultyRating = diffRating
	existing.EnergyLevel = energyLvl
	existing.SatisfactionLevel = satisfLvl
	existing.CompletedQuestCount = summary.CompletedQuestCount
	existing.SkippedQuestCount = summary.SkippedQuestCount
	existing.EarnedExp = summary.EarnedExp
	existing.CompletionRate = summary.CompletionRate
	existing.HelpfulQuests = helpfulJSON
	existing.AnnoyingQuests = annoyingJSON
	existing.BestMoment = req.BestMoment
	existing.Challenge = req.Challenge
	existing.ImprovementTomorrow = req.ImprovementTomorrow
	existing.TomorrowAdjustments = adjustmentsJSON
	existing.Note = req.Note
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
	var helpful, annoying, adjustments []string
	if r.HelpfulQuests != nil {
		json.Unmarshal(r.HelpfulQuests, &helpful)
	}
	if r.AnnoyingQuests != nil {
		json.Unmarshal(r.AnnoyingQuests, &annoying)
	}
	if r.TomorrowAdjustments != nil {
		json.Unmarshal(r.TomorrowAdjustments, &adjustments)
	}

	var diffRating, energyLvl, satisfLvl *int
	if r.DifficultyRating > 0 {
		v := r.DifficultyRating
		diffRating = &v
	}
	if r.EnergyLevel > 0 {
		v := r.EnergyLevel
		energyLvl = &v
	}
	if r.SatisfactionLevel > 0 {
		v := r.SatisfactionLevel
		satisfLvl = &v
	}

	updatedAt := r.UpdatedAt
	return dto.DailyReviewResponse{
		ID:                  r.ID,
		UserID:              r.UserID,
		Date:                timeutil.FormatDateUTC(r.Date),
		Mood:                r.Mood,
		DifficultyRating:    diffRating,
		EnergyLevel:         energyLvl,
		SatisfactionLevel:   satisfLvl,
		CompletedQuestCount: r.CompletedQuestCount,
		SkippedQuestCount:   r.SkippedQuestCount,
		EarnedExp:           r.EarnedExp,
		CompletionRate:      r.CompletionRate,
		HelpfulQuests:       helpful,
		AnnoyingQuests:      annoying,
		BestMoment:          r.BestMoment,
		Challenge:           r.Challenge,
		ImprovementTomorrow: r.ImprovementTomorrow,
		TomorrowAdjustments: adjustments,
		Note:                r.Note,
		CreatedAt:           r.CreatedAt.UTC(),
		UpdatedAt:           &updatedAt,
	}
}
