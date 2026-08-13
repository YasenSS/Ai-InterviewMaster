package mcp

import (
	"context"
	"fmt"
	"strings"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
)

type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      string `json:"schema"`
}

// Adapter exposes platform Tools through a list/call mouth. It does not speak
// the MCP network protocol and does not connect to external MCP servers.
type Adapter struct {
	tools []platformai.Tool
}

func NewAdapter(tools ...platformai.Tool) Adapter {
	return Adapter{tools: tools}
}

func (a Adapter) ListTools() []ToolInfo {
	result := make([]ToolInfo, 0, len(a.tools))
	for _, tool := range a.tools {
		if tool == nil {
			continue
		}
		result = append(result, ToolInfo{
			Name:        tool.Name(),
			Description: tool.Description(),
			Schema:      string(tool.Schema()),
		})
	}
	return result
}

func (a Adapter) Call(ctx context.Context, name, args string) (string, error) {
	name = strings.TrimSpace(name)
	for _, tool := range a.tools {
		if tool == nil || tool.Name() != name {
			continue
		}
		return tool.Call(ctx, platformai.StripTenantArgs(args))
	}
	return "", fmt.Errorf("unknown tool %q", name)
}
