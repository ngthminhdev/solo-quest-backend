package dto

import "github.com/google/uuid"

type LogItem struct {
	ID            uuid.UUID  `json:"id"`
	Type          string     `json:"type"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	QuestID       *uuid.UUID `json:"quest_id"`
	QuestType     *string    `json:"quest_type"`
	ExpChanged    int        `json:"exp_changed"`
	PointsChanged int        `json:"points_changed"`
	CreatedAt     string     `json:"created_at"`
}

type LogFilter struct {
	Type      string
	QuestType string
	Date      string
	From      string
	To        string
	Limit     int
	Offset    int
}

type LogListResponse struct {
	Items  []LogItem `json:"items"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}
