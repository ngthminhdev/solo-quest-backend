package handlers

import (
	"bytes"
	"encoding/json"
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
				LocalDate:         time.Now(),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   8, // daily target is 8
				EnabledCategories: []string{"water"},
				Rules: []quest_generation.QuestRuleContext{
					{Type: "water", Enabled: true},
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
		if generatedCount != 8 {
			t.Errorf("expected 8 quests (default to daily_quest_count), got %d", generatedCount)
		}
	})

	t.Run("preview_limit is clamped to daily_quest_count", func(t *testing.T) {
		mockAIClient := &ai.MockClient{
			ResponseText: `{"quests": [
				{"type": "water", "title": "Quest 1", "description": "Desc", "difficulty": "easy", "estimated_minutes": 5, "xp_reward": 5, "tags": ["water"], "reason": "Test", "instruction": "Do it", "reminder_time": "10:00"},
				{"type": "water", "title": "Quest 2", "description": "Desc", "difficulty": "easy", "estimated_minutes": 5, "xp_reward": 5, "tags": ["water"], "reason": "Test", "instruction": "Do it", "reminder_time": "11:00"},
				{"type": "water", "title": "Quest 3", "description": "Desc", "difficulty": "easy", "estimated_minutes": 5, "xp_reward": 5, "tags": ["water"], "reason": "Test", "instruction": "Do it", "reminder_time": "12:00"}
			]}`,
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now(),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   3, // daily target is 3
				EnabledCategories: []string{"water"},
				Rules: []quest_generation.QuestRuleContext{
					{Type: "water", Enabled: true},
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
				LocalDate:         time.Now(),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   15, // daily target is 15
				EnabledCategories: []string{"water"},
				Rules: []quest_generation.QuestRuleContext{
					{Type: "water", Enabled: true},
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
		if generatedCount != 10 {
			t.Errorf("expected 10 quests (hard max), got %d", generatedCount)
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
	for i := 0; i < n; i++ {
		quests[i] = `{"type": "water", "title": "Quest ` + string(rune('1'+i)) + `", "description": "Desc", "difficulty": "easy", "estimated_minutes": 5, "xp_reward": 5, "tags": ["water"], "reason": "Test", "instruction": "Do it", "reminder_time": "10:00"}`
	}
	return `{"quests": [` + strings.Join(quests, ",") + `]}`
}
