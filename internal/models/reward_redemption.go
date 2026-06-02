package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RewardRedemption struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index:idx_reward_redemptions_user_created" json:"user_id"`
	RewardID  uuid.UUID `gorm:"type:uuid;not null" json:"reward_id"`
	Cost      int       `gorm:"not null" json:"cost"`
	CreatedAt time.Time `gorm:"index:idx_reward_redemptions_user_created" json:"created_at"`

	User   UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Reward Reward      `gorm:"foreignKey:RewardID" json:"reward,omitempty"`
}

func (RewardRedemption) TableName() string {
	return "reward_redemptions"
}

func (rr *RewardRedemption) BeforeCreate(tx *gorm.DB) error {
	if rr.ID == uuid.Nil {
		rr.ID = uuid.New()
	}
	return nil
}
