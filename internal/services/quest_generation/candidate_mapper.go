package quest_generation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

func MapCandidateToQuest(qctx *UserQuestContext, candidate QuestCandidate) (*models.Quest, error) {
	if qctx == nil {
		return nil, fmt.Errorf("UserQuestContext cannot be nil")
	}

	var qType models.QuestType
	switch candidate.Type {
	case "water":
		qType = models.QuestTypeWater
	case "breakTime":
		qType = models.QuestTypeBreak
	case "movement":
		qType = models.QuestTypeMovement
	case "learning":
		qType = models.QuestTypeLearning
	case "sleep":
		qType = models.QuestTypeSleep
	case "review":
		qType = models.QuestTypeReview
	default:
		qType = models.QuestType(candidate.Type)
	}

	var qDiff models.QuestDifficulty
	switch strings.ToLower(candidate.Difficulty) {
	case "easy":
		qDiff = models.QuestDifficultyEasy
	case "normal", "medium":
		qDiff = models.QuestDifficultyMedium
	case "hard":
		qDiff = models.QuestDifficultyHard
	default:
		qDiff = models.QuestDifficultyMedium
	}

	var hour, minute int
	_, err := fmt.Sscanf(candidate.ReminderTime, "%d:%d", &hour, &minute)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reminder time '%s': %w", candidate.ReminderTime, err)
	}
	today := timeutil.StartOfDayVN(qctx.LocalDate)
	var reminderTime time.Time
	if qType == models.QuestTypeSleep && hour >= 0 && hour <= 4 {
		reminderTime = time.Date(today.Year(), today.Month(), today.Day()+1, hour, minute, 0, 0, timeutil.LocationVN)
	} else {
		reminderTime = time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, timeutil.LocationVN)
	}
	dueDate := today

	tagsJSON, err := json.Marshal(candidate.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}

	quest := &models.Quest{
		UserID:           qctx.UserID,
		Title:            candidate.Title,
		Description:      candidate.Description,
		Type:             qType,
		Status:           models.QuestStatusPending,
		Difficulty:       qDiff,
		Source:           models.QuestSourceAI,
		XPReward:         candidate.XPReward,
		EstimatedMinutes: candidate.EstimatedMinutes,
		Reason:           candidate.Reason,
		Instruction:      candidate.Instruction,
		Tags:             datatypes.JSON(tagsJSON),
		Date:             today,
		DueDate:          &dueDate,
		ReminderTime:     &reminderTime,
	}

	return quest, nil
}

func MapCandidatesToQuests(qctx *UserQuestContext, candidates []QuestCandidate) ([]models.Quest, error) {
	if qctx == nil {
		return nil, fmt.Errorf("UserQuestContext cannot be nil")
	}

	quests := make([]models.Quest, len(candidates))
	for i, c := range candidates {
		q, err := MapCandidateToQuest(qctx, c)
		if err != nil {
			return nil, fmt.Errorf("failed to map candidate[%d]: %w", i, err)
		}
		quests[i] = *q
	}
	return quests, nil
}
