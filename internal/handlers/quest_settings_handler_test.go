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

func setupQuestSettingsRouter(t *testing.T, db *gorm.DB) (*gin.Engine, uuid.UUID) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	questSettingsService := services.NewQuestSettingsService(db)
	questSettingsHandler := handlers.NewQuestSettingsHandler(questSettingsService)

	userID := testutils.BootstrapTestUser(t, db)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		qs := protected.Group("/api/quest-settings")
		{
			qs.GET("", questSettingsHandler.Get)
			qs.PUT("", questSettingsHandler.Update)
			qs.PATCH("", questSettingsHandler.Update)
			qs.POST("/reset", questSettingsHandler.Reset)
		}
	}

	return r, userID
}

func TestQuestSettings_GetCreatesDefaults(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/quest-settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(unwrapData(t, resp)["daily_quest_count"].(float64)) != 8 {
		t.Errorf("expected daily_quest_count 8, got %v", unwrapData(t, resp)["daily_quest_count"])
	}
	if unwrapData(t, resp)["difficulty"] != "normal" {
		t.Errorf("expected difficulty 'normal', got '%s'", unwrapData(t, resp)["difficulty"])
	}
	if unwrapData(t, resp)["auto_adjust_enabled"] != true {
		t.Error("expected auto_adjust_enabled true")
	}
	if unwrapData(t, resp)["preferred_duration"] != "medium" {
		t.Errorf("expected preferred_duration 'medium', got '%s'", unwrapData(t, resp)["preferred_duration"])
	}
	if unwrapData(t, resp)["rest_day_enabled"] != false {
		t.Error("expected rest_day_enabled false")
	}

	cats, _ := unwrapData(t, resp)["enabled_categories"].([]interface{})
	if len(cats) == 0 {
		t.Error("expected non-empty enabled_categories")
	}

	rules, _ := unwrapData(t, resp)["rules"].([]interface{})
	if len(rules) != 4 {
		t.Errorf("expected 4 default rules, got %d", len(rules))
	}
}

func TestQuestSettings_GetReturnsExisting(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	defaults := services.BuildDefaultQuestSettings(userID)
	defaults.DailyQuestCount = 5
	db.Create(defaults)

	req, _ := http.NewRequest("GET", "/api/quest-settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(unwrapData(t, resp)["daily_quest_count"].(float64)) != 5 {
		t.Errorf("expected daily_quest_count 5, got %v", unwrapData(t, resp)["daily_quest_count"])
	}
}

func TestQuestSettings_PutUpdatesSettings(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	rules := []map[string]interface{}{
		{
			"id":                  "rule_movement",
			"type":                "movement",
			"title":               "Vận động nhẹ",
			"description":         "Nhắc vận động",
			"enabled":             true,
			"difficulty":          "medium",
			"min_interval_minutes": 60,
			"max_per_day":         5,
			"active_time_range":   map[string]string{"start": "10:00", "end": "20:00"},
			"active_weekdays":     []int{1, 2, 3, 4, 5},
			"priority":            5,
			"adapt_to_energy":     true,
			"adapt_to_stress":     true,
			"adapt_to_schedule":   true,
		},
	}

	dailyQuestCount := 5
	body := map[string]interface{}{
		"daily_quest_count":  dailyQuestCount,
		"difficulty":         "easy",
		"auto_adjust_enabled": true,
		"enabled_categories": []string{"movement", "learning"},
		"preferred_duration": "short",
		"rest_day_enabled":   false,
		"rules":              rules,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(unwrapData(t, resp)["daily_quest_count"].(float64)) != 5 {
		t.Errorf("expected daily_quest_count 5, got %v", unwrapData(t, resp)["daily_quest_count"])
	}
	if unwrapData(t, resp)["difficulty"] != "easy" {
		t.Errorf("expected difficulty 'easy', got '%s'", unwrapData(t, resp)["difficulty"])
	}
	if unwrapData(t, resp)["preferred_duration"] != "short" {
		t.Errorf("expected preferred_duration 'short', got '%s'", unwrapData(t, resp)["preferred_duration"])
	}

	cats := unwrapData(t, resp)["enabled_categories"].([]interface{})
	if len(cats) != 2 {
		t.Errorf("expected 2 enabled categories, got %d", len(cats))
	}

	respRules := unwrapData(t, resp)["rules"].([]interface{})
	if len(respRules) != 4 {
		t.Errorf("expected 4 rules preserved, got %d", len(respRules))
	}

	for _, r := range respRules {
		rule := r.(map[string]interface{})
		if rule["id"] == "rule_movement" {
			if rule["enabled"] != true {
				t.Error("expected rule_movement enabled true")
			}
			if int(rule["max_per_day"].(float64)) != 5 {
				t.Errorf("expected rule_movement max_per_day 5, got %v", rule["max_per_day"])
			}
			if rule["title"] != "Vận động nhẹ" {
				t.Errorf("expected rule_movement title 'Vận động nhẹ', got '%s'", rule["title"])
			}
		}
	}
}

func TestQuestSettings_Reset(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	custom := services.BuildDefaultQuestSettings(userID)
	custom.DailyQuestCount = 3
	db.Create(custom)

	req, _ := http.NewRequest("POST", "/api/quest-settings/reset", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(unwrapData(t, resp)["daily_quest_count"].(float64)) != 8 {
		t.Errorf("expected reset daily_quest_count 8, got %v", unwrapData(t, resp)["daily_quest_count"])
	}
}

func TestQuestSettings_InvalidDailyQuestCount_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	tests := []int{0, 21}
	for _, v := range tests {
		body := map[string]interface{}{"daily_quest_count": v}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for daily_quest_count=%d, got %d", v, w.Code)
		}
	}
}

