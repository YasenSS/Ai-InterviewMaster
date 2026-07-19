package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/internal/platform/requestid"
	"go.opentelemetry.io/otel/trace"
)

func Setup(level string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
	slog.SetDefault(logger)
	return logger
}

func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if id := requestid.FromContext(ctx); id != "" {
		logger = logger.With("request_id", id)
	}
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		logger = logger.With("trace_id", spanContext.TraceID().String(), "span_id", spanContext.SpanID().String())
	}
	return logger
}
