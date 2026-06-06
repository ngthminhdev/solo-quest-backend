package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/middleware"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

type handlerFakeGoogleVerifier struct {
	claims *services.GoogleClaims
	err    error
}

func (f handlerFakeGoogleVerifier) VerifyIDToken(ctx context.Context, idToken string) (*services.GoogleClaims, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.claims, nil
}

func authHandlerConfig() *config.Config {
	return &config.Config{
		AppEnv: "test",
		JWT: config.JWTConfig{
			Secret:              "handler_secret",
			AccessTokenExpires:  15,
			RefreshTokenExpires: 30,
		},
		Google: config.GoogleConfig{ClientID: "web-client"},
	}
}

func authHandlerClaims() *services.GoogleClaims {
	return &services.GoogleClaims{
		Subject:       "handler-google-sub",
		Email:         "handler@example.com",
		EmailVerified: true,
		Name:          "Handler User",
		Picture:       "https://example.com/handler.png",
		Audience:      "web-client",
		Issuer:        "https://accounts.google.com",
		Expiry:        time.Now().UTC().Add(time.Hour),
	}
}

func setupAuthHandlerRouter(db *gorm.DB, cfg *config.Config, verifier services.GoogleTokenVerifier) (*gin.Engine, *services.AuthService) {
	gin.SetMode(gin.TestMode)
	userService := services.NewUserService(db)
	authService := services.NewAuthService(db, cfg, verifier)
	authHandler := handlers.NewAuthHandler(userService, authService)

	r := gin.New()
	auth := r.Group("/api/auth")
	{
		auth.POST("/google", authHandler.GoogleLogin)
		auth.POST("/refresh", authHandler.Refresh)
		auth.POST("/logout", authHandler.Logout)
		auth.GET("/me", middleware.JWTAuth(authService.JWTManager()), authHandler.Me)
	}
	return r, authService
}

func setupAuthIsolationRouter(db *gorm.DB, cfg *config.Config, verifier services.GoogleTokenVerifier) (*gin.Engine, *services.AuthService) {
	gin.SetMode(gin.TestMode)
	userService := services.NewUserService(db)
	authService := services.NewAuthService(db, cfg, verifier)
	logService := services.NewLogService(db)
	progressService := services.NewProgressService(db)
	onboardingService := services.NewOnboardingService(db)

	authHandler := handlers.NewAuthHandler(userService, authService)
	userHandler := handlers.NewUserHandler(userService)
	logHandler := handlers.NewLogHandler(logService)
	progressHandler := handlers.NewProgressHandler(progressService)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingService)

	r := gin.New()
	auth := r.Group("/api/auth")
	{
		auth.POST("/dev-login", authHandler.DevLogin)
		auth.POST("/google", authHandler.GoogleLogin)
	}

	protected := r.Group("/api")
	protected.Use(middleware.CurrentUserContext(cfg, authService.JWTManager()))
	{
		protected.GET("/users/me", userHandler.GetMe)
		protected.GET("/logs", logHandler.GetLogs)
		protected.GET("/progress", progressHandler.GetProgress)
		protected.POST("/onboarding", onboardingHandler.SaveOnboarding)
	}

	return r, authService
}

func TestGoogleLoginEndpointReturnsTokensAndUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupAuthHandlerRouter(db, authHandlerConfig(), handlerFakeGoogleVerifier{claims: authHandlerClaims()})

	body, _ := json.Marshal(map[string]string{"id_token": "valid"})
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/google", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	data := unwrapData(t, resp)
	if data["access_token"] == "" || data["refresh_token"] == "" {
		t.Fatalf("expected access and refresh tokens: %+v", data)
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user object: %+v", data)
	}
	if user["email"] != "handler@example.com" || user["provider"] != "google" {
		t.Fatalf("unexpected user payload: %+v", user)
	}
	if _, ok := user["has_completed_onboarding"].(bool); !ok {
		t.Fatalf("expected has_completed_onboarding boolean: %+v", user)
	}
}

