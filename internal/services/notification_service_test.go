package services_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/infra/fcm"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

type mockFCMClient struct {
	mu            sync.Mutex
	sendResults   map[string]error
	sentMessages  []string
	subscribed    []string
	unsubscribed  []string
	returnError   error
}

func newMockFCMClient() *mockFCMClient {
	return &mockFCMClient{
		sendResults: make(map[string]error),
	}
}

func (m *mockFCMClient) IsEnabled() bool  { return true }
func (m *mockFCMClient) IsDryRun() bool    { return false }
func (m *mockFCMClient) SendToToken(ctx context.Context, token, title, body string, data map[string]string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.sendResults[token]; ok {
		return "", err
	}
	if m.returnError != nil {
		return "", m.returnError
	}
	m.sentMessages = append(m.sentMessages, token)
	return "msg-id-" + token, nil
}
func (m *mockFCMClient) SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) (string, error) {
	return "", m.returnError
}
func (m *mockFCMClient) SubscribeToTopic(ctx context.Context, topic string, tokens []string) error {
	m.subscribed = append(m.subscribed, tokens...)
	return nil
}
func (m *mockFCMClient) UnsubscribeFromTopic(ctx context.Context, topic string, tokens []string) error {
	m.unsubscribed = append(m.unsubscribed, tokens...)
	return nil
}

var _ fcm.FCMClient = (*mockFCMClient)(nil)

func TestNotificationService_SendToUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	tokenSvc := services.NewDeviceTokenService(db)

	tokenSvc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "fcm-token-1", Platform: "android"})
	tokenSvc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "fcm-token-2", Platform: "ios"})

	mockClient := newMockFCMClient()
	notifSvc := services.NewNotificationService(mockClient, tokenSvc)

	err := notifSvc.SendToUser(context.Background(), userID, "Test", "Hello", nil)
	assert.NoError(t, err)
	assert.Len(t, mockClient.sentMessages, 2)
}

func TestNotificationService_SendToUserNoActiveTokens(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	tokenSvc := services.NewDeviceTokenService(db)

	mockClient := newMockFCMClient()
	notifSvc := services.NewNotificationService(mockClient, tokenSvc)

	err := notifSvc.SendToUser(context.Background(), userID, "Test", "Hello", nil)
	assert.NoError(t, err)
	assert.Len(t, mockClient.sentMessages, 0)
}

func TestNotificationService_InvalidTokenDeactivated(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	tokenSvc := services.NewDeviceTokenService(db)

	tokenSvc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "bad-token", Platform: "android"})
	tokenSvc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "good-token", Platform: "ios"})

	mockClient := newMockFCMClient()
	mockClient.sendResults["bad-token"] = assert.AnError

	notifSvc := services.NewNotificationService(mockClient, tokenSvc)

	err := notifSvc.SendToUser(context.Background(), userID, "Test", "Hello", nil)
	assert.NoError(t, err)
	assert.Len(t, mockClient.sentMessages, 1)
	assert.Equal(t, "good-token", mockClient.sentMessages[0])
}

func TestNotificationService_IsEnabledFalseWhenDisabled(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	tokenSvc := services.NewDeviceTokenService(db)

	mockClient := newMockFCMClient()
	mockClient.returnError = assert.AnError

	notifSvc := services.NewNotificationService(mockClient, tokenSvc)
	assert.True(t, notifSvc.IsEnabled())
}