func TestQuestSettings_InvalidCategory_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	body := map[string]interface{}{
		"enabled_categories": []string{"health"},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid category, got %d", w.Code)
	}
}

func TestQuestSettings_InvalidRuleType_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	rules := []map[string]interface{}{
		{
			"id":             "rule_x",
			"type":           "invalidType",
			"title":          "Test",
			"description":    "Test",
			"enabled":        true,
			"difficulty":     "easy",
			"active_weekdays": []int{1, 2, 3, 4, 5, 6, 7},
			"priority":       3,
			"adapt_to_energy":    true,
			"adapt_to_stress":    true,
			"adapt_to_schedule":  true,
		},
	}
	body := map[string]interface{}{"rules": rules}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestQuestSettings_InvalidTimeRange_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	tests := []map[string]string{
		{"start": "25:00", "end": "26:00"},
		{"start": "22:00", "end": "08:00"},
	}
	for _, tr := range tests {
		rules := []map[string]interface{}{
			{
				"id":                "rule_movement",
				"type":              "movement",
				"title":             "Test",
				"description":       "Test",
				"enabled":           true,
				"difficulty":        "medium",
				"active_time_range": tr,
				"active_weekdays":   []int{1, 2, 3, 4, 5, 6, 7},
				"priority":          3,
				"adapt_to_energy":   true,
				"adapt_to_stress":   true,
				"adapt_to_schedule": true,
			},
		}
		body := map[string]interface{}{"rules": rules}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for time range start=%s end=%s, got %d", tr["start"], tr["end"], w.Code)
		}
	}
}

func TestQuestSettings_UserIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userA := testutils.BootstrapTestUser(t, db)
	userB := uuid.New()
	testutils.CreateTestUser(db, userB, "userb@test.com")

	settingsA := services.BuildDefaultQuestSettings(userA)
	settingsA.DailyQuestCount = 7
	db.Create(settingsA)

	settingsB := services.BuildDefaultQuestSettings(userB)
	settingsB.DailyQuestCount = 3
	db.Create(settingsB)

	gin.SetMode(gin.TestMode)
	questSettingsService := services.NewQuestSettingsService(db)
	handler := handlers.NewQuestSettingsHandler(questSettingsService)

	rA := gin.New()
	rA.GET("/api/quest-settings", testutils.AuthMiddleware(userA), handler.Get)

	reqA, _ := http.NewRequest("GET", "/api/quest-settings", nil)
	wA := httptest.NewRecorder()
	rA.ServeHTTP(wA, reqA)

	var respA map[string]interface{}
	json.Unmarshal(wA.Body.Bytes(), &respA)
	if int(unwrapData(t, respA)["daily_quest_count"].(float64)) != 7 {
		t.Errorf("user A should see 7, got %v", unwrapData(t, respA)["daily_quest_count"])
	}

	rB := gin.New()
	rB.GET("/api/quest-settings", testutils.AuthMiddleware(userB), handler.Get)

	reqB, _ := http.NewRequest("GET", "/api/quest-settings", nil)
	wB := httptest.NewRecorder()
	rB.ServeHTTP(wB, reqB)

	var respB map[string]interface{}
	json.Unmarshal(wB.Body.Bytes(), &respB)
	if int(unwrapData(t, respB)["daily_quest_count"].(float64)) != 3 {
		t.Errorf("user B should see 3, got %v", unwrapData(t, respB)["daily_quest_count"])
	}
}

