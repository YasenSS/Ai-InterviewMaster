package ai

import (
	"sync/atomic"
	"time"
)

// Snapshot is a process-local view of model-call health. It is safe for
// concurrent use and never includes user content, prompts, or API keys.
type Snapshot struct {
	Requests                int64 `json:"requests"`
	Successes               int64 `json:"successes"`
	Failures                int64 `json:"failures"`
	Retries                 int64 `json:"retries"`
	Degraded                int64 `json:"degraded"`
	BudgetRejected          int64 `json:"budget_rejected"`
	PromptTokens            int64 `json:"prompt_tokens"`
	CompletionTokens        int64 `json:"completion_tokens"`
	TotalTokens             int64 `json:"total_tokens"`
	EstimatedCostMicros     int64 `json:"estimated_cost_micros"`
	LatencyMSSum            int64 `json:"latency_ms_sum"`
	StructuredFirstFail     int64 `json:"structured_first_fail"`
	StructuredRepairSuccess int64 `json:"structured_repair_success"`
	StructuredFinalFail     int64 `json:"structured_final_fail"`
}

var (
	metricRequests                atomic.Int64
	metricSuccesses               atomic.Int64
	metricFailures                atomic.Int64
	metricRetries                 atomic.Int64
	metricDegraded                atomic.Int64
	metricBudgetRejected          atomic.Int64
	metricPromptTokens            atomic.Int64
	metricCompletionTokens        atomic.Int64
	metricTotalTokens             atomic.Int64
	metricEstimatedCostMicros     atomic.Int64
	metricLatencyMSSum            atomic.Int64
	metricStructuredFirstFail     atomic.Int64
	metricStructuredRepairSuccess atomic.Int64
	metricStructuredFinalFail     atomic.Int64
)

func RecordRetry() { metricRetries.Add(1) }

func RecordDegraded() { metricDegraded.Add(1) }

func RecordBudgetRejected() { metricBudgetRejected.Add(1) }

func RecordStructuredFirstFail() { metricStructuredFirstFail.Add(1) }

func RecordStructuredRepairSuccess() { metricStructuredRepairSuccess.Add(1) }

func RecordStructuredFinalFail() { metricStructuredFinalFail.Add(1) }

func RecordGenerate(err error, usage Usage, costMicros, latencyMS int64, degraded bool) {
	metricRequests.Add(1)
	if degraded {
		metricDegraded.Add(1)
	}
	metricPromptTokens.Add(int64(usage.PromptTokens))
	metricCompletionTokens.Add(int64(usage.CompletionTokens))
	metricTotalTokens.Add(int64(usage.TotalTokens))
	metricEstimatedCostMicros.Add(costMicros)
	if latencyMS < 0 {
		latencyMS = 0
	}
	metricLatencyMSSum.Add(latencyMS)
	if err != nil {
		metricFailures.Add(1)
		return
	}
	metricSuccesses.Add(1)
}

func MetricsSnapshot() Snapshot {
	return Snapshot{
		Requests:                metricRequests.Load(),
		Successes:               metricSuccesses.Load(),
		Failures:                metricFailures.Load(),
		Retries:                 metricRetries.Load(),
		Degraded:                metricDegraded.Load(),
		BudgetRejected:          metricBudgetRejected.Load(),
		PromptTokens:            metricPromptTokens.Load(),
		CompletionTokens:        metricCompletionTokens.Load(),
		TotalTokens:             metricTotalTokens.Load(),
		EstimatedCostMicros:     metricEstimatedCostMicros.Load(),
		LatencyMSSum:            metricLatencyMSSum.Load(),
		StructuredFirstFail:     metricStructuredFirstFail.Load(),
		StructuredRepairSuccess: metricStructuredRepairSuccess.Load(),
		StructuredFinalFail:     metricStructuredFinalFail.Load(),
	}
}

func latencyMillis(started time.Time, finished time.Time) int64 {
	ms := finished.Sub(started).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}
