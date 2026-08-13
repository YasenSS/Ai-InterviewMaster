package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/requestid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
)

// Invocation is one model attempt written to model_invocations. Bodies are
// never stored; only hashes, tokens, cost, and error codes are kept.
type Invocation struct {
	ID                  string
	UserID              string
	TaskID              string
	SessionID           string
	ResourceType        string
	ResourceID          string
	Provider            string
	Model               string
	PromptKey           string
	PromptVersion       string
	Status              string
	Attempt             int
	InputHash           string
	OutputHash          string
	PromptTokens        int
	CompletionTokens    int
	TotalTokens         int
	EstimatedCostMicros int64
	LatencyMS           int
	ErrorCode           string
	TraceID             string
	RequestID           string
	CreatedAt           time.Time
	CompletedAt         *time.Time
}

// InvocationStore persists audit rows.
type InvocationStore interface {
	Write(ctx context.Context, rec Invocation) error
}

// PostgresInvocations writes to model_invocations.
type PostgresInvocations struct {
	DB *pgxpool.Pool
}

func (s PostgresInvocations) Write(ctx context.Context, rec Invocation) error {
	if s.DB == nil {
		return nil
	}
	if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	if rec.Attempt < 1 {
		rec.Attempt = 1
	}
	_, err := s.DB.Exec(ctx, `
		INSERT INTO model_invocations (
			id, user_id, task_id, session_id, resource_type, resource_id,
			provider, model, prompt_key, prompt_version, status, attempt,
			input_hash, output_hash, prompt_tokens, completion_tokens, total_tokens,
			estimated_cost_micros, latency_ms, error_code, trace_id, request_id,
			created_at, completed_at
		) VALUES (
			$1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5,NULLIF($6,'')::uuid,
			$7,$8,$9,$10,$11,$12,
			$13,$14,$15,$16,$17,
			$18,$19,NULLIF($20,''),NULLIF($21,''),NULLIF($22,''),
			$23,$24
		)`,
		rec.ID,
		rec.UserID,
		rec.TaskID,
		rec.SessionID,
		nullString(rec.ResourceType, "unknown"),
		rec.ResourceID,
		rec.Provider,
		rec.Model,
		rec.PromptKey,
		rec.PromptVersion,
		rec.Status,
		rec.Attempt,
		rec.InputHash,
		rec.OutputHash,
		rec.PromptTokens,
		rec.CompletionTokens,
		rec.TotalTokens,
		rec.EstimatedCostMicros,
		rec.LatencyMS,
		rec.ErrorCode,
		rec.TraceID,
		rec.RequestID,
		rec.CreatedAt,
		rec.CompletedAt,
	)
	return err
}

// MemoryInvocations stores rows in process for tests.
type MemoryInvocations struct {
	Rows []Invocation
}

func (s *MemoryInvocations) Write(_ context.Context, rec Invocation) error {
	s.Rows = append(s.Rows, rec)
	return nil
}

func hashText(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\n")))
	return hex.EncodeToString(sum[:])
}

func invocationMeta(ctx context.Context, req GenerateRequest) (traceID, requestID, inputHash string) {
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		traceID = span.TraceID().String()
	}
	requestID = requestid.FromContext(ctx)
	parts := make([]string, 0, len(req.Messages)+2)
	parts = append(parts, req.PromptKey, req.PromptVersion)
	for _, message := range req.Messages {
		parts = append(parts, string(message.Role)+":"+message.Content)
	}
	return traceID, requestID, hashText(parts...)
}

func logAuditFailure(ctx context.Context, err error) {
	slog.ErrorContext(ctx, "model invocation audit write failed", "error", err)
}

func nullString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