func TestQuestSettings_ResponseShapeMatchesFE(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/quest-settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	requiredFields := []string{
		"daily_quest_count", "difficulty", "auto_adjust_enabled",
		"enabled_categories", "preferred_duration", "rest_day_enabled", "rules",
	}
	data := unwrapData(t, resp)
	for _, f := range requiredFields {
		if _, ok := data[f]; !ok {
			t.Errorf("response missing field '%s'", f)
		}
	}

	rules, ok := unwrapData(t, resp)["rules"].([]interface{})
	if !ok || len(rules) == 0 {
		t.Fatal("rules should be non-empty array")
	}

	firstRule := rules[0].(map[string]interface{})
	ruleFields := []string{
		"id", "type", "title", "description", "enabled", "difficulty",
		"min_interval_minutes", "max_per_day", "active_time_range",
		"active_weekdays", "priority", "adapt_to_energy", "adapt_to_stress", "adapt_to_schedule",
	}
	for _, f := range ruleFields {
		if _, ok := firstRule[f]; !ok {
			t.Errorf("rule missing field '%s'", f)
		}
	}
}

func TestQuestSettings_SingleRuleUpdatePreservesAllRules(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	defaults := services.BuildDefaultQuestSettings(userID)
	db.Create(defaults)

	rules := []map[string]interface{}{
		{
			"id":                  "rule_movement",
			"type":                "movement",
			"title":               "Vận động nhẹ đã cập nhật",
			"description":         "Mô tả mới",
			"enabled":             true,
			"difficulty":          "medium",
			"min_interval_minutes": 60,
			"max_per_day":         5,
			"active_time_range":   map[string]string{"start": "10:00", "end": "20:00"},
			"active_weekdays":     []int{1, 2, 3, 4, 5, 6, 7},
			"priority":            5,
			"adapt_to_energy":     true,
			"adapt_to_stress":     true,
			"adapt_to_schedule":   true,
		},
	}
	body := map[string]interface{}{"rules": rules}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	respRules := unwrapData(t, resp)["rules"].([]interface{})
	if len(respRules) != 4 {
		t.Errorf("expected 4 rules preserved, got %d", len(respRules))
	}

	for _, r := range respRules {
		rule := r.(map[string]interface{})
		id := rule["id"].(string)
		switch id {
		case "rule_movement":
			if rule["title"] != "Vận động nhẹ đã cập nhật" {
				t.Errorf("expected rule_movement title updated, got '%s'", rule["title"])
			}
			if int(rule["max_per_day"].(float64)) != 5 {
				t.Errorf("expected rule_movement max_per_day 5, got %v", rule["max_per_day"])
			}
		case "rule_learning":
			if rule["title"] != "Học tập" {
				t.Errorf("rule_learning should be unchanged, got '%s'", rule["title"])
			}
		case "rule_sleep":
			if rule["title"] != "Giấc ngủ" {
				t.Errorf("rule_sleep should be unchanged, got '%s'", rule["title"])
			}
		}
	}
}

func TestQuestSettings_ToggleOneRule_PreservesOthers(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	defaults := services.BuildDefaultQuestSettings(userID)
	db.Create(defaults)

	rules := []map[string]interface{}{
		{
			"id":                  "rule_movement",
			"type":                "movement",
			"title":               "Vận động nhẹ",
			"description":         "Gợi ý vận động nhẹ trong ngày",
			"enabled":             false,
			"difficulty":          "medium",
			"active_weekdays":     []int{1, 2, 3, 4, 5, 6, 7},
			"priority":            4,
			"adapt_to_energy":     true,
			"adapt_to_stress":     true,
			"adapt_to_schedule":   true,
		},
	}
	body := map[string]interface{}{"rules": rules}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	respRules := unwrapData(t, resp)["rules"].([]interface{})
	if len(respRules) != 4 {
		t.Errorf("expected 4 rules preserved, got %d", len(respRules))
	}

	movementCount := 0
	for _, r := range respRules {
		rule := r.(map[string]interface{})
		if rule["id"] == "rule_movement" {
			movementCount++
			if rule["enabled"] != false {
				t.Error("expected rule_movement enabled false")
			}
		}
	}
	if movementCount != 1 {
		t.Errorf("expected exactly 1 rule_movement, got %d", movementCount)
	}
}

