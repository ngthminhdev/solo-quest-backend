package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/utils"
)

type questGenerationService interface {
	GenerateToday(
		ctx context.Context,
		userID uuid.UUID,
		req quest_generation.GenerateTodayRequest,
	) (*quest_generation.GenerateTodayResult, error)
}

type QuestGenerationHandler struct {
	service questGenerationService
}

func NewQuestGenerationHandler(service questGenerationService) *QuestGenerationHandler {
	return &QuestGenerationHandler{service: service}
}

type GenerateTodayResponse struct {
	Date                 string              `json:"date"`
	Inserted             bool                `json:"inserted"`
	ExistingReturned     bool                `json:"existing_returned"`
	Source               string              `json:"source"`
	FallbackUsed         bool                `json:"fallback_used"`
	AIErrorType          *string             `json:"ai_error_type"`
	GeneratedCount       int                 `json:"generated_count"`
	PreservedCount       int                 `json:"preserved_count"`
	ReplacedPendingCount int                 `json:"replaced_pending_count"`
	Quests               []dto.QuestResponse `json:"quests"`
}

func (h *QuestGenerationHandler) GenerateToday(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req quest_generation.GenerateTodayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body - defaults will be used by service
	}

	// Validate date format if provided
	if req.Date != nil && *req.Date != "" {
		_, err := timeutil.ParseDateVN(*req.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid date format, use YYYY-MM-DD"))
			return
		}
	}

	// Reject replace_pending_only = false explicitly
	if req.ReplacePendingOnly != nil && !*req.ReplacePendingOnly {
		c.JSON(http.StatusBadRequest, response.BadRequest("replace_pending_only=false is not supported"))
		return
	}

	result, err := h.service.GenerateToday(c.Request.Context(), userID, req)
	if err != nil {
		errMsg := err.Error()
		c.JSON(http.StatusInternalServerError, response.InternalError(errMsg))
		return
	}

	// Determine response message
	var msg string
	if result.ExistingReturned {
		if result.GeneratedCount == 0 && result.PreservedCount > 0 && req.Force != nil && *req.Force {
			msg = "No pending slots available for regeneration."
		} else {
			msg = "Today's quests already exist"
		}
	} else if result.FallbackUsed {
		msg = "Today's quests generated with fallback"
	} else {
		msg = "Today's quests generated"
	}

	// Prepare ai_error_type pointer (so it is serialized as null if empty)
	var aiErrPtr *string
	if result.AIErrorType != "" {
		aiErrPtr = &result.AIErrorType
	}

	resp := GenerateTodayResponse{
		Date:                 result.Date,
		Inserted:             result.Inserted,
		ExistingReturned:     result.ExistingReturned,
		Source:               result.Source,
		FallbackUsed:         result.FallbackUsed,
		AIErrorType:          aiErrPtr,
		GeneratedCount:       result.GeneratedCount,
		PreservedCount:       result.PreservedCount,
		ReplacedPendingCount: result.ReplacedPendingCount,
		Quests:               dto.ToQuestResponses(result.Quests),
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(resp, msg))
}
