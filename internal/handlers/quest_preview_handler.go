package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/utils"
)

// Interfaces for testability
type questContextBuilder interface {
	Build(ctx context.Context, userID uuid.UUID, localDate time.Time) (*quest_generation.UserQuestContext, error)
}

type QuestPreviewHandler struct {
	contextBuilder questContextBuilder
	aiGenerator    quest_generation.Generator
	ruleGenerator  quest_generation.Generator
	aiEnabled      bool
}

func NewQuestPreviewHandler(
	contextBuilder questContextBuilder,
	ruleGenerator quest_generation.Generator,
	aiClient ai.Client,
	aiConfig *ai.Config,
	aiEnabled bool,
) *QuestPreviewHandler {
	var aiGen quest_generation.Generator
	if aiClient != nil {
		aiGen = quest_generation.NewAIGeneratorWithConfig(aiClient, aiConfig)
	}

	return &QuestPreviewHandler{
		contextBuilder: contextBuilder,
		aiGenerator:    aiGen,
		ruleGenerator:  ruleGenerator,
		aiEnabled:      aiEnabled,
	}
}

type GeneratePreviewRequest struct {
	Date         string `json:"date"`
	Mode         string `json:"mode"`
	PreviewLimit *int   `json:"preview_limit,omitempty"`
}

type GeneratePreviewResponse struct {
	Date           string                     `json:"date"`
	Mode           string                     `json:"mode"`
	Inserted       bool                       `json:"inserted"`
	GeneratedCount int                        `json:"generated_count"`
	Quests         []dto.QuestPreviewResponse `json:"quests"`
}