func TestGoogleLoginEndpointMissingConfigReturnsClearError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	cfg := authHandlerConfig()
	cfg.Google = config.GoogleConfig{}
	r, _ := setupAuthHandlerRouter(db, cfg, handlerFakeGoogleVerifier{claims: authHandlerClaims()})

	body, _ := json.Marshal(map[string]string{"id_token": "valid"})
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/google", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("google auth is not configured")) {
		t.Fatalf("expected clear config error, got: %s", w.Body.String())
	}
}

func TestAuthMeEndpointRequiresJWT(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	r, _ := setupAuthHandlerRouter(db, authHandlerConfig(), handlerFakeGoogleVerifier{claims: authHandlerClaims()})

	req, _ := http.NewRequest(http.MethodGet, "/api/auth/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMeEndpointReturnsCurrentUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r, authService := setupAuthHandlerRouter(db, authHandlerConfig(), handlerFakeGoogleVerifier{claims: authHandlerClaims()})
	token, _, _, err := authService.JWTManager().GenerateAccessToken(userID, uuid.New())
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	user := unwrapData(t, resp)["user"].(map[string]any)
	if user["id"] != userID.String() {
		t.Fatalf("expected user id %s, got %+v", userID, user)
	}
}

func TestProtectedUsersMeWithGoogleTokenReturnsGoogleUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	if err := db.Model(&models.UserProfile{}).Where("id = ?", devUserID).Updates(map[string]any{
		"display_name":           "Dev User",
		"current_level_exp":      75,
		"total_completed_quests": 8,
	}).Error; err != nil {
		t.Fatalf("failed to update dev user: %v", err)
	}

	r, authService := setupAuthIsolationRouter(db, authHandlerConfig(), handlerFakeGoogleVerifier{claims: authHandlerClaims()})
	login, err := authService.GoogleLogin(context.Background(), services.GoogleLoginRequest{IDToken: "valid"}, services.AuthRequestMeta{})
	if err != nil {
		t.Fatalf("GoogleLogin failed: %v", err)
	}

	googleUserID, _, err := authService.JWTManager().ValidateAccessToken(login.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}
	if googleUserID == devUserID {
		t.Fatal("Google token must not contain the default dev user ID")
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+login.AccessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	user := unwrapData(t, resp)["user"].(map[string]any)
	if user["id"] != googleUserID.String() {
		t.Fatalf("expected Google user id %s, got %+v", googleUserID, user)
	}
	if user["id"] == devUserID.String() || user["display_name"] == "Dev User" {
		t.Fatalf("Google token resolved to dev user payload: %+v", user)
	}
}

func TestProtectedUsersMeWithDevLoginTokenReturnsDevUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	r, _ := setupAuthIsolationRouter(db, authHandlerConfig(), handlerFakeGoogleVerifier{claims: authHandlerClaims()})

	loginReq, _ := http.NewRequest(http.MethodPost, "/api/auth/dev-login", nil)
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("expected dev-login 200, got %d: %s", loginW.Code, loginW.Body.String())
	}
	var loginResp map[string]any
	if err := json.Unmarshal(loginW.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("invalid dev-login JSON: %v", err)
	}
	token, _ := unwrapData(t, loginResp)["access_token"].(string)
	if token == "" || token == "dev-token" {
		t.Fatalf("expected real dev JWT, got %q", token)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	user := unwrapData(t, resp)["user"].(map[string]any)
	if user["id"] != devUserID.String() {
		t.Fatalf("expected dev user id %s, got %+v", devUserID, user)
	}
}

