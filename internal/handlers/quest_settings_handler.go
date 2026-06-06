package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type QuestSettingsHandler struct {
	questSettingsService *services.QuestSettingsService
}

func NewQuestSettingsHandler(questSettingsService *services.QuestSettingsService) *QuestSettingsHandler {
	return &QuestSettingsHandler{questSettingsService: questSettingsService}
}

func (h *QuestSettingsHandler) Get(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	settings, err := h.questSettingsService.GetOrCreate(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch quest settings"))
		return
	}

	c.JSON(http.StatusOK, response.Success(settings))
}

func (h *QuestSettingsHandler) Update(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.UpdateQuestSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	settings, err := h.questSettingsService.Update(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(settings))
}

func (h *QuestSettingsHandler) Reset(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	settings, err := h.questSettingsService.Reset(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to reset quest settings"))
		return
	}

	c.JSON(http.StatusOK, response.Success(settings))
}
