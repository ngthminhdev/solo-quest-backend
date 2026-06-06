package dto

import (
	"time"

	"github.com/google/uuid"
)

type SaveDailyReviewRequest struct {
	Date             string `json:"date"`
	Mood             string `json:"mood" binding:"required"`
	EnergyLevel      string `json:"energy_level" binding:"required"`
	Satisfaction     int    `json:"satisfaction" binding:"required"`
	Reflection       string `json:"reflection"`
	TomorrowPriority string `json:"tomorrow_priority" binding:"required"`
}

type DailyReviewResponse struct {
	ID                  uuid.UUID  `json:"id"`
	UserID              uuid.UUID  `json:"user_id"`
	Date                string     `json:"date"`
	Mood                string     `json:"mood"`
	EnergyLevel         string     `json:"energy_level"`
	Satisfaction        int        `json:"satisfaction"`
	Reflection          string     `json:"reflection,omitempty"`
	TomorrowPriority    string     `json:"tomorrow_priority"`
	AISummary           string     `json:"ai_summary,omitempty"`
	CompletedQuestCount int        `json:"completed_quest_count"`
	SkippedQuestCount   int        `json:"skipped_quest_count"`
	EarnedExp           int        `json:"earned_exp"`
	CompletionRate      float64    `json:"completion_rate"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
}

type DailyReviewSummaryResponse struct {
	Date                string         `json:"date"`
	CompletedQuestCount int            `json:"completed_quest_count"`
	SkippedQuestCount   int            `json:"skipped_quest_count"`
	PendingQuestCount   int            `json:"pending_quest_count"`
	TotalQuestCount     int            `json:"total_quest_count"`
	EarnedExp           int            `json:"earned_exp"`
	CompletionRate      float64        `json:"completion_rate"`
	CompletedByType     map[string]int `json:"completed_by_type"`
}

type DailyReviewStatusResponse struct {
	Item        *DailyReviewResponse `json:"item"`
	HasReviewed bool                 `json:"has_reviewed"`
	Date        string               `json:"date"`
}
