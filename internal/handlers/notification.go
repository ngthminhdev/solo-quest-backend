package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/services/cron"
	"solo_quest_backend/internal/utils"
	"solo_quest_backend/pkg/logger"
)

type NotificationHandler struct {
	deviceTokenService *services.DeviceTokenService
	notificationSvc    *services.NotificationService
	notificationCron   *cron.NotificationCron
}

func NewNotificationHandler(deviceTokenService *services.DeviceTokenService, notificationSvc *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		deviceTokenService: deviceTokenService,
		notificationSvc:    notificationSvc,
	}
}

func (h *NotificationHandler) SetNotificationCron(nc *cron.NotificationCron) {
	h.notificationCron = nc
}

func (h *NotificationHandler) RegisterToken(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req dto.RegisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	if !isValidPlatform(req.Platform) {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid platform, must be android, ios, or web"))
		return
	}

	token, err := h.deviceTokenService.UpsertToken(userID, &req)
	if err != nil {
		logger.L.Error("[NotificationHandler] Failed to upsert token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to register device token"))
		return
	}

	if h.notificationSvc != nil && h.notificationSvc.IsEnabled() {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		if subErr := h.notificationSvc.SubscribeTokenToTopic(ctx, req.Token, "system_notification"); subErr != nil {
			logger.L.Warn("[NotificationHandler] Failed to subscribe token to topic, continuing",
				zap.String("token", req.Token), zap.Error(subErr))
		}
	}

	c.JSON(http.StatusCreated, response.Created(toDeviceTokenResponse(token)))
}

func (h *NotificationHandler) GetMyTokens(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	tokens, err := h.deviceTokenService.GetActiveTokensByUserID(userID)
	if err != nil {
		logger.L.Error("[NotificationHandler] Failed to get tokens", zap.Error(err))
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch device tokens"))
		return
	}

	responses := make([]dto.DeviceTokenResponse, 0, len(tokens))
	for _, token := range tokens {
		responses = append(responses, toDeviceTokenResponse(token))
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"tokens": responses}))
}

func (h *NotificationHandler) RemoveToken(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	tokenID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid token id"))
		return
	}

	existing, err := h.deviceTokenService.GetByID(userID, tokenID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.NotFound("device token not found"))
		return
	}

	if err := h.deviceTokenService.DeactivateToken(userID, tokenID); err != nil {
		logger.L.Error("[NotificationHandler] Failed to deactivate token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to remove device token"))
		return
	}

	if h.notificationSvc != nil && h.notificationSvc.IsEnabled() {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		if unsubErr := h.notificationSvc.UnsubscribeTokenFromTopic(ctx, existing.Token, "system_notification"); unsubErr != nil {
			logger.L.Warn("[NotificationHandler] Failed to unsubscribe token from topic, continuing",
				zap.String("token", existing.Token), zap.Error(unsubErr))
		}
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"message": "device token removed"}))
}

func (h *NotificationHandler) SendTest(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	if h.notificationSvc == nil || !h.notificationSvc.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, response.ServiceUnavailable("FCM is disabled or not configured"))
		return
	}

	var req dto.SendTestNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request body"))
		return
	}

	title := req.Title
	if title == "" {
		title = "SoloQuest"
	}
	body := req.Body
	if body == "" {
		body = "Test notification from SoloQuest"
	}

	data := req.Data
	if data == nil {
		data = make(map[string]string)
	}
	data["event"] = "test_notification"
	data["source"] = "soloquest_backend"

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.notificationSvc.SendToUser(ctx, userID, title, body, data); err != nil {
		logger.L.Error("[NotificationHandler] Failed to send test notification", zap.Error(err))
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to send test notification"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"message": "test notification sent"}))
}

func (h *NotificationHandler) RunDueOnce(c *gin.Context) {
	if h.notificationCron == nil {
		c.JSON(http.StatusServiceUnavailable, response.ServiceUnavailable("notification cron is not initialized"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result, err := h.notificationCron.RunOnce(ctx)
	if err != nil {
		logger.L.Error("[NotificationHandler] RunDueOnce failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, response.InternalError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{
		"processed_users": result.ProcessedUsers,
		"sent_count":      result.SentCount,
		"skipped_count":   result.SkippedCount,
		"failed_count":    result.FailedCount,
		"duration_ms":     result.DurationMs,
	}))
}

func isValidPlatform(platform string) bool {
	switch platform {
	case "android", "ios", "web":
		return true
	default:
		return false
	}
}

func toDeviceTokenResponse(token *models.DeviceToken) dto.DeviceTokenResponse {
	resp := dto.DeviceTokenResponse{
		ID:       token.ID.String(),
		UserID:   token.UserID.String(),
		Token:    token.Token,
		Platform: token.Platform,
		DeviceName: token.DeviceName,
		IsActive: token.IsActive,
	}

	if token.LastUsedAt != nil {
		ts := token.LastUsedAt.Format(time.RFC3339)
		resp.LastUsedAt = &ts
	}

	return resp
}
