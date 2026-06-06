package dto

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

// QuestResponse is the DTO for quest API responses
// It excludes the nested User object to avoid unnecessary data transfer
type QuestResponse struct {
	ID                  uuid.UUID          `json:"id"`
	UserID              uuid.UUID          `json:"user_id"`
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	Type                string             `json:"type"`
	Status              string             `json:"status"`
	Difficulty          string             `json:"difficulty"`
	Source              string             `json:"source"`
	XPReward            int                `json:"exp"`
	EstimatedMinutes    int                `json:"estimated_minutes"`
	Reason              string             `json:"reason"`
	Instruction         string             `json:"instruction"`
	Tags                datatypes.JSON     `json:"tags"`
	AvailableTimeBlocks datatypes.JSON     `json:"available_time_blocks"`
	Date                time.Time          `json:"date"`
	DueDate             *timeutil.DateOnly `json:"due_date"`
	ReminderTime        *time.Time         `json:"reminder_time"`
	StartedAt           *time.Time         `json:"started_at"`
	SnoozedUntil        *time.Time         `json:"snoozed_until"`
	CompletedAt         *time.Time         `json:"completed_at"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
}

// QuestPreviewResponse is the DTO for quest preview responses, excluding DB-only fields like user_id and timestamps
type QuestPreviewResponse struct {
	ID                  uuid.UUID          `json:"id"`
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	Type                string             `json:"type"`
	Status              string             `json:"status"`
	Difficulty          string             `json:"difficulty"`
	Source              string             `json:"source"`
	XPReward            int                `json:"exp"`
	EstimatedMinutes    int                `json:"estimated_minutes"`
	Reason              string             `json:"reason"`
	Instruction         string             `json:"instruction"`
	Tags                datatypes.JSON     `json:"tags"`
	AvailableTimeBlocks datatypes.JSON     `json:"available_time_blocks"`
	Date                time.Time          `json:"date"`
	DueDate             *timeutil.DateOnly `json:"due_date"`
	ReminderTime        *time.Time         `json:"reminder_time"`
}

// ToQuestResponse converts a models.Quest to QuestResponse DTO
func ToQuestResponse(quest models.Quest) QuestResponse {
	var dueDate *timeutil.DateOnly
	if quest.DueDate != nil {
		d := timeutil.NewDateOnly(*quest.DueDate)
		dueDate = &d
	}

	return QuestResponse{
		ID:                  quest.ID,
		UserID:              quest.UserID,
		Title:               quest.Title,
		Description:         quest.Description,
		Type:                string(quest.Type),
		Status:              string(quest.Status),
		Difficulty:          string(quest.Difficulty),
		Source:              string(quest.Source),
		XPReward:            quest.XPReward,
		EstimatedMinutes:    quest.EstimatedMinutes,
		Reason:              quest.Reason,
		Instruction:         quest.Instruction,
		Tags:                quest.Tags,
		AvailableTimeBlocks: quest.AvailableTimeBlocks,
		Date:                quest.Date,
		DueDate:             dueDate,
		ReminderTime:        quest.ReminderTime,
		StartedAt:           quest.StartedAt,
		SnoozedUntil:        quest.SnoozedUntil,
		CompletedAt:         quest.CompletedAt,
		CreatedAt:           quest.CreatedAt,
		UpdatedAt:           quest.UpdatedAt,
	}
}

// ToQuestPreviewResponse converts a models.Quest to QuestPreviewResponse DTO, generating temp UUID if empty
func ToQuestPreviewResponse(quest models.Quest) QuestPreviewResponse {
	var dueDate *timeutil.DateOnly
	if quest.DueDate != nil {
		d := timeutil.NewDateOnly(*quest.DueDate)
		dueDate = &d
	}

	id := quest.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	return QuestPreviewResponse{
		ID:                  id,
		Title:               quest.Title,
		Description:         quest.Description,
		Type:                string(quest.Type),
		Status:              string(quest.Status),
		Difficulty:          string(quest.Difficulty),
		Source:              string(quest.Source),
		XPReward:            quest.XPReward,
		EstimatedMinutes:    quest.EstimatedMinutes,
		Reason:              quest.Reason,
		Instruction:         quest.Instruction,
		Tags:                quest.Tags,
		AvailableTimeBlocks: quest.AvailableTimeBlocks,
		Date:                quest.Date,
		DueDate:             dueDate,
		ReminderTime:        quest.ReminderTime,
	}
}

// ToQuestResponses converts a slice of models.Quest to []QuestResponse
func ToQuestResponses(quests []models.Quest) []QuestResponse {
	responses := make([]QuestResponse, len(quests))
	for i, quest := range quests {
		responses[i] = ToQuestResponse(quest)
	}
	return responses
}

// ToQuestPreviewResponses converts a slice of models.Quest to []QuestPreviewResponse
func ToQuestPreviewResponses(quests []models.Quest) []QuestPreviewResponse {
	responses := make([]QuestPreviewResponse, len(quests))
	for i, quest := range quests {
		responses[i] = ToQuestPreviewResponse(quest)
	}
	return responses
}

// QuestListResponse represents the response for GET /api/quests
type QuestListResponse struct {
	Quests []QuestResponse `json:"quests"`
	Date   string          `json:"date"`
}
