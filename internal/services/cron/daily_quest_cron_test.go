package cron_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services/cron"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

type mockDailyQuestGenerator struct {
	results map[uuid.UUID]*quest_generation.GenerateTodayResult
	errs    map[uuid.UUID]error
	calls   []uuid.UUID
	reqs    []quest_generation.GenerateTodayRequest
	mu      sync.Mutex
}

func (m *mockDailyQuestGenerator) GenerateToday(
	ctx context.Context,
	userID uuid.UUID,
	req quest_generation.GenerateTodayRequest,
) (*quest_generation.GenerateTodayResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, userID)
	m.reqs = append(m.reqs, req)

	if err, exists := m.errs[userID]; exists {
		return nil, err
	}
	if res, exists := m.results[userID]; exists {
		return res, nil
	}
	return &quest_generation.GenerateTodayResult{
		Date:             *req.Date,
		Inserted:         true,
		ExistingReturned: false,
		Source:           "ai",
		GeneratedCount:   5,
	}, nil
}

func TestDailyQuestCron_DisabledByDefault(t *testing.T) {
	// Clean environment variables to test defaults
	os.Unsetenv("DAILY_QUEST_CRON_ENABLED")
	os.Unsetenv("DAILY_QUEST_CRON_TIME")
	os.Unsetenv("DAILY_QUEST_CRON_BATCH_SIZE")
	os.Unsetenv("DAILY_QUEST_CRON_TIMEZONE")

	cfg := config.Load()
	if cfg.Cron.DailyQuestEnabled {
		t.Error("expected daily quest cron to be disabled by default")
	}
	if cfg.Cron.DailyQuestTime != "04:00" {
		t.Errorf("expected default time 04:00, got %s", cfg.Cron.DailyQuestTime)
	}
	if cfg.Cron.DailyQuestBatchSize != 5 {
		t.Errorf("expected default batch size 5, got %d", cfg.Cron.DailyQuestBatchSize)
	}
	if cfg.Cron.DailyQuestTimezone != "Asia/Ho_Chi_Minh" {
		t.Errorf("expected default timezone Asia/Ho_Chi_Minh, got %s", cfg.Cron.DailyQuestTimezone)
	}
}

func TestDailyQuestCron_ConfigParsing(t *testing.T) {
	t.Run("valid HH:mm time config", func(t *testing.T) {
		os.Setenv("DAILY_QUEST_CRON_ENABLED", "true")
		os.Setenv("DAILY_QUEST_CRON_TIME", "05:30")
		os.Setenv("DAILY_QUEST_CRON_BATCH_SIZE", "10")
		defer os.Unsetenv("DAILY_QUEST_CRON_ENABLED")
		defer os.Unsetenv("DAILY_QUEST_CRON_TIME")
		defer os.Unsetenv("DAILY_QUEST_CRON_BATCH_SIZE")

		cfg := config.Load()
		if !cfg.Cron.DailyQuestEnabled {
			t.Error("expected cron to be enabled")
		}
		if cfg.Cron.DailyQuestTime != "05:30" {
			t.Errorf("expected time 05:30, got %s", cfg.Cron.DailyQuestTime)
		}
		if cfg.Cron.DailyQuestBatchSize != 10 {
			t.Errorf("expected batch size 10, got %d", cfg.Cron.DailyQuestBatchSize)
		}
	})

	t.Run("invalid time format disables cron safely", func(t *testing.T) {
		os.Setenv("DAILY_QUEST_CRON_ENABLED", "true")
		os.Setenv("DAILY_QUEST_CRON_TIME", "invalid-time")
		defer os.Unsetenv("DAILY_QUEST_CRON_ENABLED")
		defer os.Unsetenv("DAILY_QUEST_CRON_TIME")

		cfg := config.Load()
		if cfg.Cron.DailyQuestEnabled {
			t.Error("expected cron to be disabled safely due to invalid time format")
		}
	})

	t.Run("invalid hour time format disables cron safely", func(t *testing.T) {
		os.Setenv("DAILY_QUEST_CRON_ENABLED", "true")
		os.Setenv("DAILY_QUEST_CRON_TIME", "25:00")
		defer os.Unsetenv("DAILY_QUEST_CRON_ENABLED")
		defer os.Unsetenv("DAILY_QUEST_CRON_TIME")

		cfg := config.Load()
		if cfg.Cron.DailyQuestEnabled {
			t.Error("expected cron to be disabled safely due to invalid hour format")
		}
	})
}

