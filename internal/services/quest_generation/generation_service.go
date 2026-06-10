package quest_generation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/questplan"
	"solo_quest_backend/pkg/logger"
)

type GenerateTodayRequest struct {
	Date               *string `json:"date"`
	PreferAI           *bool   `json:"prefer_ai"`
	Force              *bool   `json:"force"`
	ReplacePendingOnly *bool   `json:"replace_pending_only"`
}

type GenerateTodayResult struct {
	Date                 string         `json:"date"`
	Inserted             bool           `json:"inserted"`
	ExistingReturned     bool           `json:"existing_returned"`
	Source               string         `json:"source"`
	FallbackUsed         bool           `json:"fallback_used"`
	AIErrorType          string         `json:"ai_error_type,omitempty"`
	GeneratedCount       int            `json:"generated_count"`
	PreservedCount       int            `json:"preserved_count"`
	ReplacedPendingCount int            `json:"replaced_pending_count"`
	Quests               []models.Quest `json:"quests"`
}

type GenerationService struct {
	db             *gorm.DB
	contextBuilder *UserQuestContextBuilder
	aiGenerator    Generator
	ruleGenerator  Generator

	// launchWorker starts the background generation worker for a job.
	// Defaults to spawning a recovered goroutine; tests may override it to
	// run synchronously or to capture invocations.
	launchWorker func(jobID uuid.UUID)

	// aiCallTimeout bounds a single AI provider call, derived from the
	// caller's context. Defaults to AICallTimeout; tests may shorten it.
	aiCallTimeout time.Duration
}

func NewGenerationService(
	db *gorm.DB,
	contextBuilder *UserQuestContextBuilder,
	aiGenerator Generator,
	ruleGenerator Generator,
) *GenerationService {
	s := &GenerationService{
		db:             db,
		contextBuilder: contextBuilder,
		aiGenerator:    aiGenerator,
		ruleGenerator:  ruleGenerator,
		aiCallTimeout:  AICallTimeout,
	}
	s.launchWorker = func(jobID uuid.UUID) {
		go s.ProcessJob(context.Background(), jobID)
	}
	return s
}

// SetWorkerLauncher overrides how background workers are launched. Intended
// for tests that need to run the worker synchronously or assert it was
// scheduled exactly once.
func (s *GenerationService) SetWorkerLauncher(fn func(jobID uuid.UUID)) {
	s.launchWorker = fn
}

// SetAICallTimeout overrides the per-AI-call timeout. Intended for tests that
// need to exercise the AI-timeout/fallback path quickly without waiting for the
// production 600s deadline.
func (s *GenerationService) SetAICallTimeout(d time.Duration) {
	s.aiCallTimeout = d
}

