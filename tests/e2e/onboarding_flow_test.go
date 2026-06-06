package e2e

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

func setupOnboardingTestRouter(t *testing.T, db *gorm.DB) (*gin.Engine, uuid.UUID) {
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

func TestOnboardingFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupOnboardingTestRouter(t, db)

	t.Run("GET /api/onboarding/status before onboarding", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/onboarding/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		hasCompleted, ok := data["has_completed_onboarding"].(bool)
		if !ok {
			t.Fatal("expected has_completed_onboarding in response")
		}

		if hasCompleted {
			t.Error("expected has_completed_onboarding to be false")
		}
	})

	t.Run("POST /api/onboarding with valid payload", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"display_name":  "Minh Thanh",
			"age":           25,
			"gender":        "Nam",
			"height_cm":     170,
			"weight_kg":     65,
			"main_activity": "Software Engineer",
			"main_goals":    []string{"Uống nước", "Học tập", "Vận động"},
			"quiet_after_time": "22:00",
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
	})

	t.Run("GET /api/onboarding/status after onboarding", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/onboarding/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		hasCompleted, ok := data["has_completed_onboarding"].(bool)
		if !ok {
			t.Fatal("expected has_completed_onboarding in response")
		}

		if !hasCompleted {
			t.Error("expected has_completed_onboarding to be true")
		}
	})

	t.Run("GET /api/users/me after onboarding", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/users/me", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		user, ok := data["user"].(map[string]interface{})
		if !ok {
			t.Fatal("expected user in response")
		}

		if user["display_name"] != "Minh Thanh" {
			t.Errorf("expected display_name 'Minh Thanh', got '%s'", user["display_name"])
		}

		if !user["has_completed_onboarding"].(bool) {
			t.Error("expected has_completed_onboarding to be true")
		}
	})

	_ = userID
}
