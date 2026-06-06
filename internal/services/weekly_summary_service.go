package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

type WeeklySummaryService struct {
	db *gorm.DB
}

func NewWeeklySummaryService(db *gorm.DB) *WeeklySummaryService {
	return &WeeklySummaryService{db: db}
}

type weeklyStats struct {
	completedQuestCount int
	skippedQuestCount   int
	totalQuestCount     int
	earnedExp           int
	dailyBreakdown      []dto.WeeklyDailyBreakdownResponse
	categoryBreakdown   []dto.WeeklyCategoryBreakdownResponse
	reviewedDays        int
	streakDays          int
	bestCategory        string
	weakestCategory     string
}

func (s *WeeklySummaryService) GetCurrentWeekSummary(userID uuid.UUID) (*dto.WeeklySummaryResponse, error) {
	today := timeutil.TodayVN()
	weekStart := timeutil.StartOfWeekVN(today, time.Monday)
	weekEnd := weekStart.AddDate(0, 0, 6)

	return s.buildSummary(userID, weekStart, weekEnd, today)
}

func (s *WeeklySummaryService) GetWeekSummary(userID uuid.UUID, weekStartStr string) (*dto.WeeklySummaryResponse, error) {
	weekStart, err := timeutil.ParseDateVN(weekStartStr)
	if err != nil {
		return nil, fmt.Errorf("invalid week_start format, use YYYY-MM-DD: %w", err)
	}

	weekStart = timeutil.StartOfWeekVN(weekStart, time.Monday)
	weekEnd := weekStart.AddDate(0, 0, 6)

	return s.buildSummary(userID, weekStart, weekEnd, weekEnd)
}

func (s *WeeklySummaryService) buildSummary(userID uuid.UUID, weekStart time.Time, weekEnd time.Time, streakEndDate time.Time) (*dto.WeeklySummaryResponse, error) {
	rangeStart, rangeEnd := timeutil.DayRangeVN(weekStart)
	_, rangeEndLast := timeutil.DayRangeVN(weekEnd)
	rangeEnd = rangeEndLast

	stats := s.computeStats(userID, rangeStart, rangeEnd, weekStart, streakEndDate)
	completionRate := 0.0
	if stats.totalQuestCount > 0 {
		completionRate = float64(stats.completedQuestCount) / float64(stats.totalQuestCount)
	}

	insights := generateInsights(stats, completionRate)
	suggestions := generateSuggestions(stats, completionRate)
	aiSummary, nextWeekFocus := generateAIPlaceholders(stats, completionRate)

	return &dto.WeeklySummaryResponse{
		WeekStart:          timeutil.FormatDateVN(weekStart),
		WeekEnd:            timeutil.FormatDateVN(weekEnd),
		CompletedQuestCount: stats.completedQuestCount,
		SkippedQuestCount:  stats.skippedQuestCount,
		EarnedExp:          stats.earnedExp,
		CompletionRate:     completionRate,
		StreakDays:         stats.streakDays,
		ReviewedDays:       stats.reviewedDays,
		TotalDays:          7,
		BestCategory:       stats.bestCategory,
		WeakestCategory:    stats.weakestCategory,
		DailyBreakdown:     stats.dailyBreakdown,
		CategoryBreakdown:  stats.categoryBreakdown,
		Insights:           insights,
		Suggestions:        suggestions,
		AISummary:          aiSummary,
		NextWeekFocus:      nextWeekFocus,
	}, nil
}

