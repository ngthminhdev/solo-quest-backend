package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type RewardHandler struct {
	rewardService *services.RewardService
}

func NewRewardHandler(rewardService *services.RewardService) *RewardHandler {
	return &RewardHandler{rewardService: rewardService}
}

func (h *RewardHandler) GetRewards(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	result, err := h.rewardService.GetRewards(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch rewards"))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *RewardHandler) ClaimReward(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	rewardID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid reward id"))
		return
	}

	result, err := h.rewardService.ClaimReward(userID, rewardID)
	if err != nil {
		if errors.Is(err, services.ErrRewardNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound(err.Error()))
			return
		}
		if errors.Is(err, services.ErrRewardAlreadyClaimed) {
			c.JSON(http.StatusConflict, response.Conflict(err.Error()))
			return
		}
		if errors.Is(err, services.ErrInsufficientRewardPoints) {
			c.JSON(http.StatusConflict, response.Conflict(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to claim reward"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(result, "reward claimed successfully"))
}

func (h *RewardHandler) GetRedemptions(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	filter := dto.RedemptionFilter{Limit: 50, Offset: 0}

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
				c.JSON(http.StatusBadRequest, response.BadRequest("offset must be >= 0"))
				return
			}
			filter.Offset = offset
		}
	}

	result, err := h.rewardService.GetRedemptions(userID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch redemptions"))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *RewardHandler) CreateReward(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.CreateRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	result, err := h.rewardService.CreateReward(userID, &req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidRewardType) {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid reward type"))
			return
		}
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, response.Created(result))
}

func (h *RewardHandler) UpdateReward(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	rewardID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid reward id"))
		return
	}

	var req dto.UpdateRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	result, err := h.rewardService.UpdateReward(userID, rewardID, &req)
	if err != nil {
		if errors.Is(err, services.ErrRewardNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound("reward not found"))
			return
		}
		if errors.Is(err, services.ErrInvalidRewardType) {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid reward type"))
			return
		}
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *RewardHandler) DeleteReward(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	rewardID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid reward id"))
		return
	}

	if err := h.rewardService.DeleteReward(userID, rewardID); err != nil {
		if errors.Is(err, services.ErrRewardNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound("reward not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to delete reward"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(nil, "reward deleted"))
}
