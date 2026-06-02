package e2e

import (
	"encoding/json"
	"fmt"
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

func setupProgressTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	questService := services.NewQuestService(db)
	questActionService := services.NewQuestActionService(db)
	progressService := services.NewProgressService(db)
	logService := services.NewLogService(db)

	questHandler := handlers.NewQuestHandler(questService)
	questActionHandler := handlers.NewQuestActionHandler(questActionService)
	progressHandler := handlers.NewProgressHandler(progressService)
	logHandler := handlers.NewLogHandler(logService)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(testUserID()))
	{
		protected.GET("/api/quests", questHandler.GetQuests)
		protected.POST("/api/quests/:id/start", questActionHandler.StartQuest)
		protected.POST("/api/quests/:id/complete", questActionHandler.CompleteQuest)
		protected.POST("/api/quests/:id/skip", questActionHandler.SkipQuest)

		protected.GET("/api/progress", progressHandler.GetProgress)
		protected.GET("/api/progress/weekly-chart", progressHandler.GetWeeklyChart)
		protected.GET("/api/progress/xp-history", progressHandler.GetXPHistory)
		protected.GET("/api/logs", logHandler.GetLogs)
	}

	return r
}

func testUserID() uuid.UUID {
	return uuid.MustParse("00000000-0000-0000-0000-000000000001")
}

func TestProgressLogFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := setupProgressTestRouter(t, db)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Create 3 quests for today: 2 completed, 1 pending
	questIDs := make([]uuid.UUID, 3)
	for i := 0; i < 2; i++ {
		q := models.Quest{
			UserID:    userID,
			Title:     fmt.Sprintf("Quest %d", i+1),
			Type:      models.QuestTypeWater,
			Status:    models.QuestStatusCompleted,
			XPReward:  10,
			Date:      today,
			CreatedAt: now,
		}
		db.Create(&q)
		questIDs[i] = q.ID
	}
	pendingQuest := models.Quest{
		UserID:    userID,
		Title:     "Quest 3",
		Type:      models.QuestTypeLearning,
		Status:    models.QuestStatusPending,
		XPReward:  15,
		Date:      today,
		CreatedAt: now,
	}
	db.Create(&pendingQuest)
	questIDs[2] = pendingQuest.ID

	// Create XP transactions for completed quests
	for i := 0; i < 2; i++ {
		db.Create(&models.XPTransaction{
			UserID:       userID,
			Amount:       10,
			Currency:     models.XPCurrencyXP,
			Source:       models.XPSourceTypeQuestCompletion,
			ReferenceID:  &questIDs[i],
			Description:  fmt.Sprintf("Hoàn thành quest: Quest %d", i+1),
			BalanceAfter: (i + 1) * 10,
			CreatedAt:    now,
		})
		db.Create(&models.XPTransaction{
			UserID:       userID,
			Amount:       10,
			Currency:     models.XPCurrencyRewardPoints,
			Source:       models.XPSourceTypeQuestCompletion,
			ReferenceID:  &questIDs[i],
			Description:  fmt.Sprintf("Nhận điểm thưởng từ quest: Quest %d", i+1),
			BalanceAfter: 100 + (i+1)*10,
			CreatedAt:    now,
		})
	}

	// Create log entries for completed quests
	for i := 0; i < 2; i++ {
		waterType := models.QuestTypeWater
		db.Create(&models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeQuestCompleted,
			Title:     fmt.Sprintf("Hoàn thành quest: Quest %d", i+1),
			Content:   "+10 EXP",
			QuestID:   &questIDs[i],
			QuestType: &waterType,
			CreatedAt: now,
		})
	}

	// 1. Test GET /api/progress
	t.Run("GET /api/progress", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/progress", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if int(resp["today_completed_quests"].(float64)) != 2 {
			t.Errorf("expected today_completed_quests 2, got %v", resp["today_completed_quests"])
		}
		if int(resp["today_total_quests"].(float64)) != 3 {
			t.Errorf("expected today_total_quests 3, got %v", resp["today_total_quests"])
		}

		rate := resp["today_completion_rate"].(float64)
		if rate < 0.66 || rate > 0.67 {
			t.Errorf("expected today_completion_rate ~0.6667, got %f", rate)
		}

		completedByType := resp["completed_by_type"].(map[string]interface{})
		if int(completedByType["water"].(float64)) != 2 {
			t.Errorf("expected completed_by_type water=2, got %v", completedByType["water"])
		}

		weeklyData := resp["weekly_daily_data"].([]interface{})
		if len(weeklyData) != 7 {
			t.Errorf("expected 7 weekly_daily_data items, got %d", len(weeklyData))
		}
	})

	// 2. Test GET /api/progress/weekly-chart
	t.Run("GET /api/progress/weekly-chart", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/progress/weekly-chart", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		items := resp["items"].([]interface{})
		if len(items) != 7 {
			t.Fatalf("expected 7 items, got %d", len(items))
		}

		// Verify week_start is Monday
		weekStart, _ := time.Parse("2006-01-02", resp["week_start"].(string))
		if weekStart.Weekday() != time.Monday {
			t.Errorf("expected week_start to be Monday, got %s", weekStart.Weekday())
		}

		// Verify week_end is Sunday
		weekEnd, _ := time.Parse("2006-01-02", resp["week_end"].(string))
		if weekEnd.Weekday() != time.Sunday {
			t.Errorf("expected week_end to be Sunday, got %s", weekEnd.Weekday())
		}
	})

	// 3. Test GET /api/progress/xp-history
	t.Run("GET /api/progress/xp-history", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/progress/xp-history", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		items := resp["items"].([]interface{})
		if len(items) != 4 { // 2 exp + 2 reward_points
			t.Errorf("expected 4 XP items, got %d", len(items))
		}
	})

	// 4. Test GET /api/logs?type=questCompleted
	t.Run("GET /api/logs filtered by type", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/logs?type=questCompleted", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		items := resp["items"].([]interface{})
		if len(items) != 2 {
			t.Errorf("expected 2 questCompleted items, got %d", len(items))
		}

		for _, item := range items {
			log := item.(map[string]interface{})
			if log["type"] != "questCompleted" {
				t.Errorf("expected type questCompleted, got %v", log["type"])
			}
		}
	})

	// 5. Test GET /api/logs?date=current_utc_date
	t.Run("GET /api/logs filtered by date", func(t *testing.T) {
		dateStr := today.Format("2006-01-02")
		req, _ := http.NewRequest("GET", "/api/logs?date="+dateStr, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		items := resp["items"].([]interface{})
		if len(items) != 2 {
			t.Errorf("expected 2 items for today, got %d", len(items))
		}
	})

	// 6. Test GET /api/progress/xp-history with currency filter
	t.Run("GET /api/progress/xp-history filtered by currency", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/progress/xp-history?currency=xp", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		items := resp["items"].([]interface{})
		if len(items) != 2 {
			t.Errorf("expected 2 xp items, got %d", len(items))
		}

		for _, item := range items {
			xp := item.(map[string]interface{})
			if xp["currency"] != "xp" {
				t.Errorf("expected currency xp, got %v", xp["currency"])
			}
		}
	})

	// 7. Test invalid date format returns 400
	t.Run("GET /api/logs invalid date returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/logs?date=invalid-date", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})

	// 8. Test invalid currency returns 400
	t.Run("GET /api/progress/xp-history invalid currency returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/progress/xp-history?currency=invalid", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})
}
