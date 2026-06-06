package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type AuthHandler struct {
	userService *services.UserService
	authService *services.AuthService
}

func NewAuthHandler(userService *services.UserService, authService ...*services.AuthService) *AuthHandler {
	handler := &AuthHandler{userService: userService}
	if len(authService) > 0 {
		handler.authService = authService[0]
	}
	return handler
}

func (h *AuthHandler) DevLogin(c *gin.Context) {
	user, err := h.userService.GetDevUser()
	if err != nil {
		c.JSON(http.StatusNotFound, response.NotFound("dev user not found"))
		return
	}

	if h.authService != nil {
		tokens, err := h.authService.IssueTokensForUser(user.ID, authRequestMeta(c))
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to login dev user"))
			return
		}

		c.JSON(http.StatusOK, response.Success(gin.H{
			"user":          user,
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"token_type":    tokens.TokenType,
			"expires_in":    tokens.ExpiresIn,
		}))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{
		"user":          user,
		"access_token":  "dev-token",
		"refresh_token": "dev-refresh-token",
		"token_type":    "Bearer",
	}))
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	if h.authService == nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("auth service is not configured"))
		return
	}

	var req services.GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("id_token is required"))
		return
	}

	result, err := h.authService.GoogleLogin(c.Request.Context(), req, authRequestMeta(c))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrGoogleAuthNotConfigured):
			c.JSON(http.StatusInternalServerError, response.InternalError("google auth is not configured"))
		case errors.Is(err, services.ErrInvalidGoogleToken), errors.Is(err, services.ErrGoogleEmailUnverified):
			c.JSON(http.StatusUnauthorized, response.Unauthorized())
		default:
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to login with google"))
		}
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *AuthHandler) Me(c *gin.Context) {
	if h.authService == nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("auth service is not configured"))
		return
	}

	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	user, err := h.authService.GetCurrentUser(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, response.NotFound("user not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to fetch current user"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"user": user}))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	if h.authService == nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("auth service is not configured"))
		return
	}

	var req services.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("refresh_token is required"))
		return
	}

	result, err := h.authService.Refresh(req.RefreshToken, authRequestMeta(c))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidRefreshToken), errors.Is(err, services.ErrExpiredRefreshToken):
			c.JSON(http.StatusUnauthorized, response.Unauthorized())
		default:
			c.JSON(http.StatusInternalServerError, response.InternalError("failed to refresh token"))
		}
		return
	}

	c.JSON(http.StatusOK, response.Success(result))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if h.authService == nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("auth service is not configured"))
		return
	}

	var req services.LogoutRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.authService.Logout(req.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to logout"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(nil, "logged out successfully"))
}

func authRequestMeta(c *gin.Context) services.AuthRequestMeta {
	return services.AuthRequestMeta{
		UserAgent: c.GetHeader("User-Agent"),
		IPAddress: c.ClientIP(),
	}
}
