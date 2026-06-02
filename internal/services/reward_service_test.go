package services_test

import (
	"testing"

	"github.com/google/uuid"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestGetRewards_ReturnsRewardsAndWallet(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	svc := services.NewRewardService(db)
	result, err := svc.GetRewards(userID)
	if err != nil {
		t.Fatal("failed to get rewards:", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 reward, got %d", len(result.Items))
	}

	if result.Wallet.RewardPoints != 100 {
		t.Errorf("expected wallet reward_points 100, got %d", result.Wallet.RewardPoints)
	}

	if !result.Items[0].CanClaim {
		t.Error("expected can_claim true when user has enough points")
	}
}

func TestGetRewards_CanClaimFalseWhenInsufficientPoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	testutils.CreateTestReward(t, db, userID, 200, models.RewardStatusAvailable)

	svc := services.NewRewardService(db)
	result, err := svc.GetRewards(userID)
	if err != nil {
		t.Fatal("failed to get rewards:", err)
	}

	if result.Items[0].CanClaim {
		t.Error("expected can_claim false when user has insufficient points")
	}
}

func TestClaimReward_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	svc := services.NewRewardService(db)
	result, err := svc.ClaimReward(userID, reward.ID)
	if err != nil {
		t.Fatal("failed to claim reward:", err)
	}

	if result.Reward.Status != "claimed" {
		t.Errorf("expected reward status 'claimed', got '%s'", result.Reward.Status)
	}

	if result.Reward.ClaimedAt == nil {
		t.Error("expected claimed_at to be set")
	}

	if result.Profile.RewardPoints != 70 {
		t.Errorf("expected profile reward_points 70, got %d", result.Profile.RewardPoints)
	}

	if result.Redemption.PointsSpent != 30 {
		t.Errorf("expected redemption points_spent 30, got %d", result.Redemption.PointsSpent)
	}

	if result.Transaction.Amount != -30 {
		t.Errorf("expected transaction amount -30, got %d", result.Transaction.Amount)
	}

	if result.Transaction.Currency != "reward_points" {
		t.Errorf("expected transaction currency 'reward_points', got '%s'", result.Transaction.Currency)
	}

	if result.Transaction.Source != "reward_claim" {
		t.Errorf("expected transaction source 'reward_claim', got '%s'", result.Transaction.Source)
	}

	if result.Message != "reward claimed successfully" {
		t.Errorf("expected message 'reward claimed successfully', got '%s'", result.Message)
	}

	var user models.UserProfile
	db.Where("id = ?", userID).First(&user)
	if user.RewardPoints != 70 {
		t.Errorf("expected user reward_points 70 in DB, got %d", user.RewardPoints)
	}

	var redemptionCount int64
	db.Model(&models.RewardRedemption{}).Where("user_id = ? AND reward_id = ?", userID, reward.ID).Count(&redemptionCount)
	if redemptionCount != 1 {
		t.Errorf("expected 1 redemption, got %d", redemptionCount)
	}

	var txCount int64
	db.Model(&models.XPTransaction{}).Where("user_id = ? AND currency = ? AND source = ?", userID, models.XPCurrencyRewardPoints, models.XPSourceTypeRewardClaim).Count(&txCount)
	if txCount != 1 {
		t.Errorf("expected 1 reward_claim transaction, got %d", txCount)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeRewardClaimed).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 rewardClaimed log, got %d", logCount)
	}
}

func TestClaimReward_InsufficientPoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 200, models.RewardStatusAvailable)

	svc := services.NewRewardService(db)
	_, err := svc.ClaimReward(userID, reward.ID)
	if err != services.ErrInsufficientRewardPoints {
		t.Errorf("expected ErrInsufficientRewardPoints, got %v", err)
	}

	var user models.UserProfile
	db.Where("id = ?", userID).First(&user)
	if user.RewardPoints != 100 {
		t.Errorf("expected user reward_points unchanged at 100, got %d", user.RewardPoints)
	}

	var redemptionCount int64
	db.Model(&models.RewardRedemption{}).Where("user_id = ?", userID).Count(&redemptionCount)
	if redemptionCount != 0 {
		t.Errorf("expected 0 redemptions, got %d", redemptionCount)
	}
}

func TestClaimReward_CannotClaimTwice(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	svc := services.NewRewardService(db)
	_, err := svc.ClaimReward(userID, reward.ID)
	if err != nil {
		t.Fatal("first claim failed:", err)
	}

	_, err = svc.ClaimReward(userID, reward.ID)
	if err != services.ErrRewardAlreadyClaimed {
		t.Errorf("expected ErrRewardAlreadyClaimed, got %v", err)
	}

	var redemptionCount int64
	db.Model(&models.RewardRedemption{}).Where("user_id = ? AND reward_id = ?", userID, reward.ID).Count(&redemptionCount)
	if redemptionCount != 1 {
		t.Errorf("expected 1 redemption (no double), got %d", redemptionCount)
	}
}

func TestClaimReward_CannotClaimAnotherUsersReward(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	otherUserID := uuid.New()
	otherUser := models.UserProfile{
		ID:           otherUserID,
		DisplayName:  "Other User",
		RewardPoints: 100,
	}
	db.Create(&otherUser)

	reward := testutils.CreateTestReward(t, db, otherUserID, 30, models.RewardStatusAvailable)

	svc := services.NewRewardService(db)
	_, err := svc.ClaimReward(userID, reward.ID)
	if err != services.ErrRewardNotFound {
		t.Errorf("expected ErrRewardNotFound, got %v", err)
	}
}

func TestGetRedemptions_ReturnsHistoryOrderedDesc(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	reward1 := testutils.CreateTestReward(t, db, userID, 20, models.RewardStatusAvailable)
	reward2 := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	svc := services.NewRewardService(db)
	svc.ClaimReward(userID, reward1.ID)
	svc.ClaimReward(userID, reward2.ID)

	result, err := svc.GetRedemptions(userID, dto.RedemptionFilter{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatal("failed to get redemptions:", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 redemptions, got %d", len(result.Items))
	}

	if result.Items[0].PointsSpent != 30 {
		t.Errorf("expected first redemption points_spent 30 (desc order), got %d", result.Items[0].PointsSpent)
	}
}

func TestClaimReward_UsesUTC(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	reward := testutils.CreateTestReward(t, db, userID, 30, models.RewardStatusAvailable)

	svc := services.NewRewardService(db)
	result, err := svc.ClaimReward(userID, reward.ID)
	if err != nil {
		t.Fatal("failed to claim reward:", err)
	}

	if result.Reward.ClaimedAt.Location().String() != "UTC" {
		t.Errorf("expected claimed_at to be UTC, got %s", result.Reward.ClaimedAt.Location())
	}

	if result.Redemption.CreatedAt.Location().String() != "UTC" {
		t.Errorf("expected redemption created_at to be UTC, got %s", result.Redemption.CreatedAt.Location())
	}
}