func TestQuestSettings_GlobalOnlyUpdatePreservesRules(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	defaults := services.BuildDefaultQuestSettings(userID)
	db.Create(defaults)

	body := map[string]interface{}{
		"daily_quest_count":  5,
		"difficulty":         "hard",
		"auto_adjust_enabled": false,
		"preferred_duration": "long",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(unwrapData(t, resp)["daily_quest_count"].(float64)) != 5 {
		t.Error("daily_quest_count should be 5")
	}
	if unwrapData(t, resp)["difficulty"] != "hard" {
		t.Error("difficulty should be hard")
	}
	if unwrapData(t, resp)["auto_adjust_enabled"] != false {
		t.Error("auto_adjust_enabled should be false")
	}

	respRules := unwrapData(t, resp)["rules"].([]interface{})
	if len(respRules) != 4 {
		t.Errorf("rules should be preserved, expected 4, got %d", len(respRules))
	}
}

func TestQuestSettings_EmptyRulesArrayPreservesRules(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	defaults := services.BuildDefaultQuestSettings(userID)
	db.Create(defaults)

	body := map[string]interface{}{"rules": []interface{}{}}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	respRules := unwrapData(t, resp)["rules"].([]interface{})
	if len(respRules) != 4 {
		t.Errorf("empty rules should preserve existing, expected 4, got %d", len(respRules))
	}
}

func TestQuestSettings_UnknownRuleID_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	defaults := services.BuildDefaultQuestSettings(userID)
	db.Create(defaults)

	rules := []map[string]interface{}{
		{
			"id":             "rule_unknown",
			"type":           "learning",
			"title":          "Unknown",
			"description":    "Unknown",
			"enabled":        true,
			"difficulty":     "medium",
			"active_weekdays": []int{1, 2, 3, 4, 5, 6, 7},
			"priority":       3,
			"adapt_to_energy":    true,
			"adapt_to_stress":    true,
			"adapt_to_schedule":  true,
		},
	}
	body := map[string]interface{}{"rules": rules}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for unknown rule id, got %d: %s", w.Code, w.Body.String())
	}

	getReq, _ := http.NewRequest("GET", "/api/quest-settings", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	var getResp map[string]interface{}
	json.Unmarshal(getW.Body.Bytes(), &getResp)
	getRules := unwrapData(t, getResp)["rules"].([]interface{})
	if len(getRules) != 4 {
		t.Errorf("unknown rule id must not delete existing rules, expected 4, got %d", len(getRules))
	}
}

func TestQuestSettings_MissingRuleID_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	rules := []map[string]interface{}{
		{
			"type":           "learning",
			"title":          "No ID",
			"description":    "No ID",
			"enabled":        true,
			"difficulty":     "medium",
			"active_weekdays": []int{1, 2, 3, 4, 5, 6, 7},
			"priority":       3,
			"adapt_to_energy":    true,
			"adapt_to_stress":    true,
			"adapt_to_schedule":  true,
		},
	}
	body := map[string]interface{}{"rules": rules}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for missing rule id, got %d: %s", w.Code, w.Body.String())
	}
}

