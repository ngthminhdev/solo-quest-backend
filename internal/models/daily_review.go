package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type DailyReview struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID              uuid.UUID      `gorm:"type:uuid;not null;index:idx_daily_reviews_user_date,unique" json:"user_id"`
	Date                time.Time      `gorm:"type:date;not null;index:idx_daily_reviews_user_date,unique" json:"date"`
	Mood                string         `gorm:"type:varchar(20)" json:"mood"`
	EnergyLevel         string         `gorm:"type:varchar(20)" json:"energy_level"`
	Satisfaction        int            `gorm:"default:0" json:"satisfaction"`
	Reflection          string         `gorm:"type:text" json:"reflection,omitempty"`
	TomorrowPriority    string         `gorm:"type:varchar(20)" json:"tomorrow_priority"`
	AISummary           string         `gorm:"type:text" json:"ai_summary,omitempty"`
	CompletedQuestCount int            `gorm:"default:0" json:"completed_quest_count"`
	SkippedQuestCount   int            `gorm:"default:0" json:"skipped_quest_count"`
	EarnedExp           int            `gorm:"default:0" json:"earned_exp"`
	CompletionRate      float64        `json:"completion_rate"`
	DifficultyRating    int            `gorm:"default:0" json:"difficulty_rating,omitempty"`
	EnergyLevelInt      int            `gorm:"column:energy_level_int;default:0" json:"-"`
	SatisfactionLevel   int            `gorm:"default:0" json:"satisfaction_level,omitempty"`
	HelpfulQuests       datatypes.JSON `gorm:"type:jsonb" json:"helpful_quests,omitempty"`
	AnnoyingQuests      datatypes.JSON `gorm:"type:jsonb" json:"annoying_quests,omitempty"`
	BestMoment          string         `gorm:"type:text" json:"best_moment,omitempty"`
	Challenge           string         `gorm:"type:text" json:"challenge,omitempty"`
	ImprovementTomorrow string         `gorm:"type:text" json:"improvement_tomorrow,omitempty"`
	TomorrowAdjustments datatypes.JSON `gorm:"type:jsonb" json:"tomorrow_adjustments,omitempty"`
	Note                string         `gorm:"type:text" json:"note,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`

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
