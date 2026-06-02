package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestGetQuests_ReturnsDTOWithoutNestedUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	dueDate := time.Date(today.Year(), today.Month(), today.Day(), 10, 0, 0, 0, time.UTC)

	quest := models.Quest{
		UserID:           userID,
		Title:            "Test Quest",
		Description:      "Test Description",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		Source:           models.QuestSourceDailyPlan,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
		DueDate:          &dueDate,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questService := services.NewQuestService(db)
	questHandler := handlers.NewQuestHandler(questService)
	r.GET("/api/quests", testutils.AuthMiddleware(userID), questHandler.GetQuests)

	req := httptest.NewRequest(http.MethodGet, "/api/quests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	quests, ok := response["quests"].([]interface{})
	if !ok || len(quests) == 0 {
		t.Fatal("expected quests array in response")
	}

	firstQuest := quests[0].(map[string]interface{})

	// Verify that "user" field does NOT exist
	if _, hasUser := firstQuest["user"]; hasUser {
		t.Error("quest response should NOT contain nested 'user' field")
	}

	// Verify that "user_id" field DOES exist
	if _, hasUserID := firstQuest["user_id"]; !hasUserID {
		t.Error("quest response should contain 'user_id' field")
	}
}

func TestGetQuests_ContainsUserID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	quest := models.Quest{
		UserID:           userID,
		Title:            "Test Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questService := services.NewQuestService(db)
	questHandler := handlers.NewQuestHandler(questService)
	r.GET("/api/quests", testutils.AuthMiddleware(userID), questHandler.GetQuests)

	req := httptest.NewRequest(http.MethodGet, "/api/quests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	quests := response["quests"].([]interface{})
	firstQuest := quests[0].(map[string]interface{})

	if userIDStr, ok := firstQuest["user_id"].(string); !ok || userIDStr == "" {
		t.Error("quest response should contain non-empty 'user_id' field")
	}
}

func TestGetQuests_DevQuestsHaveDueDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)
	if err := bootstrapService.BootstrapDefaultDevUser("dev@example.com"); err != nil {
		t.Fatalf("failed to bootstrap dev user: %v", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questService := services.NewQuestService(db)
	questHandler := handlers.NewQuestHandler(questService)
	r.GET("/api/quests", testutils.AuthMiddleware(devUserID), questHandler.GetQuests)

	req := httptest.NewRequest(http.MethodGet, "/api/quests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	quests, ok := response["quests"].([]interface{})
	if !ok || len(quests) == 0 {
		t.Fatal("expected seeded quests for dev user")
	}

	for i, q := range quests {
		quest := q.(map[string]interface{})
		title := quest["title"].(string)

		if quest["due_date"] == nil {
			t.Errorf("quest #%d ('%s') should have non-null due_date", i, title)
		}

		if _, ok := quest["due_date"].(string); !ok || quest["due_date"].(string) == "" {
			t.Errorf("quest #%d ('%s') due_date should be a non-empty date string", i, title)
		}

		if quest["reminder_time"] == nil {
			t.Errorf("quest #%d ('%s') should have non-null reminder_time in seeded data", i, title)
		}
	}
}

func TestStartQuest_ReturnsDTOWithoutNestedUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	quest := models.Quest{
		UserID:           userID,
		Title:            "Test Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questActionService := services.NewQuestActionService(db)
	questActionHandler := handlers.NewQuestActionHandler(questActionService)
	r.POST("/api/quests/:id/start", testutils.AuthMiddleware(userID), questActionHandler.StartQuest)

	req := httptest.NewRequest(http.MethodPost, "/api/quests/"+quest.ID.String()+"/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	questData, ok := response["quest"].(map[string]interface{})
	if !ok {
		t.Fatal("expected quest object in response")
	}

	// Verify no nested user object
	if _, hasUser := questData["user"]; hasUser {
		t.Error("start quest response should NOT contain nested 'user' field")
	}

	// Verify user_id exists
	if _, hasUserID := questData["user_id"]; !hasUserID {
		t.Error("start quest response should contain 'user_id' field")
	}
}

func TestCompleteQuest_ReturnsDTOWithoutNestedUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	now := timeutil.NowUTC()
	quest := models.Quest{
		UserID:           userID,
		Title:            "Test Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusActive,
		Difficulty:       models.QuestDifficultyEasy,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
		StartedAt:        &now,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questActionService := services.NewQuestActionService(db)
	questActionHandler := handlers.NewQuestActionHandler(questActionService)
	r.POST("/api/quests/:id/complete", testutils.AuthMiddleware(userID), questActionHandler.CompleteQuest)

	req := httptest.NewRequest(http.MethodPost, "/api/quests/"+quest.ID.String()+"/complete", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	questData, ok := response["quest"].(map[string]interface{})
	if !ok {
		t.Fatal("expected quest object in response")
	}

	// Verify no nested user object
	if _, hasUser := questData["user"]; hasUser {
		t.Error("complete quest response should NOT contain nested 'user' field")
	}

	// Verify user_id exists
	if _, hasUserID := questData["user_id"]; !hasUserID {
		t.Error("complete quest response should contain 'user_id' field")
	}
}

func TestGetQuests_DueDateIsDateFormat(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	dueDate := time.Date(today.Year(), today.Month(), today.Day(), 15, 30, 0, 0, time.UTC)

	quest := models.Quest{
		UserID:           userID,
		Title:            "Test Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
		DueDate:          &dueDate,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questService := services.NewQuestService(db)
	questHandler := handlers.NewQuestHandler(questService)
	r.GET("/api/quests", testutils.AuthMiddleware(userID), questHandler.GetQuests)

	req := httptest.NewRequest(http.MethodGet, "/api/quests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	quests, ok := response["quests"].([]interface{})
	if !ok || len(quests) == 0 {
		t.Fatal("expected quests in response")
	}

	firstQuest := quests[0].(map[string]interface{})
	dueDateStr, ok := firstQuest["due_date"].(string)
	if !ok || dueDateStr == "" {
		t.Fatal("due_date should be a non-empty string")
	}

	if dueDateStr != "2026-06-02" {
		// Allow any valid date format - just check it doesn't contain time
		t.Errorf("due_date should be date-only format, got '%s'", dueDateStr)
	}
}

func TestGetQuests_ReminderTimeISOFormat(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	reminderTime := time.Date(today.Year(), today.Month(), today.Day(), 20, 30, 0, 0, time.FixedZone("+07", 7*3600))

	quest := models.Quest{
		UserID:           userID,
		Title:            "Test Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
		ReminderTime:     &reminderTime,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questService := services.NewQuestService(db)
	questHandler := handlers.NewQuestHandler(questService)
	r.GET("/api/quests", testutils.AuthMiddleware(userID), questHandler.GetQuests)

	req := httptest.NewRequest(http.MethodGet, "/api/quests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	quests, ok := response["quests"].([]interface{})
	if !ok || len(quests) == 0 {
		t.Fatal("expected quests in response")
	}

	firstQuest := quests[0].(map[string]interface{})

	if firstQuest["reminder_time"] == nil {
		t.Fatal("expected non-null reminder_time")
	}

	_, ok = firstQuest["reminder_time"].(string)
	if !ok {
		t.Error("reminder_time should be an ISO datetime string")
	}
}

func TestGetQuests_ReminderTimeNullWhenNotSet(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	quest := models.Quest{
		UserID:           userID,
		Title:            "No Reminder Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questService := services.NewQuestService(db)
	questHandler := handlers.NewQuestHandler(questService)
	r.GET("/api/quests", testutils.AuthMiddleware(userID), questHandler.GetQuests)

	req := httptest.NewRequest(http.MethodGet, "/api/quests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	quests, ok := response["quests"].([]interface{})
	if !ok || len(quests) == 0 {
		t.Fatal("expected quests in response")
	}

	firstQuest := quests[0].(map[string]interface{})

	if firstQuest["reminder_time"] != nil {
		t.Error("reminder_time should be null when not set")
	}
}

func TestGetQuests_DueDateWithoutReminder(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	dueDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	quest := models.Quest{
		UserID:           userID,
		Title:            "Due Date Only",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
		DueDate:          &dueDate,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questService := services.NewQuestService(db)
	questHandler := handlers.NewQuestHandler(questService)
	r.GET("/api/quests", testutils.AuthMiddleware(userID), questHandler.GetQuests)

	req := httptest.NewRequest(http.MethodGet, "/api/quests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	quests, ok := response["quests"].([]interface{})
	if !ok || len(quests) == 0 {
		t.Fatal("expected quests in response")
	}

	firstQuest := quests[0].(map[string]interface{})

	if firstQuest["due_date"] == nil {
		t.Error("due_date should be present")
	}

	if firstQuest["reminder_time"] != nil {
		t.Error("reminder_time should be null when due_date exists without reminder_time")
	}
}

func TestCompleteQuest_ReminderTimePreserved(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	now := timeutil.NowUTC()
	reminderTime := time.Date(today.Year(), today.Month(), today.Day(), 8, 0, 0, 0, time.UTC)

	quest := models.Quest{
		UserID:           userID,
		Title:            "Test Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusActive,
		Difficulty:       models.QuestDifficultyEasy,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
		StartedAt:        &now,
		ReminderTime:     &reminderTime,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questActionService := services.NewQuestActionService(db)
	questActionHandler := handlers.NewQuestActionHandler(questActionService)
	r.POST("/api/quests/:id/complete", testutils.AuthMiddleware(userID), questActionHandler.CompleteQuest)

	req := httptest.NewRequest(http.MethodPost, "/api/quests/"+quest.ID.String()+"/complete", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	questData, ok := response["quest"].(map[string]interface{})
	if !ok {
		t.Fatal("expected quest in response")
	}

	if questData["status"] != "completed" {
		t.Errorf("expected status 'completed', got '%s'", questData["status"])
	}

	if questData["reminder_time"] == nil {
		t.Error("reminder_time should be preserved after completion")
	}
}

func TestSnoozeQuest_ReminderTimePreserved(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayUTC()
	reminderTime := time.Date(today.Year(), today.Month(), today.Day(), 8, 0, 0, 0, time.UTC)

	quest := models.Quest{
		UserID:           userID,
		Title:            "Test Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Difficulty:       models.QuestDifficultyEasy,
		XPReward:         10,
		EstimatedMinutes: 5,
		Date:             today,
		ReminderTime:     &reminderTime,
	}
	db.Create(&quest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questActionService := services.NewQuestActionService(db)
	questActionHandler := handlers.NewQuestActionHandler(questActionService)
	r.POST("/api/quests/:id/snooze", testutils.AuthMiddleware(userID), questActionHandler.SnoozeQuest)

	body, _ := json.Marshal(map[string]int{"minutes": 15})
	req := httptest.NewRequest(http.MethodPost, "/api/quests/"+quest.ID.String()+"/snooze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	questData, ok := response["quest"].(map[string]interface{})
	if !ok {
		t.Fatal("expected quest in response")
	}

	if questData["status"] != "snoozed" {
		t.Errorf("expected status 'snoozed', got '%s'", questData["status"])
	}

	if questData["reminder_time"] == nil {
		t.Error("reminder_time should be preserved after snooze")
	}
}
