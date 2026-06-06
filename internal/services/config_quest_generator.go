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

// ConfigQuestGenerator wraps the new RuleBasedGenerator and UserQuestContextBuilder
// to maintain backward compatibility with existing tests and services.
type ConfigQuestGenerator struct {
	db             *gorm.DB
	generator      *quest_generation.RuleBasedGenerator
	contextBuilder *quest_generation.UserQuestContextBuilder
}

func NewConfigQuestGenerator(db *gorm.DB) *ConfigQuestGenerator {
	return &ConfigQuestGenerator{
		db:             db,
		generator:      quest_generation.NewRuleBasedGenerator(db),
		contextBuilder: quest_generation.NewUserQuestContextBuilder(db),
	}
}

// GenerateConfigBasedDailyQuests delegates to the new quest generation package
func (g *ConfigQuestGenerator) GenerateConfigBasedDailyQuests(userID uuid.UUID, date time.Time) ([]models.Quest, error) {
	// Check if quests already exist for this date
	count, err := g.CountQuestsForDate(userID, date)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, nil
	}

	ctx := context.Background()
	qctx, err := g.contextBuilder.Build(ctx, userID, date)
	if err != nil {
		return nil, err
	}

	return g.generator.GenerateDailyQuests(ctx, qctx)
}

func (g *ConfigQuestGenerator) CountQuestsForDate(userID uuid.UUID, date time.Time) (int64, error) {
	start, end := timeutil.DayRangeVN(date)
	var count int64
	err := g.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Count(&count).Error
	return count, err
}
