package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

type fakeGoogleVerifier struct {
	claims *services.GoogleClaims
	err    error
}

func (f fakeGoogleVerifier) VerifyIDToken(ctx context.Context, idToken string) (*services.GoogleClaims, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.claims, nil
}

func testAuthConfig() *config.Config {
	return &config.Config{
		AppEnv: "test",
		JWT: config.JWTConfig{
			Secret:              "test_jwt_secret",
			AccessTokenExpires:  15,
			RefreshTokenExpires: 30,
		},
		Google: config.GoogleConfig{
			ClientID:        "web-client",
			AndroidClientID: "android-client",
			IOSClientID:     "ios-client",
		},
	}
}

func testGoogleClaims(sub string) *services.GoogleClaims {
	return &services.GoogleClaims{
		Subject:       sub,
		Email:         "user@example.com",
		EmailVerified: true,
		Name:          "Google User",
		Picture:       "https://example.com/avatar.png",
		Audience:      "web-client",
		Issuer:        "https://accounts.google.com",
		Expiry:        time.Now().UTC().Add(time.Hour),
	}
}

func TestAuthServiceGoogleLoginCreatesNewUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	svc := services.NewAuthService(db, testAuthConfig(), fakeGoogleVerifier{claims: testGoogleClaims("google-sub-1")})

	resp, err := svc.GoogleLogin(context.Background(), services.GoogleLoginRequest{IDToken: "valid"}, services.AuthRequestMeta{
		UserAgent: "test-agent",
		IPAddress: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("GoogleLogin failed: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	if resp.TokenType != "Bearer" {
		t.Fatalf("expected Bearer token type, got %q", resp.TokenType)
	}
	if resp.User.Email != "user@example.com" || resp.User.Provider != "google" {
		t.Fatalf("unexpected auth user response: %+v", resp.User)
	}
	if resp.User.HasCompletedOnboarding {
		t.Fatal("new Google user should not be onboarded")
	}

	var userCount int64
	db.Model(&models.UserProfile{}).Count(&userCount)
	if userCount != 1 {
		t.Fatalf("expected 1 user, got %d", userCount)
	}

	var account models.AuthAccount
	if err := db.Where("provider = ? AND provider_uid = ?", models.AuthProviderGoogle, "google-sub-1").First(&account).Error; err != nil {
		t.Fatalf("expected google auth account: %v", err)
	}
	if account.LastLoginAt == nil {
		t.Fatal("expected last_login_at to be set")
	}

	var sessionCount int64
	db.Model(&models.UserSession{}).Where("user_id = ?", account.UserID).Count(&sessionCount)
	if sessionCount != 1 {
		t.Fatalf("expected 1 user session, got %d", sessionCount)
	}
}

func TestAuthServiceGoogleLoginExistingUserUpdatesWithoutDuplicate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	claims := testGoogleClaims("google-sub-existing")
	svc := services.NewAuthService(db, testAuthConfig(), fakeGoogleVerifier{claims: claims})

	first, err := svc.GoogleLogin(context.Background(), services.GoogleLoginRequest{IDToken: "valid"}, services.AuthRequestMeta{})
	if err != nil {
		t.Fatalf("first GoogleLogin failed: %v", err)
	}

	claims.Name = "Updated Name"
	claims.Picture = "https://example.com/new-avatar.png"
	second, err := svc.GoogleLogin(context.Background(), services.GoogleLoginRequest{IDToken: "valid"}, services.AuthRequestMeta{})
	if err != nil {
		t.Fatalf("second GoogleLogin failed: %v", err)
	}

	if first.User.ID != second.User.ID {
		t.Fatalf("expected same user ID, got %s and %s", first.User.ID, second.User.ID)
	}
	if second.User.DisplayName != "Updated Name" {
		t.Fatalf("expected updated display name, got %q", second.User.DisplayName)
	}

	var userCount, accountCount int64
	db.Model(&models.UserProfile{}).Count(&userCount)
	db.Model(&models.AuthAccount{}).Where("provider = ?", models.AuthProviderGoogle).Count(&accountCount)
	if userCount != 1 || accountCount != 1 {
		t.Fatalf("expected no duplicates, users=%d accounts=%d", userCount, accountCount)
	}
}

func TestAuthServiceGoogleLoginMissingConfig(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	cfg := testAuthConfig()
	cfg.Google = config.GoogleConfig{}
	svc := services.NewAuthService(db, cfg, fakeGoogleVerifier{claims: testGoogleClaims("google-sub")})

	_, err := svc.GoogleLogin(context.Background(), services.GoogleLoginRequest{IDToken: "valid"}, services.AuthRequestMeta{})
	if !errors.Is(err, services.ErrGoogleAuthNotConfigured) {
		t.Fatalf("expected ErrGoogleAuthNotConfigured, got %v", err)
	}
}

func TestAuthServiceGoogleLoginInvalidToken(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	svc := services.NewAuthService(db, testAuthConfig(), fakeGoogleVerifier{err: services.ErrInvalidGoogleToken})

	_, err := svc.GoogleLogin(context.Background(), services.GoogleLoginRequest{IDToken: "invalid"}, services.AuthRequestMeta{})
	if !errors.Is(err, services.ErrInvalidGoogleToken) {
		t.Fatalf("expected ErrInvalidGoogleToken, got %v", err)
	}
}

func TestAuthServiceRefreshAndLogout(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	svc := services.NewAuthService(db, testAuthConfig(), fakeGoogleVerifier{claims: testGoogleClaims("google-sub-refresh")})
	login, err := svc.GoogleLogin(context.Background(), services.GoogleLoginRequest{IDToken: "valid"}, services.AuthRequestMeta{})
	if err != nil {
		t.Fatalf("GoogleLogin failed: %v", err)
	}

	refreshed, err := svc.Refresh(login.RefreshToken, services.AuthRequestMeta{})
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
		t.Fatal("expected refreshed tokens")
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("expected refresh token rotation")
	}
	if _, err := svc.Refresh(login.RefreshToken, services.AuthRequestMeta{}); !errors.Is(err, services.ErrInvalidRefreshToken) {
		t.Fatalf("expected old refresh token to be rejected, got %v", err)
	}

	if err := svc.Logout(refreshed.RefreshToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
	if err := svc.Logout(refreshed.RefreshToken); err != nil {
		t.Fatalf("repeated Logout should be safe: %v", err)
	}
	if _, err := svc.Refresh(refreshed.RefreshToken, services.AuthRequestMeta{}); !errors.Is(err, services.ErrInvalidRefreshToken) {
		t.Fatalf("expected revoked refresh token to be rejected, got %v", err)
	}
}

func TestJWTManagerValidatesAccessToken(t *testing.T) {
	manager := services.NewJWTManager("secret", 15)
	userID := uuid.New()
	sessionID := uuid.New()

	token, _, _, err := manager.GenerateAccessToken(userID, sessionID)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	gotUserID, gotSessionID, err := manager.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}
	if gotUserID != userID || gotSessionID != sessionID {
		t.Fatalf("unexpected token claims user=%s session=%s", gotUserID, gotSessionID)
	}
}