func (h *QuestPreviewHandler) GeneratePreview(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req GeneratePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body - use defaults
		req.Date = ""
		req.Mode = "ai"
	}

	// Default mode to "ai"
	if req.Mode == "" {
		req.Mode = "ai"
	}

	// Validate mode
	if req.Mode != "ai" && req.Mode != "rule_based" {
		c.JSON(http.StatusBadRequest, response.BadRequest(fmt.Sprintf("invalid mode '%s', allowed: 'ai', 'rule_based'", req.Mode)))
		return
	}

	// Check AI availability for AI mode
	if req.Mode == "ai" {
		if !h.aiEnabled {
			c.JSON(http.StatusBadRequest, response.BadRequest("AI is not enabled (AI_ENABLED=false)"))
			return
		}
		if h.aiGenerator == nil {
			c.JSON(http.StatusInternalServerError, response.InternalError("AI generator is not configured"))
			return
		}
	}

	// Parse date or use today
	var localDate time.Time
	if req.Date == "" {
		localDate = timeutil.TodayVN()
	} else {
		parsedDate, err := timeutil.ParseDateVN(req.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid date format, use YYYY-MM-DD"))
			return
		}
		localDate = parsedDate
	}
	dateStr := timeutil.FormatDateVN(localDate)

	// Build context from real DB data
	ctx := context.Background()
	qctx, err := h.contextBuilder.Build(ctx, userID, localDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError(fmt.Sprintf("failed to build quest context: %v", err)))
		return
	}

	// Check if user has completed onboarding for AI mode
	if req.Mode == "ai" {
		// For AI mode, we need meaningful user context
		if len(qctx.EnabledCategories) == 0 {
			c.JSON(http.StatusBadRequest, response.BadRequest("no quest categories enabled, please complete onboarding"))
			return
		}
		if qctx.DailyQuestCount <= 0 {
			c.JSON(http.StatusBadRequest, response.BadRequest("daily quest count is not set, please complete onboarding"))
			return
		}
	}

	// Calculate effective preview_limit for AI mode
	effectivePreviewLimit := qctx.DailyQuestCount
	if req.Mode == "ai" {
		if req.PreviewLimit != nil {
			// User provided preview_limit, clamp it
			limit := *req.PreviewLimit
			if limit < 1 {
				c.JSON(http.StatusBadRequest, response.BadRequest("preview_limit must be at least 1"))
				return
			}
			// Clamp to daily_quest_count
			if limit > qctx.DailyQuestCount {
				limit = qctx.DailyQuestCount
			}
			// Hard maximum 10
			if limit > 10 {
				limit = 10
			}
			effectivePreviewLimit = limit
		}
		// Default: use full daily_quest_count (no reduction)
		// User can pass smaller preview_limit to test faster previews

		// Apply preview_limit to context for AI generation
		qctx.PreviewLimit = effectivePreviewLimit
		qctx.RequestedPreviewLimit = req.PreviewLimit

		// Capacity check (Task 4)
		enabledCats := make(map[string]bool)
		for _, cat := range qctx.EnabledCategories {
			enabledCats[cat] = true
		}

		totalCapacity := 0
		for _, rule := range qctx.Rules {
			if !rule.Enabled {
				continue
			}
			if !enabledCats[rule.Type] {
				continue
			}

			// Check time constraints
			isAvailable := true
			if rule.ActiveTimeRange != nil && rule.ActiveTimeRange.Start != "" && rule.ActiveTimeRange.End != "" {
				usableStart := rule.ActiveTimeRange.Start
				usableEnd := rule.ActiveTimeRange.End
				if qctx.QuietAfterTime != "" {
					if usableEnd > qctx.QuietAfterTime {
						usableEnd = qctx.QuietAfterTime
					}
				}
				if usableEnd < usableStart {
					isAvailable = false
				}
			}

			if !isAvailable {
				continue
			}

			if rule.MaxPerDay != nil {
				totalCapacity += *rule.MaxPerDay
			} else {
				totalCapacity += 999
			}
		}

		if effectivePreviewLimit > totalCapacity {
			c.JSON(http.StatusUnprocessableEntity, response.Error(http.StatusUnprocessableEntity, "AI preview cannot satisfy preview_limit with current enabled rules and time constraints."))
			return
		}
	}

	// Generate quests using selected mode
	var generator quest_generation.Generator

	switch req.Mode {
	case "ai":
		generator = h.aiGenerator
	case "rule_based":
		generator = h.ruleGenerator
	default:
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid mode"))
		return
	}

	generatedQuests, err := generator.GenerateDailyQuests(ctx, qctx)
	if err != nil {
		// Map errors to appropriate HTTP status codes
		errMsg := err.Error()

		// Specific exact count validation error
		if strings.Contains(errMsg, "AI returned") && strings.Contains(errMsg, "expected") {
			c.JSON(http.StatusUnprocessableEntity, response.Error(http.StatusUnprocessableEntity, errMsg))
			return
		}

		// Timeout or context deadline
		if ctx.Err() == context.DeadlineExceeded || strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded") {
			msg := "AI preview timed out. Increase AI_QUEST_TIMEOUT_SECONDS (or AI_TIMEOUT_SECONDS), try a smaller preview_limit, or use rule_based preview."
			c.JSON(http.StatusGatewayTimeout, response.Error(http.StatusGatewayTimeout, msg))
			return
		}

		// Check for finish_reason=length (output truncated)
		if strings.Contains(errMsg, "finish_reason=length") {
			msg := "AI output was truncated. Increase AI_QUEST_MAX_TOKENS or try a smaller preview_limit."
			c.JSON(http.StatusBadGateway, response.Error(http.StatusBadGateway, msg))
			return
		}

		// Empty AI content or provider errors
		if strings.Contains(errMsg, "content is empty") || strings.Contains(errMsg, "HTTP request failed") || strings.Contains(errMsg, "status code") {
			c.JSON(http.StatusBadGateway, response.Error(http.StatusBadGateway, fmt.Sprintf("%s provider error: %v", req.Mode, err)))
			return
		}

		// Invalid JSON or validation failures
		if strings.Contains(errMsg, "failed to parse") || strings.Contains(errMsg, "validation failed") || strings.Contains(errMsg, "invalid") {
			c.JSON(http.StatusUnprocessableEntity, response.Error(http.StatusUnprocessableEntity, fmt.Sprintf("%s generation failed: %v", req.Mode, err)))
			return
		}

		// Default to 500 for unknown errors
		c.JSON(http.StatusInternalServerError, response.InternalError(fmt.Sprintf("%s preview generation failed: %v", req.Mode, err)))
		return
	}

	// Convert to response DTOs
	quests := dto.ToQuestPreviewResponses(generatedQuests)

	// Return preview response
	resp := GeneratePreviewResponse{
		Date:           dateStr,
		Mode:           req.Mode,
		Inserted:       false, // Preview never inserts to DB
		GeneratedCount: len(quests),
		Quests:         quests,
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(resp, "Quest preview generated"))
}
