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
	questService := services.NewQuestService(db)
	rewardService := services.NewRewardService(db)
	logService := services.NewLogService(db)
	settingsService := services.NewSettingsService(db)
	onboardingService := services.NewOnboardingService(db)
	questActionService := services.NewQuestActionService(db)
	progressService := services.NewProgressService(db)
	checkinService := services.NewDailyCheckinService(db)
	reviewService := services.NewDailyReviewService(db)

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

		rewards := protected.Group("/api/rewards")
		{
			rewards.GET("", rewardHandler.GetRewards)
			rewards.GET("/redemptions", rewardHandler.GetRedemptions)
			rewards.POST("/:id/claim", rewardHandler.ClaimReward)
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
