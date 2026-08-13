package ai

import (
	"context"
	"errors"
	"testing"
)

type sequenceChatModel struct {
	contents []string
	calls    int
}

func (m *sequenceChatModel) Generate(_ context.Context, _ GenerateRequest) (GenerateResponse, error) {
	if m.calls >= len(m.contents) {
		return GenerateResponse{}, errors.New("no more responses")
	}
	content := m.contents[m.calls]
	m.calls++
	return GenerateResponse{Message: Message{Role: RoleAssistant, Content: content}}, nil
}

func TestStructuredRunnerRepairsInvalidJSONOnce(t *testing.T) {
	schema := []byte(`{"type":"object","additionalProperties":false,"properties":{"name":{"type":"string"},"score":{"type":"integer"}},"required":["name","score"]}`)
	model := &sequenceChatModel{contents: []string{
		`{"name":"candidate","score":80,"unexpected":true}`,
		`{"name":"candidate","score":80}`,
	}}
	runner, err := NewStructuredRunner[testOutput](context.Background(), model, nil)
	if err != nil {
		t.Fatalf("NewStructuredRunner() error = %v", err)
	}
	result, err := runner.Invoke(context.Background(), StructuredRequest{Generate: GenerateRequest{JSONSchema: schema}})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Value.Name != "candidate" || model.calls != 2 {
		t.Fatalf("result = %#v calls = %d", result.Value, model.calls)
	}
}

func TestStructuredRunnerStopsAfterOneFailedRepair(t *testing.T) {
	schema := []byte(`{"type":"object","additionalProperties":false,"properties":{"name":{"type":"string"},"score":{"type":"integer"}},"required":["name","score"]}`)
	model := &sequenceChatModel{contents: []string{`{"bad":true}`, `{"also":false}`}}
	runner, err := NewStructuredRunner[testOutput](context.Background(), model, nil)
	if err != nil {
		t.Fatalf("NewStructuredRunner() error = %v", err)
	}
	_, err = runner.Invoke(context.Background(), StructuredRequest{Generate: GenerateRequest{JSONSchema: schema}})
	if !IsErrorCode(err, ErrorOutputInvalid) {
		t.Fatalf("error = %v", err)
	}
	if model.calls != 2 {
		t.Fatalf("calls = %d", model.calls)
	}
}
