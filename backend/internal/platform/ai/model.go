// Package ai 提供智能层基建：模型调用抽象、Provider 实现、审计与编排。
//
// 领域层（apps/*/internal/logic）只依赖本包定义的自研接口，
// 不直接 import Eino 或任何模型供应商 SDK；Eino 仅作为本包内部实现，
// 保证业务可替换、可测试、可审计。见 docs/agent设计.md。
package ai

import "context"

// Role 标识一条对话消息的角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 是一条对话消息。ToolCalls 仅在 assistant 消息上有值；
// ToolCallID 仅在 tool 消息上有值（对应某次工具调用的结果）。
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

// ToolCall 是模型发起的一次工具调用请求。
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // JSON 字符串，由模型生成
}

// Tool 是一个可被模型调用的能力。MVP 阶段面试官只挂只读检索工具。
// 该接口同时是未来的 MCP 适配口：接一个 MCP server 时写一个适配器即可。
type Tool interface {
	Name() string
	Description() string
	// Schema 返回给模型看的参数 JSON Schema。
	Schema() []byte
	// Call 执行工具。args 为模型生成的 JSON 字符串。
	Call(ctx context.Context, args string) (string, error)
}

// GenerateRequest 是一次模型调用的完整输入。
type GenerateRequest struct {
	// PromptKey/PromptVersion 用于审计与回溯（对应 backend/prompts/<key>/<version>）。
	PromptKey     string
	PromptVersion string

	Messages []Message
	Tools    []Tool // 为空表示本轮不允许工具调用

	// JSONSchema requests a structured JSON response when the configured
	// provider supports native structured outputs. Callers must still decode
	// and validate the returned JSON in Go.
	SchemaName string
	JSONSchema []byte

	// 成本与护栏
	MaxTokens   int
	Temperature *float64

	// PreferSmall asks the instrumented wrapper to use the configured small
	// model when a soft quota threshold has been crossed.
	PreferSmall bool

	// 审计关联
	UserID         string
	TaskID         string
	SessionID      string
	ResourceType   string
	ResourceID     string
	IdempotencyKey string
}

// GenerateResponse 是一次模型调用的输出。
type GenerateResponse struct {
	Message  Message
	Usage    Usage
	Provider string
	Model    string
}

// Usage 记录 token 消耗，用于成本治理与审计。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ChatModel 是对话模型抽象。Generate 为同步调用；Stream 预留流式（SSE）。
type ChatModel interface {
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}

// EmbeddingModel 是向量化抽象。MVP 直喂全文不使用；为 Beta 面经检索预留。
type EmbeddingModel interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
