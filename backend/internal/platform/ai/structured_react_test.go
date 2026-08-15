package ai

import (
	"context"
	"testing"
)

type countingStructuredModel struct {
	responses []GenerateResponse
	calls     int
}

func (m *countingStructuredModel) Generate(_ context.Context, _ GenerateRequest) (GenerateResponse, error) {
	response := m.responses[m.calls]
	m.calls++
	return response, nil
}

func TestRunStructuredToolLoopUsesOneCallWithoutTools(t *testing.T) {
	type output struct {
		Value string `json:"value"`
	}
	model := &countingStructuredModel{responses: []GenerateResponse{{
		Message: GenerateResponseMessage(`{"value":"ok"}`),
	}}}
	result, err := RunStructuredToolLoop[output](context.Background(), model, GenerateRequest{
		JSONSchema: []byte(`{"type":"object","additionalProperties":false,"properties":{"value":{"type":"string"}},"required":["value"]}`),
	}, 2, func(value output) error { return nil })
	if err != nil {
		t.Fatalf("RunStructuredToolLoop() error = %v", err)
	}
	if model.calls != 1 || result.Value.Value != "ok" || result.ToolCalls != 0 {
		t.Fatalf("unexpected result: calls=%d result=%#v", model.calls, result)
	}
}

func GenerateResponseMessage(content string) Message {
	return Message{Role: RoleAssistant, Content: content}
}
