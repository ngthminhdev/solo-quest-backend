package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/database"
	"solo_quest_backend/internal/pkg/response"
)

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{
		"status":  "ok",
		"service": "solo_quest_backend",
	}))
}

func DatabaseHealthCheck(c *gin.Context) {
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "database connection not initialized"))
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
		return
	}

	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{
		"status":   "ok",
		"database": "connected",
	}))
}

func ModelsHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{
		"status": "ok",
		"models": database.GetCoreModelNames(),
	}))
}

func Ping(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "api ready"}))
}

func AuthPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "auth route ready"}))
}

func UsersPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "users route ready"}))
}

func QuestsPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "quests route ready"}))
}

func CheckinsPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "checkins route ready"}))
}

func ReviewsPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "reviews route ready"}))
}

func ProgressPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "progress route ready"}))
}

func RewardsPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "rewards route ready"}))
}

func LogsPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "logs route ready"}))
}

func SettingsPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "settings route ready"}))
}

func WeeklySummaryPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "weekly-summary route ready"}))
}

func QuestSettingsPing(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{"message": "quest-settings route ready"}))
}
