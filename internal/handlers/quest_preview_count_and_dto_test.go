package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/quest_generation"
)

func TestQuestPreviewHandler_CountAndDTO(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("AI preview with preview_limit=8 allows response with 1 quest", func(t *testing.T) {
		// Mock AI returns 1 quest
		mockAIClient := &ai.MockClient{
			ResponseText: generateQuestsJSON(1),
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now().AddDate(0, 0, 1),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   10,
				EnabledCategories: []string{"water", "learning", "sleep", "movement"},
				Rules: []quest_generation.QuestRuleContext{
					{Type: "water", Enabled: true},
					{Type: "learning", Enabled: true},
					{Type: "sleep", Enabled: true},
					{Type: "movement", Enabled: true},
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
			"preview_limit": 8, // Expect 8 quests max
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		data := response["data"].(map[string]interface{})
		generatedCount := int(data["generated_count"].(float64))
		if generatedCount != 1 {
			t.Errorf("expected generated_count 1, got %d", generatedCount)
		}
	})

	t.Run("AI preview with exact count passes and checks DTO fields", func(t *testing.T) {
		// Mock AI returns 8 quests
		mockAIClient := &ai.MockClient{
			ResponseText: generateQuestsJSON(8),
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now().AddDate(0, 0, 1),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   10,
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
			"preview_limit": 8, // Expect 8 quests
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		data := response["data"].(map[string]interface{})
		generatedCount := int(data["generated_count"].(float64))
		if generatedCount != 4 {
			t.Errorf("expected generated_count 4, got %d", generatedCount)
		}

		quests := data["quests"].([]interface{})
		if len(quests) != 4 {
			t.Fatalf("expected 4 quest items, got %d", len(quests))
		}

		// Check all quest items for absence of user_id, created_at, updated_at
		// and presence of non-zero id UUID
		for i, q := range quests {
			questMap := q.(map[string]interface{})

			// Check user_id is absent
			if _, hasUserID := questMap["user_id"]; hasUserID {
				t.Errorf("quest[%d] must not contain 'user_id' field", i)
			}

			// Check created_at is absent
			if _, hasCreatedAt := questMap["created_at"]; hasCreatedAt {
				t.Errorf("quest[%d] must not contain 'created_at' field", i)
			}

			// Check updated_at is absent
			if _, hasUpdatedAt := questMap["updated_at"]; hasUpdatedAt {
				t.Errorf("quest[%d] must not contain 'updated_at' field", i)
			}

			// Check id is present and is not zero UUID
			idStr, hasID := questMap["id"].(string)
			if !hasID {
				t.Errorf("quest[%d] must contain 'id' field", i)
			} else if idStr == "00000000-0000-0000-0000-000000000000" || idStr == "" {
				t.Errorf("quest[%d] must contain non-zero 'id' UUID, got '%s'", i, idStr)
			}
		}
	})

	t.Run("AI preview returns 422 before AI call if preview_limit exceeds capacity", func(t *testing.T) {
		maxPerDay := 2
		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now(),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   10,
				EnabledCategories: []string{"water"},
				Rules: []quest_generation.QuestRuleContext{
					{
						Type:      "water",
						Enabled:   true,
						MaxPerDay: &maxPerDay, // Usable capacity is 2
					},
				},
			},
		}

		mockAIClient := &ai.MockClient{}
		handler := NewQuestPreviewHandler(mockBuilder, nil, mockAIClient, nil, true)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		userID := uuid.New()
		c.Set("currentUserID", userID)

		reqBody := map[string]interface{}{
			"mode":          "ai",
			"preview_limit": 8, // Request 8, capacity is 2 -> Fail!
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422 (Unprocessable Entity), got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		expectedErrorMsg := "AI preview cannot satisfy preview_limit with current enabled rules and time constraints."
		msg := response["message"].(string)
		if msg != expectedErrorMsg {
			t.Errorf("expected error message '%s', got '%s'", expectedErrorMsg, msg)
		}
	})
}
