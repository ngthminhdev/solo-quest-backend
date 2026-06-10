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

	today := timeutil.TodayVN()
	dueDate := time.Date(today.Year(), today.Month(), today.Day(), 10, 0, 0, 0, timeutil.LocationVN)

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

	quests, ok := unwrapData(t, response)["quests"].([]interface{})
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

	today := timeutil.TodayVN()
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

	quests := unwrapData(t, response)["quests"].([]interface{})
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

	devGenerator := services.NewDevQuestGenerator(db)
	if _, err := devGenerator.GenerateDevDailyQuests(devUserID, timeutil.TodayVN()); err != nil {
		t.Fatalf("failed to generate daily quests: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questService := services.NewQuestServiceWithDevGenerator(db, devGenerator)
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

	quests, ok := unwrapData(t, response)["quests"].([]interface{})
	if !ok || len(quests) == 0 {
		t.Fatal("expected seeded quests for dev user")
	}

	if len(quests) != 10 {
		t.Errorf("expected 10 dev daily quests, got %d", len(quests))
	}

	typesFound := make(map[string]bool)
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

		if src, ok := quest["source"].(string); !ok || src != string(models.QuestSourceDevRandomDailyPlan) {
			t.Errorf("quest #%d ('%s') source should be devRandomDailyPlan, got '%v'", i, title, quest["source"])
		}

		if qt, ok := quest["type"].(string); ok {
			typesFound[qt] = true
		}
	}

	if !typesFound["water"] && !typesFound["breakTime"] && !typesFound["movement"] {
		t.Error("expected at least one wellness quest type (water/breakTime/movement)")
	}
	if !typesFound["learning"] {
		t.Error("expected at least one learning quest type")
	}
	if !typesFound["review"] && !typesFound["reflection"] {
		t.Error("expected at least one review or reflection quest type")
	}
}

func TestStartQuest_ReturnsDTOWithoutNestedUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayVN()
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

	questData, ok := unwrapData(t, response)["quest"].(map[string]interface{})
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

	today := timeutil.TodayVN()
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

	questData, ok := unwrapData(t, response)["quest"].(map[string]interface{})
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

	today := timeutil.TodayVN()
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

	quests, ok := unwrapData(t, response)["quests"].([]interface{})
	if !ok || len(quests) == 0 {
		t.Fatal("expected quests in response")
	}

	firstQuest := quests[0].(map[string]interface{})
	dueDateStr, ok := firstQuest["due_date"].(string)
	if !ok || dueDateStr == "" {
		t.Fatal("due_date should be a non-empty string")
	}

	expectedDueDate := timeutil.FormatDateVN(today)
	if dueDateStr != expectedDueDate {
		t.Errorf("due_date should be date-only format '%s', got '%s'", expectedDueDate, dueDateStr)
	}

	if len(dueDateStr) != 10 {
		t.Errorf("due_date should be exactly 10 chars (YYYY-MM-DD), got '%s' (%d chars)", dueDateStr, len(dueDateStr))
	}

	if dueDateStr[4] != '-' || dueDateStr[7] != '-' {
		t.Errorf("due_date should match YYYY-MM-DD pattern, got '%s'", dueDateStr)
	}

	for _, c := range dueDateStr {
		if c == 'T' || c == 'Z' || c == '+' {
			t.Errorf("due_date must not contain time components, got '%s'", dueDateStr)
			break
		}
	}
}

