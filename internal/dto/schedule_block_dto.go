package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateScheduleBlockRequest struct {
	Title      string `json:"title" binding:"required"`
	Type       string `json:"type" binding:"required"`
	DaysOfWeek []int  `json:"days_of_week" binding:"required"`
	StartTime  string `json:"start_time" binding:"required"`
	EndTime    string `json:"end_time" binding:"required"`
	IsBusy     bool   `json:"is_busy"`
	IsFlexible bool   `json:"is_flexible"`
	Location   string `json:"location"`
	Note       string `json:"note"`
}

type UpdateScheduleBlockRequest struct {
	Title      *string `json:"title"`
	Type       *string `json:"type"`
	DaysOfWeek *[]int  `json:"days_of_week"`
	StartTime  *string `json:"start_time"`
	EndTime    *string `json:"end_time"`
	IsBusy     *bool   `json:"is_busy"`
	IsFlexible *bool   `json:"is_flexible"`
	Enabled    *bool   `json:"enabled"`
	Location   *string `json:"location"`
	Note       *string `json:"note"`
}

type ScheduleBlockResponse struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Type       string    `json:"type"`
	DaysOfWeek []int     `json:"days_of_week"`
	StartTime  string    `json:"start_time"`
	EndTime    string    `json:"end_time"`
	IsBusy     bool      `json:"is_busy"`
	IsFlexible bool      `json:"is_flexible"`
	Enabled    bool      `json:"enabled"`
	Location   *string   `json:"location"`
	Note       *string   `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
