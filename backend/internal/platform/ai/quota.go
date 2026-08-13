package ai

import (
	"context"
	"sync"
	"time"
)

// Quota captures per-user concurrency, call count, and cost limits.
type Quota struct {
	MaxInflight           int
	DailyCallsSoft        int
	DailyCallsHard        int
	DailyCostSoftMicros   int64
	DailyCostHardMicros   int64
	MonthlyCostHardMicros int64
	MaxInputRunes         int
}

func DefaultQuota() Quota {
	return Quota{
		MaxInflight:           2,
		DailyCallsSoft:        30,
		DailyCallsHard:        50,
		DailyCostSoftMicros:   2_000_000,
		DailyCostHardMicros:   5_000_000,
		MonthlyCostHardMicros: 20_000_000,
		MaxInputRunes:         50_000,
	}
}

// Counters is the storage used by the limiter. Redis in production, memory in tests.
type Counters interface {
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Add(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
	Decr(ctx context.Context, key string) error
	Get(ctx context.Context, key string) (int64, error)
}

// MemoryCounters is a process-local implementation for unit tests.
type MemoryCounters struct {
	mu     sync.Mutex
	values map[string]int64
}

func NewMemoryCounters() *MemoryCounters {
	return &MemoryCounters{values: map[string]int64{}}
}

func (m *MemoryCounters) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	return m.Add(context.Background(), key, 1, 0)
}

func (m *MemoryCounters) Add(_ context.Context, key string, delta int64, _ time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] += delta
	if m.values[key] < 0 {
		m.values[key] = 0
	}
	return m.values[key], nil
}

func (m *MemoryCounters) Decr(_ context.Context, key string) error {
	_, err := m.Add(context.Background(), key, -1, 0)
	return err
}

func (m *MemoryCounters) Get(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.values[key], nil
}

// Limiter enforces user-level AI quotas.
type Limiter struct {
	counters Counters
	quota    Quota
	now      func() time.Time
}

func NewLimiter(counters Counters, quota Quota) *Limiter {
	if quota.MaxInflight < 1 {
		quota = DefaultQuota()
	}
	return &Limiter{counters: counters, quota: quota, now: time.Now}
}

type reservation struct {
	userID   string
	inflight string
	day      string
	month    string
	soft     bool
}

func (l *Limiter) acquire(ctx context.Context, userID string, inputRunes int) (reservation, error) {
	if l == nil || userID == "" {
		return reservation{}, nil
	}
	if l.quota.MaxInputRunes > 0 && inputRunes > l.quota.MaxInputRunes {
		return reservation{}, &Error{Code: ErrorContextOverflow, Cause: errInputTooLarge(inputRunes, l.quota.MaxInputRunes)}
	}
	now := l.now()
	dayKey := "ai:calls:" + userID + ":" + now.UTC().Format("2006-01-02")
	monthKey := "ai:monthcost:" + userID + ":" + now.UTC().Format("2006-01")
	dayCostKey := "ai:daycost:" + userID + ":" + now.UTC().Format("2006-01-02")
	inflightKey := "ai:inflight:" + userID

	inflight, err := l.counters.Incr(ctx, inflightKey, time.Hour)
	if err != nil {
		return reservation{}, err
	}
	if int(inflight) > l.quota.MaxInflight {
		_ = l.counters.Decr(ctx, inflightKey)
		return reservation{}, &Error{Code: ErrorBudgetExhausted, Cause: errConcurrentLimit(l.quota.MaxInflight)}
	}
	calls, err := l.counters.Get(ctx, dayKey)
	if err != nil {
		_ = l.counters.Decr(ctx, inflightKey)
		return reservation{}, err
	}
	if l.quota.DailyCallsHard > 0 && int(calls) >= l.quota.DailyCallsHard {
		_ = l.counters.Decr(ctx, inflightKey)
		return reservation{}, &Error{Code: ErrorBudgetExhausted, Cause: errDailyCallLimit(l.quota.DailyCallsHard)}
	}
	dayCost, err := l.counters.Get(ctx, dayCostKey)
	if err != nil {
		_ = l.counters.Decr(ctx, inflightKey)
		return reservation{}, err
	}
	monthCost, err := l.counters.Get(ctx, monthKey)
	if err != nil {
		_ = l.counters.Decr(ctx, inflightKey)
		return reservation{}, err
	}
	if l.quota.DailyCostHardMicros > 0 && dayCost >= l.quota.DailyCostHardMicros {
		_ = l.counters.Decr(ctx, inflightKey)
		return reservation{}, &Error{Code: ErrorBudgetExhausted, Cause: errDailyCostLimit()}
	}
	if l.quota.MonthlyCostHardMicros > 0 && monthCost >= l.quota.MonthlyCostHardMicros {
		_ = l.counters.Decr(ctx, inflightKey)
		return reservation{}, &Error{Code: ErrorBudgetExhausted, Cause: errMonthlyCostLimit()}
	}
	if _, err := l.counters.Incr(ctx, dayKey, 36*time.Hour); err != nil {
		_ = l.counters.Decr(ctx, inflightKey)
		return reservation{}, err
	}
	soft := (l.quota.DailyCallsSoft > 0 && int(calls)+1 >= l.quota.DailyCallsSoft) ||
		(l.quota.DailyCostSoftMicros > 0 && dayCost >= l.quota.DailyCostSoftMicros)
	return reservation{userID: userID, inflight: inflightKey, day: dayCostKey, month: monthKey, soft: soft}, nil
}

func (l *Limiter) release(ctx context.Context, reserved reservation, costMicros int64) {
	if l == nil || reserved.inflight == "" {
		return
	}
	_ = l.counters.Decr(ctx, reserved.inflight)
	if costMicros > 0 {
		_, _ = l.counters.Add(ctx, reserved.day, costMicros, 36*time.Hour)
		_, _ = l.counters.Add(ctx, reserved.month, costMicros, 40*24*time.Hour)
	}
}

func errInputTooLarge(got, max int) error {
	return &quotaError{msg: "input exceeds configured maximum"}
}

func errConcurrentLimit(max int) error { return &quotaError{msg: "too many in-flight AI calls"} }
func errDailyCallLimit(max int) error  { return &quotaError{msg: "daily AI call limit reached"} }
func errDailyCostLimit() error         { return &quotaError{msg: "daily AI budget reached"} }
func errMonthlyCostLimit() error       { return &quotaError{msg: "monthly AI budget reached"} }

type quotaError struct{ msg string }

func (e *quotaError) Error() string { return e.msg }
