package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type LearningRoadmapHandler struct {
	service *services.LearningRoadmapService
}

func NewLearningRoadmapHandler(service *services.LearningRoadmapService) *LearningRoadmapHandler {
	return &LearningRoadmapHandler{service: service}
}

func (h *LearningRoadmapHandler) List(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	items, err := h.service.ListRoadmaps(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch roadmaps"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"items": items}))
}

func (h *LearningRoadmapHandler) GetDetail(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	roadmapID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid roadmap id"))
		return
	}

	item, err := h.service.GetRoadmapDetail(userID, roadmapID)
	if err != nil {
		if errors.Is(err, services.ErrRoadmapNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound("roadmap not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch roadmap detail"))
		return
	}

	c.JSON(http.StatusOK, response.Success(item))
}

func (h *LearningRoadmapHandler) Delete(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	roadmapID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid roadmap id"))
		return
	}

	if err := h.service.DeleteRoadmap(userID, roadmapID); err != nil {
		if errors.Is(err, services.ErrRoadmapNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound("roadmap not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to delete roadmap"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(nil, "roadmap deleted"))
}

func (h *LearningRoadmapHandler) Follow(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	roadmapID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid roadmap id"))
		return
	}

	result, err := h.service.FollowRoadmap(userID, roadmapID)
	if err != nil {
		if errors.Is(err, services.ErrRoadmapNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound("roadmap not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to follow roadmap"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(result, "Started following roadmap"))
}

func (h *LearningRoadmapHandler) ToggleStep(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	roadmapID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid roadmap id"))
		return
	}

	stepID, err := uuid.Parse(c.Param("step_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid step id"))
		return
	}

	var req dto.ToggleRoadmapStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	result, err := h.service.ToggleStep(userID, roadmapID, stepID, *req.Completed)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrRoadmapNotFound):
			c.JSON(http.StatusNotFound, response.NotFound("roadmap not found"))
		case errors.Is(err, services.ErrStepNotFound):
			c.JSON(http.StatusNotFound, response.NotFound("step not found"))
		case errors.Is(err, services.ErrNotFollowingRoadmap):
			c.JSON(http.StatusBadRequest, response.BadRequest("user is not following this roadmap"))
		default:
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to toggle step"))
		}
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *LearningRoadmapHandler) AiSuggest(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.AiRoadmapSuggestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	result, err := h.service.GetAISuggestions(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to get AI suggestions"))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *LearningRoadmapHandler) Create(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.CreateLearningRoadmapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	// Validate source
	if req.Source != "ai" && req.Source != "user" {
		c.JSON(http.StatusBadRequest, response.BadRequest("source must be 'ai' or 'user'"))
		return
	}

	result, err := h.service.CreateRoadmap(userID, req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSuggestionNotFound):
			c.JSON(http.StatusNotFound, response.NotFound("AI suggestion not found"))
		case errors.Is(err, services.ErrInvalidCreateRequest):
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid create roadmap request"))
		default:
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to create roadmap"))
		}
		return
	}

	c.JSON(http.StatusCreated, response.Created(result))
}

func (h *LearningRoadmapHandler) Suggest(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.TemplateSuggestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	result, err := h.service.GetTemplateSuggestions(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to get template suggestions"))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *LearningRoadmapHandler) CreateFromTemplate(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.CreateFromTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	// Validate source
	if req.Source != "template" {
		c.JSON(http.StatusBadRequest, response.BadRequest("source must be 'template'"))
		return
	}

	result, err := h.service.CreateFromTemplate(userID, req)
	if err != nil {
		if err.Error() == "template not found" {
			c.JSON(http.StatusNotFound, response.NotFound("template not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to create roadmap from template"))
		return
	}

	c.JSON(http.StatusCreated, response.Created(gin.H{"roadmap": result}))
}

func (h *LearningRoadmapHandler) Generate(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.GenerateLearningRoadmapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	result, err := h.service.StartRoadmapGeneration(c.Request.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrEmptyLearningGoal):
			c.JSON(http.StatusBadRequest, response.BadRequest("learning_goal is required, min 3 chars, max 300 chars"))
		case errors.Is(err, services.ErrInvalidMaxDuration):
			c.JSON(http.StatusBadRequest, response.BadRequest("max_duration must be between 30 and 1200 minutes"))
		default:
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to start roadmap generation"))
		}
		return
	}

	if result.Status == "completed" {
		c.JSON(http.StatusOK, response.SuccessWithMessage(gin.H{
			"job_id":               result.JobID,
			"status":               result.Status,
			"source":               result.Source,
			"poll_after_seconds":   result.PollAfterSeconds,
			"item":                 result.Item,
			"generated_step_count": result.GeneratedSteps,
		}, "Learning roadmap generation already completed"))
		return
	}

	c.JSON(http.StatusAccepted, response.Response{
		Code:    http.StatusAccepted,
		Message: "Learning roadmap generation started",
		Data: gin.H{
			"job_id":             result.JobID,
			"status":             result.Status,
			"date":               nil,
			"source":             result.Source,
			"poll_after_seconds": result.PollAfterSeconds,
		},
	})
}

func (h *LearningRoadmapHandler) GetGenerateStatus(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	jobID, err := uuid.Parse(c.Query("job_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid job_id"))
		return
	}

	status, err := h.service.GetRoadmapGenerationStatus(c.Request.Context(), userID, jobID)
	if err != nil {
		if errors.Is(err, services.ErrRoadmapNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound("generation job not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch roadmap generation status"))
		return
	}

	message := "Learning roadmap generation is still running"
	switch status.Status {
	case "completed":
		message = "Learning roadmap generated successfully"
	case "failed":
		message = "Learning roadmap generation failed"
	case "stale":
		message = "Learning roadmap generation is stale"
	case "pending":
		message = "Learning roadmap generation is pending"
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(status, message))
}
