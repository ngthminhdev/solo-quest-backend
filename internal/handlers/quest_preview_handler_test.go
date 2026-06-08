package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/quest_generation"
)

type mockContextBuilder struct {
	qctx *quest_generation.UserQuestContext
	err  error
}

func (m *mockContextBuilder) Build(ctx context.Context, userID uuid.UUID, localDate time.Time) (*quest_generation.UserQuestContext, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.qctx, nil
}

type mockRuleGenerator struct {
	quests []models.Quest
	err    error
}

func (m *mockRuleGenerator) GenerateDailyQuests(ctx context.Context, qctx *quest_generation.UserQuestContext) ([]models.Quest, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.quests, nil
}

func TestQuestPreviewHandler_GeneratePreview(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("generates AI preview successfully", func(t *testing.T) {
		mockAIClient := &ai.MockClient{
			ResponseText: `{
				"quests": [
					{
						"type": "learning",
						"title": "Học tập 20 phút",
						"description": "Dành một khoảng thời gian ngắn để học hoặc ôn lại nội dung quan trọng.",
						"difficulty": "normal",
						"estimated_minutes": 20,
						"xp_reward": 10,
						"tags": ["learning"],
						"reason": "Tích lũy kiến thức mỗi ngày",
						"instruction": "Dành 20 phút tập trung học bài, đọc tài liệu.",
						"reminder_time": "14:00"
					}
				]
			}`,
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now().AddDate(0, 0, 1),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   1,
				EnabledCategories: []string{"learning"},
				Rules: []quest_generation.QuestRuleContext{
					{
						Type:    "learning",
						Enabled: true,
						ActiveTimeRange: &quest_generation.TimeRangeContext{
							Start: "08:00",
							End:   "22:00",
						},
					},
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
			"date": "2026-06-06",
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
		if data["mode"] != "ai" {
			t.Errorf("expected mode 'ai', got %v", data["mode"])
		}

		if data["inserted"] != false {
			t.Error("expected inserted to be false")
		}

		generatedCount := data["generated_count"].(float64)
		if generatedCount != 1 {
			t.Errorf("expected generated_count 1, got %v", generatedCount)
		}
	})

	t.Run("generates rule_based preview successfully", func(t *testing.T) {
		mockRuleGen := &mockRuleGenerator{
			quests: []models.Quest{
				{
					Title:            "Uống nước",
					Description:      "Uống 1 ly nước",
					Type:             models.QuestTypeWater,
					Status:           models.QuestStatusPending,
					Difficulty:       models.QuestDifficultyEasy,
					Source:           models.QuestSourceConfigBased,
					XPReward:         5,
					EstimatedMinutes: 5,
				},
			},
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now(),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   3,
				EnabledCategories: []string{"water"},
				Rules: []quest_generation.QuestRuleContext{
					{
						Type:    "water",
						Enabled: true,
						ActiveTimeRange: &quest_generation.TimeRangeContext{
							Start: "06:00",
							End:   "22:00",
						},
					},
				},
			},
		}

		handler := NewQuestPreviewHandler(mockBuilder, mockRuleGen, nil, nil, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		userID := uuid.New()
		c.Set("currentUserID", userID)

		reqBody := map[string]interface{}{
			"mode": "rule_based",
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
		if data["mode"] != "rule_based" {
			t.Errorf("expected mode 'rule_based', got %v", data["mode"])
		}

		if data["inserted"] != false {
			t.Error("expected inserted to be false for rule_based mode")
		}
	})

	t.Run("returns error when AI is disabled for AI mode", func(t *testing.T) {
		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now(),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   3,
				EnabledCategories: []string{"water"},
			},
		}

		handler := NewQuestPreviewHandler(mockBuilder, nil, nil, nil, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		userID := uuid.New()
		c.Set("currentUserID", userID)

		reqBody := map[string]interface{}{
			"mode": "ai",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("returns error for invalid mode", func(t *testing.T) {
		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now(),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   3,
				EnabledCategories: []string{"water"},
			},
		}

		handler := NewQuestPreviewHandler(mockBuilder, nil, nil, nil, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		userID := uuid.New()
		c.Set("currentUserID", userID)

		reqBody := map[string]interface{}{
			"mode": "invalid_mode",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("returns error for invalid date format", func(t *testing.T) {
		mockBuilder := &mockContextBuilder{}
		handler := NewQuestPreviewHandler(mockBuilder, nil, nil, nil, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		userID := uuid.New()
		c.Set("currentUserID", userID)

		reqBody := map[string]interface{}{
			"mode": "rule_based",
			"date": "invalid-date",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("returns error when user not authenticated", func(t *testing.T) {
		mockBuilder := &mockContextBuilder{}
		handler := NewQuestPreviewHandler(mockBuilder, nil, nil, nil, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// No user_id set in context
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", nil)

		handler.GeneratePreview(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("defaults to AI mode when mode is not specified", func(t *testing.T) {
		mockAIClient := &ai.MockClient{
			ResponseText: `{
				"quests": [
					{
						"type": "sleep",
						"title": "Tắt màn hình",
						"description": "Chuẩn bị đi ngủ đúng giờ",
						"difficulty": "easy",
						"estimated_minutes": 5,
						"xp_reward": 5,
						"tags": ["sleep"],
						"reason": "Bảo vệ giấc ngủ",
						"instruction": "Tắt thiết bị điện tử",
						"reminder_time": "22:00"
					}
				]
			}`,
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now(),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   1,
				EnabledCategories: []string{"sleep"},
				Rules: []quest_generation.QuestRuleContext{
					{
						Type:    "sleep",
						Enabled: true,
						ActiveTimeRange: &quest_generation.TimeRangeContext{
							Start: "20:00",
							End:   "23:00",
						},
					},
				},
			},
		}

		handler := NewQuestPreviewHandler(mockBuilder, nil, mockAIClient, nil, true)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		userID := uuid.New()
		c.Set("currentUserID", userID)

		// Empty body - should default to AI mode
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader([]byte("{}")))
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
		if data["mode"] != "ai" {
			t.Errorf("expected default mode 'ai', got %v", data["mode"])
		}
	})

	t.Run("preview does not insert to database", func(t *testing.T) {
		// This test verifies the handler contract - the actual DB non-insertion
		// is guaranteed by not calling any DB insert methods in the handler
		mockAIClient := &ai.MockClient{
			ResponseText: `{
				"quests": [
					{
						"type": "learning",
						"title": "Học tập 20 phút",
						"description": "Dành một khoảng thời gian ngắn để học hoặc ôn lại nội dung quan trọng.",
						"difficulty": "easy",
						"estimated_minutes": 20,
						"xp_reward": 5,
						"tags": ["learning"],
						"reason": "Phát triển",
						"instruction": "Học bài",
						"reminder_time": "11:00"
					}
				]
			}`,
			Model:        "test-model",
			FinishReason: "stop",
		}

		mockBuilder := &mockContextBuilder{
			qctx: &quest_generation.UserQuestContext{
				UserID:            uuid.New(),
				LocalDate:         time.Now().AddDate(0, 0, 1),
				Timezone:          "Asia/Ho_Chi_Minh",
				DisplayName:       "Test User",
				DailyQuestCount:   1,
				EnabledCategories: []string{"learning"},
				Rules: []quest_generation.QuestRuleContext{
					{
						Type:    "learning",
						Enabled: true,
						ActiveTimeRange: &quest_generation.TimeRangeContext{
							Start: "08:00",
							End:   "18:00",
						},
					},
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
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-preview", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GeneratePreview(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})
		if data["inserted"] != false {
			t.Error("preview must not insert to database - inserted field must be false")
		}

		// Verify quests are returned
		quests := data["quests"].([]interface{})
		if len(quests) == 0 {
			t.Error("expected quests in preview response")
		}
	})
}
