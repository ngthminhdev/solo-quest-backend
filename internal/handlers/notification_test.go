package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/infra/fcm"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

type mockFCMClient struct {
	mu           sync.Mutex
	enabled      bool
	sentTokens   []string
	subscribed   []string
	unsubscribed []string
}

func newMockFCMClient(enabled bool) *mockFCMClient {
	return &mockFCMClient{enabled: enabled}
}

func (m *mockFCMClient) IsEnabled() bool { return m.enabled }
func (m *mockFCMClient) IsDryRun() bool  { return false }
func (m *mockFCMClient) SendToToken(ctx context.Context, token, title, body string, data map[string]string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentTokens = append(m.sentTokens, token)
	return "msg-id-" + token, nil
}
func (m *mockFCMClient) SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) (string, error) {
	return "", nil
}
func (m *mockFCMClient) SubscribeToTopic(ctx context.Context, topic string, tokens []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribed = append(m.subscribed, tokens...)
	return nil
}
func (m *mockFCMClient) UnsubscribeFromTopic(ctx context.Context, topic string, tokens []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unsubscribed = append(m.unsubscribed, tokens...)
	return nil
}

var _ fcm.FCMClient = (*mockFCMClient)(nil)

func createNotificationRouter(t *testing.T, db *gorm.DB, userID uuid.UUID, fcmEnabled bool) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	tokenSvc := services.NewDeviceTokenService(db)
	mockClient := newMockFCMClient(fcmEnabled)
	notifSvc := services.NewNotificationService(mockClient, tokenSvc)
	handler := handlers.NewNotificationHandler(tokenSvc, notifSvc)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		notifications := protected.Group("/api/notifications")
		{
			notifications.POST("/tokens", handler.RegisterToken)
			notifications.GET("/tokens", handler.GetMyTokens)
			notifications.DELETE("/tokens/:id", handler.RemoveToken)
			notifications.POST("/test", handler.SendTest)
		}
	}

	return r
}

func TestRegisterToken(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := createNotificationRouter(t, db, userID, true)

	body := map[string]interface{}{
		"token":    "test-fcm-token-handler",
		"platform": "android",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/tokens", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGetMyTokens(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := createNotificationRouter(t, db, userID, true)

	tokenSvc := services.NewDeviceTokenService(db)
	tokenSvc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "token-list-1", Platform: "android"})
	tokenSvc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "token-list-2", Platform: "ios"})

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/tokens", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	tokens := data["tokens"].([]interface{})
	assert.Len(t, tokens, 2)
}

func TestRemoveToken(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := createNotificationRouter(t, db, userID, true)

	tokenSvc := services.NewDeviceTokenService(db)
	token, _ := tokenSvc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "token-remove", Platform: "android"})

	req := httptest.NewRequest(http.MethodDelete, "/api/notifications/tokens/"+token.ID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	tokens, _ := tokenSvc.GetActiveTokensByUserID(userID)
	assert.Len(t, tokens, 0)
}

func TestRemoveTokenNotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := createNotificationRouter(t, db, userID, true)

	req := httptest.NewRequest(http.MethodDelete, "/api/notifications/tokens/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSendTestNotificationDisabled(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := createNotificationRouter(t, db, userID, false)

	body := map[string]interface{}{
		"title": "Test",
		"body":  "Hello",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSendTestNotification(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := createNotificationRouter(t, db, userID, true)

	tokenSvc := services.NewDeviceTokenService(db)
	tokenSvc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "test-send-token", Platform: "android"})

	body := map[string]interface{}{
		"title": "Test Title",
		"body":  "Test Body",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID1 := testutils.BootstrapTestUser(t, db)
	userID2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testutils.CreateTestUser(db, userID2, "test2@test.com")

	tokenSvc := services.NewDeviceTokenService(db)
	token1, _ := tokenSvc.UpsertToken(userID1, &dto.RegisterDeviceTokenRequest{Token: "user1-token", Platform: "android"})
	token2, _ := tokenSvc.UpsertToken(userID2, &dto.RegisterDeviceTokenRequest{Token: "user2-token", Platform: "ios"})

	r := createNotificationRouter(t, db, userID1, true)

	req := httptest.NewRequest(http.MethodDelete, "/api/notifications/tokens/"+token2.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	tokens1, _ := tokenSvc.GetActiveTokensByUserID(userID1)
	tokens2, _ := tokenSvc.GetActiveTokensByUserID(userID2)
	assert.Len(t, tokens1, 1)
	assert.Equal(t, token1.ID, tokens1[0].ID)
	assert.Len(t, tokens2, 1)
	assert.Equal(t, token2.ID, tokens2[0].ID)

	_ = token2
}

func TestRegisterTokenInvalidPlatform(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := createNotificationRouter(t, db, userID, true)

	body := map[string]interface{}{
		"token":    "test-token",
		"platform": "windows",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/tokens", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
