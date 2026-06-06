package e2e

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
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupDailyFlowRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	userService := services.NewUserService(db)
	questService := services.NewQuestService(db)
	checkinService := services.NewDailyCheckinService(db)
	reviewService := services.NewDailyReviewService(db)
	logService := services.NewLogService(db)

	userHandler := handlers.NewUserHandlerWithDaily(userService, checkinService, reviewService)
	questHandler := handlers.NewQuestHandler(questService)
	checkinHandler := handlers.NewDailyCheckinHandler(checkinService)
	reviewHandler := handlers.NewDailyReviewHandler(reviewService)
	logHandler := handlers.NewLogHandler(logService)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(testDailyUserID()))
	{
		users := protected.Group("/api/users")
		{
			users.GET("/me/daily-status", userHandler.GetDailyStatus)
		}

		quests := protected.Group("/api/quests")
		{
			quests.GET("", questHandler.GetQuests)
		}

		checkins := protected.Group("/api/checkins")
		{
			checkins.GET("/today", checkinHandler.GetToday)
			checkins.POST("", checkinHandler.Save)
		}

		reviews := protected.Group("/api/reviews")
		{
			reviews.GET("/today", reviewHandler.GetToday)
			reviews.GET("/summary", reviewHandler.GetSummary)
			reviews.POST("", reviewHandler.Save)
		}

		logs := protected.Group("/api/logs")
		{
			logs.GET("", logHandler.GetLogs)
		}
	}

	return r
}

func testDailyUserID() uuid.UUID {
	return uuid.MustParse("00000000-0000-0000-0000-000000000001")
}

func unwrapData(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	d, ok := resp["data"]
	if !ok {
		t.Fatal("response missing 'data' field")
	}
	return d.(map[string]interface{})
}

func TestDailyCheckinReviewFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := setupDailyFlowRouter(t, db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)

	t.Run("GET /api/users/me/daily-status before check-in", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/users/me/daily-status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		if data["has_checked_in_today"] != false {
			t.Error("expected has_checked_in_today = false")
		}
		if data["has_reviewed_today"] != false {
			t.Error("expected has_reviewed_today = false")
		}
	})

	t.Run("POST /api/checkins", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"mood":         "good",
			"energy_level": "high",
			"availability": "normal",
			"priority":     "learning",
		})

		req, _ := http.NewRequest("POST", "/api/checkins", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["message"] != "daily check-in saved successfully" {
			t.Errorf("expected success message, got '%v'", resp["message"])
		}
	})

	t.Run("GET /api/checkins/today after check-in", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/checkins/today", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		if data["has_checked_in"] != true {
			t.Error("expected has_checked_in = true")
		}
	})

	db.Create(&models.Quest{UserID: userID, Title: "Uống nước", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})
	db.Create(&models.Quest{UserID: userID, Title: "Học Go", Type: models.QuestTypeLearning, Status: models.QuestStatusCompleted, XPReward: 15, Date: today, CreatedAt: now})
	db.Create(&models.Quest{UserID: userID, Title: "Tập thể dục", Type: models.QuestTypeMovement, Status: models.QuestStatusSkipped, XPReward: 10, Date: today, CreatedAt: now})
	db.Create(&models.Quest{UserID: userID, Title: "Review code", Type: models.QuestTypeReview, Status: models.QuestStatusPending, XPReward: 10, Date: today, CreatedAt: now})

	t.Run("GET /api/reviews/summary", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/reviews/summary", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		if int(data["completed_quest_count"].(float64)) != 2 {
			t.Errorf("expected completed=2, got %v", data["completed_quest_count"])
		}
		if int(data["skipped_quest_count"].(float64)) != 1 {
			t.Errorf("expected skipped=1, got %v", data["skipped_quest_count"])
		}
		if int(data["pending_quest_count"].(float64)) != 1 {
			t.Errorf("expected pending=1, got %v", data["pending_quest_count"])
		}
		if int(data["total_quest_count"].(float64)) != 4 {
			t.Errorf("expected total=4, got %v", data["total_quest_count"])
		}
		if int(data["earned_exp"].(float64)) != 20 {
			t.Errorf("expected earned_exp=20, got %v", data["earned_exp"])
		}

		completedByType := data["completed_by_type"].(map[string]interface{})
		if int(completedByType["water"].(float64)) != 1 {
			t.Errorf("expected water=1, got %v", completedByType["water"])
		}
		if int(completedByType["learning"].(float64)) != 1 {
			t.Errorf("expected learning=1, got %v", completedByType["learning"])
		}
	})

	t.Run("POST /api/reviews", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"mood":              "good",
			"energy_level":      "medium",
			"satisfaction":      4,
			"reflection":        "Hoàn thành tốt các quest chính.",
			"tomorrow_priority": "learning",
		})

		req, _ := http.NewRequest("POST", "/api/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["message"] != "daily review saved successfully" {
			t.Errorf("expected success message, got '%v'", resp["message"])
		}
	})

	t.Run("GET /api/reviews/today after review", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/reviews/today", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		if data["has_reviewed"] != true {
			t.Error("expected has_reviewed = true")
		}
	})

	t.Run("GET /api/users/me/daily-status after both", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/users/me/daily-status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		if data["has_checked_in_today"] != true {
			t.Error("expected has_checked_in_today = true")
		}
		if data["has_reviewed_today"] != true {
			t.Error("expected has_reviewed_today = true")
		}
	})

	t.Run("Verify logs contain morningCheckin and dailyReview", func(t *testing.T) {
		var checkinLogCount int64
		db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeMorningCheckin).Count(&checkinLogCount)
		if checkinLogCount != 1 {
			t.Errorf("expected 1 morningCheckin log, got %d", checkinLogCount)
		}

		var reviewLogCount int64
		db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeDailyReview).Count(&reviewLogCount)
		if reviewLogCount != 1 {
			t.Errorf("expected 1 dailyReview log, got %d", reviewLogCount)
		}
	})
}
