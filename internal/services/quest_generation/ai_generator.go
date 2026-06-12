package quest_generation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/pkg/logger"
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

// AIGenerationReport summarizes how the per-candidate quality gate processed a
// raw AI batch. It is backend-only telemetry surfaced to the generation service
// for structured logging; it never reaches the frontend.
type AIGenerationReport struct {
	CandidateCount int
	KeptCount      int
	DroppedCount   int
	TopDropReasons string
}

func (g *AIGenerator) GenerateDailyQuests(ctx context.Context, qctx *UserQuestContext) ([]models.Quest, error) {
	quests, _, err := g.GenerateDailyQuestsReport(ctx, qctx)
	return quests, err
}

// GenerateDailyQuestsReport generates quests and returns a per-batch quality
// report. Partial success is NOT an error: when the AI returns more candidates
// than needed or some are dropped by the quality gate (e.g. category caps), the
// valid candidates are kept and returned. An error is returned only when the
// provider request fails, the response cannot be parsed, or zero valid
// candidates remain — in which case the generation service still continues to its
// smart fallback rather than failing the job.
func (g *AIGenerator) GenerateDailyQuestsReport(ctx context.Context, qctx *UserQuestContext) ([]models.Quest, AIGenerationReport, error) {
	quests, report, err := g.generate(ctx, qctx)
	return quests, report, err
}

func (g *AIGenerator) generate(ctx context.Context, qctx *UserQuestContext) ([]models.Quest, AIGenerationReport, error) {
	var report AIGenerationReport
	if g.aiClient == nil {
		return nil, report, fmt.Errorf("AI client is not configured")
	}

	if qctx == nil {
		return nil, report, fmt.Errorf("UserQuestContext cannot be nil")
	}

	// Step 1: Build prompt from context
	systemPrompt, userPrompt, err := g.promptBuilder.BuildDailyQuestPrompt(qctx)
	if err != nil {
		return nil, report, fmt.Errorf("failed to build prompt: %w", err)
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
		return nil, report, fmt.Errorf("AI call failed: %w", err)
	}
	latency := time.Since(startTime)

	if aiResp == nil {
		return nil, report, fmt.Errorf("AI response is nil")
	}

	if aiResp.Text == "" {
		return nil, report, fmt.Errorf("AI response text is empty")
	}

	// Step 3: Parse AI response
	plan := BuildQuestCompositionPlan(qctx)
	expectedCount := plan.NeededCount
	if expectedCount <= 0 {
		return nil, report, fmt.Errorf("insufficient_ai_output: needed_count is 0, AI should not be called")
	}

	requestedLimitStr := "nil"
	if qctx.RequestedPreviewLimit != nil {
		requestedLimitStr = fmt.Sprintf("%d", *qctx.RequestedPreviewLimit)
	}

	candidateResp, err := ParseQuestCandidateResponse(aiResp.Text)
	if err != nil {
		logger.L.Warn("AI response parse failed; generation service will fill via smart fallback",
			zap.String("user_id", qctx.UserID.String()),
			zap.Int("needed_count", expectedCount),
			zap.String("raw_response_snippet", truncateStr(aiResp.Text, 500)),
			zap.Error(err),
		)
		fmt.Printf("[AIGenerator] Metadata: daily_quest_count=%d, requested_preview_limit=%s, effective_preview_limit=%d, generated_candidate_count=0\n",
			qctx.DailyQuestCount, requestedLimitStr, expectedCount)
		if errors.Is(err, ErrNoCandidates) {
			return nil, report, fmt.Errorf("insufficient_ai_output: ai returned empty quests, needed_count=%d: %w", expectedCount, err)
		}
		return nil, report, fmt.Errorf("failed to parse AI response: %w", err)
	}

	candidateCount := 0
	if candidateResp != nil {
		candidateCount = len(candidateResp.Quests)
	}
	report.CandidateCount = candidateCount

	// Log safe metadata
	fmt.Printf("[AIGenerator] Metadata: daily_quest_count=%d, requested_preview_limit=%s, effective_preview_limit=%d, generated_candidate_count=%d\n",
		qctx.DailyQuestCount, requestedLimitStr, expectedCount, candidateCount)

	if candidateResp == nil || len(candidateResp.Quests) == 0 {
		return nil, report, fmt.Errorf("insufficient_ai_output: ai returned empty quests, needed_count=%d", expectedCount)
	}

	// Step 4: Normalize candidates (reminder times, sleep crossing midnight, tags).
	// This places normal quests in a safe future slot before the daily cutoff and
	// derives sleep/review reminders from settings.
	now := time.Now().In(timeutil.LocationVN)
	candidateResp.Quests = NormalizeCandidates(qctx, candidateResp.Quests, now)

	// Step 5: Per-candidate quality gate. Repairs minor issues and drops bad
	// candidates individually instead of failing the whole batch. Partial success
	// is fine: the generation service keeps these valid quests and fills any
	// shortfall with the smart fallback. The batch only "fails" (returns an error)
	// when nothing usable remains.
	validCandidates, gateReport := RepairAndValidateCandidates(qctx, candidateResp.Quests, now)

	// Enforce the per-day target/max as a soft cap by truncating rather than
	// failing the batch when AI returns more than requested.
	truncated := 0
	if expectedCount > 0 && len(validCandidates) > expectedCount {
		truncated = len(validCandidates) - expectedCount
		validCandidates = validCandidates[:expectedCount]
	}

	report.KeptCount = len(validCandidates)
	report.DroppedCount = gateReport.DroppedCount + truncated
	report.TopDropReasons = gateReport.topDropReasons(5)

	fmt.Printf("[AIGenerator] QualityGate: raw=%d, kept=%d, repaired=%d, dropped=%d, truncated=%d, top_drop_reasons=[%s]\n",
		gateReport.RawCount, len(validCandidates), gateReport.RepairedCount, gateReport.DroppedCount, truncated, gateReport.topDropReasons(5))

	if len(validCandidates) == 0 {
		return nil, report, fmt.Errorf("insufficient_ai_output: no valid quest candidates after repair/validation (needed_count=%d raw=%d dropped=%d)", expectedCount, gateReport.RawCount, gateReport.DroppedCount)
	}

	// Step 6: Map candidates to Quest models. Returning fewer than expectedCount
	// is intentional — the generation service fills the remainder.
	quests, err := MapCandidatesToQuests(qctx, validCandidates)
	if err != nil {
		return nil, report, fmt.Errorf("failed to map candidates to quests: %w", err)
	}

	// Log summary (without exposing raw prompt/response)
	logAIGenerationSummary(aiResp.Model, len(quests), latency)

	return quests, report, nil
}

func logAIGenerationSummary(model string, questCount int, latency time.Duration) {
	fmt.Printf("[AIGenerator] model=%s, quests=%d, latency=%dms\n", model, questCount, latency.Milliseconds())
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}
