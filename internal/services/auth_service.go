package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/models"
)

var (
	ErrGoogleAuthNotConfigured = errors.New("google auth is not configured")
	ErrInvalidGoogleToken      = errors.New("invalid google id token")
	ErrGoogleEmailUnverified   = errors.New("google email is not verified")
	ErrInvalidAccessToken      = errors.New("invalid access token")
	ErrInvalidRefreshToken     = errors.New("invalid refresh token")
	ErrExpiredRefreshToken     = errors.New("refresh token is expired")
)

type GoogleTokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*GoogleClaims, error)
}

type GoogleClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
	Audience      string
	Issuer        string
	Expiry        time.Time
}

type AuthRequestMeta struct {
	UserAgent string
	IPAddress string
}

type GoogleLoginRequest struct {
	IDToken  string `json:"id_token" binding:"required"`
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthUserResponse struct {
	ID                     string  `json:"id"`
	Email                  string  `json:"email"`
	DisplayName            string  `json:"display_name"`
	AvatarURL              *string `json:"avatar_url"`
	Provider               string  `json:"provider"`
	HasCompletedOnboarding bool    `json:"has_completed_onboarding"`
}

type AuthTokenResponse struct {
	AccessToken  string            `json:"access_token"`
	RefreshToken string            `json:"refresh_token"`
	TokenType    string            `json:"token_type"`
	ExpiresIn    int               `json:"expires_in"`
	User         *AuthUserResponse `json:"user,omitempty"`
}

type IssuedTokenPair struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
}

type AuthService struct {
	db               *gorm.DB
	cfg              *config.Config
	verifier         GoogleTokenVerifier
	jwtManager       *JWTManager
	allowedAudiences map[string]struct{}
	refreshTokenDays int
}

func NewAuthService(db *gorm.DB, cfg *config.Config, verifier GoogleTokenVerifier) *AuthService {
	if cfg == nil {
		cfg = config.Load()
	}
	if verifier == nil {
		verifier = NewHTTPGoogleTokenVerifier(cfg.Google.AllowedAudiences())
	}

	audiences := make(map[string]struct{})
	for _, audience := range cfg.Google.AllowedAudiences() {
		audiences[audience] = struct{}{}
	}

	return &AuthService{
		db:               db,
		cfg:              cfg,
		verifier:         verifier,
		jwtManager:       NewJWTManager(cfg.JWT.Secret, cfg.JWT.AccessTokenExpires),
		allowedAudiences: audiences,
		refreshTokenDays: cfg.JWT.RefreshTokenExpires,
	}
}

func (s *AuthService) JWTManager() *JWTManager {
	return s.jwtManager
}

func (s *AuthService) GoogleLogin(ctx context.Context, req GoogleLoginRequest, meta AuthRequestMeta) (*AuthTokenResponse, error) {
	if len(s.allowedAudiences) == 0 {
		return nil, ErrGoogleAuthNotConfigured
	}

	idToken := strings.TrimSpace(req.IDToken)
	if idToken == "" {
		return nil, ErrInvalidGoogleToken
	}

	claims, err := s.verifier.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, ErrInvalidGoogleToken
	}
	if claims == nil || claims.Subject == "" {
		return nil, ErrInvalidGoogleToken
	}
	if _, ok := s.allowedAudiences[claims.Audience]; !ok {
		return nil, ErrInvalidGoogleToken
	}
	if claims.Email != "" && !claims.EmailVerified {
		return nil, ErrGoogleEmailUnverified
	}

	var user *models.UserProfile
	var account *models.AuthAccount

	err = s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		var authAccount models.AuthAccount
		err := tx.Where("provider = ? AND provider_uid = ?", models.AuthProviderGoogle, claims.Subject).
			First(&authAccount).Error
		if err == nil {
			account = &authAccount
			var existingUser models.UserProfile
			if err := tx.Where("id = ?", authAccount.UserID).First(&existingUser).Error; err != nil {
				return err
			}
			user = &existingUser
			if err := s.updateGoogleUser(tx, user, account, claims, now); err != nil {
				return err
			}
			return s.ensureLoginDefaults(tx, user.ID)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		linkedUser, err := s.findUserByEmailAccount(tx, claims.Email)
		if err != nil {
			return err
		}
		if linkedUser != nil {
			user = linkedUser
		} else {
			createdUser, err := s.createGoogleUser(tx, claims)
			if err != nil {
				return err
			}
			user = createdUser
		}

		newAccount := models.AuthAccount{
			UserID:      user.ID,
			Provider:    models.AuthProviderGoogle,
			ProviderUID: claims.Subject,
			Email:       claims.Email,
			LastLoginAt: &now,
		}
		if err := tx.Create(&newAccount).Error; err != nil {
			return err
		}
		account = &newAccount
		if err := s.updateProfileFromClaims(tx, user, claims); err != nil {
			return err
		}
		return s.ensureLoginDefaults(tx, user.ID)
	})
	if err != nil {
		return nil, err
	}

	return s.issueTokens(user.ID, meta, s.authUserResponse(user, account))
}

