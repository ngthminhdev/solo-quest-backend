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

func BuildLearningMetadata(path *ActiveLearningPathDetail) datatypes.JSON {
	if path == nil || path.RoadmapID == "" || path.StepID == "" {
		return nil
	}
	type lm struct {
		LearningRoadmapID    string `json:"learning_roadmap_id"`
		LearningStepID       string `json:"learning_step_id"`
		LearningStepTitle    string `json:"learning_step_title"`
		LearningStepOrder    int    `json:"learning_step_order_index"`
		LearningTotalSteps   int    `json:"learning_total_steps"`
	}
	m := lm{
		LearningRoadmapID:  path.RoadmapID,
		LearningStepID:     path.StepID,
		LearningStepTitle:  path.CurrentStepTitle,
		LearningStepOrder:  path.StepOrderIndex,
		LearningTotalSteps: path.TotalSteps,
	}
	b, _ := json.Marshal(m)
	return datatypes.JSON(b)
}

type LearningMeta struct {
	LearningRoadmapID string `json:"learning_roadmap_id"`
	LearningStepID    string `json:"learning_step_id"`
}

func ParseLearningMetadata(q models.Quest) (LearningMeta, bool) {
	if len(q.LearningMetadata) == 0 {
		return LearningMeta{}, false
	}
	var m LearningMeta
	if err := json.Unmarshal(q.LearningMetadata, &m); err != nil {
		return LearningMeta{}, false
	}
	return m, m.LearningStepID != ""
}

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

	hhMM, ok := parseReminderToHHMM(candidate.ReminderTime)
	if !ok {
		return nil, fmt.Errorf("failed to parse reminder time '%s'", candidate.ReminderTime)
	}
	var hour, minute int
	fmt.Sscanf(hhMM, "%d:%d", &hour, &minute)

	today := timeutil.StartOfDayVN(qctx.LocalDate)
	var reminderTime time.Time
	if qType == models.QuestTypeSleep && hour >= 0 && hour <= 4 {
		reminderTime = time.Date(today.Year(), today.Month(), today.Day()+1, hour, minute, 0, 0, timeutil.LocationVN)
	} else {
		reminderTime = time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, timeutil.LocationVN)
	}

	// Safety: if generating for today and reminder is in the past, push to next safe slot.
	now := time.Now().In(timeutil.LocationVN)
	if today.Equal(timeutil.StartOfDayVN(now)) && reminderTime.Before(now) {
		reminderTime = NextSafeTimeSlot(now)
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
		// XP is always derived from the (repaired) difficulty by backend policy;
		// AI-provided xp_reward is never trusted.
		XPReward:         XPForDifficulty(candidate.Difficulty),
		EstimatedMinutes: candidate.EstimatedMinutes,
		Reason:           candidate.Reason,
		Instruction:      candidate.Instruction,
		Tags:             datatypes.JSON(tagsJSON),
		Date:             today,
		DueDate:          &dueDate,
		ReminderTime:     &reminderTime,
		LearningMetadata: BuildLearningMetadata(qctx.ActiveLearningPath),
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
