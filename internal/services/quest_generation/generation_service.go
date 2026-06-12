package quest_generation

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
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
	Date               *string   `json:"date"`
	PreferAI           *bool     `json:"prefer_ai"`
	Force              *bool     `json:"force"`
	ReplacePendingOnly *bool     `json:"replace_pending_only"`
	RequestUserID      uuid.UUID `json:"-"`
	AuthSource         string    `json:"-"`
}

type GenerateTodayResult struct {
	Date                 string         `json:"date"`
	TargetCount          int            `json:"target_count"`
	ExistingCount        int            `json:"existing_count"`
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
	result, err := s.generateTodayInternal(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	if invErr := validateGenerationInvariants(result, req); invErr != nil {
		logger.L.Error("Quest generation invariant failed",
			zap.String("user_id", userID.String()),
			zap.Error(invErr),
		)
		appEnv := os.Getenv("APP_ENV")
		if appEnv == "" {
			appEnv = "development"
		}
		if appEnv == "development" || appEnv == "test" {
			return nil, fmt.Errorf("quest generation invariant failure: %w", invErr)
		}
	}

	return result, nil
}

func validateGenerationInvariants(result *GenerateTodayResult, req GenerateTodayRequest) error {
	if result.TargetCount <= 0 {
		return fmt.Errorf("target_count must be > 0 (got %d)", result.TargetCount)
	}
	if result.ExistingCount != len(result.Quests) {
		return fmt.Errorf("existing_count (%d) does not match len(quests) (%d)", result.ExistingCount, len(result.Quests))
	}
	if result.GeneratedCount < 0 {
		return fmt.Errorf("generated_count must be >= 0 (got %d)", result.GeneratedCount)
	}
	if result.PreservedCount < 0 {
		return fmt.Errorf("preserved_count must be >= 0 (got %d)", result.PreservedCount)
	}
	if result.ExistingReturned && result.GeneratedCount != 0 {
		return fmt.Errorf("if existing_returned is true then generated_count must be 0 (got %d)", result.GeneratedCount)
	}
	if result.GeneratedCount == 0 && result.ExistingCount < result.TargetCount {
		return fmt.Errorf("%s: generated_count=0 existing_count=%d target_count=%d",
			models.QuestGenJobErrorNoValidQuestsGenerated,
			result.ExistingCount,
			result.TargetCount,
		)
	}

	isForceNoPendingSlots := result.GeneratedCount == 0 && result.PreservedCount > 0 && req.Force != nil && *req.Force
	willHaveAlreadyExistMessage := result.ExistingReturned && !isForceNoPendingSlots

	if willHaveAlreadyExistMessage && result.ExistingCount < result.TargetCount {
		return fmt.Errorf("message contains 'already exist' but existing_count (%d) < target_count (%d)", result.ExistingCount, result.TargetCount)
	}
	return nil
}

func (s *GenerationService) generateTodayInternal(
	ctx context.Context,
	userID uuid.UUID,
	req GenerateTodayRequest,
) (*GenerateTodayResult, error) {
	// 1. Resolve local date
	var localDate time.Time
	if req.Date == nil || *req.Date == "" {
		// Use user timezone if available, otherwise default to VN
		var settings models.AppSettings
		err := s.db.Where("user_id = ?", userID).First(&settings).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("failed to load user settings: %w", err)
		}

		userLoc := timeutil.LocationVN
		if err == nil && settings.Timezone != "" {
			if loc, err := time.LoadLocation(settings.Timezone); err == nil {
				userLoc = loc
			}
		}
		now := time.Now().In(userLoc)
		localDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, userLoc)
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

	// Acquire advisory lock on PostgreSQL to prevent concurrent generation.
	// Skipped on non-PostgreSQL dialects (e.g. SQLite used in unit tests).
	// pg_advisory_xact_lock() returns void, so the statement is executed (not
	// scanned) and the lock is held for the lifetime of this transaction.
	dialect := s.db.Dialector.Name()
	if dialect == "postgres" {
		lockKey := advisoryLockKey(userID, dateStr)
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", lockKey).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to acquire advisory lock: %w", err)
		}
		logger.L.Info("Advisory lock acquired for quest generation",
			zap.String("user_id", userID.String()),
			zap.String("date", dateStr),
			zap.Int64("lock_key", lockKey),
			zap.String("db_dialect", dialect),
			zap.Bool("advisory_lock_acquired", true),
		)
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

	// 4. Build UserQuestContext to get target count
	txBuilder := NewUserQuestContextBuilder(tx)
	qctx, err := txBuilder.Build(ctx, userID, localDate)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to build quest context: %w", err)
	}

	// Debug: Log roadmap context loading
	roadmapLoaded := false
	var roadmapID, stepID string
	if qctx.ActiveLearningPath != nil {
		roadmapLoaded = true
		roadmapID = qctx.ActiveLearningPath.RoadmapID
		stepID = qctx.ActiveLearningPath.StepID
		logger.L.Info("Active learning roadmap loaded for quest generation",
			zap.String("user_id", userID.String()),
			zap.String("roadmap_id", roadmapID),
			zap.String("roadmap_title", qctx.ActiveLearningPath.RoadmapTitle),
			zap.String("current_step_id", stepID),
			zap.String("current_step_title", qctx.ActiveLearningPath.CurrentStepTitle),
			zap.Int("completed_steps", qctx.ActiveLearningPath.CompletedSteps),
			zap.Int("total_steps", qctx.ActiveLearningPath.TotalSteps),
		)
	} else {
		logger.L.Info("No active learning roadmap found",
			zap.String("user_id", userID.String()),
		)
	}

	var settings models.QuestSettings
	settingsFound := true
	err = tx.Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			settingsFound = false
			// Create default settings in DB
			defaultSettings := buildDefaultQuestSettings(userID)
			if err := tx.Create(defaultSettings).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create default quest settings: %w", err)
			}
			settings = *defaultSettings
		} else {
			tx.Rollback()
			return nil, fmt.Errorf("failed to load user settings: %w", err)
		}
	}

	rawDailyQuestCount := settings.DailyQuestCount
	resolvedTargetCount := rawDailyQuestCount
	if resolvedTargetCount <= 0 {
		resolvedTargetCount = 6 // fallback to default daily quest count
	}

	logger.L.Info("Resolved target daily quest count",
		zap.String("user_id", userID.String()),
		zap.String("date", dateStr),
		zap.Bool("settings_found", settingsFound),
		zap.Int("raw_daily_quest_count", rawDailyQuestCount),
		zap.Int("resolved_target_count", resolvedTargetCount),
	)

	targetCount := resolvedTargetCount
	qctx.DailyQuestCount = targetCount
	preservedCount := len(preservedQuests)
	existingTotal := len(existingQuests)

	// 3. If force=false and already at/above target: return existing
	if !force && existingTotal >= targetCount {
		tx.Rollback()
		models.SortQuests(existingQuests)
		return &GenerateTodayResult{
			Date:                 dateStr,
			TargetCount:          targetCount,
			ExistingCount:        len(existingQuests),
			Inserted:             false,
			ExistingReturned:     true,
			Source:               "existing",
			FallbackUsed:         false,
			GeneratedCount:       0,
			PreservedCount:       len(existingQuests),
			ReplacedPendingCount: 0,
			Quests:               existingQuests,
		}, nil
	}

	// Delete pending quests if force=true (before calculating capacity)
	replacedPendingCount := 0
	if force && len(pendingQuests) > 0 {
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

	// Calculate remaining capacity
	// If force=false, pending quests count toward existing capacity
	existingCountForCapacity := preservedCount
	if !force {
		existingCountForCapacity += len(pendingQuests)
	}
	remainingCapacity := targetCount - existingCountForCapacity
	if remainingCapacity <= 0 {
		tx.Commit()
		// Return all preserved + pending (if not deleted)
		returnQuests := append([]models.Quest{}, preservedQuests...)
		if !force {
			returnQuests = append(returnQuests, pendingQuests...)
		}
		models.SortQuests(returnQuests)
		return &GenerateTodayResult{
			Date:                 dateStr,
			TargetCount:          targetCount,
			ExistingCount:        len(returnQuests),
			Inserted:             false,
			ExistingReturned:     true,
			Source:               "existing",
			FallbackUsed:         false,
			GeneratedCount:       0,
			PreservedCount:       len(returnQuests),
			ReplacedPendingCount: replacedPendingCount,
			Quests:               returnQuests,
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

	// Per-type caps, duplicate-title avoidance, and reminder-time allocation must
	// be computed against the quests that will actually COEXIST with the newly
	// generated ones — not the quests being replaced. The context builder counted
	// every existing quest (including the pending ones just deleted under
	// force=true), so recompute those fields from the surviving set here.
	survivingExisting := append([]models.Quest{}, preservedQuests...)
	if !force {
		survivingExisting = append(survivingExisting, pendingQuests...)
	}
	qctx.ExistingQuestTypeCount = existingTypeCountFrom(survivingExisting)
	qctx.ExistingQuestTitles = existingTitlesFrom(survivingExisting)
	qctx.ExistingReminderTimes = existingReminderTimesFrom(survivingExisting)

	// Set generation capacity constraints on context
	qctx.DailyQuestCount = remainingCapacity
	qctx.PreviewLimit = remainingCapacity

	// 5. Generate quests. AI partial success is never fatal: valid AI quests are
	// kept and any shortfall is filled by the smart fallback (and, if still short,
	// the deterministic last-resort fill). The job only fails on DB save errors or
	// when a quest's scheduled time cannot be constructed.
	plan := BuildQuestCompositionPlan(qctx)
	neededCount := plan.NeededCount
	if neededCount <= 0 {
		neededCount = remainingCapacity
	}
	now := time.Now().In(timeutil.LocationVN)

	// persist inserts service-built quests (AI candidates, mock-generator output,
	// and fallback quests) into the open transaction.
	persist := func(quests []models.Quest) error {
		for i := range quests {
			quests[i].UserID = userID
			if quests[i].ID == uuid.Nil {
				quests[i].ID = uuid.New()
			}
			if err := tx.Create(&quests[i]).Error; err != nil {
				return err
			}
		}
		return nil
	}

	var generatedQuests []models.Quest
	var genSource string
	var fallbackUsed bool
	var aiErrorType string

	var aiCandidateCount, aiKeptCount, aiDroppedCount int
	var topDropReasons string

	runAI := preferAI && s.aiGenerator != nil

	if runAI {
		// The AI call runs on its own deadline derived from the caller's context.
		// When it expires, only aiCtx is cancelled — the parent ctx (and the DB
		// transaction bound to it) stay alive, so the fallback fill and DB save
		// below do not inherit a cancelled context.
		aiTimeout := s.aiCallTimeout
		if aiTimeout <= 0 {
			aiTimeout = AICallTimeout
		}
		aiCtx, aiCancel := context.WithTimeout(ctx, aiTimeout)
		var aiQuests []models.Quest
		var aiErr error
		if reporter, ok := s.aiGenerator.(interface {
			GenerateDailyQuestsReport(context.Context, *UserQuestContext) ([]models.Quest, AIGenerationReport, error)
		}); ok {
			var rep AIGenerationReport
			aiQuests, rep, aiErr = reporter.GenerateDailyQuestsReport(aiCtx, qctx)
			aiCandidateCount = rep.CandidateCount
			aiDroppedCount = rep.DroppedCount
			topDropReasons = rep.TopDropReasons
		} else {
			aiQuests, aiErr = s.aiGenerator.GenerateDailyQuests(aiCtx, qctx)
			aiCandidateCount = len(aiQuests)
		}
		aiCancel()

		if aiErr != nil {
			// Parse/provider failure or zero valid candidates. Not fatal — the
			// smart fallback fills the day below.
			aiErrorType = mapAIErrorType(aiErr)
			fallbackUsed = true
			logger.L.Warn("AI quest generation partially accepted; filling missing quests",
				zap.String("user_id", userID.String()),
				zap.String("date", dateStr),
				zap.String("ai_error_type", aiErrorType),
				zap.Error(aiErr),
			)
		} else if len(aiQuests) > 0 {
			if err := persist(aiQuests); err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to insert AI generated quest: %w", err)
			}
			generatedQuests = aiQuests
		}
		aiKeptCount = len(generatedQuests)
	} else {
		if preferAI && s.aiGenerator == nil {
			fallbackUsed = true
			aiErrorType = "provider_error"
			logger.L.Warn("AI generator is not configured/enabled, using rule-based then smart fallback",
				zap.String("user_id", userID.String()),
				zap.String("date", dateStr),
			)
		}

		// Rule-based primary path (preferAI=false or AI unavailable). Failure is
		// not fatal — the smart fallback fills the day.
		var ruleGen Generator = s.ruleGenerator
		ruleAlreadyPersisted := false
		if _, ok := s.ruleGenerator.(*RuleBasedGenerator); ok {
			ruleGen = NewRuleBasedGenerator(tx)
			ruleAlreadyPersisted = true
		}
		ruleQuests, ruleErr := ruleGen.GenerateDailyQuests(ctx, qctx)
		if ruleErr != nil {
			fallbackUsed = true
			logger.L.Warn("rule-based generation produced no quests; filling via smart fallback",
				zap.String("user_id", userID.String()),
				zap.String("date", dateStr),
				zap.Error(ruleErr),
			)
		} else if len(ruleQuests) > 0 {
			if !ruleAlreadyPersisted {
				if err := persist(ruleQuests); err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("failed to insert rule-based quest: %w", err)
				}
			}
			generatedQuests = ruleQuests
		}
	}

	if len(generatedQuests) > 0 && runAI {
		genSource = models.QuestGenJobSourceAI
	} else {
		genSource = models.QuestGenJobSourceRuleBased
	}

	// 6. Fill any shortfall: smart fallback first (context-aware, cap-respecting),
	// then deterministic last-resort fill so exactly neededCount quests are saved.
	missingAfterPrimary := neededCount - len(generatedQuests)
	missingAfterAI := missingAfterPrimary
	smartFallbackCount := 0
	lastResortCount := 0

	if missingAfterPrimary > 0 {
		smart, err := BuildSmartFallbackQuests(qctx, plan, generatedQuests, missingAfterPrimary, now)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("smart fallback failed: %w", err)
		}
		if len(smart) > 0 {
			if err := persist(smart); err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to insert smart fallback quest: %w", err)
			}
			generatedQuests = append(generatedQuests, smart...)
			smartFallbackCount = len(smart)
			fallbackUsed = true
		}

		stillMissing := neededCount - len(generatedQuests)
		if stillMissing > 0 {
			lastResort, err := BuildLastResortFill(qctx, plan, generatedQuests, stillMissing, now)
			if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("last-resort fill failed: %w", err)
			}
			if len(lastResort) > 0 {
				if err := persist(lastResort); err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("failed to insert last-resort quest: %w", err)
				}
				generatedQuests = append(generatedQuests, lastResort...)
				lastResortCount = len(lastResort)
				fallbackUsed = true
			}
		}
	}

	postGenerationExistingCount := existingCountForCapacity + len(generatedQuests)
	if len(generatedQuests) == 0 && postGenerationExistingCount < targetCount {
		tx.Rollback()
		return nil, fmt.Errorf("%s: generated_count=0 existing_count=%d target_count=%d",
			models.QuestGenJobErrorNoValidQuestsGenerated,
			postGenerationExistingCount,
			targetCount,
		)
	}

	// Debug: Count learning quests and roadmap linkage
	learningQuestCount := 0
	learningQuestLinkedCount := 0
	for _, q := range generatedQuests {
		if q.Type == models.QuestTypeLearning {
			learningQuestCount++
			if len(q.LearningMetadata) > 0 {
				learningQuestLinkedCount++
			}
		}
	}
	logger.L.Info("Quest generation completed",
		zap.String("user_id", userID.String()),
		zap.String("date", dateStr),
		zap.String("source", genSource),
		zap.Int("target_count", targetCount),
		zap.Int("existing_count", existingCountForCapacity),
		zap.Int("needed_count", neededCount),
		zap.Int("composition_plan_slots", len(plan.Slots)),
		zap.Int("ai_candidate_count", aiCandidateCount),
		zap.Int("ai_kept_count", aiKeptCount),
		zap.Int("ai_dropped_count", aiDroppedCount),
		zap.String("top_drop_reasons", topDropReasons),
		zap.Int("missing_after_ai", missingAfterAI),
		zap.Int("smart_fallback_count", smartFallbackCount),
		zap.Int("last_resort_count", lastResortCount),
		zap.Int("final_saved_count", len(generatedQuests)),
		zap.Int("generated_count", len(generatedQuests)),
		zap.Int("learning_quest_count", learningQuestCount),
		zap.Int("learning_quest_linked_count", learningQuestLinkedCount),
		zap.Bool("roadmap_context_loaded", roadmapLoaded),
		zap.String("roadmap_id", roadmapID),
		zap.String("current_step_id", stepID),
	)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 6. Combine preserved, pending (if not deleted), and generated quests
	allQuests := append([]models.Quest{}, preservedQuests...)
	if !force {
		allQuests = append(allQuests, pendingQuests...)
	}
	allQuests = append(allQuests, generatedQuests...)
	models.SortQuests(allQuests)

	// Count of existing quests that remain
	preservedTotal := preservedCount
	if !force {
		preservedTotal += len(pendingQuests)
	}

	return &GenerateTodayResult{
		Date:                 dateStr,
		TargetCount:          targetCount,
		ExistingCount:        len(allQuests),
		Inserted:             true,
		ExistingReturned:     false,
		Source:               genSource,
		FallbackUsed:         fallbackUsed,
		AIErrorType:          aiErrorType,
		GeneratedCount:       len(generatedQuests),
		PreservedCount:       preservedTotal,
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

// existingTypeCountFrom counts quests by raw quest type for cap enforcement.
func existingTypeCountFrom(quests []models.Quest) map[string]int {
	counts := make(map[string]int, len(quests))
	for _, q := range quests {
		counts[string(q.Type)]++
	}
	return counts
}

// existingTitlesFrom collects quest titles for duplicate-title avoidance.
func existingTitlesFrom(quests []models.Quest) []string {
	titles := make([]string, 0, len(quests))
	for _, q := range quests {
		titles = append(titles, q.Title)
	}
	return titles
}

// existingReminderTimesFrom collects reminder times to seed the fallback
// reminder allocator so generated quests don't reuse an occupied slot.
func existingReminderTimesFrom(quests []models.Quest) []time.Time {
	var times []time.Time
	for _, q := range quests {
		if q.ReminderTime != nil {
			times = append(times, *q.ReminderTime)
		}
	}
	return times
}

// advisoryLockKey computes a PostgreSQL advisory lock key from userID + date.
// Uses FNV-1a hash to convert to int64 for pg_advisory_xact_lock.
func advisoryLockKey(userID uuid.UUID, date string) int64 {
	h := fnv.New64a()
	h.Write(userID[:])
	h.Write([]byte(date))
	return int64(h.Sum64())
}
