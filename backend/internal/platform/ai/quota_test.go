package ai

import (
	"context"
	"testing"
)

func TestLimiterRejectsWhenDailyHardLimitReached(t *testing.T) {
	counters := NewMemoryCounters()
	limiter := NewLimiter(counters, Quota{MaxInflight: 2, DailyCallsHard: 1, MaxInputRunes: 100})
	first, err := limiter.acquire(context.Background(), "user-1", 10)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	limiter.release(context.Background(), first, 0)
	if _, err := limiter.acquire(context.Background(), "user-1", 10); !IsErrorCode(err, ErrorBudgetExhausted) {
		t.Fatalf("second acquire error = %v", err)
	}
}

func TestLimiterRejectsOversizedInput(t *testing.T) {
	limiter := NewLimiter(NewMemoryCounters(), Quota{MaxInflight: 2, DailyCallsHard: 10, MaxInputRunes: 5})
	if _, err := limiter.acquire(context.Background(), "user-1", 9); !IsErrorCode(err, ErrorContextOverflow) {
		t.Fatalf("error = %v", err)
	}
}
