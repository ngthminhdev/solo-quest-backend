package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"solo_quest_backend/internal/observability"
	"solo_quest_backend/pkg/logger"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		ctx := context.WithValue(c.Request.Context(), logger.RequestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		var capture *observability.RequestBodyCapture
		if c.Request.Body != nil && c.Request.Method != "GET" {
			capture = observability.NewRequestBodyCapture(c.Request.Body)
			c.Request.Body = capture
		}

		start := time.Now()

		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("client_ip", c.ClientIP()),
		}

		logger.L.Info("http request started", fields...)

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		completionFields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("client_ip", c.ClientIP()),
			zap.Int("status", status),
			zap.Int("body_bytes", c.Writer.Size()),
			zap.Duration("duration", duration),
			zap.Int64("duration_ms", duration.Milliseconds()),
		}

		if len(c.Errors) > 0 {
			completionFields = append(completionFields, zap.String("errors", c.Errors.String()))
		}

		switch {
		case status >= 500:
			logger.L.Error("http request completed", completionFields...)
		case status >= 400:
			logger.L.Warn("http request completed", completionFields...)
		default:
			logger.L.Info("http request completed", completionFields...)
		}
	}
}
