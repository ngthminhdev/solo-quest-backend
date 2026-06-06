package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LearningRoadmapSource string

const (
	LearningRoadmapSourceSystem   LearningRoadmapSource = "system"
	LearningRoadmapSourceTemplate LearningRoadmapSource = "template"
	LearningRoadmapSourceAI       LearningRoadmapSource = "ai"
	LearningRoadmapSourceCustom   LearningRoadmapSource = "custom"
)

type LearningRoadmap struct {
	ID               uuid.UUID             `gorm:"type:uuid;primaryKey" json:"id"`
	Title            string                `gorm:"type:varchar(255);not null" json:"title"`
	Description      string                `gorm:"type:text" json:"description"`
	Category         string                `gorm:"type:varchar(50);not null" json:"category"`
	Difficulty       string                `gorm:"type:varchar(20);not null;default:'normal'" json:"difficulty"`
	EstimatedMinutes int                   `gorm:"not null;default:0" json:"estimated_minutes"`
	TotalSteps       int                   `gorm:"not null;default:0" json:"total_steps"`
	Source           LearningRoadmapSource `gorm:"type:varchar(20);not null;default:'system'" json:"source"`
	CreatedByUserID  *uuid.UUID            `gorm:"type:uuid;index" json:"created_by_user_id,omitempty"`
	Enabled          bool                  `gorm:"not null;default:true" json:"enabled"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

func (LearningRoadmap) TableName() string {
	return "learning_roadmaps"
}

func (r *LearningRoadmap) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

type LearningRoadmapStep struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	RoadmapID        uuid.UUID `gorm:"type:uuid;not null;index:idx_learning_roadmap_steps_roadmap_id" json:"roadmap_id"`
	Title            string    `gorm:"type:varchar(255);not null" json:"title"`
	Description      string    `gorm:"type:text" json:"description"`
	OrderIndex       int       `gorm:"not null" json:"order_index"`
	EstimatedMinutes int       `gorm:"not null;default:0" json:"estimated_minutes"`
	Enabled          bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	Roadmap LearningRoadmap `gorm:"foreignKey:RoadmapID" json:"roadmap,omitempty"`
}

func (LearningRoadmapStep) TableName() string {
	return "learning_roadmap_steps"
}

func (s *LearningRoadmapStep) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type UserLearningRoadmapStatus string

const (
	UserLearningRoadmapStatusTracking  UserLearningRoadmapStatus = "tracking"
	UserLearningRoadmapStatusCompleted UserLearningRoadmapStatus = "completed"
	UserLearningRoadmapStatusArchived  UserLearningRoadmapStatus = "archived"
)

type UserLearningRoadmap struct {
	ID          uuid.UUID                  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID                  `gorm:"type:uuid;not null;uniqueIndex:uq_user_learning_roadmap" json:"user_id"`
	RoadmapID   uuid.UUID                  `gorm:"type:uuid;not null;uniqueIndex:uq_user_learning_roadmap" json:"roadmap_id"`
	Status      UserLearningRoadmapStatus  `gorm:"type:varchar(20);not null;default:'tracking'" json:"status"`
	StartedAt   time.Time                  `gorm:"not null" json:"started_at"`
	CompletedAt *time.Time                 `json:"completed_at"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`

	User    UserProfile     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Roadmap LearningRoadmap `gorm:"foreignKey:RoadmapID" json:"roadmap,omitempty"`
}

func (UserLearningRoadmap) TableName() string {
	return "user_learning_roadmaps"
}

func (u *UserLearningRoadmap) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

type UserLearningRoadmapStepProgress struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_user_roadmap_step_progress" json:"user_id"`
	RoadmapID   uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_user_roadmap_step_progress" json:"roadmap_id"`
	StepID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_user_roadmap_step_progress" json:"step_id"`
	Completed   bool       `gorm:"not null;default:false" json:"completed"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	User    UserProfile         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Roadmap LearningRoadmap     `gorm:"foreignKey:RoadmapID" json:"roadmap,omitempty"`
	Step    LearningRoadmapStep `gorm:"foreignKey:StepID" json:"step,omitempty"`
}

func (UserLearningRoadmapStepProgress) TableName() string {
	return "user_learning_roadmap_step_progress"
}

func (p *UserLearningRoadmapStepProgress) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
