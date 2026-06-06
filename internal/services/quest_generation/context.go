package quest_generation

import (
	"time"

	"github.com/google/uuid"
)

type UserQuestContext struct {
	UserID    uuid.UUID `json:"user_id"`
	LocalDate time.Time `json:"local_date"`
	Timezone  string    `json:"timezone"`

	DisplayName string `json:"display_name"`

	MainActivity      string   `json:"main_activity"`
	MainGoals         []string `json:"main_goals"`
	HealthLimitations []string `json:"health_limitations"`

	WorkScheduleType string `json:"work_schedule_type"`
	WorkStartTime    string `json:"work_start_time"`
	WorkEndTime      string `json:"work_end_time"`

	WakeUpTime      string `json:"wake_up_time"`
	TargetSleepTime string `json:"target_sleep_time"`
	QuietAfterTime  string `json:"quiet_after_time"`
	FreeTimeStart   string `json:"free_time_start"`
	FreeTimeEnd     string `json:"free_time_end"`

	LearningTimePreference  string   `json:"learning_time_preference"`
	LearningTimePreferences []string `json:"learning_time_preferences"`
	MovementTimePreference  string   `json:"movement_time_preference"`
	MovementTimePreferences []string `json:"movement_time_preferences"`

	DailyQuestCount       int                 `json:"daily_quest_count"`
	PreviewLimit          int                 `json:"preview_limit,omitempty"`          // For AI preview generation only
	RequestedPreviewLimit *int                `json:"requested_preview_limit,omitempty"` // For AI preview generation logging
	Difficulty            string              `json:"difficulty"`
	PreferredDuration     string              `json:"preferred_duration"`
	EnabledCategories     []string            `json:"enabled_categories"`
	Rules                 []QuestRuleContext  `json:"rules"`

	ExistingQuestTitles []string `json:"existing_quest_titles"`
}

type QuestRuleContext struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Enabled            bool              `json:"enabled"`
	Difficulty         string            `json:"difficulty"`
	Priority           int               `json:"priority"`
	MinIntervalMinutes *int              `json:"min_interval_minutes"`
	MaxPerDay          *int              `json:"max_per_day"`
	ActiveTimeRange    *TimeRangeContext `json:"active_time_range"`
	Weekdays           []int             `json:"weekdays"`
	AdaptToEnergy      bool              `json:"adapt_to_energy"`
	AdaptToStress      bool              `json:"adapt_to_stress"`
	AdaptToSchedule    bool              `json:"adapt_to_schedule"`
}

type TimeRangeContext struct {
	Start string `json:"start"`
	End   string `json:"end"`
}
