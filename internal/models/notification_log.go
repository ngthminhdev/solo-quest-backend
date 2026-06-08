package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type NotificationLogStatus string

const (
	NotificationLogStatusPending NotificationLogStatus = "pending"
	NotificationLogStatusSent    NotificationLogStatus = "sent"
	NotificationLogStatusFailed  NotificationLogStatus = "failed"
	NotificationLogStatusSkipped NotificationLogStatus = "skipped"
)

type NotificationLog struct {
	ID                uuid.UUID                `gorm:"type:uuid;primaryKey" json:"id"`
	UserID            uuid.UUID                `gorm:"type:uuid;not null;index:idx_notification_logs_user_id" json:"user_id"`
	QuestID           *uuid.UUID               `gorm:"type:uuid" json:"quest_id"`
	ReminderSettingID *uuid.UUID               `gorm:"type:uuid" json:"reminder_setting_id"`
	EventType         string                   `gorm:"type:varchar(50);not null;index:idx_notification_logs_event_type" json:"event_type"`
	Channel           string                   `gorm:"type:varchar(20);not null;default:'fcm'" json:"channel"`
	IdempotencyKey    string                   `gorm:"type:varchar(255);not null;uniqueIndex:uq_notification_logs_idempotency_key" json:"idempotency_key"`
	Title             string                   `gorm:"type:varchar(255)" json:"title"`
	Body              string                   `gorm:"type:text" json:"body"`
	Payload           datatypes.JSON           `gorm:"type:jsonb" json:"payload"`
	Status            NotificationLogStatus    `gorm:"type:varchar(20);not null;default:'pending';index:idx_notification_logs_status" json:"status"`
	ScheduledAt       *time.Time               `gorm:"index:idx_notification_logs_scheduled_at" json:"scheduled_at"`
	SentAt            *time.Time               `json:"sent_at"`
	Error             *string                  `gorm:"type:text" json:"error"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

func (NotificationLog) TableName() string {
	return "notification_logs"
}

func (nl *NotificationLog) BeforeCreate(tx *gorm.DB) error {
	if nl.ID == uuid.Nil {
		nl.ID = uuid.New()
	}
	return nil
}

func (nl *NotificationLog) SetPayload(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	nl.Payload = data
	return nil
}
