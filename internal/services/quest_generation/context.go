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

	// Profile
	Age          int      `json:"age"`
	Height       float64  `json:"height"`
	Weight       float64  `json:"weight"`
	MainActivity string   `json:"main_activity"`
	MainGoals    []string `json:"main_goals"`

	// Schedule
	WorkScheduleType string                `json:"work_schedule_type"`
	WorkWeekdays     []int                 `json:"work_weekdays"`
	WorkStartTime    string                `json:"work_start_time"`
	WorkEndTime      string                `json:"work_end_time"`
	ScheduleBlocks   []ScheduleBlockDetail `json:"schedule_blocks"`

	WakeUpTime      string `json:"wake_up_time"`
	TargetSleepTime string `json:"target_sleep_time"`
	QuietAfterTime  string `json:"quiet_after_time"`
	FreeTimeStart   string `json:"free_time_start"`
	FreeTimeEnd     string `json:"free_time_end"`

	// Preferences
	PreferredFreeTimes      []string `json:"preferred_free_times"`
	LearningTimePreference  string   `json:"learning_time_preference"`
	LearningTimePreferences []string `json:"learning_time_preferences"`
	MovementTimePreference  string   `json:"movement_time_preference"`
	MovementTimePreferences []string `json:"movement_time_preferences"`
	SleepTimePreference     string   `json:"sleep_time_preference"`
	NutritionTimePreference string   `json:"nutrition_time_preference"`

	// Health
	ActivityLevel     string   `json:"activity_level"`
	LastWorkout       string   `json:"last_workout"`
	HealthLimitations []string `json:"health_limitations"`

	DailyQuestCount       int                      `json:"daily_quest_count"`
	PreviewLimit          int                      `json:"preview_limit,omitempty"`          // For AI preview generation only
	RequestedPreviewLimit *int                     `json:"requested_preview_limit,omitempty"` // For AI preview generation logging
	Difficulty            string                   `json:"difficulty"`
	PreferredDuration     string                   `json:"preferred_duration"`
	EnabledCategories     []string                 `json:"enabled_categories"`
	Rules                 []QuestRuleContext       `json:"rules"`
	ReminderSettings      []ReminderSettingContext `json:"reminder_settings"`

	ExistingQuestTitles []string `json:"existing_quest_titles"`

	// Runtime Context
	TodayCheckIn        *TodayCheckInDetail        `json:"today_check_in,omitempty"`
	PreviousDailyReview *PreviousDailyReviewDetail `json:"previous_daily_review,omitempty"`
	ActiveLearningPath  *ActiveLearningPathDetail  `json:"active_learning_path,omitempty"`
}

type ScheduleBlockDetail struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	DaysOfWeek []int  `json:"days_of_week"`
	IsBusy     bool   `json:"is_busy"`
}

type TodayCheckInDetail struct {
	Mood         string `json:"mood"`
	EnergyLevel  string `json:"energy_level"`
	Availability string `json:"availability"`
	Priority     string `json:"priority"`
}

type PreviousDailyReviewDetail struct {
	CompletionRate      float64 `json:"completion_rate"`
	CompletedQuestCount int     `json:"completed_quest_count"`
	SkippedQuestCount   int     `json:"skipped_quest_count"`
}

type ActiveLearningPathDetail struct {
	RoadmapTitle     string `json:"roadmap_title"`
	CurrentStepTitle string `json:"current_step_title"`
	Description      string `json:"description"`
}

type ReminderSettingContext struct {
	Type            string  `json:"type"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Frequency       string  `json:"frequency"`
	Status          string  `json:"status"`
	StartTime       *string `json:"start_time"`
	EndTime         *string `json:"end_time"`
	IntervalMinutes *int    `json:"interval_minutes"`
	MaxPerDay       *int    `json:"max_per_day"`
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
