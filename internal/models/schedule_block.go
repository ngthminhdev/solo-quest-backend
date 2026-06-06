package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ScheduleBlockType string

const (
	ScheduleBlockTypeSchool    ScheduleBlockType = "school"
	ScheduleBlockTypeWork      ScheduleBlockType = "work"
	ScheduleBlockTypeCommute   ScheduleBlockType = "commute"
	ScheduleBlockTypeMeal      ScheduleBlockType = "meal"
	ScheduleBlockTypeSleep     ScheduleBlockType = "sleep"
	ScheduleBlockTypeStudy     ScheduleBlockType = "study"
	ScheduleBlockTypePersonal  ScheduleBlockType = "personal"
	ScheduleBlockTypeBusy      ScheduleBlockType = "busy"
	ScheduleBlockTypeFree      ScheduleBlockType = "free"
	ScheduleBlockTypeOther     ScheduleBlockType = "other"
)

func ValidScheduleBlockTypes() []ScheduleBlockType {
	return []ScheduleBlockType{
		ScheduleBlockTypeSchool,
		ScheduleBlockTypeWork,
		ScheduleBlockTypeCommute,
		ScheduleBlockTypeMeal,
		ScheduleBlockTypeSleep,
		ScheduleBlockTypeStudy,
		ScheduleBlockTypePersonal,
		ScheduleBlockTypeBusy,
		ScheduleBlockTypeFree,
		ScheduleBlockTypeOther,
	}
}

func IsValidScheduleBlockType(t string) bool {
	for _, v := range ValidScheduleBlockTypes() {
		if string(v) == t {
			return true
		}
	}
	return false
}

type ScheduleBlock struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;index:idx_schedule_blocks_user_id" json:"user_id"`
	Title       string         `gorm:"type:varchar(200);not null" json:"title"`
	Type        ScheduleBlockType `gorm:"type:varchar(20);not null;index:idx_schedule_blocks_type" json:"type"`
	DaysOfWeek  datatypes.JSON `gorm:"type:jsonb;not null" json:"days_of_week"`
	StartTime   string         `gorm:"type:varchar(5);not null" json:"start_time"`
	EndTime     string         `gorm:"type:varchar(5);not null" json:"end_time"`
	IsBusy      bool           `gorm:"not null;default:false" json:"is_busy"`
	IsFlexible  bool           `gorm:"not null;default:false" json:"is_flexible"`
	Enabled     bool           `gorm:"not null;default:true;index:idx_schedule_blocks_enabled" json:"enabled"`
	Location    *string        `gorm:"type:varchar(300)" json:"location"`
	Note        *string        `gorm:"type:text" json:"note"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (ScheduleBlock) TableName() string {
	return "schedule_blocks"
}

func (sb *ScheduleBlock) BeforeCreate(tx *gorm.DB) error {
	if sb.ID == uuid.Nil {
		sb.ID = uuid.New()
	}
	return nil
}
