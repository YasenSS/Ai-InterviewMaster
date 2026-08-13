package ai

import (
	"errors"
	"fmt"
)

// ErrorCode classifies model failures without leaking provider response bodies
// into API responses.
type ErrorCode string

const (
	ErrorInvalidRequest      ErrorCode = "AI_INVALID_REQUEST"
	ErrorNotConfigured       ErrorCode = "AI_NOT_CONFIGURED"
	ErrorAuthentication      ErrorCode = "AI_AUTHENTICATION_FAILED"
	ErrorRateLimited         ErrorCode = "AI_RATE_LIMITED"
	ErrorProviderUnavailable ErrorCode = "AI_PROVIDER_UNAVAILABLE"
	ErrorTimeout             ErrorCode = "AI_TIMEOUT"
	ErrorOutputInvalid       ErrorCode = "AI_OUTPUT_INVALID"
	ErrorContextOverflow     ErrorCode = "AI_CONTEXT_OVERFLOW"
	ErrorBudgetExhausted     ErrorCode = "AI_BUDGET_EXHAUSTED"
)

// Error is the provider-neutral error returned by the AI infrastructure.
type Error struct {
	Code      ErrorCode
	Retryable bool
	Cause     error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Cause)
}

func (e *Error) Unwrap() error { return e.Cause }

// IsErrorCode reports whether err contains an AI error with the requested code.
func IsErrorCode(err error, code ErrorCode) bool {
	var modelErr *Error
	return errors.As(err, &modelErr) && modelErr.Code == code
}