func (s *WeeklySummaryService) computeStats(userID uuid.UUID, rangeStart time.Time, rangeEnd time.Time, weekStart time.Time, streakEndDate time.Time) weeklyStats {
	stats := weeklyStats{}

	stats.dailyBreakdown = s.computeDailyBreakdown(userID, weekStart)
	for _, d := range stats.dailyBreakdown {
		stats.completedQuestCount += d.Completed
		stats.skippedQuestCount += d.Skipped
		stats.totalQuestCount += d.Total
	}

	stats.earnedExp = s.computeEarnedExp(userID, rangeStart, rangeEnd)
	stats.categoryBreakdown = s.computeCategoryBreakdown(userID, rangeStart, rangeEnd)
	stats.reviewedDays = s.computeReviewedDays(userID, rangeStart, rangeEnd)
	stats.streakDays = s.computeStreakDays(userID, streakEndDate)
	stats.bestCategory, stats.weakestCategory = computeBestWeakestCategory(stats.categoryBreakdown)

	return stats
}

func (s *WeeklySummaryService) computeDailyBreakdown(userID uuid.UUID, weekStart time.Time) []dto.WeeklyDailyBreakdownResponse {
	result := make([]dto.WeeklyDailyBreakdownResponse, 7)

	for i := 0; i < 7; i++ {
		dayStart := weekStart.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)

		var total int64
		s.db.Model(&models.Quest{}).
			Where("user_id = ? AND date >= ? AND date < ?", userID, dayStart, dayEnd).
			Count(&total)

		var completed int64
		s.db.Model(&models.Quest{}).
			Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, dayStart, dayEnd, models.QuestStatusCompleted).
			Count(&completed)

		var skipped int64
		s.db.Model(&models.Quest{}).
			Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, dayStart, dayEnd, models.QuestStatusSkipped).
			Count(&skipped)

		rate := 0.0
		if total > 0 {
			rate = float64(completed) / float64(total)
		}

		result[i] = dto.WeeklyDailyBreakdownResponse{
			Date:      timeutil.FormatDateVN(dayStart),
			Completed: int(completed),
			Skipped:   int(skipped),
			Total:     int(total),
			Rate:      rate,
		}
	}

	return result
}

func (s *WeeklySummaryService) computeEarnedExp(userID uuid.UUID, rangeStart time.Time, rangeEnd time.Time) int {
	var earnedExp int
	s.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, rangeStart, rangeEnd, models.QuestStatusCompleted).
		Select("COALESCE(SUM(xp_reward), 0)").
		Scan(&earnedExp)
	return earnedExp
}

func (s *WeeklySummaryService) computeCategoryBreakdown(userID uuid.UUID, rangeStart time.Time, rangeEnd time.Time) []dto.WeeklyCategoryBreakdownResponse {
	var typeResults []struct {
		Type  string
		Count int
	}
	s.db.Model(&models.Quest{}).
		Select("type, count(*) as count").
		Where("user_id = ? AND date >= ? AND date < ?", userID, rangeStart, rangeEnd).
		Group("type").
		Find(&typeResults)

	totalByType := make(map[string]int)
	for _, r := range typeResults {
		totalByType[r.Type] = r.Count
	}

	var completedResults []struct {
		Type  string
		Count int
	}
	s.db.Model(&models.Quest{}).
		Select("type, count(*) as count").
		Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, rangeStart, rangeEnd, models.QuestStatusCompleted).
		Group("type").
		Find(&completedResults)

	completedByType := make(map[string]int)
	for _, r := range completedResults {
		completedByType[r.Type] = r.Count
	}

	var result []dto.WeeklyCategoryBreakdownResponse
	for internalType, total := range totalByType {
		if total == 0 {
			continue
		}
		completed := completedByType[internalType]
		rate := 0.0
		if total > 0 {
			rate = float64(completed) / float64(total)
		}
		result = append(result, dto.WeeklyCategoryBreakdownResponse{
			Category:  mapCategoryToFE(internalType),
			Completed: completed,
			Total:     total,
			Rate:      rate,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Category < result[j].Category
	})

	return result
}

