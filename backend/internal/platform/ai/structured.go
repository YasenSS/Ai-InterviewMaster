package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// StructuredRequest is the input to a deterministic generate-and-decode graph.
type StructuredRequest struct {
	Generate GenerateRequest
}

// StructuredResult contains both the validated domain value and model metadata
// needed by audit and cost layers.
type StructuredResult[T any] struct {
	Value    T
	Response GenerateResponse
}

// Validator applies domain rules after JSON decoding. JSON Schema constrains
// generation; this validator remains authoritative before business writes.
type Validator[T any] func(T) error

// StructuredRunner generates JSON, validates schema, decodes into T, then
// applies domain rules. Invalid output is repaired at most once.
type StructuredRunner[T any] struct {
	chat     ChatModel
	validate Validator[T]
}

// NewStructuredRunner constructs the reusable generate/decode/repair executor.
func NewStructuredRunner[T any](_ context.Context, chatModel ChatModel, validate Validator[T]) (*StructuredRunner[T], error) {
	if chatModel == nil {
		return nil, &Error{Code: ErrorNotConfigured, Cause: errors.New("chat model is nil")}
	}
	return &StructuredRunner[T]{chat: chatModel, validate: validate}, nil
}

// Invoke generates, decodes and validates one typed result.
func (r *StructuredRunner[T]) Invoke(ctx context.Context, request StructuredRequest) (StructuredResult[T], error) {
	result, err := r.attempt(ctx, request.Generate)
	if err == nil {
		return result, nil
	}
	if !IsErrorCode(err, ErrorOutputInvalid) {
		return StructuredResult[T]{}, err
	}
	RecordStructuredFirstFail()
	repair := request.Generate
	repair.Messages = append(append([]Message{}, request.Generate.Messages...), Message{
		Role:    RoleUser,
		Content: "上次输出未通过校验：" + err.Error() + "\n请只输出修正后的完整 JSON，不要解释。",
	})
	repaired, repairErr := r.attempt(ctx, repair)
	if repairErr != nil {
		RecordStructuredFinalFail()
		return StructuredResult[T]{}, repairErr
	}
	RecordStructuredRepairSuccess()
	return repaired, nil
}

func (r *StructuredRunner[T]) attempt(ctx context.Context, request GenerateRequest) (StructuredResult[T], error) {
	response, err := r.chat.Generate(ctx, request)
	if err != nil {
		return StructuredResult[T]{}, err
	}
	if err := ValidateJSON(request.JSONSchema, []byte(strings.TrimSpace(response.Message.Content))); err != nil {
		return StructuredResult[T]{}, &Error{Code: ErrorOutputInvalid, Cause: err}
	}
	value, err := decodeJSON[T](response.Message.Content)
	if err != nil {
		return StructuredResult[T]{}, &Error{Code: ErrorOutputInvalid, Cause: err}
	}
	if r.validate != nil {
		if err := r.validate(value); err != nil {
			return StructuredResult[T]{}, &Error{Code: ErrorOutputInvalid, Cause: err}
		}
	}
	return StructuredResult[T]{Value: value, Response: response}, nil
}

func decodeJSON[T any](content string) (T, error) {
	var result T
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") && strings.HasSuffix(content, "```") {
		content = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(content, "```json"), "```"))
	} else if strings.HasPrefix(content, "```") && strings.HasSuffix(content, "```") {
		content = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(content, "```"), "```"))
	}
	if content == "" {
		return result, errors.New("model returned empty structured output")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode structured output: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return result, errors.New("structured output contains multiple JSON values")
		}
		return result, fmt.Errorf("decode trailing structured output: %w", err)
	}
	return result, nil
}
