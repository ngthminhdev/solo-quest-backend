package testutils

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/services"
)

func CreateTestRouter(t *testing.T, db *gorm.DB, userID uuid.UUID) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	userService := services.NewUserService(db)
	devQuestGenerator := services.NewDevQuestGenerator(db)
	questService := services.NewQuestServiceWithDevGenerator(db, devQuestGenerator)
	logService := services.NewLogService(db)
	settingsService := services.NewSettingsService(db)
	reminderSettingService := services.NewReminderSettingService(db)
	onboardingService := services.NewOnboardingService(db)
	questActionService := services.NewQuestActionService(db)
	progressService := services.NewProgressService(db)
	checkinService := services.NewDailyCheckinService(db)
	reviewService := services.NewDailyReviewService(db)
	weeklySummaryService := services.NewWeeklySummaryService(db)
	questSettingsService := services.NewQuestSettingsService(db)
	scheduleBlockService := services.NewScheduleBlockService(db)
	learningRoadmapService := services.NewLearningRoadmapService(db)

	userHandler := handlers.NewUserHandlerWithDaily(userService, checkinService, reviewService)
	questHandler := handlers.NewQuestHandler(questService)
	logHandler := handlers.NewLogHandler(logService)
	settingsHandler := handlers.NewSettingsHandler(settingsService)
	reminderSettingsHandler := handlers.NewReminderSettingsHandler(reminderSettingService)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingService)
	questActionHandler := handlers.NewQuestActionHandler(questActionService)
	progressHandler := handlers.NewProgressHandler(progressService)
	checkinHandler := handlers.NewDailyCheckinHandler(checkinService)
	reviewHandler := handlers.NewDailyReviewHandler(reviewService)
	weeklySummaryHandler := handlers.NewWeeklySummaryHandler(weeklySummaryService)
	questSettingsHandler := handlers.NewQuestSettingsHandler(questSettingsService)
	scheduleBlockHandler := handlers.NewScheduleBlockHandler(scheduleBlockService)
	learningRoadmapHandler := handlers.NewLearningRoadmapHandler(learningRoadmapService)

	protected := r.Group("")
	protected.Use(TestUserContext(userID))
	{
		users := protected.Group("/api/users")
		{
			users.GET("/me", userHandler.GetMe)
			users.GET("/dev", userHandler.GetDevUser)
			users.GET("/me/daily-status", userHandler.GetDailyStatus)
		}

		onboarding := protected.Group("/api/onboarding")
		{
			onboarding.POST("", onboardingHandler.SaveOnboarding)
			onboarding.GET("/status", onboardingHandler.GetOnboardingStatus)
		}

		quests := protected.Group("/api/quests")
		{
			quests.GET("", questHandler.GetQuests)
			quests.POST("/:id/start", questActionHandler.StartQuest)
			quests.POST("/:id/complete", questActionHandler.CompleteQuest)
			quests.POST("/:id/skip", questActionHandler.SkipQuest)
			quests.POST("/:id/snooze", questActionHandler.SnoozeQuest)
		}

		progress := protected.Group("/api/progress")
		{
			progress.GET("", progressHandler.GetProgress)
			progress.GET("/weekly-chart", progressHandler.GetWeeklyChart)
			progress.GET("/xp-history", progressHandler.GetXPHistory)
		}

		checkins := protected.Group("/api/checkins")
		{
			checkins.GET("/today", checkinHandler.GetToday)
			checkins.GET("", checkinHandler.GetByDate)
			checkins.POST("", checkinHandler.Save)
		}

		reviews := protected.Group("/api/reviews")
		{
			reviews.GET("/today", reviewHandler.GetToday)
			reviews.GET("/summary", reviewHandler.GetSummary)
			reviews.GET("", reviewHandler.GetByDate)
			reviews.POST("", reviewHandler.Save)
		}

		logs := protected.Group("/api/logs")
		{
			logs.GET("", logHandler.GetLogs)
		}

		settings := protected.Group("/api/settings")
		{
			settings.GET("", settingsHandler.GetSettings)
			settings.PATCH("", settingsHandler.UpdateSettings)
			settings.GET("/reminders", reminderSettingsHandler.GetReminderSettings)
			settings.PATCH("/reminders/:type", reminderSettingsHandler.UpdateReminderSetting)
			settings.PATCH("/reminders/:type/toggle", reminderSettingsHandler.ToggleReminderSetting)
		}

		weeklySummary := protected.Group("/api/weekly-summary")
		{
			weeklySummary.GET("", weeklySummaryHandler.GetWeeklySummary)
		}

		questSettings := protected.Group("/api/quest-settings")
		{
			questSettings.GET("", questSettingsHandler.Get)
			questSettings.PUT("", questSettingsHandler.Update)
			questSettings.PATCH("", questSettingsHandler.Update)
			questSettings.POST("/reset", questSettingsHandler.Reset)
		}

		scheduleBlocks := protected.Group("/api/schedule-blocks")
		{
			scheduleBlocks.GET("", scheduleBlockHandler.List)
			scheduleBlocks.POST("", scheduleBlockHandler.Create)
			scheduleBlocks.PUT("/:id", scheduleBlockHandler.Update)
			scheduleBlocks.PATCH("/:id", scheduleBlockHandler.Patch)
			scheduleBlocks.DELETE("/:id", scheduleBlockHandler.Delete)
		}

		learningRoadmaps := protected.Group("/api/learning-roadmaps")
		{
			learningRoadmaps.POST("/suggest", learningRoadmapHandler.Suggest)
			learningRoadmaps.POST("/ai-suggest", learningRoadmapHandler.AiSuggest)
			learningRoadmaps.POST("/generate", learningRoadmapHandler.Generate)
			learningRoadmaps.GET("/generate/status", learningRoadmapHandler.GetGenerateStatus)
			learningRoadmaps.POST("", learningRoadmapHandler.CreateFromTemplate)
			learningRoadmaps.POST("/create", learningRoadmapHandler.Create)
			learningRoadmaps.GET("", learningRoadmapHandler.List)
			learningRoadmaps.GET("/:id", learningRoadmapHandler.GetDetail)
			learningRoadmaps.DELETE("/:id", learningRoadmapHandler.Delete)
			learningRoadmaps.POST("/:id/follow", learningRoadmapHandler.Follow)
			learningRoadmaps.PATCH("/:id/steps/:step_id", learningRoadmapHandler.ToggleStep)
		}
	}

	return r
}

func TestUserContext(userID uuid.UUID) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("currentUserID", userID)
		c.Next()
	}
}

func AuthMiddleware(userID uuid.UUID) gin.HandlerFunc {
	return TestUserContext(userID)
}
