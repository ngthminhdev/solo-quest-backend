package routes

import (
	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/database"
	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/infra/fcm"
	"solo_quest_backend/internal/middleware"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/cron"
	"solo_quest_backend/internal/services/quest_generation"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, cfgOpt ...*config.Config) {
	SetupRoutesWithCron(r, nil, cfgOpt...)
}

func SetupRoutesWithCron(r *gin.Engine, notificationCron *cron.NotificationCron, cfgOpt ...*config.Config) {
	var cfg *config.Config
	if len(cfgOpt) > 0 {
		cfg = cfgOpt[0]
	}
	if cfg == nil {
		cfg = config.Load()
	}

	db := database.GetDB()

	userService := services.NewUserService(db)
	authService := services.NewAuthService(db, cfg, nil)
	devQuestGenerator := services.NewDevQuestGenerator(db)
	questService := services.NewQuestServiceWithDevGenerator(db, devQuestGenerator)
	rewardService := services.NewRewardService(db)
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

	// AI services
	aiCfg := ai.LoadConfig()
	var aiClient ai.Client
	if aiCfg.Enabled {
		client, err := ai.NewClient(aiCfg)
		if err == nil {
			aiClient = client
		}
	}

	// Quest generation services
	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	ruleBasedGenerator := quest_generation.NewRuleBasedGenerator(db)
	var aiGen quest_generation.Generator
	if aiClient != nil {
		aiGen = quest_generation.NewAIGeneratorWithConfig(aiClient, aiCfg)
	}
	questPreviewHandler := handlers.NewQuestPreviewHandler(contextBuilder, ruleBasedGenerator, aiClient, aiCfg, aiCfg.Enabled)

	generationService := quest_generation.NewGenerationService(db, contextBuilder, aiGen, ruleBasedGenerator)
	questGenerationHandler := handlers.NewQuestGenerationHandler(generationService)

	authHandler := handlers.NewAuthHandler(userService, authService)
	userHandler := handlers.NewUserHandlerWithDaily(userService, checkinService, reviewService)
	questHandler := handlers.NewQuestHandler(questService)
	rewardHandler := handlers.NewRewardHandler(rewardService)
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

	deviceTokenService := services.NewDeviceTokenService(db)
	fcmClient, _ := fcm.NewClient(cfg.FCM)
	notificationSvc := services.NewNotificationService(fcmClient, deviceTokenService)
	notificationHandler := handlers.NewNotificationHandler(deviceTokenService, notificationSvc)
	if notificationCron != nil {
		notificationHandler.SetNotificationCron(notificationCron)
	}

	// Health check
	r.GET("/health", handlers.HealthCheck)
	r.GET("/health/db", handlers.DatabaseHealthCheck)
	r.GET("/health/models", handlers.ModelsHealthCheck)

	// API group
	api := r.Group("/api")
	{
		api.GET("/ping", handlers.Ping)

		// Auth routes (no user context needed)
		auth := api.Group("/auth")
		{
			auth.GET("/ping", handlers.AuthPing)
			auth.POST("/dev-login", authHandler.DevLogin)
			auth.POST("/google", authHandler.GoogleLogin)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.JWTAuth(authService.JWTManager()), authHandler.Me)
		}

		// Protected routes keep dev user context in development/test and use JWT in production.
		protected := api.Group("")
		protected.Use(middleware.CurrentUserContext(cfg, authService.JWTManager()))
		{
			// Users routes
			users := protected.Group("/users")
			{
				users.GET("/ping", handlers.UsersPing)
				users.GET("/me", userHandler.GetMe)
				users.GET("/dev", userHandler.GetDevUser)
				users.GET("/me/daily-status", userHandler.GetDailyStatus)
			}

			// Onboarding routes
			onboarding := protected.Group("/onboarding")
			{
				onboarding.POST("", onboardingHandler.SaveOnboarding)
				onboarding.GET("/status", onboardingHandler.GetOnboardingStatus)
			}

			// Quests routes
			quests := protected.Group("/quests")
			{
				quests.GET("/ping", handlers.QuestsPing)
				quests.GET("", questHandler.GetQuests)
				quests.POST("/generate-preview", questPreviewHandler.GeneratePreview)
				quests.POST("/generate-today", questGenerationHandler.GenerateToday)
				quests.GET("/generate-today/status", questGenerationHandler.GetTodayGenerationStatus)
				quests.POST("/:id/start", questActionHandler.StartQuest)
				quests.POST("/:id/complete", questActionHandler.CompleteQuest)
				quests.POST("/:id/skip", questActionHandler.SkipQuest)
				quests.POST("/:id/snooze", questActionHandler.SnoozeQuest)
			}

			// Checkins routes
			checkins := protected.Group("/checkins")
			{
				checkins.GET("/ping", handlers.CheckinsPing)
				checkins.GET("/today", checkinHandler.GetToday)
				checkins.GET("", checkinHandler.GetByDate)
				checkins.POST("", checkinHandler.Save)
			}

			// Reviews routes
			reviews := protected.Group("/reviews")
			{
				reviews.GET("/ping", handlers.ReviewsPing)
				reviews.GET("/today", reviewHandler.GetToday)
				reviews.GET("/summary", reviewHandler.GetSummary)
				reviews.GET("", reviewHandler.GetByDate)
				reviews.POST("", reviewHandler.Save)
			}

			// Progress routes
			progress := protected.Group("/progress")
			{
				progress.GET("/ping", handlers.ProgressPing)
				progress.GET("", progressHandler.GetProgress)
				progress.GET("/weekly-chart", progressHandler.GetWeeklyChart)
				progress.GET("/xp-history", progressHandler.GetXPHistory)
			}

			// Rewards routes
			rewards := protected.Group("/rewards")
			{
				rewards.GET("/ping", handlers.RewardsPing)
				rewards.GET("", rewardHandler.GetRewards)
				rewards.POST("", rewardHandler.CreateReward)
				rewards.PATCH("/:id", rewardHandler.UpdateReward)
				rewards.DELETE("/:id", rewardHandler.DeleteReward)
				rewards.GET("/redemptions", rewardHandler.GetRedemptions)
				rewards.POST("/:id/claim", rewardHandler.ClaimReward)
			}

			// Logs routes
			logs := protected.Group("/logs")
			{
				logs.GET("/ping", handlers.LogsPing)
				logs.GET("", logHandler.GetLogs)
			}

			// Settings routes
			settings := protected.Group("/settings")
			{
				settings.GET("/ping", handlers.SettingsPing)
				settings.GET("", settingsHandler.GetSettings)
				settings.PATCH("", settingsHandler.UpdateSettings)
				settings.GET("/reminders", reminderSettingsHandler.GetReminderSettings)
				settings.PATCH("/reminders/:type", reminderSettingsHandler.UpdateReminderSetting)
				settings.PATCH("/reminders/:type/toggle", reminderSettingsHandler.ToggleReminderSetting)
			}

			// Weekly Summary routes
			weeklySummary := protected.Group("/weekly-summary")
			{
				weeklySummary.GET("/ping", handlers.WeeklySummaryPing)
				weeklySummary.GET("", weeklySummaryHandler.GetWeeklySummary)
			}

			// Quest Settings routes
			questSettings := protected.Group("/quest-settings")
			{
				questSettings.GET("/ping", handlers.QuestSettingsPing)
				questSettings.GET("", questSettingsHandler.Get)
				questSettings.PUT("", questSettingsHandler.Update)
				questSettings.PATCH("", questSettingsHandler.Update)
				questSettings.POST("/reset", questSettingsHandler.Reset)
			}

			// Schedule Blocks routes
			scheduleBlocks := protected.Group("/schedule-blocks")
			{
				scheduleBlocks.GET("", scheduleBlockHandler.List)
				scheduleBlocks.POST("", scheduleBlockHandler.Create)
				scheduleBlocks.PUT("/:id", scheduleBlockHandler.Update)
				scheduleBlocks.PATCH("/:id", scheduleBlockHandler.Patch)
				scheduleBlocks.DELETE("/:id", scheduleBlockHandler.Delete)
			}

			// Learning Roadmaps routes
			learningRoadmaps := protected.Group("/learning-roadmaps")
			{
				learningRoadmaps.POST("/suggest", learningRoadmapHandler.Suggest)
				learningRoadmaps.POST("/ai-suggest", learningRoadmapHandler.AiSuggest)
				learningRoadmaps.POST("", learningRoadmapHandler.CreateFromTemplate)
				learningRoadmaps.POST("/create", learningRoadmapHandler.Create)
				learningRoadmaps.GET("", learningRoadmapHandler.List)
				learningRoadmaps.GET("/:id", learningRoadmapHandler.GetDetail)
				learningRoadmaps.POST("/:id/follow", learningRoadmapHandler.Follow)
				learningRoadmaps.PATCH("/:id/steps/:step_id", learningRoadmapHandler.ToggleStep)
			}

			notifications := protected.Group("/notifications")
			{
				notifications.POST("/tokens", notificationHandler.RegisterToken)
				notifications.GET("/tokens", notificationHandler.GetMyTokens)
				notifications.DELETE("/tokens/:id", notificationHandler.RemoveToken)
				notifications.POST("/test", notificationHandler.SendTest)
				notifications.POST("/run-due-once", notificationHandler.RunDueOnce)
			}
		}
	}
}
