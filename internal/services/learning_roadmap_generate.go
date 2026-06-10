package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/ai"
)

var (
	ErrAIDisabled         = errors.New("AI generation is disabled")
	ErrAIProviderFailed   = errors.New("AI provider failed")
	ErrAIInvalidOutput    = errors.New("AI returned invalid output")
	ErrAITooFewValidSteps = errors.New("AI returned too few valid steps")
	ErrEmptyLearningGoal  = errors.New("learning goal is required")
	ErrInvalidMaxDuration = errors.New("max_duration must be between 30 and 1200 minutes")
)

type aiGeneratedRoadmap struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Difficulty  string            `json:"difficulty"`
	Steps       []aiGeneratedStep `json:"steps"`
}

type aiGeneratedStep struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	OrderIndex       int    `json:"order_index"`
	EstimatedMinutes int    `json:"estimated_minutes"`
	Outcome          string `json:"outcome"`
}

func (s *LearningRoadmapService) GenerateRoadmapFromPreferences(ctx context.Context, userID uuid.UUID, req dto.GenerateLearningRoadmapRequest) (*dto.GenerateLearningRoadmapResponse, error) {
	return s.generateRoadmapSync(ctx, userID, req)
}

func validateGenerateRoadmapRequest(req dto.GenerateLearningRoadmapRequest) (string, int, error) {
	prefs := req.Preferences

	goal := strings.TrimSpace(prefs.LearningGoal)
	if goal == "" || len([]rune(goal)) < 3 {
		return "", 0, ErrEmptyLearningGoal
	}
	if len([]rune(goal)) > 300 {
		return "", 0, ErrEmptyLearningGoal
	}

	if prefs.Category != "" && len([]rune(prefs.Category)) > 80 {
		return "", 0, errors.New("category too long")
	}

	maxDuration := prefs.MaxDuration
	if maxDuration == 0 {
		maxDuration = 300
	} else if maxDuration < 30 || maxDuration > 1200 {
		return "", 0, ErrInvalidMaxDuration
	}

	return goal, maxDuration, nil
}

func (s *LearningRoadmapService) generateRoadmapSync(ctx context.Context, userID uuid.UUID, req dto.GenerateLearningRoadmapRequest) (*dto.GenerateLearningRoadmapResponse, error) {
	prefs := req.Preferences
	goal, maxDuration, err := validateGenerateRoadmapRequest(req)
	if err != nil {
		return nil, err
	}

	if s.aiClient == nil {
		return nil, ErrAIDisabled
	}

	systemPrompt := buildGenerateRoadmapSystemPrompt()
	userPrompt := buildGenerateRoadmapUserPrompt(goal, prefs.Category, prefs.Difficulty, maxDuration)

	aiReq := ai.GenerateTextRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3,
		MaxTokens:    32000,
	}

	aiResp, err := s.aiClient.GenerateText(ctx, aiReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAIProviderFailed, err)
	}

	parsed, err := parseAndValidateGeneratedRoadmap(aiResp.Text, goal, prefs.Category, prefs.Difficulty, maxDuration)
	if err != nil {
		return nil, err
	}

	difficulty := normalizeGenerateDifficulty(parsed.Difficulty)
	if prefs.Difficulty != "" && prefs.Difficulty != "any" {
		difficulty = normalizeDifficulty(prefs.Difficulty)
	}

	category := strings.TrimSpace(parsed.Category)
	if prefs.Category != "" {
		category = prefs.Category
	}
	if category == "" {
		category = "General"
	}

	estimatedMinutes := 0
	for _, step := range parsed.Steps {
		estimatedMinutes += step.EstimatedMinutes
	}

	totalSteps := len(parsed.Steps)

	var roadmap models.LearningRoadmap
	var steps []models.LearningRoadmapStep
	var userRoadmap models.UserLearningRoadmap

	err = s.db.Transaction(func(tx *gorm.DB) error {
		roadmap = models.LearningRoadmap{
			Title:            parsed.Title,
			Description:      parsed.Description,
			Category:         category,
			Difficulty:       difficulty,
			EstimatedMinutes: estimatedMinutes,
			TotalSteps:       totalSteps,
			Source:           models.LearningRoadmapSourceAI,
			CreatedByUserID:  &userID,
			Enabled:          true,
		}

		if err := tx.Create(&roadmap).Error; err != nil {
			return err
		}

		steps = make([]models.LearningRoadmapStep, 0, len(parsed.Steps))
		for i, aiStep := range parsed.Steps {
			step := models.LearningRoadmapStep{
				RoadmapID:        roadmap.ID,
				Title:            aiStep.Title,
				Description:      aiStep.Description,
				OrderIndex:       i,
				EstimatedMinutes: aiStep.EstimatedMinutes,
				Enabled:          true,
			}
			if err := tx.Create(&step).Error; err != nil {
				return err
			}
			steps = append(steps, step)
		}

		now := timeutil.NowUTC()
		userRoadmap = models.UserLearningRoadmap{
			UserID:    userID,
			RoadmapID: roadmap.ID,
			Status:    models.UserLearningRoadmapStatusTracking,
			StartedAt: now,
		}
		if err := tx.Create(&userRoadmap).Error; err != nil {
			return err
		}

		logEntry := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeLearningRoadmapCreated,
			Title:     "Tạo lộ trình học từ AI",
			Content:   parsed.Title,
			CreatedAt: now,
		}
		if err := tx.Create(&logEntry).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	stepItems := make([]dto.LearningRoadmapStepItem, 0, len(steps))
	for _, step := range steps {
		stepItems = append(stepItems, dto.LearningRoadmapStepItem{
			ID:               step.ID,
			Title:            step.Title,
			Description:      step.Description,
			OrderIndex:       step.OrderIndex,
			EstimatedMinutes: step.EstimatedMinutes,
			Completed:        false,
			CompletedAt:      nil,
		})
	}

	item := dto.LearningRoadmapItem{
		ID:               roadmap.ID,
		Title:            roadmap.Title,
		Description:      roadmap.Description,
		Category:         roadmap.Category,
		Difficulty:       roadmap.Difficulty,
		EstimatedMinutes: roadmap.EstimatedMinutes,
		TotalSteps:       roadmap.TotalSteps,
		CompletedSteps:   0,
		ProgressPercent:  0,
		Source:           string(roadmap.Source),
		Status:           string(userRoadmap.Status),
		Enabled:          roadmap.Enabled,
		StartedAt:        &userRoadmap.StartedAt,
		CompletedAt:      nil,
		Steps:            stepItems,
	}

	return &dto.GenerateLearningRoadmapResponse{
		Item:               item,
		Source:             "ai",
		GeneratedStepCount: totalSteps,
	}, nil
}

