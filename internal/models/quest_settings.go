package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type QuestSettings struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID            uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_quest_settings_user_id" json:"user_id"`
	DailyQuestCount   int            `gorm:"not null;default:8" json:"daily_quest_count"`
	Difficulty        string         `gorm:"type:varchar(20);not null;default:'normal'" json:"difficulty"`
	AutoAdjustEnabled bool           `gorm:"not null;default:true" json:"auto_adjust_enabled"`
	EnabledCategories datatypes.JSON `gorm:"type:jsonb;not null" json:"enabled_categories"`
	PreferredDuration string         `gorm:"type:varchar(20);not null;default:'medium'" json:"preferred_duration"`
	RestDayEnabled    bool           `gorm:"not null;default:false" json:"rest_day_enabled"`
	Rules             datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"rules"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (QuestSettings) TableName() string {
	return "quest_settings"
}

func (qs *QuestSettings) BeforeCreate(tx *gorm.DB) error {
	if qs.ID == uuid.Nil {
		qs.ID = uuid.New()
	}
	return nil
}