func TestProtectedRouteRejectsInvalidAuthorizationWithoutDevFallback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	testutils.BootstrapTestUser(t, db)
	r, _ := setupAuthIsolationRouter(db, authHandlerConfig(), handlerFakeGoogleVerifier{claims: authHandlerClaims()})

	req, _ := http.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 instead of dev fallback, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGoogleTokenDoesNotReadDevLogsOrProgress(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	if err := db.Model(&models.UserProfile{}).Where("id = ?", devUserID).Updates(map[string]any{
		"current_level_exp":      75,
		"total_completed_quests": 8,
	}).Error; err != nil {
		t.Fatalf("failed to update dev progress: %v", err)
	}
	if err := db.Create(&models.LogEntry{
		UserID:  devUserID,
		Type:    models.LogEntryTypeActivity,
		Title:   "Dev-only log",
		Content: "should not be visible to Google user",
	}).Error; err != nil {
		t.Fatalf("failed to create dev log: %v", err)
	}

	r, authService := setupAuthIsolationRouter(db, authHandlerConfig(), handlerFakeGoogleVerifier{claims: authHandlerClaims()})
	login, err := authService.GoogleLogin(context.Background(), services.GoogleLoginRequest{IDToken: "valid"}, services.AuthRequestMeta{})
	if err != nil {
		t.Fatalf("GoogleLogin failed: %v", err)
	}

	logReq, _ := http.NewRequest(http.MethodGet, "/api/logs", nil)
	logReq.Header.Set("Authorization", "Bearer "+login.AccessToken)
	logW := httptest.NewRecorder()
	r.ServeHTTP(logW, logReq)
	if logW.Code != http.StatusOK {
		t.Fatalf("expected logs 200, got %d: %s", logW.Code, logW.Body.String())
	}
	var logResp map[string]any
	if err := json.Unmarshal(logW.Body.Bytes(), &logResp); err != nil {
		t.Fatalf("invalid logs JSON: %v", err)
	}
	items := unwrapData(t, logResp)["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("Google user should not see dev logs, got %+v", items)
	}

	progressReq, _ := http.NewRequest(http.MethodGet, "/api/progress", nil)
	progressReq.Header.Set("Authorization", "Bearer "+login.AccessToken)
	progressW := httptest.NewRecorder()
	r.ServeHTTP(progressW, progressReq)
	if progressW.Code != http.StatusOK {
		t.Fatalf("expected progress 200, got %d: %s", progressW.Code, progressW.Body.String())
	}
	var progressResp map[string]any
	if err := json.Unmarshal(progressW.Body.Bytes(), &progressResp); err != nil {
		t.Fatalf("invalid progress JSON: %v", err)
	}
	progress := unwrapData(t, progressResp)
	if int(progress["current_level_exp"].(float64)) == 75 || int(progress["total_completed_quests"].(float64)) == 8 {
		t.Fatalf("Google user should not see dev progress, got %+v", progress)
	}
}

func TestGoogleTokenOnboardingCreatesScheduleBlocksForGoogleUserOnly(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	devUserID := testutils.BootstrapTestUser(t, db)
	r, authService := setupAuthIsolationRouter(db, authHandlerConfig(), handlerFakeGoogleVerifier{claims: authHandlerClaims()})
	login, err := authService.GoogleLogin(context.Background(), services.GoogleLoginRequest{IDToken: "valid"}, services.AuthRequestMeta{})
	if err != nil {
		t.Fatalf("GoogleLogin failed: %v", err)
	}
	googleUserID, _, err := authService.JWTManager().ValidateAccessToken(login.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"display_name":             "Google Onboarded",
		"gender":                   "male",
		"main_activity":            "developer",
		"main_goals":               []string{"learn"},
		"work_schedule_type":       "weekdays",
		"work_start_time":          "09:00",
		"work_end_time":            "17:00",
		"preferred_free_times":     []string{"evening"},
		"learning_time_preference": "evening",
		"movement_time_preference": "morning",
		"break_reminder_interval":  60,
		"water_reminder_mode":      "normal",
		"quiet_after_time":         "22:00",
		"preferred_rewards":        []string{"coffee"},
		"health_limitations":       []string{},
		"activity_level":           "medium",
		"free_time_preference":     "evening",
		"wake_up_time":             "07:00",
		"target_sleep_time":        "23:00",
		"free_time_start":          "18:00",
		"free_time_end":            "21:00",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/onboarding", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+login.AccessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected onboarding 200, got %d: %s", w.Code, w.Body.String())
	}

	var googleBlocks, devBlocks int64
	db.Model(&models.ScheduleBlock{}).Where("user_id = ?", googleUserID).Count(&googleBlocks)
	db.Model(&models.ScheduleBlock{}).Where("user_id = ?", devUserID).Count(&devBlocks)
	if googleBlocks == 0 {
		t.Fatal("expected onboarding to create schedule blocks for Google user")
	}
	if devBlocks != 0 {
		t.Fatalf("expected no dev schedule blocks, got %d", devBlocks)
	}
}
