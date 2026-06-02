package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type UserHandler struct {
	userService      *services.UserService
	checkinService   *services.DailyCheckinService
	reviewService    *services.DailyReviewService
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

// TODO: replace dev user with authenticated user after auth is implemented.
func (h *UserHandler) GetDevUser(c *gin.Context) {
	user, err := h.userService.GetDevUser()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "dev user not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "user not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

func (h *UserHandler) GetDailyStatus(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	today := timeutil.TodayUTC()
	dateStr := timeutil.FormatDateUTC(today)

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

	c.JSON(http.StatusOK, gin.H{
		"user_id":               userID,
		"date":                  dateStr,
		"has_checked_in_today":  hasCheckedIn,
		"has_reviewed_today":    hasReviewed,
	})
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
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	dateStr := c.Query("date")

	var date time.Time
	var resolvedDateStr string

	if dateStr == "" {
		date = timeutil.TodayUTC()
		resolvedDateStr = timeutil.FormatDateUTC(date)
	} else {
		var err error
		date, err = timeutil.ParseDateUTC(dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid date format, use YYYY-MM-DD",
			})
			return
		}
		resolvedDateStr = timeutil.FormatDateUTC(date)
	}

	start, end := timeutil.DayRangeUTC(date)

	// Debug logs
	c.Request.Context().Value("logger")
	if logger, ok := c.Request.Context().Value("logger").(interface{ Info(string, ...interface{}) }); ok {
		logger.Info("[QUEST API] date_param=" + dateStr)
		logger.Info("[QUEST API] resolved_date=" + resolvedDateStr)
		logger.Info("[QUEST API] range_start=" + start.String())
		logger.Info("[QUEST API] range_end=" + end.String())
		logger.Info("[QUEST API] user_id=" + userID.String())
	}

	quests, err := h.questService.GetQuestsByUserIDAndDate(userID, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to fetch quests",
		})
		return
	}

	// Debug log
	if logger, ok := c.Request.Context().Value("logger").(interface{ Info(string, ...interface{}) }); ok {
		logger.Info(fmt.Sprintf("[QUEST API] returned_count=%d", len(quests)))
	}

	c.JSON(http.StatusOK, gin.H{
		"quests": dto.ToQuestResponses(quests),
		"date":   resolvedDateStr,
	})
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
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	settings, err := h.settingsService.GetSettingsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "settings not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"settings": settings,
	})
}
