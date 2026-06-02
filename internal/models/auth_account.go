package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuthProvider string

const (
	AuthProviderGoogle AuthProvider = "google"
	AuthProviderEmail  AuthProvider = "email"
	AuthProviderDev    AuthProvider = "dev"
)

type AuthAccount struct {
	ID           uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID    `gorm:"type:uuid;not null;index" json:"user_id"`
	Provider     AuthProvider `gorm:"type:varchar(20);not null" json:"provider"`
	ProviderUID  string       `gorm:"type:varchar(255);not null" json:"provider_uid"`
	Email        string       `gorm:"type:varchar(255)" json:"email"`
	PasswordHash string       `gorm:"type:varchar(255)" json:"-"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (AuthAccount) TableName() string {
	return "auth_accounts"
}

func (a *AuthAccount) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
