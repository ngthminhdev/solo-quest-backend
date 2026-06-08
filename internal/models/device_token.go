package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DeviceToken struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index:idx_device_tokens_user_id;uniqueIndex:uq_device_tokens_user_token" json:"user_id"`
	Token      string         `gorm:"type:text;not null;uniqueIndex:uq_device_tokens_user_token" json:"token"`
	Platform   string         `gorm:"type:varchar(20);not null;default:android" json:"platform"`
	DeviceName *string        `gorm:"type:varchar(100)" json:"device_name"`
	IsActive   bool           `gorm:"not null;default:true" json:"is_active"`
	LastUsedAt *time.Time     `json:"last_used_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (DeviceToken) TableName() string {
	return "device_tokens"
}

func (dt *DeviceToken) BeforeCreate(tx *gorm.DB) error {
	if dt.ID == uuid.Nil {
		dt.ID = uuid.New()
	}
	return nil
}
