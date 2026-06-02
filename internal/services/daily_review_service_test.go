package services_test

import (
	"testing"
	"time"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestGetSummary_ComputesCorrectly(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	db.Create(&models.Quest{UserID: userID, Title: "Water", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})
	db.Create(&models.Quest{UserID: userID, Title: "Learn", Type: models.QuestTypeLearning, Status: models.QuestStatusCompleted, XPReward: 15, Date: today, CreatedAt: now})
	db.Create(&models.Quest{UserID: userID, Title: "Move", Type: models.QuestTypeMovement, Status: models.QuestStatusSkipped, XPReward: 10, Date: today, CreatedAt: now})
	db.Create(&models.Quest{UserID: userID, Title: "Review", Type: models.QuestTypeReview, Status: models.QuestStatusPending, XPReward: 10, Date: today, CreatedAt: now})

	dateStr := now.Format("2006-01-02")
	summary, err := svc.GetSummary(userID, dateStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.CompletedQuestCount != 2 {
		t.Errorf("expected completed=2, got %d", summary.CompletedQuestCount)
	}
	if summary.SkippedQuestCount != 1 {
		t.Errorf("expected skipped=1, got %d", summary.SkippedQuestCount)
	}
	if summary.PendingQuestCount != 1 {
		t.Errorf("expected pending=1, got %d", summary.PendingQuestCount)
	}
	if summary.TotalQuestCount != 4 {
		t.Errorf("expected total=4, got %d", summary.TotalQuestCount)
	}
	if summary.EarnedExp != 20 {
		t.Errorf("expected earned_exp=20, got %d", summary.EarnedExp)
	}
	if summary.CompletionRate != 0.5 {
		t.Errorf("expected completion_rate=0.5, got %f", summary.CompletionRate)
	}
	if summary.CompletedByType["water"] != 1 {
		t.Errorf("expected water=1, got %d", summary.CompletedByType["water"])
	}
	if summary.CompletedByType["learning"] != 1 {
		t.Errorf("expected learning=1, got %d", summary.CompletedByType["learning"])
	}
}

func TestSaveDailyReview_CreatesReview(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	db.Create(&models.Quest{UserID: userID, Title: "Water", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	dateStr := now.Format("2006-01-02")
	diffRating := 3
	energyLvl := 4
	satisfLvl := 4

	req := dto.SaveDailyReviewRequest{
		Date:                dateStr,
		Mood:                "good",
		DifficultyRating:    &diffRating,
		EnergyLevel:         &energyLvl,
		SatisfactionLevel:   &satisfLvl,
		HelpfulQuests:       []string{"water"},
		AnnoyingQuests:      []string{},
		BestMoment:          "Finished API",
		Challenge:           "Tired in afternoon",
		ImprovementTomorrow: "Smaller tasks",
		TomorrowAdjustments: []string{"more_breaks"},
		Note:                "Good day",
	}

	resp, summary, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Mood != "good" {
		t.Errorf("expected mood 'good', got '%s'", resp.Mood)
	}
	if resp.CompletedQuestCount != 1 {
		t.Errorf("expected completed_quest_count=1, got %d", resp.CompletedQuestCount)
	}
	if summary.CompletedQuestCount != 1 {
		t.Errorf("expected summary completed=1, got %d", summary.CompletedQuestCount)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeDailyReview).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 dailyReview log, got %d", logCount)
	}
}

func TestSaveDailyReview_UpsertsExisting(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	dateStr := now.Format("2006-01-02")
	req := dto.SaveDailyReviewRequest{Date: dateStr, Mood: "good", Note: "First"}

	_, _, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	req.Note = "Updated"
	_, _, err = svc.Save(userID, req)
	if err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	var count int64
	db.Model(&models.DailyReview{}).Where("user_id = ? AND date = ?", userID, today).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 review row, got %d", count)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeDailyReview).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 dailyReview log (no duplicate), got %d", logCount)
	}
}

func TestGetReviewToday_ReturnsFalseWhenMissing(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	result, err := svc.GetToday(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.HasReviewed {
		t.Error("expected has_reviewed to be false")
	}
	if result.Item != nil {
		t.Error("expected item to be nil")
	}
}

func TestGetReviewToday_ReturnsTrueWhenExists(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	dateStr := now.Format("2006-01-02")
	svc.Save(userID, dto.SaveDailyReviewRequest{Date: dateStr, Mood: "good"})

	result, err := svc.GetToday(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.HasReviewed {
		t.Error("expected has_reviewed to be true")
	}
	if result.Item == nil {
		t.Fatal("expected item to be non-nil")
	}
}

func TestSaveDailyReview_InvalidMoodRejected(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	_, _, err := svc.Save(userID, dto.SaveDailyReviewRequest{Mood: "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid mood")
	}
}

func TestSaveDailyReview_InvalidRatingRejected(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	badRating := 6
	_, _, err := svc.Save(userID, dto.SaveDailyReviewRequest{Mood: "good", DifficultyRating: &badRating})
	if err == nil {
		t.Fatal("expected error for rating > 5")
	}

	badRating = 0
	_, _, err = svc.Save(userID, dto.SaveDailyReviewRequest{Mood: "good", DifficultyRating: &badRating})
	if err == nil {
		t.Fatal("expected error for rating < 1")
	}
}
