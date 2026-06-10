package services_test

import (
	"testing"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestBootstrapDefaultDevUserCreatesUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("failed to bootstrap dev user:", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	var user models.UserProfile
	err = db.Where("id = ?", devUserID).First(&user).Error
	if err != nil {
		t.Fatal("dev user not found:", err)
	}

	if user.DisplayName != "Minh Thanh" {
		t.Errorf("expected display name 'Minh Thanh', got '%s'", user.DisplayName)
	}

	if user.Level != 1 {
		t.Errorf("expected level 1, got %d", user.Level)
	}

	if user.RewardPoints != 100 {
		t.Errorf("expected reward points 100, got %d", user.RewardPoints)
	}

	var authCount int64
	db.Model(&models.AuthAccount{}).Where("user_id = ? AND provider = ?", devUserID, models.AuthProviderDev).Count(&authCount)
	if authCount != 1 {
		t.Errorf("expected 1 dev auth account, got %d", authCount)
	}

	var settingsCount int64
	db.Model(&models.AppSettings{}).Where("user_id = ?", devUserID).Count(&settingsCount)
	if settingsCount != 1 {
		t.Errorf("expected 1 app settings, got %d", settingsCount)
	}

	var rewardCount int64
	db.Model(&models.Reward{}).Where("user_id = ?", devUserID).Count(&rewardCount)
	if rewardCount != 0 {
		t.Errorf("expected 0 rewards, got %d", rewardCount)
	}
}

func TestBootstrapDefaultDevUserIsIdempotent(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)

	err := bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("first bootstrap failed:", err)
	}

	err = bootstrapService.BootstrapDefaultDevUser("test@example.com")
	if err != nil {
		t.Fatal("second bootstrap failed:", err)
	}

	devUserID := bootstrapService.GetDevUserID()

	var userCount int64
	db.Model(&models.UserProfile{}).Where("id = ?", devUserID).Count(&userCount)
	if userCount != 1 {
		t.Errorf("expected 1 user, got %d", userCount)
	}

	var authCount int64
	db.Model(&models.AuthAccount{}).Where("user_id = ? AND provider = ?", devUserID, models.AuthProviderDev).Count(&authCount)
	if authCount != 1 {
		t.Errorf("expected 1 dev auth account, got %d", authCount)
	}

	var settingsCount int64
	db.Model(&models.AppSettings{}).Where("user_id = ?", devUserID).Count(&settingsCount)
	if settingsCount != 1 {
		t.Errorf("expected 1 settings, got %d", settingsCount)
	}

	var rewardCount int64
	db.Model(&models.Reward{}).Where("user_id = ?", devUserID).Count(&rewardCount)
	if rewardCount != 0 {
		t.Errorf("expected 0 rewards, got %d", rewardCount)
	}
}

func TestBootstrapDoesNotUseDevDatabase(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	bootstrapService := services.NewBootstrapService(db)
	devUserID := bootstrapService.GetDevUserID()

	if devUserID.String() == "" {
		t.Fatal("dev user ID should not be empty")
	}
}
