package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	platformauth "github.com/interviewmaster/interviewmaster/backend/internal/platform/auth"
)

func currentUserID(ctx context.Context) (string, error) {
	return platformauth.UserID(ctx)
}

func extractCapabilities(content string) []string {
	keywords := []string{"Go", "Java", "Python", "SQL", "PostgreSQL", "Redis", "Docker", "Kubernetes", "React", "TypeScript", "系统设计", "微服务", "分布式", "机器学习"}
	lower := strings.ToLower(content)
	result := make([]string, 0, 6)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) { result = append(result, keyword) }
	}
	return result
}

func capabilitiesJSON(values []string) ([]byte, error) {
	data, err := json.Marshal(values)
	if err != nil { return nil, fmt.Errorf("encode capabilities: %w", err) }
	return data, nil
}
