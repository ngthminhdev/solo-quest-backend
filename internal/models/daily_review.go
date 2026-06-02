package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type DailyReview struct {
	ID                    uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                uuid.UUID      `gorm:"type:uuid;not null;index:idx_daily_reviews_user_date,unique" json:"user_id"`
	Date                  time.Time      `gorm:"type:date;not null;index:idx_daily_reviews_user_date,unique" json:"date"`
	Mood                  string         `gorm:"type:varchar(20)" json:"mood"`
	DifficultyRating      int            `gorm:"default:0" json:"difficulty_rating"`
	EnergyLevel           int            `gorm:"default:0" json:"energy_level"`
	SatisfactionLevel     int            `gorm:"default:0" json:"satisfaction_level"`
	CompletedQuestCount   int            `gorm:"default:0" json:"completed_quest_count"`
	SkippedQuestCount     int            `gorm:"default:0" json:"skipped_quest_count"`
	EarnedExp             int            `gorm:"default:0" json:"earned_exp"`
	CompletionRate        float64        `json:"completion_rate"`
	HelpfulQuests         datatypes.JSON `gorm:"type:jsonb" json:"helpful_quests"`
	AnnoyingQuests        datatypes.JSON `gorm:"type:jsonb" json:"annoying_quests"`
	BestMoment            string         `gorm:"type:text" json:"best_moment"`
	Challenge             string         `gorm:"type:text" json:"challenge"`
	ImprovementTomorrow   string         `gorm:"type:text" json:"improvement_tomorrow"`
	TomorrowAdjustments   datatypes.JSON `gorm:"type:jsonb" json:"tomorrow_adjustments"`
	Note                  string         `gorm:"type:text" json:"note"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (DailyReview) TableName() string {
	return "daily_reviews"
}

func (dr *DailyReview) BeforeCreate(tx *gorm.DB) error {
	if dr.ID == uuid.Nil {
		dr.ID = uuid.New()
	}
	return nil
}
