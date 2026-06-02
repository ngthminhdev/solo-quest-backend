package dto

import (
	"time"

	"github.com/google/uuid"
)

type SaveDailyCheckinRequest struct {
	Date                string   `json:"date"`
	EnergyLevel         string   `json:"energy_level" binding:"required"`
	StressLevel         string   `json:"stress_level" binding:"required"`
	FocusLevel          string   `json:"focus_level" binding:"required"`
	DayIntensity        string   `json:"day_intensity" binding:"required"`
	MainFocusToday      string   `json:"main_focus_today"`
	Note                string   `json:"note"`
	AvailableTimeBlocks []string `json:"available_time_blocks"`
}

type DailyCheckinResponse struct {
	ID                  uuid.UUID  `json:"id"`
	UserID              uuid.UUID  `json:"user_id"`
	Date                string     `json:"date"`
	EnergyLevel         string     `json:"energy_level"`
	StressLevel         string     `json:"stress_level"`
	FocusLevel          string     `json:"focus_level"`
	DayIntensity        string     `json:"day_intensity"`
	MainFocusToday      string     `json:"main_focus_today"`
	Note                string     `json:"note"`
	AvailableTimeBlocks []string   `json:"available_time_blocks"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
}

type DailyCheckinStatusResponse struct {
	Item           *DailyCheckinResponse `json:"item"`
	HasCheckedIn   bool                  `json:"has_checked_in"`
	Date           string                `json:"date"`
}
