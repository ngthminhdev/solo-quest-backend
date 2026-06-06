package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupReviewHandlerRouter(t *testing.T, db *gorm.DB) (*gin.Engine, uuid.UUID) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	reviewService := services.NewDailyReviewService(db)
	reviewHandler := handlers.NewDailyReviewHandler(reviewService)

	userID := testutils.BootstrapTestUser(t, db)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		reviews := protected.Group("/api/reviews")
		{
			reviews.GET("/today", reviewHandler.GetToday)
			reviews.GET("/summary", reviewHandler.GetSummary)
			reviews.GET("", reviewHandler.GetByDate)
			reviews.POST("", reviewHandler.Save)
		}
	}

	return r, userID
}

func setupReviewHandlerRouterWithUser(t *testing.T, db *gorm.DB, userID uuid.UUID) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	reviewService := services.NewDailyReviewService(db)
	reviewHandler := handlers.NewDailyReviewHandler(reviewService)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		reviews := protected.Group("/api/reviews")
		{
			reviews.GET("/today", reviewHandler.GetToday)
			reviews.GET("/summary", reviewHandler.GetSummary)
			reviews.GET("", reviewHandler.GetByDate)
			reviews.POST("", reviewHandler.Save)
		}
	}

	return r
}

func TestReviewGetToday_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/reviews/today", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if unwrapData(t, resp)["has_reviewed"] != false {
		t.Error("expected has_reviewed to be false")
	}
}

func TestReviewSummary_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	r := setupReviewHandlerRouterWithUser(t, db, userID)

	req, _ := http.NewRequest("GET", "/api/reviews/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(unwrapData(t, resp)["completed_quest_count"].(float64)) != 1 {
		t.Errorf("expected completed=1, got %v", unwrapData(t, resp)["completed_quest_count"])
	}
}

func TestReviewSave_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	r := setupReviewHandlerRouterWithUser(t, db, userID)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "medium",
		"satisfaction":      4,
		"tomorrow_priority": "learning",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["message"] != "daily review saved successfully" {
		t.Errorf("expected success message, got '%s'", resp["message"])
	}
}

func TestReviewSave_WithDateAndReflection_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := setupReviewHandlerRouterWithUser(t, db, userID)

	body, _ := json.Marshal(map[string]interface{}{
		"date":              "2026-06-03",
		"mood":              "normal",
		"energy_level":      "low",
		"satisfaction":      3,
		"reflection":        "Hôm nay hơi mệt.",
		"tomorrow_priority": "rest",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	item := unwrapData(t, resp)["item"].(map[string]interface{})
	if item["date"] != "2026-06-03" {
		t.Errorf("expected date '2026-06-03', got '%s'", item["date"])
	}
	if item["reflection"] != "Hôm nay hơi mệt." {
		t.Errorf("expected reflection 'Hôm nay hơi mệt.', got '%s'", item["reflection"])
	}
}

func TestReviewSave_MissingMood_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"energy_level":      "medium",
		"satisfaction":      4,
		"tomorrow_priority": "learning",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_MissingEnergyLevel_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"satisfaction":      4,
		"tomorrow_priority": "learning",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_MissingSatisfaction_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "medium",
		"tomorrow_priority": "learning",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_MissingTomorrowPriority_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":         "good",
		"energy_level": "medium",
		"satisfaction": 4,
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_InvalidMood_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "veryLow",
		"energy_level":      "medium",
		"satisfaction":      4,
		"tomorrow_priority": "learning",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_InvalidEnergyLevel_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "very_high",
		"satisfaction":      4,
		"tomorrow_priority": "learning",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_InvalidTomorrowPriority_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "medium",
		"satisfaction":      4,
		"tomorrow_priority": "school",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_InvalidSatisfaction_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "medium",
		"satisfaction":      0,
		"tomorrow_priority": "learning",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for satisfaction=0, got %d: %s", w.Code, w.Body.String())
	}

	body, _ = json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "medium",
		"satisfaction":      6,
		"tomorrow_priority": "learning",
	})

	req, _ = http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for satisfaction=6, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_ReflectionMaxLength_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	longReflection := strings.Repeat("a", 201)
	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "medium",
		"satisfaction":      4,
		"tomorrow_priority": "learning",
		"reflection":        longReflection,
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_OldFieldsNotRequired_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := setupReviewHandlerRouterWithUser(t, db, userID)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "medium",
		"satisfaction":      4,
		"tomorrow_priority": "learning",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_ResponseIncludesNewFields(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := setupReviewHandlerRouterWithUser(t, db, userID)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "high",
		"satisfaction":      5,
		"reflection":        "Hôm nay hoàn thành tốt.",
		"tomorrow_priority": "health",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
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
		t.Errorf("expected mood 'good', got '%s'", item["mood"])
	}
	if item["energy_level"] != "high" {
		t.Errorf("expected energy_level 'high', got '%s'", item["energy_level"])
	}
	if int(item["satisfaction"].(float64)) != 5 {
		t.Errorf("expected satisfaction 5, got %v", item["satisfaction"])
	}
	if item["reflection"] != "Hôm nay hoàn thành tốt." {
		t.Errorf("expected reflection, got '%s'", item["reflection"])
	}
	if item["tomorrow_priority"] != "health" {
		t.Errorf("expected tomorrow_priority 'health', got '%s'", item["tomorrow_priority"])
	}
}

func TestReviewGetToday_ResponseIncludesNewFields(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	r := setupReviewHandlerRouterWithUser(t, db, userID)

	saveBody, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"energy_level":      "high",
		"satisfaction":      5,
		"reflection":        "Tốt lắm",
		"tomorrow_priority": "learning",
	})

	saveReq, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(saveBody))
	saveReq.Header.Set("Content-Type", "application/json")
	saveW := httptest.NewRecorder()
	r.ServeHTTP(saveW, saveReq)

	if saveW.Code != http.StatusOK {
		t.Fatalf("save failed: %d: %s", saveW.Code, saveW.Body.String())
	}

	getReq, _ := http.NewRequest("GET", "/api/reviews/today", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", getW.Code, getW.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(getW.Body.Bytes(), &resp)

	if unwrapData(t, resp)["has_reviewed"] != true {
		t.Error("expected has_reviewed to be true")
	}

	item := unwrapData(t, resp)["item"].(map[string]interface{})
	if item["mood"] != "good" {
		t.Errorf("expected mood 'good', got '%s'", item["mood"])
	}
	if item["energy_level"] != "high" {
		t.Errorf("expected energy_level 'high', got '%s'", item["energy_level"])
	}
	if int(item["satisfaction"].(float64)) != 5 {
		t.Errorf("expected satisfaction 5, got %v", item["satisfaction"])
	}
	if item["tomorrow_priority"] != "learning" {
		t.Errorf("expected tomorrow_priority 'learning', got '%s'", item["tomorrow_priority"])
	}
}
