package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type MoodLevel string

const (
	MoodLevelVeryBad  MoodLevel = "veryBad"
	MoodLevelBad      MoodLevel = "bad"
	MoodLevelNeutral  MoodLevel = "neutral"
	MoodLevelGood     MoodLevel = "good"
	MoodLevelVeryGood MoodLevel = "veryGood"
)

type DailyCheckin struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID              uuid.UUID      `gorm:"type:uuid;not null;index:idx_daily_checkins_user_date,unique" json:"user_id"`
	Date                time.Time      `gorm:"type:date;not null;index:idx_daily_checkins_user_date,unique" json:"date"`
	EnergyLevel         string         `gorm:"type:varchar(20)" json:"energy_level"`
	StressLevel         string         `gorm:"type:varchar(20)" json:"stress_level"`
	FocusLevel          string         `gorm:"type:varchar(20)" json:"focus_level"`
	DayIntensity        string         `gorm:"type:varchar(20)" json:"day_intensity"`
	MainFocusToday      string         `gorm:"type:text" json:"main_focus_today"`
	Note                string         `gorm:"type:text" json:"note"`
	AvailableTimeBlocks datatypes.JSON `gorm:"type:jsonb" json:"available_time_blocks"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (DailyCheckin) TableName() string {
	return "daily_checkins"
}

func (dc *DailyCheckin) BeforeCreate(tx *gorm.DB) error {
	if dc.ID == uuid.Nil {
		dc.ID = uuid.New()
	}
	return nil
}
