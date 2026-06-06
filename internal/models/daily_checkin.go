package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MoodLevel string

const (
	MoodLevelVeryBad  MoodLevel = "very_bad"
	MoodLevelBad      MoodLevel = "bad"
	MoodLevelNormal   MoodLevel = "normal"
	MoodLevelGood     MoodLevel = "good"
	MoodLevelVeryGood MoodLevel = "very_good"
)

type DailyCheckin struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID              uuid.UUID  `gorm:"type:uuid;not null;index:idx_daily_checkins_user_date,unique" json:"user_id"`
	Date                time.Time  `gorm:"type:date;not null;index:idx_daily_checkins_user_date,unique" json:"date"`
	Mood                string     `gorm:"type:varchar(20)" json:"mood"`
	EnergyLevel         string     `gorm:"type:varchar(20)" json:"energy_level"`
	Availability        string     `gorm:"type:varchar(20)" json:"availability"`
	Priority            string     `gorm:"type:varchar(20)" json:"priority"`
	StressLevel         string     `gorm:"type:varchar(20)" json:"stress_level,omitempty"`
	FocusLevel          string     `gorm:"type:varchar(20)" json:"focus_level,omitempty"`
	DayIntensity        string     `gorm:"type:varchar(20)" json:"day_intensity,omitempty"`
	MainFocusToday      string     `gorm:"type:text" json:"main_focus_today,omitempty"`
	Note                string     `gorm:"type:text" json:"note,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`

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
