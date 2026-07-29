package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	platformauth "github.com/interviewmaster/interviewmaster/backend/internal/platform/auth"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func currentUserID(ctx context.Context) (string, error) {
	userID, err := platformauth.UserID(ctx)
	if err != nil {
		return "", apperror.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	return userID, nil
}

func validateID(field, value string) error {
	if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
		return apperror.Validation(map[string][]string{
			field: {"必须是有效的 UUID"},
		})
	}
	return nil
}

func validateTitle(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	length := utf8.RuneCountInString(value)
	if length < 1 || length > 120 {
		return "", apperror.Validation(map[string][]string{
			field: {"长度必须为 1–120 个字符"},
		})
	}
	return value, nil
}

func validateOptionalText(field, value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > max {
		return "", apperror.Validation(map[string][]string{
			field: {fmt.Sprintf("长度不能超过 %d 个字符", max)},
		})
	}
	return value, nil
}

func validateAnswer(value string) (string, error) {
	value = strings.TrimSpace(value)
	length := utf8.RuneCountInString(value)
	if length < 1 || length > 20000 {
		return "", apperror.Validation(map[string][]string{
			"answer": {"回答长度必须为 1–20,000 个字符"},
		})
	}
	return value, nil
}

func pageParams(page, pageSize int) (int, int, int, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	fields := make(map[string][]string)
	if page < 1 {
		fields["page"] = []string{"必须大于等于 1"}
	}
	if pageSize < 1 || pageSize > maxPageSize {
		fields["page_size"] = []string{"必须在 1–100 之间"}
	}
	if len(fields) > 0 {
		return 0, 0, 0, apperror.Validation(fields)
	}
	return page, pageSize, (page - 1) * pageSize, nil
}

func parseEnumFilter(field string, values []string, allowed map[string]struct{}) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := allowed[item]; !ok {
				return nil, apperror.Validation(map[string][]string{
					field: {"包含不支持的值"},
				})
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result, nil
}

func sortClause(value, defaultValue string, allowed map[string]string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultValue
	}
	clause, ok := allowed[value]
	if !ok {
		return "", apperror.Validation(map[string][]string{
			"sort": {"不支持该排序方式"},
		})
	}
	return clause, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}

func extractCapabilities(content string) []string {
	keywords := []string{
		"Go", "Java", "Python", "SQL", "PostgreSQL", "Redis", "Docker",
		"Kubernetes", "React", "TypeScript", "系统设计", "微服务", "分布式", "机器学习",
	}
	lower := strings.ToLower(content)
	result := make([]string, 0, 6)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			result = append(result, keyword)
		}
	}
	return result
}

func capabilitiesJSON(values []string) ([]byte, error) {
	data, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode capabilities: %w", err)
	}
	return data, nil
}

func encodeStrings(values []string) []byte {
	if values == nil {
		values = []string{}
	}
	data, _ := json.Marshal(values)
	return data
}

func decodeStrings(raw []byte) []string {
	values := []string{}
	_ = json.Unmarshal(raw, &values)
	return values
}

func nullUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func resourceNotFound(code, message string, cause error) error {
	return apperror.NotFound(code, message, cause)
}

func conflict(code, message string, details any) error {
	return apperror.Conflict(code, message, details)
}

func unsupportedMedia(code, message string) error {
	return apperror.New(code, message, http.StatusUnsupportedMediaType, nil, nil)
}

func requestEntityTooLarge(code, message string) error {
	return apperror.New(code, message, http.StatusRequestEntityTooLarge, nil, nil)
}
