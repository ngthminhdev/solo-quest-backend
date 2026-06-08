package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type UserProfile struct {
	// ── Primary key ──────────────────────────────────────────────────────────
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	// ── Identity / profile ──────────────────────────────────────────────────
	DisplayName  string   `gorm:"type:varchar(100);not null" json:"display_name"`
	AvatarURL    *string  `gorm:"type:varchar(500)" json:"avatar_url"`
	Age          *int     `json:"age"`
	Gender       *string  `gorm:"type:varchar(20)" json:"gender"`
	HeightCm     *float64 `json:"height_cm"`
	WeightKg     *float64 `json:"weight_kg"`
	MainActivity *string  `gorm:"type:varchar(100)" json:"main_activity"`

	// ── Gamification / progress ─────────────────────────────────────────────
	Level                int `gorm:"default:1" json:"level"`
	CurrentLevelExp      int `gorm:"default:0" json:"current_level_exp"`
	NextLevelExp         int `gorm:"default:100" json:"next_level_exp"`
	TotalExp             int `gorm:"default:0" json:"total_exp"`
	RewardPoints         int `gorm:"default:0" json:"reward_points"`          // Actively used: quest completion, reward claims, wallet
	StreakDays           int `gorm:"default:0" json:"streak_days"`
	BestStreak           int `gorm:"default:0" json:"best_streak"`
	TotalCompletedQuests int `gorm:"default:0" json:"total_completed_quests"`
	TotalSkippedQuests   int `gorm:"default:0" json:"total_skipped_quests"`

	// ── Deprecated gamification fields ─────────────────────────────────────
	// StreakShields: DEPRECATED — initialized but shield feature removed from MVP.
	// Retained for backward compatibility; never consumed by any feature.
	StreakShields int `gorm:"default:0" json:"streak_shields"`

	// ── Onboarding / generation context ────────────────────────────────────
	HasCompletedOnboarding bool           `gorm:"default:false" json:"has_completed_onboarding"`
	MainGoals              datatypes.JSON `gorm:"type:jsonb" json:"main_goals"`              // []string, nullable-safe
	HealthLimitations      datatypes.JSON `gorm:"type:jsonb" json:"health_limitations"`      // []string, nullable-safe
	WorkScheduleType       *string        `gorm:"type:varchar(50)" json:"work_schedule_type"`
	WorkWeekdays           datatypes.JSON `gorm:"type:jsonb" json:"work_weekdays"`           // []int, nullable-safe
	WorkStartTime          *string        `gorm:"type:varchar(10)" json:"work_start_time"`
	WorkEndTime            *string        `gorm:"type:varchar(10)" json:"work_end_time"`
	PreferredFreeTimes     datatypes.JSON `gorm:"type:jsonb" json:"preferred_free_times"`    // []string, nullable-safe
	FreeTimePreference     *string        `gorm:"type:varchar(50)" json:"free_time_preference"`
	LearningTimePreferences datatypes.JSON `gorm:"type:jsonb" json:"learning_time_preferences"` // []string, nullable-safe
	LearningTimePreference  *string        `gorm:"type:varchar(50)" json:"learning_time_preference"`
	MovementTimePreferences datatypes.JSON `gorm:"type:jsonb" json:"movement_time_preferences"` // []string, nullable-safe
	MovementTimePreference  *string        `gorm:"type:varchar(50)" json:"movement_time_preference"`
	SleepTimePreference     *string        `gorm:"type:varchar(50)" json:"sleep_time_preference"`
	NutritionTimePreference *string        `gorm:"type:varchar(50)" json:"nutrition_time_preference"`
	ActivityLevel           *string        `gorm:"type:varchar(50)" json:"activity_level"`
	LastWorkout             *string        `gorm:"type:varchar(50)" json:"last_workout"`
	WakeUpTime              *string        `gorm:"type:varchar(10)" json:"wake_up_time"`
	TargetSleepTime         *string        `gorm:"type:varchar(10)" json:"target_sleep_time"`
	QuietAfterTime          *string        `gorm:"type:varchar(10)" json:"quiet_after_time"`

	// ── Deprecated onboarding fields ───────────────────────────────────────
	// PreferredRewards: DEPRECATED — write-only (saved during onboarding but never read back).
	// Rewards were removed from MVP. Retained for backward compatibility.
	PreferredRewards datatypes.JSON `gorm:"type:jsonb" json:"preferred_rewards"` // []string, nullable-safe

	// LearningTopic: DEPRECATED — FE now sends null. Learning Path is the source of truth
	// for learning details. Quest generation uses ActiveLearningPath from roadmaps instead.
	// The AI system prompt explicitly bans using onboarding learning_topic.
	// Retained for backward compatibility; kept nullable.
	LearningTopic *string `gorm:"type:varchar(255)" json:"learning_topic"`

	// ── Timestamps ──────────────────────────────────────────────────────────
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserProfile) TableName() string {
	return "user_profiles"
}

func (u *UserProfile) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
