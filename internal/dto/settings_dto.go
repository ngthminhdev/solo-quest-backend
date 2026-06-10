package dto

import (
	"time"

	"github.com/google/uuid"
)

type UpdateSettingsRequest struct {
	NotificationsEnabled *bool   `json:"notifications_enabled"`
	DailyReminderTime    *string `json:"daily_reminder_time"`
	QuietAfterTime       *string `json:"quiet_after_time"`
	QuietHoursEnabled    *bool   `json:"quiet_hours_enabled"`
	QuietStartTime       *string `json:"quiet_start_time"`
	QuietEndTime         *string `json:"quiet_end_time"`
	Timezone             *string `json:"timezone"`
}

// --- Reminder Settings DTOs (FE-aligned) ---

type ReminderSettingResponse struct {
	ID              uuid.UUID `json:"id"`
	Type            string    `json:"type"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Frequency       string    `json:"frequency"`
	Status          string    `json:"status"`
	StartTime       *string   `json:"start_time"`
	EndTime         *string   `json:"end_time"`
	IntervalMinutes *int      `json:"interval_minutes"`
	MaxPerDay       *int      `json:"max_per_day"`
	SmartEnabled    bool      `json:"smart_enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ReminderSettingsListResponse struct {
	Data []ReminderSettingResponse `json:"data"`
}

type ReminderSettingPatchResponse struct {
	Data ReminderSettingResponse `json:"data"`
}

type UpdateReminderSettingRequest struct {
	Frequency       *string `json:"frequency"`
	Status          *string `json:"status"`
	StartTime       *string `json:"start_time"`
	EndTime         *string `json:"end_time"`
	IntervalMinutes *int          `json:"interval_minutes"`
	MaxPerDay       Optional[int] `json:"max_per_day"`
	SmartEnabled    *bool         `json:"smart_enabled"`
}

type ToggleReminderSettingRequest struct {
	Status string `json:"status"`
}
