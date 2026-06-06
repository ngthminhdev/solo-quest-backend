package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupReminderSettingsRouter(t *testing.T) (*gin.Engine, uuid.UUID) {
	t.Helper()

	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })

	userID := testutils.BootstrapTestUser(t, db)

	reminderSettingService := services.NewReminderSettingService(db)
	reminderSettingService.EnsureDefaultReminderSettingsForUser(userID)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	reminderSettingsHandler := handlers.NewReminderSettingsHandler(reminderSettingService)

	r.GET("/api/settings/reminders", testutils.AuthMiddleware(userID), reminderSettingsHandler.GetReminderSettings)
	r.PATCH("/api/settings/reminders/:type", testutils.AuthMiddleware(userID), reminderSettingsHandler.UpdateReminderSetting)
	r.PATCH("/api/settings/reminders/:type/toggle", testutils.AuthMiddleware(userID), reminderSettingsHandler.ToggleReminderSetting)

	return r, userID
}

func TestGetReminderSettings_ReturnsDefaults(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/reminders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}

	if len(data) != 7 {
		t.Errorf("expected 7 seeded reminder settings, got %d", len(data))
	}

	typesFound := make(map[string]bool)
	for _, r := range data {
		reminder := r.(map[string]interface{})
		t, _ := reminder["type"].(string)
		typesFound[t] = true
	}

	expectedTypes := []string{"water", "break_time", "movement", "learning", "sleep", "daily_review", "custom"}
	for _, et := range expectedTypes {
		if !typesFound[et] {
			t.Errorf("expected type '%s' in reminders", et)
		}
	}
}

func TestGetReminderSettings_CreatesDefaultsIfMissing(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })

	userID := testutils.BootstrapTestUser(t, db)

	reminderSettingService := services.NewReminderSettingService(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	reminderSettingsHandler := handlers.NewReminderSettingsHandler(reminderSettingService)
	r.GET("/api/settings/reminders", testutils.AuthMiddleware(userID), reminderSettingsHandler.GetReminderSettings)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/reminders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array in response, got: %v", response["data"])
	}

	if len(data) != 7 {
		t.Errorf("expected 7 default settings created on GET, got %d", len(data))
	}
}

func TestGetReminderSettings_DoesNotDuplicateOnRepeatedCalls(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/settings/reminders", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("call %d: failed to unmarshal: %v", i+1, err)
		}
		data, ok := response["data"].([]interface{})
		if !ok {
			t.Fatalf("call %d: expected data array, got: %v", i+1, response["data"])
		}

		if len(data) != 7 {
			t.Errorf("call %d: expected 7 settings, got %d", i+1, len(data))
		}
	}
}

func TestGetReminderSettings_ReturnsSnakeCaseTypes(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/reminders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].([]interface{})

	for _, r := range data {
		reminder := r.(map[string]interface{})
		reminderType, _ := reminder["type"].(string)
		if strings.Contains(reminderType, "breakTime") || strings.Contains(reminderType, "dailyReview") {
			t.Errorf("expected snake_case type, got '%s'", reminderType)
		}
	}
}

func TestGetReminderSettings_LearningSettingsExist(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/reminders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].([]interface{})

	var learningSetting map[string]interface{}
	for _, r := range data {
		reminder := r.(map[string]interface{})
		if reminder["type"] == "learning" {
			learningSetting = reminder
			break
		}
	}

	if learningSetting == nil {
		t.Fatal("expected learning reminder setting in response")
	}

	if learningSetting["start_time"] != "20:00" {
		t.Errorf("expected learning start_time '20:00', got '%v'", learningSetting["start_time"])
	}
	if learningSetting["status"] != "enabled" {
		t.Errorf("expected learning status 'enabled', got '%v'", learningSetting["status"])
	}
	if learningSetting["title"] != "Học tập" {
		t.Errorf("expected learning title 'Học tập', got '%v'", learningSetting["title"])
	}
}

func TestPatchReminderSetting_UpdateWaterInterval(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"frequency":        "interval",
		"interval_minutes": 60,
		"start_time":       "08:00",
		"end_time":         "22:00",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/water", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	reminder := unwrapData(t, response)

	if reminder["frequency"] != "interval" {
		t.Errorf("expected frequency 'interval', got '%v'", reminder["frequency"])
	}
	if reminder["interval_minutes"].(float64) != 60 {
		t.Errorf("expected interval_minutes 60, got '%v'", reminder["interval_minutes"])
	}
}

func TestPatchReminderSetting_UpdateLearningStartTime(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"start_time": "19:00",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/learning", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	reminder := unwrapData(t, response)

	if reminder["start_time"] != "19:00" {
		t.Errorf("expected start_time '19:00', got '%v'", reminder["start_time"])
	}
}