func normalizeGenerateDifficulty(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	switch d {
	case "beginner", "intermediate", "advanced", "normal":
		return normalizeDifficulty(d)
	default:
		return "intermediate"
	}
}

var genericStepTitlePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^học tập\b`),
	regexp.MustCompile(`(?i)^học\s+\d+\s*phút\b`),
	regexp.MustCompile(`(?i)^đọc tài liệu\b`),
	regexp.MustCompile(`(?i)^tìm hiểu kiến thức\b`),
	regexp.MustCompile(`(?i)^chọn một chủ đề\b`),
	regexp.MustCompile(`(?i)^chọn chủ đề\b`),
	regexp.MustCompile(`(?i)^ghi lại 3 ý\b`),
	regexp.MustCompile(`(?i)^ôn lại\b`),
	regexp.MustCompile(`(?i)^ôn tập\b`),
	regexp.MustCompile(`(?i)^practice\b`),
	regexp.MustCompile(`(?i)^luyện tập\b`),
	regexp.MustCompile(`(?i)^nghiên cứu\b`),
	regexp.MustCompile(`(?i)^tìm hiểu về\b`),
}

func isGenericGeneratedStepTitle(title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return true
	}
	if len([]rune(title)) < 5 {
		return true
	}
	for _, p := range genericStepTitlePatterns {
		if p.MatchString(title) {
			return true
		}
	}
	return false
}

func clampGeneratedStepMinutes(minutes int) int {
	if minutes < 10 {
		return 10
	}
	if minutes > 45 {
		return 45
	}
	return minutes
}

func parseAndValidateGeneratedRoadmap(raw string, goal string, requestCategory string, requestDifficulty string, maxDuration int) (*aiGeneratedRoadmap, error) {
	cleaned := extractJSONFromAIResponse(raw)

	var parsed aiGeneratedRoadmap
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAIInvalidOutput, err)
	}

	if strings.TrimSpace(parsed.Title) == "" {
		return nil, fmt.Errorf("%w: missing title", ErrAIInvalidOutput)
	}
	if strings.TrimSpace(parsed.Description) == "" {
		return nil, fmt.Errorf("%w: missing description", ErrAIInvalidOutput)
	}

	validSteps := make([]aiGeneratedStep, 0, len(parsed.Steps))
	for _, step := range parsed.Steps {
		title := strings.TrimSpace(step.Title)
		desc := strings.TrimSpace(step.Description)
		if title == "" || desc == "" {
			continue
		}
		if isGenericGeneratedStepTitle(title) {
			continue
		}
		step.Title = title
		step.Description = desc
		step.EstimatedMinutes = clampGeneratedStepMinutes(step.EstimatedMinutes)
		validSteps = append(validSteps, step)
	}

	if len(validSteps) < 3 {
		return nil, fmt.Errorf("%w: got %d valid steps, need at least 3", ErrAITooFewValidSteps, len(validSteps))
	}

	if len(validSteps) > 12 {
		validSteps = validSteps[:12]
	}

	if maxDuration > 0 {
		total := 0
		for _, s := range validSteps {
			total += s.EstimatedMinutes
		}
		for total > maxDuration && len(validSteps) > 3 {
			total -= validSteps[len(validSteps)-1].EstimatedMinutes
			validSteps = validSteps[:len(validSteps)-1]
		}
	}

	if len(validSteps) < 3 {
		return nil, fmt.Errorf("%w: after max_duration truncation only %d steps remain", ErrAITooFewValidSteps, len(validSteps))
	}

	parsed.Steps = validSteps
	return &parsed, nil
}

func extractJSONFromAIResponse(raw string) string {
	raw = strings.TrimSpace(raw)

	if idx := strings.Index(raw, "```json"); idx >= 0 {
		start := idx + len("```json")
		raw = raw[start:]
		if endIdx := strings.LastIndex(raw, "```"); endIdx >= 0 {
			raw = raw[:endIdx]
		}
	} else if idx := strings.Index(raw, "```"); idx >= 0 {
		start := idx + len("```")
		raw = raw[start:]
		if endIdx := strings.LastIndex(raw, "```"); endIdx >= 0 {
			raw = raw[:endIdx]
		}
	}

	raw = strings.TrimSpace(raw)

	for {
		stripped := strings.TrimLeft(raw, " \t\n\r")
		if stripped == raw {
			break
		}
		raw = stripped

		if trimmed, ok := trimCodeBlockFencePrefix(raw); ok {
			raw = trimmed
			continue
		}
		break
	}

	return raw
}

