package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/utils"
)

type AccessTokenValidator interface {
	ValidateAccessToken(token string) (uuid.UUID, uuid.UUID, error)
}

func JWTAuth(validator AccessTokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, response.Unauthorized())
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, response.Unauthorized())
			c.Abort()
			return
		}

		if validator == nil {
			c.JSON(http.StatusUnauthorized, response.Unauthorized())
			c.Abort()
			return
		}
		userID, sessionID, err := validator.ValidateAccessToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.Unauthorized())
			c.Abort()
			return
		}

		utils.SetCurrentUserID(c, userID)
		if sessionID != uuid.Nil {
			c.Set("sessionID", sessionID)
		}
		c.Next()
	}
}

func CurrentUserContext(cfg *config.Config, validator AccessTokenValidator) gin.HandlerFunc {
	_ = cfg
	return JWTAuth(validator)
}
