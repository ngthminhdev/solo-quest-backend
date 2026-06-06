package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserSession struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	RefreshTokenHash string     `gorm:"type:text;not null;uniqueIndex" json:"-"`
	UserAgent        *string    `gorm:"type:text" json:"user_agent"`
	IPAddress        *string    `gorm:"type:text" json:"ip_address"`
	ExpiresAt        time.Time  `gorm:"not null;index" json:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (UserSession) TableName() string {
	return "user_sessions"
}

func (s *UserSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
