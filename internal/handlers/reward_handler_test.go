package handlers_test

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

func setupRewardHandlerRouter(t *testing.T, db *gorm.DB) (*gin.Engine, uuid.UUID) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	rewardService := services.NewRewardService(db)
	rewardHandler := handlers.NewRewardHandler(rewardService)

	userID := testutils.BootstrapTestUser(t, db)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		rewards := protected.Group("/api/rewards")
		{
			rewards.GET("", rewardHandler.GetRewards)
			rewards.GET("/redemptions", rewardHandler.GetRedemptions)
			rewards.POST("/:id/claim", rewardHandler.ClaimReward)
		}
	}

	return r, userID
}

func TestGetRewards_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	req, _ := http.NewRequest("GET", "/api/rewards", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	items, ok := resp["items"].([]interface{})
	if !ok || len(items) == 0 {
		t.Fatal("expected items array with at least 1 reward")
	}

	wallet, ok := resp["wallet"].(map[string]interface{})
	if !ok {
		t.Fatal("expected wallet in response")
	}

	if wallet["reward_points"] != float64(100) {
		t.Errorf("expected wallet reward_points 100, got %v", wallet["reward_points"])
	}
}

func TestClaimReward_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	req, _ := http.NewRequest("POST", "/api/rewards/"+reward.ID.String()+"/claim", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["message"] != "reward claimed successfully" {
		t.Errorf("expected success message, got '%s'", resp["message"])
	}

	rewardResp, ok := resp["reward"].(map[string]interface{})
	if !ok {
		t.Fatal("expected reward in response")
	}
	if rewardResp["status"] != "claimed" {
		t.Errorf("expected reward status 'claimed', got '%s'", rewardResp["status"])
	}
}

func TestClaimReward_InsufficientPoints_Returns409(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 200, models.RewardStatusAvailable)

	req, _ := http.NewRequest("POST", "/api/rewards/"+reward.ID.String()+"/claim", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClaimReward_DoubleClaim_Returns409(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	req, _ := http.NewRequest("POST", "/api/rewards/"+reward.ID.String()+"/claim", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first claim: expected 200, got %d", w.Code)
	}

	req, _ = http.NewRequest("POST", "/api/rewards/"+reward.ID.String()+"/claim", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("second claim: expected 409, got %d", w.Code)
	}
}

func TestClaimReward_InvalidID_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	req, _ := http.NewRequest("POST", "/api/rewards/not-a-uuid/claim", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestGetRedemptions_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	svc := services.NewRewardService(db)
	svc.ClaimReward(userID, reward.ID)

	req, _ := http.NewRequest("GET", "/api/rewards/redemptions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	items, ok := resp["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 redemption, got %v", resp["items"])
	}
}
