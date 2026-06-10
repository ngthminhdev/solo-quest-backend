package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type LogEntryType string

const (
	LogEntryTypeActivity      LogEntryType = "activity"
	LogEntryTypeMood          LogEntryType = "mood"
	LogEntryTypeHabit         LogEntryType = "habit"
	LogEntryTypeGoal          LogEntryType = "goal"
	LogEntryTypeSystem        LogEntryType = "system"
	LogEntryTypeQuestStarted                    LogEntryType = "questStarted"
	LogEntryTypeQuestCompleted                  LogEntryType = "questCompleted"
	LogEntryTypeQuestSkipped                    LogEntryType = "questSkipped"
	LogEntryTypeQuestSnoozed                    LogEntryType = "questSnoozed"
	LogEntryTypeMorningCheckin                  LogEntryType = "morningCheckin"
	LogEntryTypeDailyReview                     LogEntryType = "dailyReview"
	LogEntryTypeRewardClaimed                   LogEntryType = "rewardClaimed"
	LogEntryTypeLevelUp                         LogEntryType = "level_up"
	LogEntryTypeLearningRoadmapCreated          LogEntryType = "learning_roadmap_created"
	LogEntryTypeLearningRoadmapFollowed         LogEntryType = "learning_roadmap_followed"
	LogEntryTypeLearningRoadmapStepCompleted    LogEntryType = "learning_roadmap_step_completed"
	LogEntryTypeLearningRoadmapStepUncompleted  LogEntryType = "learning_roadmap_step_uncompleted"
	LogEntryTypeLearningRoadmapCompleted        LogEntryType = "learning_roadmap_completed"
)

type LogEntry struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID      `gorm:"type:uuid;not null;index:idx_log_entries_user_created,idx_log_entries_user_type" json:"user_id"`
	Type         LogEntryType   `gorm:"type:varchar(50);not null;index:idx_log_entries_user_type" json:"type"`
	Title        string         `gorm:"type:varchar(255);not null" json:"title"`
	Content      string         `gorm:"type:text" json:"content"`
	Metadata     datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	QuestID      *uuid.UUID     `gorm:"type:uuid" json:"quest_id"`
	QuestType    *QuestType     `gorm:"type:varchar(20)" json:"quest_type"`
	ExpChanged   int            `gorm:"default:0" json:"exp_changed"`
	PointsChanged int           `gorm:"default:0" json:"points_changed"`
	CreatedAt    time.Time      `gorm:"index:idx_log_entries_user_created" json:"created_at"`

	User UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (LogEntry) TableName() string {
	return "log_entries"
}

func (le *LogEntry) BeforeCreate(tx *gorm.DB) error {
	if le.ID == uuid.Nil {
		le.ID = uuid.New()
	}
	return nil
}
