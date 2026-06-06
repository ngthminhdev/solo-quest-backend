package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupWeeklySummaryRouter(t *testing.T, db *gorm.DB) (*gin.Engine, uuid.UUID) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	weeklySummaryService := services.NewWeeklySummaryService(db)
	weeklySummaryHandler := handlers.NewWeeklySummaryHandler(weeklySummaryService)

	userID := testutils.BootstrapTestUser(t, db)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		ws := protected.Group("/api/weekly-summary")
		{
			ws.GET("", weeklySummaryHandler.GetWeeklySummary)
		}
	}

	return r, userID
}

func createQuestForDate(t *testing.T, db *gorm.DB, userID uuid.UUID, date time.Time, questType models.QuestType, status models.QuestStatus, xp int) {
	t.Helper()

	quest := &models.Quest{
		UserID:    userID,
		Title:     "Test Quest",
		Type:      questType,
		Status:    status,
		XPReward:  xp,
		Date:      date,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(quest).Error; err != nil {
		t.Fatalf("failed to create quest: %v", err)
	}
}

func TestWeeklySummary_CurrentWeek_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	today := time.Now().In(timeutil.LocationVN)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	createQuestForDate(t, db, userID, todayDate, models.QuestTypeWater, models.QuestStatusCompleted, 10)

	req, _ := http.NewRequest("GET", "/api/weekly-summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if unwrapData(t, resp)["week_start"] == nil {
		t.Error("expected week_start in response")
	}
	if unwrapData(t, resp)["week_end"] == nil {
		t.Error("expected week_end in response")
	}
	if unwrapData(t, resp)["completed_quest_count"] == nil {
		t.Error("expected completed_quest_count in response")
	}
	if unwrapData(t, resp)["daily_breakdown"] == nil {
		t.Error("expected daily_breakdown in response")
	}
	if unwrapData(t, resp)["category_breakdown"] == nil {
		t.Error("expected category_breakdown in response")
	}
	if unwrapData(t, resp)["insights"] == nil {
		t.Error("expected insights in response")
	}
	if unwrapData(t, resp)["suggestions"] == nil {
		t.Error("expected suggestions in response")
	}
	if unwrapData(t, resp)["total_days"] == nil {
		t.Error("expected total_days in response")
	}
}

func TestWeeklySummary_SpecificWeek_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	monday, _ := timeutil.ParseDateVN("2026-06-01")
	createQuestForDate(t, db, userID, monday, models.QuestTypeLearning, models.QuestStatusCompleted, 15)

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=2026-06-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if unwrapData(t, resp)["week_start"] != "2026-06-01" {
		t.Errorf("expected week_start '2026-06-01', got '%v'", unwrapData(t, resp)["week_start"])
	}
	if unwrapData(t, resp)["week_end"] != "2026-06-07" {
		t.Errorf("expected week_end '2026-06-07', got '%v'", unwrapData(t, resp)["week_end"])
	}
}

func TestWeeklySummary_SpecificWeek_NormalizesToMonday(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupWeeklySummaryRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=2026-06-04", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if unwrapData(t, resp)["week_start"] != "2026-06-01" {
		t.Errorf("expected normalized week_start '2026-06-01', got '%v'", unwrapData(t, resp)["week_start"])
	}
}

func TestWeeklySummary_InvalidWeekStart_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupWeeklySummaryRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=bad-date", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWeeklySummary_DailyBreakdownExactly7Days(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupWeeklySummaryRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/weekly-summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	breakdown, ok := unwrapData(t, resp)["daily_breakdown"].([]interface{})
	if !ok {
		t.Fatal("daily_breakdown is not an array")
	}

	if len(breakdown) != 7 {
		t.Errorf("expected daily_breakdown length 7, got %d", len(breakdown))
	}
}

