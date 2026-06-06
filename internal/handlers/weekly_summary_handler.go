package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type WeeklySummaryHandler struct {
	weeklySummaryService *services.WeeklySummaryService
}

func NewWeeklySummaryHandler(weeklySummaryService *services.WeeklySummaryService) *WeeklySummaryHandler {
	return &WeeklySummaryHandler{weeklySummaryService: weeklySummaryService}
}

func (h *WeeklySummaryHandler) GetWeeklySummary(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	weekStartStr := c.Query("week_start")

	var summary interface{}
	var svcErr error

	if weekStartStr == "" {
		summary, svcErr = h.weeklySummaryService.GetCurrentWeekSummary(userID)
	} else {
		summary, svcErr = h.weeklySummaryService.GetWeekSummary(userID, weekStartStr)
	}

	if svcErr != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(svcErr.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(summary))
}
