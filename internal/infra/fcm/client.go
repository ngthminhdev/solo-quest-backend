package fcm

import (
	"context"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/zap"
	"google.golang.org/api/option"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/pkg/logger"
)

type FCMClient interface {
	SendToToken(ctx context.Context, token, title, body string, data map[string]string) (string, error)
	SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) (string, error)
	SubscribeToTopic(ctx context.Context, topic string, tokens []string) error
	UnsubscribeFromTopic(ctx context.Context, topic string, tokens []string) error
	IsEnabled() bool
	IsDryRun() bool
}

func IsInvalidTokenError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	invalidPatterns := []string{
		"registration token is not a valid fcm registration token",
		"requested entity was not found",
		"registration-token-not-registered",
		"notregistered",
		"unregistered",
	}
	for _, pattern := range invalidPatterns {
		if strings.Contains(msg, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	transientPatterns := []string{
		"internal",
		"unavailable",
		"deadline exceeded",
		"timeout",
	}
	for _, pattern := range transientPatterns {
		if strings.Contains(msg, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

type realClient struct {
	app    *firebase.App
	client *messaging.Client
	cfg    config.FCMConfig
}

func (c *realClient) IsEnabled() bool {
	return c.client != nil
}

func (c *realClient) IsDryRun() bool {
	return c.cfg.DryRun
}

func NewClient(cfg config.FCMConfig) (FCMClient, error) {
	if !cfg.Enabled {
		logger.L.Info("[FCM] Disabled by configuration")
		return &disabledClient{}, nil
	}

	var opts []option.ClientOption

	if cfg.CredentialsJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(cfg.CredentialsJSON)))
	} else if cfg.CredentialsPath != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsPath))
	}

	app, err := firebase.NewApp(context.Background(), &firebase.Config{
		ProjectID: cfg.ProjectID,
	}, opts...)
	if err != nil {
		logger.L.Error("[FCM] Failed to create Firebase app, disabling FCM", zap.Error(err))
		return &disabledClient{}, nil
	}

	msgClient, err := app.Messaging(context.Background())
	if err != nil {
		logger.L.Error("[FCM] Failed to create messaging client, disabling FCM", zap.Error(err))
		return &disabledClient{}, nil
	}

	if cfg.DryRun {
		logger.L.Info("[FCM] Enabled (dry run mode — messages will be validated but not delivered)")
	} else {
		logger.L.Info("[FCM] Enabled and ready to send push notifications")
	}

	return &realClient{
		app:    app,
		client: msgClient,
		cfg:    cfg,
	}, nil
}

func (c *realClient) SendToToken(ctx context.Context, token, title, body string, data map[string]string) (string, error) {
	msg := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	if c.cfg.DryRun {
		msg.Token = token
		return c.client.SendDryRun(ctx, msg)
	}

	resp, err := c.client.Send(ctx, msg)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (c *realClient) SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) (string, error) {
	msg := &messaging.Message{
		Topic: topic,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	if c.cfg.DryRun {
		return c.client.SendDryRun(ctx, msg)
	}

	resp, err := c.client.Send(ctx, msg)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (c *realClient) SubscribeToTopic(ctx context.Context, topic string, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	_, err := c.client.SubscribeToTopic(ctx, tokens, topic)
	return err
}

func (c *realClient) UnsubscribeFromTopic(ctx context.Context, topic string, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	_, err := c.client.UnsubscribeFromTopic(ctx, tokens, topic)
	return err
}

type disabledClient struct{}

func (d *disabledClient) IsEnabled() bool  { return false }
func (d *disabledClient) IsDryRun() bool    { return false }
func (d *disabledClient) SendToToken(ctx context.Context, token, title, body string, data map[string]string) (string, error) {
	return "", fmt.Errorf("FCM is disabled")
}
func (d *disabledClient) SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) (string, error) {
	return "", fmt.Errorf("FCM is disabled")
}
func (d *disabledClient) SubscribeToTopic(ctx context.Context, topic string, tokens []string) error {
	return fmt.Errorf("FCM is disabled")
}
func (d *disabledClient) UnsubscribeFromTopic(ctx context.Context, topic string, tokens []string) error {
	return fmt.Errorf("FCM is disabled")
}

var _ FCMClient = (*realClient)(nil)
var _ FCMClient = (*disabledClient)(nil)
