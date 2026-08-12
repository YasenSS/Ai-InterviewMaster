package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
)

// OpenAIConfig configures Eino's OpenAI-compatible ChatModel component.
type OpenAIConfig struct {
	Provider          string
	BaseURL           string
	APIKey            string
	Model             string
	Timeout           time.Duration
	MaxOutputTokens   int
	StructuredOutputs bool
}

// OpenAI is a provider-neutral adapter around Eino's OpenAI component. It is
// safe to share across requests because tools are attached with WithTools,
// which returns an immutable per-request model.
type OpenAI struct {
	model             model.ToolCallingChatModel
	provider          string
	modelName         string
	timeout           time.Duration
	maxOutputTokens   int
	structuredOutputs bool
}

// NewOpenAI constructs an OpenAI or OpenAI-compatible model through Eino.
func NewOpenAI(ctx context.Context, cfg OpenAIConfig) (*OpenAI, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, &platformai.Error{Code: platformai.ErrorNotConfigured, Cause: errors.New("AI API key is empty")}
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, &platformai.Error{Code: platformai.ErrorNotConfigured, Cause: errors.New("AI model is empty")}
	}
	if cfg.Timeout <= 0 || cfg.MaxOutputTokens < 1 {
		return nil, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: errors.New("AI timeout and max output tokens must be positive")}
	}

	maxTokens := cfg.MaxOutputTokens
	component, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:              cfg.APIKey,
		BaseURL:             strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		Model:               strings.TrimSpace(cfg.Model),
		Timeout:             cfg.Timeout,
		MaxCompletionTokens: &maxTokens,
	})
	if err != nil {
		return nil, mapError(err)
	}
	providerName := strings.TrimSpace(cfg.Provider)
	if providerName == "" {
		providerName = "openai"
	}
	return &OpenAI{
		model:             component,
		provider:          providerName,
		modelName:         strings.TrimSpace(cfg.Model),
		timeout:           cfg.Timeout,
		maxOutputTokens:   cfg.MaxOutputTokens,
		structuredOutputs: cfg.StructuredOutputs,
	}, nil
}

// Generate maps the application's stable message and tool types to Eino and
// maps Eino's output back without exposing Eino to domain packages.
func (p *OpenAI) Generate(ctx context.Context, req platformai.GenerateRequest) (platformai.GenerateResponse, error) {
	messages, err := toEinoMessages(req.Messages)
	if err != nil {
		return platformai.GenerateResponse{}, err
	}
	if len(messages) == 0 {
		return platformai.GenerateResponse{}, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: errors.New("at least one message is required")}
	}

	requestModel := p.model
	if len(req.Tools) > 0 {
		toolInfos, toolErr := toEinoTools(req.Tools)
		if toolErr != nil {
			return platformai.GenerateResponse{}, toolErr
		}
		requestModel, err = p.model.WithTools(toolInfos)
		if err != nil {
			return platformai.GenerateResponse{}, mapError(err)
		}
	}

	options := make([]model.Option, 0, 3)
	maxTokens := p.maxOutputTokens
	if req.MaxTokens > 0 && req.MaxTokens < maxTokens {
		maxTokens = req.MaxTokens
	}
	options = append(options, einoopenai.WithMaxCompletionTokens(maxTokens))
	if req.Temperature != nil {
		if *req.Temperature < 0 || *req.Temperature > 2 {
			return platformai.GenerateResponse{}, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: errors.New("temperature must be between 0 and 2")}
		}
		options = append(options, model.WithTemperature(float32(*req.Temperature)))
	}
	if len(req.JSONSchema) > 0 {
		var rawSchema map[string]any
		if err := json.Unmarshal(req.JSONSchema, &rawSchema); err != nil {
			return platformai.GenerateResponse{}, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: fmt.Errorf("decode JSON Schema: %w", err)}
		}
		if strings.TrimSpace(req.SchemaName) == "" {
			return platformai.GenerateResponse{}, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: errors.New("schema name is required with JSON Schema")}
		}
		if p.structuredOutputs {
			options = append(options, einoopenai.WithExtraFields(map[string]any{
				"response_format": map[string]any{
					"type": "json_schema",
					"json_schema": map[string]any{
						"name":   req.SchemaName,
						"strict": true,
						"schema": rawSchema,
					},
				},
			}))
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	output, err := requestModel.Generate(callCtx, messages, options...)
	if err != nil {
		return platformai.GenerateResponse{}, mapError(err)
	}
	if output == nil {
		return platformai.GenerateResponse{}, &platformai.Error{Code: platformai.ErrorProviderUnavailable, Retryable: true, Cause: errors.New("provider returned an empty message")}
	}

	result := platformai.GenerateResponse{
		Message:  fromEinoMessage(output),
		Provider: p.provider,
		Model:    p.modelName,
	}
	if output.ResponseMeta != nil && output.ResponseMeta.Usage != nil {
		result.Usage = platformai.Usage{
			PromptTokens:     output.ResponseMeta.Usage.PromptTokens,
			CompletionTokens: output.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      output.ResponseMeta.Usage.TotalTokens,
		}
	}
	return result, nil
}

