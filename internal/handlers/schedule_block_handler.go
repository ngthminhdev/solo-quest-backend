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

type ScheduleBlockHandler struct {
	scheduleBlockService *services.ScheduleBlockService
}

func NewScheduleBlockHandler(scheduleBlockService *services.ScheduleBlockService) *ScheduleBlockHandler {
	return &ScheduleBlockHandler{scheduleBlockService: scheduleBlockService}
}

func (h *ScheduleBlockHandler) List(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	blocks, err := h.scheduleBlockService.ListBlocks(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch schedule blocks"))
		return
	}

	c.JSON(http.StatusOK, response.Success(blocks))
}

func (h *ScheduleBlockHandler) Create(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.CreateScheduleBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	result, err := h.scheduleBlockService.CreateBlock(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, response.Created(result))
}

func (h *ScheduleBlockHandler) Update(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	blockID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid block id"))
		return
	}

	var req dto.CreateScheduleBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	result, err := h.scheduleBlockService.UpdateBlock(userID, blockID, &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrScheduleBlockNotFound):
			c.JSON(http.StatusNotFound, response.NotFound("schedule block not found"))
		case errors.Is(err, services.ErrInvalidScheduleBlockType):
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid schedule block type"))
		case errors.Is(err, services.ErrInvalidDaysOfWeek):
			c.JSON(http.StatusBadRequest, response.BadRequest("days_of_week must not be empty"))
		case errors.Is(err, services.ErrInvalidDayValue):
			c.JSON(http.StatusBadRequest, response.BadRequest("days_of_week must contain values 1-7"))
		case errors.Is(err, services.ErrInvalidTimeRange):
			c.JSON(http.StatusBadRequest, response.BadRequest("start_time must be before end_time"))
		default:
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to update schedule block"))
		}
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *ScheduleBlockHandler) Patch(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	blockID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid block id"))
		return
	}

	var req dto.UpdateScheduleBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	result, err := h.scheduleBlockService.PatchBlock(userID, blockID, &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrScheduleBlockNotFound):
			c.JSON(http.StatusNotFound, response.NotFound("schedule block not found"))
		case errors.Is(err, services.ErrInvalidScheduleBlockType):
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid schedule block type"))
		case errors.Is(err, services.ErrInvalidDaysOfWeek):
			c.JSON(http.StatusBadRequest, response.BadRequest("days_of_week must not be empty"))
		case errors.Is(err, services.ErrInvalidDayValue):
			c.JSON(http.StatusBadRequest, response.BadRequest("days_of_week must contain values 1-7"))
		case errors.Is(err, services.ErrInvalidTimeRange):
			c.JSON(http.StatusBadRequest, response.BadRequest("start_time must be before end_time"))
		default:
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to update schedule block"))
		}
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *ScheduleBlockHandler) Delete(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	blockID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid block id"))
		return
	}

	if err := h.scheduleBlockService.DeleteBlock(userID, blockID); err != nil {
		if errors.Is(err, services.ErrScheduleBlockNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound("schedule block not found"))
		} else {
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to delete schedule block"))
		}
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(nil, "schedule block deleted"))
}
