package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type ProgressHandler struct {
	progressService *services.ProgressService
}

func NewProgressHandler(progressService *services.ProgressService) *ProgressHandler {
	return &ProgressHandler{progressService: progressService}
}

func (h *ProgressHandler) GetProgress(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	progress, err := h.progressService.GetProgress(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, progress)
}

func (h *ProgressHandler) GetWeeklyChart(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	chart, err := h.progressService.GetWeeklyChart(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch weekly chart"})
		return
	}

	c.JSON(http.StatusOK, chart)
}

func (h *ProgressHandler) GetXPHistory(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	filter := dto.XPHistoryFilter{
		Limit:  50,
		Offset: 0,
	}

	if currency := c.Query("currency"); currency != "" {
		currency = strings.ToLower(currency)
		if currency != "xp" && currency != "gem" && currency != "reward_points" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid currency, must be xp, gem, or reward_points"})
			return
		}
		filter.Currency = currency
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			if limit > 100 {
				limit = 100
			}
			filter.Limit = limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		var offset int
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err == nil {
			if offset < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "offset must be >= 0"})
				return
			}
			filter.Offset = offset
		}
	}

	result, err := h.progressService.GetXPHistory(userID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch XP history"})
		return
	}

	c.JSON(http.StatusOK, result)
}
