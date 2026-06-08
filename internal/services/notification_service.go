package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"solo_quest_backend/internal/infra/fcm"
	"solo_quest_backend/pkg/logger"
)

const maxRetries = 3

type NotificationService struct {
	fcmClient      fcm.FCMClient
	tokenService   *DeviceTokenService
}

func NewNotificationService(fcmClient fcm.FCMClient, tokenService *DeviceTokenService) *NotificationService {
	return &NotificationService{
		fcmClient:    fcmClient,
		tokenService: tokenService,
	}
}

func (s *NotificationService) IsEnabled() bool {
	return s.fcmClient != nil && s.fcmClient.IsEnabled()
}

func (s *NotificationService) SendToUser(ctx context.Context, userID uuid.UUID, title, body string, data map[string]string) error {
	if !s.IsEnabled() {
		logger.L.Warn("[NotificationService] FCM is disabled, skipping SendToUser", zap.String("user_id", userID.String()))
		return nil
	}

	tokens, err := s.tokenService.GetActiveTokensByUserID(userID)
	if err != nil {
		logger.L.Error("[NotificationService] Failed to get active tokens", zap.String("user_id", userID.String()), zap.Error(err))
		return err
	}

	if len(tokens) == 0 {
		logger.L.Warn("[NotificationService] No active tokens for user", zap.String("user_id", userID.String()))
		return nil
	}

	var lastErr error
	failCount := 0

	for _, token := range tokens {
		err := s.sendToTokenWithRetry(ctx, token.Token, title, body, data)
		if err != nil {
			if fcm.IsInvalidTokenError(err) {
				logger.L.Warn("[NotificationService] Deactivating invalid token", zap.String("token_id", token.ID.String()))
				if deactErr := s.tokenService.DeactivateByRawToken(token.Token); deactErr != nil {
					logger.L.Error("[NotificationService] Failed to deactivate token", zap.Error(deactErr))
				}
			}
			lastErr = err
			failCount++
		}
	}

	if failCount == len(tokens) && lastErr != nil {
		return lastErr
	}

	return nil
}

func (s *NotificationService) SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) error {
	if !s.IsEnabled() {
		logger.L.Warn("[NotificationService] FCM is disabled, skipping SendToTopic", zap.String("topic", topic))
		return nil
	}

	msgID, err := s.fcmClient.SendToTopic(ctx, topic, title, body, data)
	if err != nil {
		logger.L.Error("[NotificationService] Failed to send to topic", zap.String("topic", topic), zap.Error(err))
		return err
	}

	logger.L.Info("[NotificationService] Sent to topic", zap.String("topic", topic), zap.String("message_id", msgID))
	return nil
}

func (s *NotificationService) SubscribeTokenToTopic(ctx context.Context, token, topic string) error {
	if !s.IsEnabled() {
		return nil
	}
	return s.fcmClient.SubscribeToTopic(ctx, topic, []string{token})
}

func (s *NotificationService) UnsubscribeTokenFromTopic(ctx context.Context, token, topic string) error {
	if !s.IsEnabled() {
		return nil
	}
	return s.fcmClient.UnsubscribeFromTopic(ctx, topic, []string{token})
}

func (s *NotificationService) sendToTokenWithRetry(ctx context.Context, token, title, body string, data map[string]string) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(backoff)
		}

		msgID, err := s.fcmClient.SendToToken(ctx, token, title, body, data)
		if err == nil {
			logger.L.Info("[NotificationService] Sent to token", zap.String("message_id", msgID))
			return nil
		}

		if fcm.IsInvalidTokenError(err) {
			return err
		}

		if !fcm.IsTransientError(err) {
			return err
		}

		lastErr = err
		logger.L.Warn("[NotificationService] Transient error, retrying", zap.Int("attempt", attempt+1), zap.Error(err))
	}

	return lastErr
}
