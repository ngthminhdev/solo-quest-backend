package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Quest generation job statuses.
const (
	QuestGenJobStatusPending    = "pending"
	QuestGenJobStatusGenerating = "generating"
	QuestGenJobStatusCompleted  = "completed"
	QuestGenJobStatusFailed     = "failed"
)

// Quest generation job sources.
const (
	QuestGenJobSourceAI        = "ai"
	QuestGenJobSourceRuleBased = "rule_based"
)

// DailyQuestGenerationJob tracks an asynchronous "generate today's quests"
// run for a single user/date. The HTTP handler returns immediately after
// creating/claiming a job and a background worker fills in the result so the
// request never blocks on a slow AI call.
type DailyQuestGenerationJob struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_quest_gen_jobs_user_date" json:"user_id"`
	// Date is the local (VN) calendar date the job generates quests for,
	// stored normalized to start-of-day.
	Date   time.Time `gorm:"type:date;not null;index;uniqueIndex:idx_quest_gen_jobs_user_date" json:"date"`
	Status string    `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`

	Source       *string `gorm:"type:varchar(20)" json:"source"`
	FallbackUsed bool    `gorm:"not null;default:false" json:"fallback_used"`
	AIErrorType  *string `gorm:"type:varchar(50)" json:"ai_error_type"`
	ErrorMessage *string `gorm:"type:text" json:"error_message"`

	GeneratedCount       int `gorm:"not null;default:0" json:"generated_count"`
	PreservedCount       int `gorm:"not null;default:0" json:"preserved_count"`
	ReplacedPendingCount int `gorm:"not null;default:0" json:"replaced_pending_count"`

	PreferAI           bool `gorm:"not null;default:true" json:"prefer_ai"`
	Force              bool `gorm:"not null;default:false" json:"force"`
	ReplacePendingOnly bool `gorm:"not null;default:true" json:"replace_pending_only"`

	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (DailyQuestGenerationJob) TableName() string {
	return "daily_quest_generation_jobs"
}

func (j *DailyQuestGenerationJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return nil
}
