package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"

	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupSmokeRouter(t *testing.T, db *gorm.DB) func(string, string, []byte) *httptest.ResponseRecorder {
	t.Helper()

	userID := testutils.BootstrapTestUser(t, db)

	reminderSettingService := services.NewReminderSettingService(db)
	reminderSettingService.EnsureDefaultReminderSettingsForUser(userID)

	r := testutils.CreateTestRouter(t, db, userID)

	return func(method, path string, body []byte) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			req, _ = http.NewRequest(method, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, path, nil)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
}

func TestRouteContract_QuestSettingsEndpoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	send := setupSmokeRouter(t, db)

	endpoints := []struct {
		method       string
		path         string
		body         map[string]interface{}
		expectedCode int
		label        string
	}{
		{"GET", "/api/quest-settings", nil, http.StatusOK, "GET /api/quest-settings"},
		{"PATCH", "/api/quest-settings", map[string]interface{}{"difficulty": "easy"}, http.StatusOK, "PATCH /api/quest-settings"},
		{"PUT", "/api/quest-settings", map[string]interface{}{"difficulty": "hard"}, http.StatusOK, "PUT /api/quest-settings (backward compat)"},
		{"POST", "/api/quest-settings/reset", nil, http.StatusOK, "POST /api/quest-settings/reset"},
	}

	for _, ep := range endpoints {
		t.Run(ep.label, func(t *testing.T) {
			var jsonBody []byte
			if ep.body != nil {
				jsonBody, _ = json.Marshal(ep.body)
			}
			w := send(ep.method, ep.path, jsonBody)

			if w.Code != ep.expectedCode {
				t.Errorf("%s: expected status %d, got %d: %s", ep.label, ep.expectedCode, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if code, ok := resp["code"].(float64); !ok || int(code) != ep.expectedCode {
				t.Errorf("%s: expected envelope code %d, got %v", ep.label, ep.expectedCode, resp["code"])
			}
			if _, ok := resp["message"]; !ok {
				t.Errorf("%s: response missing 'message' field", ep.label)
			}
		})
	}
}

func TestRouteContract_ReminderSettingsEndpoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	send := setupSmokeRouter(t, db)

	endpoints := []struct {
		method       string
		path         string
		body         map[string]interface{}
		expectedCode int
		label        string
	}{
		{"GET", "/api/settings/reminders", nil, http.StatusOK, "GET /api/settings/reminders"},
		{"PATCH", "/api/settings/reminders/water", map[string]interface{}{"interval_minutes": 60}, http.StatusOK, "PATCH /api/settings/reminders/:type"},
		{"PATCH", "/api/settings/reminders/water/toggle", map[string]interface{}{"status": "disabled"}, http.StatusOK, "PATCH /api/settings/reminders/:type/toggle"},
	}

	for _, ep := range endpoints {
		t.Run(ep.label, func(t *testing.T) {
			var jsonBody []byte
			if ep.body != nil {
				jsonBody, _ = json.Marshal(ep.body)
			}
			w := send(ep.method, ep.path, jsonBody)

			if w.Code != ep.expectedCode {
				t.Errorf("%s: expected status %d, got %d: %s", ep.label, ep.expectedCode, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("%s: failed to unmarshal: %v", ep.label, err)
			}
			if code, ok := resp["code"].(float64); !ok || int(code) != ep.expectedCode {
				t.Errorf("%s: expected envelope code %d, got %v", ep.label, ep.expectedCode, resp["code"])
			}
			if _, ok := resp["message"]; !ok {
				t.Errorf("%s: response missing 'message' field", ep.label)
			}
		})
	}
}

func TestRouteContract_ReminderListResponseShape(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	send := setupSmokeRouter(t, db)

	w := send("GET", "/api/settings/reminders", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	code, ok := resp["code"].(float64)
	if !ok || int(code) != 200 {
		t.Errorf("expected code 200, got %v", resp["code"])
	}
	if msg, ok := resp["message"].(string); !ok || msg != "Success" {
		t.Errorf("expected message 'Success', got '%v'", resp["message"])
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in reminder list response")
	}
	if len(data) < 1 {
		t.Fatal("expected at least 1 reminder setting")
	}

	first := data[0].(map[string]interface{})
	requiredFields := []string{"id", "type", "status"}
	for _, f := range requiredFields {
		if _, ok := first[f]; !ok {
			t.Errorf("reminder list item missing field '%s'", f)
		}
	}
}

func TestRouteContract_ReminderToggleResponseShape(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	send := setupSmokeRouter(t, db)

	body := map[string]interface{}{"status": "enabled"}
	jsonBody, _ := json.Marshal(body)

	w := send("PATCH", "/api/settings/reminders/water/toggle", jsonBody)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if code, ok := resp["code"].(float64); !ok || int(code) != 200 {
		t.Errorf("expected code 200, got %v", resp["code"])
	}
	if _, ok := resp["message"]; !ok {
		t.Error("response missing 'message' field")
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in toggle response")
	}
	requiredFields := []string{"id", "type", "status"}
	for _, f := range requiredFields {
		if _, ok := data[f]; !ok {
			t.Errorf("toggle response data missing field '%s'", f)
		}
	}
}

func TestRouteContract_ReminderUpdateResponseShape(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	send := setupSmokeRouter(t, db)

	body := map[string]interface{}{"interval_minutes": 45}
	jsonBody, _ := json.Marshal(body)

	w := send("PATCH", "/api/settings/reminders/water", jsonBody)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if code, ok := resp["code"].(float64); !ok || int(code) != 200 {
		t.Errorf("expected code 200, got %v", resp["code"])
	}
	if _, ok := resp["message"]; !ok {
		t.Error("response missing 'message' field")
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in update response")
	}
	requiredFields := []string{"id", "type", "status"}
	for _, f := range requiredFields {
		if _, ok := data[f]; !ok {
			t.Errorf("update response data missing field '%s'", f)
		}
	}
}

func TestRouteContract_InvalidRequestReturns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	send := setupSmokeRouter(t, db)

	body := []byte(`{"status":"enabled"}`)
	w := send("PATCH", "/api/settings/reminders/invalid_type/toggle", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected code 400, got %v", resp["code"])
	}
	if _, ok := resp["message"]; !ok {
		t.Error("error response missing 'message' field")
	}
}

func TestRouteContract_LearningRoadmapEndpoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	send := setupSmokeRouter(t, db)

	endpoints := []struct {
		method       string
		path         string
		body         map[string]interface{}
		expectedCode int
		label        string
	}{
		{"GET", "/api/learning-roadmaps", nil, http.StatusOK, "GET /api/learning-roadmaps"},
	}

	for _, ep := range endpoints {
		t.Run(ep.label, func(t *testing.T) {
			var jsonBody []byte
			if ep.body != nil {
				jsonBody, _ = json.Marshal(ep.body)
			}
			w := send(ep.method, ep.path, jsonBody)

			if w.Code != ep.expectedCode {
				t.Errorf("%s: expected status %d, got %d: %s", ep.label, ep.expectedCode, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if code, ok := resp["code"].(float64); !ok || int(code) != ep.expectedCode {
				t.Errorf("%s: expected envelope code %d, got %v", ep.label, ep.expectedCode, resp["code"])
			}
			if _, ok := resp["message"]; !ok {
				t.Errorf("%s: response missing 'message' field", ep.label)
			}
		})
	}
}

func TestRouteContract_LearningRoadmapListResponseShape(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	send := setupSmokeRouter(t, db)

	w := send("GET", "/api/learning-roadmaps", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	code, ok := resp["code"].(float64)
	if !ok || int(code) != 200 {
		t.Errorf("expected code 200, got %v", resp["code"])
	}
	if msg, ok := resp["message"].(string); !ok || msg != "Success" {
		t.Errorf("expected message 'Success', got '%v'", resp["message"])
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in learning roadmaps list response")
	}
	if _, ok := data["items"]; !ok {
		t.Error("data missing 'items' field")
	}
}
