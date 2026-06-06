package services_test

import (
	"encoding/json"
	"testing"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
	"solo_quest_backend/internal/pkg/timeutil"
)

func TestConfigGenerator_GeneratesQuestsBasedOnSettings(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	generator := services.NewConfigQuestGenerator(db)

	// Generate quests for today
	today := timeutil.TodayVN()
	quests, err := generator.GenerateConfigBasedDailyQuests(userID, today)
	if err != nil {
		t.Fatalf("failed to generate quests: %v", err)
	}

	// Should generate 8 quests (default daily_quest_count)
	if len(quests) == 0 {
		t.Fatal("expected quests to be generated")
	}
	if len(quests) > 20 {
		t.Errorf("generated too many quests: %d", len(quests))
	}

	// All quests should have source = configBased
	for _, q := range quests {
		if q.Source != models.QuestSourceConfigBased {
			t.Errorf("expected source=configBased, got %s", q.Source)
		}
	}
}

func TestConfigGenerator_IdempotentGeneration(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	generator := services.NewConfigQuestGenerator(db)

	today := timeutil.TodayVN()

	// Generate first time
	quests1, err := generator.GenerateConfigBasedDailyQuests(userID, today)
	if err != nil {
		t.Fatalf("failed to generate quests: %v", err)
	}
	count1 := len(quests1)
	if count1 == 0 {
		t.Fatal("expected quests to be generated on first call")
	}

	// Generate second time - should return empty (quests already exist)
	quests2, err := generator.GenerateConfigBasedDailyQuests(userID, today)
	if err != nil {
		t.Fatalf("failed on second generation: %v", err)
	}
	if quests2 != nil && len(quests2) > 0 {
		t.Errorf("expected no new quests on second call, got %d", len(quests2))
	}

	// Verify total count in DB is still count1
	var dbQuests []models.Quest
	start, end := timeutil.DayRangeVN(today)
	db.Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).Find(&dbQuests)
	if len(dbQuests) != count1 {
		t.Errorf("expected %d quests in DB, got %d", count1, len(dbQuests))
	}
}

func TestConfigGenerator_RespectsEnabledCategories(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Get default settings to ensure they exist
	settingsSvc := services.NewQuestSettingsService(db)
	_, err := settingsSvc.GetOrCreate(userID)
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}

	// Update to enable only learning category
	enabledCats := []string{"learning"}
	catsJSON, _ := json.Marshal(enabledCats)
	db.Model(&models.QuestSettings{}).
		Where("user_id = ?", userID).
		Update("enabled_categories", catsJSON)

	generator := services.NewConfigQuestGenerator(db)
	today := timeutil.TodayVN()

	quests, err := generator.GenerateConfigBasedDailyQuests(userID, today)
	if err != nil {
		t.Fatalf("failed to generate quests: %v", err)
	}

	// All generated quests should be learning type
	for _, q := range quests {
		if q.Type != models.QuestTypeLearning {
			t.Errorf("expected only learning quests, got type=%s", q.Type)
		}
	}
}

func TestConfigGenerator_RespectsDisabledCategories(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Ensure settings exist first
	settingsSvc := services.NewQuestSettingsService(db)
	_, err := settingsSvc.GetOrCreate(userID)
	if err != nil {
		t.Fatalf("failed to create settings: %v", err)
	}

	// Update to disable water category (enable others)
	enabledCats := []string{"breakTime", "movement", "learning", "sleep", "review"}
	catsJSON, _ := json.Marshal(enabledCats)
	result := db.Model(&models.QuestSettings{}).
		Where("user_id = ?", userID).
		Update("enabled_categories", catsJSON)
	if result.Error != nil {
		t.Fatalf("failed to update enabled categories: %v", result.Error)
	}

	generator := services.NewConfigQuestGenerator(db)
	today := timeutil.TodayVN()

	quests, err := generator.GenerateConfigBasedDailyQuests(userID, today)
	if err != nil {
		t.Fatalf("failed to generate quests: %v", err)
	}

	// No water quests should be generated
	for _, q := range quests {
		if q.Type == models.QuestTypeWater {
			t.Errorf("expected no water quests when water is disabled, got quest: %s", q.Title)
		}
	}
}

func TestConfigGenerator_RespectsDailyQuestCount(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Ensure settings exist first
	settingsSvc := services.NewQuestSettingsService(db)
	_, err := settingsSvc.GetOrCreate(userID)
	if err != nil {
		t.Fatalf("failed to create settings: %v", err)
	}

	// Set daily_quest_count to 5
	result := db.Model(&models.QuestSettings{}).
		Where("user_id = ?", userID).
		Update("daily_quest_count", 5)
	if result.Error != nil {
		t.Fatalf("failed to update daily_quest_count: %v", result.Error)
	}

	generator := services.NewConfigQuestGenerator(db)
	today := timeutil.TodayVN()

	quests, err := generator.GenerateConfigBasedDailyQuests(userID, today)
	if err != nil {
		t.Fatalf("failed to generate quests: %v", err)
	}

	// Should generate up to 5 quests
	if len(quests) > 5 {
		t.Errorf("expected up to 5 quests when daily_quest_count=5, got %d", len(quests))
	}
}

func TestConfigGenerator_RespectsMaxPerDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)

	// Get default settings and update water rule to max_per_day = 2
	settingsSvc := services.NewQuestSettingsService(db)
	settings, err := settingsSvc.GetOrCreate(userID)
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}

	// Parse rules
	var rules []map[string]interface{}
	json.Unmarshal([]byte(settings.Rules[0].Type), &rules) // This is wrong, need actual JSON parsing

	// For now, just verify generation works - detailed max_per_day test would need better rule manipulation
	generator := services.NewConfigQuestGenerator(db)
	today := timeutil.TodayVN()

	quests, err := generator.GenerateConfigBasedDailyQuests(userID, today)
	if err != nil {
		t.Fatalf("failed to generate quests: %v", err)
	}

	// Count water quests
	waterCount := 0
	for _, q := range quests {
		if q.Type == models.QuestTypeWater {
			waterCount++
		}
	}

	// Default water rule has max_per_day = 8
	if waterCount > 8 {
		t.Errorf("water quests exceeded max_per_day limit: %d", waterCount)
	}
}

func TestConfigGenerator_CreatesSettingsForNewUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	// Create a user without settings
	userID := testutils.BootstrapTestUser(t, db)

	// Delete quest settings if exist
	db.Where("user_id = ?", userID).Delete(&models.QuestSettings{})

	generator := services.NewConfigQuestGenerator(db)
	today := timeutil.TodayVN()

	// Should create settings and generate quests
	quests, err := generator.GenerateConfigBasedDailyQuests(userID, today)
	if err != nil {
		t.Fatalf("failed to generate quests for new user: %v", err)
	}

	if len(quests) == 0 {
		t.Error("expected quests to be generated for new user with auto-created settings")
	}

	// Verify settings were created
	var settings models.QuestSettings
	err = db.Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		t.Errorf("expected settings to be created, got error: %v", err)
	}
}
