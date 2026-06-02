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

func setupCheckinHandlerRouter(t *testing.T, db *gorm.DB) (*gin.Engine, uuid.UUID) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	userService := services.NewUserService(db)
	checkinService := services.NewDailyCheckinService(db)
	reviewService := services.NewDailyReviewService(db)

	userHandler := handlers.NewUserHandlerWithDaily(userService, checkinService, reviewService)
	checkinHandler := handlers.NewDailyCheckinHandler(checkinService)

	userID := testutils.BootstrapTestUser(t, db)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		users := protected.Group("/api/users")
		{
			users.GET("/me/daily-status", userHandler.GetDailyStatus)
		}

		checkins := protected.Group("/api/checkins")
		{
			checkins.GET("/today", checkinHandler.GetToday)
			checkins.GET("", checkinHandler.GetByDate)
			checkins.POST("", checkinHandler.Save)
		}
	}

	return r, userID
}

func TestCheckinGetToday_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupCheckinHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/checkins/today", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["has_checked_in"] != false {
		t.Error("expected has_checked_in to be false")
	}
}

func TestCheckinSave_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupCheckinHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"energy_level":          "high",
		"stress_level":          "low",
		"focus_level":           "medium",
		"day_intensity":         "normal",
		"main_focus_today":      "Backend work",
		"note":                  "Good day",
		"available_time_blocks": []string{"morning", "evening"},
	})

	req, _ := http.NewRequest("POST", "/api/checkins", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["message"] != "daily check-in saved successfully" {
		t.Errorf("expected success message, got '%s'", resp["message"])
	}
}

func TestCheckinSave_InvalidEnum_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupCheckinHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"energy_level":  "invalid",
		"stress_level":  "low",
		"focus_level":   "medium",
		"day_intensity": "normal",
	})

	req, _ := http.NewRequest("POST", "/api/checkins", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDailyStatus_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupCheckinHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/users/me/daily-status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["has_checked_in_today"] != false {
		t.Error("expected has_checked_in_today to be false initially")
	}
	if resp["has_reviewed_today"] != false {
		t.Error("expected has_reviewed_today to be false initially")
	}
}
