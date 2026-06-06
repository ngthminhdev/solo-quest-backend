package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type DailyReviewHandler struct {
	reviewService *services.DailyReviewService
}

func NewDailyReviewHandler(reviewService *services.DailyReviewService) *DailyReviewHandler {
	return &DailyReviewHandler{reviewService: reviewService}
}

func (h *DailyReviewHandler) GetToday(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	result, err := h.reviewService.GetToday(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch review"))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *DailyReviewHandler) GetByDate(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	dateStr := c.Query("date")
	if dateStr == "" {
		result, err := h.reviewService.GetToday(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch review"))
			return
		}
		c.JSON(http.StatusOK, response.Success(result))
		return
	}

	result, err := h.reviewService.GetByDate(userID, dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *DailyReviewHandler) GetSummary(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	dateStr := c.Query("date")
	result, err := h.reviewService.GetSummary(userID, dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *DailyReviewHandler) Save(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.SaveDailyReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	item, _, err := h.reviewService.Save(userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(gin.H{"item": item}, "daily review saved successfully"))
}
