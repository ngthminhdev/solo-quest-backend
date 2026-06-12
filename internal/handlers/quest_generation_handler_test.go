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

	startResult *quest_generation.StartResult
	startErr    error

	status    *quest_generation.JobStatus
	statusErr error

	generateTodayCalled bool
	startCalled         bool
	lastGenerateUserID  uuid.UUID
	lastStartUserID     uuid.UUID
	lastStatusUserID    uuid.UUID
}

func (m *mockQuestGenerationService) GenerateToday(
	ctx context.Context,
	userID uuid.UUID,
	req quest_generation.GenerateTodayRequest,
) (*quest_generation.GenerateTodayResult, error) {
	m.generateTodayCalled = true
	m.lastGenerateUserID = userID
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockQuestGenerationService) StartTodayGeneration(
	ctx context.Context,
	userID uuid.UUID,
	req quest_generation.GenerateTodayRequest,
) (*quest_generation.StartResult, error) {
	m.startCalled = true
	m.lastStartUserID = userID
	if m.startErr != nil {
		return nil, m.startErr
	}
	return m.startResult, nil
}

func (m *mockQuestGenerationService) GetJobStatus(
	ctx context.Context,
	userID uuid.UUID,
	date *string,
) (*quest_generation.JobStatus, error) {
	m.lastStatusUserID = userID
	if m.statusErr != nil {
		return nil, m.statusErr
	}
	return m.status, nil
}

func postGenerateToday(t *testing.T, handler *handlers.QuestGenerationHandler, body map[string]interface{}, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	if !authed {
		return postGenerateTodayAs(t, handler, body, uuid.Nil, "")
	}
	return postGenerateTodayAs(t, handler, body, uuid.New(), "")
}

func postGenerateTodayAs(t *testing.T, handler *handlers.QuestGenerationHandler, body map[string]interface{}, userID uuid.UUID, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if userID != uuid.Nil {
		c.Set("currentUserID", userID)
	}
	bodyBytes, _ := json.Marshal(body)
	c.Request = httptest.NewRequest("POST", "/api/quests/generate-today", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		c.Request.Header.Set("Authorization", authHeader)
	}
	handler.GenerateToday(c)
	return w
}

