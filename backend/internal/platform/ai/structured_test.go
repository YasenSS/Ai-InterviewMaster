package ai

import (
	"context"
	"errors"
	"testing"
)

type fakeChatModel struct {
	response GenerateResponse
	err      error
}

func (f fakeChatModel) Generate(context.Context, GenerateRequest) (GenerateResponse, error) {
	return f.response, f.err
}

type testOutput struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

func TestStructuredRunnerDecodesAndValidates(t *testing.T) {
	runner, err := NewStructuredRunner[testOutput](context.Background(), fakeChatModel{response: GenerateResponse{
		Message: Message{Role: RoleAssistant, Content: `{"name":"candidate","score":80}`},
		Usage:   Usage{TotalTokens: 12},
	}}, func(value testOutput) error {
		if value.Score < 0 || value.Score > 100 {
			return errors.New("score outside range")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("NewStructuredRunner() error = %v", err)
	}
	result, err := runner.Invoke(context.Background(), StructuredRequest{})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Value.Name != "candidate" || result.Value.Score != 80 {
		t.Fatalf("result = %#v", result.Value)
	}
	if result.Response.Usage.TotalTokens != 12 {
		t.Fatalf("usage = %#v", result.Response.Usage)
	}
}

func TestStructuredRunnerRejectsUnknownFields(t *testing.T) {
	runner, err := NewStructuredRunner[testOutput](context.Background(), fakeChatModel{response: GenerateResponse{
		Message: Message{Role: RoleAssistant, Content: `{"name":"candidate","score":80,"unexpected":true}`},
	}}, nil)
	if err != nil {
		t.Fatalf("NewStructuredRunner() error = %v", err)
	}
	_, err = runner.Invoke(context.Background(), StructuredRequest{})
	if !IsErrorCode(err, ErrorOutputInvalid) {
		t.Fatalf("Invoke() error = %v, want AI_OUTPUT_INVALID", err)
	}
}
