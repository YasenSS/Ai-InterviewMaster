package ai

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

const maxTransportAttempts = 3

// RetryingChatModel retries only transient provider failures. Parameter errors,
// output validation, authentication, and budget rejections are not retried.
type RetryingChatModel struct {
	Inner ChatModel
	Sleep func(time.Duration)
}

func (m RetryingChatModel) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	inner := m.Inner
	if inner == nil {
		return GenerateResponse{}, &Error{Code: ErrorNotConfigured, Cause: errors.New("chat model is nil")}
	}
	sleep := m.Sleep
	if sleep == nil {
		sleep = func(delay time.Duration) {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	}
	var last error
	for attempt := 1; attempt <= maxTransportAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return GenerateResponse{}, &Error{Code: ErrorTimeout, Retryable: true, Cause: err}
		}
		response, err := inner.Generate(ctx, req)
		if err == nil {
			return response, nil
		}
		last = err
		if !isTransportRetryable(err) || attempt == maxTransportAttempts {
			return GenerateResponse{}, err
		}
		RecordRetry()
		sleep(transportBackoff(attempt))
	}
	return GenerateResponse{}, last
}

func isTransportRetryable(err error) bool {
	var modelErr *Error
	if !errors.As(err, &modelErr) {
		return false
	}
	switch modelErr.Code {
	case ErrorRateLimited, ErrorTimeout, ErrorProviderUnavailable:
		return modelErr.Retryable
	default:
		return false
	}
}

func transportBackoff(attempt int) time.Duration {
	base := time.Duration(1<<uint(attempt-1)) * 200 * time.Millisecond
	jitter := time.Duration(rand.Int63n(int64(base/2) + 1))
	return base + jitter
}
