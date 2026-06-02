package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/services"
)

type AuthHandler struct {
	userService *services.UserService
}

func NewAuthHandler(userService *services.UserService) *AuthHandler {
	return &AuthHandler{userService: userService}
}

// DevLogin is a local development only endpoint.
// WARNING: Do not use in production.
func (h *AuthHandler) DevLogin(c *gin.Context) {
	user, err := h.userService.GetDevUser()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "dev user not found",
		})
		return
	}

	// TODO: replace with real JWT tokens in production
	c.JSON(http.StatusOK, gin.H{
		"user":          user,
		"access_token":  "dev-token",
		"refresh_token": "dev-refresh-token",
		"token_type":    "Bearer",
	})
}
