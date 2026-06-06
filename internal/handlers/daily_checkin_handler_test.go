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

func unwrapData(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, ok := resp["data"]
	if !ok {
		t.Fatal("response missing 'data' field")
	}
	d, ok := data.(map[string]interface{})
	if !ok {
		t.Fatal("response 'data' is not an object")
	}
	return d
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

	if unwrapData(t, resp)["has_checked_in"] != false {
		t.Error("expected has_checked_in to be false")
	}
}

func TestCheckinSave_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupCheckinHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":         "good",
		"energy_level": "high",
		"availability": "free",
		"priority":     "learning",
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
		"mood":         "normal",
		"energy_level": "invalid",
		"availability": "normal",
		"priority":     "learning",
	})

	req, _ := http.NewRequest("POST", "/api/checkins", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCheckinSave_MissingRequiredField_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupCheckinHandlerRouter(t, db)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"missing mood", map[string]interface{}{"energy_level": "medium", "availability": "normal", "priority": "learning"}},
		{"missing energy_level", map[string]interface{}{"mood": "normal", "availability": "normal", "priority": "learning"}},
		{"missing availability", map[string]interface{}{"mood": "normal", "energy_level": "medium", "priority": "learning"}},
		{"missing priority", map[string]interface{}{"mood": "normal", "energy_level": "medium", "availability": "normal"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("POST", "/api/checkins", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400 for %s, got %d: %s", tt.name, w.Code, w.Body.String())
			}
		})
	}
}

func TestCheckinSave_OldFieldsNotRequired_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupCheckinHandlerRouter(t, db)

	// Only the 4 new fields, no old fields
	body, _ := json.Marshal(map[string]interface{}{
		"mood":         "normal",
		"energy_level": "medium",
		"availability": "normal",
		"priority":     "learning",
	})

	req, _ := http.NewRequest("POST", "/api/checkins", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCheckinSave_ResponseIncludesNewFields(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupCheckinHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":         "good",
		"energy_level": "high",
		"availability": "free",
		"priority":     "learning",
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

	item := unwrapData(t, resp)["item"].(map[string]interface{})
	if item["mood"] != "good" {
		t.Errorf("expected mood 'good', got '%v'", item["mood"])
	}
	if item["energy_level"] != "high" {
		t.Errorf("expected energy_level 'high', got '%v'", item["energy_level"])
	}
	if item["availability"] != "free" {
		t.Errorf("expected availability 'free', got '%v'", item["availability"])
	}
	if item["priority"] != "learning" {
		t.Errorf("expected priority 'learning', got '%v'", item["priority"])
	}
}

func TestCheckinGetToday_ResponseIncludesNewFields(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupCheckinHandlerRouter(t, db)

	// First create a check-in
	body, _ := json.Marshal(map[string]interface{}{
		"mood":         "good",
		"energy_level": "high",
		"availability": "free",
		"priority":     "health",
	})
	createReq, _ := http.NewRequest("POST", "/api/checkins", bytes.NewBuffer(body))
	createReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, createReq)

	// Then get today
	req, _ := http.NewRequest("GET", "/api/checkins/today", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if unwrapData(t, resp)["has_checked_in"] != true {
		t.Error("expected has_checked_in to be true")
	}

	item := unwrapData(t, resp)["item"].(map[string]interface{})
	if item["mood"] != "good" {
		t.Errorf("expected mood 'good', got '%v'", item["mood"])
	}
	if item["energy_level"] != "high" {
		t.Errorf("expected energy_level 'high', got '%v'", item["energy_level"])
	}
	if item["availability"] != "free" {
		t.Errorf("expected availability 'free', got '%v'", item["availability"])
	}
	if item["priority"] != "health" {
		t.Errorf("expected priority 'health', got '%v'", item["priority"])
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

	if unwrapData(t, resp)["has_checked_in_today"] != false {
		t.Error("expected has_checked_in_today to be false initially")
	}
	if unwrapData(t, resp)["has_reviewed_today"] != false {
		t.Error("expected has_reviewed_today to be false initially")
	}
}
