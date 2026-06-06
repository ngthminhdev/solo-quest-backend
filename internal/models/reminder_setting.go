package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReminderType string

const (
	ReminderTypeWater       ReminderType = "water"
	ReminderTypeBreakTime   ReminderType = "break_time"
	ReminderTypeMovement    ReminderType = "movement"
	ReminderTypeLearning    ReminderType = "learning"
	ReminderTypeSleep       ReminderType = "sleep"
	ReminderTypeDailyReview ReminderType = "daily_review"
	ReminderTypeCustom      ReminderType = "custom"
)

func ValidReminderTypes() []ReminderType {
	return []ReminderType{
		ReminderTypeWater,
		ReminderTypeBreakTime,
		ReminderTypeMovement,
		ReminderTypeLearning,
		ReminderTypeSleep,
		ReminderTypeDailyReview,
		ReminderTypeCustom,
	}
}

func IsValidReminderType(t string) bool {
	for _, v := range ValidReminderTypes() {
		if string(v) == t {
			return true
		}
	}
	return false
}

type ReminderFrequency string

const (
	ReminderFrequencyFixed         ReminderFrequency = "fixed"
	ReminderFrequencyInterval      ReminderFrequency = "interval"
	ReminderFrequencyRandomInRange ReminderFrequency = "random_in_range"
	ReminderFrequencySmart         ReminderFrequency = "smart"
)

func ValidReminderFrequencies() []ReminderFrequency {
	return []ReminderFrequency{
		ReminderFrequencyFixed,
		ReminderFrequencyInterval,
		ReminderFrequencyRandomInRange,
		ReminderFrequencySmart,
	}
}

func IsValidReminderFrequency(f string) bool {
	for _, v := range ValidReminderFrequencies() {
		if string(v) == f {
			return true
		}
	}
	return false
}

type ReminderStatus string

const (
	ReminderStatusEnabled  ReminderStatus = "enabled"
	ReminderStatusDisabled ReminderStatus = "disabled"
)

func ValidReminderStatuses() []ReminderStatus {
	return []ReminderStatus{
		ReminderStatusEnabled,
		ReminderStatusDisabled,
	}
}

func IsValidReminderStatus(s string) bool {
	for _, v := range ValidReminderStatuses() {
		if string(v) == s {
			return true
		}
	}
	return false
}

type ReminderSetting struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID          uuid.UUID      `gorm:"type:uuid;not null;index:idx_reminder_settings_user_id;uniqueIndex:uq_reminder_settings_user_type" json:"user_id"`
	Type            ReminderType   `gorm:"type:varchar(30);not null;uniqueIndex:uq_reminder_settings_user_type;index:idx_reminder_settings_type" json:"type"`
	Title           string         `gorm:"type:varchar(100);not null" json:"title"`
	Description     string         `gorm:"type:text;not null;default:''" json:"description"`
	Frequency       ReminderFrequency `gorm:"type:varchar(20);not null;default:'fixed'" json:"frequency"`
	Status          ReminderStatus `gorm:"type:varchar(10);not null;default:'enabled'" json:"status"`
	StartTime       *string        `gorm:"type:varchar(5)" json:"start_time"`
	EndTime         *string        `gorm:"type:varchar(5)" json:"end_time"`
	IntervalMinutes *int           `json:"interval_minutes"`
	MaxPerDay       *int           `json:"max_per_day"`
	SmartEnabled    bool           `gorm:"default:false" json:"smart_enabled"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (ReminderSetting) TableName() string {
	return "reminder_settings"
}

func (rs *ReminderSetting) BeforeCreate(tx *gorm.DB) error {
	if rs.ID == uuid.Nil {
		rs.ID = uuid.New()
	}
	return nil
}
