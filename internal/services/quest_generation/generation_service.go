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
}

func NewGenerationService(
	db *gorm.DB,
	contextBuilder *UserQuestContextBuilder,
	aiGenerator Generator,
	ruleGenerator Generator,
) *GenerationService {
	return &GenerationService{
		db:             db,
		contextBuilder: contextBuilder,
		aiGenerator:    aiGenerator,
		ruleGenerator:  ruleGenerator,
	}
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
		Order("created_at ASC").
		Find(&existingQuests).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to load existing quests: %w", err)
	}

	// 3. If existing quests exist and force=false: return existing
	if !force && len(existingQuests) > 0 {
		tx.Rollback()
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
		generatedQuests, err = s.aiGenerator.GenerateDailyQuests(ctx, qctx)
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
