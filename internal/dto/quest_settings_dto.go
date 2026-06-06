package dto

type TimeRangeResponse struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type QuestRuleResponse struct {
	ID                string           `json:"id"`
	Type              string           `json:"type"`
	Title             string           `json:"title"`
	Description       string           `json:"description"`
	Enabled           bool             `json:"enabled"`
	Difficulty        string           `json:"difficulty"`
	MinIntervalMinutes *int            `json:"min_interval_minutes"`
	MaxPerDay         *int            `json:"max_per_day"`
	ActiveTimeRange   *TimeRangeResponse `json:"active_time_range"`
	ActiveWeekdays    []int            `json:"active_weekdays"`
	Priority          int              `json:"priority"`
	AdaptToEnergy     bool             `json:"adapt_to_energy"`
	AdaptToStress     bool             `json:"adapt_to_stress"`
	AdaptToSchedule   bool             `json:"adapt_to_schedule"`
}

type QuestSettingsResponse struct {
	DailyQuestCount   int                 `json:"daily_quest_count"`
	Difficulty        string              `json:"difficulty"`
	AutoAdjustEnabled bool                `json:"auto_adjust_enabled"`
	EnabledCategories []string            `json:"enabled_categories"`
	PreferredDuration string              `json:"preferred_duration"`
	RestDayEnabled    bool                `json:"rest_day_enabled"`
	Rules             []QuestRuleResponse `json:"rules"`
}

type TimeRangeRequest struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type UpdateQuestRuleRequest struct {
	ID                string            `json:"id"`
	Type              *string           `json:"type"`
	Title             *string           `json:"title"`
	Description       *string           `json:"description"`
	Enabled           *bool             `json:"enabled"`
	Difficulty        *string           `json:"difficulty"`
	MinIntervalMinutes *int             `json:"min_interval_minutes"`
	MaxPerDay         *int             `json:"max_per_day"`
	ActiveTimeRange   *TimeRangeRequest `json:"active_time_range"`
	ActiveWeekdays    *[]int            `json:"active_weekdays"`
	Priority          *int              `json:"priority"`
	AdaptToEnergy     *bool             `json:"adapt_to_energy"`
	AdaptToStress     *bool             `json:"adapt_to_stress"`
	AdaptToSchedule   *bool             `json:"adapt_to_schedule"`
}

type UpdateQuestSettingsRequest struct {
	DailyQuestCount   *int                      `json:"daily_quest_count"`
	Difficulty        *string                   `json:"difficulty"`
	AutoAdjustEnabled *bool                     `json:"auto_adjust_enabled"`
	EnabledCategories []string                  `json:"enabled_categories"`
	PreferredDuration *string                   `json:"preferred_duration"`
	RestDayEnabled    *bool                     `json:"rest_day_enabled"`
	Rules             []UpdateQuestRuleRequest  `json:"rules"`
}
