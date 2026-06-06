package e2e

import (
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

func setupRewardClaimRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	rewardService := services.NewRewardService(db)
	progressService := services.NewProgressService(db)
	logService := services.NewLogService(db)

	rewardHandler := handlers.NewRewardHandler(rewardService)
	progressHandler := handlers.NewProgressHandler(progressService)
	logHandler := handlers.NewLogHandler(logService)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(rewardClaimUserID()))
	{
		rewards := protected.Group("/api/rewards")
		{
			rewards.GET("", rewardHandler.GetRewards)
			rewards.GET("/redemptions", rewardHandler.GetRedemptions)
			rewards.POST("/:id/claim", rewardHandler.ClaimReward)
		}

		progress := protected.Group("/api/progress")
		{
			progress.GET("/xp-history", progressHandler.GetXPHistory)
		}

		logs := protected.Group("/api/logs")
		{
			logs.GET("", logHandler.GetLogs)
		}
	}

	return r
}

func rewardClaimUserID() uuid.UUID {
	return uuid.MustParse("00000000-0000-0000-0000-000000000001")
}

func TestRewardClaimFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	r := setupRewardClaimRouter(t, db)

	t.Run("GET /api/rewards", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/rewards", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		if len(items) != 1 {
			t.Fatalf("expected 1 reward, got %d", len(items))
		}

		firstReward := items[0].(map[string]interface{})
		if firstReward["can_claim"] != true {
			t.Error("expected can_claim = true")
		}

		wallet := data["wallet"].(map[string]interface{})
		if wallet["reward_points"] != float64(100) {
			t.Errorf("expected wallet reward_points 100, got %v", wallet["reward_points"])
		}
	})

	t.Run("POST /api/rewards/:id/claim", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/rewards/"+reward.ID.String()+"/claim", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		if resp["message"] != "reward claimed successfully" {
			t.Errorf("expected success message, got '%s'", resp["message"])
		}

		rewardResp := data["reward"].(map[string]interface{})
		if rewardResp["status"] != "claimed" {
			t.Errorf("expected reward status 'claimed', got '%s'", rewardResp["status"])
		}

		profile := data["profile"].(map[string]interface{})
		if profile["reward_points"] != float64(70) {
			t.Errorf("expected profile reward_points 70, got %v", profile["reward_points"])
		}
	})

	t.Run("Verify user reward_points decreased", func(t *testing.T) {
		var user models.UserProfile
		db.Where("id = ?", userID).First(&user)
		if user.RewardPoints != 70 {
			t.Errorf("expected user reward_points 70, got %d", user.RewardPoints)
		}
	})

	t.Run("GET /api/rewards/redemptions", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/rewards/redemptions", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		if len(items) != 1 {
			t.Fatalf("expected 1 redemption, got %d", len(items))
		}

		first := items[0].(map[string]interface{})
		if first["points_spent"] != float64(30) {
			t.Errorf("expected points_spent 30, got %v", first["points_spent"])
		}
	})

	t.Run("GET /api/progress/xp-history?currency=reward_points", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/progress/xp-history?currency=reward_points", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		found := false
		for _, item := range items {
			tx := item.(map[string]interface{})
			if tx["source"] == "reward_claim" && tx["amount"] == float64(-30) {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected reward_claim transaction with amount -30")
		}
	})

	t.Run("GET /api/logs?type=rewardClaimed", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/logs?type=rewardClaimed", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		if len(items) != 1 {
			t.Fatalf("expected 1 rewardClaimed log, got %d", len(items))
		}
	})

	t.Run("POST /api/rewards/:id/claim again returns 409", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/rewards/"+reward.ID.String()+"/claim", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Verify user reward_points remains 70", func(t *testing.T) {
		var user models.UserProfile
		db.Where("id = ?", userID).First(&user)
		if user.RewardPoints != 70 {
			t.Errorf("expected user reward_points 70, got %d", user.RewardPoints)
		}
	})
}
