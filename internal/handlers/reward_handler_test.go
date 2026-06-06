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
			rewards.POST("", rewardHandler.CreateReward)
			rewards.PATCH("/:id", rewardHandler.UpdateReward)
			rewards.DELETE("/:id", rewardHandler.DeleteReward)
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

	items, ok := unwrapData(t, resp)["items"].([]interface{})
	if !ok || len(items) == 0 {
		t.Fatal("expected items array with at least 1 reward")
	}

	wallet, ok := unwrapData(t, resp)["wallet"].(map[string]interface{})
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

	rewardResp, ok := unwrapData(t, resp)["reward"].(map[string]interface{})
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

	items, ok := unwrapData(t, resp)["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 redemption, got %v", unwrapData(t, resp)["items"])
	}
}

func TestCreateReward_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	body := map[string]interface{}{
		"title":            "Chơi game 45 phút",
		"description":      "Tự thưởng sau quest",
		"type":             "entertainment",
		"cost_points":      60,
		"icon_text":        "🎮",
		"duration_minutes": 45,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/rewards", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	code, _ := resp["code"].(float64)
	if int(code) != 201 {
		t.Errorf("expected code 201, got %v", code)
	}

	data := resp["data"].(map[string]interface{})
	if data["title"] != "Chơi game 45 phút" {
		t.Errorf("expected title, got '%v'", data["title"])
	}
	if data["type"] != "entertainment" {
		t.Errorf("expected type entertainment, got '%v'", data["type"])
	}
	if int(data["cost_points"].(float64)) != 60 {
		t.Errorf("expected cost_points 60, got %v", data["cost_points"])
	}
	if data["status"] != "available" {
		t.Errorf("expected status available, got '%v'", data["status"])
	}
	if int(data["duration_minutes"].(float64)) != 45 {
		t.Errorf("expected duration_minutes 45, got %v", data["duration_minutes"])
	}
	if _, ok := data["id"]; !ok {
		t.Error("response missing id")
	}
}

