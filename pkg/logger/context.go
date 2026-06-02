package logger

import (
	"context"

	"go.uber.org/zap"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
)

func extractContextFields(ctx context.Context) []zap.Field {
	var fields []zap.Field

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		fields = append(fields, zap.String("user_id", userID))
	}

	return fields
}

func mergeFields(ctxFields, explicitFields []zap.Field) []zap.Field {
	seen := make(map[string]bool)
	var result []zap.Field

	for _, f := range explicitFields {
		seen[f.Key] = true
		result = append(result, f)
	}

	for _, f := range ctxFields {
		if !seen[f.Key] {
			result = append(result, f)
		}
	}

	return result
}

func WithContext(ctx context.Context) *zap.Logger {
	ctxFields := extractContextFields(ctx)
	if len(ctxFields) == 0 {
		return L
	}
	return L.With(ctxFields...)
}

func DebugContext(ctx context.Context, msg string, fields ...zap.Field) {
	ctxFields := extractContextFields(ctx)
	L.Debug(msg, mergeFields(ctxFields, fields)...)
}

func InfoContext(ctx context.Context, msg string, fields ...zap.Field) {
	ctxFields := extractContextFields(ctx)
	L.Info(msg, mergeFields(ctxFields, fields)...)
}

func WarnContext(ctx context.Context, msg string, fields ...zap.Field) {
	ctxFields := extractContextFields(ctx)
	L.Warn(msg, mergeFields(ctxFields, fields)...)
}

func ErrorContext(ctx context.Context, msg string, fields ...zap.Field) {
	ctxFields := extractContextFields(ctx)
	L.Error(msg, mergeFields(ctxFields, fields)...)
}

func DetachedContext(ctx context.Context) context.Context {
	detached := context.Background()

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		detached = context.WithValue(detached, RequestIDKey, requestID)
	}

	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		detached = context.WithValue(detached, UserIDKey, userID)
	}

	return detached
}
