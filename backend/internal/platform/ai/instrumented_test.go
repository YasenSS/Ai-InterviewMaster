package ai

import (
	"context"
	"errors"
	"testing"
)

func TestInstrumentedChatModelWritesAuditAndEnforcesQuota(t *testing.T) {
	store := &MemoryInvocations{}
	limiter := NewLimiter(NewMemoryCounters(), Quota{MaxInflight: 2, DailyCallsHard: 5, MaxInputRunes: 1000})
	inner := fakeChatModel{response: GenerateResponse{
		Message:  Message{Role: RoleAssistant, Content: "hello"},
		Usage:    Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
		Provider: "fake",
		Model:    "fake-model",
	}}
	model := NewInstrumented(inner, nil, store, limiter, Pricing{}, false)
	response, err := model.Generate(context.Background(), GenerateRequest{
		UserID:        "user-1",
		PromptKey:     "question.generate",
		PromptVersion: "v1",
		Messages:      []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.Message.Content != "hello" {
		t.Fatalf("content = %q", response.Message.Content)
	}
	if len(store.Rows) != 1 || store.Rows[0].Status != "succeeded" || store.Rows[0].TotalTokens != 12 {
		t.Fatalf("audit = %#v", store.Rows)
	}
	if store.Rows[0].InputHash == "" || store.Rows[0].OutputHash == "" {
		t.Fatal("expected input and output hashes")
	}
}

func TestInstrumentedChatModelSurfacesBudgetErrors(t *testing.T) {
	limiter := NewLimiter(NewMemoryCounters(), Quota{MaxInflight: 2, DailyCallsHard: 0, MaxInputRunes: 4})
	model := NewInstrumented(fakeChatModel{}, nil, &MemoryInvocations{}, limiter, Pricing{}, false)
	_, err := model.Generate(context.Background(), GenerateRequest{
		UserID:   "user-1",
		Messages: []Message{{Role: RoleUser, Content: "too-long"}},
	})
	if !IsErrorCode(err, ErrorContextOverflow) && !errors.Is(err, err) {
		t.Fatalf("error = %v", err)
	}
	if !IsErrorCode(err, ErrorContextOverflow) {
		t.Fatalf("error = %v, want AI_CONTEXT_OVERFLOW", err)
	}
}
