package mcp

import (
	"context"
	"testing"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
)

type stubTool struct{}

func (stubTool) Name() string        { return "lookup_resume_facts" }
func (stubTool) Description() string { return "facts" }
func (stubTool) Schema() []byte      { return []byte(`{"type":"object"}`) }
func (stubTool) Call(_ context.Context, args string) (string, error) {
	return args, nil
}

func TestAdapterStripsTenantOnCall(t *testing.T) {
	adapter := NewAdapter(stubTool{})
	if len(adapter.ListTools()) != 1 {
		t.Fatalf("listed = %d", len(adapter.ListTools()))
	}
	got, err := adapter.Call(t.Context(), "lookup_resume_facts", `{"query":"go","user_id":"other"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"query":"go"}` {
		t.Fatalf("call = %s", got)
	}
}

func TestAdapterRejectsUnknownTool(t *testing.T) {
	adapter := NewAdapter()
	if _, err := adapter.Call(t.Context(), "delete_user", `{}`); err == nil {
		t.Fatal("unknown tool accepted")
	}
}

var _ platformai.Tool = stubTool{}
