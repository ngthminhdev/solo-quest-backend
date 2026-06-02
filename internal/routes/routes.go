package routes

import (
	"solo_quest_backend/internal/database"
	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/middleware"
	"solo_quest_backend/internal/services"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	db := database.GetDB()

	userService := services.NewUserService(db)
	questService := services.NewQuestService(db)
	rewardService := services.NewRewardService(db)
	logService := services.NewLogService(db)
	settingsService := services.NewSettingsService(db)
	onboardingService := services.NewOnboardingService(db)
	questActionService := services.NewQuestActionService(db)
	progressService := services.NewProgressService(db)
	checkinService := services.NewDailyCheckinService(db)
	reviewService := services.NewDailyReviewService(db)

	authHandler := handlers.NewAuthHandler(userService)
	userHandler := handlers.NewUserHandlerWithDaily(userService, checkinService, reviewService)
	questHandler := handlers.NewQuestHandler(questService)
	rewardHandler := handlers.NewRewardHandler(rewardService)
	logHandler := handlers.NewLogHandler(logService)
	settingsHandler := handlers.NewSettingsHandler(settingsService)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingService)
	questActionHandler := handlers.NewQuestActionHandler(questActionService)
	progressHandler := handlers.NewProgressHandler(progressService)
	checkinHandler := handlers.NewDailyCheckinHandler(checkinService)
	reviewHandler := handlers.NewDailyReviewHandler(reviewService)

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
		}

		// Protected routes with dev user context
		protected := api.Group("")
		protected.Use(middleware.DevUserContext())
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
			}
		}
	}
}
