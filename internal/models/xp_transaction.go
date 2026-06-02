package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type XPSourceType string

const (
	XPSourceTypeQuest           XPSourceType = "quest"
	XPSourceTypeCheckin         XPSourceType = "checkin"
	XPSourceTypeReview          XPSourceType = "review"
	XPSourceTypeStreak          XPSourceType = "streak"
	XPSourceTypeBonus           XPSourceType = "bonus"
	XPSourceTypeRedemption      XPSourceType = "redemption"
	XPSourceTypeQuestCompletion XPSourceType = "quest_completion"
	XPSourceTypeRewardClaim     XPSourceType = "reward_claim"
)

type XPCurrencyType string

const (
	XPCurrencyXP          XPCurrencyType = "xp"
	XPCurrencyGem         XPCurrencyType = "gem"
	XPCurrencyRewardPoints XPCurrencyType = "reward_points"
)

type XPTransaction struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index:idx_xp_transactions_user_created,idx_xp_transactions_user_currency" json:"user_id"`
	Amount        int            `gorm:"not null" json:"amount"`
	Currency      XPCurrencyType `gorm:"type:varchar(20);not null;index:idx_xp_transactions_user_currency" json:"currency"`
	Source        XPSourceType   `gorm:"type:varchar(20);not null" json:"source"`
	SourceID      *uuid.UUID     `gorm:"type:uuid" json:"source_id"`
	ReferenceID   *uuid.UUID     `gorm:"type:uuid" json:"reference_id"`
	Description   string         `gorm:"type:text" json:"description"`
	BalanceAfter  int            `gorm:"default:0" json:"balance_after"`
	CreatedAt     time.Time      `gorm:"index:idx_xp_transactions_user_created" json:"created_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (XPTransaction) TableName() string {
	return "xp_transactions"
}

func (xt *XPTransaction) BeforeCreate(tx *gorm.DB) error {
	if xt.ID == uuid.Nil {
		xt.ID = uuid.New()
	}
	return nil
}
