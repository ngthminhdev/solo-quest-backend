package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

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

	StartTodayGeneration(
		ctx context.Context,
		userID uuid.UUID,
		req quest_generation.GenerateTodayRequest,
	) (*quest_generation.StartResult, error)

	GetJobStatus(
		ctx context.Context,
		userID uuid.UUID,
		date *string,
	) (*quest_generation.JobStatus, error)
}

type QuestGenerationHandler struct {
	service questGenerationService
}

func NewQuestGenerationHandler(service questGenerationService) *QuestGenerationHandler {
	return &QuestGenerationHandler{service: service}
}

type GenerateTodayResponse struct {
	Date                 string              `json:"date"`
	TargetCount          int                 `json:"target_count"`
	ExistingCount        int                 `json:"existing_count"`
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

// GenerateTodayAsyncResponse is the HTTP 202 payload returned when AI
// generation runs in the background.
type GenerateTodayAsyncResponse struct {
	Date             string `json:"date"`
	Status           string `json:"status"`
	JobID            string `json:"job_id"`
	EstimatedSeconds int    `json:"estimated_seconds"`
}

// GenerationStatusResponse is the payload for the status endpoint.
type GenerationStatusResponse struct {
	Date           string     `json:"date"`
	Status         string     `json:"status"`
	JobID          *string    `json:"job_id"`
	QuestCount     int        `json:"quest_count"`
	TargetCount    int        `json:"target_count"`
	ExistingCount  int        `json:"existing_count"`
	GeneratedCount int        `json:"generated_count"`
	Source         *string    `json:"source"`
	FallbackUsed   bool       `json:"fallback_used"`
	AIErrorType    *string    `json:"ai_error_type"`
	ErrorCode      *string    `json:"error_code"`
	ErrorMessage   *string    `json:"error_message"`
	FinishedAt     *time.Time `json:"finished_at"`
	DurationMS     int64      `json:"duration_ms"`
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
	req.RequestUserID = userID
	req.AuthSource = authSourceFromRequest(c)

	// Reject date override - this endpoint always uses user's local today
	if req.Date != nil && *req.Date != "" {
		c.JSON(http.StatusBadRequest, response.BadRequest("generate-today does not accept date parameter, use /api/quests/generate?date=YYYY-MM-DD instead"))
		return
	}

	// Reject replace_pending_only = false explicitly
	if req.ReplacePendingOnly != nil && !*req.ReplacePendingOnly {
		c.JSON(http.StatusBadRequest, response.BadRequest("replace_pending_only=false is not supported"))
		return
	}

	preferAI := true
	if req.PreferAI != nil {
		preferAI = *req.PreferAI
	}

	// AI path: never block the HTTP request on a slow AI call. Start (or
	// reuse) a background job and return 202 immediately. If today's quests
	// already exist and force=false, the existing quests are returned 200.
	if preferAI {
		h.handleAsyncGenerate(c, userID, req)
		return
	}

	// Rule-based path is fast enough to run synchronously.
	result, err := h.service.GenerateToday(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.SuccessWithMessage(buildGenerateTodayResponse(result), messageForResult(result, req, true)))
}

func (h *QuestGenerationHandler) handleAsyncGenerate(
	c *gin.Context,
	userID uuid.UUID,
	req quest_generation.GenerateTodayRequest,
) {
	start, err := h.service.StartTodayGeneration(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError(err.Error()))
		return
	}

	// Existing quests returned synchronously.
	if start.Existing != nil {
		isToday := req.Date == nil || *req.Date == ""
		c.JSON(http.StatusOK, response.SuccessWithMessage(
			buildGenerateTodayResponse(start.Existing),
			messageForResult(start.Existing, req, isToday),
		))
		return
	}

	// Background job started/reused.
	job := start.Job
	c.JSON(http.StatusAccepted, response.Response{
		Code:    http.StatusAccepted,
		Message: "Quest generation started",
		Data: GenerateTodayAsyncResponse{
			Date:             job.Date,
			Status:           job.Status,
			JobID:            job.JobID,
			EstimatedSeconds: job.EstimatedSeconds,
		},
	})
}

