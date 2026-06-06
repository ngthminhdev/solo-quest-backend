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
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupLearningRoadmapRouter(t *testing.T) (*gin.Engine, uuid.UUID, *gorm.DB) {
	t.Helper()

	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })

	userID := testutils.BootstrapTestUser(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	svc := services.NewLearningRoadmapService(db)
	handler := handlers.NewLearningRoadmapHandler(svc)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		lr := protected.Group("/api/learning-roadmaps")
		{
			lr.POST("/ai-suggest", handler.AiSuggest)
			lr.POST("", handler.Create)
			lr.GET("", handler.List)
			lr.GET("/:id", handler.GetDetail)
			lr.POST("/:id/follow", handler.Follow)
			lr.PATCH("/:id/steps/:step_id", handler.ToggleStep)
		}
	}

	return r, userID, db
}

func seedRoadmapData(t *testing.T, db *gorm.DB) (*models.LearningRoadmap, []*models.LearningRoadmapStep) {
	t.Helper()

	roadmap := &models.LearningRoadmap{
		Title: "Test Roadmap", Description: "A test roadmap", Category: "flutter",
		Difficulty: "normal", EstimatedMinutes: 60, TotalSteps: 2, Source: "system", Enabled: true,
	}
	if err := db.Create(roadmap).Error; err != nil {
		t.Fatal("failed to create roadmap:", err)
	}

	step1 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", Description: "First step", OrderIndex: 1, EstimatedMinutes: 30, Enabled: true}
	step2 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 2", Description: "Second step", OrderIndex: 2, EstimatedMinutes: 30, Enabled: true}
	db.Create(step1)
	db.Create(step2)

	return roadmap, []*models.LearningRoadmapStep{step1, step2}
}

func TestLearningRoadmapHandler_ListReturns200(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)
	seedRoadmapData(t, db)

	req, _ := http.NewRequest("GET", "/api/learning-roadmaps", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 200 {
		t.Errorf("expected code 200, got %v", resp["code"])
	}
	if _, ok := resp["message"]; !ok {
		t.Error("response missing 'message' field")
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}
	items, ok := data["items"].([]interface{})
	if !ok {
		t.Fatal("data missing 'items' array")
	}
	if len(items) < 1 {
		t.Error("expected at least 1 roadmap in items")
	}
}

func TestLearningRoadmapHandler_GetDetailReturnsSteps(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)
	roadmap, _ := seedRoadmapData(t, db)

	req, _ := http.NewRequest("GET", "/api/learning-roadmaps/"+roadmap.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object")
	}
	steps, ok := data["steps"].([]interface{})
	if !ok {
		t.Fatal("expected steps array")
	}
	if len(steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(steps))
	}
}

func TestLearningRoadmapHandler_GetDetailInvalidID(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	req, _ := http.NewRequest("GET", "/api/learning-roadmaps/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected code 400, got %v", resp["code"])
	}
}

