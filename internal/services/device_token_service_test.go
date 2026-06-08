package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestDeviceTokenService_UpsertNewToken(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDeviceTokenService(db)

	token, err := svc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{
		Token:    "test-fcm-token-1",
		Platform: "android",
	})
	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, userID, token.UserID)
	assert.Equal(t, "test-fcm-token-1", token.Token)
	assert.Equal(t, "android", token.Platform)
	assert.True(t, token.IsActive)
}

func TestDeviceTokenService_UpsertReactivateToken(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDeviceTokenService(db)

	token, err := svc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{
		Token:    "test-fcm-token-reactivate",
		Platform: "android",
	})
	assert.NoError(t, err)

	err = svc.DeactivateToken(userID, token.ID)
	assert.NoError(t, err)

	tokens, err := svc.GetActiveTokensByUserID(userID)
	assert.NoError(t, err)
	assert.Len(t, tokens, 0)

	reToken, err := svc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{
		Token:    "test-fcm-token-reactivate",
		Platform: "ios",
	})
	assert.NoError(t, err)
	assert.True(t, reToken.IsActive)
	assert.Equal(t, "ios", reToken.Platform)

	tokens, err = svc.GetActiveTokensByUserID(userID)
	assert.NoError(t, err)
	assert.Len(t, tokens, 1)
}

func TestDeviceTokenService_GetActiveTokensByUserID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDeviceTokenService(db)

	svc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "token-a", Platform: "android"})
	svc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "token-b", Platform: "ios"})

	tokens, err := svc.GetActiveTokensByUserID(userID)
	assert.NoError(t, err)
	assert.Len(t, tokens, 2)
}

func TestDeviceTokenService_DeactivateToken(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDeviceTokenService(db)

	token, err := svc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "token-deact", Platform: "android"})
	assert.NoError(t, err)

	err = svc.DeactivateToken(userID, token.ID)
	assert.NoError(t, err)

	tokens, err := svc.GetActiveTokensByUserID(userID)
	assert.NoError(t, err)
	assert.Len(t, tokens, 0)
}

func TestDeviceTokenService_DeactivateByRawToken(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDeviceTokenService(db)

	svc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "token-raw-deact", Platform: "android"})

	err := svc.DeactivateByRawToken("token-raw-deact")
	assert.NoError(t, err)

	tokens, err := svc.GetActiveTokensByUserID(userID)
	assert.NoError(t, err)
	assert.Len(t, tokens, 0)
}

func TestDeviceTokenService_ListActiveUserIDs(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID1 := testutils.BootstrapTestUser(t, db)
	userID2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testutils.CreateTestUser(db, userID2, "test2@test.com")

	svc := services.NewDeviceTokenService(db)

	svc.UpsertToken(userID1, &dto.RegisterDeviceTokenRequest{Token: "token-1", Platform: "android"})
	svc.UpsertToken(userID2, &dto.RegisterDeviceTokenRequest{Token: "token-2", Platform: "ios"})

	userIDs, err := svc.ListActiveUserIDs()
	assert.NoError(t, err)
	assert.Len(t, userIDs, 2)
}

func TestDeviceTokenService_GetByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewDeviceTokenService(db)

	token, err := svc.UpsertToken(userID, &dto.RegisterDeviceTokenRequest{Token: "token-getbyid", Platform: "android"})
	assert.NoError(t, err)

	found, err := svc.GetByID(userID, token.ID)
	assert.NoError(t, err)
	assert.Equal(t, token.ID, found.ID)

	_, err = svc.GetByID(userID, uuid.New())
	assert.Error(t, err)
}
