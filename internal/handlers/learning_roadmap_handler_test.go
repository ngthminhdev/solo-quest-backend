package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/services/ai"
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
	svc.SetRoadmapGenerationWorkerLauncher(func(uuid.UUID) {})
	handler := handlers.NewLearningRoadmapHandler(svc)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		lr := protected.Group("/api/learning-roadmaps")
		{
			lr.POST("/ai-suggest", handler.AiSuggest)
			lr.POST("/suggest", handler.Suggest)
			lr.POST("/generate", handler.Generate)
			lr.GET("/generate/status", handler.GetGenerateStatus)
			lr.POST("", handler.Create)
			lr.GET("", handler.List)
			lr.GET("/:id", handler.GetDetail)
			lr.DELETE("/:id", handler.Delete)
			lr.POST("/:id/follow", handler.Follow)
			lr.PATCH("/:id/steps/:step_id", handler.ToggleStep)
		}
	}

	return r, userID, db
}

func handlerValidGenerateRoadmapRequest() dto.GenerateLearningRoadmapRequest {
	return dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter cơ bản",
			Category:     "Flutter",
			Difficulty:   "beginner",
			MaxDuration:  300,
		},
	}
}

func seedRoadmapData(t *testing.T, db *gorm.DB, userID uuid.UUID) (*models.LearningRoadmap, []*models.LearningRoadmapStep) {
	t.Helper()

	roadmap := &models.LearningRoadmap{
		Title: "Test Roadmap", Description: "A test roadmap", Category: "flutter",
		Difficulty: "normal", EstimatedMinutes: 60, TotalSteps: 2, Source: "system", CreatedByUserID: &userID, Enabled: true,
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
	r, userID, db := setupLearningRoadmapRouter(t)
	seedRoadmapData(t, db, userID)

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
	r, userID, db := setupLearningRoadmapRouter(t)
	roadmap, _ := seedRoadmapData(t, db, userID)

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

func TestLearningRoadmapHandler_DeleteUserRoadmapReturns200(t *testing.T) {
	r, userID, db := setupLearningRoadmapRouter(t)
	roadmap, steps := seedRoadmapData(t, db, userID)
	if err := db.Model(roadmap).Updates(map[string]interface{}{
		"source":             models.LearningRoadmapSourceAI,
		"created_by_user_id": userID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("DELETE", "/api/learning-roadmaps/"+roadmap.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if int(resp["code"].(float64)) != http.StatusOK {
		t.Fatalf("expected envelope code 200, got %v", resp["code"])
	}

	var reloaded models.LearningRoadmap
	if err := db.First(&reloaded, "id = ?", roadmap.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.Enabled {
		t.Fatal("expected roadmap to be disabled")
	}

	var enabledStepCount int64
	db.Model(&models.LearningRoadmapStep{}).
		Where("roadmap_id = ? AND enabled = ?", roadmap.ID, true).
		Count(&enabledStepCount)
	if enabledStepCount != 0 {
		t.Fatalf("expected all steps disabled, got %d enabled", enabledStepCount)
	}

	if len(steps) == 0 {
		t.Fatal("test setup expected steps")
	}
}

func TestLearningRoadmapHandler_DeleteDefaultRoadmapReturns404(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)
	roadmap := &models.LearningRoadmap{
		Title: "Default Roadmap", Description: "A default roadmap", Category: "flutter",
		Difficulty: "normal", EstimatedMinutes: 60, TotalSteps: 1, Source: "system", Enabled: true,
	}
	if err := db.Create(roadmap).Error; err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("DELETE", "/api/learning-roadmaps/"+roadmap.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var reloaded models.LearningRoadmap
	if err := db.First(&reloaded, "id = ?", roadmap.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reloaded.Enabled {
		t.Fatal("default roadmap should not be disabled")
	}
}

func TestLearningRoadmapHandler_DeleteOtherUsersRoadmapReturns404(t *testing.T) {
	r, userID, db := setupLearningRoadmapRouter(t)
	otherUserID := uuid.New()
	testutils.CreateTestUser(db, otherUserID, "other@example.com")
	roadmap, _ := seedRoadmapData(t, db, userID)
	if err := db.Model(roadmap).Updates(map[string]interface{}{
		"source":             models.LearningRoadmapSourceAI,
		"created_by_user_id": otherUserID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("DELETE", "/api/learning-roadmaps/"+roadmap.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var reloaded models.LearningRoadmap
	if err := db.First(&reloaded, "id = ?", roadmap.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reloaded.Enabled {
		t.Fatal("other user's roadmap should not be disabled")
	}
}

func TestLearningRoadmapHandler_FollowReturns200(t *testing.T) {
	r, userID, db := setupLearningRoadmapRouter(t)
	roadmap, _ := seedRoadmapData(t, db, userID)

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
	r, userID, db := setupLearningRoadmapRouter(t)
	_, steps := seedRoadmapData(t, db, userID)

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
	r, userID, db := setupLearningRoadmapRouter(t)
	roadmap, steps := seedRoadmapData(t, db, userID)

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
	r, userID, db := setupLearningRoadmapRouter(t)
	roadmap, _ := seedRoadmapData(t, db, userID)

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

func TestLearningRoadmapHandler_GenerateReturns202AndJob(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"learning_goal": "Học Flutter cơ bản",
			"category":      "Flutter",
			"difficulty":    "beginner",
			"max_duration":  300,
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if int(resp["code"].(float64)) != http.StatusAccepted {
		t.Fatalf("expected envelope code 202, got %v", resp["code"])
	}
	data := resp["data"].(map[string]interface{})
	if data["job_id"] == "" {
		t.Fatal("expected job_id")
	}
	if data["status"] != "generating" {
		t.Fatalf("expected generating, got %v", data["status"])
	}

	var roadmapCount int64
	db.Model(&models.LearningRoadmap{}).Where("source = ?", models.LearningRoadmapSourceAI).Count(&roadmapCount)
	if roadmapCount != 0 {
		t.Fatalf("POST /generate should not synchronously create roadmap, got %d", roadmapCount)
	}
}

func TestLearningRoadmapHandler_GenerateInvalidRequestReturns400AndNoJob(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"learning_goal": "",
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var jobCount int64
	db.Model(&models.LearningRoadmapGenerationJob{}).Count(&jobCount)
	if jobCount != 0 {
		t.Fatalf("expected no job for invalid request, got %d", jobCount)
	}
}

func TestLearningRoadmapHandler_GetGenerateStatusGenerating(t *testing.T) {
	r, userID, db := setupLearningRoadmapRouter(t)
	hash, prefs, _ := services.BuildLearningRoadmapRequestHash(handlerValidGenerateRoadmapRequest())
	now := time.Now()
	job := models.LearningRoadmapGenerationJob{
		UserID:      userID,
		RequestHash: hash,
		Preferences: prefs,
		Status:      models.LearningRoadmapGenJobStatusGenerating,
		StartedAt:   &now,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", "/api/learning-roadmaps/generate/status?job_id="+job.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	data := resp["data"].(map[string]interface{})
	if data["status"] != "generating" {
		t.Fatalf("expected generating, got %v", data["status"])
	}
}

func TestLearningRoadmapHandler_GetGenerateStatusNotOwnedReturns404(t *testing.T) {
	r, _, db := setupLearningRoadmapRouter(t)
	otherUser := uuid.New()
	testutils.CreateTestUser(db, otherUser, "other@example.com")
	hash, prefs, _ := services.BuildLearningRoadmapRequestHash(handlerValidGenerateRoadmapRequest())
	job := models.LearningRoadmapGenerationJob{
		UserID:      otherUser,
		RequestHash: hash,
		Preferences: prefs,
		Status:      models.LearningRoadmapGenJobStatusGenerating,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", "/api/learning-roadmaps/generate/status?job_id="+job.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLearningRoadmapHandler_GenerateStatusRouteDoesNotHitDetail(t *testing.T) {
	r, _, _ := setupLearningRoadmapRouter(t)

	req, _ := http.NewRequest("GET", "/api/learning-roadmaps/generate/status?job_id=not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status handler 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["message"] != "invalid job_id" {
		t.Fatalf("expected status handler invalid job_id, got %v", resp["message"])
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

// Generate endpoint tests

func validGenerateAIResponse() string {
	resp := map[string]interface{}{
		"title":       "Lộ trình học Flutter",
		"description": "Lộ trình Flutter cơ bản",
		"category":    "Flutter",
		"difficulty":  "beginner",
		"steps": []map[string]interface{}{
			{"title": "Cài đặt Flutter SDK", "description": "Hướng dẫn cài đặt", "order_index": 1, "estimated_minutes": 30, "outcome": "ok"},
			{"title": "Tạo project đầu tiên", "description": "flutter create", "order_index": 2, "estimated_minutes": 30, "outcome": "ok"},
			{"title": "Widget cơ bản", "description": "Text, Container", "order_index": 3, "estimated_minutes": 30, "outcome": "ok"},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func setupGenerateRouter(t *testing.T, mockResponse string, mockErr error) (*gin.Engine, uuid.UUID, *gorm.DB) {
	t.Helper()

	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })

	userID := testutils.BootstrapTestUser(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	svc := services.NewLearningRoadmapService(db)
	mock := ai.NewMockClient(mockResponse, mockErr)
	svc.SetAIClient(mock)
	svc.SetRoadmapGenerationWorkerLauncher(func(uuid.UUID) {})
	handler := handlers.NewLearningRoadmapHandler(svc)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		lr := protected.Group("/api/learning-roadmaps")
		{
			lr.POST("/generate", handler.Generate)
			lr.GET("/generate/status", handler.GetGenerateStatus)
			lr.GET("/:id", handler.GetDetail)
		}
	}

	return r, userID, db
}

func TestLearningRoadmapHandler_Generate_Success(t *testing.T) {
	r, _, _ := setupGenerateRouter(t, validGenerateAIResponse(), nil)

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"learning_goal": "Học Flutter cơ bản",
			"category":      "Flutter",
			"difficulty":    "beginner",
			"max_duration":  300,
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["code"].(float64) != http.StatusAccepted {
		t.Errorf("expected code 202, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if data["source"] != "ai" {
		t.Errorf("expected source='ai', got '%v'", data["source"])
	}
	if data["status"] != "generating" {
		t.Errorf("expected status='generating', got '%v'", data["status"])
	}
	if data["job_id"] == "" {
		t.Fatal("expected job_id in async response")
	}
}

func TestLearningRoadmapHandler_Generate_InvalidBody(t *testing.T) {
	r, _, _ := setupGenerateRouter(t, validGenerateAIResponse(), nil)

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/generate", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLearningRoadmapHandler_Generate_MissingGoal(t *testing.T) {
	r, _, _ := setupGenerateRouter(t, validGenerateAIResponse(), nil)

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"learning_goal": "",
			"max_duration":  300,
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty goal, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLearningRoadmapHandler_Generate_AIDisabled(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)
	userID := testutils.BootstrapTestUser(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	svc := services.NewLearningRoadmapService(db)
	svc.SetRoadmapGenerationWorkerLauncher(func(uuid.UUID) {})
	handler := handlers.NewLearningRoadmapHandler(svc)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		lr := protected.Group("/api/learning-roadmaps")
		{
			lr.POST("/generate", handler.Generate)
		}
	}

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"learning_goal": "Học Flutter",
			"max_duration":  300,
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for async AI-disabled request, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLearningRoadmapHandler_Generate_NoConflictWithIDRoute(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	svc := services.NewLearningRoadmapService(db)
	svc.SetRoadmapGenerationWorkerLauncher(func(uuid.UUID) {})
	handler := handlers.NewLearningRoadmapHandler(svc)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		lr := protected.Group("/api/learning-roadmaps")
		{
			lr.POST("/generate", handler.Generate)
			lr.GET("/:id", handler.GetDetail)
		}
	}

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"learning_goal": "Học Flutter",
			"max_duration":  300,
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 (async job created before AI check), got %d", w.Code)
	}

	id := uuid.New().String()
	req2, _ := http.NewRequest("GET", "/api/learning-roadmaps/"+id, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Fatalf("/:id route should still work, got %d", w2.Code)
	}
}

func TestLearningRoadmapHandler_Generate_ResponseHasSteps(t *testing.T) {
	r, userID, db := setupGenerateRouter(t, validGenerateAIResponse(), nil)
	roadmap, steps := seedRoadmapData(t, db, userID)
	if err := db.Model(roadmap).Updates(map[string]interface{}{
		"source":             models.LearningRoadmapSourceAI,
		"created_by_user_id": userID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	hash, prefs, _ := services.BuildLearningRoadmapRequestHash(handlerValidGenerateRoadmapRequest())
	completedAt := time.Now()
	job := models.LearningRoadmapGenerationJob{
		UserID:             userID,
		RequestHash:        hash,
		Preferences:        prefs,
		Status:             models.LearningRoadmapGenJobStatusCompleted,
		RoadmapID:          &roadmap.ID,
		GeneratedStepCount: len(steps),
		CompletedAt:        &completedAt,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", "/api/learning-roadmaps/generate/status?job_id="+job.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	item := data["item"].(map[string]interface{})
	responseSteps := item["steps"].([]interface{})
	if len(responseSteps) == 0 {
		t.Error("expected steps array in response item")
	}

	for _, s := range responseSteps {
		step := s.(map[string]interface{})
		if step["id"] == nil {
			t.Error("step missing id")
		}
		if step["title"] == nil {
			t.Error("step missing title")
		}
		if step["order_index"] == nil {
			t.Error("step missing order_index")
		}
	}
}

func TestLearningRoadmapHandler_Generate_AIProviderError(t *testing.T) {
	mockErr := errors.New("simulated error")

	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	svc := services.NewLearningRoadmapService(db)
	mock := ai.NewMockClient("", mockErr)
	svc.SetAIClient(mock)
	svc.SetRoadmapGenerationWorkerLauncher(func(uuid.UUID) {})
	handler := handlers.NewLearningRoadmapHandler(svc)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		lr := protected.Group("/api/learning-roadmaps")
		{
			lr.POST("/generate", handler.Generate)
		}
	}

	body, _ := json.Marshal(map[string]interface{}{
		"preferences": map[string]interface{}{
			"learning_goal": "Học Flutter",
			"max_duration":  300,
		},
	})

	req, _ := http.NewRequest("POST", "/api/learning-roadmaps/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 because AI provider errors are reported by polling, got %d: %s", w.Code, w.Body.String())
	}
}