func TestCreateReward_DefaultIcon(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	body := map[string]interface{}{
		"title":       "Test Reward",
		"type":        "custom",
		"cost_points": 50,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/rewards", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})

	if data["icon_text"] != "🎁" {
		t.Errorf("expected default icon 🎁, got '%v'", data["icon_text"])
	}
}

func TestCreateReward_MissingTitle_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	body := map[string]interface{}{
		"type":        "entertainment",
		"cost_points": 50,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/rewards", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateReward_InvalidType_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	body := map[string]interface{}{
		"title":       "Test",
		"type":        "invalid_type_xyz",
		"cost_points": 50,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/rewards", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateReward_NegativeCost_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	body := map[string]interface{}{
		"title":       "Test",
		"type":        "rest",
		"cost_points": -5,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/rewards", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateReward_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	body := map[string]interface{}{
		"title":       "Updated Reward",
		"cost_points": 45,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/rewards/"+reward.ID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})

	if data["title"] != "Updated Reward" {
		t.Errorf("expected title 'Updated Reward', got '%v'", data["title"])
	}
	if int(data["cost_points"].(float64)) != 45 {
		t.Errorf("expected cost_points 45, got %v", data["cost_points"])
	}
}

func TestUpdateReward_NotFound_Returns404(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	body := map[string]interface{}{
		"title": "Test",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/rewards/"+uuid.New().String(), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteReward_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	req, _ := http.NewRequest("DELETE", "/api/rewards/"+reward.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	req2, _ := http.NewRequest("GET", "/api/rewards", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var listResp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &listResp)
	items := unwrapData(t, listResp)["items"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected deleted reward not in list, got %d items", len(items))
	}
}

func TestDeleteReward_NotFound_Returns404(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	req, _ := http.NewRequest("DELETE", "/api/rewards/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestReward_UserIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userA := testutils.BootstrapTestUser(t, db)
	userB := uuid.New()
	testutils.CreateTestUser(db, userB, "userb@test.com")

	svc := services.NewRewardService(db)

	rewardA := models.Reward{
		UserID:     userA,
		Title:      "Reward A",
		Type:       models.RewardTypeRest,
		CostPoints: 30,
		Status:     models.RewardStatusAvailable,
	}
	db.Create(&rewardA)

	rewardB := models.Reward{
		UserID:     userB,
		Title:      "Reward B",
		Type:       models.RewardTypeRest,
		CostPoints: 30,
		Status:     models.RewardStatusAvailable,
	}
	db.Create(&rewardB)

	handler := handlers.NewRewardHandler(svc)

	rA := gin.New()
	rA.GET("/api/rewards", testutils.AuthMiddleware(userA), handler.GetRewards)
	reqA, _ := http.NewRequest("GET", "/api/rewards", nil)
	wA := httptest.NewRecorder()
	rA.ServeHTTP(wA, reqA)

	var respA map[string]interface{}
	json.Unmarshal(wA.Body.Bytes(), &respA)
	itemsA := unwrapData(t, respA)["items"].([]interface{})
	if len(itemsA) != 1 {
		t.Errorf("user A should see 1 reward, got %d", len(itemsA))
	}

	// User A cannot update User B's reward
	body := map[string]interface{}{"title": "Hacked"}
	jsonBody, _ := json.Marshal(body)
	rPatchA := gin.New()
	rPatchA.PATCH("/api/rewards/:id", testutils.AuthMiddleware(userA), handler.UpdateReward)
	req, _ := http.NewRequest("PATCH", "/api/rewards/"+rewardB.ID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	rPatchA.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("user A should not update user B's reward, got status %d", w.Code)
	}
}

func TestClaimReward_DoesNotDeductExp(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	var userBefore models.UserProfile
	db.Where("id = ?", userID).First(&userBefore)
	expBefore := userBefore.TotalExp

	req, _ := http.NewRequest("POST", "/api/rewards/"+reward.ID.String()+"/claim", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var userAfter models.UserProfile
	db.Where("id = ?", userID).First(&userAfter)
	if userAfter.TotalExp != expBefore {
		t.Errorf("EXP changed from %d to %d, should remain unchanged", expBefore, userAfter.TotalExp)
	}
	if userAfter.RewardPoints != userBefore.RewardPoints-30 {
		t.Errorf("RewardPoints should decrease by 30, from %d to %d", userBefore.RewardPoints, userAfter.RewardPoints)
	}
}

func TestClaimReward_CreatesRedemption(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	req, _ := http.NewRequest("POST", "/api/rewards/"+reward.ID.String()+"/claim", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var count int64
	db.Model(&models.RewardRedemption{}).Where("user_id = ? AND reward_id = ?", userID, reward.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 redemption record, got %d", count)
	}
}

func TestReward_GetRewardsResponseEnvelope(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	req, _ := http.NewRequest("GET", "/api/rewards", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	code, _ := resp["code"].(float64)
	if int(code) != 200 {
		t.Errorf("expected code 200, got %v", code)
	}
	if _, ok := resp["message"]; !ok {
		t.Error("response missing message")
	}

	data := unwrapData(t, resp)
	if _, ok := data["items"]; !ok {
		t.Error("response data missing items")
	}
	wallet, ok := data["wallet"].(map[string]interface{})
	if !ok {
		t.Fatal("response data missing wallet")
	}
	if _, ok := wallet["reward_points"]; !ok {
		t.Error("wallet missing reward_points")
	}
}

func TestReward_CreateRewardResponseEnvelope(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	body := map[string]interface{}{
		"title":       "Test Reward",
		"type":        "rest",
		"cost_points": 30,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/rewards", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if code, ok := resp["code"].(float64); !ok || int(code) != 201 {
		t.Errorf("expected code 201, got %v", resp["code"])
	}
	if _, ok := resp["message"]; !ok {
		t.Error("response missing message")
	}
	if _, ok := resp["data"]; !ok {
		t.Error("response missing data")
	}
}

func TestReward_AllTypesValid(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupRewardHandlerRouter(t, db)

	types := []string{"rest", "entertainment", "food", "shopping", "learning", "social", "self_care", "custom"}
	for _, typ := range types {
		body := map[string]interface{}{
			"title":       "Test " + typ,
			"type":        typ,
			"cost_points": 10,
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/rewards", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("type '%s': expected 201, got %d: %s", typ, w.Code, w.Body.String())
		}
	}
}

func TestReward_UpdateStatusViaPATCH(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupRewardHandlerRouter(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	body := map[string]interface{}{
		"status": "claimed",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/rewards/"+reward.ID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["status"] != "claimed" {
		t.Errorf("expected status claimed, got '%v'", data["status"])
	}
}
