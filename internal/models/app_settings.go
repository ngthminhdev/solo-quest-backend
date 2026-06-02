package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AppSettings struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID               uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	Locale               string         `gorm:"type:varchar(10);default:'en'" json:"locale"`
	Theme                string         `gorm:"type:varchar(20);default:'system'" json:"theme"`
	DailyQuestLimit      int            `gorm:"default:10" json:"daily_quest_limit"`
	NotificationsEnabled bool           `gorm:"default:true" json:"notifications_enabled"`
	QuietAfterTime       string         `gorm:"type:varchar(10);default:'22:00'" json:"quiet_after_time"`
	DailyReminderTime    *string        `gorm:"type:varchar(10)" json:"daily_reminder_time"`
	WeeklyReviewDay      *string        `gorm:"type:varchar(10)" json:"weekly_review_day"`
	Timezone             string         `gorm:"type:varchar(50);default:'UTC'" json:"timezone"`
	Preferences          datatypes.JSON `gorm:"type:jsonb" json:"preferences"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (AppSettings) TableName() string {
	return "app_settings"
}

func (as *AppSettings) BeforeCreate(tx *gorm.DB) error {
	if as.ID == uuid.Nil {
		as.ID = uuid.New()
	}
	return nil
}
