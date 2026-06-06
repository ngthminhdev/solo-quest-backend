package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"solo_quest_backend/internal/middleware"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

func authTestRouter(manager *services.JWTManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", middleware.JWTAuth(manager), func(c *gin.Context) {
		userID, err := utils.GetCurrentUserID(c)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"user_id": userID.String()})
	})
	return r
}

func TestJWTAuthRejectsMissingHeader(t *testing.T) {
	r := authTestRouter(services.NewJWTManager("secret", 15))

	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuthRejectsMalformedHeader(t *testing.T) {
	r := authTestRouter(services.NewJWTManager("secret", 15))

	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Token abc")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuthRejectsInvalidToken(t *testing.T) {
	r := authTestRouter(services.NewJWTManager("secret", 15))

	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuthRejectsExpiredToken(t *testing.T) {
	manager := services.NewJWTManager("secret", 15)
	r := authTestRouter(manager)

	userID := uuid.New()
	claims := services.AccessTokenClaims{
		UserID:    userID.String(),
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(-1 * time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuthValidTokenSetsCurrentUserID(t *testing.T) {
	manager := services.NewJWTManager("secret", 15)
	r := authTestRouter(manager)

	userID := uuid.New()
	token, _, _, err := manager.GenerateAccessToken(userID, uuid.New())
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
