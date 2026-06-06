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

func setupProgressHandlerRouter(t *testing.T, db *gorm.DB) (*gin.Engine, uuid.UUID) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	progressService := services.NewProgressService(db)
	logService := services.NewLogService(db)

	progressHandler := handlers.NewProgressHandler(progressService)
	logHandler := handlers.NewLogHandler(logService)

	userID := testutils.BootstrapTestUser(t, db)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		progress := protected.Group("/api/progress")
		{
			progress.GET("", progressHandler.GetProgress)
			progress.GET("/weekly-chart", progressHandler.GetWeeklyChart)
			progress.GET("/xp-history", progressHandler.GetXPHistory)
		}

		logs := protected.Group("/api/logs")
		{
			logs.GET("", logHandler.GetLogs)
		}
	}

	return r, userID
}

func TestGetProgress_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupProgressHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/progress", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if _, ok := unwrapData(t, resp)["level"]; !ok {
		t.Error("expected 'level' in response")
	}
	if _, ok := unwrapData(t, resp)["today_completed_quests"]; !ok {
		t.Error("expected 'today_completed_quests' in response")
	}
	if _, ok := unwrapData(t, resp)["weekly_daily_data"]; !ok {
		t.Error("expected 'weekly_daily_data' in response")
	}
}

func TestGetWeeklyChart_Returns200And7Items(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupProgressHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/progress/weekly-chart", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	items, ok := unwrapData(t, resp)["items"].([]interface{})
	if !ok {
		t.Fatal("expected items array")
	}
	if len(items) != 7 {
		t.Errorf("expected 7 items, got %d", len(items))
	}
}

func TestGetXPHistory_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupProgressHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/progress/xp-history", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if _, ok := unwrapData(t, resp)["items"]; !ok {
		t.Error("expected 'items' in response")
	}
}

func TestGetXPHistory_InvalidCurrency_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupProgressHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/progress/xp-history?currency=invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestGetLogs_Returns200(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupProgressHandlerRouter(t, db)

	req, _ := http.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if _, ok := unwrapData(t, resp)["items"]; !ok {
		t.Error("expected 'items' in response")
	}
}

func TestGetLogs_InvalidDate_Returns400(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupProgressHandlerRouter(t, db)

	tests := []string{
		"/api/logs?date=invalid",
		"/api/logs?from=2026/06/01",
		"/api/logs?to=bad-date",
	}

	for _, url := range tests {
		req, _ := http.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for %s, got %d", url, w.Code)
		}
	}
}

func TestGetProgress_WithQuests_ReturnsCorrectData(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupProgressHandlerRouter(t, db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)

	for i := 0; i < 2; i++ {
		db.Create(&models.Quest{
			UserID: userID, Title: "Done", Type: models.QuestTypeWater,
			Status: models.QuestStatusCompleted, XPReward: 10, Date: today, CreatedAt: now,
		})
	}
	db.Create(&models.Quest{
		UserID: userID, Title: "Pending", Type: models.QuestTypeDaily,
		Status: models.QuestStatusPending, XPReward: 10, Date: today, CreatedAt: now,
	})

	req, _ := http.NewRequest("GET", "/api/progress", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(unwrapData(t, resp)["today_completed_quests"].(float64)) != 2 {
		t.Errorf("expected today_completed_quests 2, got %v", unwrapData(t, resp)["today_completed_quests"])
	}
	if int(unwrapData(t, resp)["today_total_quests"].(float64)) != 3 {
		t.Errorf("expected today_total_quests 3, got %v", unwrapData(t, resp)["today_total_quests"])
	}
}

func TestGetLogs_WithTypeFilter_ReturnsFilteredData(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupProgressHandlerRouter(t, db)

	now := time.Now().UTC()
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeQuestCompleted, Title: "Completed",
		Content: "+10 EXP", CreatedAt: now,
	})
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeActivity, Title: "Activity",
		Content: "", CreatedAt: now,
	})

	req, _ := http.NewRequest("GET", "/api/logs?type=questCompleted", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	items := unwrapData(t, resp)["items"].([]interface{})
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}

	item := items[0].(map[string]interface{})
	if item["type"] != "questCompleted" {
		t.Errorf("expected type questCompleted, got %v", item["type"])
	}
}

func TestGetXPHistory_WithCurrencyFilter(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, userID := setupProgressHandlerRouter(t, db)

	now := time.Now().UTC()
	db.Create(&models.XPTransaction{
		UserID: userID, Amount: 10, Currency: models.XPCurrencyXP,
		Source: models.XPSourceTypeQuestCompletion, Description: "XP", BalanceAfter: 10, CreatedAt: now,
	})
	db.Create(&models.XPTransaction{
		UserID: userID, Amount: 5, Currency: models.XPCurrencyRewardPoints,
		Source: models.XPSourceTypeQuestCompletion, Description: "Points", BalanceAfter: 5, CreatedAt: now,
	})

	req, _ := http.NewRequest("GET", "/api/progress/xp-history?currency=xp", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	items := unwrapData(t, resp)["items"].([]interface{})
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestGetProgress_UserNotFound_Returns404(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	progressService := services.NewProgressService(db)
	progressHandler := handlers.NewProgressHandler(progressService)

	fakeID := uuid.New()
	protected := r.Group("")
	protected.Use(testutils.TestUserContext(fakeID))
	{
		protected.GET("/api/progress", progressHandler.GetProgress)
	}

	req, _ := http.NewRequest("GET", "/api/progress", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}