func TestQuestSettings_Unauthorized_Returns401(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	questSettingsService := services.NewQuestSettingsService(db)
	handler := handlers.NewQuestSettingsHandler(questSettingsService)
	r.GET("/api/quest-settings", handler.Get)

	req, _ := http.NewRequest("GET", "/api/quest-settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}
}

func TestQuestSettings_PutInvalidBody_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestQuestSettings_PatchUpdatesSettings(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	rules := []map[string]interface{}{
		{
			"id":                  "rule_movement",
			"type":                "movement",
			"title":               "Vận động nhẹ",
			"description":         "Nhắc vận động",
			"enabled":             true,
			"difficulty":          "medium",
			"min_interval_minutes": 60,
			"max_per_day":         5,
			"active_time_range":   map[string]string{"start": "10:00", "end": "20:00"},
			"active_weekdays":     []int{1, 2, 3, 4, 5},
			"priority":            5,
			"adapt_to_energy":     true,
			"adapt_to_stress":     true,
			"adapt_to_schedule":   true,
		},
	}

	dailyQuestCount := 5
	body := map[string]interface{}{
		"daily_quest_count":  dailyQuestCount,
		"difficulty":         "easy",
		"auto_adjust_enabled": true,
		"enabled_categories": []string{"movement", "learning"},
		"preferred_duration": "short",
		"rest_day_enabled":   false,
		"rules":              rules,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(unwrapData(t, resp)["daily_quest_count"].(float64)) != 5 {
		t.Errorf("expected daily_quest_count 5, got %v", unwrapData(t, resp)["daily_quest_count"])
	}
	if unwrapData(t, resp)["difficulty"] != "easy" {
		t.Errorf("expected difficulty 'easy', got '%s'", unwrapData(t, resp)["difficulty"])
	}
	if unwrapData(t, resp)["preferred_duration"] != "short" {
		t.Errorf("expected preferred_duration 'short', got '%s'", unwrapData(t, resp)["preferred_duration"])
	}

	cats := unwrapData(t, resp)["enabled_categories"].([]interface{})
	if len(cats) != 2 {
		t.Errorf("expected 2 enabled categories, got %d", len(cats))
	}

	respRules := unwrapData(t, resp)["rules"].([]interface{})
	if len(respRules) != 4 {
		t.Errorf("expected 4 rules preserved, got %d", len(respRules))
	}
}

func TestQuestSettings_PatchResponseEnvelope(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	body := map[string]interface{}{
		"difficulty": "easy",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	code, ok := resp["code"].(float64)
	if !ok || int(code) != 200 {
		t.Errorf("expected code 200, got %v", resp["code"])
	}
	if msg, ok := resp["message"].(string); !ok || msg != "Success" {
		t.Errorf("expected message 'Success', got '%v'", resp["message"])
	}
	if _, ok := resp["data"]; !ok {
		t.Error("response missing 'data' field")
	}
}

func TestQuestSettings_PatchInvalidBody_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	req, _ := http.NewRequest("PATCH", "/api/quest-settings", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected code 400, got %v", resp["code"])
	}
}

func TestQuestSettings_PutStillWorks(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestSettingsRouter(t, db)

	body := map[string]interface{}{
		"difficulty": "hard",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for PUT, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if unwrapData(t, resp)["difficulty"] != "hard" {
		t.Errorf("expected difficulty 'hard' via PUT, got '%v'", unwrapData(t, resp)["difficulty"])
	}
}

func TestQuestSettings_PartialUpdate_OnlyPriority(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	defaults := services.BuildDefaultQuestSettings(userID)
	db.Create(defaults)

	body := map[string]interface{}{
		"rules": []map[string]interface{}{
			{
				"id":       "rule_movement",
				"priority": 1,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	respRules := unwrapData(t, resp)["rules"].([]interface{})
	if len(respRules) != 4 {
		t.Errorf("expected 4 rules, got %d", len(respRules))
	}

	for _, r := range respRules {
		rule := r.(map[string]interface{})
		if rule["id"] == "rule_movement" {
			if int(rule["priority"].(float64)) != 1 {
				t.Errorf("expected priority 1, got %v", rule["priority"])
			}
			if rule["enabled"] != true {
				t.Errorf("expected enabled to remain true, got %v", rule["enabled"])
			}
			if rule["difficulty"] != "medium" {
				t.Errorf("expected difficulty to remain 'medium', got %v", rule["difficulty"])
			}
			if rule["title"] != "Vận động nhẹ" {
				t.Errorf("expected title to remain 'Vận động nhẹ', got %v", rule["title"])
			}
			if rule["description"] != "Gợi ý vận động nhẹ trong ngày" {
				t.Errorf("expected description to remain unchanged, got %v", rule["description"])
			}
		}
	}
}

func TestQuestSettings_PartialUpdate_OnlyEnabledFalse(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	defaults := services.BuildDefaultQuestSettings(userID)
	db.Create(defaults)

	body := map[string]interface{}{
		"rules": []map[string]interface{}{
			{
				"id":      "rule_movement",
				"enabled": false,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	respRules := unwrapData(t, resp)["rules"].([]interface{})
	for _, r := range respRules {
		rule := r.(map[string]interface{})
		if rule["id"] == "rule_movement" {
			if rule["enabled"] != false {
				t.Errorf("expected enabled to be false, got %v", rule["enabled"])
			}
			if int(rule["priority"].(float64)) != 4 {
				t.Errorf("expected priority to remain 4, got %v", rule["priority"])
			}
		}
	}
}

func TestQuestSettings_PartialUpdate_OnlyDifficulty(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestSettingsRouter(t, db)

	defaults := services.BuildDefaultQuestSettings(userID)
	db.Create(defaults)

	body := map[string]interface{}{
		"rules": []map[string]interface{}{
			{
				"id":         "rule_movement",
				"difficulty": "hard",
			},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/quest-settings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	respRules := unwrapData(t, resp)["rules"].([]interface{})
	for _, r := range respRules {
		rule := r.(map[string]interface{})
		if rule["id"] == "rule_movement" {
			if rule["difficulty"] != "hard" {
				t.Errorf("expected difficulty to be 'hard', got %v", rule["difficulty"])
			}
			if rule["enabled"] != true {
				t.Errorf("expected enabled to remain true, got %v", rule["enabled"])
			}
		}
	}
}
