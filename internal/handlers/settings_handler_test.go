package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupSettingsRouter(t *testing.T) (*gin.Engine, uuid.UUID) {
	t.Helper()

	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })

	userID := testutils.BootstrapTestUser(t, db)

	settings := models.AppSettings{
		UserID:               userID,
		Locale:               "vi",
		Theme:                "dark",
		DailyQuestLimit:      10,
		NotificationsEnabled: true,
		QuietAfterTime:       "22:00",
		Timezone:             "Asia/Ho_Chi_Minh",
	}
	db.Create(&settings)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	settingsService := services.NewSettingsService(db)
	settingsHandler := handlers.NewSettingsHandler(settingsService)

	r.GET("/api/settings", testutils.AuthMiddleware(userID), settingsHandler.GetSettings)
	r.PATCH("/api/settings", testutils.AuthMiddleware(userID), settingsHandler.UpdateSettings)

	return r, userID
}

func TestGetSettings_ReturnsSettings(t *testing.T) {
	r, _ := setupSettingsRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	settings, ok := unwrapData(t, response)["settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected settings object in response")
	}

	if settings["quiet_after_time"] != "22:00" {
		t.Errorf("expected quiet_after_time '22:00', got '%v'", settings["quiet_after_time"])
	}

	if settings["notifications_enabled"] != true {
		t.Errorf("expected notifications_enabled true, got '%v'", settings["notifications_enabled"])
	}
}

func TestPatchSettings_UpdateNotificationsEnabled(t *testing.T) {
	r, _ := setupSettingsRouter(t)

	body := map[string]interface{}{
		"notifications_enabled": false,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	settings, ok := unwrapData(t, response)["settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected settings in response")
	}

	if settings["notifications_enabled"] != false {
		t.Errorf("expected notifications_enabled false after patch, got '%v'", settings["notifications_enabled"])
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var getResponse map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &getResponse)
	getSettings := unwrapData(t, getResponse)["settings"].(map[string]interface{})

	if getSettings["notifications_enabled"] != false {
		t.Errorf("expected persisted notifications_enabled false, got '%v'", getSettings["notifications_enabled"])
	}
}

func TestPatchSettings_UpdateQuietHours(t *testing.T) {
	r, _ := setupSettingsRouter(t)

	body := map[string]interface{}{
		"quiet_hours_enabled": true,
		"quiet_start_time":    "22:00",
		"quiet_end_time":      "07:00",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	settings := unwrapData(t, response)["settings"].(map[string]interface{})
	if settings["quiet_hours_enabled"] != true {
		t.Errorf("expected quiet_hours_enabled true, got '%v'", settings["quiet_hours_enabled"])
	}
	if settings["quiet_start_time"] != "22:00" {
		t.Errorf("expected quiet_start_time '22:00', got '%v'", settings["quiet_start_time"])
	}
	if settings["quiet_end_time"] != "07:00" {
		t.Errorf("expected quiet_end_time '07:00', got '%v'", settings["quiet_end_time"])
	}
}

func TestPatchSettings_UpdateQuietAfterTime(t *testing.T) {
	r, _ := setupSettingsRouter(t)

	body := map[string]interface{}{
		"quiet_after_time": "23:00",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	settings := unwrapData(t, response)["settings"].(map[string]interface{})

	if settings["quiet_after_time"] != "23:00" {
		t.Errorf("expected quiet_after_time '23:00', got '%v'", settings["quiet_after_time"])
	}
}

func TestPatchSettings_UpdateDailyReminderTime(t *testing.T) {
	r, _ := setupSettingsRouter(t)

	body := map[string]interface{}{
		"daily_reminder_time": "09:00",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	settings := unwrapData(t, response)["settings"].(map[string]interface{})

	if settings["daily_reminder_time"] != "09:00" {
		t.Errorf("expected daily_reminder_time '09:00', got '%v'", settings["daily_reminder_time"])
	}
}

func TestPatchSettings_RejectsInvalidTimeFormat(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
	}{
		{"invalid daily_reminder_time", "daily_reminder_time", "25:00"},
		{"invalid quiet_after_time", "quiet_after_time", "9:00"},
		{"invalid quiet_start_time", "quiet_start_time", "abc"},
		{"invalid quiet_end_time", "quiet_end_time", "24:60"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := setupSettingsRouter(t)

			body := map[string]interface{}{tt.field: tt.value}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPatch, "/api/settings", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400 for invalid %s, got %d", tt.field, w.Code)
			}
		})
	}
}

func TestPatchSettings_PartialUpdate(t *testing.T) {
	r, _ := setupSettingsRouter(t)

	body := map[string]interface{}{
		"notifications_enabled": false,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	settings := unwrapData(t, response)["settings"].(map[string]interface{})

	if settings["quiet_after_time"] != "22:00" {
		t.Errorf("unchanged quiet_after_time should remain '22:00', got '%v'", settings["quiet_after_time"])
	}
	if settings["notifications_enabled"] != false {
		t.Errorf("expected notifications_enabled false, got '%v'", settings["notifications_enabled"])
	}
}
