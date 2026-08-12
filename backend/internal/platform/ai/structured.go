package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/compose"
)

// StructuredRequest is the input to a deterministic generate-and-decode graph.
type StructuredRequest struct {
	Generate GenerateRequest
}

// StructuredResult contains both the validated domain value and model metadata
// needed by future audit and cost layers.
type StructuredResult[T any] struct {
	Value    T
	Response GenerateResponse
}

// Validator applies domain rules after JSON decoding. JSON Schema constrains
// generation; this validator remains authoritative before business writes.
type Validator[T any] func(T) error

// StructuredRunner is a compiled Eino Graph with two deterministic nodes:
// model generation followed by strict JSON decoding and domain validation.
type StructuredRunner[T any] struct {
	runnable compose.Runnable[StructuredRequest, StructuredResult[T]]
}

// NewStructuredRunner compiles the reusable Eino graph once. The supplied
// ChatModel can be a real Provider or a test fake.
func NewStructuredRunner[T any](ctx context.Context, chatModel ChatModel, validate Validator[T]) (*StructuredRunner[T], error) {
	if chatModel == nil {
		return nil, &Error{Code: ErrorNotConfigured, Cause: errors.New("chat model is nil")}
	}

	graph := compose.NewGraph[StructuredRequest, StructuredResult[T]]()
	if err := graph.AddLambdaNode("generate", compose.InvokableLambda(func(ctx context.Context, input StructuredRequest) (GenerateResponse, error) {
		return chatModel.Generate(ctx, input.Generate)
	})); err != nil {
		return nil, fmt.Errorf("add generate node: %w", err)
	}
	if err := graph.AddLambdaNode("decode", compose.InvokableLambda(func(_ context.Context, response GenerateResponse) (StructuredResult[T], error) {
		value, err := decodeJSON[T](response.Message.Content)
		if err != nil {
			return StructuredResult[T]{}, &Error{Code: ErrorOutputInvalid, Cause: err}
		}
		if validate != nil {
			if err := validate(value); err != nil {
				return StructuredResult[T]{}, &Error{Code: ErrorOutputInvalid, Cause: err}
			}
		}
		return StructuredResult[T]{Value: value, Response: response}, nil
	})); err != nil {
		return nil, fmt.Errorf("add decode node: %w", err)
	}
	if err := graph.AddEdge(compose.START, "generate"); err != nil {
		return nil, fmt.Errorf("connect graph start: %w", err)
	}
	if err := graph.AddEdge("generate", "decode"); err != nil {
		return nil, fmt.Errorf("connect graph decoder: %w", err)
	}
	if err := graph.AddEdge("decode", compose.END); err != nil {
		return nil, fmt.Errorf("connect graph end: %w", err)
	}
	runnable, err := graph.Compile(ctx, compose.WithMaxRunSteps(4), compose.WithGraphName("structured_generation"))
	if err != nil {
		return nil, fmt.Errorf("compile structured generation graph: %w", err)
	}
	return &StructuredRunner[T]{runnable: runnable}, nil
}

// Invoke generates, decodes and validates one typed result.
func (r *StructuredRunner[T]) Invoke(ctx context.Context, request StructuredRequest) (StructuredResult[T], error) {
	return r.runnable.Invoke(ctx, request)
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