func TestDailyQuestCron_CalculateNextRunDelay(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")

	// 1. Cron scheduled time is in the future today
	now := time.Date(2026, 6, 7, 2, 0, 0, 0, loc)
	delay, err := cron.CalculateNextRunDelay(now, "04:00", loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if delay != 2*time.Hour {
		t.Errorf("expected 2h delay, got %v", delay)
	}

	// 2. Cron scheduled time is in the past today (should schedule for tomorrow)
	nowPast := time.Date(2026, 6, 7, 5, 0, 0, 0, loc)
	delayPast, err := cron.CalculateNextRunDelay(nowPast, "04:00", loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if delayPast != 23*time.Hour {
		t.Errorf("expected 23h delay, got %v", delayPast)
	}
}

func setupCronTestUsers(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	// User 1: Onboarded
	userID1 := uuid.New()
	user1 := models.UserProfile{
		ID:                     userID1,
		DisplayName:            "Onboarded User",
		HasCompletedOnboarding: true,
	}
	if err := db.Create(&user1).Error; err != nil {
		t.Fatal(err)
	}

	// User 2: Non-onboarded
	userID2 := uuid.New()
	user2 := models.UserProfile{
		ID:                     userID2,
		DisplayName:            "Non-onboarded User",
		HasCompletedOnboarding: false,
	}
	if err := db.Create(&user2).Error; err != nil {
		t.Fatal(err)
	}

	return userID1, userID2
}

func TestDailyQuestCron_RunOnce_UserSelection(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	onboardedUserID, nonOnboardedUserID := setupCronTestUsers(t, db)

	cfg := &config.Config{
		Cron: config.CronConfig{
			DailyQuestBatchSize: 5,
		},
	}

	mockGen := &mockDailyQuestGenerator{
		results: make(map[uuid.UUID]*quest_generation.GenerateTodayResult),
		errs:    make(map[uuid.UUID]error),
	}

	dailyCron := cron.NewDailyQuestCron(db, mockGen, cfg)

	result, err := dailyCron.RunOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ProcessedUsers != 1 {
		t.Errorf("expected 1 processed user, got %d", result.ProcessedUsers)
	}

	mockGen.mu.Lock()
	defer mockGen.mu.Unlock()

	// Verify onboarded user was called
	foundOnboarded := false
	for _, callID := range mockGen.calls {
		if callID == onboardedUserID {
			foundOnboarded = true
		}
		if callID == nonOnboardedUserID {
			t.Error("non-onboarded user should not be processed by cron")
		}
	}
	if !foundOnboarded {
		t.Error("expected onboarded user to be processed")
	}

	// Verify GenerationService call request params
	if len(mockGen.reqs) != 1 {
		t.Fatalf("expected 1 generator call, got %d", len(mockGen.reqs))
	}
	req := mockGen.reqs[0]
	if req.PreferAI == nil || !*req.PreferAI {
		t.Error("expected prefer_ai = true")
	}
	if req.Force == nil || *req.Force {
		t.Error("expected force = false")
	}
	if req.ReplacePendingOnly == nil || !*req.ReplacePendingOnly {
		t.Error("expected replace_pending_only = true")
	}
}

func TestDailyQuestCron_RunOnce_BatchingAndPaging(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	// Create 7 onboarded users
	for i := 0; i < 7; i++ {
		u := models.UserProfile{
			ID:                     uuid.New(),
			DisplayName:            "User",
			HasCompletedOnboarding: true,
		}
		db.Create(&u)
	}

	// Batch size = 3 (will require 3 pages: 3, 3, 1)
	cfg := &config.Config{
		Cron: config.CronConfig{
			DailyQuestBatchSize: 3,
		},
	}

	mockGen := &mockDailyQuestGenerator{
		results: make(map[uuid.UUID]*quest_generation.GenerateTodayResult),
		errs:    make(map[uuid.UUID]error),
	}

	dailyCron := cron.NewDailyQuestCron(db, mockGen, cfg)
	result, err := dailyCron.RunOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ProcessedUsers != 7 {
		t.Errorf("expected 7 users processed, got %d", result.ProcessedUsers)
	}
	if len(mockGen.calls) != 7 {
		t.Errorf("expected 7 calls to generator, got %d", len(mockGen.calls))
	}
}

func TestDailyQuestCron_RunOnce_ErrorIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID1 := uuid.New()
	u1 := models.UserProfile{ID: userID1, DisplayName: "User 1", HasCompletedOnboarding: true}
	db.Create(&u1)

	userID2 := uuid.New()
	u2 := models.UserProfile{ID: userID2, DisplayName: "User 2", HasCompletedOnboarding: true}
	db.Create(&u2)

	cfg := &config.Config{
		Cron: config.CronConfig{
			DailyQuestBatchSize: 5,
		},
	}

	mockGen := &mockDailyQuestGenerator{
		results: make(map[uuid.UUID]*quest_generation.GenerateTodayResult),
		errs:    make(map[uuid.UUID]error),
	}

	// User 1 will fail, User 2 will succeed
	mockGen.errs[userID1] = errors.New("AI error")
	mockGen.results[userID2] = &quest_generation.GenerateTodayResult{
		Inserted:         true,
		ExistingReturned: false,
		Source:           "ai",
		GeneratedCount:   3,
	}

	dailyCron := cron.NewDailyQuestCron(db, mockGen, cfg)
	result, err := dailyCron.RunOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify error isolation: cron continues and successfully processes User 2
	if result.ProcessedUsers != 2 {
		t.Errorf("expected 2 users processed, got %d", result.ProcessedUsers)
	}
	if result.FailedCount != 1 {
		t.Errorf("expected 1 failed user, got %d", result.FailedCount)
	}
	if result.GeneratedCount != 3 {
		t.Errorf("expected 3 generated quests from User 2, got %d", result.GeneratedCount)
	}

	// Verify calls map has both users
	if len(mockGen.calls) != 2 {
		t.Errorf("expected 2 calls, got %d", len(mockGen.calls))
	}
}

func TestDailyQuestCron_RunOnce_CountsAndSummary(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID1 := uuid.New()
	u1 := models.UserProfile{ID: userID1, DisplayName: "User 1", HasCompletedOnboarding: true}
	db.Create(&u1)

	userID2 := uuid.New()
	u2 := models.UserProfile{ID: userID2, DisplayName: "User 2", HasCompletedOnboarding: true}
	db.Create(&u2)

	userID3 := uuid.New()
	u3 := models.UserProfile{ID: userID3, DisplayName: "User 3", HasCompletedOnboarding: true}
	db.Create(&u3)

	cfg := &config.Config{
		Cron: config.CronConfig{
			DailyQuestBatchSize: 5,
		},
	}

	mockGen := &mockDailyQuestGenerator{
		results: make(map[uuid.UUID]*quest_generation.GenerateTodayResult),
		errs:    make(map[uuid.UUID]error),
	}

	// User 1: Generated successfully with AI
	mockGen.results[userID1] = &quest_generation.GenerateTodayResult{
		Inserted:         true,
		ExistingReturned: false,
		Source:           "ai",
		FallbackUsed:     false,
		GeneratedCount:   5,
	}

	// User 2: Generated successfully with fallback
	mockGen.results[userID2] = &quest_generation.GenerateTodayResult{
		Inserted:         true,
		ExistingReturned: false,
		Source:           "rule_based",
		FallbackUsed:     true,
		GeneratedCount:   4,
	}

	// User 3: Existing quests returned
	mockGen.results[userID3] = &quest_generation.GenerateTodayResult{
		Inserted:         false,
		ExistingReturned: true,
		Source:           "existing",
		FallbackUsed:     false,
		GeneratedCount:   0,
		PreservedCount:   5,
	}

	dailyCron := cron.NewDailyQuestCron(db, mockGen, cfg)
	result, err := dailyCron.RunOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ProcessedUsers != 3 {
		t.Errorf("expected 3 processed users, got %d", result.ProcessedUsers)
	}
	if result.GeneratedCount != 9 { // 5 (User 1) + 4 (User 2)
		t.Errorf("expected 9 generated quests, got %d", result.GeneratedCount)
	}
	if result.ExistingCount != 1 { // User 3
		t.Errorf("expected 1 existing user, got %d", result.ExistingCount)
	}
	if result.FallbackCount != 1 { // User 2
		t.Errorf("expected 1 fallback user, got %d", result.FallbackCount)
	}
	if result.FailedCount != 0 {
		t.Errorf("expected 0 failed users, got %d", result.FailedCount)
	}
}