func TestQuestGenerationHandler_GenerateToday(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("unauthenticated request rejected", func(t *testing.T) {
		handler := handlers.NewQuestGenerationHandler(&mockQuestGenerationService{})
		w := postGenerateToday(t, handler, map[string]interface{}{}, false)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid date format returns 400", func(t *testing.T) {
		handler := handlers.NewQuestGenerationHandler(&mockQuestGenerationService{})
		w := postGenerateToday(t, handler, map[string]interface{}{"date": "invalid-date"}, true)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("replace_pending_only=false returns 400", func(t *testing.T) {
		handler := handlers.NewQuestGenerationHandler(&mockQuestGenerationService{})
		w := postGenerateToday(t, handler, map[string]interface{}{"replace_pending_only": false}, true)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("prefer_ai=false runs synchronously and returns quests with real IDs", func(t *testing.T) {
		realID := uuid.New()
		mockService := &mockQuestGenerationService{
			result: &quest_generation.GenerateTodayResult{
				Date:           "2026-06-06",
				Inserted:       true,
				Source:         "rule_based",
				GeneratedCount: 1,
				Quests:         []models.Quest{{ID: realID, Title: "Rule Quest"}},
			},
		}
		handler := handlers.NewQuestGenerationHandler(mockService)
		w := postGenerateToday(t, handler, map[string]interface{}{"prefer_ai": false}, true)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if !mockService.generateTodayCalled {
			t.Error("expected GenerateToday (sync) to be called")
		}
		if mockService.startCalled {
			t.Error("StartTodayGeneration should NOT be called for prefer_ai=false")
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		quests := data["quests"].([]interface{})
		q0 := quests[0].(map[string]interface{})
		if q0["id"] != realID.String() {
			t.Errorf("expected real quest id %s, got %v", realID, q0["id"])
		}
	})

	t.Run("prefer_ai=true starts background job and returns 202", func(t *testing.T) {
		mockService := &mockQuestGenerationService{
			startResult: &quest_generation.StartResult{
				Job: &quest_generation.JobInfo{
					Date:             "2026-06-09",
					Status:           "generating",
					JobID:            "job-123",
					EstimatedSeconds: 15,
				},
			},
		}
		handler := handlers.NewQuestGenerationHandler(mockService)
		w := postGenerateToday(t, handler, map[string]interface{}{"prefer_ai": true}, true)

		if w.Code != http.StatusAccepted {
			t.Fatalf("expected status 202, got %d: %s", w.Code, w.Body.String())
		}
		if !mockService.startCalled {
			t.Error("expected StartTodayGeneration to be called")
		}
		if mockService.generateTodayCalled {
			t.Error("GenerateToday (sync) should NOT be called for prefer_ai=true")
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["code"].(float64) != 202 {
			t.Errorf("expected envelope code 202, got %v", resp["code"])
		}
		data := resp["data"].(map[string]interface{})
		if data["status"] != "generating" {
			t.Errorf("expected status generating, got %v", data["status"])
		}
		if data["job_id"] != "job-123" {
			t.Errorf("expected job_id job-123, got %v", data["job_id"])
		}
	})

	t.Run("bearer authenticated user starts job for that user not seed dev user", func(t *testing.T) {
		googleUserID := uuid.New()
		seedDevUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockService := &mockQuestGenerationService{
			startResult: &quest_generation.StartResult{
				Job: &quest_generation.JobInfo{
					Date:             "2026-06-09",
					Status:           "generating",
					JobID:            "job-google",
					EstimatedSeconds: 15,
				},
			},
		}
		handler := handlers.NewQuestGenerationHandler(mockService)
		w := postGenerateTodayAs(t, handler, map[string]interface{}{"prefer_ai": true}, googleUserID, "Bearer test-token")
		if w.Code != http.StatusAccepted {
			t.Fatalf("expected status 202, got %d: %s", w.Code, w.Body.String())
		}
		if mockService.lastStartUserID != googleUserID {
			t.Fatalf("expected job user %s, got %s", googleUserID, mockService.lastStartUserID)
		}
		if mockService.lastStartUserID == seedDevUserID {
			t.Fatal("generate-today created a job for the seed dev user")
		}
	})

	t.Run("prefer_ai=true with existing quests returns 200 existing", func(t *testing.T) {
		mockService := &mockQuestGenerationService{
			startResult: &quest_generation.StartResult{
				Existing: &quest_generation.GenerateTodayResult{
					Date:             "2026-06-09",
					ExistingReturned: true,
					Source:           "existing",
					PreservedCount:   2,
					Quests:           []models.Quest{{Title: "Q1"}, {Title: "Q2"}},
				},
			},
		}
		handler := handlers.NewQuestGenerationHandler(mockService)
		w := postGenerateToday(t, handler, map[string]interface{}{"prefer_ai": true, "force": false}, true)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["message"] != "Today's quests already exist" {
			t.Errorf("unexpected message: %v", resp["message"])
		}
		data := resp["data"].(map[string]interface{})
		if data["existing_returned"] != true {
			t.Error("expected existing_returned true")
		}
	})

	t.Run("returns 500 when async start fails", func(t *testing.T) {
		mockService := &mockQuestGenerationService{startErr: errors.New("boom")}
		handler := handlers.NewQuestGenerationHandler(mockService)
		w := postGenerateToday(t, handler, map[string]interface{}{"prefer_ai": true}, true)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("returns 500 when sync service fails", func(t *testing.T) {
		mockService := &mockQuestGenerationService{err: errors.New("boom")}
		handler := handlers.NewQuestGenerationHandler(mockService)
		w := postGenerateToday(t, handler, map[string]interface{}{"prefer_ai": false}, true)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func getStatus(t *testing.T, handler *handlers.QuestGenerationHandler, query string, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if authed {
		c.Set("currentUserID", uuid.New())
	}
	c.Request = httptest.NewRequest("GET", "/api/quests/generate-today/status"+query, nil)
	handler.GetTodayGenerationStatus(c)
	return w
}

func TestQuestGenerationHandler_GetTodayGenerationStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("unauthenticated rejected", func(t *testing.T) {
		handler := handlers.NewQuestGenerationHandler(&mockQuestGenerationService{})
		w := getStatus(t, handler, "", false)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("invalid date returns 400", func(t *testing.T) {
		handler := handlers.NewQuestGenerationHandler(&mockQuestGenerationService{})
		w := getStatus(t, handler, "?date=nope", true)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("not_started status", func(t *testing.T) {
		handler := handlers.NewQuestGenerationHandler(&mockQuestGenerationService{
			status: &quest_generation.JobStatus{Date: "2026-06-09", Status: "not_started", QuestCount: 0},
		})
		w := getStatus(t, handler, "?date=2026-06-09", true)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		if data["status"] != "not_started" {
			t.Errorf("expected not_started, got %v", data["status"])
		}
	})

	t.Run("completed status carries source and quest_count", func(t *testing.T) {
		jobID := "job-9"
		src := "ai"
		handler := handlers.NewQuestGenerationHandler(&mockQuestGenerationService{
			status: &quest_generation.JobStatus{
				Date: "2026-06-09", Status: "completed", JobID: &jobID,
				QuestCount: 5, Source: &src, FallbackUsed: false,
			},
		})
		w := getStatus(t, handler, "?date=2026-06-09", true)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		if data["status"] != "completed" {
			t.Errorf("expected completed, got %v", data["status"])
		}
		if data["source"] != "ai" {
			t.Errorf("expected source ai, got %v", data["source"])
		}
		if data["quest_count"].(float64) != 5 {
			t.Errorf("expected quest_count 5, got %v", data["quest_count"])
		}
	})

	t.Run("failed status carries error_message", func(t *testing.T) {
		jobID := "job-10"
		msg := "provider exploded"
		handler := handlers.NewQuestGenerationHandler(&mockQuestGenerationService{
			status: &quest_generation.JobStatus{
				Date: "2026-06-09", Status: "failed", JobID: &jobID,
				FallbackUsed: true, ErrorMessage: &msg,
			},
		})
		w := getStatus(t, handler, "?date=2026-06-09", true)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		if data["status"] != "failed" {
			t.Errorf("expected failed, got %v", data["status"])
		}
		if data["error_message"] != "provider exploded" {
			t.Errorf("expected error_message, got %v", data["error_message"])
		}
	})
}
