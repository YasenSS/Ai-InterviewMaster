package ai

import "testing"

func TestRecordGenerateCountsSuccessAndFailure(t *testing.T) {
	before := MetricsSnapshot()
	RecordGenerate(nil, Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5}, 10, 12, false)
	RecordGenerate(&Error{Code: ErrorTimeout}, Usage{}, 0, 4, true)
	RecordBudgetRejected()
	after := MetricsSnapshot()
	if after.Requests-before.Requests != 2 {
		t.Fatalf("requests delta = %d", after.Requests-before.Requests)
	}
	if after.Successes-before.Successes != 1 || after.Failures-before.Failures != 1 {
		t.Fatalf("success/fail = %d/%d", after.Successes-before.Successes, after.Failures-before.Failures)
	}
	if after.Degraded-before.Degraded != 1 || after.BudgetRejected-before.BudgetRejected != 1 {
		t.Fatalf("degraded/budget = %d/%d", after.Degraded-before.Degraded, after.BudgetRejected-before.BudgetRejected)
	}
	if after.TotalTokens-before.TotalTokens != 5 {
		t.Fatalf("tokens delta = %d", after.TotalTokens-before.TotalTokens)
	}
}
