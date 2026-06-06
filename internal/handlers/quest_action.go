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

type QuestActionHandler struct {
	questActionService *services.QuestActionService
}

func NewQuestActionHandler(questActionService *services.QuestActionService) *QuestActionHandler {
	return &QuestActionHandler{questActionService: questActionService}
}

func (h *QuestActionHandler) StartQuest(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	questID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid quest id"))
		return
	}

	quest, err := h.questActionService.StartQuest(userID, questID)
	if err != nil {
		if errors.Is(err, services.ErrQuestNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound(err.Error()))
			return
		}
		if errors.Is(err, services.ErrInvalidQuestStatus) {
			c.JSON(http.StatusConflict, response.Conflict(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to start quest"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(gin.H{
		"quest": dto.ToQuestResponse(*quest),
	}, "quest started successfully"))
}

func (h *QuestActionHandler) CompleteQuest(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	questID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid quest id"))
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Note = ""
	}

	result, err := h.questActionService.CompleteQuest(userID, questID, req.Note)
	if err != nil {
		if errors.Is(err, services.ErrQuestNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound(err.Error()))
			return
		}
		if errors.Is(err, services.ErrQuestAlreadyCompleted) {
			c.JSON(http.StatusConflict, response.Conflict(err.Error()))
			return
		}
		if errors.Is(err, services.ErrInvalidQuestStatus) {
			c.JSON(http.StatusConflict, response.Conflict(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to complete quest"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(gin.H{
		"quest":                     dto.ToQuestResponse(*result.Quest),
		"exp_transaction":           result.EXPTransaction,
		"reward_points_transaction": result.RewardPointsTransaction,
		"profile":                   result.Profile,
	}, result.Message))
}

func (h *QuestActionHandler) SkipQuest(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	questID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid quest id"))
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = ""
	}

	quest, err := h.questActionService.SkipQuest(userID, questID, req.Reason)
	if err != nil {
		if errors.Is(err, services.ErrQuestNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound(err.Error()))
			return
		}
		if errors.Is(err, services.ErrQuestAlreadyCompleted) {
			c.JSON(http.StatusConflict, response.Conflict(err.Error()))
			return
		}
		if errors.Is(err, services.ErrInvalidQuestStatus) {
			c.JSON(http.StatusConflict, response.Conflict(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to skip quest"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(gin.H{
		"quest": dto.ToQuestResponse(*quest),
	}, "quest skipped successfully"))
}

func (h *QuestActionHandler) SnoozeQuest(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	questID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid quest id"))
		return
	}

	var req struct {
		Minutes int `json:"minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	quest, err := h.questActionService.SnoozeQuest(userID, questID, req.Minutes)
	if err != nil {
		if errors.Is(err, services.ErrQuestNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound(err.Error()))
			return
		}
		if errors.Is(err, services.ErrInvalidQuestStatus) {
			c.JSON(http.StatusConflict, response.Conflict(err.Error()))
			return
		}
		if errors.Is(err, services.ErrInvalidSnoozeDuration) {
			c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to snooze quest"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(gin.H{
		"quest": dto.ToQuestResponse(*quest),
	}, "quest snoozed successfully"))
}
