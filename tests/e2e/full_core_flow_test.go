package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/testutils"
)

func TestFullCoreFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	r := testutils.CreateTestRouter(t, db, userID)

	now := time.Now().In(timeutil.LocationVN)
	today := now.Format("2006-01-02")

	// Step 1: GET /api/users/me
	t.Run("01_GET_/api/users/me", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/users/me", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		user, ok := data["user"].(map[string]interface{})
		if !ok {
			t.Fatal("expected user in response")
		}
		if user["display_name"] != "Test User" {
			t.Errorf("expected display_name 'Test User', got '%s'", user["display_name"])
		}
	})

	// Step 2: GET /api/onboarding/status (initially false)
	t.Run("02_GET_/api/onboarding/status_initial", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/onboarding/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		if data["has_completed_onboarding"] != false {
			t.Error("expected has_completed_onboarding = false initially")
		}
	})

	// Step 3: POST /api/onboarding
	t.Run("03_POST_/api/onboarding", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"display_name":  "Minh Thanh",
			"age":           25,
			"gender":        "Nam",
			"main_activity": "Engineer",
			"main_goals":    []string{"Uống nước", "Học tập"},
		})

		req, _ := http.NewRequest("POST", "/api/onboarding", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["message"] != "onboarding saved successfully" {
			t.Errorf("expected success message, got '%s'", resp["message"])
		}
	})

	// Step 4: GET /api/onboarding/status (now true)
	t.Run("04_GET_/api/onboarding/status_after", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/onboarding/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		if data["has_completed_onboarding"] != true {
			t.Error("expected has_completed_onboarding = true after onboarding")
		}
	})

	// Step 5: POST /api/checkins
	t.Run("05_POST_/api/checkins", func(t *testing.T) {
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
			t.Errorf("expected success message, got '%s'", resp["message"])
		}
	})

	// Step 6: GET /api/users/me/daily-status
	t.Run("06_GET_/api/users/me/daily-status", func(t *testing.T) {
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
		if data["has_reviewed_today"] != false {
			t.Error("expected has_reviewed_today = false (not yet reviewed)")
		}
	})

	// Step 7: Seed quests for today
	quest1 := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)
	quest2 := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)
	// Override dates to today
	todayDate, _ := timeutil.ParseDateVN(today)
	db.Model(&models.Quest{}).Where("id IN ?", []uuid.UUID{quest1.ID, quest2.ID}).Update("date", todayDate)

	// Step 8: GET /api/quests
	t.Run("08_GET_/api/quests", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/quests?date="+today, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		quests, ok := data["quests"].([]interface{})
		if !ok || len(quests) < 2 {
			t.Fatalf("expected at least 2 quests, got %v", data["quests"])
		}
	})

	// Step 9: POST /api/quests/:id/start
	t.Run("09_POST_/api/quests/:id/start", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/quests/"+quest1.ID.String()+"/start", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		quest := data["quest"].(map[string]interface{})
		if quest["status"] != "active" {
			t.Errorf("expected status 'active', got '%s'", quest["status"])
		}
	})

	// Step 10: POST /api/quests/:id/complete
	t.Run("10_POST_/api/quests/:id/complete", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"note": "Done!"})
		req, _ := http.NewRequest("POST", "/api/quests/"+quest1.ID.String()+"/complete", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		quest := data["quest"].(map[string]interface{})
		if quest["status"] != "completed" {
			t.Errorf("expected status 'completed', got '%s'", quest["status"])
		}
		if resp["message"] != "quest completed successfully" {
			t.Errorf("expected success message, got '%s'", resp["message"])
		}
	})

	// Step 11: POST /api/quests/:id/complete again (409)
	t.Run("11_POST_/api/quests/:id/complete_again_409", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"note": "Again"})
		req, _ := http.NewRequest("POST", "/api/quests/"+quest1.ID.String()+"/complete", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Step 12: GET /api/progress
	t.Run("12_GET_/api/progress", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/progress", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		totalXP, ok := data["total_exp"].(float64)
		if !ok {
			t.Fatal("expected total_exp in response")
		}
		if totalXP < 10 {
			t.Errorf("expected total_exp >= 10, got %v", totalXP)
		}

		completedToday, ok := data["today_completed_quests"].(float64)
		if !ok || completedToday < 1 {
			t.Errorf("expected today_completed_quests >= 1, got %v", data["today_completed_quests"])
		}
	})

	// Step 13: GET /api/progress/xp-history?currency=xp
	t.Run("13_GET_/api/progress/xp-history_xp", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/progress/xp-history?currency=xp", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		found := false
		for _, item := range items {
			tx := item.(map[string]interface{})
			if tx["source"] == "quest_completion" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected quest_completion transaction in XP history")
		}
	})

	// Step 14: GET /api/logs?type=questCompleted
	t.Run("14_GET_/api/logs_questCompleted", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/logs?type=questCompleted", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		if len(items) < 1 {
			t.Error("expected at least 1 questCompleted log")
		}
	})

	// Step 15: GET /api/reviews/summary
	t.Run("15_GET_/api/reviews/summary", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/reviews/summary?date="+today, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		totalQuests, ok := data["total_quest_count"].(float64)
		if !ok || totalQuests < 1 {
			t.Errorf("expected total_quest_count >= 1, got %v", data["total_quest_count"])
		}

		completedQuests, ok := data["completed_quest_count"].(float64)
		if !ok || completedQuests < 1 {
			t.Errorf("expected completed_quest_count >= 1, got %v", data["completed_quest_count"])
		}
	})

	// Step 16: POST /api/reviews
	t.Run("16_POST_/api/reviews", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"mood":              "good",
			"energy_level":      "medium",
			"satisfaction":      4,
			"reflection":        "Hoàn thành quest",
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
			t.Errorf("expected success message, got '%s'", resp["message"])
		}
	})

	// Step 17: GET /api/reviews/today
	t.Run("17_GET_/api/reviews/today", func(t *testing.T) {
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

	// Step 18: GET /api/users/me/daily-status (both true)
	t.Run("18_GET_/api/users/me/daily-status_both", func(t *testing.T) {
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

	// Step 19: GET /api/rewards
	t.Run("19_GET_/api/rewards", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/rewards", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		if len(items) < 1 {
			t.Fatal("expected at least 1 reward")
		}

		wallet := data["wallet"].(map[string]interface{})
		if wallet["reward_points"].(float64) < 30 {
			t.Errorf("expected wallet reward_points >= 30, got %v", wallet["reward_points"])
		}

		first := items[0].(map[string]interface{})
		if first["can_claim"] != true {
			t.Error("expected can_claim = true for affordable reward")
		}
	})

	// Step 20: POST /api/rewards/:id/claim
	t.Run("20_POST_/api/rewards/:id/claim", func(t *testing.T) {
		req, _ := http.NewRequest("POST", fmt.Sprintf("/api/rewards/%s/claim", reward.ID.String()), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
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
	})

	// Step 21: GET /api/rewards/redemptions
	t.Run("21_GET_/api/rewards/redemptions", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/rewards/redemptions", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		if len(items) < 1 {
			t.Fatal("expected at least 1 redemption")
		}

		first := items[0].(map[string]interface{})
		if first["points_spent"].(float64) != 30 {
			t.Errorf("expected points_spent 30, got %v", first["points_spent"])
		}
	})

	// Step 22: GET /api/progress/xp-history?currency=reward_points
	t.Run("22_GET_/api/progress/xp-history_reward_points", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/progress/xp-history?currency=reward_points", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		found := false
		for _, item := range items {
			tx := item.(map[string]interface{})
			if tx["source"] == "reward_claim" && tx["amount"].(float64) == -30 {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected reward_claim transaction with amount -30")
		}
	})

	// Step 23: GET /api/logs?type=rewardClaimed
	t.Run("23_GET_/api/logs_rewardClaimed", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/logs?type=rewardClaimed", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := unwrapData(t, resp)

		items := data["items"].([]interface{})
		if len(items) < 1 {
			t.Error("expected at least 1 rewardClaimed log")
		}
	})

	// Step 24: POST /api/rewards/:id/claim again (409)
	t.Run("24_POST_/api/rewards/:id/claim_again_409", func(t *testing.T) {
		req, _ := http.NewRequest("POST", fmt.Sprintf("/api/rewards/%s/claim", reward.ID.String()), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Step 25: Verify log counts
	t.Run("25_Verify_log_counts", func(t *testing.T) {
		var morningCheckinCount int64
		db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeMorningCheckin).Count(&morningCheckinCount)
		if morningCheckinCount != 1 {
			t.Errorf("expected 1 morningCheckin log, got %d", morningCheckinCount)
		}

		var dailyReviewCount int64
		db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeDailyReview).Count(&dailyReviewCount)
		if dailyReviewCount != 1 {
			t.Errorf("expected 1 dailyReview log, got %d", dailyReviewCount)
		}

		var questCompletedCount int64
		db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeQuestCompleted).Count(&questCompletedCount)
		if questCompletedCount != 1 {
			t.Errorf("expected 1 questCompleted log, got %d", questCompletedCount)
		}

		var rewardClaimedCount int64
		db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeRewardClaimed).Count(&rewardClaimedCount)
		if rewardClaimedCount != 1 {
			t.Errorf("expected 1 rewardClaimed log, got %d", rewardClaimedCount)
		}
	})

	// Step 26: Verify user profile final state
	t.Run("26_Verify_user_profile_final_state", func(t *testing.T) {
		var user models.UserProfile
		db.Where("id = ?", userID).First(&user)

		if user.TotalCompletedQuests != 1 {
			t.Errorf("expected total_completed_quests 1, got %d", user.TotalCompletedQuests)
		}
		if user.TotalExp < 10 {
			t.Errorf("expected total_exp >= 10, got %d", user.TotalExp)
		}
		if user.RewardPoints >= 100 {
			t.Errorf("expected reward_points < 100 (after claim), got %d", user.RewardPoints)
		}
		if !user.HasCompletedOnboarding {
			t.Error("expected has_completed_onboarding = true")
		}
	})
}
