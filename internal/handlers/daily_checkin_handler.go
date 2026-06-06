package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type DailyCheckinHandler struct {
	checkinService *services.DailyCheckinService
}

func NewDailyCheckinHandler(checkinService *services.DailyCheckinService) *DailyCheckinHandler {
	return &DailyCheckinHandler{checkinService: checkinService}
}

func (h *DailyCheckinHandler) GetToday(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	result, err := h.checkinService.GetToday(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch check-in"))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *DailyCheckinHandler) GetByDate(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	dateStr := c.Query("date")
	if dateStr == "" {
		result, err := h.checkinService.GetToday(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch check-in"))
			return
		}
		c.JSON(http.StatusOK, response.Success(result))
		return
	}

	result, err := h.checkinService.GetByDate(userID, dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *DailyCheckinHandler) Save(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.SaveDailyCheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	item, err := h.checkinService.Save(userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(gin.H{"item": item}, "daily check-in saved successfully"))
}