func (s *AuthService) GetCurrentUser(userID uuid.UUID) (*AuthUserResponse, error) {
	var user models.UserProfile
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}

	account, err := s.primaryAuthAccount(userID)
	if err != nil {
		return nil, err
	}

	resp := s.authUserResponse(&user, account)
	return &resp, nil
}

func (s *AuthService) IssueTokensForUser(userID uuid.UUID, meta AuthRequestMeta) (*IssuedTokenPair, error) {
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return s.ensureLoginDefaults(tx, userID)
	}); err != nil {
		return nil, err
	}

	var refreshToken string
	var sessionID uuid.UUID
	err := s.db.Transaction(func(tx *gorm.DB) error {
		token, sid, err := s.createRefreshSession(tx, userID, meta)
		if err != nil {
			return err
		}
		refreshToken = token
		sessionID = sid
		return nil
	})
	if err != nil {
		return nil, err
	}

	accessToken, expiresIn, _, err := s.jwtManager.GenerateAccessToken(userID, sessionID)
	if err != nil {
		return nil, err
	}

	return &IssuedTokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	}, nil
}

func (s *AuthService) Refresh(refreshToken string, meta AuthRequestMeta) (*AuthTokenResponse, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}

	var userID uuid.UUID
	var newRefreshToken string
	var newSessionID uuid.UUID

	err := s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		var session models.UserSession
		if err := tx.Where("refresh_token_hash = ?", hashRefreshToken(refreshToken)).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidRefreshToken
			}
			return err
		}
		if session.RevokedAt != nil {
			return ErrInvalidRefreshToken
		}
		if !session.ExpiresAt.After(now) {
			return ErrExpiredRefreshToken
		}

		session.RevokedAt = &now
		if err := tx.Save(&session).Error; err != nil {
			return err
		}

		token, sessionID, err := s.createRefreshSession(tx, session.UserID, meta)
		if err != nil {
			return err
		}
		userID = session.UserID
		newRefreshToken = token
		newSessionID = sessionID
		return nil
	})
	if err != nil {
		return nil, err
	}

	accessToken, expiresIn, _, err := s.jwtManager.GenerateAccessToken(userID, newSessionID)
	if err != nil {
		return nil, err
	}

	return &AuthTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	}, nil
}

func (s *AuthService) Logout(refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}

	now := time.Now().UTC()
	var session models.UserSession
	err := s.db.Where("refresh_token_hash = ?", hashRefreshToken(refreshToken)).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if session.RevokedAt != nil {
		return nil
	}
	session.RevokedAt = &now
	return s.db.Save(&session).Error
}

func (s *AuthService) issueTokens(userID uuid.UUID, meta AuthRequestMeta, user AuthUserResponse) (*AuthTokenResponse, error) {
	var refreshToken string
	var sessionID uuid.UUID

	err := s.db.Transaction(func(tx *gorm.DB) error {
		token, sid, err := s.createRefreshSession(tx, userID, meta)
		if err != nil {
			return err
		}
		refreshToken = token
		sessionID = sid
		return nil
	})
	if err != nil {
		return nil, err
	}

	accessToken, expiresIn, _, err := s.jwtManager.GenerateAccessToken(userID, sessionID)
	if err != nil {
		return nil, err
	}

	return &AuthTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		User:         &user,
	}, nil
}

func (s *AuthService) createRefreshSession(tx *gorm.DB, userID uuid.UUID, meta AuthRequestMeta) (string, uuid.UUID, error) {
	token, err := generateRefreshToken()
	if err != nil {
		return "", uuid.Nil, err
	}

	userAgent := stringPtrOrNil(meta.UserAgent)
	ipAddress := stringPtrOrNil(meta.IPAddress)
	session := models.UserSession{
		UserID:           userID,
		RefreshTokenHash: hashRefreshToken(token),
		UserAgent:        userAgent,
		IPAddress:        ipAddress,
		ExpiresAt:        time.Now().UTC().Add(time.Duration(s.refreshTokenDays) * 24 * time.Hour),
	}
	if err := tx.Create(&session).Error; err != nil {
		return "", uuid.Nil, err
	}
	return token, session.ID, nil
}

func (s *AuthService) createGoogleUser(tx *gorm.DB, claims *GoogleClaims) (*models.UserProfile, error) {
	displayName := strings.TrimSpace(claims.Name)
	if displayName == "" {
		displayName = displayNameFromEmail(claims.Email)
	}
	if displayName == "" {
		displayName = "SoloQuest User"
	}

	avatarURL := stringPtrOrNil(claims.Picture)
	user := models.UserProfile{
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}
	if err := tx.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *AuthService) updateGoogleUser(tx *gorm.DB, user *models.UserProfile, account *models.AuthAccount, claims *GoogleClaims, now time.Time) error {
	account.Email = claims.Email
	account.LastLoginAt = &now
	if err := tx.Save(account).Error; err != nil {
		return err
	}
	return s.updateProfileFromClaims(tx, user, claims)
}

