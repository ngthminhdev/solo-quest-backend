package dto

import (
	"time"

	"github.com/google/uuid"
)

type SaveDailyReviewRequest struct {
	Date                 string   `json:"date"`
	Mood                 string   `json:"mood" binding:"required"`
	DifficultyRating     *int     `json:"difficulty_rating"`
	EnergyLevel          *int     `json:"energy_level"`
	SatisfactionLevel    *int     `json:"satisfaction_level"`
	HelpfulQuests        []string `json:"helpful_quests"`
	AnnoyingQuests       []string `json:"annoying_quests"`
	BestMoment           string   `json:"best_moment"`
	Challenge            string   `json:"challenge"`
	ImprovementTomorrow  string   `json:"improvement_tomorrow"`
	TomorrowAdjustments  []string `json:"tomorrow_adjustments"`
	Note                 string   `json:"note"`
}

type DailyReviewResponse struct {
	ID                   uuid.UUID  `json:"id"`
	UserID               uuid.UUID  `json:"user_id"`
	Date                 string     `json:"date"`
	Mood                 string     `json:"mood"`
	DifficultyRating     *int       `json:"difficulty_rating,omitempty"`
	EnergyLevel          *int       `json:"energy_level,omitempty"`
	SatisfactionLevel    *int       `json:"satisfaction_level,omitempty"`
	CompletedQuestCount  int        `json:"completed_quest_count"`
	SkippedQuestCount    int        `json:"skipped_quest_count"`
	EarnedExp            int        `json:"earned_exp"`
	CompletionRate       float64    `json:"completion_rate"`
	HelpfulQuests        []string   `json:"helpful_quests"`
	AnnoyingQuests       []string   `json:"annoying_quests"`
	BestMoment           string     `json:"best_moment"`
	Challenge            string     `json:"challenge"`
	ImprovementTomorrow  string     `json:"improvement_tomorrow"`
	TomorrowAdjustments  []string   `json:"tomorrow_adjustments"`
	Note                 string     `json:"note"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
}

type DailyReviewSummaryResponse struct {
	Date               string         `json:"date"`
	CompletedQuestCount int           `json:"completed_quest_count"`
	SkippedQuestCount   int           `json:"skipped_quest_count"`
	PendingQuestCount   int           `json:"pending_quest_count"`
	TotalQuestCount     int           `json:"total_quest_count"`
	EarnedExp           int           `json:"earned_exp"`
	CompletionRate      float64       `json:"completion_rate"`
	CompletedByType     map[string]int `json:"completed_by_type"`
}

type DailyReviewStatusResponse struct {
	Item        *DailyReviewResponse `json:"item"`
	HasReviewed bool                 `json:"has_reviewed"`
	Date        string               `json:"date"`
}
