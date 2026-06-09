package database

import (
	"solo_quest_backend/internal/models"

	"gorm.io/gorm"
)

var coreModels = []interface{}{
	&models.UserProfile{},
	&models.AuthAccount{},
	&models.UserSession{},
	&models.OnboardingAnswer{},
	&models.Quest{},
	&models.QuestAction{},
	&models.DailyCheckin{},
	&models.DailyReview{},
	&models.LogEntry{},
	&models.XPTransaction{},
	&models.Reward{},
	&models.RewardRedemption{},
	&models.AppSettings{},
	&models.LearningRoadmap{},
	&models.LearningRoadmapStep{},
	&models.UserLearningRoadmap{},
	&models.UserLearningRoadmapStepProgress{},
	&models.ScheduleBlock{},
	&models.DailyQuestGenerationJob{},
}

// DeprecatedAutoMigrate runs GORM AutoMigrate for all core models.
//
// DEPRECATED: SQL migrations (migrations/*.sql) are now the source of truth
// for schema changes. Do NOT call this in normal application startup.
//
// This function is kept only for emergency/dev-only prototyping where
// someone needs to quickly test a model change before writing a proper
// SQL migration. It must never be called in production or CI.
//
// Use RunMigrations() or `go run cmd/migrate/main.go up` instead.
func DeprecatedAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(coreModels...)
}

func GetCoreModelNames() []string {
	return []string{
		"UserProfile",
		"AuthAccount",
		"UserSession",
		"OnboardingAnswer",
		"Quest",
		"QuestAction",
		"DailyCheckin",
		"DailyReview",
		"LogEntry",
		"XPTransaction",
		"Reward",
		"RewardRedemption",
		"AppSettings",
		"LearningRoadmap",
		"LearningRoadmapStep",
		"UserLearningRoadmap",
		"UserLearningRoadmapStepProgress",
		"ScheduleBlock",
		"DailyQuestGenerationJob",
	}
}
