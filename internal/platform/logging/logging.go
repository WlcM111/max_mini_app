// Package logging настраивает структурированные логи (slog, JSON в stdout)
// с обязательными полями раздела 1 docs/architecture/observability.md.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyTraceID
	keySpanID
	keyOperation
)

// New возвращает логгер сервиса с постоянными полями service и version.
func New(level, service, version string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Key = "time"
			case slog.LevelKey:
				a.Key = "level"
			}
			return a
		},
	})
	return slog.New(h).With(slog.String("service", service), slog.String("version", version))
}

// WithCorrelation кладёт идентификаторы корреляции в контекст.
func WithCorrelation(ctx context.Context, requestID, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, keyRequestID, requestID)
	ctx = context.WithValue(ctx, keyTraceID, traceID)
	return context.WithValue(ctx, keySpanID, spanID)
}

// WithOperation помечает контекст именем операции (RPC или фоновой задачи).
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, keyOperation, operation)
}

func str(ctx context.Context, k ctxKey) string {
	if v, ok := ctx.Value(k).(string); ok {
		return v
	}
	return ""
}

// From возвращает логгер с полями корреляции из контекста.
func From(ctx context.Context, base *slog.Logger) *slog.Logger {
	l := base
	if v := str(ctx, keyRequestID); v != "" {
		l = l.With(slog.String("request_id", v))
	}
	if v := str(ctx, keyTraceID); v != "" {
		l = l.With(slog.String("trace_id", v))
	}
	if v := str(ctx, keySpanID); v != "" {
		l = l.With(slog.String("span_id", v))
	}
	if v := str(ctx, keyOperation); v != "" {
		l = l.With(slog.String("operation", v))
	}
	return l
}

// RequestID возвращает идентификатор запроса из контекста.
func RequestID(ctx context.Context) string { return str(ctx, keyRequestID) }

// TraceIDFromContext возвращает идентификатор трассировки для исходящих вызовов.
func TraceIDFromContext(ctx context.Context) string { return str(ctx, keyTraceID) }
