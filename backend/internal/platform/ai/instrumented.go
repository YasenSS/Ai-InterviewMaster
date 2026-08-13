package ai

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Pricing estimates USD micros from token usage. Zero prices record tokens only.
type Pricing struct {
	PromptMicrosPer1k     int64
	CompletionMicrosPer1k int64
}

func EstimateCost(usage Usage, pricing Pricing) int64 {
	return int64(usage.PromptTokens)*pricing.PromptMicrosPer1k/1000 +
		int64(usage.CompletionTokens)*pricing.CompletionMicrosPer1k/1000
}

// InstrumentedChatModel applies quota, transport retry, and invocation audit
// around a provider ChatModel. Domain packages still depend only on ChatModel.
type InstrumentedChatModel struct {
	Primary      ChatModel
	Small        ChatModel
	Store        InvocationStore
	Limiter      *Limiter
	Retry        RetryingChatModel
	Pricing      Pricing
	BlockOnAudit bool
}

func NewInstrumented(primary, small ChatModel, store InvocationStore, limiter *Limiter, pricing Pricing, blockOnAudit bool) ChatModel {
	if small == nil {
		small = primary
	}
	return InstrumentedChatModel{
		Primary:      primary,
		Small:        small,
		Store:        store,
		Limiter:      limiter,
		Retry:        RetryingChatModel{Inner: primary},
		Pricing:      pricing,
		BlockOnAudit: blockOnAudit,
	}
}

func (m InstrumentedChatModel) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	inputRunes := 0
	for _, message := range req.Messages {
		inputRunes += utf8.RuneCountInString(message.Content)
	}
	reserved, err := m.Limiter.acquire(ctx, req.UserID, inputRunes)
	if err != nil {
		if IsErrorCode(err, ErrorBudgetExhausted) {
			RecordBudgetRejected()
		}
		return GenerateResponse{}, err
	}
	inner := m.Retry
	if req.PreferSmall || reserved.soft {
		inner.Inner = m.Small
		req.PreferSmall = true
	} else {
		inner.Inner = m.Primary
	}

	started := time.Now()
	response, callErr := inner.Generate(ctx, req)
	finished := time.Now()
	cost := EstimateCost(response.Usage, m.Pricing)
	m.Limiter.release(ctx, reserved, cost)
	degraded := req.PreferSmall || reserved.soft
	RecordGenerate(callErr, response.Usage, cost, latencyMillis(started, finished), degraded)

	traceID, requestID, inputHash := invocationMeta(ctx, req)
	rec := Invocation{
		ID:                  uuid.NewString(),
		UserID:              req.UserID,
		TaskID:              req.TaskID,
		SessionID:           req.SessionID,
		ResourceType:        req.ResourceType,
		ResourceID:          req.ResourceID,
		Provider:            response.Provider,
		Model:               response.Model,
		PromptKey:           req.PromptKey,
		PromptVersion:       req.PromptVersion,
		Status:              "succeeded",
		Attempt:             1,
		InputHash:           inputHash,
		PromptTokens:        response.Usage.PromptTokens,
		CompletionTokens:    response.Usage.CompletionTokens,
		TotalTokens:         response.Usage.TotalTokens,
		EstimatedCostMicros: cost,
		LatencyMS:           int(finished.Sub(started).Milliseconds()),
		TraceID:             traceID,
		RequestID:           requestID,
		CreatedAt:           started,
		CompletedAt:         &finished,
	}
	if strings.TrimSpace(response.Message.Content) != "" {
		rec.OutputHash = hashText(response.Message.Content)
	}
	if rec.Provider == "" && callErr == nil {
		rec.Provider = "unknown"
	}
	if callErr != nil {
		rec.Status = "failed"
		var modelErr *Error
		if errors.As(callErr, &modelErr) {
			rec.ErrorCode = string(modelErr.Code)
			rec.Provider = firstNonEmpty(rec.Provider)
		} else {
			rec.ErrorCode = string(ErrorProviderUnavailable)
		}
	}
	if m.Store != nil {
		if err := m.Store.Write(ctx, rec); err != nil {
			logAuditFailure(ctx, err)
			if m.BlockOnAudit {
				return GenerateResponse{}, err
			}
		}
	}
	return response, callErr
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
