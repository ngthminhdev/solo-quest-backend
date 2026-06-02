package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	pkglogger "solo_quest_backend/pkg/logger"
)

var DB *gorm.DB

func Connect(databaseURL string, slowQueryThresholdMs int) error {
	gormLogger := NewZapGormLogger(slowQueryThresholdMs)

	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: gormLogger.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	pkglogger.L.Info("database connected successfully")
	return nil
}

func GetDB() *gorm.DB {
	return DB
}

func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
