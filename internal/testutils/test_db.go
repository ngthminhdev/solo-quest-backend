package testutils

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/pkg/logger"
)

func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	logger.InitForTest()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal("failed to connect to test database:", err)
	}

	err = db.AutoMigrate(
		&models.UserProfile{},
		&models.AuthAccount{},
		&models.OnboardingAnswer{},
		&models.Quest{},
		&models.QuestAction{},
		&models.DailyCheckin{},
		&models.DailyReview{},
		&models.LogEntry{},
		&models.XPTransaction{},
		&models.Reward{},
		&models.RewardRedemption{},
		&models.AppSettings{},
	)
	if err != nil {
		t.Fatal("failed to migrate test database:", err)
	}

	return db
}

func CleanupTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	tables := []string{
		"quest_actions",
		"xp_transactions",
		"log_entries",
		"daily_checkins",
		"daily_reviews",
		"reward_redemptions",
		"rewards",
		"onboarding_answers",
		"auth_accounts",
		"app_settings",
		"quests",
		"user_profiles",
	}

	for _, table := range tables {
		db.Exec("DELETE FROM " + table)
	}
}

func BootstrapTestUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()

	devUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	quietTime := "22:00"
	user := models.UserProfile{
		ID:                     devUserID,
		DisplayName:            "Test User",
		Level:                  1,
		CurrentLevelExp:        0,
		NextLevelExp:           100,
		TotalExp:               0,
		RewardPoints:           100,
		StreakDays:             0,
		BestStreak:             0,
		StreakShields:          2,
		TotalCompletedQuests:   0,
		TotalSkippedQuests:     0,
		HasCompletedOnboarding: false,
		QuietAfterTime:         &quietTime,
	}

	if err := db.Create(&user).Error; err != nil {
		t.Fatal("failed to create test user:", err)
	}

	return devUserID
}

func CreateTestUser(db *gorm.DB, userID uuid.UUID, email string) {
	quietTime := "22:00"
	user := models.UserProfile{
		ID:                     userID,
		DisplayName:            "Test User",
		Level:                  1,
		CurrentLevelExp:        0,
		NextLevelExp:           100,
		TotalExp:               0,
		RewardPoints:           100,
		StreakDays:             0,
		BestStreak:             0,
		StreakShields:          2,
		TotalCompletedQuests:   0,
		TotalSkippedQuests:     0,
		HasCompletedOnboarding: false,
		QuietAfterTime:         &quietTime,
	}

	db.Create(&user)
}

func CreateTestQuest(t *testing.T, db *gorm.DB, userID uuid.UUID, status models.QuestStatus) *models.Quest {
	t.Helper()

	quest := &models.Quest{
		UserID:           userID,
		Title:            "Test Quest",
		Description:      "Test Description",
		Type:             models.QuestTypeDaily,
		Status:           status,
		Difficulty:       models.QuestDifficultyEasy,
		Source:           models.QuestSourceDailyPlan,
		XPReward:         10,
		EstimatedMinutes: 5,
		Reason:           "Test reason",
		Instruction:      "Test instruction",
		Date:             time.Now().UTC().Truncate(24 * time.Hour),
	}

	if err := db.Create(quest).Error; err != nil {
		t.Fatal("failed to create test quest:", err)
	}

	return quest
}

func CreateTestReward(t *testing.T, db *gorm.DB, userID uuid.UUID, costPoints int, status models.RewardStatus) *models.Reward {
	t.Helper()

	reward := &models.Reward{
		UserID:      userID,
		Title:       "Test Reward",
		Description: "A test reward",
		Type:        models.RewardTypeRest,
		CostPoints:  costPoints,
		IconText:    "🛋️",
		Status:      status,
	}

	if err := db.Create(reward).Error; err != nil {
		t.Fatal("failed to create test reward:", err)
	}

	return reward
}
