package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/pkg/validate"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type ReminderSettingsHandler struct {
	reminderSettingService *services.ReminderSettingService
}

func NewReminderSettingsHandler(reminderSettingService *services.ReminderSettingService) *ReminderSettingsHandler {
	return &ReminderSettingsHandler{reminderSettingService: reminderSettingService}
}

func (h *ReminderSettingsHandler) GetReminderSettings(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	settings, err := h.reminderSettingService.GetReminderSettingsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch reminder settings"))
		return
	}

	c.JSON(http.StatusOK, response.Success(settings))
}

func (h *ReminderSettingsHandler) UpdateReminderSetting(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	reminderType := c.Param("type")

	if !validate.IsValidReminderType(reminderType) {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid reminder type"))
		return
	}

	var req dto.UpdateReminderSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	if req.StartTime != nil && *req.StartTime != "" && !validate.IsValidHHMM(*req.StartTime) {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid start_time format, expected HH:MM"))
		return
	}
	if req.EndTime != nil && *req.EndTime != "" && !validate.IsValidHHMM(*req.EndTime) {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid end_time format, expected HH:MM"))
		return
	}

	result, err := h.reminderSettingService.UpdateReminderSetting(userID, reminderType, &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidReminderType):
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid reminder type"))
		case errors.Is(err, services.ErrInvalidFrequency):
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid frequency"))
		case errors.Is(err, services.ErrInvalidReminderStatus):
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid status, must be enabled or disabled"))
		case errors.Is(err, services.ErrInvalidInterval):
			c.JSON(http.StatusBadRequest, response.BadRequest("interval_minutes must be positive"))
		case errors.Is(err, services.ErrInvalidMaxPerDay):
			c.JSON(http.StatusBadRequest, response.BadRequest("max_per_day must be positive"))
		default:
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to update reminder setting"))
		}
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *ReminderSettingsHandler) ToggleReminderSetting(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	reminderType := c.Param("type")

	if !validate.IsValidReminderType(reminderType) {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid reminder type"))
		return
	}

	var req dto.ToggleReminderSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	if !validate.IsValidReminderStatus(req.Status) {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid status, must be enabled or disabled"))
		return
	}

	result, err := h.reminderSettingService.ToggleReminderSetting(userID, reminderType, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidReminderType):
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid reminder type"))
		case errors.Is(err, services.ErrInvalidReminderStatus):
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid status, must be enabled or disabled"))
		default:
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to toggle reminder setting"))
		}
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}
