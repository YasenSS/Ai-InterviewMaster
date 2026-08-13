package ai

import (
	"context"
	"errors"
	"testing"
	"time"
)

type sequenceModel struct {
	err      []error
	response GenerateResponse
	calls    int
}

func (m *sequenceModel) Generate(context.Context, GenerateRequest) (GenerateResponse, error) {
	index := m.calls
	m.calls++
	if index < len(m.err) && m.err[index] != nil {
		return GenerateResponse{}, m.err[index]
	}
	return m.response, nil
}

func TestRetryingChatModelRetriesTransientErrorsThenSucceeds(t *testing.T) {
	inner := &sequenceModel{
		err: []error{
			&Error{Code: ErrorRateLimited, Retryable: true, Cause: errors.New("429")},
			nil,
		},
		response: GenerateResponse{Message: Message{Content: "ok"}},
	}
	model := RetryingChatModel{Inner: inner, Sleep: func(time.Duration) {}}
	response, err := model.Generate(context.Background(), GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.Message.Content != "ok" || inner.calls != 2 {
		t.Fatalf("calls = %d response = %#v", inner.calls, response)
	}
}

func TestRetryingChatModelDoesNotRetryInvalidOutput(t *testing.T) {
	inner := &sequenceModel{
		err: []error{&Error{Code: ErrorOutputInvalid, Cause: errors.New("json")}},
	}
	model := RetryingChatModel{Inner: inner, Sleep: func(time.Duration) {}}
	_, err := model.Generate(context.Background(), GenerateRequest{})
	if !IsErrorCode(err, ErrorOutputInvalid) {
		t.Fatalf("error = %v", err)
	}
	if inner.calls != 1 {
		t.Fatalf("calls = %d", inner.calls)
	}
}
