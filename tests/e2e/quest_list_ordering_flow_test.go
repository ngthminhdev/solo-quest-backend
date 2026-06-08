package e2e

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

func setupQuestOrderingTestRouter(t *testing.T, db *gorm.DB, devUserID uuid.UUID) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	questService := services.NewQuestService(db)
	questHandler := handlers.NewQuestHandler(questService)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(devUserID))
	{
		protected.GET("/api/quests", questHandler.GetQuests)
	}

	return r
}

func TestQuestListOrderingFlow_E2E(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	today := timeutil.TodayVN()

	t8 := time.Date(today.Year(), today.Month(), today.Day(), 8, 0, 0, 0, timeutil.LocationVN)
	t12 := time.Date(today.Year(), today.Month(), today.Day(), 12, 0, 0, 0, timeutil.LocationVN)
	t20 := time.Date(today.Year(), today.Month(), today.Day(), 20, 0, 0, 0, timeutil.LocationVN)

	now := time.Now()

	// Insert in non-ordered reminder_time to verify sorting
	q20 := models.Quest{
		UserID:           userID,
		Title:            "E2E Quest 20:00",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusCompleted,
		Date:             today,
		ReminderTime:     &t20,
		CreatedAt:        now.Add(-1 * time.Hour),
	}
	db.Create(&q20)

	q8 := models.Quest{
		UserID:           userID,
		Title:            "E2E Quest 08:00",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Date:             today,
		ReminderTime:     &t8,
		CreatedAt:        now.Add(-2 * time.Hour),
	}
	db.Create(&q8)

	qNull := models.Quest{
		UserID:           userID,
		Title:            "E2E Quest Null Reminder",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusSkipped,
		Date:             today,
		ReminderTime:     nil,
		CreatedAt:        now.Add(-3 * time.Hour),
	}
	db.Create(&qNull)

	q12a := models.Quest{
		UserID:           userID,
		Title:            "E2E Quest 12:00 A",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusSnoozed,
		Date:             today,
		ReminderTime:     &t12,
		CreatedAt:        now.Add(-5 * time.Minute),
	}
	db.Create(&q12a)

	q12b := models.Quest{
		UserID:           userID,
		Title:            "E2E Quest 12:00 B",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Date:             today,
		ReminderTime:     &t12,
		CreatedAt:        now.Add(-10 * time.Minute), // Older created_at should come first
	}
	db.Create(&q12b)

	// User isolation check
	otherUserID := uuid.New()
	testutils.CreateTestUser(db, otherUserID, "other@example.com")
	t7 := time.Date(today.Year(), today.Month(), today.Day(), 7, 0, 0, 0, timeutil.LocationVN)
	qOther := models.Quest{
		UserID:           otherUserID,
		Title:            "E2E Other User Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Date:             today,
		ReminderTime:     &t7,
	}
	db.Create(&qOther)

	// Date filter check
	tomorrow := today.AddDate(0, 0, 1)
	qTomorrow := models.Quest{
		UserID:           userID,
		Title:            "E2E Tomorrow Quest",
		Type:             models.QuestTypeWater,
		Status:           models.QuestStatusPending,
		Date:             tomorrow,
		ReminderTime:     &t8,
	}
	db.Create(&qTomorrow)

	r := setupQuestOrderingTestRouter(t, db, userID)

	t.Run("Verify Quest Ordering E2E", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/quests", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		data := unwrapData(t, resp)
		quests, ok := data["quests"].([]interface{})
		if !ok {
			t.Fatal("expected quests array in response")
		}

		if len(quests) != 5 {
			t.Fatalf("expected 5 quests, got %d", len(quests))
		}

		expectedTitles := []string{
			"E2E Quest 08:00",
			"E2E Quest 12:00 B",
			"E2E Quest 12:00 A",
			"E2E Quest 20:00",
			"E2E Quest Null Reminder",
		}

		for i, expectedTitle := range expectedTitles {
			qObj := quests[i].(map[string]interface{})
			title := qObj["title"].(string)
			if title != expectedTitle {
				t.Errorf("at index %d: expected quest %q, got %q", i, expectedTitle, title)
			}
		}
	})
}
