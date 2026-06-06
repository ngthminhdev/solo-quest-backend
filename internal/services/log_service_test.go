package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestGetLogs_ReturnsLatestOrderedDesc(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLogService(db)

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		db.Create(&models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeActivity,
			Title:     "Log entry",
			Content:   "Content",
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}

	result, err := svc.GetLogs(userID, dto.LogFilter{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}

	if result.Items[0].CreatedAt <= result.Items[2].CreatedAt {
		t.Error("expected items ordered by created_at desc")
	}
}

func TestGetLogs_FilterByType(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLogService(db)

	now := time.Now().UTC()
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeQuestCompleted, Title: "Completed", Content: "", CreatedAt: now,
	})
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeActivity, Title: "Activity", Content: "", CreatedAt: now,
	})

	result, err := svc.GetLogs(userID, dto.LogFilter{Type: "questCompleted", Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Type != "questCompleted" {
		t.Errorf("expected type questCompleted, got %s", result.Items[0].Type)
	}
}

func TestGetLogs_FilterByQuestType(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLogService(db)

	now := time.Now().UTC()
	waterType := models.QuestTypeWater
	dailyType := models.QuestTypeDaily
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeQuestCompleted, Title: "Water", QuestType: &waterType, CreatedAt: now,
	})
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeQuestCompleted, Title: "Daily", QuestType: &dailyType, CreatedAt: now,
	})

	result, err := svc.GetLogs(userID, dto.LogFilter{QuestType: "water", Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
}

func TestGetLogs_FilterByDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLogService(db)

	date1 := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	date2 := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeActivity, Title: "Day 1", CreatedAt: date1,
	})
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeActivity, Title: "Day 2", CreatedAt: date2,
	})

	result, err := svc.GetLogs(userID, dto.LogFilter{Date: "2026-06-01", Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Title != "Day 1" {
		t.Errorf("expected title 'Day 1', got '%s'", result.Items[0].Title)
	}
}

func TestGetLogs_FilterByFromTo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLogService(db)

	dates := []time.Time{
		time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC),
	}
	for _, d := range dates {
		db.Create(&models.LogEntry{
			UserID: userID, Type: models.LogEntryTypeActivity, Title: "Entry", CreatedAt: d,
		})
	}

	result, err := svc.GetLogs(userID, dto.LogFilter{From: "2026-06-02", To: "2026-06-04", Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
}

func TestGetLogs_Pagination(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLogService(db)

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		db.Create(&models.LogEntry{
			UserID: userID, Type: models.LogEntryTypeActivity, Title: "Entry",
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}

	result, err := svc.GetLogs(userID, dto.LogFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}

	result, err = svc.GetLogs(userID, dto.LogFilter{Limit: 50, Offset: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items with offset 3, got %d", len(result.Items))
	}
}

func TestGetLogs_InvalidDateFormat(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLogService(db)

	_, err := svc.GetLogs(userID, dto.LogFilter{Date: "not-a-date", Limit: 50, Offset: 0})
	if err == nil {
		t.Fatal("expected error for invalid date format")
	}

	_, err = svc.GetLogs(userID, dto.LogFilter{From: "2026/06/01", Limit: 50, Offset: 0})
	if err == nil {
		t.Fatal("expected error for invalid from format")
	}

	_, err = svc.GetLogs(userID, dto.LogFilter{To: "bad", Limit: 50, Offset: 0})
	if err == nil {
		t.Fatal("expected error for invalid to format")
	}
}

func TestGetLogs_EmptyResult(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLogService(db)

	result, err := svc.GetLogs(userID, dto.LogFilter{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
	if result.Limit != 50 {
		t.Errorf("expected limit 50, got %d", result.Limit)
	}
}

func TestGetLogs_UserIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	user1ID := testutils.BootstrapTestUser(t, db)
	user2ID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testutils.CreateTestUser(db, user2ID, "user2@test.com")

	svc := services.NewLogService(db)

	now := time.Now().UTC()
	db.Create(&models.LogEntry{
		UserID: user1ID, Type: models.LogEntryTypeQuestCompleted, Title: "User1 log", CreatedAt: now,
	})
	db.Create(&models.LogEntry{
		UserID: user2ID, Type: models.LogEntryTypeQuestCompleted, Title: "User2 log", CreatedAt: now,
	})

	result, err := svc.GetLogs(user1ID, dto.LogFilter{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item for user1, got %d", len(result.Items))
	}
	if result.Items[0].Title != "User1 log" {
		t.Errorf("expected 'User1 log', got '%s'", result.Items[0].Title)
	}
}

func TestGetLogs_FilterByNewTypes(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLogService(db)

	now := time.Now().UTC()
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeLearningRoadmapCreated, Title: "Created", CreatedAt: now,
	})
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeLearningRoadmapFollowed, Title: "Followed", CreatedAt: now,
	})
	db.Create(&models.LogEntry{
		UserID: userID, Type: models.LogEntryTypeLevelUp, Title: "Level Up", CreatedAt: now,
	})

	// Filter by learning_roadmap_created
	result, err := svc.GetLogs(userID, dto.LogFilter{Type: "learning_roadmap_created", Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Type != "learning_roadmap_created" {
		t.Errorf("expected type learning_roadmap_created, got %s", result.Items[0].Type)
	}

	// Filter by level_up
	result, err = svc.GetLogs(userID, dto.LogFilter{Type: "level_up", Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
}
