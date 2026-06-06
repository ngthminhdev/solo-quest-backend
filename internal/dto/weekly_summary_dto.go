package dto

type WeeklySummaryResponse struct {
	WeekStart          string                           `json:"week_start"`
	WeekEnd            string                           `json:"week_end"`
	CompletedQuestCount int                             `json:"completed_quest_count"`
	SkippedQuestCount  int                             `json:"skipped_quest_count"`
	EarnedExp          int                             `json:"earned_exp"`
	CompletionRate     float64                         `json:"completion_rate"`
	StreakDays         int                             `json:"streak_days"`
	ReviewedDays       int                             `json:"reviewed_days"`
	TotalDays          int                             `json:"total_days"`
	BestCategory       string                           `json:"best_category,omitempty"`
	WeakestCategory    string                           `json:"weakest_category,omitempty"`
	DailyBreakdown     []WeeklyDailyBreakdownResponse   `json:"daily_breakdown"`
	CategoryBreakdown  []WeeklyCategoryBreakdownResponse `json:"category_breakdown"`
	Insights           []string                         `json:"insights"`
	Suggestions        []WeeklySuggestionResponse       `json:"suggestions"`
	AISummary          string                           `json:"ai_summary,omitempty"`
	NextWeekFocus      string                           `json:"next_week_focus,omitempty"`
}

type WeeklyDailyBreakdownResponse struct {
	Date      string  `json:"date"`
	Completed int     `json:"completed"`
	Skipped   int     `json:"skipped"`
	Total     int     `json:"total"`
	Rate      float64 `json:"rate"`
}

type WeeklyCategoryBreakdownResponse struct {
	Category  string  `json:"category"`
	Completed int     `json:"completed"`
	Total     int     `json:"total"`
	Rate      float64 `json:"rate"`
}

type WeeklySuggestionResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Actionable  bool   `json:"actionable"`
}
