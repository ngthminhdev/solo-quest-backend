package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/database"
	"solo_quest_backend/internal/infra/fcm"
	"solo_quest_backend/internal/middleware"
	"solo_quest_backend/internal/routes"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/cron"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/pkg/logger"
)

func main() {
	cfg := config.Load()

	logger.Init(cfg)
	defer logger.Sync()

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	if cfg.Logging.Format == "json" {
		gin.DefaultWriter = io.Discard
	}

	if err := database.Connect(cfg.DatabaseURL, cfg.Logging.SlowOperationMs); err != nil {
		logger.L.Fatal("failed to connect to database", zap.Error(err))
	}
	defer database.Close()

	// Run SQL migrations (source of truth for schema).
	// This replaces the old GORM AutoMigrate approach.
	// SQL migration files live in migrations/ and are versioned.
	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		logger.L.Fatal("failed to run database migrations", zap.Error(err))
	}
	logger.L.Info("database migrations completed")

	if cfg.AppEnv == "development" {
		db := database.GetDB()
		bootstrapService := services.NewBootstrapService(db)
		if err := bootstrapService.BootstrapDefaultDevUser(cfg.DevUserEmail); err != nil {
			logger.L.Fatal("failed to bootstrap default dev user", zap.Error(err))
		}
		logger.L.Info("default dev user bootstrap completed")
	}

	r := gin.New()

	r.Use(middleware.Recovery())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.CORS())

	// Initialize quest generation services
	db := database.GetDB()
	contextBuilder := quest_generation.NewUserQuestContextBuilder(db)
	ruleBasedGenerator := quest_generation.NewRuleBasedGenerator(db)
	aiCfg := ai.LoadConfig()

	var aiClient ai.Client
	if aiCfg.Enabled && aiCfg.APIKey != "" {
		client, err := ai.NewOpenAIClient(aiCfg)
		if err == nil {
			aiClient = client
		}
	}

	var aiGen quest_generation.Generator
	if aiClient != nil {
		aiGen = quest_generation.NewAIGeneratorWithConfig(aiClient, aiCfg)
	}

	generationService := quest_generation.NewGenerationService(db, contextBuilder, aiGen, ruleBasedGenerator)
	dailyCron := cron.NewDailyQuestCron(db, generationService, cfg)

	// Initialize FCM and services for notification cron
	fcmClient, _ := fcm.NewClient(cfg.FCM)
	deviceTokenService := services.NewDeviceTokenService(db)
	notificationSvc := services.NewNotificationService(fcmClient, deviceTokenService)
	notificationCron := cron.NewNotificationCron(db, notificationSvc, cfg.NotificationCron)

	routes.SetupRoutesWithCron(r, notificationCron, cfg)

	// Set up HTTP server
	srv := &http.Server{
		Addr:    "0.0.0.0:" + cfg.Port,
		Handler: r,
	}

	// Start cron if enabled
	cronCtx, cancelCron := context.WithCancel(context.Background())
	defer cancelCron()

	if cfg.Cron.DailyQuestEnabled {
		logger.L.Info("Daily quest cron is enabled")
		dailyCron.Start(cronCtx)
	} else {
		logger.L.Info("Daily quest cron is disabled")
	}

	if cfg.NotificationCron.Enabled {
		logger.L.Info("Notification cron is enabled")
		notificationCron.Start(cronCtx)
	} else {
		logger.L.Info("Notification cron is disabled")
	}

	// Start HTTP server in a goroutine so it doesn't block startup
	go func() {
		logger.L.Info("server starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for OS signals for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.L.Info("shutting down server...")

	// Stop cron scheduler
	if cfg.Cron.DailyQuestEnabled {
		dailyCron.Stop()
	}
	if cfg.NotificationCron.Enabled {
		notificationCron.Stop()
	}

	// Gracefully shutdown HTTP server with a 5-second timeout
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.L.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.L.Info("server exited cleanly")
}
