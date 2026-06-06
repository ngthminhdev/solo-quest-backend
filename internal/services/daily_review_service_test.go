package services_test

import (
	"strings"
	"testing"
	"time"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestGetSummary_ComputesCorrectly(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)

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

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)

	db.Create(&models.Quest{UserID: userID, Title: "Water", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	dateStr := now.Format("2006-01-02")

	req := dto.SaveDailyReviewRequest{
		Date:             dateStr,
		Mood:             "good",
		EnergyLevel:      "medium",
		Satisfaction:     4,
		TomorrowPriority: "learning",
	}

	resp, summary, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Mood != "good" {
		t.Errorf("expected mood 'good', got '%s'", resp.Mood)
	}
	if resp.EnergyLevel != "medium" {
		t.Errorf("expected energy_level 'medium', got '%s'", resp.EnergyLevel)
	}
	if resp.Satisfaction != 4 {
		t.Errorf("expected satisfaction 4, got %d", resp.Satisfaction)
	}
	if resp.TomorrowPriority != "learning" {
		t.Errorf("expected tomorrow_priority 'learning', got '%s'", resp.TomorrowPriority)
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

func TestSaveDailyReview_WithDateAndReflection(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	req := dto.SaveDailyReviewRequest{
		Date:             "2026-06-03",
		Mood:             "normal",
		EnergyLevel:      "low",
		Satisfaction:     3,
		Reflection:       "Hôm nay hơi mệt.",
		TomorrowPriority: "rest",
	}

	resp, _, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Date != "2026-06-03" {
		t.Errorf("expected date '2026-06-03', got '%s'", resp.Date)
	}
	if resp.Reflection != "Hôm nay hơi mệt." {
		t.Errorf("expected reflection 'Hôm nay hơi mệt.', got '%s'", resp.Reflection)
	}
	if resp.TomorrowPriority != "rest" {
		t.Errorf("expected tomorrow_priority 'rest', got '%s'", resp.TomorrowPriority)
	}
}

func TestSaveDailyReview_UpsertsExisting(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	dateStr := now.Format("2006-01-02")
	req := dto.SaveDailyReviewRequest{
		Date:             dateStr,
		Mood:             "good",
		EnergyLevel:      "medium",
		Satisfaction:     4,
		TomorrowPriority: "learning",
	}

	_, _, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	req.Mood = "very_good"
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

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	dateStr := now.Format("2006-01-02")
	svc.Save(userID, dto.SaveDailyReviewRequest{
		Date:             dateStr,
		Mood:             "good",
		EnergyLevel:      "medium",
		Satisfaction:     4,
		TomorrowPriority: "learning",
	})

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

	_, _, err := svc.Save(userID, dto.SaveDailyReviewRequest{
		Mood:             "invalid",
		EnergyLevel:      "medium",
		Satisfaction:     4,
		TomorrowPriority: "learning",
	})
	if err == nil {
		t.Fatal("expected error for invalid mood")
	}
}

func TestSaveDailyReview_InvalidEnergyLevelRejected(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	_, _, err := svc.Save(userID, dto.SaveDailyReviewRequest{
		Mood:             "good",
		EnergyLevel:      "very_high",
		Satisfaction:     4,
		TomorrowPriority: "learning",
	})
	if err == nil {
		t.Fatal("expected error for invalid energy_level")
	}
}

func TestSaveDailyReview_InvalidSatisfactionRejected(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	_, _, err := svc.Save(userID, dto.SaveDailyReviewRequest{
		Mood:             "good",
		EnergyLevel:      "medium",
		Satisfaction:     0,
		TomorrowPriority: "learning",
	})
	if err == nil {
		t.Fatal("expected error for satisfaction < 1")
	}

	_, _, err = svc.Save(userID, dto.SaveDailyReviewRequest{
		Mood:             "good",
		EnergyLevel:      "medium",
		Satisfaction:     6,
		TomorrowPriority: "learning",
	})
	if err == nil {
		t.Fatal("expected error for satisfaction > 5")
	}
}

func TestSaveDailyReview_InvalidTomorrowPriorityRejected(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	_, _, err := svc.Save(userID, dto.SaveDailyReviewRequest{
		Mood:             "good",
		EnergyLevel:      "medium",
		Satisfaction:     4,
		TomorrowPriority: "school",
	})
	if err == nil {
		t.Fatal("expected error for invalid tomorrow_priority")
	}
}

func TestSaveDailyReview_ReflectionMaxLength(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	longReflection := strings.Repeat("a", 201)
	_, _, err := svc.Save(userID, dto.SaveDailyReviewRequest{
		Mood:             "good",
		EnergyLevel:      "medium",
		Satisfaction:     4,
		TomorrowPriority: "learning",
		Reflection:       longReflection,
	})
	if err == nil {
		t.Fatal("expected error for reflection > 200 chars")
	}
}

func TestSaveDailyReview_OldFieldsNotRequired(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	dateStr := now.Format("2006-01-02")
	req := dto.SaveDailyReviewRequest{
		Date:             dateStr,
		Mood:             "good",
		EnergyLevel:      "medium",
		Satisfaction:     4,
		TomorrowPriority: "learning",
	}

	resp, _, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Mood != "good" {
		t.Errorf("expected mood 'good', got '%s'", resp.Mood)
	}
	if resp.EnergyLevel != "medium" {
		t.Errorf("expected energy_level 'medium', got '%s'", resp.EnergyLevel)
	}
	if resp.Satisfaction != 4 {
		t.Errorf("expected satisfaction 4, got %d", resp.Satisfaction)
	}
	if resp.TomorrowPriority != "learning" {
		t.Errorf("expected tomorrow_priority 'learning', got '%s'", resp.TomorrowPriority)
	}
}

func TestSaveDailyReview_ResponseIncludesNewFields(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDailyReviewService(db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)
	db.Create(&models.Quest{UserID: userID, Title: "Quest", Type: models.QuestTypeWater, Status: models.QuestStatusCompleted, XPReward: 5, Date: today, CreatedAt: now})

	dateStr := now.Format("2006-01-02")
	req := dto.SaveDailyReviewRequest{
		Date:             dateStr,
		Mood:             "good",
		EnergyLevel:      "high",
		Satisfaction:     5,
		Reflection:       "Hôm nay hoàn thành tốt.",
		TomorrowPriority: "health",
	}

	resp, _, err := svc.Save(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Mood != "good" {
		t.Errorf("expected mood 'good', got '%s'", resp.Mood)
	}
	if resp.EnergyLevel != "high" {
		t.Errorf("expected energy_level 'high', got '%s'", resp.EnergyLevel)
	}
	if resp.Satisfaction != 5 {
		t.Errorf("expected satisfaction 5, got %d", resp.Satisfaction)
	}
	if resp.Reflection != "Hôm nay hoàn thành tốt." {
		t.Errorf("expected reflection 'Hôm nay hoàn thành tốt.', got '%s'", resp.Reflection)
	}
	if resp.TomorrowPriority != "health" {
		t.Errorf("expected tomorrow_priority 'health', got '%s'", resp.TomorrowPriority)
	}
	if resp.AISummary == "" {
		t.Error("expected ai_summary to be non-empty")
	}
}
