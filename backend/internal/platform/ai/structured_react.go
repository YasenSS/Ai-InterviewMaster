package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// StructuredToolLoopResult is the validated final value from one logical
// ReAct run. A model call is repeated only when a real tool result must be
// returned or the one allowed structured-output repair is needed.
type StructuredToolLoopResult[T any] struct {
	Value     T
	Response  GenerateResponse
	ToolCalls int
}

func RunStructuredToolLoop[T any](
	ctx context.Context,
	chat ChatModel,
	request GenerateRequest,
	maxToolCalls int,
	validate Validator[T],
) (StructuredToolLoopResult[T], error) {
	var zero StructuredToolLoopResult[T]
	if chat == nil {
		return zero, &Error{Code: ErrorNotConfigured, Cause: errors.New("chat model is nil")}
	}
	if maxToolCalls < 0 {
		maxToolCalls = 0
	}
	schema := append([]byte(nil), request.JSONSchema...)
	schemaName := request.SchemaName
	working := request
	working.JSONSchema = nil
	working.SchemaName = ""
	used := 0
	for {
		response, err := chat.Generate(ctx, working)
		if err != nil {
			return zero, err
		}
		if len(response.Message.ToolCalls) > 0 {
			if used+len(response.Message.ToolCalls) > maxToolCalls {
				return zero, &Error{Code: ErrorOutputInvalid, Cause: fmt.Errorf("tool-call limit exceeded")}
			}
			used += len(response.Message.ToolCalls)
			working.Messages = append(append([]Message{}, working.Messages...), response.Message)
			for _, call := range response.Message.ToolCalls {
				working.Messages = append(working.Messages, Message{
					Role:       RoleTool,
					ToolCallID: call.ID,
					Content:    ClipRunes(invokeNamedTool(ctx, working.Tools, call), 2000),
				})
			}
			continue
		}
		value, parseErr := parseStructuredValue[T](schema, response.Message.Content, validate)
		if parseErr == nil {
			return StructuredToolLoopResult[T]{Value: value, Response: response, ToolCalls: used}, nil
		}
		RecordStructuredFirstFail()
		repair := working
		repair.Tools = nil
		repair.SchemaName = schemaName
		repair.JSONSchema = schema
		repair.Messages = append(append([]Message{}, working.Messages...), response.Message, Message{
			Role:    RoleUser,
			Content: "上次最终输出未通过校验：" + parseErr.Error() + "\n请只输出修正后的完整 JSON，不要解释，也不要调用工具。",
		})
		repaired, repairErr := chat.Generate(ctx, repair)
		if repairErr != nil {
			RecordStructuredFinalFail()
			return zero, repairErr
		}
		value, parseErr = parseStructuredValue[T](schema, repaired.Message.Content, validate)
		if parseErr != nil {
			RecordStructuredFinalFail()
			return zero, parseErr
		}
		RecordStructuredRepairSuccess()
		return StructuredToolLoopResult[T]{Value: value, Response: repaired, ToolCalls: used}, nil
	}
}

func parseStructuredValue[T any](schema []byte, content string, validate Validator[T]) (T, error) {
	var zero T
	content = strings.TrimSpace(content)
	if err := ValidateJSON(schema, []byte(content)); err != nil {
		return zero, &Error{Code: ErrorOutputInvalid, Cause: err}
	}
	value, err := decodeJSON[T](content)
	if err != nil {
		return zero, &Error{Code: ErrorOutputInvalid, Cause: err}
	}
	if validate != nil {
		if err := validate(value); err != nil {
			return zero, &Error{Code: ErrorOutputInvalid, Cause: err}
		}
	}
	return value, nil
}