func (s *GenerationService) GenerateToday(
	ctx context.Context,
	userID uuid.UUID,
	req GenerateTodayRequest,
) (*GenerateTodayResult, error) {
	// 1. Resolve local date
	var localDate time.Time
	if req.Date == nil || *req.Date == "" {
		localDate = timeutil.TodayVN()
	} else {
		parsedDate, err := timeutil.ParseDateVN(*req.Date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %w", err)
		}
		localDate = parsedDate
	}
	dateStr := timeutil.FormatDateVN(localDate)

	// Resolve defaults
	preferAI := true
	if req.PreferAI != nil {
		preferAI = *req.PreferAI
	}
	force := false
	if req.Force != nil {
		force = *req.Force
	}
	replacePendingOnly := true
	if req.ReplacePendingOnly != nil {
		replacePendingOnly = *req.ReplacePendingOnly
	}

	// Reject replace_pending_only=false for now
	if !replacePendingOnly {
		return nil, fmt.Errorf("replace_pending_only=false is not supported")
	}

	// Start a database transaction
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// 2. Load existing quests for user/date
	var existingQuests []models.Quest
	start, end := timeutil.DayRangeVN(localDate)
	if err := tx.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Order("reminder_time IS NULL ASC").
		Order("reminder_time ASC").
		Order("created_at ASC").
		Order("id ASC").
		Find(&existingQuests).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to load existing quests: %w", err)
	}

	// 3. If existing quests exist and force=false: return existing
	if !force && len(existingQuests) > 0 {
		tx.Rollback()
		models.SortQuests(existingQuests)
		return &GenerateTodayResult{
			Date:             dateStr,
			Inserted:         false,
			ExistingReturned: true,
			Source:           "existing",
			FallbackUsed:     false,
			GeneratedCount:   0,
			PreservedCount:   len(existingQuests),
			Quests:           existingQuests,
		}, nil
	}

	// Separate pending from preserved quests
	var pendingQuests []models.Quest
	var preservedQuests []models.Quest
	for _, q := range existingQuests {
		if q.Status == models.QuestStatusPending {
			pendingQuests = append(pendingQuests, q)
		} else {
			preservedQuests = append(preservedQuests, q)
		}
	}

	replacedPendingCount := 0
	if force && len(pendingQuests) > 0 {
		// Delete only pending quests
		var pendingIDs []uuid.UUID
		for _, pq := range pendingQuests {
			pendingIDs = append(pendingIDs, pq.ID)
		}
		if err := tx.Where("id IN ?", pendingIDs).Delete(&models.Quest{}).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete pending quests: %w", err)
		}
		replacedPendingCount = len(pendingQuests)
	}

	// 4. Build UserQuestContext from real DB data
	txBuilder := NewUserQuestContextBuilder(tx)
	qctx, err := txBuilder.Build(ctx, userID, localDate)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to build quest context: %w", err)
	}

	// Capacity check / calculation
	preservedCount := len(preservedQuests)
	remainingCapacity := qctx.DailyQuestCount - preservedCount

	// If remaining capacity <= 0: return preserved existing quests
	if remainingCapacity <= 0 {
		if err := tx.Commit().Error; err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
		return &GenerateTodayResult{
			Date:                 dateStr,
			Inserted:             false,
			ExistingReturned:     true,
			Source:               "existing",
			FallbackUsed:         false,
			GeneratedCount:       0,
			PreservedCount:       preservedCount,
			ReplacedPendingCount: replacedPendingCount,
			Quests:               preservedQuests,
		}, nil
	}

	// Validate onboarding completion for AI mode
	if preferAI {
		if len(qctx.EnabledCategories) == 0 {
			tx.Rollback()
			return nil, fmt.Errorf("no quest categories enabled, please complete onboarding")
		}
		if qctx.DailyQuestCount <= 0 {
			tx.Rollback()
			return nil, fmt.Errorf("daily quest count is not set, please complete onboarding")
		}
	}

	// Set generation capacity constraints on context
	qctx.DailyQuestCount = remainingCapacity
	qctx.PreviewLimit = remainingCapacity

	// 5. Generate quests using AIGenerator or RuleBasedGenerator
	var generatedQuests []models.Quest
	var genSource string
	var fallbackUsed bool
	var aiErrorType string

	runAI := preferAI && s.aiGenerator != nil

	if runAI {
		var err error
		// The AI call runs on its own deadline derived from the caller's
		// context. When it expires, only aiCtx is cancelled — the parent
		// ctx (and the DB transaction bound to it) stay alive, so the
		// rule-based fallback and DB save below do not inherit a cancelled
		// context.
		aiTimeout := s.aiCallTimeout
		if aiTimeout <= 0 {
			aiTimeout = AICallTimeout
		}
		aiCtx, aiCancel := context.WithTimeout(ctx, aiTimeout)
		generatedQuests, err = s.aiGenerator.GenerateDailyQuests(aiCtx, qctx)
		aiCancel()
		if err != nil {
			fallbackUsed = true
			aiErrorType = mapAIErrorType(err)
			logger.L.Warn("AI quest generation failed, falling back to rule-based",
				zap.String("user_id", userID.String()),
				zap.String("date", dateStr),
				zap.String("ai_error_type", aiErrorType),
				zap.Error(err),
			)

			// Fallback: use transaction rule generator if real, else mock
			var ruleGen Generator = s.ruleGenerator
			isRealRuleGen := false
			if _, ok := s.ruleGenerator.(*RuleBasedGenerator); ok {
				ruleGen = NewRuleBasedGenerator(tx)
				isRealRuleGen = true
			}
			generatedQuests, err = ruleGen.GenerateDailyQuests(ctx, qctx)
			if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("fallback rule-based generation failed: %w", err)
			}
			genSource = "rule_based"

			if !isRealRuleGen && len(generatedQuests) > 0 {
				for i := range generatedQuests {
					generatedQuests[i].UserID = userID
					if generatedQuests[i].ID == uuid.Nil {
						generatedQuests[i].ID = uuid.New()
					}
					if err := tx.Create(&generatedQuests[i]).Error; err != nil {
						tx.Rollback()
						return nil, fmt.Errorf("failed to insert fallback quest: %w", err)
					}
				}
			}
		} else {
			genSource = "ai"
			// AIGenerator returns quests without DB insertion, insert them here
			if len(generatedQuests) > 0 {
				for i := range generatedQuests {
					generatedQuests[i].UserID = userID
					if generatedQuests[i].ID == uuid.Nil {
						generatedQuests[i].ID = uuid.New()
					}
					if err := tx.Create(&generatedQuests[i]).Error; err != nil {
						tx.Rollback()
						return nil, fmt.Errorf("failed to insert AI generated quest: %w", err)
					}
				}
			}

			// Top-up with rule-based quests when AI returned fewer than the
			// remaining capacity (partial AI result). Rule-based selection
			// respects max_per_day and avoids duplicate titles, so it
			// naturally stops at the effective per-day target.
			shortfall := remainingCapacity - len(generatedQuests)
			if shortfall > 0 {
				topUp, topErr := s.generateRuleBasedTopUp(ctx, tx, userID, localDate, shortfall)
				if topErr != nil {
					tx.Rollback()
					return nil, fmt.Errorf("rule-based top-up failed: %w", topErr)
				}
				if len(topUp) > 0 {
					kept, removed := deduplicateTopUp(generatedQuests, topUp)
					if len(removed) > 0 {
						var removedIDs []uuid.UUID
						for _, q := range removed {
							if q.ID != uuid.Nil {
								removedIDs = append(removedIDs, q.ID)
							}
						}
						if len(removedIDs) > 0 {
							if err := tx.WithContext(ctx).Where("id IN ?", removedIDs).Delete(&models.Quest{}).Error; err != nil {
								tx.Rollback()
								return nil, fmt.Errorf("failed to remove duplicate top-up quests: %w", err)
							}
						}
						logger.L.Info("deduped top-up quests",
							zap.String("user_id", userID.String()),
							zap.Int("removed", len(removed)),
							zap.Int("kept", len(kept)),
						)
					}
					topUp = kept
					if len(topUp) > 0 {
						generatedQuests = append(generatedQuests, topUp...)
						fallbackUsed = true
						logger.L.Info("AI result topped up with rule-based quests",
							zap.String("user_id", userID.String()),
							zap.String("date", dateStr),
							zap.Int("ai_count", len(generatedQuests)-len(topUp)),
							zap.Int("topup_count", len(topUp)),
						)
					}
				}
			}
		}
	} else {
		if preferAI && s.aiGenerator == nil {
			fallbackUsed = true
			aiErrorType = "provider_error"
			logger.L.Warn("AI generator is not configured/enabled, falling back to rule-based",
				zap.String("user_id", userID.String()),
				zap.String("date", dateStr),
			)
		}

		// Use transaction rule generator if real, else mock
		var ruleGen Generator = s.ruleGenerator
		isRealRuleGen := false
		if _, ok := s.ruleGenerator.(*RuleBasedGenerator); ok {
			ruleGen = NewRuleBasedGenerator(tx)
			isRealRuleGen = true
		}
		var err error
		generatedQuests, err = ruleGen.GenerateDailyQuests(ctx, qctx)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("rule-based generation failed: %w", err)
		}
		genSource = "rule_based"

		if !isRealRuleGen && len(generatedQuests) > 0 {
			for i := range generatedQuests {
				generatedQuests[i].UserID = userID
				if generatedQuests[i].ID == uuid.Nil {
					generatedQuests[i].ID = uuid.New()
				}
				if err := tx.Create(&generatedQuests[i]).Error; err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("failed to insert rule-based quest: %w", err)
				}
			}
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 6. Combine preserved and generated quests
	allQuests := append([]models.Quest{}, preservedQuests...)
	allQuests = append(allQuests, generatedQuests...)
	models.SortQuests(allQuests)

	return &GenerateTodayResult{
		Date:                 dateStr,
		Inserted:             true,
		ExistingReturned:     false,
		Source:               genSource,
		FallbackUsed:         fallbackUsed,
		AIErrorType:          aiErrorType,
		GeneratedCount:       len(generatedQuests),
		PreservedCount:       preservedCount,
		ReplacedPendingCount: replacedPendingCount,
		Quests:               allQuests,
	}, nil
}

