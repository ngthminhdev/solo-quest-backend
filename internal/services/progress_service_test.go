package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestGetProgress_NoQuests(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewProgressService(db)

	progress, err := svc.GetProgress(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if progress.Level != 1 {
		t.Errorf("expected level 1, got %d", progress.Level)
	}
	if progress.TotalExp != 0 {
		t.Errorf("expected total_exp 0, got %d", progress.TotalExp)
	}
	if progress.RewardPoints != 100 {
		t.Errorf("expected reward_points 100, got %d", progress.RewardPoints)
	}
	if progress.TodayCompletedQuests != 0 {
		t.Errorf("expected today_completed_quests 0, got %d", progress.TodayCompletedQuests)
	}
	if progress.TodayTotalQuests != 0 {
		t.Errorf("expected today_total_quests 0, got %d", progress.TodayTotalQuests)
	}
	if progress.TodayCompletionRate != 0.0 {
		t.Errorf("expected today_completion_rate 0.0, got %f", progress.TodayCompletionRate)
	}
	if progress.WeeklyCompletionRate != 0.0 {
		t.Errorf("expected weekly_completion_rate 0.0, got %f", progress.WeeklyCompletionRate)
	}
	if len(progress.WeeklyDailyData) != 7 {
		t.Errorf("expected 7 weekly_daily_data items, got %d", len(progress.WeeklyDailyData))
	}
}

func TestGetProgress_WithQuestsToday(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewProgressService(db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)

	for i := 0; i < 2; i++ {
		q := models.Quest{
			UserID:    userID,
			Title:     "Completed Quest",
			Type:      models.QuestTypeWater,
			Status:    models.QuestStatusCompleted,
			XPReward:  10,
			Date:      today,
			CreatedAt: now,
		}
		db.Create(&q)
	}
	q := models.Quest{
		UserID:    userID,
		Title:     "Pending Quest",
		Type:      models.QuestTypeDaily,
		Status:    models.QuestStatusPending,
		XPReward:  10,
		Date:      today,
		CreatedAt: now,
	}
	db.Create(&q)

	progress, err := svc.GetProgress(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if progress.TodayCompletedQuests != 2 {
		t.Errorf("expected today_completed_quests 2, got %d", progress.TodayCompletedQuests)
	}
	if progress.TodayTotalQuests != 3 {
		t.Errorf("expected today_total_quests 3, got %d", progress.TodayTotalQuests)
	}

	expectedRate := 2.0 / 3.0
	if progress.TodayCompletionRate < expectedRate-0.01 || progress.TodayCompletionRate > expectedRate+0.01 {
		t.Errorf("expected today_completion_rate ~0.6667, got %f", progress.TodayCompletionRate)
	}
}

func TestGetProgress_CompletedByType(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewProgressService(db)

	now := time.Now().In(timeutil.LocationVN)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timeutil.LocationVN)

	types := []models.QuestType{models.QuestTypeWater, models.QuestTypeWater, models.QuestTypeLearning}
	for _, qt := range types {
		q := models.Quest{
			UserID:    userID,
			Title:     "Quest",
			Type:      qt,
			Status:    models.QuestStatusCompleted,
			XPReward:  10,
			Date:      today,
			CreatedAt: now,
		}
		db.Create(&q)
	}

	progress, err := svc.GetProgress(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if progress.CompletedByType["water"] != 2 {
		t.Errorf("expected completed_by_type water=2, got %d", progress.CompletedByType["water"])
	}
	if progress.CompletedByType["learning"] != 1 {
		t.Errorf("expected completed_by_type learning=1, got %d", progress.CompletedByType["learning"])
	}
}

func TestGetWeeklyChart_Returns7Days(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewProgressService(db)

	chart, err := svc.GetWeeklyChart(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chart.Items) != 7 {
		t.Fatalf("expected 7 items, got %d", len(chart.Items))
	}

	monday, _ := timeutil.ParseDateVN(chart.Items[0].Date)
	if monday.Weekday() != time.Monday {
		t.Errorf("expected first day to be Monday, got %s", monday.Weekday())
	}

	sunday, _ := timeutil.ParseDateVN(chart.Items[6].Date)
	if sunday.Weekday() != time.Sunday {
		t.Errorf("expected last day to be Sunday, got %s", sunday.Weekday())
	}

	if chart.WeekStart != chart.Items[0].Date {
		t.Errorf("expected week_start %s, got %s", chart.Items[0].Date, chart.WeekStart)
	}
	if chart.WeekEnd != chart.Items[6].Date {
		t.Errorf("expected week_end %s, got %s", chart.Items[6].Date, chart.WeekEnd)
	}

	expectedLabels := []string{"T2", "T3", "T4", "T5", "T6", "T7", "CN"}
	for i, item := range chart.Items {
		if item.DayLabel != expectedLabels[i] {
			t.Errorf("expected day_label %s at index %d, got %s", expectedLabels[i], i, item.DayLabel)
		}
	}
}

func TestGetXPHistory_ReturnsOrderedDesc(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewProgressService(db)

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		tx := models.XPTransaction{
			UserID:       userID,
			Amount:       10,
			Currency:     models.XPCurrencyXP,
			Source:       models.XPSourceTypeQuestCompletion,
			Description:  "Test XP",
			BalanceAfter: (i + 1) * 10,
			CreatedAt:    now.Add(time.Duration(i) * time.Minute),
		}
		db.Create(&tx)
	}

	result, err := svc.GetXPHistory(userID, dto.XPHistoryFilter{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}

	if result.Items[0].BalanceAfter != 30 {
		t.Errorf("expected first item balance_after 30, got %d", result.Items[0].BalanceAfter)
	}
	if result.Items[2].BalanceAfter != 10 {
		t.Errorf("expected last item balance_after 10, got %d", result.Items[2].BalanceAfter)
	}
}

func TestGetXPHistory_FilterCurrency(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewProgressService(db)

	now := time.Now().UTC()
	db.Create(&models.XPTransaction{
		UserID: userID, Amount: 10, Currency: models.XPCurrencyXP,
		Source: models.XPSourceTypeQuestCompletion, Description: "XP", BalanceAfter: 10, CreatedAt: now,
	})
	db.Create(&models.XPTransaction{
		UserID: userID, Amount: 5, Currency: models.XPCurrencyRewardPoints,
		Source: models.XPSourceTypeQuestCompletion, Description: "Points", BalanceAfter: 5, CreatedAt: now,
	})

	result, err := svc.GetXPHistory(userID, dto.XPHistoryFilter{Currency: "xp", Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Currency != "xp" {
		t.Errorf("expected currency xp, got %s", result.Items[0].Currency)
	}
}

func TestGetXPHistory_LimitAndOffset(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewProgressService(db)

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		db.Create(&models.XPTransaction{
			UserID: userID, Amount: 10, Currency: models.XPCurrencyXP,
			Source: models.XPSourceTypeQuestCompletion, Description: "XP",
			BalanceAfter: (i + 1) * 10, CreatedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}

	result, err := svc.GetXPHistory(userID, dto.XPHistoryFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}

	result, err = svc.GetXPHistory(userID, dto.XPHistoryFilter{Limit: 50, Offset: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items with offset 3, got %d", len(result.Items))
	}
}

func TestGetProgress_UserNotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	svc := services.NewProgressService(db)

	_, err := svc.GetProgress(uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}
