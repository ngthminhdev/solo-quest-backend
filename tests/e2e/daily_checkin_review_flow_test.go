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

func TestDailyCheckinReviewFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := setupDailyFlowRouter(t, db)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// 1. Check daily-status before check-in
	t.Run("GET /api/users/me/daily-status before check-in", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/users/me/daily-status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["has_checked_in_today"] != false {
			t.Error("expected has_checked_in_today = false")
		}
		if resp["has_reviewed_today"] != false {
			t.Error("expected has_reviewed_today = false")
		}
	})

	// 2. POST /api/checkins
	t.Run("POST /api/checkins", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"energy_level":          "high",
			"stress_level":          "low",
			"focus_level":           "medium",
			"day_intensity":         "normal",
			"main_focus_today":      "Backend Phase 9",
			"note":                  "Focus on check-in/review API",
			"available_time_blocks": []string{"morning", "evening"},
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

	// 3. GET /api/checkins/today after check-in
	t.Run("GET /api/checkins/today after check-in", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/checkins/today", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["has_checked_in"] != true {
			t.Error("expected has_checked_in = true")
		}
	})

	// 4. Create quests for today
	db.Create(&models.Quest{UserID: userID, Title: "Uống nước", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})
	db.Create(&models.Quest{UserID: userID, Title: "Học Go", Type: models.QuestTypeLearning, Status: models.QuestStatusCompleted, XPReward: 15, Date: today, CreatedAt: now})
	db.Create(&models.Quest{UserID: userID, Title: "Tập thể dục", Type: models.QuestTypeMovement, Status: models.QuestStatusSkipped, XPReward: 10, Date: today, CreatedAt: now})
	db.Create(&models.Quest{UserID: userID, Title: "Review code", Type: models.QuestTypeReview, Status: models.QuestStatusPending, XPReward: 10, Date: today, CreatedAt: now})

	// 5. GET /api/reviews/summary
	t.Run("GET /api/reviews/summary", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/reviews/summary", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if int(resp["completed_quest_count"].(float64)) != 2 {
			t.Errorf("expected completed=2, got %v", resp["completed_quest_count"])
		}
		if int(resp["skipped_quest_count"].(float64)) != 1 {
			t.Errorf("expected skipped=1, got %v", resp["skipped_quest_count"])
		}
		if int(resp["pending_quest_count"].(float64)) != 1 {
			t.Errorf("expected pending=1, got %v", resp["pending_quest_count"])
		}
		if int(resp["total_quest_count"].(float64)) != 4 {
			t.Errorf("expected total=4, got %v", resp["total_quest_count"])
		}
		if int(resp["earned_exp"].(float64)) != 20 {
			t.Errorf("expected earned_exp=20, got %v", resp["earned_exp"])
		}

		completedByType := resp["completed_by_type"].(map[string]interface{})
		if int(completedByType["water"].(float64)) != 1 {
			t.Errorf("expected water=1, got %v", completedByType["water"])
		}
		if int(completedByType["learning"].(float64)) != 1 {
			t.Errorf("expected learning=1, got %v", completedByType["learning"])
		}
	})

	// 6. POST /api/reviews
	t.Run("POST /api/reviews", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"mood":                  "good",
			"difficulty_rating":     3,
			"energy_level":          4,
			"satisfaction_level":    4,
			"helpful_quests":        []string{"water", "learning"},
			"annoying_quests":       []string{"breakTime"},
			"best_moment":           "Hoàn thành backend API",
			"challenge":             "Hơi mệt buổi chiều",
			"improvement_tomorrow":  "Chia task nhỏ hơn",
			"tomorrow_adjustments":  []string{"more_breaks"},
			"note":                  "Ngày khá ổn",
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

		summary := resp["summary"].(map[string]interface{})
		if int(summary["completed_quest_count"].(float64)) != 2 {
			t.Errorf("expected summary completed=2, got %v", summary["completed_quest_count"])
		}
	})

	// 7. GET /api/reviews/today after review
	t.Run("GET /api/reviews/today after review", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/reviews/today", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["has_reviewed"] != true {
			t.Error("expected has_reviewed = true")
		}
	})

	// 8. GET /api/users/me/daily-status after both
	t.Run("GET /api/users/me/daily-status after both", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/users/me/daily-status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["has_checked_in_today"] != true {
			t.Error("expected has_checked_in_today = true")
		}
		if resp["has_reviewed_today"] != true {
			t.Error("expected has_reviewed_today = true")
		}
	})

	// 9. Verify logs contain morningCheckin and dailyReview
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
