package dto

import (
	"time"

	"github.com/google/uuid"
)

type RewardItem struct {
	ID         uuid.UUID  `json:"id"`
	Title      string     `json:"title"`
	Description string    `json:"description"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	CostPoints int        `json:"cost_points"`
	IconText   string     `json:"icon_text"`
	ClaimedAt  *time.Time `json:"claimed_at"`
	CanClaim   bool       `json:"can_claim"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type RewardWallet struct {
	RewardPoints int `json:"reward_points"`
}

type RewardListResponse struct {
	Items  []RewardItem  `json:"items"`
	Wallet RewardWallet  `json:"wallet"`
}

type ClaimRewardResponse struct {
	Reward     RewardItem         `json:"reward"`
	Redemption RedemptionItem     `json:"redemption"`
	Transaction XPTransactionItem `json:"transaction"`
	Profile    ProfileSummary     `json:"profile"`
	Message    string             `json:"message"`
}

type RedemptionItem struct {
	ID          uuid.UUID `json:"id"`
	RewardID    uuid.UUID `json:"reward_id"`
	RewardTitle string    `json:"reward_title"`
	RewardType  string    `json:"reward_type"`
	IconText    string    `json:"icon_text"`
	PointsSpent int       `json:"points_spent"`
	CreatedAt   time.Time `json:"created_at"`
}

type RedemptionFilter struct {
	Limit  int
	Offset int
}

type RedemptionListResponse struct {
	Items  []RedemptionItem `json:"items"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type XPTransactionItem struct {
	ID           uuid.UUID `json:"id"`
	Amount       int       `json:"amount"`
	Currency     string    `json:"currency"`
	Source       string    `json:"source"`
	ReferenceID  *uuid.UUID `json:"reference_id"`
	Description  string    `json:"description"`
	BalanceAfter int       `json:"balance_after"`
	CreatedAt    time.Time `json:"created_at"`
}

type ProfileSummary struct {
	ID           uuid.UUID `json:"id"`
	RewardPoints int       `json:"reward_points"`
}
