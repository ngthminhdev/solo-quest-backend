package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	LearningRoadmapGenJobStatusPending    = "pending"
	LearningRoadmapGenJobStatusGenerating = "generating"
	LearningRoadmapGenJobStatusCompleted  = "completed"
	LearningRoadmapGenJobStatusFailed     = "failed"
)

const (
	LearningRoadmapGenErrorAI         = "ai_error"
	LearningRoadmapGenErrorInvalid    = "invalid_output"
	LearningRoadmapGenErrorTimeout    = "timeout"
	LearningRoadmapGenErrorValidation = "validation_error"
	LearningRoadmapGenErrorInternal   = "internal_error"
)

// LearningRoadmapGenerationJob tracks an asynchronous AI roadmap generation
// request so POST /learning-roadmaps/generate can return before the AI call.
type LearningRoadmapGenerationJob struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_learning_roadmap_gen_jobs_user_hash" json:"user_id"`
	RequestHash string         `gorm:"type:varchar(64);not null;uniqueIndex:idx_learning_roadmap_gen_jobs_user_hash" json:"request_hash"`
	Preferences datatypes.JSON `gorm:"type:jsonb;not null" json:"preferences"`
	Status      string         `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	RoadmapID   *uuid.UUID     `gorm:"type:uuid;index" json:"roadmap_id"`

	ErrorType    *string `gorm:"type:varchar(50)" json:"error_type"`
	ErrorMessage *string `gorm:"type:text" json:"error_message"`

	GeneratedStepCount int `gorm:"not null;default:0" json:"generated_step_count"`

	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (LearningRoadmapGenerationJob) TableName() string {
	return "learning_roadmap_generation_jobs"
}

func (j *LearningRoadmapGenerationJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return nil
}
