package logger

import (
	"context"
	"time"

	"go.uber.org/zap"
)

var slowOperationThresholdMs int

func SetSlowOperationThreshold(ms int) {
	slowOperationThresholdMs = ms
}

type OperationTrace struct {
	ctx       context.Context
	name      string
	startTime time.Time
	fields    []zap.Field
}

func StartOperation(ctx context.Context, name string, fields ...zap.Field) *OperationTrace {
	return &OperationTrace{
		ctx:       ctx,
		name:      name,
		startTime: time.Now(),
		fields:    fields,
	}
}

func (t *OperationTrace) End(extraFields ...zap.Field) {
	duration := time.Since(t.startTime)
	durationMs := duration.Milliseconds()

	allFields := make([]zap.Field, 0, len(t.fields)+len(extraFields)+3)
	allFields = append(allFields, t.fields...)
	allFields = append(allFields, extraFields...)
	allFields = append(allFields,
		zap.String("outcome", "completed"),
		zap.Int64("duration_ms", durationMs),
	)

	if slowOperationThresholdMs > 0 && durationMs >= int64(slowOperationThresholdMs) {
		WarnContext(t.ctx, "slow operation detected", allFields...)
	}

	DebugContext(t.ctx, t.name, allFields...)
}

func (t *OperationTrace) Skip(reason string, extraFields ...zap.Field) {
	duration := time.Since(t.startTime)
	durationMs := duration.Milliseconds()

	allFields := make([]zap.Field, 0, len(t.fields)+len(extraFields)+4)
	allFields = append(allFields, t.fields...)
	allFields = append(allFields, extraFields...)
	allFields = append(allFields,
		zap.String("outcome", "skipped"),
		zap.String("reason", reason),
		zap.Int64("duration_ms", durationMs),
	)

	DebugContext(t.ctx, t.name, allFields...)
}

func (t *OperationTrace) Fail(err error, extraFields ...zap.Field) {
	duration := time.Since(t.startTime)
	durationMs := duration.Milliseconds()

	allFields := make([]zap.Field, 0, len(t.fields)+len(extraFields)+4)
	allFields = append(allFields, t.fields...)
	allFields = append(allFields, extraFields...)
	allFields = append(allFields,
		zap.String("outcome", "failed"),
		zap.Int64("duration_ms", durationMs),
		zap.Error(err),
	)

	ErrorContext(t.ctx, t.name, allFields...)
}