// generateRuleBasedTopUp generates up to `shortfall` additional quests using
// the rule-based generator, within the given transaction. The context is
// rebuilt from the transaction so it reflects quests already inserted in this
// run (preserved + AI), letting rule-based selection respect max_per_day and
// avoid duplicate titles.
func (s *GenerationService) generateRuleBasedTopUp(
	ctx context.Context,
	tx *gorm.DB,
	userID uuid.UUID,
	localDate time.Time,
	shortfall int,
) ([]models.Quest, error) {
	if shortfall <= 0 {
		return nil, nil
	}

	txBuilder := NewUserQuestContextBuilder(tx)
	qctx, err := txBuilder.Build(ctx, userID, localDate)
	if err != nil {
		return nil, fmt.Errorf("failed to build top-up context: %w", err)
	}
	qctx.DailyQuestCount = shortfall
	qctx.PreviewLimit = shortfall

	var ruleGen Generator = s.ruleGenerator
	isRealRuleGen := false
	if _, ok := s.ruleGenerator.(*RuleBasedGenerator); ok {
		ruleGen = NewRuleBasedGenerator(tx)
		isRealRuleGen = true
	}

	topUp, err := ruleGen.GenerateDailyQuests(ctx, qctx)
	if err != nil {
		return nil, err
	}
	if len(topUp) > shortfall {
		topUp = topUp[:shortfall]
	}

	if !isRealRuleGen {
		for i := range topUp {
			topUp[i].UserID = userID
			if topUp[i].ID == uuid.Nil {
				topUp[i].ID = uuid.New()
			}
			if err := tx.Create(&topUp[i]).Error; err != nil {
				return nil, fmt.Errorf("failed to insert top-up quest: %w", err)
			}
		}
	}

	return topUp, nil
}

