package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/quest_generation"
)

func TestQuestPreviewHandler_PreviewLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing preview_limit defaults to daily_quest_count", func(t *testing.T) {
		mockAIClient := &ai.MockClient{
			ResponseText: generateQuestsJSON(8), // Generate 8 quests
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now().AddDate(0, 0, 1),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   8, // daily target is 8
				EnabledCategories: []string{"water", "learning", "sleep", "review", "movement"},
				Rules: []quest_generation.QuestRuleContext{
					{Type: "water", Enabled: true},
					{Type: "learning", Enabled: true},
					{Type: "sleep", Enabled: true},
					{Type: "review", Enabled: true},
					{Type: "movement", Enabled: true},
				},
			},
		}

		handler := NewQuestPreviewHandler(mockBuilder, nil, mockAIClient, nil, true)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		userID := uuid.New()
		c.Set("currentUserID", userID)

		reqBody := map[string]interface{}{
			"mode": "ai",
			// No preview_limit specified
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})

		generatedCount := int(data["generated_count"].(float64))
		if generatedCount != 4 {
			t.Errorf("expected 4 quests (type caps: movement=1, sleep=1, review=1, learning=1), got %d", generatedCount)
		}
	})

	t.Run("preview_limit is clamped to daily_quest_count", func(t *testing.T) {
		mockAIClient := &ai.MockClient{
			ResponseText: `{"quests": [
				{"type": "learning", "title": "Quest 1", "description": "Desc", "difficulty": "easy", "estimated_minutes": 5, "xp_reward": 5, "tags": ["learning"], "reason": "Test", "instruction": "Do it", "reminder_time": "10:00"},
				{"type": "movement", "title": "Quest 2", "description": "Desc", "difficulty": "easy", "estimated_minutes": 5, "xp_reward": 5, "tags": ["movement"], "reason": "Test", "instruction": "Do it", "reminder_time": "11:00"},
				{"type": "review", "title": "Quest 3", "description": "Desc", "difficulty": "easy", "estimated_minutes": 5, "xp_reward": 5, "tags": ["review"], "reason": "Test", "instruction": "Do it", "reminder_time": "12:00"}
			]}`,
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now().AddDate(0, 0, 1),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   3, // daily target is 3
				EnabledCategories: []string{"learning", "movement", "review"},
				Rules: []quest_generation.QuestRuleContext{
					{Type: "learning", Enabled: true},
					{Type: "movement", Enabled: true},
					{Type: "review", Enabled: true},
				},
				ActiveLearningPath: &quest_generation.ActiveLearningPathDetail{
					RoadmapTitle:     "Test Roadmap",
					CurrentStepTitle: "Test Step",
					Description:      "Test Desc",
				},
			},
		}

		handler := NewQuestPreviewHandler(mockBuilder, nil, mockAIClient, nil, true)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		userID := uuid.New()
		c.Set("currentUserID", userID)

		reqBody := map[string]interface{}{
			"mode":          "ai",
			"preview_limit": 10, // Request 10, but should be clamped to 3
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})

		generatedCount := int(data["generated_count"].(float64))
		if generatedCount != 3 {
			t.Errorf("expected 3 quests (clamped to daily_quest_count), got %d", generatedCount)
		}
	})

	t.Run("preview_limit is clamped to hard max 10", func(t *testing.T) {
		mockAIClient := &ai.MockClient{
			ResponseText: generateQuestsJSON(10),
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now().AddDate(0, 0, 1),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   15, // daily target is 15
				EnabledCategories: []string{"water", "learning", "sleep", "review", "movement"},
				Rules: []quest_generation.QuestRuleContext{
					{Type: "water", Enabled: true},
					{Type: "learning", Enabled: true},
					{Type: "sleep", Enabled: true},
					{Type: "review", Enabled: true},
					{Type: "movement", Enabled: true},
				},
			},
		}

		handler := NewQuestPreviewHandler(mockBuilder, nil, mockAIClient, nil, true)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		userID := uuid.New()
		c.Set("currentUserID", userID)

		reqBody := map[string]interface{}{
			"mode":          "ai",
			"preview_limit": 15, // Request 15, should be clamped to 10
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})

		generatedCount := int(data["generated_count"].(float64))
		if generatedCount != 4 {
			t.Errorf("expected 4 quests (type caps), got %d", generatedCount)
		}
	})

	t.Run("preview_limit < 1 returns 400", func(t *testing.T) {
		mockAIClient := &ai.MockClient{
			ResponseText: `{"quests": []}`,
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now(),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   5,
				EnabledCategories: []string{"water"},
			},
		}

		handler := NewQuestPreviewHandler(mockBuilder, nil, mockAIClient, nil, true)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		userID := uuid.New()
		c.Set("currentUserID", userID)

		reqBody := map[string]interface{}{
			"mode":          "ai",
			"preview_limit": 0, // Invalid
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

// Helper to generate quest JSON with N quests
func generateQuestsJSON(n int) string {
	quests := make([]string, n)
	types := []string{"sleep", "review", "movement", "learning", "movement", "movement", "review", "sleep"}
	for i := 0; i < n; i++ {
		t := types[i%len(types)]
		quests[i] = `{"type": "` + t + `", "title": "Quest ` + fmt.Sprintf("%d", i+1) + `", "description": "Desc", "difficulty": "easy", "estimated_minutes": 5, "xp_reward": 5, "tags": ["` + t + `"], "reason": "Test", "instruction": "Do it", "reminder_time": "10:00"}`
	}
	return `{"quests": [` + strings.Join(quests, ",") + `]}`
}