// Generate creates quests for an explicit date. Use query param date=YYYY-MM-DD.
func (h *QuestGenerationHandler) Generate(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	// Require explicit date in query param
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, response.BadRequest("date query parameter is required (YYYY-MM-DD)"))
		return
	}

	// Validate date format
	if _, err := timeutil.ParseDateVN(dateStr); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid date format, use YYYY-MM-DD"))
		return
	}

	var req quest_generation.GenerateTodayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body - defaults will be used
	}
	req.RequestUserID = userID
	req.AuthSource = authSourceFromRequest(c)

	// Override with explicit date from query
	req.Date = &dateStr

	// Reject replace_pending_only = false explicitly
	if req.ReplacePendingOnly != nil && !*req.ReplacePendingOnly {
		c.JSON(http.StatusBadRequest, response.BadRequest("replace_pending_only=false is not supported"))
		return
	}

	preferAI := true
	if req.PreferAI != nil {
		preferAI = *req.PreferAI
	}

	if preferAI {
		h.handleAsyncGenerate(c, userID, req)
		return
	}

	// Rule-based path
	result, err := h.service.GenerateToday(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.SuccessWithMessage(buildGenerateTodayResponse(result), messageForResult(result, req, false)))
}

// GetTodayGenerationStatus reports the progress of an async generation job so
// the frontend can poll until completion.
func (h *QuestGenerationHandler) GetTodayGenerationStatus(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var datePtr *string
	if dateStr := c.Query("date"); dateStr != "" {
		if _, err := timeutil.ParseDateVN(dateStr); err != nil {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid date format, use YYYY-MM-DD"))
			return
		}
		datePtr = &dateStr
	}

	status, err := h.service.GetJobStatus(c.Request.Context(), userID, datePtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(GenerationStatusResponse{
		Date:           status.Date,
		Status:         status.Status,
		JobID:          status.JobID,
		QuestCount:     status.QuestCount,
		TargetCount:    status.TargetCount,
		ExistingCount:  status.ExistingCount,
		GeneratedCount: status.GeneratedCount,
		Source:         status.Source,
		FallbackUsed:   status.FallbackUsed,
		AIErrorType:    status.AIErrorType,
		ErrorCode:      status.ErrorCode,
		ErrorMessage:   status.ErrorMessage,
		FinishedAt:     status.FinishedAt,
		DurationMS:     status.DurationMS,
	}))
}

func buildGenerateTodayResponse(result *quest_generation.GenerateTodayResult) GenerateTodayResponse {
	var aiErrPtr *string
	if result.AIErrorType != "" {
		aiErrPtr = &result.AIErrorType
	}
	return GenerateTodayResponse{
		Date:                 result.Date,
		TargetCount:          result.TargetCount,
		ExistingCount:        result.ExistingCount,
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
}

func messageForResult(result *quest_generation.GenerateTodayResult, req quest_generation.GenerateTodayRequest, isToday bool) string {
	if result.ExistingReturned {
		if result.GeneratedCount == 0 && result.PreservedCount > 0 && req.Force != nil && *req.Force {
			return "No pending slots available for regeneration"
		}
		if result.GeneratedCount > 0 {
			if isToday {
				return "Generated missing quests for today"
			}
			return "Generated missing quests for requested date"
		}
		if isToday {
			return "Today's quests already exist"
		}
		return "Quests already exist for requested date"
	}
	if result.FallbackUsed {
		if isToday {
			return "Today's quests generated with fallback"
		}
		return "Quests generated with fallback for requested date"
	}
	if isToday {
		return "Today's quests generated"
	}
	return "Quests generated for requested date"
}

func authSourceFromRequest(c *gin.Context) string {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		return "jwt"
	}
	if authHeader != "" {
		return "authorization_header"
	}
	return "context"
}
