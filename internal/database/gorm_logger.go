package database

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"

	pkglogger "solo_quest_backend/pkg/logger"
)

type ZapGormLogger struct {
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration
}

func NewZapGormLogger(slowThresholdMs int) *ZapGormLogger {
	return &ZapGormLogger{
		LogLevel:      logger.Info,
		SlowThreshold: time.Duration(slowThresholdMs) * time.Millisecond,
	}
}

func (l *ZapGormLogger) LogMode(level logger.LogLevel) logger.Interface {
	return &ZapGormLogger{
		LogLevel:      level,
		SlowThreshold: l.SlowThreshold,
	}
}

func (l *ZapGormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		pkglogger.L.Info(msg,
			zap.String("source", "gorm"),
			zap.Any("data", data),
		)
	}
}

func (l *ZapGormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		pkglogger.L.Warn(msg,
			zap.String("source", "gorm"),
			zap.Any("data", data),
		)
	}
}

func (l *ZapGormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		pkglogger.L.Error(msg,
			zap.String("source", "gorm"),
			zap.Any("data", data),
		)
	}
}

func (l *ZapGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []zap.Field{
		zap.String("source", "gorm"),
		zap.String("file", utils.FileWithLineNum()),
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
		zap.String("sql", sql),
	}

	switch {
	case err != nil && l.LogLevel >= logger.Error:
		if err != logger.ErrRecordNotFound {
			fields = append(fields, zap.Error(err))
			pkglogger.L.Error("query failed", fields...)
		}
	case elapsed > l.SlowThreshold && l.LogLevel >= logger.Warn:
		fields = append(fields, zap.Duration("slow_threshold", l.SlowThreshold))
		pkglogger.L.Warn("slow query", fields...)
	case l.LogLevel >= logger.Info:
		pkglogger.L.Debug("query executed", fields...)
	}
}