func TestPatchReminderSetting_PartialUpdatePreservesOtherFields(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"interval_minutes": 120,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/water", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	reminder := unwrapData(t, response)

	if reminder["frequency"] != "interval" {
		t.Errorf("expected frequency 'interval' preserved, got '%v'", reminder["frequency"])
	}
	if reminder["status"] != "enabled" {
		t.Errorf("expected status 'enabled' preserved, got '%v'", reminder["status"])
	}
}

func TestPatchReminderSetting_InvalidType(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"start_time": "10:00",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/invalid_type", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid type, got %d", w.Code)
	}
}

func TestPatchReminderSetting_InvalidFrequency(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"frequency": "yearly",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/learning", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid frequency, got %d", w.Code)
	}
}

func TestPatchReminderSetting_InvalidStatus(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"status": "active",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/learning", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid status, got %d", w.Code)
	}
}

func TestPatchReminderSetting_InvalidHHMM(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
	}{
		{"invalid start_time", "start_time", "25:00"},
		{"invalid end_time", "end_time", "abc"},
		{"invalid end_time minutes", "end_time", "09:60"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := setupReminderSettingsRouter(t)

			body := map[string]interface{}{tt.field: tt.value}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/learning", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400 for invalid %s, got %d", tt.field, w.Code)
			}
		})
	}
}

func TestPatchReminderSetting_RejectsNegativeInterval(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"interval_minutes": -5,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/water", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative interval, got %d: %s", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "positive") {
		t.Errorf("error message should mention 'positive', got: %s", w.Body.String())
	}
}

func TestPatchReminderSetting_RejectsZeroMaxPerDay(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"max_per_day": 0,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/water", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for zero max_per_day, got %d: %s", w.Code, w.Body.String())
	}
}

func TestToggleReminderSetting_DisableLearning(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"status": "disabled",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/learning/toggle", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	reminder := unwrapData(t, response)

	if reminder["status"] != "disabled" {
		t.Errorf("expected status 'disabled', got '%v'", reminder["status"])
	}
}

func TestToggleReminderSetting_ReEnableLearning(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body1 := map[string]interface{}{"status": "disabled"}
	jsonBody1, _ := json.Marshal(body1)
	req1 := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/learning/toggle", bytes.NewReader(jsonBody1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected status 200 for disable, got %d", w1.Code)
	}

	body2 := map[string]interface{}{"status": "enabled"}
	jsonBody2, _ := json.Marshal(body2)
	req2 := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/learning/toggle", bytes.NewReader(jsonBody2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200 for enable, got %d", w2.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &response)
	reminder := unwrapData(t, response)

	if reminder["status"] != "enabled" {
		t.Errorf("expected status 'enabled' after re-enable, got '%v'", reminder["status"])
	}
}

func TestToggleReminderSetting_InvalidType(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{"status": "disabled"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/invalid_type/toggle", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid type, got %d", w.Code)
	}
}

func TestToggleReminderSetting_InvalidStatus(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{"status": "active"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/learning/toggle", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid status, got %d", w.Code)
	}
}

func TestGetReminderSettings_ReturnsOnlyCurrentUserSettings(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/reminders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].([]interface{})

	if len(data) != 7 {
		t.Errorf("expected 7 settings for authenticated user, got %d", len(data))
	}

	for _, r := range data {
		reminder := r.(map[string]interface{})
		if _, ok := reminder["type"]; !ok {
			t.Error("each reminder should have a 'type' field")
		}
	}
}

func TestPatchReminderSetting_ResponseShapeMatchesFE(t *testing.T) {
	r, _ := setupReminderSettingsRouter(t)

	body := map[string]interface{}{
		"frequency":       "interval",
		"start_time":      "08:00",
		"end_time":        "22:00",
		"interval_minutes": 60,
		"max_per_day":     5,
		"smart_enabled":   true,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/settings/reminders/water", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	reminder := unwrapData(t, response)

	if _, ok := reminder["id"]; !ok {
		t.Error("response missing 'id' field")
	}
	if _, ok := reminder["type"]; !ok {
		t.Error("response missing 'type' field")
	}
	if _, ok := reminder["title"]; !ok {
		t.Error("response missing 'title' field")
	}
	if _, ok := reminder["description"]; !ok {
		t.Error("response missing 'description' field")
	}
	if _, ok := reminder["frequency"]; !ok {
		t.Error("response missing 'frequency' field")
	}
	if _, ok := reminder["status"]; !ok {
		t.Error("response missing 'status' field")
	}
	if _, ok := reminder["start_time"]; !ok {
		t.Error("response missing 'start_time' field")
	}
	if _, ok := reminder["end_time"]; !ok {
		t.Error("response missing 'end_time' field")
	}
	if _, ok := reminder["interval_minutes"]; !ok {
		t.Error("response missing 'interval_minutes' field")
	}
	if _, ok := reminder["max_per_day"]; !ok {
		t.Error("response missing 'max_per_day' field")
	}
	if _, ok := reminder["smart_enabled"]; !ok {
		t.Error("response missing 'smart_enabled' field")
	}
}
