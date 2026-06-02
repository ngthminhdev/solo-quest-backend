package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/models"
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

	if resp["has_reviewed"] != false {
		t.Error("expected has_reviewed to be false")
	}
}

func TestReviewSummary_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
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

	if int(resp["completed_quest_count"].(float64)) != 1 {
		t.Errorf("expected completed=1, got %v", resp["completed_quest_count"])
	}
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

func TestReviewSave_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	r := setupReviewHandlerRouterWithUser(t, db, userID)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":                  "good",
		"difficulty_rating":     3,
		"energy_level":          4,
		"satisfaction_level":    4,
		"helpful_quests":        []string{"water"},
		"best_moment":           "Finished API",
		"note":                  "Good day",
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

func TestReviewSave_InvalidMood_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood": "invalid",
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReviewSave_InvalidRating_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupReviewHandlerRouter(t, db)

	body, _ := json.Marshal(map[string]interface{}{
		"mood":              "good",
		"difficulty_rating": 6,
	})

	req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}