func trimCodeBlockFencePrefix(s string) (string, bool) {
	if strings.HasPrefix(s, "json\n") || strings.HasPrefix(s, "json\r\n") {
		return s[5:], true
	}
	if strings.HasPrefix(s, "JSON\n") || strings.HasPrefix(s, "JSON\r\n") {
		return s[5:], true
	}
	return s, false
}

func buildGenerateRoadmapSystemPrompt() string {
	return `You are an expert learning path designer. Your task is to generate a personalized learning roadmap from the user's learning goal.

The roadmap will drive daily learning quests. Each step must be specific, actionable, and small enough to become a daily learning quest.

Rules:
- Avoid generic steps like "Học tập 20 phút", "Đọc tài liệu", "Tìm hiểu kiến thức", "Practice".
- Each step title must be specific and descriptive (e.g., "Viết unit test đầu tiên với JUnit" not "Học testing").
- Each step description must be a clear, actionable instruction.
- Use Vietnamese language by default (both titles and descriptions).
- The roadmap should have a logical progression from foundational to advanced topics.

Return strict JSON only, no markdown, no explanation. The JSON must follow this exact schema:

{
  "title": "string (specific to the learning goal)",
  "description": "string (overview of what this roadmap covers)",
  "category": "string (the main category/subject of the roadmap)",
  "difficulty": "beginner|intermediate|advanced",
  "steps": [
    {
      "title": "string (specific, action-oriented step title)",
      "description": "string (what to learn/do in this step)",
      "order_index": 1,
      "estimated_minutes": 20,
      "outcome": "string (what the learner will be able to do after this step)"
    }
  ]
}`
}

func buildGenerateRoadmapUserPrompt(goal string, category string, difficulty string, maxDuration int) string {
	var sb strings.Builder
	sb.WriteString("Generate a personalized learning roadmap.\n\n")
	sb.WriteString(fmt.Sprintf("Learning goal: %s\n", goal))

	if category != "" {
		sb.WriteString(fmt.Sprintf("Preferred category: %s\n", category))
	}

	diff := difficulty
	if diff == "" || diff == "any" {
		diff = "let me decide based on the goal"
	}
	sb.WriteString(fmt.Sprintf("Difficulty level: %s\n", diff))
	sb.WriteString(fmt.Sprintf("Maximum total duration: %d minutes\n", maxDuration))
	sb.WriteString(fmt.Sprintf("Target steps: between 5 and 12 steps, with each step taking 10-45 minutes.\n"))
	sb.WriteString("Ensure steps are specific, actionable, and suitable for daily learning quests.\n")
	sb.WriteString("Return JSON only.\n")

	return sb.String()
}
