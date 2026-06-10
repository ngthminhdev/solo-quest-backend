package dto

import "github.com/google/uuid"

type DailyData struct {
	Date           string  `json:"date"`
	DayLabel       string  `json:"day_label"`
	Completed      int     `json:"completed"`
	Planned        int     `json:"planned"`
	CompletionRate float64 `json:"completion_rate"`
}

type ProgressResponse struct {
	Level                int                `json:"level"`
	CurrentLevelExp      int                `json:"current_level_exp"`
	NextLevelExp         int                `json:"next_level_exp"`
	TotalExp             int                `json:"total_exp"`
	RewardPoints         int                `json:"reward_points"`
	StreakDays           int                `json:"streak_days"`
	BestStreak           int                `json:"best_streak"`
	StreakShields        int                `json:"streak_shields"`
	TotalCompletedQuests int                `json:"total_completed_quests"`
	TotalSkippedQuests   int                `json:"total_skipped_quests"`
	TodayCompletedQuests int                `json:"today_completed_quests"`
	TodayTotalQuests     int                `json:"today_total_quests"`
	TodayCompletionRate  float64            `json:"today_completion_rate"`
	TodayEarnedExp       int                `json:"today_earned_exp"`
	WeeklyCompletionRate float64            `json:"weekly_completion_rate"`
	CompletedByType      map[string]int     `json:"completed_by_type"`
	WeeklyDailyData      []DailyData        `json:"weekly_daily_data"`
}

type WeeklyChartResponse struct {
	WeekStart string      `json:"week_start"`
	WeekEnd   string      `json:"week_end"`
	Items     []DailyData `json:"items"`
}

type XPHistoryItem struct {
	ID           uuid.UUID `json:"id"`
	Amount       int       `json:"amount"`
	Currency     string    `json:"currency"`
	Source       string    `json:"source"`
	ReferenceID  *uuid.UUID `json:"reference_id"`
	Description  string    `json:"description"`
	BalanceAfter int       `json:"balance_after"`
	CreatedAt    string    `json:"created_at"`
}

type XPHistoryFilter struct {
	Currency string
	Limit    int
	Offset   int
}

type XPHistoryResponse struct {
	Items      []XPHistoryItem `json:"items"`
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
	TotalCount int64           `json:"total_count"`
}