func TestGetQuests_ReminderTimeISOFormat(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayVN()
	reminderTime := time.Date(today.Year(), today.Month(), today.Day(), 20, 30, 0, 0, timeutil.LocationVN)

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

	quests, ok := unwrapData(t, response)["quests"].([]interface{})
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

	today := timeutil.TodayVN()
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

	quests, ok := unwrapData(t, response)["quests"].([]interface{})
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

	today := timeutil.TodayVN()
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

	quests, ok := unwrapData(t, response)["quests"].([]interface{})
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

	today := timeutil.TodayVN()
	now := timeutil.NowUTC()
	reminderTime := time.Date(today.Year(), today.Month(), today.Day(), 8, 0, 0, 0, timeutil.LocationVN)

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

	questData, ok := unwrapData(t, response)["quest"].(map[string]interface{})
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

	today := timeutil.TodayVN()
	reminderTime := time.Date(today.Year(), today.Month(), today.Day(), 8, 0, 0, 0, timeutil.LocationVN)

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

	questData, ok := unwrapData(t, response)["quest"].(map[string]interface{})
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

func TestDevQuestGenerator_GeneratesTenQuestsForEmptyDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	generator := services.NewDevQuestGenerator(db)

	today := timeutil.TodayVN()
	quests, err := generator.GenerateDevDailyQuests(devUserID, today)
	if err != nil {
		t.Fatalf("failed to generate dev daily quests: %v", err)
	}

	if len(quests) != 10 {
		t.Errorf("expected 10 quests, got %d", len(quests))
	}

	for _, q := range quests {
		if q.Source != models.QuestSourceDevRandomDailyPlan {
			t.Errorf("expected source devRandomDailyPlan, got '%s'", q.Source)
		}
		if q.Status != models.QuestStatusPending {
			t.Errorf("expected status pending, got '%s'", q.Status)
		}
		if q.UserID != devUserID {
			t.Errorf("expected user_id %s, got %s", devUserID, q.UserID)
		}
	}
}

func TestDevQuestGenerator_IdempotentNoDuplicates(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	generator := services.NewDevQuestGenerator(db)

	today := timeutil.TodayVN()

	first, err := generator.GenerateDevDailyQuests(devUserID, today)
	if err != nil {
		t.Fatalf("first generation failed: %v", err)
	}
	if len(first) != 10 {
		t.Fatalf("expected 10 quests from first gen, got %d", len(first))
	}

	second, err := generator.GenerateDevDailyQuests(devUserID, today)
	if err != nil {
		t.Fatalf("second generation failed: %v", err)
	}
	if second != nil {
		t.Errorf("second generation should return nil (no-op), got %d quests", len(second))
	}

	var count int64
	db.Model(&models.Quest{}).Where("user_id = ?", devUserID).Count(&count)
	if count != 10 {
		t.Errorf("expected 10 total quests in DB, got %d", count)
	}
}

