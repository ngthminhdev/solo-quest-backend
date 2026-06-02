package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"solo_quest_backend/internal/database"
	"solo_quest_backend/internal/utils"
	"solo_quest_backend/pkg/logger"
)

func DevUserContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		devUserID := database.GetDevUserID()
		utils.SetCurrentUserID(c, devUserID)
		logger.L.Debug("dev user context set", zap.String("user_id", devUserID.String()))
		c.Next()
	}
}
