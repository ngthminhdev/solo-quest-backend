package dto

import (
	"time"

	"github.com/google/uuid"
)

type LearningRoadmapStepItem struct {
	ID               uuid.UUID  `json:"id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	OrderIndex       int        `json:"order_index"`
	EstimatedMinutes int        `json:"estimated_minutes"`
	Completed        bool       `json:"completed"`
	CompletedAt      *time.Time `json:"completed_at"`
}

type LearningRoadmapItem struct {
	ID               uuid.UUID                  `json:"id"`
	Title            string                     `json:"title"`
	Description      string                     `json:"description"`
	Category         string                     `json:"category"`
	Difficulty       string                     `json:"difficulty"`
	EstimatedMinutes int                        `json:"estimated_minutes"`
	TotalSteps       int                        `json:"total_steps"`
	CompletedSteps   int                        `json:"completed_steps"`
	ProgressPercent  int                        `json:"progress_percent"`
	Source           string                     `json:"source"`
	Status           string                     `json:"status"`
	Enabled          bool                       `json:"enabled"`
	StartedAt        *time.Time                 `json:"started_at"`
	CompletedAt      *time.Time                 `json:"completed_at"`
	Steps            []LearningRoadmapStepItem  `json:"steps"`
}

type FollowRoadmapResponse struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	RoadmapID   uuid.UUID  `json:"roadmap_id"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type ToggleRoadmapStepRequest struct {
	Completed *bool `json:"completed" binding:"required"`
}

type ToggleRoadmapStepResponse struct {
	RoadmapID      uuid.UUID  `json:"roadmap_id"`
	StepID         uuid.UUID  `json:"step_id"`
	Completed      bool       `json:"completed"`
	CompletedAt    *time.Time `json:"completed_at"`
	CompletedSteps int        `json:"completed_steps"`
	TotalSteps     int        `json:"total_steps"`
	ProgressPercent int       `json:"progress_percent"`
	RoadmapStatus  string     `json:"roadmap_status"`
}

// AI Suggestion DTOs

type UserProfileInput struct {
	CurrentSkills      []string `json:"current_skills"`
	ExperienceLevel    string   `json:"experience_level"`
	LearningGoals      []string `json:"learning_goals"`
	AvailableTimePerDay int      `json:"available_time_per_day"`
}

type RoadmapPreferencesInput struct {
	LearningGoal string   `json:"learning_goal"`
	Difficulty   string   `json:"difficulty"`
	Categories   []string `json:"categories"`
	Category     string   `json:"category"`
	MaxDuration  int      `json:"max_duration"`
}

type AiRoadmapSuggestRequest struct {
	UserProfile *UserProfileInput        `json:"user_profile"`
	Preferences RoadmapPreferencesInput  `json:"preferences"`
	Limit       int                      `json:"limit"`
}

type AiRoadmapSuggestionItem struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Difficulty       string `json:"difficulty"`
	EstimatedMinutes int    `json:"estimated_minutes"`
	TotalSteps       int    `json:"total_steps"`
	Reason           string `json:"reason"`
}

type AiRoadmapSuggestResponse struct {
	Suggestions []AiRoadmapSuggestionItem `json:"suggestions"`
	GeneratedAt time.Time                 `json:"generated_at"`
}

// Create Roadmap DTOs

type CreateLearningRoadmapCustomize struct {
	Title       *string `json:"title"`
	Difficulty  *string `json:"difficulty"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
}

type CreateLearningRoadmapStepRequest struct {
	Title            string `json:"title" binding:"required"`
	Description      string `json:"description"`
	OrderIndex       int    `json:"order_index"`
	EstimatedMinutes int    `json:"estimated_minutes"`
}

type CreateLearningRoadmapRequest struct {
	// For creating from suggestion
	SuggestionID *string                              `json:"suggestion_id"`
	Customize    *CreateLearningRoadmapCustomize      `json:"customize"`

	// For creating custom roadmap
	Title       *string                               `json:"title"`
	Description *string                               `json:"description"`
	Category    *string                               `json:"category"`
	Difficulty  *string                               `json:"difficulty"`
	Steps       []CreateLearningRoadmapStepRequest    `json:"steps"`

	// Common field
	Source      string                                `json:"source" binding:"required"`
}

// Template-based suggestion DTOs

type TemplateSuggestPreferences struct {
	LearningGoal string `json:"learning_goal"`
	Category     string `json:"category"`
	Difficulty   string `json:"difficulty"`
	MaxDuration  int    `json:"max_duration"`
}

type TemplateSuggestRequest struct {
	Preferences TemplateSuggestPreferences `json:"preferences"`
}

type TemplateSuggestionItem struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Difficulty       string `json:"difficulty"`
	EstimatedMinutes int    `json:"estimated_minutes"`
	TotalSteps       int    `json:"total_steps"`
	Source           string `json:"source"`
}

type TemplateSuggestResponse struct {
	Suggestions []TemplateSuggestionItem `json:"suggestions"`
}

type CreateFromTemplateRequest struct {
	TemplateID  string                      `json:"template_id" binding:"required"`
	Source      string                      `json:"source" binding:"required"`
	Preferences *TemplateSuggestPreferences `json:"preferences"`
}

type CreateFromTemplateResponse struct {
	Roadmap LearningRoadmapItem `json:"roadmap"`
}