func TestConfigGenerator_AllUsersGetGeneration(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	// Create a regular (non-dev) user
	nonDevID := uuid.New()
	testutils.CreateTestUser(db, nonDevID, "normal@example.com")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	devGenerator := services.NewDevQuestGenerator(db)
	questService := services.NewQuestServiceWithDevGenerator(db, devGenerator)
	questHandler := handlers.NewQuestHandler(questService)
	r.GET("/api/quests", testutils.AuthMiddleware(nonDevID), questHandler.GetQuests)

	req := httptest.NewRequest(http.MethodGet, "/api/quests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	quests, ok := unwrapData(t, response)["quests"].([]interface{})
	if !ok {
		t.Fatal("expected quests array in response")
	}

	// With config-based generation, all users now get quests generated
	// Should generate quests based on user's settings (default 8)
	if len(quests) == 0 {
		t.Error("expected quests to be generated for non-dev user with config-based generator")
	}

	// Verify quests have source = configBased
	if len(quests) > 0 {
		firstQuest := quests[0].(map[string]interface{})
		source := firstQuest["source"].(string)
		if source != "configBased" {
			t.Errorf("expected source=configBased, got %s", source)
		}
	}
}

func TestDevQuestGenerator_ExistingQuestsNotOverwritten(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	today := timeutil.TodayVN()

	existing := models.Quest{
		UserID: devUserID,
		Title:  "My Custom Quest",
		Type:   models.QuestTypeLearning,
		Status: models.QuestStatusActive,
		Source: models.QuestSourceUser,
		Date:   today,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("failed to create existing quest: %v", err)
	}

	generator := services.NewDevQuestGenerator(db)
	result, err := generator.GenerateDevDailyQuests(devUserID, today)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	if result != nil {
		t.Errorf("generation should be no-op when quests exist, got %d quests", len(result))
	}

	var quests []models.Quest
	db.Where("user_id = ?", devUserID).Find(&quests)
	if len(quests) != 1 {
		t.Errorf("expected 1 quest (the original), got %d", len(quests))
	}
	if quests[0].Title != "My Custom Quest" {
		t.Errorf("expected original quest to be preserved, got '%s'", quests[0].Title)
	}
}

func TestDevQuestGenerator_QuestTypeDiversity(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	generator := services.NewDevQuestGenerator(db)

	today := timeutil.TodayVN()
	quests, err := generator.GenerateDevDailyQuests(devUserID, today)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	typesFound := make(map[models.QuestType]int)
	for _, q := range quests {
		typesFound[q.Type]++
	}

	wellnessCount := typesFound[models.QuestTypeWater] + typesFound[models.QuestTypeBreak] + typesFound[models.QuestTypeMovement]
	if wellnessCount < 2 {
		t.Errorf("expected at least 2 wellness quests (water/breakTime/movement), got %d", wellnessCount)
	}

	if typesFound[models.QuestTypeLearning] < 1 {
		t.Error("expected at least 1 learning quest")
	}

	reflectionCount := typesFound[models.QuestTypeReview] + typesFound[models.QuestTypeReflection]
	if reflectionCount < 1 {
		t.Error("expected at least 1 review/reflection quest")
	}
}

func TestDevQuestGenerator_NoDuplicateTitles(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	generator := services.NewDevQuestGenerator(db)

	today := timeutil.TodayVN()
	quests, err := generator.GenerateDevDailyQuests(devUserID, today)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	seen := make(map[string]bool)
	for _, q := range quests {
		if seen[q.Title] {
			t.Errorf("duplicate quest title: '%s'", q.Title)
		}
		seen[q.Title] = true
	}
}

func TestDevQuestGenerator_CountQuestsForDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	generator := services.NewDevQuestGenerator(db)

	today := timeutil.TodayVN()

	count, err := generator.CountQuestsForDate(devUserID, today)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 quests before generation, got %d", count)
	}

	_, err = generator.GenerateDevDailyQuests(devUserID, today)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	count, err = generator.CountQuestsForDate(devUserID, today)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 10 {
		t.Errorf("expected 10 quests after generation, got %d", count)
	}
}

func TestDevQuestGenerator_IsDevUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	generator := services.NewDevQuestGenerator(db)

	if !generator.IsDevUser(generator.GetDevUserID()) {
		t.Error("expected IsDevUser to return true for dev user")
	}

	if generator.IsDevUser(uuid.New()) {
		t.Error("expected IsDevUser to return false for random user")
	}
}

