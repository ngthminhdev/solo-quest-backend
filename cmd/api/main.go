package main

import (
	"io"
	"log"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/database"
	"solo_quest_backend/internal/middleware"
	"solo_quest_backend/internal/routes"
	"solo_quest_backend/internal/services"
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

	routes.SetupRoutes(r)

	logger.L.Info("server starting", zap.String("port", cfg.Port))
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