func (s *WeeklySummaryService) computeReviewedDays(userID uuid.UUID, rangeStart time.Time, rangeEnd time.Time) int {
	var count int64
	s.db.Model(&models.DailyReview{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, rangeStart, rangeEnd).
		Count(&count)
	return int(count)
}

func (s *WeeklySummaryService) computeStreakDays(userID uuid.UUID, streakEndDate time.Time) int {
	streak := 0

	for i := 0; i < 366; i++ {
		checkDate := streakEndDate.AddDate(0, 0, -i)
		dayStart, dayEnd := timeutil.DayRangeVN(checkDate)

		var count int64
		s.db.Model(&models.Quest{}).
			Where("user_id = ? AND date >= ? AND date < ? AND status = ?", userID, dayStart, dayEnd, models.QuestStatusCompleted).
			Count(&count)

		if count > 0 {
			streak++
		} else {
			break
		}
	}

	return streak
}

func computeBestWeakestCategory(breakdown []dto.WeeklyCategoryBreakdownResponse) (string, string) {
	if len(breakdown) == 0 {
		return "", ""
	}

	type candidate struct {
		category string
		rate     float64
		total    int
	}

	var candidates []candidate
	for _, b := range breakdown {
		if b.Total > 0 {
			candidates = append(candidates, candidate{b.Category, b.Rate, b.Total})
		}
	}

	if len(candidates) == 0 {
		return "", ""
	}

	best := candidates[0]
	weakest := candidates[0]
	for _, c := range candidates[1:] {
		if c.rate > best.rate || (c.rate == best.rate && c.total > best.total) || (c.rate == best.rate && c.total == best.total && c.category < best.category) {
			best = c
		}
		if c.rate < weakest.rate || (c.rate == weakest.rate && c.total > weakest.total) || (c.rate == weakest.rate && c.total == weakest.total && c.category < weakest.category) {
			weakest = c
		}
	}

	return best.category, weakest.category
}

func mapCategoryToFE(internalType string) string {
	switch internalType {
	case "water":
		return "water"
	case "learning":
		return "learning"
	case "breakTime":
		return "breakTime"
	case "movement":
		return "movement"
	case "sleep":
		return "sleep"
	case "review":
		return "review"
	case "reflection":
		return "mindfulness"
	default:
		return internalType
	}
}

func generateInsights(stats weeklyStats, completionRate float64) []string {
	var insights []string

	if completionRate >= 0.8 {
		insights = append(insights, "Bạn duy trì tiến độ rất tốt trong tuần này.")
	} else if completionRate >= 0.5 {
		insights = append(insights, "Tuần này bạn hoàn thành ở mức ổn định, vẫn còn chỗ để cải thiện.")
	} else if stats.totalQuestCount > 0 {
		insights = append(insights, "Tuần này có nhiều quest bị bỏ qua, nên giảm độ khó hoặc số lượng quest.")
	}

	if stats.reviewedDays < 3 {
		insights = append(insights, "Bạn review cuối ngày chưa đều. Review ngắn giúp AI điều chỉnh quest tốt hơn.")
	}

	if stats.weakestCategory != "" && len(stats.categoryBreakdown) > 1 {
		insights = append(insights, fmt.Sprintf("Nhóm %s cần được điều chỉnh nhẹ hơn trong tuần tới.", stats.weakestCategory))
	}

	if stats.bestCategory != "" && stats.bestCategory != stats.weakestCategory {
		insights = append(insights, fmt.Sprintf("Bạn hoàn thành tốt các quest nhóm %s trong tuần này.", stats.bestCategory))
	}

	if len(insights) == 0 {
		insights = append(insights, "Hãy bắt đầu hoàn thành quest để nhận phân tích chi tiết.")
	}

	return insights
}

func generateSuggestions(stats weeklyStats, completionRate float64) []dto.WeeklySuggestionResponse {
	var suggestions []dto.WeeklySuggestionResponse
	sugID := 0

	if completionRate < 0.5 && stats.totalQuestCount > 0 {
		sugID++
		suggestions = append(suggestions, dto.WeeklySuggestionResponse{
			ID:          fmt.Sprintf("suggestion-%d", sugID),
			Title:       "Giảm số lượng quest",
			Description: "Tuần này tỉ lệ hoàn thành thấp. Hãy giảm bớt quest để dễ duy trì hơn.",
			Type:        "frequency",
			Actionable:  true,
		})
	}

	if stats.weakestCategory != "" && len(stats.categoryBreakdown) > 1 {
		for _, cb := range stats.categoryBreakdown {
			if cb.Category == stats.weakestCategory && cb.Rate < 0.5 && cb.Total > 0 {
				sugID++
				suggestions = append(suggestions, dto.WeeklySuggestionResponse{
					ID:          fmt.Sprintf("suggestion-%d", sugID),
					Title:       fmt.Sprintf("Làm nhẹ nhóm %s", stats.weakestCategory),
					Description: fmt.Sprintf("Nhóm này có tỉ lệ hoàn thành thấp, nên giảm thời lượng hoặc độ khó."),
					Type:        "duration",
					Actionable:  true,
				})
				break
			}
		}
	}

	if stats.reviewedDays < 3 {
		sugID++
		suggestions = append(suggestions, dto.WeeklySuggestionResponse{
			ID:          fmt.Sprintf("suggestion-%d", sugID),
			Title:       "Giữ review cuối ngày",
			Description: "Review ngắn mỗi tối giúp AI hiểu ngày nào quá tải.",
			Type:        "general",
			Actionable:  false,
		})
	}

	if stats.totalQuestCount == 0 {
		sugID++
		suggestions = append(suggestions, dto.WeeklySuggestionResponse{
			ID:          fmt.Sprintf("suggestion-%d", sugID),
			Title:       "Bắt đầu với quest đơn giản",
			Description: "Bạn chưa có quest nào trong tuần. Hãy thêm vài quest nhẹ để tạo thói quen.",
			Type:        "general",
			Actionable:  true,
		})
	}

	if completionRate >= 0.5 && completionRate < 0.8 && stats.totalQuestCount > 0 && sugID < 2 {
		sugID++
		suggestions = append(suggestions, dto.WeeklySuggestionResponse{
			ID:          fmt.Sprintf("suggestion-%d", sugID),
			Title:       "Duy trì đà tiến bộ",
			Description: "Bạn đang làm tốt. Hãy duy trì lịch quest và review đều đặn để cải thiện hơn nữa.",
			Type:        "general",
			Actionable:  false,
		})
	}

	return suggestions
}

func generateAIPlaceholders(stats weeklyStats, completionRate float64) (string, string) {
	aiSummary := ""
	nextWeekFocus := ""

	if stats.totalQuestCount > 0 {
		if completionRate >= 0.7 {
			aiSummary = "Tuần này bạn duy trì tiến độ tốt. Tiếp tục phát huy nhé!"
		} else if completionRate >= 0.4 {
			aiSummary = "Tuần này bạn hoàn thành ở mức trung bình. Vẫn còn nhiều cơ hội để cải thiện."
		} else {
			aiSummary = "Tuần này có vẻ hơi khó khăn. Hãy thử điều chỉnh lại kế hoạch tuần tới."
		}

		if stats.bestCategory != "" {
			aiSummary += fmt.Sprintf(" Nhóm %s là điểm mạnh của bạn.", stats.bestCategory)
		}
		if stats.weakestCategory != "" && stats.weakestCategory != stats.bestCategory {
			aiSummary += fmt.Sprintf(" Nhóm %s cần chú ý hơn.", stats.weakestCategory)
		}
	} else {
		aiSummary = "Bạn chưa có hoạt động nào trong tuần này. Hãy bắt đầu với vài quest nhẹ nhàng."
	}

	if stats.weakestCategory != "" {
		nextWeekFocus = stats.weakestCategory
	} else if stats.bestCategory != "" {
		nextWeekFocus = stats.bestCategory
	}

	return aiSummary, nextWeekFocus
}
