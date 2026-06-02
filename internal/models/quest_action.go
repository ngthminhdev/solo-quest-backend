package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type QuestActionType string

const (
	QuestActionStart       QuestActionType = "start"
	QuestActionComplete    QuestActionType = "complete"
	QuestActionSnooze      QuestActionType = "snooze"
	QuestActionSkip        QuestActionType = "skip"
	QuestActionViewReason  QuestActionType = "viewReason"
)

type QuestAction struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	QuestID       uuid.UUID       `gorm:"type:uuid;not null;index" json:"quest_id"`
	UserID        uuid.UUID       `gorm:"type:uuid;not null;index:idx_quest_actions_user_created" json:"user_id"`
	Action        QuestActionType `gorm:"type:varchar(20);not null" json:"action"`
	Note          string          `gorm:"type:text" json:"note"`
	Reason        string          `gorm:"type:text" json:"reason"`
	SnoozeMinutes int             `gorm:"default:0" json:"snooze_minutes"`
	CreatedAt     time.Time       `gorm:"index:idx_quest_actions_user_created" json:"created_at"`

	Quest  Quest       `gorm:"foreignKey:QuestID" json:"quest,omitempty"`
	User   UserProfile `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (QuestAction) TableName() string {
	return "quest_actions"
}

func (qa *QuestAction) BeforeCreate(tx *gorm.DB) error {
	if qa.ID == uuid.Nil {
		qa.ID = uuid.New()
	}
	return nil
}
