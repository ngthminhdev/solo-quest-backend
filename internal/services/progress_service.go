package services

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

type ProgressService struct {
	db *gorm.DB
}

func NewProgressService(db *gorm.DB) *ProgressService {
	return &ProgressService{db: db}
}

func (s *ProgressService) GetProgress(userID uuid.UUID) (*dto.ProgressResponse, error) {
	var profile models.UserProfile
	if err := s.db.Where("id = ?", userID).First(&profile).Error; err != nil {
		return nil, err
	}

	today := timeutil.TodayVN()
	todayStart, todayEnd := timeutil.DayRangeVN(today)

	// today stats
	var todayTotal int64
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, todayStart, todayEnd).
		Count(&todayTotal)

	var todayCompleted int64
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, todayStart, todayEnd, models.QuestStatusCompleted).
		Count(&todayCompleted)

	todayRate := 0.0
	if todayTotal > 0 {
		todayRate = float64(todayCompleted) / float64(todayTotal)
	}

	// week boundaries (VN Monday to Sunday)
	weekStart, weekEnd := timeutil.WeekRangeVN(today, time.Monday)

	// weekly stats
	var weekPlanned int64
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, weekStart, weekEnd).
		Count(&weekPlanned)

	var weekCompleted int64
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, weekStart, weekEnd, models.QuestStatusCompleted).
		Count(&weekCompleted)

	weeklyRate := 0.0
	if weekPlanned > 0 {
		weeklyRate = float64(weekCompleted) / float64(weekPlanned)
	}

	// completed by type
	var typeResults []struct {
		Type  string
		Count int
	}
	s.db.Model(&models.Quest{}).
		Select("type, count(*) as count").
		Where("user_id = ? AND status = ?", userID, models.QuestStatusCompleted).
		Group("type").
		Find(&typeResults)

	completedByType := make(map[string]int)
	for _, r := range typeResults {
		completedByType[r.Type] = r.Count
	}

	// weekly daily data
	weeklyData := buildWeeklyDailyData(s.db, userID, weekStart)

	return &dto.ProgressResponse{
		Level:                profile.Level,
		CurrentLevelExp:      profile.CurrentLevelExp,
		NextLevelExp:         profile.NextLevelExp,
		TotalExp:             profile.TotalExp,
		RewardPoints:         profile.RewardPoints,
		StreakDays:           profile.StreakDays,
		BestStreak:           profile.BestStreak,
		StreakShields:        profile.StreakShields,
		TotalCompletedQuests: profile.TotalCompletedQuests,
		TotalSkippedQuests:   profile.TotalSkippedQuests,
		TodayCompletedQuests: int(todayCompleted),
		TodayTotalQuests:     int(todayTotal),
		TodayCompletionRate:  todayRate,
		WeeklyCompletionRate: weeklyRate,
		CompletedByType:      completedByType,
		WeeklyDailyData:      weeklyData,
	}, nil
}

func (s *ProgressService) GetWeeklyChart(userID uuid.UUID) (*dto.WeeklyChartResponse, error) {
	today := timeutil.TodayVN()
	weekStart, weekEnd := timeutil.WeekRangeVN(today, time.Monday)

	weeklyData := buildWeeklyDailyData(s.db, userID, weekStart)

	return &dto.WeeklyChartResponse{
		WeekStart: timeutil.FormatDateVN(weekStart),
		WeekEnd:   timeutil.FormatDateVN(weekEnd.AddDate(0, 0, -1)),
		Items:     weeklyData,
	}, nil
}

func (s *ProgressService) GetXPHistory(userID uuid.UUID, filter dto.XPHistoryFilter) (*dto.XPHistoryResponse, error) {
	query := s.db.Model(&models.XPTransaction{}).Where("user_id = ?", userID)

	if filter.Currency != "" {
		query = query.Where("currency = ?", filter.Currency)
	}

	var total int64
	query.Count(&total)

	var items []models.XPTransaction
	err := query.Order("created_at DESC").
		Offset(filter.Offset).
		Limit(filter.Limit).
		Find(&items).Error
	if err != nil {
		return nil, err
	}

	result := make([]dto.XPHistoryItem, len(items))
	for i, item := range items {
		result[i] = dto.XPHistoryItem{
			ID:           item.ID,
			Amount:       item.Amount,
			Currency:     string(item.Currency),
			Source:       string(item.Source),
			ReferenceID:  item.ReferenceID,
			Description:  item.Description,
			BalanceAfter: item.BalanceAfter,
			CreatedAt:    item.CreatedAt.UTC().Format(time.RFC3339),
		}
	}

	return &dto.XPHistoryResponse{
		Items:  result,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

// getWeekStart is deprecated - use timeutil.StartOfWeekUTC instead
func getWeekStart(t time.Time) time.Time {
	return timeutil.StartOfWeekUTC(t, time.Monday)
}

var dayLabels = map[time.Weekday]string{
	time.Monday:    "T2",
	time.Tuesday:   "T3",
	time.Wednesday: "T4",
	time.Thursday:  "T5",
	time.Friday:    "T6",
	time.Saturday:  "T7",
	time.Sunday:    "CN",
}

func buildWeeklyDailyData(db *gorm.DB, userID uuid.UUID, weekStart time.Time) []dto.DailyData {
	result := make([]dto.DailyData, 7)

	for i := 0; i < 7; i++ {
		dayStart := weekStart.AddDate(0, 0, i)
		dayEnd := timeutil.EndExclusiveOfDayVN(dayStart)

		var planned int64
		db.Model(&models.Quest{}).
			Where("user_id = ? AND date >= ? AND date < ?", userID, dayStart, dayEnd).
			Count(&planned)

		var completed int64
		db.Model(&models.Quest{}).
			Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, dayStart, dayEnd, models.QuestStatusCompleted).
			Count(&completed)

		rate := 0.0
		if planned > 0 {
			rate = float64(completed) / float64(planned)
		}

		result[i] = dto.DailyData{
			Date:           timeutil.FormatDateVN(dayStart),
			DayLabel:       dayLabels[dayStart.Weekday()],
			Completed:      int(completed),
			Planned:        int(planned),
			CompletionRate: rate,
		}
	}

	return result
}
