package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type UserProfile struct {
	ID                     uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	DisplayName            string         `gorm:"type:varchar(100);not null" json:"display_name"`
	AvatarURL              *string        `gorm:"type:varchar(500)" json:"avatar_url"`
	Age                    *int           `json:"age"`
	Gender                 *string        `gorm:"type:varchar(20)" json:"gender"`
	HeightCm               *float64       `json:"height_cm"`
	WeightKg               *float64       `json:"weight_kg"`
	MainActivity           *string        `gorm:"type:varchar(100)" json:"main_activity"`
	Level                  int            `gorm:"default:1" json:"level"`
	CurrentLevelExp        int            `gorm:"default:0" json:"current_level_exp"`
	NextLevelExp           int            `gorm:"default:100" json:"next_level_exp"`
	TotalExp               int            `gorm:"default:0" json:"total_exp"`
	RewardPoints           int            `gorm:"default:0" json:"reward_points"`
	StreakDays             int            `gorm:"default:0" json:"streak_days"`
	BestStreak             int            `gorm:"default:0" json:"best_streak"`
	StreakShields          int            `gorm:"default:0" json:"streak_shields"`
	TotalCompletedQuests   int            `gorm:"default:0" json:"total_completed_quests"`
	TotalSkippedQuests     int            `gorm:"default:0" json:"total_skipped_quests"`
	HasCompletedOnboarding bool           `gorm:"default:false" json:"has_completed_onboarding"`
	QuietAfterTime         *string        `gorm:"type:varchar(10)" json:"quiet_after_time"`
	MainGoals              datatypes.JSON `gorm:"type:jsonb" json:"main_goals"`
	HealthLimitations      datatypes.JSON `gorm:"type:jsonb" json:"health_limitations"`
	PreferredRewards       datatypes.JSON `gorm:"type:jsonb" json:"preferred_rewards"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
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
