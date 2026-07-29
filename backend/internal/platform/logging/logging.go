package logging

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	platformauth "github.com/interviewmaster/interviewmaster/backend/internal/platform/auth"
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

func HTTPMiddleware(jwtSecret string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r = requestid.Ensure(w, r)
			writer := &auditResponseWriter{ResponseWriter: w}
			startedAt := time.Now()
			next(writer, r)

			attributes := []any{
				"request_id", requestid.FromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", writer.statusCode(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			}
			if userID := accessTokenUserID(r, jwtSecret); userID != "" {
				attributes = append(attributes, "user_id", userID)
			}
			if resourceIDs := pathResourceIDs(r.URL.Path); len(resourceIDs) > 0 {
				attributes = append(attributes, "resource_ids", resourceIDs)
			}
			if code := writer.errorCode(); code != "" {
				attributes = append(attributes, "error_code", code)
			}
			slog.InfoContext(r.Context(), "http request completed", attributes...)
		}
	}
}

type auditResponseWriter struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (w *auditResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	const maxAuditBody = 4096
	if len(w.body) < maxAuditBody {
		remaining := maxAuditBody - len(w.body)
		if len(body) < remaining {
			remaining = len(body)
		}
		w.body = append(w.body, body[:remaining]...)
	}
	return w.ResponseWriter.Write(body)
}

func (w *auditResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *auditResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *auditResponseWriter) errorCode() string {
	if w.statusCode() < http.StatusBadRequest {
		return ""
	}
	var payload struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.body, &payload)
	return strings.TrimSpace(payload.Code)
}

func accessTokenUserID(r *http.Request, secret string) string {
	const bearer = "Bearer "
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(value, bearer) {
		return ""
	}
	return platformauth.UserIDFromToken(strings.TrimSpace(strings.TrimPrefix(value, bearer)), secret)
}

func pathResourceIDs(path string) []string {
	result := make([]string, 0, 2)
	for _, part := range strings.Split(path, "/") {
		if _, err := uuid.Parse(part); err == nil {
			result = append(result, part)
		}
	}
	return result
}
