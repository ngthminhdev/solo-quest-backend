package handlers

import (
	"net/http"
	"solo_quest_backend/internal/database"

	"github.com/gin-gonic/gin"
)

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "soloquest-backend",
	})
}

func DatabaseHealthCheck(c *gin.Context) {
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"database": "disconnected",
			"message":  "database connection not initialized",
		})
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"database": "disconnected",
			"message":  err.Error(),
		})
		return
	}

	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"database": "disconnected",
			"message":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"database": "connected",
	})
}

func ModelsHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"models": database.GetCoreModelNames(),
	})
}

func Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "api ready",
	})
}

func AuthPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "auth route ready",
	})
}

func UsersPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "users route ready",
	})
}

func QuestsPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "quests route ready",
	})
}

func CheckinsPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "checkins route ready",
	})
}

func ReviewsPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "reviews route ready",
	})
}

func ProgressPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "progress route ready",
	})
}

func RewardsPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "rewards route ready",
	})
}

func LogsPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "logs route ready",
	})
}

func SettingsPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "settings route ready",
	})
}