func TestGetQuests_Ordering(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := uuid.New()
	testutils.CreateTestUser(db, userID, "test@example.com")

	today := timeutil.TodayVN()

	t8 := time.Date(today.Year(), today.Month(), today.Day(), 8, 0, 0, 0, timeutil.LocationVN)
	t12 := time.Date(today.Year(), today.Month(), today.Day(), 12, 0, 0, 0, timeutil.LocationVN)
	t20 := time.Date(today.Year(), today.Month(), today.Day(), 20, 0, 0, 0, timeutil.LocationVN)

	now := time.Now()

	// Insert in non-ordered reminder_time to verify sorting does not depend on insertion order
	q20 := models.Quest{
		UserID:           userID,
		Title:            "Quest 20:00 (Completed)",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusCompleted,
		Date:             today,
		ReminderTime:     &t20,
		CreatedAt:        now.Add(-1 * time.Hour),
	}
	db.Create(&q20)

	q8 := models.Quest{
		UserID:           userID,
		Title:            "Quest 08:00 (Pending)",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Date:             today,
		ReminderTime:     &t8,
		CreatedAt:        now.Add(-2 * time.Hour),
	}
	db.Create(&q8)

	qNull := models.Quest{
		UserID:           userID,
		Title:            "Quest Null Reminder (Skipped)",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusSkipped,
		Date:             today,
		ReminderTime:     nil,
		CreatedAt:        now.Add(-3 * time.Hour),
	}
	db.Create(&qNull)

	q12a := models.Quest{
		UserID:           userID,
		Title:            "Quest 12:00 A (Snoozed)",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusSnoozed,
		Date:             today,
		ReminderTime:     &t12,
		CreatedAt:        now.Add(-5 * time.Minute),
	}
	db.Create(&q12a)

	q12b := models.Quest{
		UserID:           userID,
		Title:            "Quest 12:00 B (Pending)",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Date:             today,
		ReminderTime:     &t12,
		CreatedAt:        now.Add(-10 * time.Minute), // Older than 12a, so should appear before 12a
	}
	db.Create(&q12b)

	// Create a quest for a different user to verify user isolation
	otherUserID := uuid.New()
	testutils.CreateTestUser(db, otherUserID, "other@example.com")
	t7 := time.Date(today.Year(), today.Month(), today.Day(), 7, 0, 0, 0, timeutil.LocationVN)
	qOther := models.Quest{
		UserID:           otherUserID,
		Title:            "Other User Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Date:             today,
		ReminderTime:     &t7,
	}
	db.Create(&qOther)

	// Create a quest for the same user on a different date to verify date filter
	tomorrow := today.AddDate(0, 0, 1)
	qTomorrow := models.Quest{
		UserID:           userID,
		Title:            "Tomorrow Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Date:             tomorrow,
		ReminderTime:     &t8,
	}
	db.Create(&qTomorrow)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	questService := services.NewQuestService(db)
	questHandler := handlers.NewQuestHandler(questService)
	r.GET("/api/quests", testutils.AuthMiddleware(userID), questHandler.GetQuests)

	// 1. Check GET /api/quests without date parameter (defaults to today)
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

	quests, ok := unwrapData(t, response)["quests"].([]interface{})
	if !ok {
		t.Fatal("expected quests array in response")
	}

	// For today, we expect exactly 5 quests: q8, q12b (created older), q12a, q20, qNull
	if len(quests) != 5 {
		t.Fatalf("expected 5 quests for today, got %d", len(quests))
	}

	expectedTitles := []string{
		"Quest 08:00 (Pending)",
		"Quest 12:00 B (Pending)",
		"Quest 12:00 A (Snoozed)",
		"Quest 20:00 (Completed)",
		"Quest Null Reminder (Skipped)",
	}

	for i, expectedTitle := range expectedTitles {
		questObj := quests[i].(map[string]interface{})
		title := questObj["title"].(string)
		if title != expectedTitle {
			t.Errorf("at index %d: expected quest title %q, got %q", i, expectedTitle, title)
		}
	}

	// 2. Check GET /api/quests with date parameter for tomorrow
	reqTomorrow := httptest.NewRequest(http.MethodGet, "/api/quests?date="+timeutil.FormatDateVN(tomorrow), nil)
	wTomorrow := httptest.NewRecorder()
	r.ServeHTTP(wTomorrow, reqTomorrow)

	if wTomorrow.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", wTomorrow.Code, wTomorrow.Body.String())
	}

	var responseTomorrow map[string]interface{}
	if err := json.Unmarshal(wTomorrow.Body.Bytes(), &responseTomorrow); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	questsTomorrow, ok := unwrapData(t, responseTomorrow)["quests"].([]interface{})
	if !ok {
		t.Fatal("expected quests array in response")
	}

	if len(questsTomorrow) != 1 {
		t.Fatalf("expected 1 quest for tomorrow, got %d", len(questsTomorrow))
	}

	tomorrowTitle := questsTomorrow[0].(map[string]interface{})["title"].(string)
	if tomorrowTitle != "Tomorrow Quest" {
		t.Errorf("expected quest title 'Tomorrow Quest', got %q", tomorrowTitle)
	}
}