func (s *AuthService) updateProfileFromClaims(tx *gorm.DB, user *models.UserProfile, claims *GoogleClaims) error {
	changed := false
	if name := strings.TrimSpace(claims.Name); name != "" && user.DisplayName != name {
		user.DisplayName = name
		changed = true
	}
	if picture := strings.TrimSpace(claims.Picture); picture != "" {
		if user.AvatarURL == nil || *user.AvatarURL != picture {
			user.AvatarURL = &picture
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return tx.Save(user).Error
}

func (s *AuthService) findUserByEmailAccount(tx *gorm.DB, email string) (*models.UserProfile, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}

	var account models.AuthAccount
	err := tx.Where("email = ?", email).Order("created_at ASC").First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var user models.UserProfile
	if err := tx.Where("id = ?", account.UserID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *AuthService) ensureLoginDefaults(tx *gorm.DB, userID uuid.UUID) error {
	var settingsCount int64
	if err := tx.Model(&models.AppSettings{}).Where("user_id = ?", userID).Count(&settingsCount).Error; err != nil {
		return err
	}
	if settingsCount == 0 {
		settings := models.AppSettings{
			UserID:               userID,
			Locale:               "vi",
			Theme:                "dark",
			DailyQuestLimit:      10,
			NotificationsEnabled: true,
			QuietAfterTime:       "22:00",
			Timezone:             "Asia/Ho_Chi_Minh",
		}
		if err := tx.Create(&settings).Error; err != nil {
			return err
		}
	}

	var questSettingsCount int64
	if err := tx.Model(&models.QuestSettings{}).Where("user_id = ?", userID).Count(&questSettingsCount).Error; err != nil {
		return err
	}
	if questSettingsCount == 0 {
		if err := tx.Create(BuildDefaultQuestSettings(userID)).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *AuthService) primaryAuthAccount(userID uuid.UUID) (*models.AuthAccount, error) {
	var account models.AuthAccount
	err := s.db.Where("user_id = ? AND provider = ?", userID, models.AuthProviderGoogle).First(&account).Error
	if err == nil {
		return &account, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = s.db.Where("user_id = ?", userID).Order("created_at ASC").First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *AuthService) authUserResponse(user *models.UserProfile, account *models.AuthAccount) AuthUserResponse {
	resp := AuthUserResponse{
		ID:                     user.ID.String(),
		DisplayName:            user.DisplayName,
		AvatarURL:              user.AvatarURL,
		HasCompletedOnboarding: user.HasCompletedOnboarding,
	}
	if account != nil {
		resp.Email = account.Email
		resp.Provider = string(account.Provider)
	}
	return resp
}

type AccessTokenClaims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"sid,omitempty"`
	TokenType string `json:"type"`
	LegacyTyp string `json:"typ,omitempty"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret        []byte
	accessMinutes int
}

func NewJWTManager(secret string, accessMinutes int) *JWTManager {
	if secret == "" {
		secret = "change_me"
	}
	if accessMinutes <= 0 {
		accessMinutes = 60
	}
	return &JWTManager{
		secret:        []byte(secret),
		accessMinutes: accessMinutes,
	}
}

func (m *JWTManager) GenerateAccessToken(userID uuid.UUID, sessionID uuid.UUID) (string, int, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(m.accessMinutes) * time.Minute)
	claims := AccessTokenClaims{
		UserID:    userID.String(),
		SessionID: sessionID.String(),
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", 0, time.Time{}, err
	}
	return signed, int(expiresAt.Sub(now).Seconds()), expiresAt, nil
}

func (m *JWTManager) ValidateAccessToken(tokenValue string) (uuid.UUID, uuid.UUID, error) {
	tokenValue = strings.TrimSpace(tokenValue)
	if tokenValue == "" {
		return uuid.Nil, uuid.Nil, ErrInvalidAccessToken
	}

	claims := &AccessTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidAccessToken
		}
		return m.secret, nil
	}, jwt.WithExpirationRequired())
	if err != nil || token == nil || !token.Valid {
		return uuid.Nil, uuid.Nil, ErrInvalidAccessToken
	}
	tokenType := claims.TokenType
	if tokenType == "" {
		tokenType = claims.LegacyTyp
	}
	if tokenType != "access" {
		return uuid.Nil, uuid.Nil, ErrInvalidAccessToken
	}

	userIDStr := claims.UserID
	if userIDStr == "" {
		userIDStr = claims.Subject
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidAccessToken
	}

	sessionID := uuid.Nil
	if claims.SessionID != "" {
		sessionID, err = uuid.Parse(claims.SessionID)
		if err != nil {
			return uuid.Nil, uuid.Nil, ErrInvalidAccessToken
		}
	}
	return userID, sessionID, nil
}

func generateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func stringPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func displayNameFromEmail(email string) string {
	local, _, ok := strings.Cut(email, "@")
	if !ok {
		return strings.TrimSpace(email)
	}
	return strings.TrimSpace(local)
}
