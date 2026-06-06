package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services/quest_generation"
)

type mockQuestGenerationService struct {
	result *quest_generation.GenerateTodayResult
	err    error
}

func (m *mockQuestGenerationService) GenerateToday(
	ctx context.Context,
	userID uuid.UUID,
	req quest_generation.GenerateTodayRequest,
) (*quest_generation.GenerateTodayResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestQuestGenerationHandler_GenerateToday(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("unauthenticated request rejected", func(t *testing.T) {
		mockService := &mockQuestGenerationService{}
		handler := handlers.NewQuestGenerationHandler(mockService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-today", nil)

		handler.GenerateToday(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid date format returns 400", func(t *testing.T) {
		mockService := &mockQuestGenerationService{}
		handler := handlers.NewQuestGenerationHandler(mockService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("currentUserID", uuid.New())

		reqBody := map[string]interface{}{
			"date": "invalid-date",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-today", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GenerateToday(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("replace_pending_only=false returns 400", func(t *testing.T) {
		mockService := &mockQuestGenerationService{}
		handler := handlers.NewQuestGenerationHandler(mockService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("currentUserID", uuid.New())

		reqBody := map[string]interface{}{
			"replace_pending_only": false,
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-today", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GenerateToday(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("existing quests with force=false returns existing_returned=true", func(t *testing.T) {
		mockService := &mockQuestGenerationService{
			result: &quest_generation.GenerateTodayResult{
				Date:             "2026-06-06",
				Inserted:         false,
				ExistingReturned: true,
				Source:           "existing",
				GeneratedCount:   0,
				PreservedCount:   2,
				Quests: []models.Quest{
					{Title: "Quest 1"},
					{Title: "Quest 2"},
				},
			},
		}
		handler := handlers.NewQuestGenerationHandler(mockService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("currentUserID", uuid.New())

		reqBody := map[string]interface{}{
			"date":     "2026-06-06",
			"force":    false,
			"prefer_ai": true,
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-today", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GenerateToday(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal("failed to unmarshal response:", err)
		}

		if resp["message"] != "Today's quests already exist" {
			t.Errorf("expected message \"Today's quests already exist\", got %v", resp["message"])
		}

		data := resp["data"].(map[string]interface{})
		if data["existing_returned"] != true {
			t.Error("expected existing_returned to be true")
		}
		if data["inserted"] != false {
			t.Error("expected inserted to be false")
		}
		if len(data["quests"].([]interface{})) != 2 {
			t.Errorf("expected 2 quests, got %d", len(data["quests"].([]interface{})))
		}
	})

	t.Run("AI fallback response contains fallback_used=true and ai_error_type", func(t *testing.T) {
		mockService := &mockQuestGenerationService{
			result: &quest_generation.GenerateTodayResult{
				Date:             "2026-06-06",
				Inserted:         true,
				ExistingReturned: false,
				Source:           "rule_based",
				FallbackUsed:     true,
				AIErrorType:      "timeout",
				GeneratedCount:   1,
				PreservedCount:   0,
				Quests: []models.Quest{
					{Title: "Fallback Quest 1"},
				},
			},
		}
		handler := handlers.NewQuestGenerationHandler(mockService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("currentUserID", uuid.New())

		reqBody := map[string]interface{}{
			"date":     "2026-06-06",
			"prefer_ai": true,
		}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-today", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GenerateToday(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal("failed to unmarshal response:", err)
		}

		if resp["message"] != "Today's quests generated with fallback" {
			t.Errorf("expected message \"Today's quests generated with fallback\", got %v", resp["message"])
		}

		data := resp["data"].(map[string]interface{})
		if data["fallback_used"] != true {
			t.Error("expected fallback_used to be true")
		}
		if data["source"] != "rule_based" {
			t.Errorf("expected source to be 'rule_based', got %s", data["source"])
		}
		if data["ai_error_type"] != "timeout" {
			t.Errorf("expected ai_error_type to be 'timeout', got %v", data["ai_error_type"])
		}
	})

	t.Run("returns 500 when service fails", func(t *testing.T) {
		mockService := &mockQuestGenerationService{
			err: errors.New("something went wrong internally"),
		}
		handler := handlers.NewQuestGenerationHandler(mockService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("currentUserID", uuid.New())

		reqBody := map[string]interface{}{}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/api/quests/generate-today", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GenerateToday(c)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
		}
	})
}
