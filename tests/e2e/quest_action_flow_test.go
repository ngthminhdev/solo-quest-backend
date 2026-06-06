package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/middleware"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	questService := services.NewQuestService(db)
	questActionService := services.NewQuestActionService(db)

	questHandler := handlers.NewQuestHandler(questService)
	questActionHandler := handlers.NewQuestActionHandler(questActionService)

	protected := r.Group("")
	protected.Use(middleware.DevUserContext())
	{
		protected.GET("/api/quests", questHandler.GetQuests)
		protected.POST("/api/quests/:id/start", questActionHandler.StartQuest)
		protected.POST("/api/quests/:id/complete", questActionHandler.CompleteQuest)
		protected.POST("/api/quests/:id/skip", questActionHandler.SkipQuest)
		protected.POST("/api/quests/:id/snooze", questActionHandler.SnoozeQuest)
	}

	return r
}

func TestQuestActionFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	r := setupTestRouter(t, db)

	t.Run("GET /api/quests", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/quests", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		quests, ok := data["quests"].([]interface{})
		if !ok || len(quests) == 0 {
			t.Fatal("expected quests array with at least 1 quest")
		}
	})

	t.Run("POST /api/quests/:id/start", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/start", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		questResp, ok := data["quest"].(map[string]interface{})
		if !ok {
			t.Fatal("expected quest in response")
		}

		if questResp["status"] != "active" {
			t.Errorf("expected status 'active', got '%s'", questResp["status"])
		}
	})

	t.Run("POST /api/quests/:id/complete", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"note": "Done!"})
		req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/complete", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		questResp, ok := data["quest"].(map[string]interface{})
		if !ok {
			t.Fatal("expected quest in response")
		}

		if questResp["status"] != "completed" {
			t.Errorf("expected status 'completed', got '%s'", questResp["status"])
		}

		if resp["message"] != "quest completed successfully" {
			t.Errorf("expected success message, got '%s'", resp["message"])
		}
	})

	t.Run("GET /api/quests after complete", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/quests", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		quests, ok := data["quests"].([]interface{})
		if !ok || len(quests) == 0 {
			t.Fatal("expected quests array with at least 1 quest")
		}

		firstQuest := quests[0].(map[string]interface{})
		if firstQuest["status"] != "completed" {
			t.Errorf("expected quest status 'completed', got '%s'", firstQuest["status"])
		}
	})

	t.Run("POST /api/quests/:id/complete double award", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"note": "Try again"})
		req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/complete", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", w.Code)
		}
	})

	t.Run("Verify user profile updated", func(t *testing.T) {
		var user models.UserProfile
		db.Where("id = ?", userID).First(&user)

		if user.TotalExp != quest.XPReward {
			t.Errorf("expected total_exp %d, got %d", quest.XPReward, user.TotalExp)
		}

		if user.TotalCompletedQuests != 1 {
			t.Errorf("expected total_completed_quests 1, got %d", user.TotalCompletedQuests)
		}
	})

	t.Run("Verify XP transactions created", func(t *testing.T) {
		var expTxCount int64
		db.Model(&models.XPTransaction{}).Where("user_id = ? AND currency = ?", userID, models.XPCurrencyXP).Count(&expTxCount)
		if expTxCount != 1 {
			t.Errorf("expected 1 exp transaction, got %d", expTxCount)
		}

		var rewardTxCount int64
		db.Model(&models.XPTransaction{}).Where("user_id = ? AND currency = ?", userID, models.XPCurrencyRewardPoints).Count(&rewardTxCount)
		if rewardTxCount != 1 {
			t.Errorf("expected 1 reward points transaction, got %d", rewardTxCount)
		}
	})

	t.Run("Verify logs contain questCompleted", func(t *testing.T) {
		var logCount int64
		db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeQuestCompleted).Count(&logCount)
		if logCount != 1 {
			t.Errorf("expected 1 questCompleted log, got %d", logCount)
		}
	})
}

func TestSnoozeQuestFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	r := setupTestRouter(t, db)

	t.Run("POST /api/quests/:id/snooze", func(t *testing.T) {
		body, _ := json.Marshal(map[string]int{"minutes": 15})
		req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/snooze", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		questResp, ok := data["quest"].(map[string]interface{})
		if !ok {
			t.Fatal("expected quest in response")
		}

		if questResp["status"] != "snoozed" {
			t.Errorf("expected status 'snoozed', got '%s'", questResp["status"])
		}
	})

	t.Run("POST /api/quests/:id/snooze invalid minutes", func(t *testing.T) {
		body, _ := json.Marshal(map[string]int{"minutes": 7})
		req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/snooze", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})
}

func TestSkipQuestFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	r := setupTestRouter(t, db)

	t.Run("POST /api/quests/:id/skip", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"reason": "Đang bận"})
		req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/skip", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		questResp, ok := data["quest"].(map[string]interface{})
		if !ok {
			t.Fatal("expected quest in response")
		}

		if questResp["status"] != "skipped" {
			t.Errorf("expected status 'skipped', got '%s'", questResp["status"])
		}
	})

	t.Run("POST /api/quests/:id/skip on completed quest", func(t *testing.T) {
		completedQuest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusCompleted)

		body, _ := json.Marshal(map[string]string{"reason": "Test"})
		req, _ := http.NewRequest("POST", "/api/quests/"+completedQuest.ID.String()+"/skip", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", w.Code)
		}
	})
}