func TestWeeklySummary_CompletionRate_Range(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	today := time.Now().In(timeutil.LocationVN)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	createQuestForDate(t, db, userID, todayDate, models.QuestTypeWater, models.QuestStatusCompleted, 10)
	createQuestForDate(t, db, userID, todayDate, models.QuestTypeWater, models.QuestStatusSkipped, 0)

	req, _ := http.NewRequest("GET", "/api/weekly-summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	rate, ok := unwrapData(t, resp)["completion_rate"].(float64)
	if !ok {
		t.Fatal("completion_rate is not a float64")
	}

	if rate < 0 || rate > 1 {
		t.Errorf("completion_rate must be 0.0-1.0, got %f", rate)
	}

	if rate != 0.5 {
		t.Errorf("expected completion_rate 0.5, got %f", rate)
	}
}

func TestWeeklySummary_CategoryBreakdown_FEValues(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	monday, _ := timeutil.ParseDateVN("2026-06-01")
	createQuestForDate(t, db, userID, monday, models.QuestTypeWater, models.QuestStatusCompleted, 10)
	createQuestForDate(t, db, userID, monday, models.QuestTypeLearning, models.QuestStatusCompleted, 15)
	createQuestForDate(t, db, userID, monday, models.QuestTypeBreak, models.QuestStatusSkipped, 0)
	createQuestForDate(t, db, userID, monday, models.QuestTypeSleep, models.QuestStatusCompleted, 20)
	createQuestForDate(t, db, userID, monday, models.QuestTypeReflection, models.QuestStatusCompleted, 5)

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=2026-06-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	cb, ok := unwrapData(t, resp)["category_breakdown"].([]interface{})
	if !ok {
		t.Fatal("category_breakdown is not an array")
	}

	validCategories := map[string]bool{
		"water": true, "learning": true, "breakTime": true,
		"movement": true, "sleep": true, "fitness": true,
		"mindfulness": true, "review": true, "custom": true,
	}

	for _, item := range cb {
		cat := item.(map[string]interface{})
		category := cat["category"].(string)
		// Check that the category is one of the valid FE values or passes through
		if _, exists := validCategories[category]; !exists {
			t.Logf("warning: category '%s' not in expected FE QuestType.name list", category)
		}
	}

	// verify reflection -> mindfulness mapping
	foundMindfulness := false
	for _, item := range cb {
		cat := item.(map[string]interface{})
		if cat["category"].(string) == "mindfulness" {
			foundMindfulness = true
			break
		}
	}
	if !foundMindfulness {
		t.Error("expected 'mindfulness' category (mapped from 'reflection')")
	}
}

func TestWeeklySummary_ReviewedDays(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	monday, _ := timeutil.ParseDateVN("2026-06-01")
	tuesday := monday.AddDate(0, 0, 1)

	db.Create(&models.DailyReview{
		UserID:           userID,
		Date:             monday,
		Mood:             "good",
		EnergyLevel:      "high",
		Satisfaction:     4,
		TomorrowPriority: "learning",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	})
	db.Create(&models.DailyReview{
		UserID:           userID,
		Date:             tuesday,
		Mood:             "normal",
		EnergyLevel:      "medium",
		Satisfaction:     3,
		TomorrowPriority: "health",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	})

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=2026-06-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	reviewedDays, ok := unwrapData(t, resp)["reviewed_days"].(float64)
	if !ok {
		t.Fatal("reviewed_days is not in response")
	}

	if int(reviewedDays) != 2 {
		t.Errorf("expected reviewed_days 2, got %v", int(reviewedDays))
	}
}

func TestWeeklySummary_StreakDays_Computed(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	today := time.Now().In(timeutil.LocationVN)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	yesterday := todayDate.AddDate(0, 0, -1)

	createQuestForDate(t, db, userID, yesterday, models.QuestTypeWater, models.QuestStatusCompleted, 10)
	createQuestForDate(t, db, userID, todayDate, models.QuestTypeLearning, models.QuestStatusCompleted, 15)

	req, _ := http.NewRequest("GET", "/api/weekly-summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	streakDays, ok := unwrapData(t, resp)["streak_days"].(float64)
	if !ok {
		t.Fatal("streak_days is not in response")
	}

	if int(streakDays) < 1 {
		t.Errorf("expected streak_days >= 1, got %v", int(streakDays))
	}
}

func TestWeeklySummary_SuggestionsAndInsights_HaveContent(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	monday, _ := timeutil.ParseDateVN("2026-06-01")
	createQuestForDate(t, db, userID, monday, models.QuestTypeWater, models.QuestStatusCompleted, 10)
	createQuestForDate(t, db, userID, monday, models.QuestTypeLearning, models.QuestStatusSkipped, 0)
	createQuestForDate(t, db, userID, monday, models.QuestTypeLearning, models.QuestStatusSkipped, 0)

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=2026-06-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	insights, ok := unwrapData(t, resp)["insights"].([]interface{})
	if !ok {
		t.Fatal("insights is not an array")
	}
	if len(insights) == 0 {
		t.Error("expected non-empty insights")
	}

	suggestions, ok := unwrapData(t, resp)["suggestions"].([]interface{})
	if !ok {
		t.Fatal("suggestions is not an array")
	}
	if len(suggestions) == 0 {
		t.Error("expected non-empty suggestions")
	}

	for _, s := range suggestions {
		sug := s.(map[string]interface{})
		sugType, ok := sug["type"].(string)
		if !ok {
			t.Error("suggestion missing type field")
			continue
		}
		validTypes := map[string]bool{
			"schedule": true, "frequency": true, "duration": true, "general": true,
		}
		if !validTypes[sugType] {
			t.Errorf("invalid suggestion type: '%s'", sugType)
		}
	}
}

func TestWeeklySummary_EmptyWeek_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupWeeklySummaryRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=2026-06-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(unwrapData(t, resp)["completed_quest_count"].(float64)) != 0 {
		t.Error("expected completed_quest_count 0 for empty week")
	}
	if int(unwrapData(t, resp)["skipped_quest_count"].(float64)) != 0 {
		t.Error("expected skipped_quest_count 0 for empty week")
	}
	if unwrapData(t, resp)["completion_rate"].(float64) != 0.0 {
		t.Error("expected completion_rate 0.0 for empty week")
	}

	breakdown, _ := unwrapData(t, resp)["daily_breakdown"].([]interface{})
	if len(breakdown) != 7 {
		t.Errorf("expected daily_breakdown length 7, got %d", len(breakdown))
	}

	insights, _ := unwrapData(t, resp)["insights"].([]interface{})
	if len(insights) == 0 {
		t.Error("expected at least 1 insight even for empty week")
	}

	cb, _ := unwrapData(t, resp)["category_breakdown"].([]interface{})
	if len(cb) != 0 {
		t.Error("expected empty category_breakdown for empty week")
	}
}

func TestWeeklySummary_BestWeakestCategory_Computed(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	monday, _ := timeutil.ParseDateVN("2026-06-01")
	createQuestForDate(t, db, userID, monday, models.QuestTypeWater, models.QuestStatusCompleted, 10)
	createQuestForDate(t, db, userID, monday, models.QuestTypeWater, models.QuestStatusCompleted, 10)
	createQuestForDate(t, db, userID, monday, models.QuestTypeLearning, models.QuestStatusCompleted, 15)
	createQuestForDate(t, db, userID, monday, models.QuestTypeLearning, models.QuestStatusSkipped, 0)

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=2026-06-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	best, bestOk := unwrapData(t, resp)["best_category"].(string)
	weakest, weakestOk := unwrapData(t, resp)["weakest_category"].(string)

	if !bestOk || best == "" {
		t.Error("expected best_category to be set")
	}
	if !weakestOk || weakest == "" {
		t.Error("expected weakest_category to be set")
	}

	// water has rate 1.0, learning has rate 0.5
	if best != "water" {
		t.Errorf("expected best_category 'water', got '%s'", best)
	}
	if weakest != "learning" {
		t.Errorf("expected weakest_category 'learning', got '%s'", weakest)
	}
}

func TestWeeklySummary_AISummaryAndNextWeekFocus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	monday, _ := timeutil.ParseDateVN("2026-06-01")
	createQuestForDate(t, db, userID, monday, models.QuestTypeWater, models.QuestStatusCompleted, 10)

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=2026-06-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	aiSummary, ok := unwrapData(t, resp)["ai_summary"].(string)
	if !ok || aiSummary == "" {
		t.Error("expected non-empty ai_summary")
	}

	nextWeekFocus, ok := unwrapData(t, resp)["next_week_focus"].(string)
	if !ok || nextWeekFocus == "" {
		t.Error("expected non-empty next_week_focus")
	}
}

func TestWeeklySummary_Unauthorized_Returns401(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	weeklySummaryService := services.NewWeeklySummaryService(db)
	weeklySummaryHandler := handlers.NewWeeklySummaryHandler(weeklySummaryService)

	r.GET("/api/weekly-summary", weeklySummaryHandler.GetWeeklySummary)

	req, _ := http.NewRequest("GET", "/api/weekly-summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWeeklySummary_HighCompletionRate_Insights(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupWeeklySummaryRouter(t, db)

	monday, _ := timeutil.ParseDateVN("2026-06-01")
	for i := 0; i < 10; i++ {
		day := monday.AddDate(0, 0, i) // this will overflow past Sunday but quests only counted within week
		createQuestForDate(t, db, userID, day, models.QuestTypeWater, models.QuestStatusCompleted, 10)
	}

	req, _ := http.NewRequest("GET", "/api/weekly-summary?week_start=2026-06-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	insights, _ := unwrapData(t, resp)["insights"].([]interface{})
	hasDuyTri := false
	for _, ins := range insights {
		if s, ok := ins.(string); ok && contains(s, "duy trì") {
			hasDuyTri = true
			break
		}
	}
	if !hasDuyTri {
		t.Error("expected high completion rate insight about maintaining progress")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
