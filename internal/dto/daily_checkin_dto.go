package dto

import (
	"time"

	"github.com/google/uuid"
)

type SaveDailyCheckinRequest struct {
	Date         string `json:"date"`
	Mood         string `json:"mood" binding:"required"`
	EnergyLevel  string `json:"energy_level" binding:"required"`
	Availability string `json:"availability" binding:"required"`
	Priority     string `json:"priority" binding:"required"`
}

type DailyCheckinResponse struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Date        string     `json:"date"`
	Mood        string     `json:"mood"`
	EnergyLevel string     `json:"energy_level"`
	Availability string    `json:"availability"`
	Priority    string     `json:"priority"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type DailyCheckinStatusResponse struct {
	Item           *DailyCheckinResponse `json:"item"`
	HasCheckedIn   bool                  `json:"has_checked_in"`
	Date           string                `json:"date"`
}
