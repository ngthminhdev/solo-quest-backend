package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type UserHandler struct {
	userService    *services.UserService
	checkinService *services.DailyCheckinService
	reviewService  *services.DailyReviewService
}

func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func NewUserHandlerWithDaily(userService *services.UserService, checkinService *services.DailyCheckinService, reviewService *services.DailyReviewService) *UserHandler {
	return &UserHandler{
		userService:    userService,
		checkinService: checkinService,
		reviewService:  reviewService,
	}
}

func (h *UserHandler) GetDevUser(c *gin.Context) {
	user, err := h.userService.GetDevUser()
	if err != nil {
		c.JSON(http.StatusNotFound, response.NotFound("dev user not found"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"user": user}))
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.NotFound("user not found"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"user": user}))
}

func (h *UserHandler) GetDailyStatus(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	today := timeutil.TodayVN()
	dateStr := timeutil.FormatDateVN(today)

	hasCheckedIn := false
	if h.checkinService != nil {
		checkinStatus, err := h.checkinService.GetToday(userID)
		if err == nil && checkinStatus.HasCheckedIn {
			hasCheckedIn = true
		}
	}

	hasReviewed := false
	if h.reviewService != nil {
		reviewStatus, err := h.reviewService.GetToday(userID)
		if err == nil && reviewStatus.HasReviewed {
			hasReviewed = true
		}
	}

	c.JSON(http.StatusOK, response.Success(gin.H{
		"user_id":              userID,
		"date":                 dateStr,
		"has_checked_in_today": hasCheckedIn,
		"has_reviewed_today":   hasReviewed,
	}))
}

type QuestHandler struct {
	questService *services.QuestService
}

func NewQuestHandler(questService *services.QuestService) *QuestHandler {
	return &QuestHandler{questService: questService}
}

func (h *QuestHandler) GetQuests(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	dateStr := c.Query("date")

	var date time.Time
	var resolvedDateStr string

	if dateStr == "" {
		date = timeutil.TodayVN()
		resolvedDateStr = timeutil.FormatDateVN(date)
	} else {
		var err error
		date, err = timeutil.ParseDateVN(dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid date format, use YYYY-MM-DD"))
			return
		}
		resolvedDateStr = timeutil.FormatDateVN(date)
	}

	start, end := timeutil.DayRangeVN(date)

	if logger, ok := c.Request.Context().Value("logger").(interface{ Info(string, ...interface{}) }); ok {
		logger.Info("[QUEST API] date_param=" + dateStr)
		logger.Info("[QUEST API] resolved_date=" + resolvedDateStr)
		logger.Info("[QUEST API] range_start=" + start.String())
		logger.Info("[QUEST API] range_end=" + end.String())
		logger.Info("[QUEST API] user_id=" + userID.String())
	}

	quests, err := h.questService.GetQuestsByUserIDAndDate(userID, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch quests"))
		return
	}

	if logger, ok := c.Request.Context().Value("logger").(interface{ Info(string, ...interface{}) }); ok {
		logger.Info(fmt.Sprintf("[QUEST API] returned_count=%d", len(quests)))
	}

	c.JSON(http.StatusOK, response.Success(gin.H{
		"quests": dto.ToQuestResponses(quests),
		"date":   resolvedDateStr,
	}))
}

type SettingsHandler struct {
	settingsService *services.SettingsService
}

func NewSettingsHandler(settingsService *services.SettingsService) *SettingsHandler {
	return &SettingsHandler{settingsService: settingsService}
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	settings, err := h.settingsService.GetSettingsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.NotFound("settings not found"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"settings": settings}))
}

func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	if req.DailyReminderTime != nil && *req.DailyReminderTime != "" {
		if !isValidHHMM(*req.DailyReminderTime) {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid daily_reminder_time format, expected HH:MM"))
			return
		}
	}
	if req.QuietAfterTime != nil && *req.QuietAfterTime != "" {
		if !isValidHHMM(*req.QuietAfterTime) {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid quiet_after_time format, expected HH:MM"))
			return
		}
	}
	if req.QuietStartTime != nil && *req.QuietStartTime != "" {
		if !isValidHHMM(*req.QuietStartTime) {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid quiet_start_time format, expected HH:MM"))
			return
		}
	}
	if req.QuietEndTime != nil && *req.QuietEndTime != "" {
		if !isValidHHMM(*req.QuietEndTime) {
			c.JSON(http.StatusBadRequest, response.BadRequest("invalid quiet_end_time format, expected HH:MM"))
			return
		}
	}

	settings, err := h.settingsService.UpdateSettingsByUserID(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to update settings"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"settings": settings}))
}

func isValidHHMM(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	if s[0] < '0' || s[0] > '2' {
		return false
	}
	if s[1] < '0' || s[1] > '9' {
		return false
	}
	if s[0] == '2' && s[1] > '3' {
		return false
	}
	if s[3] < '0' || s[3] > '5' {
		return false
	}
	if s[4] < '0' || s[4] > '9' {
		return false
	}
	return true
}