func TestLearningRoadmapHandler_GetDetailNotFound(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	req, _ := http.NewRequest("GET", "/api/learning-roadmaps/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLearningRoadmapHandler_FollowReturns200(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)
	roadmap, _ := seedRoadmapData(t, db)

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/"+roadmap.ID.String()+"/follow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 200 {
		t.Errorf("expected code 200, got %v", resp["code"])
	}
}

func TestLearningRoadmapHandler_FollowNotFound(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/"+uuid.New().String()+"/follow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLearningRoadmapHandler_ToggleStepBeforeFollow(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)
	_, steps := seedRoadmapData(t, db)

	body, _ := json.Marshal(map[string]bool{"completed": true})
	req, _ := http.NewRequest("PATCH", "/api/learning-roadmaps/"+steps[0].RoadmapID.String()+"/steps/"+steps[0].ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for toggle before follow, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLearningRoadmapHandler_ToggleStepAfterFollow(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)
	roadmap, steps := seedRoadmapData(t, db)

	// Follow first
	reqFollow, _ := http.NewRequest("POST", "/api/learning-roadmaps/"+roadmap.ID.String()+"/follow", nil)
	wFollow := httptest.NewRecorder()
	r.ServeHTTP(wFollow, reqFollow)
	if wFollow.Code != http.StatusOK {
		t.Fatalf("follow failed: %d", wFollow.Code)
	}

	// Toggle step
	body, _ := json.Marshal(map[string]bool{"completed": true})
	req, _ := http.NewRequest("PATCH", "/api/learning-roadmaps/"+roadmap.ID.String()+"/steps/"+steps[0].ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object")
	}
	if data["completed"] != true {
		t.Error("expected completed=true")
	}
	if data["completed_steps"].(float64) != 1 {
		t.Errorf("expected completed_steps=1, got %v", data["completed_steps"])
	}
	if data["total_steps"].(float64) != 2 {
		t.Errorf("expected total_steps=2, got %v", data["total_steps"])
	}
	if data["progress_percent"].(float64) != 50 {
		t.Errorf("expected progress_percent=50, got %v", data["progress_percent"])
	}
	if data["roadmap_status"] != "tracking" {
		t.Errorf("expected roadmap_status=tracking, got %v", data["roadmap_status"])
	}
}

func TestLearningRoadmapHandler_ToggleStepMissingStep(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)
	roadmap, _ := seedRoadmapData(t, db)

	// Follow first
	reqFollow, _ := http.NewRequest("POST", "/api/learning-roadmaps/"+roadmap.ID.String()+"/follow", nil)
	r.ServeHTTP(httptest.NewRecorder(), reqFollow)

	body, _ := json.Marshal(map[string]bool{"completed": true})
	req, _ := http.NewRequest("PATCH", "/api/learning-roadmaps/"+roadmap.ID.String()+"/steps/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLearningRoadmapHandler_ToggleStepInvalidJSON(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	body := bytes.NewReader([]byte(`{invalid`))
	req, _ := http.NewRequest("PATCH", "/api/learning-roadmaps/"+uuid.New().String()+"/steps/"+uuid.New().String(), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLearningRoadmapHandler_ToggleStepInvalidIDs(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	body, _ := json.Marshal(map[string]bool{"completed": true})
	req, _ := http.NewRequest("PATCH", "/api/learning-roadmaps/not-uuid/steps/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid roadmap id, got %d", w.Code)
	}

	req2, _ := http.NewRequest("PATCH", "/api/learning-roadmaps/"+uuid.New().String()+"/steps/not-uuid", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid step id, got %d", w2.Code)
	}
}

func TestLearningRoadmapHandler_AiSuggest_Returns200(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"learning_goal": "Flutter",
		},
		"limit": 3,
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/ai-suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["code"].(float64) != 200 {
		t.Errorf("expected code 200, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	suggestions := data["suggestions"].([]interface{})
	if len(suggestions) == 0 {
		t.Error("expected at least one suggestion")
	}
}

func TestLearningRoadmapHandler_AiSuggest_InvalidBody(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/ai-suggest", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestLearningRoadmapHandler_Create_FromSuggestion_Returns201(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	suggestionID := "ai_dart_async"
	body, _ := json.Marshal(map[string]interface{}{
		"suggestion_id": suggestionID,
		"source":        "ai",
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// The response envelope uses nested code field which is 201
	if resp["code"].(float64) != 201 {
		t.Errorf("expected code 201, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if data["source"].(string) != "ai" {
		t.Errorf("expected source 'ai', got %v", data["source"])
	}
	if data["status"].(string) != "tracking" {
		t.Errorf("expected status 'tracking', got %v", data["status"])
	}
}

func TestLearningRoadmapHandler_Create_FromSuggestion_InvalidID(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"suggestion_id": "invalid_suggestion",
		"source":        "ai",
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for invalid suggestion, got %d", w.Code)
	}
}

func TestLearningRoadmapHandler_Create_Custom_Returns201(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"title":      "My Custom Roadmap",
		"category":   "Custom",
		"difficulty": "intermediate",
		"source":     "user",
		"steps": []map[string]interface{}{
			{"title": "Step 1", "description": "First", "order_index": 0, "estimated_minutes": 30},
			{"title": "Step 2", "description": "Second", "order_index": 1, "estimated_minutes": 45},
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data := resp["data"].(map[string]interface{})
	if data["source"].(string) != "user" {
		t.Errorf("expected source 'user', got %v", data["source"])
	}
	steps := data["steps"].([]interface{})
	if len(steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(steps))
	}
}

func TestLearningRoadmapHandler_Create_InvalidSource(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"suggestion_id": "ai_dart_async",
		"source":        "invalid",
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid source, got %d", w.Code)
	}
}

func TestLearningRoadmapHandler_Create_Custom_MissingTitle(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"category":   "Custom",
		"difficulty": "intermediate",
		"source":     "user",
		"steps": []map[string]interface{}{
			{"title": "Step 1", "order_index": 0, "estimated_minutes": 30},
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing title, got %d", w.Code)
	}
}

func TestLearningRoadmapHandler_Create_InvalidBody(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// Template-based handler tests

func setupTemplateRouter(t *testing.T) (*gin.Engine, uuid.UUID, *gorm.DB) {
	t.Helper()

	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })

	userID := testutils.BootstrapTestUser(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	svc := services.NewLearningRoadmapService(db)
	handler := handlers.NewLearningRoadmapHandler(svc)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		lr := protected.Group("/api/learning-roadmaps")
		{
			lr.POST("/suggest", handler.Suggest)
			lr.POST("", handler.CreateFromTemplate)
			lr.GET("", handler.List)
			lr.GET("/:id", handler.GetDetail)
			lr.POST("/:id/follow", handler.Follow)
			lr.PATCH("/:id/steps/:step_id", handler.ToggleStep)
		}
	}

	return r, userID, db
}

func TestLearningRoadmapHandler_Suggest_Success(t *testing.T) {
	r, _, _ := setupTemplateRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"category": "Flutter",
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	suggestions := data["suggestions"].([]interface{})

	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// Verify first suggestion has required fields
	first := suggestions[0].(map[string]interface{})
	if first["id"] == nil || first["title"] == nil || first["source"] == nil {
		t.Error("suggestion missing required fields")
	}
	if first["source"] != "template" {
		t.Errorf("expected source='template', got '%v'", first["source"])
	}
}

func TestLearningRoadmapHandler_Suggest_FiltersByCategory(t *testing.T) {
	r, _, _ := setupTemplateRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"category": "Dart",
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	suggestions := data["suggestions"].([]interface{})

	if len(suggestions) == 0 {
		t.Fatal("expected Dart suggestions")
	}

	for _, sugg := range suggestions {
		s := sugg.(map[string]interface{})
		if s["category"] != "Dart" {
			t.Errorf("expected category='Dart', got '%v'", s["category"])
		}
	}
}

func TestLearningRoadmapHandler_Suggest_InvalidBody(t *testing.T) {
	r, _, _ := setupTemplateRouter(t)

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/suggest", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestLearningRoadmapHandler_CreateFromTemplate_Success(t *testing.T) {
	r, _, _ := setupTemplateRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"template_id": "template_dart_async",
		"source":      "template",
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	roadmap := data["roadmap"].(map[string]interface{})

	if roadmap["source"] != "template" {
		t.Errorf("expected source='template', got '%v'", roadmap["source"])
	}
	if roadmap["title"] != "Dart Async Programming" {
		t.Errorf("unexpected title: %v", roadmap["title"])
	}
	if roadmap["status"] != "tracking" {
		t.Errorf("expected status='tracking', got '%v'", roadmap["status"])
	}

	steps := roadmap["steps"].([]interface{})
	if len(steps) == 0 {
		t.Error("expected steps to be included")
	}
}

func TestLearningRoadmapHandler_CreateFromTemplate_InvalidTemplateID(t *testing.T) {
	r, _, _ := setupTemplateRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"template_id": "invalid_template",
		"source":      "template",
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for invalid template, got %d", w.Code)
	}
}

func TestLearningRoadmapHandler_CreateFromTemplate_InvalidSource(t *testing.T) {
	r, _, _ := setupTemplateRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"template_id": "template_dart_async",
		"source":      "invalid",
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid source, got %d", w.Code)
	}
}

func TestLearningRoadmapHandler_CreateFromTemplate_InvalidBody(t *testing.T) {
	r, _, _ := setupTemplateRouter(t)

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}
