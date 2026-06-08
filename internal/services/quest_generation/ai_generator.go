package quest_generation

import (
	"context"
	"fmt"
	"time"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/ai"
)

type AIGenerator struct {
	aiClient      ai.Client
	aiConfig      *ai.Config
	promptBuilder *PromptBuilder
	validator     *CandidateValidator
}

func NewAIGenerator(aiClient ai.Client) *AIGenerator {
	return &AIGenerator{
		aiClient:      aiClient,
		aiConfig:      nil, // Will use client defaults
		promptBuilder: NewPromptBuilder(),
		validator:     NewCandidateValidator(),
	}
}

func NewAIGeneratorWithConfig(aiClient ai.Client, aiConfig *ai.Config) *AIGenerator {
	return &AIGenerator{
		aiClient:      aiClient,
		aiConfig:      aiConfig,
		promptBuilder: NewPromptBuilder(),
		validator:     NewCandidateValidator(),
	}
}

func (g *AIGenerator) GenerateDailyQuests(ctx context.Context, qctx *UserQuestContext) ([]models.Quest, error) {
	if g.aiClient == nil {
		return nil, fmt.Errorf("AI client is not configured")
	}

	if qctx == nil {
		return nil, fmt.Errorf("UserQuestContext cannot be nil")
	}

	// Step 1: Build prompt from context
	systemPrompt, userPrompt, err := g.promptBuilder.BuildDailyQuestPrompt(qctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Step 2: Determine max tokens for quest generation
	maxTokens := 32000 // Default for quest generation
	if g.aiConfig != nil && g.aiConfig.QuestMaxTokens > 0 {
		maxTokens = g.aiConfig.QuestMaxTokens
	}

	// Step 3: Call AI client with lower temperature for consistency
	startTime := time.Now()
	aiResp, err := g.aiClient.GenerateText(ctx, ai.GenerateTextRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.2, // Lower temperature for more consistent output
		MaxTokens:    maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}
	latency := time.Since(startTime)

	if aiResp == nil {
		return nil, fmt.Errorf("AI response is nil")
	}

	if aiResp.Text == "" {
		return nil, fmt.Errorf("AI response text is empty")
	}

	// Step 3: Parse AI response
	expectedCount := qctx.DailyQuestCount
	if qctx.PreviewLimit > 0 {
		expectedCount = qctx.PreviewLimit
	}

	requestedLimitStr := "nil"
	if qctx.RequestedPreviewLimit != nil {
		requestedLimitStr = fmt.Sprintf("%d", *qctx.RequestedPreviewLimit)
	}

	candidateResp, err := ParseQuestCandidateResponse(aiResp.Text)
	if err != nil {
		fmt.Printf("[AIGenerator] Metadata: daily_quest_count=%d, requested_preview_limit=%s, effective_preview_limit=%d, generated_candidate_count=0\n",
			qctx.DailyQuestCount, requestedLimitStr, expectedCount)
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	candidateCount := 0
	if candidateResp != nil {
		candidateCount = len(candidateResp.Quests)
	}

	// Log safe metadata
	fmt.Printf("[AIGenerator] Metadata: daily_quest_count=%d, requested_preview_limit=%s, effective_preview_limit=%d, generated_candidate_count=%d\n",
		qctx.DailyQuestCount, requestedLimitStr, expectedCount, candidateCount)

	if candidateResp == nil || len(candidateResp.Quests) == 0 {
		return nil, fmt.Errorf("no quest candidates returned from AI")
	}

	// Verify target/max count limit for AI preview
	if candidateCount > expectedCount {
		return nil, fmt.Errorf("AI returned %d quests, which exceeds the target/max limit of %d.", candidateCount, expectedCount)
	}

	// Step 4: Normalize candidates (reminder times, sleep crossing midnight, tags)
	candidateResp.Quests = NormalizeCandidates(qctx, candidateResp.Quests, time.Now().In(timeutil.LocationVN))

	// Step 5: Validate candidates
	if err := g.validator.Validate(qctx, candidateResp.Quests); err != nil {
		return nil, fmt.Errorf("candidate validation failed: %w", err)
	}

	// Step 5: Map candidates to Quest models
	quests, err := MapCandidatesToQuests(qctx, candidateResp.Quests)
	if err != nil {
		return nil, fmt.Errorf("failed to map candidates to quests: %w", err)
	}

	// Log summary (without exposing raw prompt/response)
	logAIGenerationSummary(aiResp.Model, len(quests), latency)

	return quests, nil
}

func logAIGenerationSummary(model string, questCount int, latency time.Duration) {
	// Simple logging - can be enhanced with proper logging framework
	fmt.Printf("[AIGenerator] model=%s, quests=%d, latency=%dms\n", model, questCount, latency.Milliseconds())
}