func toEinoMessages(messages []platformai.Message) ([]*schema.Message, error) {
	result := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case platformai.RoleSystem:
			result = append(result, schema.SystemMessage(message.Content))
		case platformai.RoleUser:
			result = append(result, schema.UserMessage(message.Content))
		case platformai.RoleAssistant:
			calls := make([]schema.ToolCall, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				calls = append(calls, schema.ToolCall{ID: call.ID, Type: "function", Function: schema.FunctionCall{Name: call.Name, Arguments: call.Arguments}})
			}
			result = append(result, schema.AssistantMessage(message.Content, calls))
		case platformai.RoleTool:
			if strings.TrimSpace(message.ToolCallID) == "" {
				return nil, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: errors.New("tool message is missing tool call ID")}
			}
			result = append(result, schema.ToolMessage(message.Content, message.ToolCallID))
		default:
			return nil, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: fmt.Errorf("unsupported message role %q", message.Role)}
		}
	}
	return result, nil
}

func toEinoTools(tools []platformai.Tool) ([]*schema.ToolInfo, error) {
	result := make([]*schema.ToolInfo, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, item := range tools {
		name := strings.TrimSpace(item.Name())
		if name == "" {
			return nil, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: errors.New("tool name is empty")}
		}
		if _, exists := seen[name]; exists {
			return nil, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: fmt.Errorf("duplicate tool %q", name)}
		}
		seen[name] = struct{}{}
		var params jsonschema.Schema
		if err := json.Unmarshal(item.Schema(), &params); err != nil {
			return nil, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: fmt.Errorf("decode schema for tool %q: %w", name, err)}
		}
		result = append(result, &schema.ToolInfo{
			Name:        name,
			Desc:        strings.TrimSpace(item.Description()),
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&params),
		})
	}
	return result, nil
}

func fromEinoMessage(message *schema.Message) platformai.Message {
	result := platformai.Message{Role: platformai.Role(message.Role), Content: message.Content, ToolCallID: message.ToolCallID}
	for _, call := range message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, platformai.ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
	}
	return result
}

func mapError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &platformai.Error{Code: platformai.ErrorTimeout, Retryable: true, Cause: err}
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	var apiErr *einoopenai.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.HTTPStatusCode == http.StatusUnauthorized || apiErr.HTTPStatusCode == http.StatusForbidden:
			return &platformai.Error{Code: platformai.ErrorAuthentication, Cause: err}
		case apiErr.HTTPStatusCode == http.StatusTooManyRequests:
			return &platformai.Error{Code: platformai.ErrorRateLimited, Retryable: true, Cause: err}
		case apiErr.HTTPStatusCode == http.StatusRequestTimeout || apiErr.HTTPStatusCode >= http.StatusInternalServerError:
			return &platformai.Error{Code: platformai.ErrorProviderUnavailable, Retryable: true, Cause: err}
		default:
			return &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: err}
		}
	}
	return &platformai.Error{Code: platformai.ErrorProviderUnavailable, Retryable: true, Cause: err}
}
