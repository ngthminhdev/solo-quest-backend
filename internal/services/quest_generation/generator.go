package quest_generation

import (
	"context"
	"solo_quest_backend/internal/models"
)

type Generator interface {
	GenerateDailyQuests(ctx context.Context, qctx *UserQuestContext) ([]models.Quest, error)
}
