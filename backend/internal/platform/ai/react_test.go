package ai

import (
	"context"
	"testing"
)

type scriptedToolModel struct {
	calls []GenerateResponse
}

func (m *scriptedToolModel) Generate(_ context.Context, _ GenerateRequest) (GenerateResponse, error) {
	if len(m.calls) == 0 {
		return GenerateResponse{Message: Message{Role: RoleAssistant, Content: `{"action":"next_capability","question":"","capability_key":"","evidence_fact_ids":[],"reason":"done"}`}}, nil
	}
	next := m.calls[0]
	m.calls = m.calls[1:]
	return next, nil
}

type recordingTool struct {
	name string
	args []string
}

func (t *recordingTool) Name() string        { return t.name }
func (t *recordingTool) Description() string { return t.name }
func (t *recordingTool) Schema() []byte {
	return []byte(`{"type":"object","properties":{"query":{"type":"string"}}}`)
}
func (t *recordingTool) Call(_ context.Context, args string) (string, error) {
	t.args = append(t.args, args)
	return `{"hits":[]}`, nil
}

func TestStripTenantArgsRemovesUserID(t *testing.T) {
	got := StripTenantArgs(`{"query":"kafka","user_id":"other-user","workspace_id":"ws"}`)
	if got != `{"query":"kafka"}` {
		t.Fatalf("stripped = %s", got)
	}
}

func TestRunToolLoopStripsTenantAndStopsAfterTwoCalls(t *testing.T) {
	tool := &recordingTool{name: "lookup_resume_facts"}
	model := &scriptedToolModel{calls: []GenerateResponse{
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "1", Name: "lookup_resume_facts", Arguments: `{"query":"x","user_id":"attacker"}`}}}},
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "2", Name: "lookup_resume_facts", Arguments: `{"query":"y"}`}}}},
		{Message: Message{Role: RoleAssistant, Content: "should not be requested"}},
	}}
	req := GenerateRequest{
		Messages: []Message{{Role: RoleUser, Content: "decide"}},
		Tools:    []Tool{tool},
	}
	working, _, err := RunToolLoop(context.Background(), model, req, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(tool.args) != 2 {
		t.Fatalf("tool calls = %d", len(tool.args))
	}
	if tool.args[0] != `{"query":"x"}` {
		t.Fatalf("first args = %s", tool.args[0])
	}
	toolMsgs := 0
	for _, message := range working.Messages {
		if message.Role == RoleTool {
			toolMsgs++
		}
	}
	if toolMsgs != 2 {
		t.Fatalf("tool messages = %d", toolMsgs)
	}
}
