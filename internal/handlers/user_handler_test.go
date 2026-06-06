package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupUserHandlerRouter(t *testing.T, db *gorm.DB) (*gin.Engine, uuid.UUID) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	userService := services.NewUserService(db)
	onboardingService := services.NewOnboardingService(db)

	userHandler := handlers.NewUserHandler(userService)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingService)

	userID := testutils.BootstrapTestUser(t, db)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		users := protected.Group("/api/users")
		{
			users.GET("/me", userHandler.GetMe)
		}

		onboarding := protected.Group("/api/onboarding")
		{
			onboarding.POST("", onboardingHandler.SaveOnboarding)
			onboarding.GET("/status", onboardingHandler.GetOnboardingStatus)
		}
	}

	return r, userID
}

func TestGetMeReturnsUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupUserHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/users/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	user, ok := unwrapData(t, resp)["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user in response")
	}

	if user["display_name"] != "Test User" {
		t.Errorf("expected display_name 'Test User', got '%s'", user["display_name"])
	}
}

func TestGetOnboardingStatusReturnsFalseBeforeOnboarding(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupUserHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/onboarding/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	hasCompleted, ok := unwrapData(t, resp)["has_completed_onboarding"].(bool)
	if !ok {
		t.Fatal("expected has_completed_onboarding in response")
	}

	if hasCompleted {
		t.Error("expected has_completed_onboarding to be false before onboarding")
	}
}

func TestSaveOnboardingReturns200ForValidPayload(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupUserHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"display_name": "Minh Thanh",
		"gender":       "Nam",
		"main_activity": "Engineer",
		"main_goals":   []string{"Health"},
	})

	req, _ := http.NewRequest("POST", "/api/onboarding", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["message"] != "onboarding saved successfully" {
		t.Errorf("expected success message, got '%s'", resp["message"])
	}

	profile, ok := unwrapData(t, resp)["profile"].(map[string]interface{})
	if !ok {
		t.Fatal("expected profile in response")
	}

	if !profile["has_completed_onboarding"].(bool) {
		t.Error("expected has_completed_onboarding to be true")
	}
}

func TestSaveOnboardingReturns400ForInvalidPayload(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupUserHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"gender": "Nam",
	})

	req, _ := http.NewRequest("POST", "/api/onboarding", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestGetOnboardingStatusReturnsTrueAfterOnboarding(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupUserHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"display_name": "Minh Thanh",
		"gender":       "Nam",
		"main_activity": "Engineer",
		"main_goals":   []string{"Health"},
	})

	req, _ := http.NewRequest("POST", "/api/onboarding", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("onboarding failed: %d", w.Code)
	}

	req, _ = http.NewRequest("GET", "/api/onboarding/status", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	hasCompleted, ok := unwrapData(t, resp)["has_completed_onboarding"].(bool)
	if !ok {
		t.Fatal("expected has_completed_onboarding in response")
	}

	if !hasCompleted {
		t.Error("expected has_completed_onboarding to be true after onboarding")
	}
}
