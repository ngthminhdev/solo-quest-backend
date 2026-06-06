package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
)

type QuestService struct {
	db             *gorm.DB
	devGenerator   *DevQuestGenerator
	generator      quest_generation.Generator
	contextBuilder *quest_generation.UserQuestContextBuilder
}

func NewQuestService(db *gorm.DB) *QuestService {
	return &QuestService{
		db:             db,
		generator:      quest_generation.NewRuleBasedGenerator(db),
		contextBuilder: quest_generation.NewUserQuestContextBuilder(db),
	}
}

func NewQuestServiceWithDevGenerator(db *gorm.DB, devGenerator *DevQuestGenerator) *QuestService {
	return &QuestService{
		db:             db,
		devGenerator:   devGenerator,
		generator:      quest_generation.NewRuleBasedGenerator(db),
		contextBuilder: quest_generation.NewUserQuestContextBuilder(db),
	}
}

func (s *QuestService) GetQuestsByUserIDAndDate(userID uuid.UUID, date time.Time) ([]models.Quest, error) {
	var quests []models.Quest
	start, end := timeutil.DayRangeVN(date)
	err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Order("created_at ASC").
		Find(&quests).Error
	if err != nil {
		return nil, err
	}

	// Generate quests if none exist for this date
	if len(quests) == 0 {
		ctx := context.Background()
		qctx, err := s.contextBuilder.Build(ctx, userID, date)
		if err != nil {
			return nil, err
		}
		generated, genErr := s.generator.GenerateDailyQuests(ctx, qctx)
		if genErr != nil {
			return nil, genErr
		}
		if len(generated) > 0 {
			quests = generated
		}
	}

	return quests, nil
}
