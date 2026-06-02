package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RewardStatus string

const (
	RewardStatusAvailable RewardStatus = "available"
	RewardStatusClaimed   RewardStatus = "claimed"
	RewardStatusRedeemed  RewardStatus = "redeemed"
	RewardStatusExpired   RewardStatus = "expired"
)

type RewardType string

const (
	RewardTypeRest          RewardType = "rest"
	RewardTypeEntertainment RewardType = "entertainment"
	RewardTypeFood          RewardType = "food"
	RewardTypeCustom        RewardType = "custom"
)

type Reward struct {
	ID          uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID    `gorm:"type:uuid;not null;index:idx_rewards_user_status" json:"user_id"`
	Title       string       `gorm:"type:varchar(255);not null" json:"title"`
	Description string       `gorm:"type:text" json:"description"`
	Type        RewardType   `gorm:"type:varchar(20);not null" json:"type"`
	CostPoints  int          `gorm:"not null" json:"cost_points"`
	IconText    string       `gorm:"type:varchar(10)" json:"icon_text"`
	Status      RewardStatus `gorm:"type:varchar(20);not null;default:'available';index:idx_rewards_user_status" json:"status"`
	ImageURL    string       `gorm:"type:varchar(500)" json:"image_url"`
	ClaimedAt   *time.Time   `json:"claimed_at"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Reward) TableName() string {
	return "rewards"
}

func (r *Reward) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