// ---------------------------------------------------------------------------
// Semantic dedup helpers
// ---------------------------------------------------------------------------

func bridgeToQuestplanCandidate(q models.Quest) questplan.Candidate {
	src := questplan.SourceRuleBased
	if q.Source == models.QuestSourceAI {
		src = questplan.SourceAI
	}
	stepID := ""
	if meta, ok := ParseLearningMetadata(q); ok {
		stepID = meta.LearningStepID
	}
	return questplan.Candidate{
		Title:         q.Title,
		Description:   q.Description,
		Type:          questplan.QuestType(string(q.Type)),
		Source:        src,
		RoadmapStepID: stepID,
	}
}

// deduplicateTopUp filters topUp quests that are semantically equivalent to
// existing AI quests using questplan.CanonicalKey. Returns kept and removed slices.
func deduplicateTopUp(aiQuests, topUp []models.Quest) (kept, removed []models.Quest) {
	seenKeys := make(map[string]bool, len(aiQuests))
	for _, q := range aiQuests {
		seenKeys[questplan.CanonicalKey(bridgeToQuestplanCandidate(q))] = true
	}
	for _, q := range topUp {
		k := questplan.CanonicalKey(bridgeToQuestplanCandidate(q))
		if seenKeys[k] {
			removed = append(removed, q)
		} else {
			seenKeys[k] = true // prevent duplicates within topUp itself
			kept = append(kept, q)
		}
	}
	return
}

func mapAIErrorType(err error) string {
	if err == nil {
		return ""
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "AI returned") && strings.Contains(errMsg, "expected") {
		return "count_mismatch"
	}
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded") {
		return "timeout"
	}
	if strings.Contains(errMsg, "finish_reason=length") {
		return "provider_error"
	}
	if strings.Contains(errMsg, "content is empty") || strings.Contains(errMsg, "HTTP request failed") || strings.Contains(errMsg, "status code") {
		return "provider_error"
	}
	if strings.Contains(errMsg, "failed to parse") || strings.Contains(errMsg, "JSON") || strings.Contains(errMsg, "unmarshal") {
		return "invalid_json"
	}
	if strings.Contains(errMsg, "candidate validation failed") || strings.Contains(errMsg, "validation failed") {
		return "validation_failed"
	}
	return "unknown"
}
