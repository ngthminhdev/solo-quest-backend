package models

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type QuestStatus string

const (
	QuestStatusPending   QuestStatus = "pending"
	QuestStatusActive    QuestStatus = "active"
	QuestStatusCompleted QuestStatus = "completed"
	QuestStatusSkipped   QuestStatus = "skipped"
	QuestStatusSnoozed   QuestStatus = "snoozed"
	QuestStatusExpired   QuestStatus = "expired"
)

type QuestType string

const (
	QuestTypeMain       QuestType = "main"
	QuestTypeSide       QuestType = "side"
	QuestTypeDaily      QuestType = "daily"
	QuestTypeWeekly     QuestType = "weekly"
	QuestTypeWater      QuestType = "water"
	QuestTypeBreak      QuestType = "breakTime"
	QuestTypeMovement   QuestType = "movement"
	QuestTypeLearning   QuestType = "learning"
	QuestTypeReview     QuestType = "review"
	QuestTypeSleep      QuestType = "sleep"
	QuestTypeReflection QuestType = "reflection"
)

func ValidQuestTypes() []QuestType {
	return []QuestType{
		QuestTypeMain,
		QuestTypeSide,
		QuestTypeDaily,
		QuestTypeWeekly,
		QuestTypeWater,
		QuestTypeBreak,
		QuestTypeMovement,
		QuestTypeLearning,
		QuestTypeReview,
		QuestTypeSleep,
		QuestTypeReflection,
	}
}

func IsValidQuestType(t string) bool {
	for _, v := range ValidQuestTypes() {
		if string(v) == t {
			return true
		}
	}
	return false
}

type QuestDifficulty string

const (
	QuestDifficultyEasy   QuestDifficulty = "easy"
	QuestDifficultyMedium QuestDifficulty = "medium"
	QuestDifficultyHard   QuestDifficulty = "hard"
)

type QuestSource string

const (
	QuestSourceDailyPlan          QuestSource = "dailyPlan"
	QuestSourceUser               QuestSource = "user"
	QuestSourceAI                 QuestSource = "ai"
	QuestSourceDevRandomDailyPlan QuestSource = "devRandomDailyPlan"
	QuestSourceConfigBased        QuestSource = "configBased"
	// QuestSourceLearningRoadmap marks quests generated from an active learning
	// roadmap / current step (including deterministic roadmap fallbacks), so quest
	// completion can later update roadmap progress. Roadmap linkage detail lives in
	// Quest.LearningMetadata.
	QuestSourceLearningRoadmap QuestSource = "learning_roadmap"
)

type Quest struct {
	ID                  uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	UserID              uuid.UUID       `gorm:"type:uuid;not null;index:idx_quests_user_date,idx_quests_user_status,idx_quests_user_type" json:"user_id"`
	Title               string          `gorm:"type:varchar(255);not null" json:"title"`
	Description         string          `gorm:"type:text" json:"description"`
	Type                QuestType       `gorm:"type:varchar(20);not null;index:idx_quests_user_type" json:"type"`
	Status              QuestStatus     `gorm:"type:varchar(20);not null;default:'pending';index:idx_quests_user_status" json:"status"`
	Difficulty          QuestDifficulty `gorm:"type:varchar(20);default:'easy'" json:"difficulty"`
	Source              QuestSource     `gorm:"type:varchar(20);default:'dailyPlan'" json:"source"`
	XPReward            int             `gorm:"default:0" json:"exp"`
	EstimatedMinutes    int             `gorm:"default:0" json:"estimated_minutes"`
	Reason              string          `gorm:"type:text" json:"reason"`
	Instruction         string          `gorm:"type:text" json:"instruction"`
	Tags                datatypes.JSON  `gorm:"type:jsonb" json:"tags"`
	AvailableTimeBlocks datatypes.JSON  `gorm:"type:jsonb" json:"available_time_blocks"`
	LearningMetadata    datatypes.JSON  `gorm:"type:jsonb" json:"learning_metadata"`
	Date                time.Time       `gorm:"type:date;not null;index:idx_quests_user_date" json:"date"`
	DueDate             *time.Time      `gorm:"type:date" json:"due_date"`
	ReminderTime        *time.Time      `json:"reminder_time"`
	StartedAt           *time.Time      `json:"started_at"`
	SnoozedUntil        *time.Time      `json:"snoozed_until"`
	CompletedAt         *time.Time      `json:"completed_at"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Quest) TableName() string {
	return "quests"
}

func (q *Quest) BeforeCreate(tx *gorm.DB) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return nil
}

// SortQuests sorts a slice of Quest models consistently:
// 1. reminder_time ascending (NULLS LAST)
// 2. created_at ascending
// 3. id ascending
func SortQuests(quests []Quest) {
	sort.Slice(quests, func(i, j int) bool {
		qi, qj := quests[i], quests[j]
		
		// 1. reminder_time comparison (NULLS LAST)
		if qi.ReminderTime == nil && qj.ReminderTime != nil {
			return false // qi is null, qj is not null => qi should be after qj
		}
		if qi.ReminderTime != nil && qj.ReminderTime == nil {
			return true // qi is not null, qj is null => qi should be before qj
		}
		if qi.ReminderTime != nil && qj.ReminderTime != nil {
			if !qi.ReminderTime.Equal(*qj.ReminderTime) {
				return qi.ReminderTime.Before(*qj.ReminderTime)
			}
		}
		
		// 2. created_at comparison
		if !qi.CreatedAt.Equal(qj.CreatedAt) {
			return qi.CreatedAt.Before(qj.CreatedAt)
		}
		
		// 3. id comparison
		return qi.ID.String() < qj.ID.String()
	})
}
