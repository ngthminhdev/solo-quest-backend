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

func setupQuestActionHandlerRouter(t *testing.T, db *gorm.DB) (*gin.Engine, uuid.UUID) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	questService := services.NewQuestService(db)
	questActionService := services.NewQuestActionService(db)

	questHandler := handlers.NewQuestHandler(questService)
	questActionHandler := handlers.NewQuestActionHandler(questActionService)

	userID := testutils.BootstrapTestUser(t, db)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		quests := protected.Group("/api/quests")
		{
			quests.GET("", questHandler.GetQuests)
			quests.POST("/:id/start", questActionHandler.StartQuest)
			quests.POST("/:id/complete", questActionHandler.CompleteQuest)
			quests.POST("/:id/skip", questActionHandler.SkipQuest)
			quests.POST("/:id/snooze", questActionHandler.SnoozeQuest)
		}
	}

	return r, userID
}

func TestStartQuestReturns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestActionHandlerRouter(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	questResp, ok := unwrapData(t, resp)["quest"].(map[string]interface{})
	if !ok {
		t.Fatal("expected quest in response")
	}

	if questResp["status"] != "active" {
		t.Errorf("expected status 'active', got '%s'", questResp["status"])
	}
}

func TestCompleteQuestReturns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestActionHandlerRouter(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusActive)

	body, _ := json.Marshal(map[string]string{"note": "Done!"})
	req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/complete", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	questResp, ok := unwrapData(t, resp)["quest"].(map[string]interface{})
	if !ok {
		t.Fatal("expected quest in response")
	}

	if questResp["status"] != "completed" {
		t.Errorf("expected status 'completed', got '%s'", questResp["status"])
	}
}

func TestCompleteQuestTwiceReturns409(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestActionHandlerRouter(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusActive)

	body, _ := json.Marshal(map[string]string{"note": "First"})
	req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/complete", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first complete failed: %d", w.Code)
	}

	body, _ = json.Marshal(map[string]string{"note": "Second"})
	req, _ = http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/complete", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", w.Code)
	}
}

func TestSnoozeQuestInvalidMinutesReturns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestActionHandlerRouter(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusPending)

	body, _ := json.Marshal(map[string]int{"minutes": 7})
	req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/snooze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestSkipCompletedQuestReturns409(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupQuestActionHandlerRouter(t, db)
	quest := testutils.CreateTestQuest(t, db, userID, models.QuestStatusCompleted)

	body, _ := json.Marshal(map[string]string{"reason": "Test"})
	req, _ := http.NewRequest("POST", "/api/quests/"+quest.ID.String()+"/skip", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", w.Code)
	}
}

func TestUnknownQuestIdReturns404(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupQuestActionHandlerRouter(t, db)

	unknownID := uuid.New()
	req, _ := http.NewRequest("POST", "/api/quests/"+unknownID.String()+"/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}
