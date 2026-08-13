package ai

import (
	"context"
	"encoding/json"
	"strings"
)

const MaxToolCallsPerTurn = 2

// StripTenantArgs removes tenant identifiers the model is not allowed to set.
func StripTenantArgs(args string) string {
	args = strings.TrimSpace(args)
	if args == "" {
		return "{}"
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return args
	}
	for _, key := range []string{"user_id", "userId", "workspace_id", "workspaceId", "tenant_id"} {
		delete(payload, key)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// RunToolLoop executes at most maxCalls read-only tool rounds without JSON
// Schema. Callers should then invoke StructuredRunner without tools.
func RunToolLoop(ctx context.Context, chat ChatModel, req GenerateRequest, maxCalls int) (GenerateRequest, GenerateResponse, error) {
	if maxCalls < 1 {
		maxCalls = MaxToolCallsPerTurn
	}
	working := req
	var last GenerateResponse
	for range maxCalls {
		loopReq := working
		loopReq.JSONSchema = nil
		loopReq.SchemaName = ""
		response, err := chat.Generate(ctx, loopReq)
		if err != nil {
			return working, last, err
		}
		last = response
		if len(response.Message.ToolCalls) == 0 {
			return working, response, nil
		}
		working.Messages = append(append([]Message{}, working.Messages...), response.Message)
		for _, call := range response.Message.ToolCalls {
			result := invokeNamedTool(ctx, working.Tools, call)
			working.Messages = append(working.Messages, Message{
				Role:       RoleTool,
				ToolCallID: call.ID,
				Content:    ClipRunes(result, 2000),
			})
		}
	}
	return working, last, nil
}

func invokeNamedTool(ctx context.Context, tools []Tool, call ToolCall) string {
	args := StripTenantArgs(call.Arguments)
	for _, tool := range tools {
		if tool == nil || tool.Name() != call.Name {
			continue
		}
		result, err := tool.Call(ctx, args)
		if err != nil {
			return `{"error":"tool_failed"}`
		}
		if strings.TrimSpace(result) == "" {
			return `{"error":"empty_result"}`
		}
		return result
	}
	return `{"error":"unknown_tool"}`
}
