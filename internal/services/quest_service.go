package services

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

type QuestService struct {
	db *gorm.DB
}

func NewQuestService(db *gorm.DB) *QuestService {
	return &QuestService{db: db}
}

func (s *QuestService) GetQuestsByUserIDAndDate(userID uuid.UUID, date time.Time) ([]models.Quest, error) {
	var quests []models.Quest
	start, end := timeutil.DayRangeUTC(date)
	err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Order("created_at ASC").
		Find(&quests).Error
	if err != nil {
		return nil, err
	}
	return quests, nil
}
